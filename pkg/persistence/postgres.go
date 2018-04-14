package persistence

import (
	"database/sql"
	"time"
	"fmt"
	"go.uber.org/zap"

	_ "github.com/lib/pq"
)

type postgresDatabase struct {
	conn *sql.DB
}

func (db *postgresDatabase) DoesTorrentExist(infoHash []byte) (bool, error) {
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

func (db *postgresDatabase) AddNewTorrent(infoHash []byte, name string, files []File) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	var totalSize int64
	for _, files := range files {
		totalSize += files.Size
	}

	if totalSize == 0 {
		return nil
	}

	res, err := tx.Exec(`
		INSERT INTO torrents (
			info_hash,
			name,
			total_size,
			discovered_on
		) VALUES (?, ?, ?, ?)
		ON CONFLICT
		DO NOTHING;
	`, infoHash, name, totalSize, time.Now().Unix())
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

func (db *postgresDatabase) Close() error {
	return db.conn.Close()
}

func (db *postgresDatabase) GetNumberOfTorrents() (uint, error) {
	rows, err := db.conn.Query("SELECT MAX(ctid) FROM torrents;")
	if err != nil {
		return 0, err
	}

	if rows.Next() != true {
		fmt.Errorf("No rows returned from `SELECT MAX(ctid)`!")
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

func (db *postgresDatabase) QueryTorrents(query string, discoveredOnBefore int64, orderBy orderingCriteria, ascending bool, page uint, pageSize uint) ([]TorrentMetadata, error) {
	if query == "" && orderBy == ByRelevance {
		return nil, fmt.Errorf("torrents cannot be ordered by \"relevance\" when the query is empty")
	}

	// TODO

	return nil, nil
}

func (db *postgresDatabase) GetTorrent(infoHash []byte) (*TorrentMetadata, error) {
	rows, err := db.conn.Query(
		`SELECT 
			info_hash,
			name,
			total_size,
			discovered_on,
			(SELECT COUNT(1) FROM files WHERE torrent_id = torrents.id) AS n_files
		FROM torrents
		WHERE info_hash = ?;`,
		infoHash,
	)
	if err != nil {
		return nil, err
	}

	if rows.Next() != true {
		zap.L().Warn("torrent not found")
		return nil, nil
	}

	var tm TorrentMetadata
	rows.Scan(&tm.InfoHash, &tm.Name, &tm.TotalSize, &tm.DiscoveredOn, &tm.NFiles)
	if err = rows.Close(); err != nil {
		return nil, err
	}

	return &tm, nil
}

func (db *postgresDatabase) GetFiles(infoHash []byte) ([]File, error) {
	rows, err := db.conn.Query(`
		SELECT size, path
		FROM files
		WHERE torrent_id = ?;`,
		infoHash)
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

func (db *postgresDatabase) GetStatistics(n uint, granularity Granularity, to time.Time) (*Statistics, error) {
	// TODO
	return nil, nil
}

func (db *postgresDatabase) GetNewestTorrents(amount int, since int64) ([]TorrentMetadata, error) {
	return nil, nil
}

func (db *postgresDatabase) Engine() databaseEngine {
	return Postgres
}