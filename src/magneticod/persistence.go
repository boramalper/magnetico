package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"net/url"
	"path"
	"os"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"

	"magneticod/bittorrent"
)

type engineType uint8

const (
	SQLITE     engineType = 0
	POSTGRESQL            = 1
	MYSQL                 = 2
)

type Database struct {
	database    *sql.DB
	engine      engineType
	newTorrents [] bittorrent.Metadata
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

	case "mysql":
		db.engine = MYSQL
		db.database, err = sql.Open("mysql", rawurl)

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


func (db *Database) DoesExist(infoHash []byte) bool {
	for _, torrent := range db.newTorrents {
		if bytes.Equal(infoHash, torrent.InfoHash) {
			return true;
		}
	}

	rows, err := db.database.Query("SELECT info_hash FROM torrents WHERE info_hash = ?;", infoHash)
	if err != nil {
		zap.L().Sugar().Fatalf("Could not query whether a torrent exists in the database! %s", err.Error())
	}
	defer rows.Close()

	// If rows.Next() returns true, meaning that the torrent is in the database, return true; else
	// return false.
	return rows.Next()
}


// AddNewTorrent adds a new torrent to the *queue* to be flushed to the persistent database.
func (db *Database) AddNewTorrent(torrent bittorrent.Metadata) error {
	// Although we check whether the torrent exists in the database before asking MetadataSink to
	// fetch its metadata, the torrent can also exists in the Sink before that. Now, if a torrent in
	// the sink is still being fetched, that's still not a problem as we just add the new peer for
	// the torrent and exit, but if the torrent is complete (i.e. its metadata) and if its waiting
	// in the channel to be received, a race condition arises when we query the database and seeing
	// that it doesn't exists there, add it to the sink.
	// Hence check for the last time whether the torrent exists in the database, and only if not,
	// add it.
	if db.DoesExist(torrent.InfoHash) {
		return nil;
	}

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

	case MYSQL:
		return setupMySQLDatabase(db.database)

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

	tx, err := database.Begin()
	if err != nil {
		return err
	}

	// Essential, and valid for all user_version`s:
	_, err = tx.Exec(
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
		);
		`,
	)
	if err != nil {
		return err
	}

	// Get the user_version:
	res, err := tx.Query(
		`PRAGMA user_version;`,
	)
	if err != nil {
		return err
	}
	var userVersion int;
	res.Next()
	res.Scan(&userVersion)

	// Upgrade to the latest schema:
	switch userVersion {
	// Upgrade from user_version 0 to 1
	case 0:
		_, err = tx.Exec(
			`ALTER TABLE torrents ADD COLUMN readme TEXT;
			PRAGMA user_version = 1;`,
		)
		if err != nil {
			return err
		}
		// Add `fallthrough`s as needed to keep upgrading...
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

func setupMySQLDatabase(database *sql.DB) error {
	// Set strict mode to prevent silent truncation
	_, err := database.Exec(`SET SESSION SQL_MODE = 'STRICT_ALL_TABLES';`)
	if err != nil {
		return err
	}

	_, err = database.Exec(
		`CREATE TABLE IF NOT EXISTS torrents ("
			id		INTEGER PRIMARY KEY AUTO_INCREMENT,
			info_hash	BINARY(20) NOT NULL UNIQUE,
			name		VARCHAR(1024) NOT NULL,
			total_size	BIGINT UNSIGNED NOT NULL,
			discovered_on	INTEGER UNSIGNED NOT NULL
		);

		ALTER TABLE torrents ADD INDEX info_hash_index (info_hash);

		CREATE TABLE IF NOT EXISTS files (
			id		INTEGER PRIMARY KEY AUTO_INCREMENT,
			torrent_id	INTEGER REFERENCES torrents (id) ON DELETE CASCADE ON UPDATE RESTRICT,
			size		BIGINT NOT NULL,
			path		TEXT NOT NULL
		);`,
	)

	if err != nil {
		return err
	}

	return nil
}
