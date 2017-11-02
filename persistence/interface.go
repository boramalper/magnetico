package persistence

import (
	"fmt"
	"go.uber.org/zap"
	"net/url"
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
	QueryTorrents(query string, orderBy orderingCriteria, ord order, n uint, when presence, timePoint int64) ([]TorrentMetadata, error)
	// GetTorrents returns the TorrentExtMetadata for the torrent of the given InfoHash. Might return
	// nil, nil if the torrent does not exist in the database.
	GetTorrent(infoHash []byte) (*TorrentMetadata, error)
	GetFiles(infoHash []byte) ([]File, error)
	GetStatistics(from ISO8601, period uint) (*Statistics, error)
}

type orderingCriteria uint8

const (
	BY_RELEVANCE orderingCriteria = 1
	BY_SIZE                       = 2
	BY_DISCOVERED_ON              = 3
	BY_N_FILES                    = 4
)

type order uint8

const (
	ASCENDING  order = 1
	DESCENDING       = 2
)

type presence uint8

const (
	BEFORE presence = 1
	AFTER           = 2
)

type statisticsGranularity uint8
type ISO8601 string

const (
	HOURLY_STATISTICS statisticsGranularity = 1
	DAILY_STATISTICS                        = 2
	WEEKLY_STATISTICS                       = 3
	MONTHLY_STATISTICS                      = 4
	YEARLY_STATISTICS                       = 5
)

type databaseEngine uint8

const (
	SQLITE3_ENGINE databaseEngine = 1
)

type Statistics struct {
	Granularity statisticsGranularity
	From        ISO8601
	Period      uint

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
	Size         uint64
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
	}

	return nil, fmt.Errorf("unknown URI scheme (database engine)!")
}
