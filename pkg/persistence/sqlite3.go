package persistence

import (
	"bytes"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path"
	"text/template"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// Close your rows lest you get "database table is locked" error(s)!
// See https://github.com/mattn/go-sqlite3/issues/2741

type sqlite3Database struct {
	conn *sql.DB
}

func makeSqlite3Database(url_ *url.URL) (Database, error) {
	db := new(sqlite3Database)

	dbDir, _ := path.Split(url_.Path)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, errors.Wrapf(err, "mkdirAll error for `%s`", dbDir)
	}

	var err error
	// To handle spaces in the file path, we ensure that URI path handling is triggered in the
	// sqlite3 driver, and that escaping is applied to the URL on this side. See issue #240.
	url_.Scheme = "file"
	// To ensure that // isn't injected into the URI. The query is still handled.
	url_.Opaque = url_.Path
	db.conn, err = sql.Open("sqlite3", url_.String())
	if err != nil {
		return nil, errors.Wrap(err, "sql.Open")
	}

	// > Open may just validate its arguments without creating a connection to the database. To
	// > verify that the data source Name is valid, call Ping.
	// https://golang.org/pkg/database/sql/#Open
	if err = db.conn.Ping(); err != nil {
		return nil, errors.Wrap(err, "sql.DB.Ping")
	}

	// > After some time we receive "unable to open database file" error while trying to execute a transaction using
	// > Tx.Exec().
	// -- boramalper
	//
	// > Not sure if this would be contributing to your issue, but one of the problems we've observed in the past is the
	// > standard library's attempt to pool connections. (This makes more sense for database connections that are actual
	// > network connections, as opposed to SQLite.)
	// > Basically, the problem we encountered was that most pragmas (except specifically PRAGMA journal_mode=WAL, as
	// > per the documentation) apply to the connection, so if the standard library is opening/closing connections
	// > behind your back for pooling purposes, it can lead to unintended behavior.
	// -- rittneje
	//
	// https://github.com/mattn/go-sqlite3/issues/618
	//
	// Our solution is to set the connection max lifetime to infinity (reuse connection forever), and max open
	// connections to 3 (1 causes deadlocks, unlimited is too lax!). Max idle conns are set to 3 to persist connections
	// (instead of opening the database again and again).
	db.conn.SetConnMaxLifetime(0) // https://golang.org/pkg/database/sql/#DB.SetConnMaxLifetime
	db.conn.SetMaxOpenConns(3)
	db.conn.SetMaxIdleConns(3)

	if err := db.setupDatabase(); err != nil {
		return nil, errors.Wrap(err, "setupDatabase")
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
	defer rows.Close()

	// If rows.Next() returns true, meaning that the torrent is in the database, return true; else
	// return false.
	exists := rows.Next()
	if rows.Err() != nil {
		return false, err
	}

	return exists, nil
}

func (db *sqlite3Database) AddNewTorrent(infoHash []byte, name string, files []File) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return errors.Wrap(err, "conn.Begin")
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
		zap.L().Debug("Ignoring a torrent whose total size is zero.")
		return nil
	}

	// Although we check whether the torrent exists in the database before asking MetadataSink to
	// fetch its metadata, the torrent can also exists in the Sink before that:
	//
	// If the torrent is complete (i.e. its metadata) and if its waiting in the channel to be
	// received, a race condition arises when we query the database and seeing that it doesn't
	// exists there, add it to the sink.
	//
	// Do NOT try to be clever and attempt to use INSERT OR IGNORE INTO or INSERT OR REPLACE INTO
	// without understanding their consequences fully:
	//
	// https://www.sqlite.org/lang_conflict.html
	//
	//   INSERT OR IGNORE INTO
	//     INSERT OR IGNORE INTO will ignore:
	//       1. CHECK constraint violations
	//       2. UNIQUE or PRIMARY KEY constraint violations
	//       3. NOT NULL constraint violations
	//
	//     You would NOT want to ignore #1 and #2 as they are likely to indicate programmer errors.
	//     Instead of silently ignoring them, let the program err and investigate the causes.
	//
	//   INSERT OR REPLACE INTO
	//     INSERT OR REPLACE INTO will replace on:
	//       1. UNIQUE or PRIMARY KEY constraint violations (by "deleting pre-existing rows that are
	//          causing the constraint violation prior to inserting or updating the current row")
	//
	//     INSERT OR REPLACE INTO will abort on:
	//       2. CHECK constraint violations
	//       3. NOT NULL constraint violations (if "the column has no default value")
	//
	//     INSERT OR REPLACE INTO is definitely much closer to what you may want, but deleting
	//     pre-existing rows means that you might cause users loose data (such as seeder and leecher
	//     information, readme, and so on) at the expense of /your/ own laziness...
	if exist, err := db.DoesTorrentExist(infoHash); exist || err != nil {
		return err
	}

	res, err := tx.Exec(`
		INSERT INTO torrents (
			info_hash,
			name,
			total_size,
			discovered_on
		) VALUES (?, ?, ?, ?);
	`, infoHash, name, totalSize, time.Now().Unix())
	if err != nil {
		return errors.Wrap(err, "tx.Exec (INSERT OR REPLACE INTO torrents)")
	}

	var lastInsertId int64
	if lastInsertId, err = res.LastInsertId(); err != nil {
		return errors.Wrap(err, "sql.Result.LastInsertId")
	}

	// > last_insert_rowid()
	// >   The last_insert_rowid() function returns the ROWID of the last row insert from the
	// >   database connection which invoked the function. If no successful INSERTs into rowid
	// >   tables have ever occurred on the database connection, then last_insert_rowid() returns
	// >   zero.
	// https://www.sqlite.org/lang_corefunc.html#last_insert_rowid
	// https://www.sqlite.org/c3ref/last_insert_rowid.html
	//
	// Now, last_insert_rowid() should never return zero (or any negative values really) as we
	// insert into torrents and handle any errors accordingly right afterwards.
	if lastInsertId <= 0 {
		zap.L().Panic("last_insert_rowid() <= 0 (this should have never happened!)",
			zap.Int64("lastInsertId", lastInsertId))
	}

	for _, file := range files {
		_, err = tx.Exec("INSERT INTO files (torrent_id, size, path) VALUES (?, ?, ?);",
			lastInsertId, file.Size, file.Path,
		)
		if err != nil {
			return errors.Wrap(err, "tx.Exec (INSERT INTO files)")
		}
	}

	err = tx.Commit()
	if err != nil {
		return errors.Wrap(err, "tx.Commit")
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
	defer rows.Close()

	if !rows.Next() {
		return 0, fmt.Errorf("no rows returned from `SELECT MAX(ROWID)`")
	}

	var n *uint
	if err = rows.Scan(&n); err != nil {
		return 0, err
	}

	// If the database is empty (i.e. 0 entries in 'torrents') then the query will return nil.
	if n == nil {
		return 0, nil
	} else {
		return *n, nil
	}
}

