package persistence

import (
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
	QueryTorrents(
		query string,
		epoch int64,
		orderBy orderingCriteria,
		ascending bool,
		limit uint,
		lastOrderedValue *uint,
		lastID *uint,
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
	InfoHash     []byte
	Name         string
	Size         uint64
	DiscoveredOn int64
	NFiles       uint
}

func MakeDatabase(dbURL *url.URL, logger *zap.Logger) (Database, error) {
	if logger != nil {
		zap.ReplaceGlobals(logger)
	}

	switch dbURL.Scheme {
	case "sqlite3":
		return makeSqlite3Database(dbURL)

	case "postgresql":
		return nil, fmt.Errorf("postgresql is not yet supported!")

	case "mysql":
		return nil, fmt.Errorf("mysql is not yet supported!")

	default:
		return nil, fmt.Errorf("unknown URI scheme (database engine)!")
	}
}
