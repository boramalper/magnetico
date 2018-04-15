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

func makePostgresDatabase(url_ string) (Database, error) {
	db := new(postgresDatabase)

	var err error
	db.conn, err = sql.Open("postgres", url_)
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

func (db *postgresDatabase) DoesTorrentExist(infoHash []byte) (bool, error) {
	rows, err := db.conn.Query("SELECT 1 FROM torrents WHERE info_hash = $1::BYTEA;", infoHash)
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

	var lastInsertId int64
	// we will insert the name as tsvector for easier searching and split on word boundaries
	err = tx.QueryRow(`
		INSERT INTO torrents (
			info_hash,
			name,
			total_size,
			discovered_on,
			search
		) VALUES ($1::BYTEA, $2::VARCHAR, $3, $4::TIMESTAMP, to_tsvector(regexp_replace(coalesce($2, ''), '[^\w]+', ' ', 'gi')))
		ON CONFLICT
		DO NOTHING
		RETURNING id;
	`, infoHash, name, totalSize, time.Now()).Scan(&lastInsertId)
	if err != nil {
		return fmt.Errorf("sql.Result.LastInsertId()!  %s", err.Error())
	}

	for _, file := range files {
		_, err = tx.Exec("INSERT INTO files (torrent_id, Size, path) VALUES ($1, $2, $3::VARCHAR);",
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
		WHERE info_hash = $1::BYTEA;`,
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
		WHERE torrent_id = $1;`,
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

func (db *postgresDatabase) setupDatabase() error {
	// utf8 will be ensured by locale of postgres user

	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("sql.DB.Begin: %s", err.Error())
	}
	defer tx.Rollback()

	// Create settings table first
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS settings (
			name	VARCHAR UNIQUE,
			value 	VARCHAR
		);
	`)

	if err != nil {
		return fmt.Errorf("sql.Tx.Exec (v0): %s", err.Error())
	}

	rows, err := tx.Query("SELECT value FROM settings WHERE name='SCHEMA_VERSION';")
	if err != nil {
		return fmt.Errorf("sql.Tx.Query (SCHEMA_VERSION): %s", err.Error())
	}

	var userVersion string
	if rows.Next() != true {
		// SCHEMA_VERSION does not exist
		zap.L().Error("sql.Rows.Next (SCHEMA_VERSION): SCHEMA_VERSION did not return any rows!")
		_, err = tx.Exec(`
			INSERT INTO settings(name, value) VALUES ('SCHEMA_VERSION', '0');`)
		rows, err = tx.Query("SELECT value FROM settings WHERE name='SCHEMA_VERSION';")
		if err != nil {
			return fmt.Errorf("sql.Tx.Query (SCHEMA_VERSION): %s", err.Error())
		}
		rows.Next()
	}

	if err = rows.Scan(&userVersion); err != nil {
		return fmt.Errorf("sql.Rows.Scan (SCHEMA_VERSION): %s", err.Error())
	}
	if err = rows.Close(); err != nil {
		return fmt.Errorf("sql.Rows.Close (SCHEMA_VERSION): %s", err.Error())
	}

	// at the moment we just have 2 versions (0=no db or 1=current db)
	switch userVersion {
	case "0":
		// initialise db
		// we use postgres fulltext search in the torrents table
		zap.L().Warn("Database was empty, initialising it")
		_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS torrents (
			id             	SERIAL PRIMARY KEY,
			info_hash      	BYTEA NOT NULL UNIQUE,
			name           	VARCHAR NOT NULL,
			total_size     	BIGINT NOT NULL CHECK(total_size > 0),
			discovered_on	TIMESTAMP NOT NULL,
			updated_on		TIMESTAMP,
			n_seeders		INTEGER CHECK ((updated_on IS NOT NULL AND n_seeders >= 0) OR (updated_on IS NULL AND n_seeders IS NULL)) DEFAULT NULL,
			n_leechers		INTEGER CHECK ((updated_on IS NOT NULL AND n_leechers >= 0) OR (updated_on IS NULL AND n_leechers IS NULL)) DEFAULT NULL,
			search			tsvector
		);
		CREATE TABLE IF NOT EXISTS files (
			id          	SERIAL PRIMARY KEY,
			torrent_id  	INTEGER REFERENCES torrents ON DELETE CASCADE ON UPDATE RESTRICT,
			size        	BIGINT NOT NULL,
			path        	VARCHAR NOT NULL,
			is_readme		BOOLEAN DEFAULT NULL,
			content		TEXT CHECK ((content IS NULL AND is_readme IS FALSE) OR (content IS NOT NULL AND is_readme IS TRUE) OR (content IS NULL AND is_readme IS NULL)) DEFAULT NULL
		);
		CREATE UNIQUE INDEX readme_index on files(torrent_id, is_readme);
		CREATE INDEX torrents_idx ON torrents USING gin(search);
		UPDATE settings SET value = '1' WHERE name = 'SCHEMA_VERSION';
		`)
		if err != nil {
			return fmt.Errorf("sql.Tx.Exec (v0 -> v1): %s", err.Error())
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("sql.Tx.Commit: %s", err.Error())
	}

	return nil
}