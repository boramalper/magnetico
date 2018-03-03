package persistence

import (
	"fmt"
	"net/url"
	"time"

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
	// QueryTorrents returns @n torrents
	// * that are discovered before the @timePoint if @isAfter is false, else that are
	//   discovered after the @timePoint,
	// * that match the @query if it's not empty,
	// ordered by the @orderBy in ascending order if @isDescending is false, else in descending
	// order.
	QueryTorrents(query string, discoveredOnBefore int64, orderBy orderingCriteria, ascending bool, page uint, pageSize uint) ([]TorrentMetadata, error)
	// GetTorrents returns the TorrentExtMetadata for the torrent of the given InfoHash. Might return
	// nil, nil if the torrent does not exist in the database.
	GetTorrent(infoHash []byte) (*TorrentMetadata, error)
	GetFiles(infoHash []byte) ([]File, error)
	GetStatistics(n uint, granularity Granularity, to time.Time) (*Statistics, error)
}

type orderingCriteria uint8
const (
	ByRelevance orderingCriteria = iota
	BySize
	ByDiscoveredOn
	ByNFiles
)

type Granularity uint8
const (
	Yearly Granularity = iota
	Monthly
	Weekly
	Daily
	Hourly
)

type databaseEngine uint8
const (
	Sqlite3 databaseEngine = 1
)

type Statistics struct {
	N uint

	// All these slices below have the exact length equal to the Period.
	NTorrentsDiscovered []uint
	NFilesDiscovered    []uint
}

type File struct {
	Size int64
	Path string
}

type TorrentMetadata struct {
	InfoHash     []byte
	Name         string
	TotalSize    uint64
	DiscoveredOn int64
	NFiles       uint
}

func MakeDatabase(rawURL string, enableFTS bool, logger *zap.Logger) (Database, error) {
	if logger != nil {
		zap.ReplaceGlobals(logger)
	}

	url_, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	switch url_.Scheme {
	case "sqlite3":
		return makeSqlite3Database(url_, enableFTS)

	case "postgresql":
		return nil, fmt.Errorf("postgresql is not yet supported!")

	case "mysql":
		return nil, fmt.Errorf("mysql is not yet supported!")

	default:
		return nil, fmt.Errorf("unknown URI scheme (database engine)!")
	}
}
