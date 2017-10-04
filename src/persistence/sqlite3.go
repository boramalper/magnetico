package persistence

import (
	"net/url"
	"path"
	"os"
	"fmt"
	"database/sql"
	"regexp"

	"go.uber.org/zap"
	"time"
)

type sqlite3Database struct {
	conn *sql.DB
}

func (db *sqlite3Database) Engine() databaseEngine {
	return SQLITE3_ENGINE
}

func makeSqlite3Database(url_ *url.URL) (Database, error) {
	db := new(sqlite3Database)

	dbDir, _ := path.Split(url_.Path)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("for directory `%s`:  %s", dbDir, err.Error())
	}

	var err error
	db.conn, err = sql.Open("sqlite3", url_.Path)
	if err != nil {
		return nil, err
	}

	// > Open may just validate its arguments without creating a connection to the database. To
	// > verify that the data source name is valid, call Ping.
	// https://golang.org/pkg/database/sql/#Open
	if err = db.conn.Ping(); err != nil {
		return nil, err
	}

	if err := db.setupDatabase(); err != nil {
		return nil, err
	}

	return db, nil
}

func (db *sqlite3Database) DoesTorrentExist(infoHash []byte) (bool, error) {
	rows, err := db.conn.Query("SELECT 1 FROM torrents WHERE info_hash = ?;", infoHash)
	if err != nil {
		return false, err;
	}

	// If rows.Next() returns true, meaning that the torrent is in the database, return true; else
	// return false.
	exists := rows.Next()

	if err = rows.Close(); err != nil {
		return false, err;
	}

	return exists, nil;
}

func (db *sqlite3Database) GiveAnIncompleteTorrent(pathRegex *regexp.Regexp, maxSize uint) (infoHash []byte, path string, err error) {
	rows, err := db.conn.Query("SELECT info_hash FROM torrents WHERE has_readme = 0;")
	if err != nil {
		return nil, "", err
	}

	if rows.Next() != true {
		return nil, "", nil
	}

	if err = rows.Scan(&infoHash); err != nil {
		return nil, "", err
	}

	if err = rows.Close(); err != nil {
		return nil, "", err
	}

	// TODO
	return infoHash, "", nil
}

func (db *sqlite3Database) GiveAStaleTorrent() (infoHash []byte, err error) {
	// TODO
	return nil, nil
}