func (db *sqlite3Database) QueryTorrents(
	query string,
	epoch int64,
	orderBy OrderingCriteria,
	ascending bool,
	limit uint,
	lastOrderedValue *float64,
	lastID *uint64,
) ([]TorrentMetadata, error) {
	if query == "" && orderBy == ByRelevance {
		return nil, fmt.Errorf("torrents cannot be ordered by relevance when the query is empty")
	}
	if (lastOrderedValue == nil) != (lastID == nil) {
		return nil, fmt.Errorf("lastOrderedValue and lastID should be supplied together, if supplied")
	}

	doJoin := query != ""
	firstPage := lastID == nil

	// executeTemplate is used to prepare the SQL query, WITH PLACEHOLDERS FOR USER INPUT.
	sqlQuery := executeTemplate(`
		SELECT id 
             , info_hash
			 , name
			 , total_size
			 , discovered_on
			 , (SELECT COUNT(*) FROM files WHERE torrents.id = files.torrent_id) AS n_files
	{{ if .DoJoin }}
			 , idx.rank
	{{ else }}
			 , 0
	{{ end }}
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
			  AND ( {{.OrderOn}}, id ) {{GTEorLTE .Ascending}} (?, ?) -- https://www.sqlite.org/rowvalue.html#row_value_comparisons
	{{ end }}
		ORDER BY {{.OrderOn}} {{AscOrDesc .Ascending}}, id {{AscOrDesc .Ascending}}
		LIMIT ?;	
	`, struct {
		DoJoin    bool
		FirstPage bool
		OrderOn   string
		Ascending bool
	}{
		DoJoin:    doJoin,
		FirstPage: firstPage,
		OrderOn:   orderOn(orderBy),
		Ascending: ascending,
	}, template.FuncMap{
		"GTEorLTE": func(ascending bool) string {
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
		queryArgs = append(queryArgs, lastOrderedValue)
		queryArgs = append(queryArgs, lastID)
	}
	queryArgs = append(queryArgs, limit)

	rows, err := db.conn.Query(sqlQuery, queryArgs...)
	defer closeRows(rows)
	if err != nil {
		return nil, errors.Wrap(err, "query error")
	}

	torrents := make([]TorrentMetadata, 0)
	for rows.Next() {
		var torrent TorrentMetadata
		err = rows.Scan(
			&torrent.ID,
			&torrent.InfoHash,
			&torrent.Name,
			&torrent.Size,
			&torrent.DiscoveredOn,
			&torrent.NFiles,
			&torrent.Relevance,
		)
		if err != nil {
			return nil, err
		}
		torrents = append(torrents, torrent)
	}

	return torrents, nil
}

func orderOn(orderBy OrderingCriteria) string {
	switch orderBy {
	case ByRelevance:
		return "idx.rank"

	case ByTotalSize:
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
	defer closeRows(rows)
	if err != nil {
		return nil, err
	}

	if !rows.Next() {
		return nil, nil
	}

	var tm TorrentMetadata
	if err = rows.Scan(&tm.InfoHash, &tm.Name, &tm.Size, &tm.DiscoveredOn, &tm.NFiles); err != nil {
		return nil, err
	}

	return &tm, nil
}

func (db *sqlite3Database) GetFiles(infoHash []byte) ([]File, error) {
	rows, err := db.conn.Query(
		"SELECT size, path FROM files, torrents WHERE files.torrent_id = torrents.id AND torrents.info_hash = ?;",
		infoHash)
	defer closeRows(rows)
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

	return files, nil
}

func (db *sqlite3Database) GetStatistics(from string, n uint) (*Statistics, error) {
	fromTime, gran, err := ParseISO8601(from)
	if err != nil {
		return nil, errors.Wrap(err, "parsing ISO8601 error")
	}

	var toTime time.Time
	var timef string // time format: https://www.sqlite.org/lang_datefunc.html

	switch gran {
	case Year:
		toTime = fromTime.AddDate(int(n), 0, 0)
		timef = "%Y"
	case Month:
		toTime = fromTime.AddDate(0, int(n), 0)
		timef = "%Y-%m"
	case Week:
		toTime = fromTime.AddDate(0, 0, int(n)*7)
		timef = "%Y-%W"
	case Day:
		toTime = fromTime.AddDate(0, 0, int(n))
		timef = "%Y-%m-%d"
	case Hour:
		toTime = fromTime.Add(time.Duration(n) * time.Hour)
		timef = "%Y-%m-%dT%H"
	}

	// TODO: make it faster!
	rows, err := db.conn.Query(fmt.Sprintf(`
			SELECT strftime('%s', discovered_on, 'unixepoch') AS dT
                 , sum(files.size) AS tS
                 , count(DISTINCT torrents.id) AS nD              
                 , count(DISTINCT files.id) AS nF
			FROM torrents, files
 			WHERE     torrents.id = files.torrent_id
                  AND discovered_on >= ?
                  AND discovered_on <= ?
			GROUP BY dt;`,
		timef),
		fromTime.Unix(), toTime.Unix())
	defer closeRows(rows)
	if err != nil {
		return nil, err
	}

	stats := NewStatistics()

	for rows.Next() {
		var dT string
		var tS, nD, nF uint64
		if err := rows.Scan(&dT, &tS, &nD, &nF); err != nil {
			if err := rows.Close(); err != nil {
				panic(err.Error())
			}
			return nil, err
		}
		stats.NDiscovered[dT] = nD
		stats.TotalSize[dT] = tS
		stats.NFiles[dT] = nF
	}

	return stats, nil
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
		return errors.Wrap(err, "sql.DB.Exec (PRAGMAs)")
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return errors.Wrap(err, "sql.DB.Begin")
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
		return errors.Wrap(err, "sql.Tx.Exec (v0)")
	}

	// Get the user_version:
	rows, err := tx.Query("PRAGMA user_version;")
	if err != nil {
		return errors.Wrap(err, "sql.Tx.Query (user_version)")
	}
	defer rows.Close()
	var userVersion int
	if !rows.Next() {
		return fmt.Errorf("sql.Rows.Next (user_version): PRAGMA user_version did not return any rows")
	}
	if err = rows.Scan(&userVersion); err != nil {
		return errors.Wrap(err, "sql.Rows.Scan (user_version)")
	}

	switch userVersion {
	case 0: // FROZEN.
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
			return errors.Wrap(err, "sql.Tx.Exec (v0 -> v1)")
		}
		fallthrough

	case 1: // FROZEN.
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
			return errors.Wrap(err, "sql.Tx.Exec (v1 -> v2)")
		}
		fallthrough

	case 2: // NOT FROZEN! (subject to change or complete removal)
		// Upgrade from user_version 2 to 3
		// Changes:
		//   * Created `torrents_idx` FTS5 virtual table.
		//
		//     See:
		//     * https://sqlite.org/fts5.html
		//     * https://sqlite.org/fts3.html
		//
		//   * Added `modified_on` column to the `torrents` table.
		zap.L().Warn("Updating database schema from 2 to 3... (this might take a while)")
		_, err = tx.Exec(`
			CREATE VIRTUAL TABLE torrents_idx USING fts5(name, content='torrents', content_rowid='id', tokenize="porter unicode61 separators ' !""#$%&''()*+,-./:;<=>?@[\]^_` + "`" + `{|}~'");
			
			-- Populate the index
			INSERT INTO torrents_idx(rowid, name) SELECT id, name FROM torrents;

			-- Triggers to keep the FTS index up to date.
			CREATE TRIGGER torrents_idx_ai_t AFTER INSERT ON torrents BEGIN
			  INSERT INTO torrents_idx(rowid, name) VALUES (new.id, new.name);
			END;
			CREATE TRIGGER torrents_idx_ad_t AFTER DELETE ON torrents BEGIN
			  INSERT INTO torrents_idx(torrents_idx, rowid, name) VALUES('delete', old.id, old.name);
			END;
			CREATE TRIGGER torrents_idx_au_t AFTER UPDATE ON torrents BEGIN
			  INSERT INTO torrents_idx(torrents_idx, rowid, name) VALUES('delete', old.id, old.name);
			  INSERT INTO torrents_idx(rowid, name) VALUES (new.id, new.name);
			END;

            -- Add column 'modified_on'
			-- BEWARE: code needs to be updated before January 1, 3000 (32503680000)!
			ALTER TABLE torrents ADD COLUMN modified_on INTEGER NOT NULL
				CHECK (modified_on >= discovered_on AND (updated_on IS NOT NULL OR modified_on >= updated_on))
				DEFAULT 32503680000
			;

			-- If 'modified_on' is not explicitly supplied, then it shall be set, by default, to
			-- 'discovered_on' right after the row is inserted to 'torrents'.
            --
			-- {WHEN expr} does NOT work for some reason (trigger doesn't get triggered), so we use
            --   AND NEW."modified_on" = 32503680000
            -- instead in the WHERE clause.
			CREATE TRIGGER "torrents_modified_on_default_t" AFTER INSERT ON "torrents" BEGIN
	          UPDATE "torrents" SET "modified_on" = NEW."discovered_on" WHERE "id" = NEW."id" AND NEW."modified_on" = 32503680000;
            END;

			-- Set 'modified_on' value of all rows to 'discovered_on' or 'updated_on', whichever is
            -- greater; beware that 'updated_on' can be NULL too.			
			UPDATE torrents SET modified_on = (SELECT MAX(discovered_on, IFNULL(updated_on, 0)));

			CREATE INDEX modified_on_index ON torrents (modified_on);

			PRAGMA user_version = 3;
		`)
		if err != nil {
			return errors.Wrap(err, "sql.Tx.Exec (v2 -> v3)")
		}
	}

	if err = tx.Commit(); err != nil {
		return errors.Wrap(err, "sql.Tx.Commit")
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

func closeRows(rows *sql.Rows) {
	if err := rows.Close(); err != nil {
		zap.L().Error("could not close row", zap.Error(err))
	}
}
