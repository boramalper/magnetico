package persistence

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"

	"go.uber.org/zap"
)

type Database interface {
	Engine() databaseEngine
	DoesTorrentExist(infoHash []byte) (bool, error)
	AddNewTorrent(infoHash []byte, name string, files []File) error
	Close() error

	// GetNumberOfTorrents returns the number of torrents saved in the database. Might be an
	// approximation.
	GetNumberOfTorrents() (uint, error)
	// QueryTorrents returns @pageSize amount of torrents,
	// * that are discovered before @discoveredOnBefore
	// * that match the @query if it's not empty, else all torrents
	// * ordered by the @orderBy in ascending order if @ascending is true, else in descending order
	// after skipping (@page * @pageSize) torrents that also fits the criteria above.
	//
	// On error, returns (nil, error), otherwise a non-nil slice of TorrentMetadata and nil.
	QueryTorrents(
		query string,
		epoch int64,
		orderBy orderingCriteria,
		ascending bool,
		limit uint,
		lastOrderedValue *float64,
		lastID *uint64,
	) ([]TorrentMetadata, error)
	// GetTorrents returns the TorrentExtMetadata for the torrent of the given InfoHash. Will return
	// nil, nil if the torrent does not exist in the database.
	GetTorrent(infoHash []byte) (*TorrentMetadata, error)
	GetFiles(infoHash []byte) ([]File, error)
	GetStatistics(n uint, to string) (*Statistics, error)
}

type orderingCriteria uint8
const (
	ByRelevance orderingCriteria = iota
	BySize
	ByDiscoveredOn
	ByNFiles
	ByNSeeders
	ByNLeechers
)

type databaseEngine uint8
const (
	Sqlite3 databaseEngine = 1
)

type Statistics struct {
	N uint64

	// All these slices below have the exact length equal to the Period.
	NTorrents []uint64
	NFiles    []uint64
	TotalSize []uint64
}

type File struct {
	Size int64
	Path string
}

type TorrentMetadata struct {
	ID           uint64   `json:"id"`
	InfoHash     []byte   `json:"infoHash"`  // marshalled differently
	Name         string   `json:"name"`
	Size         uint64   `json:"size"`
	DiscoveredOn int64    `json:"discoveredOn"`
	NFiles       uint     `json:"nFiles"`
	Relevance    float64  `json:"relevance"`
}

func (tm *TorrentMetadata) MarshalJSON() ([]byte, error) {
	type Alias TorrentMetadata
	return json.Marshal(&struct {
		InfoHash string `json:"infoHash"`
		*Alias
	}{
		InfoHash: hex.EncodeToString(tm.InfoHash),
		Alias:    (*Alias)(tm),
	})
}

func MakeDatabase(rawURL string, logger *zap.Logger) (Database, error) {
	if logger != nil {
		zap.ReplaceGlobals(logger)
	}

	url_, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	switch url_.Scheme {
	case "sqlite3":
		return makeSqlite3Database(url_)

	case "postgresql":
		return nil, fmt.Errorf("postgresql is not yet supported!")

	case "mysql":
		return nil, fmt.Errorf("mysql is not yet supported!")

	default:
		return nil, fmt.Errorf("unknown URI scheme (database engine)!")
	}
}
