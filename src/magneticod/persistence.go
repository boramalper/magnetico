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
	newTorrents []bittorrent.Metadata
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
		dbDir, _ := path.Split(dbURL.Path)
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			return nil, fmt.Errorf("for directory `%s`:  %s", dbDir, err.Error())
		}
		db.database, err = sql.Open("sqlite3", dbURL.Path)

	case "postgresql":
		db.engine = POSTGRESQL
		db.database, err = sql.Open("postgresql", rawurl)

	default:
		return nil, fmt.Errorf("unknown URI scheme (or malformed URI)!")
	}

	// Check for errors from sql.Open()
	if err != nil {
		return nil, fmt.Errorf("error in sql.Open():  %s", err.Error())
	}

	if err = db.database.Ping(); err != nil {
		return nil, fmt.Errorf("error in DB.Ping():  %s", err.Error())
	}

	if err := db.setupDatabase(); err != nil {
		return nil, fmt.Errorf("error in setupDatabase():  %s", err.Error())
	}

	return &db, nil
}


// AddNewTorrent adds a new torrent to the *queue* to be flushed to the persistent database.
func (db *Database) AddNewTorrent(torrent bittorrent.Metadata) error {
	db.newTorrents = append(db.newTorrents, torrent)

	if len(db.newTorrents) >= 10 {
		zap.L().Sugar().Debugf("newTorrents queue is full, attempting to commit %d torrents...",
			len(db.newTorrents))
		if err := db.commitNewTorrents(); err != nil {
			return err
		}
	}

	return nil
}


func (db *Database) commitNewTorrents() error {
	tx, err := db.database.Begin()
	if err != nil {
		return fmt.Errorf("sql.DB.Begin()!  %s", err.Error())
	}

	var nTorrents, nFiles uint
	nTorrents = uint(len(db.newTorrents))
	for i, torrent := range db.newTorrents {
		zap.L().Sugar().Debugf("Flushing torrent %d of %d: `%s` (%x)...",
			i + 1, len(db.newTorrents), torrent.Name, torrent.InfoHash)
		res, err := tx.Exec("INSERT INTO torrents (info_hash, name, total_size, discovered_on) VALUES (?, ?, ?, ?);",
			torrent.InfoHash, torrent.Name, torrent.TotalSize, torrent.DiscoveredOn)
		if err != nil {
			ourError := fmt.Errorf("error while INSERTing INTO torrent:  %s", err.Error())
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
			zap.L().Sugar().Debugf("Flushing file `%s` (of torrent %x)", path.Join(file.Path...), torrent.InfoHash)
			_, err := tx.Exec("INSERT INTO files (torrent_id, size, path) VALUES(?, ?, ?);",
				lastInsertId, file.Length, path.Join(file.Path...))
			if err != nil {
				ourError := fmt.Errorf("error while INSERTing INTO files:  %s", err.Error())
				if err := tx.Rollback(); err != nil {
					return fmt.Errorf("%s\tmeanwhile, could not rollback the current transaction either!  %s", ourError.Error(), err.Error())
				}
				return ourError
			}
		}
		nFiles += uint(len(torrent.Files))
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("sql.Tx.Commit()!  %s", err.Error())
	}

	// Clear the queue
	db.newTorrents = nil

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
