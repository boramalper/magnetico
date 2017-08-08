package main

import (
	"fmt"
	"database/sql"
	"net/url"

	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"

	"magneticod/bittorrent"

	"path"
	"os"
)


type engineType uint8

const (
	SQLITE engineType = 0
	POSTGRESQL = 1
)


type Database struct {
	database *sql.DB
	engine engineType
	newTorrents chan bittorrent.Metadata
}


// NewDatabase creates a new Database.
//
// url either starts with "sqlite:" or "postgresql:"
func NewDatabase(rawurl string) (*Database, error) {
	db := Database{}

	dbURL, err := url.Parse(rawurl)
	if err != nil {
		return nil, err
	}

	switch dbURL.Scheme {
	case "sqlite":
		db.engine = SQLITE
		// All this pain is to make sure that an empty file exist (in case the database is not there
		// yet) so that sql.Open won't fail.
		dbDir, _ := path.Split(dbURL.Path)
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			return nil, fmt.Errorf("for directory `%s`:  %s", dbDir, err.Error())
		}
		f, err := os.OpenFile(dbURL.Path, os.O_CREATE, 0666)
		if err != nil {
			return nil, fmt.Errorf("for file `%s`: %s", dbURL.Path, err.Error())
		}
		if err := f.Sync(); err != nil {
			return nil, fmt.Errorf("for file `%s`: %s", dbURL.Path, err.Error())
		}
		if err := f.Close(); err != nil {
			return nil, fmt.Errorf("for file `%s`: %s", dbURL.Path, err.Error())
		}
		db.database, err = sql.Open("sqlite3", dbURL.RawPath)

	case "postgresql":
		db.engine = POSTGRESQL
		db.database, err = sql.Open("postgresql", rawurl)

	default:
		return nil, fmt.Errorf("unknown URI scheme (or malformed URI)!")
	}

	// Check for errors from sql.Open()
	if err != nil {
		return nil, fmt.Errorf("sql.Open()!  %s", err.Error())
	}

	if err = db.database.Ping(); err != nil {
		return nil, fmt.Errorf("DB.Ping()!  %s", err.Error())
	}

	if err := db.setupDatabase(); err != nil {
		return nil, fmt.Errorf("setupDatabase()!  %s", err.Error())
	}

	db.newTorrents = make(chan bittorrent.Metadata, 10)

	return &db, nil
}


// AddNewTorrent adds a new torrent to the *queue* to be flushed to the persistent database.
func (db *Database) AddNewTorrent(torrent bittorrent.Metadata) error {
	for {
		select {
		case db.newTorrents <- torrent:
			return nil
		default:
			// newTorrents queue was full: flush and try again and again (and again)...
			err := db.flushNewTorrents()
			if err != nil {
				return err
			}
			continue
		}
	}
}


func (db *Database) flushNewTorrents() error {
	tx, err := db.database.Begin()
	if err != nil {
		return fmt.Errorf("sql.DB.Begin()!  %s", err.Error())
	}

	var nTorrents, nFiles uint
	for torrent := range db.newTorrents {
		res, err := tx.Exec("INSERT INTO torrents (info_hash, name, total_size, discovered_on) VALUES ($1, $2, $3, $4);",
			torrent.InfoHash, torrent.Name, torrent.TotalSize, torrent.DiscoveredOn)
		if err != nil {
			ourError := fmt.Errorf("error while INSERTing INTO torrents!  %s", err.Error())
			if err := tx.Rollback(); err != nil {
				return fmt.Errorf("%s\tmeanwhile, could not rollback the current transaction either!  %s", ourError.Error(), err.Error())
			}
			return ourError
		}
		var lastInsertId int64
		if lastInsertId, err = res.LastInsertId(); err != nil {
			return fmt.Errorf("sql.Result.LastInsertId()!  %s", err.Error())
		}

		for _, file := range torrent.Files {
			_, err := tx.Exec("INSERT INTO files (torrent_id, size, path) VALUES($1, $2, $3);",
			lastInsertId, file.Length, file.Path)
			if err != nil {
				ourError := fmt.Errorf("error while INSERTing INTO files!  %s", err.Error())
				if err := tx.Rollback(); err != nil {
					return fmt.Errorf("%s\tmeanwhile, could not rollback the current transaction either!  %s", ourError.Error(), err.Error())
				}
				return ourError
			}
			nFiles++
		}
		nTorrents++
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("sql.Tx.Commit()!  %s", err.Error())
	}

	zap.L().Sugar().Infof("%d torrents (%d files) are flushed to the database successfully.",
		nTorrents, nFiles)
	return nil
}


func (db *Database) Close() {
	// Be careful to not to get into an infinite loop. =)
	db.database.Close()
}


func (db *Database) setupDatabase() error {
	switch db.engine {
	case SQLITE:
		return setupSqliteDatabase(db.database)

	case POSTGRESQL:
		zap.L().Fatal("setupDatabase() is not implemented for PostgreSQL yet!")

	default:
		zap.L().Sugar().Fatalf("Unknown database engine value %d! (programmer error)", db.engine)
	}

	return nil
}


func setupSqliteDatabase(database *sql.DB) error {
	// Enable Write-Ahead Logging for SQLite as "WAL provides more concurrency as readers do not
	// block writers and a writer does not block readers. Reading and writing can proceed
	// concurrently."
	// Caveats:
	//   * Might be unsupported by OSes other than Windows and UNIXes.
	//   * Does not work over a network filesystem.
	//   * Transactions that involve changes against multiple ATTACHed databases are not atomic
	//     across all databases as a set.
	// See: https://www.sqlite.org/wal.html
	//
	// Force SQLite to use disk, instead of memory, for all temporary files to reduce the memory
	// footprint.
	//
	// Enable foreign key constraints in SQLite which are crucial to prevent programmer errors on
	// our side.
	_, err := database.Exec(
		`PRAGMA journal_mode=WAL;
		PRAGMA temp_store=1;
		PRAGMA foreign_keys=ON;`,
	)
	if err != nil {
		return err
	}

	_, err = database.Exec(
		`CREATE TABLE IF NOT EXISTS torrents (
			id             INTEGER PRIMARY KEY,
			info_hash      BLOB NOT NULL UNIQUE,
			name           TEXT NOT NULL,
			total_size     INTEGER NOT NULL CHECK(total_size > 0),
			discovered_on  INTEGER NOT NULL CHECK(discovered_on > 0)
		);

		CREATE INDEX IF NOT EXISTS info_hash_index ON torrents (info_hash);

		CREATE TABLE IF NOT EXISTS files (
			id          INTEGER PRIMARY KEY,
			torrent_id  INTEGER REFERENCES torrents ON DELETE CASCADE ON UPDATE RESTRICT,
			size        INTEGER NOT NULL,
			path        TEXT NOT NULL
		);`,
	)
	if err != nil {
		return err
	}

	return nil
}
