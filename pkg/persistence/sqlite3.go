package persistence

import (
	"bytes"
	"database/sql"
	"fmt"
	"text/template"
	"net/url"
	"os"
	"path"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

type sqlite3Database struct {
	conn *sql.DB
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
		return nil, fmt.Errorf("sql.Open: %s", err.Error())
	}

	// > Open may just validate its arguments without creating a connection to the database. To
	// > verify that the data source Name is valid, call Ping.
	// https://golang.org/pkg/database/sql/#Open
	if err = db.conn.Ping(); err != nil {
		return nil, fmt.Errorf("sql.DB.Ping: %s", err.Error())
	}

	if err := db.setupDatabase(); err != nil {
		return nil, fmt.Errorf("setupDatabase: %s", err.Error())
	}

	return db, nil
}

func (db *sqlite3Database) Engine() databaseEngine {
	return Sqlite3
}

func (db *sqlite3Database) DoesTorrentExist(infoHash []byte) (bool, error) {
	rows, err := db.conn.Query("SELECT 1 FROM torrents WHERE info_hash = ?;", infoHash)
	if err != nil {
		return false, err
	}

	// If rows.Next() returns true, meaning that the torrent is in the database, return true; else
	// return false.
	exists := rows.Next()
	if !exists && rows.Err() != nil {
		return false, err
	}

	if err = rows.Close(); err != nil {
		return false, err
	}

	return exists, nil
}

func (db *sqlite3Database) AddNewTorrent(infoHash []byte, name string, files []File) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	// If everything goes as planned and no error occurs, we will commit the transaction before
	// returning from the function so the tx.Rollback() call will fail, trying to rollback a
	// committed transaction. BUT, if an error occurs, we'll get our transaction rollback'ed, which
	// is nice.
	defer tx.Rollback()

	var totalSize uint64 = 0
	for _, file := range files {
		totalSize += uint64(file.Size)
	}

	// This is a workaround for a bug: the database will not accept total_size to be zero.
	if totalSize == 0 {
		return nil
	}

	// Although we check whether the torrent exists in the database before asking MetadataSink to
	// fetch its metadata, the torrent can also exists in the Sink before that. Now, if a torrent in
	// the sink is still being fetched, that's still not a problem as we just add the new peer for
	// the torrent and exit, but if the torrent is complete (i.e. its metadata) and if its waiting
	// in the channel to be received, a race condition arises when we query the database and seeing
	// that it doesn't exists there, add it to the sink.
	// Hence INSERT OR IGNORE.
	res, err := tx.Exec(`
		INSERT OR IGNORE INTO torrents (
			info_hash,
			name,
			total_size,
			discovered_on
		) VALUES (?, ?, ?, ?);
	`, infoHash, name, totalSize, time.Now().Unix())
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

func (db *sqlite3Database) Close() error {
	return db.conn.Close()
}

