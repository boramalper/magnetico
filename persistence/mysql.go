package persistence

import (
	"database/sql"
	"fmt"
	"net/url"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
	"math"
	"strings"
)

type mySQLDatabase struct {
	conn *sql.DB
}

func (db *mySQLDatabase) Engine() databaseEngine {
	return MYSQL_ENGINE
}

func makeMySQLDatabase(url_ *url.URL, enableFTS bool) (Database, error) {
	db := new(mySQLDatabase)

	var err error
	db.conn, err = sql.Open("mysql", strings.Replace(url_.String(), "mysql://", "", -1))
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %s", err.Error())
	}

	// > Open may just validate its arguments without creating a connection to the database. To
	// > verify that the data source Name is valid, call Ping.

	if err = db.conn.Ping(); err != nil {
		return nil, fmt.Errorf("sql.DB.Ping: %s", err.Error())
	}

	if err := db.setupDatabase(); err != nil {
		return nil, fmt.Errorf("setupDatabase: %s", err.Error())
	}

	return db, nil
}

func (db *mySQLDatabase) DoesTorrentExist(infoHash []byte) (bool, error) {
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

func (db *mySQLDatabase) AddNewTorrent(infoHash []byte, name string, files []File) error {
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
		return err
	} else if exists {
		return nil
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

	// This is a workaround for a bug: the database will not accept total_size to be zero.
	if total_size == 0 {
		return nil
	}

	res, err := tx.Exec(`
		INSERT INTO torrents (
			info_hash,
			name,
			total_size,
			discovered_on
		) VALUES (?, ?, ?, ?);
	`, infoHash, name, total_size, time.Now().Unix())
	if err != nil {
		return err
	}

	var lastInsertId int64
	if lastInsertId, err = res.LastInsertId(); err != nil {
		return fmt.Errorf("sql.Result.LastInsertId()!  %s", err.Error())
	}

	for _, file := range files {
		_, err = tx.Exec("INSERT INTO files (torrent_id, Size, path) VALUES (?, ?, ?);",
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

func (db *mySQLDatabase) Close() error {
	return db.conn.Close()
}

func (db *mySQLDatabase) GetNumberOfTorrents() (uint, error) {
	// COUNT(1) is much more inefficient since it scans the whole table, so use MAX(ROWID)
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

func (db *mySQLDatabase) QueryTorrents(query string, orderBy orderingCriteria, ord order, n uint, when presence, timePoint int64) ([]TorrentMetadata, error) {
	if query == "" && orderBy == BY_RELEVANCE {
		return nil, fmt.Errorf("torrents cannot be ordered by \"relevance\" when the query is empty")
	}

	if timePoint == 0 && when == BEFORE {
		return nil, fmt.Errorf("nothing can come \"before\" time 0")
	}

	if timePoint == math.MaxInt64 && when == AFTER {
		return nil, fmt.Errorf("nothing can come \"after\" time %d", math.MaxInt64)
	}

	// TODO

	return nil, nil
}

func (db *mySQLDatabase) GetTorrent(infoHash []byte) (*TorrentMetadata, error) {
	rows, err := db.conn.Query(
		`SELECT
			info_hash,
			name,
			total_size,
			discovered_on,
			(SELECT COUNT(1) FROM files WHERE torrent_id = torrents.id) AS n_files
		FROM torrents
		WHERE info_hash = ?`,
		infoHash,
	)
	if err != nil {
		return nil, err
	}

	if rows.Next() != true {
		zap.L().Warn("torrent not found amk")
		return nil, nil
	}

	var tm TorrentMetadata
	rows.Scan(&tm.InfoHash, &tm.Name, &tm.Size, &tm.DiscoveredOn, &tm.NFiles)
	if err = rows.Close(); err != nil {
		return nil, err
	}

	return &tm, nil
}

func (db *mySQLDatabase) GetFiles(infoHash []byte) ([]File, error) {
	rows, err := db.conn.Query("SELECT size, path FROM files WHERE torrent_id = ?;", infoHash)
	if err != nil {
		return nil, err
	}

	var files []File
	for rows.Next() {
		var file File
		rows.Scan(&file.Size, &file.Path)
		files = append(files, file)
	}

	return files, nil
}

func (db *mySQLDatabase) GetStatistics(from ISO8601, period uint) (*Statistics, error) {
	// TODO
	return nil, nil
}

func (db *mySQLDatabase) commitQueuedTorrents() error {
	// TODO
	return nil
}

func (db *mySQLDatabase) setupDatabase() error {

	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("sql.DB.Begin: %s", err.Error())
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
		  id INT(11) NOT NULL AUTO_INCREMENT,
		  info_hash TINYBLOB NOT NULL,
		  name TEXT CHARACTER SET utf8 NOT NULL,
		  total_size BIGINT NOT NULL,
		  discovered_on INT(11) NOT NULL,
		  updated_on INT(11) DEFAULT NULL,
		  n_seeders INT(11) DEFAULT NULL,
		  n_leechers INT(11) DEFAULT NULL,
		  PRIMARY KEY (id) USING BTREE
		) ENGINE=InnoDB DEFAULT CHARSET=latin1;
	`)

	if err != nil {
		return fmt.Errorf("sql.Tx.Exec (v0): %s", err.Error())
	}
	_, err = tx.Exec(`
		 CREATE TABLE IF NOT EXISTS files (
				  id INT(11) NOT NULL AUTO_INCREMENT,
				  torrent_id INT(11) NOT NULL,
				  size BIGINT NOT NULL,
				  path TEXT CHARACTER SET utf8 NOT NULL,
				  is_readme INT(11) DEFAULT NULL,
				  content TEXT,
				  PRIMARY KEY (id),
				  UNIQUE KEY readme_index (torrent_id,is_readme),
				  KEY torrentid_id (torrent_id),
				  FOREIGN KEY (torrent_id) REFERENCES torrents (id) ON DELETE CASCADE
				) ENGINE=InnoDB DEFAULT CHARSET=latin1;
 	`)

	if err != nil {
		return fmt.Errorf("sql.Tx.Exec (v0): %s", err.Error())
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("sql.Tx.Commit: %s", err.Error())
	}

	return nil
}