func (db *sqlite3Database) AddNewTorrent(infoHash []byte, name string, files []File) error {
	// Although we check whether the torrent exists in the database before asking MetadataSink to
	// fetch its metadata, the torrent can also exists in the Sink before that. Now, if a torrent in
	// the sink is still being fetched, that's still not a problem as we just add the new peer for
	// the torrent and exit, but if the torrent is complete (i.e. its metadata) and if its waiting
	// in the channel to be received, a race condition arises when we query the database and seeing
	// that it doesn't exists there, add it to the sink.
	// Hence check for the last time whether the torrent exists in the database, and only if not,
	// add it.
	exists, err := db.DoesTorrentExist(infoHash)
	if err != nil {
		return err;
	} else if exists {
		return nil;
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	// If everything goes as planned and no error occurs, we will commit the transaction before
	// returning from the function so the tx.Rollback() call will fail, trying to rollback a
	// committed transaction. BUT, if an error occurs, we'll get our transaction rollback'ed, which
	// is nice.
	defer tx.Rollback()

	var total_size int64 = 0
	for _, file := range files {
		total_size += file.Size
	}

	res, err := tx.Exec(`
		INSERT INTO torrents (
			info_hash,
			name,
			total_size,
			discovered_on,
			n_files,
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?);
	`, infoHash, name, total_size, time.Now().Unix(), len(files))
	if err != nil {
		return err
	}

	var lastInsertId int64
	if lastInsertId, err = res.LastInsertId(); err != nil {
		return fmt.Errorf("sql.Result.LastInsertId()!  %s", err.Error())
	}

	for _, file := range files {
		_, err = tx.Exec("INSERT INTO files (torrent_id, size, path) VALUES (?, ?, ?);",
			lastInsertId, file.Size, file.Path,
		)
		if err != nil {
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (db *sqlite3Database) AddReadme(infoHash []byte, path string, content string) error {
	_, err := db.conn.Exec(
		`UPDATE files SET is_readme = 1, content = ?
		WHERE path = ? AND (SELECT id FROM torrents WHERE info_hash = ?) = torrent_id;`,
		content, path, infoHash,
	)
	if err != nil {
		return err
	}

	return nil
}

func (db *sqlite3Database) Close() error {
	return db.conn.Close()
}

func (db *sqlite3Database) GetNumberOfTorrents() (uint, error) {
	// COUNT(ROWID) is much more inefficient since it scans the whole table, so use MAX(ROWID)
	rows, err := db.conn.Query("SELECT MAX(ROWID) FROM torrents;")
	if err != nil {
		return 0, err
	}

	if rows.Next() != true {
		fmt.Errorf("No rows returned from `SELECT MAX(ROWID)`!")
	}

	var n uint
	if err = rows.Scan(&n); err != nil {
		return 0, err
	}

	if err = rows.Close(); err != nil {
		return 0, err
	}

	return n, nil
}

func (db *sqlite3Database) NewestTorrents(n uint) ([]TorrentMetadata, error) {
	rows, err := db.conn.Query(`
		SELECT
			info_hash,
			name,
			total_size,
			discovered_on,
			has_readme,
			n_files,
			n_seeders,
			n_leechers,
			updated_on
		FROM torrents
		ORDER BY discovered_on DESC LIMIT ?;
		`, n,
	)
	if err != nil {
		return nil, err
	}

	var torrents []TorrentMetadata
	for rows.Next() {
		tm := new(TorrentMetadata)
		rows.Scan(
			&tm.infoHash, &tm.name, &tm.discoveredOn, &tm.hasReadme, &tm.nFiles, &tm.nSeeders,
			&tm.nLeechers, &tm.updatedOn,
		)
		torrents = append(torrents, *tm)
	}

	if err = rows.Close(); err != nil {
		return nil, err
	}

	return torrents, nil
}

func (db *sqlite3Database) SearchTorrents(query string, orderBy orderingCriteria, descending bool, mustHaveReadme bool) ([]TorrentMetadata, error) { // TODO
	// TODO:
	return nil, nil
}

func (db *sqlite3Database) GetTorrent(infoHash []byte) (*TorrentMetadata, error) {
	rows, err := db.conn.Query(
		`SELECT
			info_hash,
			name,
			size,
			discovered_on,
			has_readme,
			n_files,
			n_seeders,
			n_leechers,
			updated_on
		FROM torrents
		WHERE info_hash = ?`,
		infoHash,
	)
	if err != nil {
		return nil, err
	}

	if rows.Next() != true {
		return nil, nil
	}

	tm := new(TorrentMetadata)
	rows.Scan(
		&tm.infoHash, &tm.name, &tm.discoveredOn, &tm.hasReadme, &tm.nFiles, &tm.nSeeders,
		&tm.nLeechers, &tm.updatedOn,
	)

	if err = rows.Close(); err != nil {
		return nil, err
	}

	return tm, nil
}

func (db *sqlite3Database) GetFiles(infoHash []byte) ([]File, error) {
	// TODO
	return nil, nil
}

func (db *sqlite3Database) GetReadme(infoHash []byte) (string, error) {
	// TODO
	return "", nil
}


func (db *sqlite3Database) GetStatistics(from ISO8601, period uint) (*Statistics, error) {
	// TODO
	return nil, nil
}

func (db *sqlite3Database) commitQueuedTorrents() error {
	return nil
}

func (db *sqlite3Database) setupDatabase() error {
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
	_, err := db.conn.Exec(`
		PRAGMA journal_mode=WAL;
		PRAGMA temp_store=1;
		PRAGMA foreign_keys=ON;
	`)
	if err != nil {
		return err
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	// If everything goes as planned and no error occurs, we will commit the transaction before
	// returning from the function so the tx.Rollback() call will fail, trying to rollback a
	// committed transaction. BUT, if an error occurs, we'll get our transaction rollback'ed, which
	// is nice.
	defer tx.Rollback()

	// Essential, and valid for all user_version`s:
	// TODO: "torrent_id" column of the "files" table can be NULL, how can we fix this in a new schema?
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS torrents (
			id             INTEGER PRIMARY KEY,
			info_hash      BLOB NOT NULL UNIQUE,
			name           TEXT NOT NULL,
			total_size     INTEGER NOT NULL CHECK(total_size > 0),
			discovered_on  INTEGER NOT NULL CHECK(discovered_on > 0)
		);
		CREATE TABLE IF NOT EXISTS files (
			id          INTEGER PRIMARY KEY,
			torrent_id  INTEGER REFERENCES torrents ON DELETE CASCADE ON UPDATE RESTRICT,
			size        INTEGER NOT NULL,
			path        TEXT NOT NULL
		);
	`)
	if err != nil {
		return err
	}

	// Get the user_version:
	res, err := tx.Query("PRAGMA user_version;")
	if err != nil {
		return err
	}
	var userVersion int;
	if res.Next() != true {
		return fmt.Errorf("PRAGMA user_version did not return any rows!")
	}
	if err = res.Scan(&userVersion); err != nil {
		return err
	}

	switch userVersion {
	// Upgrade from user_version 0 to 1
	// The Change:
	//   * `info_hash_index` is recreated as UNIQUE.
	case 0:
		zap.S().Warnf("Updating database schema from 0 to 1... (this might take a while)")
		_, err = tx.Exec(`
			DROP INDEX info_hash_index;
			CREATE UNIQUE INDEX info_hash_index ON torrents	(info_hash);
			PRAGMA user_version = 1;
		`)
		if err != nil {
			return err
		}
		fallthrough
	// Upgrade from user_version 1 to 2
	// The Change:
	//   * Added `is_readme` and `content` columns to the `files` table, and the constraints & the
	//     the indices they entail.
	//     * Added unique index `readme_index`  on `files` table.
	case 1:
		zap.S().Warnf("Updating database schema from 1 to 2... (this might take a while)")
		// We introduce two new columns here: content BLOB, and is_readme INTEGER which we treat as
		// a bool (hence the CHECK).
		// The reason for the change is that as we introduce the new "readme" feature which
		// downloads a readme file as a torrent descriptor, we needed to store it somewhere in the
		// database with the following conditions:
		//
		//   1. There can be one and only one readme (content) for a given torrent; hence the
		//      UNIQUE INDEX on (torrent_id, is_description) (remember that SQLite treats each NULL
		//      value as distinct [UNIQUE], see https://sqlite.org/nulls.html).
		//   2. We would like to keep the readme (content) associated with the file it came from;
		//      hence we modify the files table instead of the torrents table.
		//
		// Regarding the implementation details, following constraints arise:
		//
		//   1. The column is_readme is either NULL or 1, and if it is 1, then content column cannot
		//      be NULL (but might be an empty BLOB). Vice versa, if content column of a row is,
		//      NULL then is_readme must be NULL.
		//
		//      This is to prevent unused content fields filling up the database, and to catch
		//      programmers' errors.
		_, err = tx.Exec(`
			ALTER TABLE files ADD COLUMN is_readme INTEGER CHECK (is_readme IS NULL OR is_readme=1) DEFAULT NULL;
			ALTER TABLE files ADD COLUMN content   BLOB CHECK((content IS NULL AND is_readme IS NULL) OR (content IS NOT NULL AND is_readme=1)) DEFAULT NULL;
			CREATE UNIQUE INDEX readme_index ON files (torrent_id, is_readme);
			PRAGMA user_version = 2;
		`)
		if err != nil {
			return err
		}
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}