func (db *sqlite3Database) GetNumberOfTorrents() (uint, error) {
	// COUNT(1) is much more inefficient since it scans the whole table, so use MAX(ROWID).
	// Keep in mind that the value returned by GetNumberOfTorrents() might be an approximation.
	rows, err := db.conn.Query("SELECT MAX(ROWID) FROM torrents;")
	if err != nil {
		return 0, err
	}

	if rows.Next() != true {
		fmt.Errorf("No rows returned from `SELECT MAX(ROWID)`")
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

func (db *sqlite3Database) QueryTorrents(
	query string,
	epoch int64,
	orderBy orderingCriteria,
	ascending bool,
	limit uint,
	lastOrderedValue *uint64,
	lastID *uint64,
) ([]TorrentMetadata, error) {
	if query == "" && orderBy == ByRelevance {
		return nil, fmt.Errorf("torrents cannot be ordered by relevance when the query is empty")
	}
	if (lastOrderedValue == nil) != (lastID == nil) {
		return nil, fmt.Errorf("lastOrderedValue and lastID should be supplied together, if supplied")
	}

	doJoin    := query != ""
	firstPage := true // lastID != nil

	// executeTemplate is used to prepare the SQL query, WITH PLACEHOLDERS FOR USER INPUT.
	sqlQuery := executeTemplate(`
		SELECT info_hash
			 , name
			 , total_size
			 , discovered_on
			 , (SELECT COUNT(*) FROM files WHERE torrents.id = files.torrent_id) AS n_files
		FROM torrents
	{{ if .DoJoin }}
		INNER JOIN (
			SELECT rowid AS id
				 , bm25(torrents_idx) AS rank
			FROM torrents_idx
			WHERE torrents_idx MATCH ?
		) AS idx USING(id)
	{{ end }}
		WHERE     modified_on <= ?
	{{ if not .FirstPage }}
			  AND id > ?
			  AND {{ .OrderOn }} {{ GTEorLTE .Ascending }} ?
	{{ end }}
		ORDER BY {{ .OrderOn }} {{ AscOrDesc .Ascending }}, id ASC
		LIMIT ?;	
	`, struct {
		DoJoin bool
		FirstPage bool
		OrderOn string
		Ascending bool
	}{
		DoJoin: doJoin,
		FirstPage: firstPage,
		OrderOn: orderOn(orderBy),
		Ascending: ascending,
	}, template.FuncMap{
		"GTEorLTE": func(ascending bool) string {
			// TODO: or maybe vice versa idk
			if ascending {
				return ">"
			} else {
				return "<"
			}
		},
		"AscOrDesc": func(ascending bool) string {
			if ascending {
				return "ASC"
			} else {
				return "DESC"
			}
		},
	})

	// Prepare query
	queryArgs := make([]interface{}, 0)
	if doJoin {
		queryArgs = append(queryArgs, query)
	}
	queryArgs = append(queryArgs, epoch)
	if !firstPage {
		queryArgs = append(queryArgs, lastID)
		queryArgs = append(queryArgs, lastOrderedValue)
	}
	queryArgs = append(queryArgs, limit)

	rows, err := db.conn.Query(sqlQuery, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("error while querying torrents: %s", err.Error())
	}

	torrents := make([]TorrentMetadata, 0)
	for rows.Next() {
		var torrent TorrentMetadata
		if err = rows.Scan(&torrent.InfoHash, &torrent.Name, &torrent.Size, &torrent.DiscoveredOn, &torrent.NFiles); err != nil {
			return nil, err
		}
		torrents = append(torrents, torrent)
	}

	if err := rows.Close(); err != nil {
		return nil, err
	}

	return torrents, nil
}

func orderOn(orderBy orderingCriteria) string {
	switch orderBy {
	case ByRelevance:
		return "idx.rank"

	case BySize:
		return "total_size"

	case ByDiscoveredOn:
		return "discovered_on"

	case ByNFiles:
		return "n_files"

	default:
		panic(fmt.Sprintf("unknown orderBy: %v", orderBy))
	}
}

func (db *sqlite3Database) GetTorrent(infoHash []byte) (*TorrentMetadata, error) {
	rows, err := db.conn.Query(`
		SELECT
			info_hash,
			name,
			total_size,
			discovered_on,
			(SELECT COUNT(*) FROM files WHERE torrent_id = torrents.id) AS n_files
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

	var tm TorrentMetadata
	if err = rows.Scan(&tm.InfoHash, &tm.Name, &tm.Size, &tm.DiscoveredOn, &tm.NFiles); err != nil {
		return nil, err
	}

	if err = rows.Close(); err != nil {
		return nil, err
	}

	return &tm, nil
}

func (db *sqlite3Database) GetFiles(infoHash []byte) ([]File, error) {
	rows, err := db.conn.Query("SELECT size, path FROM files WHERE torrent_id = ?;", infoHash)
	if err != nil {
		return nil, err
	}

	var files []File
	for rows.Next() {
		var file File
		if err = rows.Scan(&file.Size, &file.Path); err != nil {
			return nil, err
		}
		files = append(files, file)
	}

	if err := rows.Close(); err != nil {
		return nil, err
	}

	return files, nil
}

func (db *sqlite3Database) GetStatistics(n uint, to string) (*Statistics, error) {
	/*
	to_time, granularity, err := ParseISO8601(to)
	if err != nil {
		return nil, fmt.Errorf("parsing @to error: %s", err.Error())
	}

	// TODO
	*/

	return nil, nil
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
		PRAGMA encoding='UTF-8';
	`)
	if err != nil {
		return fmt.Errorf("sql.DB.Exec (PRAGMAs): %s", err.Error())
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("sql.DB.Begin: %s", err.Error())
	}
	// If everything goes as planned and no error occurs, we will commit the transaction before
	// returning from the function so the tx.Rollback() call will fail, trying to rollback a
	// committed transaction. BUT, if an error occurs, we'll get our transaction rollback'ed, which
	// is nice.
	defer tx.Rollback()

	// Initial Setup for `user_version` 0:
	// FROZEN.
	// TODO: "torrent_id" column of the "files" table can be NULL, how can we fix this in a new
	//       version schema?
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
		return fmt.Errorf("sql.Tx.Exec (v0): %s", err.Error())
	}

	// Get the user_version:
	rows, err := tx.Query("PRAGMA user_version;")
	if err != nil {
		return fmt.Errorf("sql.Tx.Query (user_version): %s", err.Error())
	}
	var userVersion int
	if rows.Next() != true {
		return fmt.Errorf("sql.Rows.Next (user_version): PRAGMA user_version did not return any rows!")
	}
	if err = rows.Scan(&userVersion); err != nil {
		return fmt.Errorf("sql.Rows.Scan (user_version): %s", err.Error())
	}
	// Close your rows lest you get "database table is locked" error(s)!
	// See https://github.com/mattn/go-sqlite3/issues/2741
	if err = rows.Close(); err != nil {
		return fmt.Errorf("sql.Rows.Close (user_version): %s", err.Error())
	}

	switch userVersion {
	case 0:  // FROZEN.
		// Upgrade from user_version 0 to 1
		// Changes:
		//   * `info_hash_index` is recreated as UNIQUE.
		zap.L().Warn("Updating database schema from 0 to 1... (this might take a while)")
		_, err = tx.Exec(`
			DROP INDEX IF EXISTS info_hash_index;
			CREATE UNIQUE INDEX info_hash_index ON torrents	(info_hash);
			PRAGMA user_version = 1;
		`)
		if err != nil {
			return fmt.Errorf("sql.Tx.Exec (v0 -> v1): %s", err.Error())
		}
		fallthrough

	case 1:  // FROZEN.
		// Upgrade from user_version 1 to 2
		// Changes:
		//   * Added `n_seeders`, `n_leechers`, and `updated_on` columns to the `torrents` table, and
		//     the constraints they entail.
		//   * Added `is_readme` and `content` columns to the `files` table, and the constraints & the
		//     the indices they entail.
		//     * Added unique index `readme_index`  on `files` table.
		zap.L().Warn("Updating database schema from 1 to 2... (this might take a while)")
		// We introduce two new columns in `files`: content BLOB, and is_readme INTEGER which we
		// treat as a bool (NULL for false, and 1 for true; see the CHECK statement).
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
		//   1. The column is_readme is either NULL or 1, and if it is 1, then column content cannot
		//      be NULL (but might be an empty BLOB). Vice versa, if column content of a row is,
		//      NULL then column is_readme must be NULL.
		//
		//      This is to prevent unused content fields filling up the database, and to catch
		//      programmers' errors.
		_, err = tx.Exec(`
			ALTER TABLE torrents ADD COLUMN updated_on INTEGER CHECK (updated_on > 0) DEFAULT NULL;
			ALTER TABLE torrents ADD COLUMN n_seeders  INTEGER CHECK ((updated_on IS NOT NULL AND n_seeders >= 0) OR (updated_on IS NULL AND n_seeders IS NULL)) DEFAULT NULL;
			ALTER TABLE torrents ADD COLUMN n_leechers INTEGER CHECK ((updated_on IS NOT NULL AND n_leechers >= 0) OR (updated_on IS NULL AND n_leechers IS NULL)) DEFAULT NULL;

			ALTER TABLE files ADD COLUMN is_readme INTEGER CHECK (is_readme IS NULL OR is_readme=1) DEFAULT NULL;
			ALTER TABLE files ADD COLUMN content   TEXT    CHECK ((content IS NULL AND is_readme IS NULL) OR (content IS NOT NULL AND is_readme=1)) DEFAULT NULL;
			CREATE UNIQUE INDEX readme_index ON files (torrent_id, is_readme);

			PRAGMA user_version = 2;
		`)
		if err != nil {
			return fmt.Errorf("sql.Tx.Exec (v1 -> v2): %s", err.Error())
		}
		fallthrough

	case 2:  // NOT FROZEN! (subject to change or complete removal)
		// Upgrade from user_version 2 to 3
		// Changes:
		//   * Created `torrents_idx` FTS5 virtual table.
		//
		//     See:
		//     * https://sqlite.org/fts5.html
		//     * https://sqlite.org/fts3.html
		//
		//   * Added `n_files` column to the `torrents` table.
		zap.L().Warn("Updating database schema from 2 to 3... (this might take a while)")
		_, err = tx.Exec(`
			CREATE VIRTUAL TABLE IF NOT EXISTS torrents_idx USING fts5(name, content='torrents', content_rowid='id', tokenize="porter unicode61 separators ' !""#$%&''()*+,-./:;<=>?@[\]^_` + "`" + `{|}~'");
			
			-- Populate the index
			INSERT INTO torrents_idx(rowid, name) SELECT id, name FROM torrents;

			-- Triggers to keep the FTS index up to date.
			CREATE TRIGGER torrents_ai AFTER INSERT ON torrents BEGIN
			  INSERT INTO torrents_idx(rowid, name) VALUES (new.id, new.name);
			END;
			CREATE TRIGGER torrents_ad AFTER DELETE ON torrents BEGIN
			  INSERT INTO torrents_idx(torrents_idx, rowid, name) VALUES('delete', old.id, old.name);
			END;
			CREATE TRIGGER torrents_au AFTER UPDATE ON torrents BEGIN
			  INSERT INTO torrents_idx(torrents_idx, rowid, name) VALUES('delete', old.id, old.name);
			  INSERT INTO torrents_idx(rowid, name) VALUES (new.id, new.name);
			END;

            -- Add column modified_on
			ALTER TABLE torrents ADD COLUMN modified_on INTEGER;
			UPDATE torrents SET modified_on = (SELECT discovered_on);
			CREATE INDEX modified_on_index ON torrents (modified_on);

			PRAGMA user_version = 3;
		`)
		if err != nil {
			return fmt.Errorf("sql.Tx.Exec (v2 -> v3): %s", err.Error())
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("sql.Tx.Commit: %s", err.Error())
	}

	return nil
}

func executeTemplate(text string, data interface{}, funcs template.FuncMap) string {
	t := template.Must(template.New("anon").Funcs(funcs).Parse(text))

	var buf bytes.Buffer
	err := t.Execute(&buf, data)
	if err != nil {
		panic(err.Error())
	}
	return buf.String()
}
