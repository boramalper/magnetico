package persistence

import (
	"fmt"
	"regexp"
	"net/url"
)

type Database interface {
	Engine() databaseEngine
	DoesTorrentExist(infoHash []byte) (bool, error)
	// GiveAnIncompleteTorrentByInfoHash returns (*gives*) an incomplete -i.e. one that doesn't have
	// readme downloaded yet- torrent from the database.
	// GiveAnIncompleteTorrent might return a nil slice for infoHash, a nil string, and a nil err,
	// meaning that no incomplete torrent could be found in the database (congrats!).
	GiveAnIncompleteTorrent(pathRegex *regexp.Regexp, maxSize uint) (infoHash []byte, path string, err error)
	GiveAStaleTorrent() (infoHash []byte, err error)
	AddNewTorrent(infoHash []byte, name string, files []File) error
	AddReadme(infoHash []byte, path string, content string) error
	Close() error

	// GetNumberOfTorrents returns the number of torrents saved in the database. Might be an
	// approximation.
	GetNumberOfTorrents() (uint, error)
	NewestTorrents(n uint) ([]TorrentMetadata, error)
	SearchTorrents(query string, orderBy orderingCriteria, descending bool, mustHaveReadme bool) ([]TorrentMetadata, error)
	// GetTorrents returns the TorrentExtMetadata for the torrent of the given infoHash. Might return
	// nil, nil if the torrent does not exist in the database.
	GetTorrent(infoHash []byte) (*TorrentMetadata, error)
	GetFiles(infoHash []byte) ([]File, error)
	GetReadme(infoHash []byte) (string, error)
	GetStatistics(from ISO8601, period uint) (*Statistics, error)
}

type orderingCriteria uint8

const (
	BY_NAME orderingCriteria         = 1
	BY_SIZE                          = 2
	BY_DISCOVERED_ON                 = 3
	BY_N_FILES                       = 4
	BY_N_SEEDERS                     = 5
	BY_N_LEECHERS                    = 6
	BY_UPDATED_ON                    = 7
	BY_N_SEEDERS_TO_N_LEECHERS_RATIO = 8
	BY_N_SEEDERS_PLUS_N_LEECHERS     = 9
)


type statisticsGranularity uint8
type ISO8601 string

const (
	MINUTELY_STATISTICS statisticsGranularity = 1
	HOURLY_STATISTICS  = 2
	DAILY_STATISTICS   = 3
	WEEKLY_STATISTICS  = 4
	MONTHLY_STATISTICS = 5
	YEARLY_STATISTICS  = 6
)

type databaseEngine uint8

const (
	SQLITE3_ENGINE databaseEngine = 1
)

type Statistics struct {
	Granularity statisticsGranularity
	From ISO8601
	Period uint

	// All these slices below have the exact length equal to the Period.
	NTorrentsDiscovered []uint
	NFilesDiscovered    []uint
	NReadmesDownloaded  []uint
	NTorrentsUpdated    []uint
}

type File struct {
	Size int64
	Path string
}

type TorrentMetadata struct {
	infoHash     []byte
	name         string
	size         uint64
	discoveredOn int64
	hasReadme    bool
	nFiles       uint
	// values below 0 indicates that no data is available:
	nSeeders     int
	nLeechers    int
	updatedOn    int
}

func MakeDatabase(rawURL string) (Database, error) {
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
	}

	return nil, fmt.Errorf("unknown URI scheme (database engine)!")
}
