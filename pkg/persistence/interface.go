package persistence

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
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
		orderBy OrderingCriteria,
		ascending bool,
		limit uint,
		lastOrderedValue *float64,
		lastID *uint64,
	) ([]TorrentMetadata, error)
	// GetTorrents returns the TorrentExtMetadata for the torrent of the given InfoHash. Will return
	// nil, nil if the torrent does not exist in the database.
	GetTorrent(infoHash []byte) (*TorrentMetadata, error)
	GetFiles(infoHash []byte) ([]File, error)
	GetStatistics(from string, n uint) (*Statistics, error)
}

//implements only a subset (the read functions) of Database
type WriteOnlyDatabase struct{kind databaseEngine}
func (wod *WriteOnlyDatabase) WriteOnlyError() error{
	return errors.New(`This dummy storage engine ("`+wod.kind.String()+`") is write-only`)
}
func (wod *WriteOnlyDatabase) GetTorrent(infoHash []byte) (*TorrentMetadata, error) {
	return nil, wod.WriteOnlyError()
}
func (wod *WriteOnlyDatabase) GetFiles(infoHash []byte) ([]File, error) {
	return nil, wod.WriteOnlyError()
}
func (wod *WriteOnlyDatabase) GetStatistics(from string, n uint) (*Statistics, error) {
	return nil, wod.WriteOnlyError()
}
func (wod *WriteOnlyDatabase) GetNumberOfTorrents() (uint, error) {
	return 0, wod.WriteOnlyError()
}
func (wod *WriteOnlyDatabase) QueryTorrents(
	query string,
	epoch int64,
	orderBy OrderingCriteria,
	ascending bool,
	limit uint,
	lastOrderedValue *float64,
	lastID *uint64,
) ([]TorrentMetadata, error) {
	return nil, wod.WriteOnlyError()
}

type OrderingCriteria uint8

const (
	ByRelevance OrderingCriteria = iota
	ByTotalSize
	ByDiscoveredOn
	ByNFiles
	ByNSeeders
	ByNLeechers
	ByUpdatedOn
)

// TODO: search `swtich (orderBy)` and see if all cases are covered all the time

type databaseEngine uint8
func( dbe databaseEngine) String() string{
	switch dbe{
	case Sqlite3:{return "Sqlite3"}
	case Stdout:{return "Stdout"}
	default:
		return "unnamed"
	}
}

const (
	Sqlite3    databaseEngine = 1
	Stdout     databaseEngine = 3
)

type Statistics struct {
	NDiscovered map[string]uint64 `json:"nDiscovered"`
	NFiles      map[string]uint64 `json:"nFiles"`
	TotalSize   map[string]uint64 `json:"totalSize"`

	// All these slices below have the exact length equal to the Period.
	//NDiscovered []uint64  `json:"nDiscovered"`

}

type File struct {
	Size int64  `json:"size"`
	Path string `json:"path"`
}

type TorrentMetadata struct {
	ID           uint64  `json:"id"`
	InfoHash     []byte  `json:"infoHash"` // marshalled differently
	Name         string  `json:"name"`
	Size         uint64  `json:"size"`
	DiscoveredOn int64   `json:"discoveredOn"`
	NFiles       uint    `json:"nFiles"`
	Relevance    float64 `json:"relevance"`
}

type SimpleTorrentSummary struct {
	InfoHash string `json:"infoHash"`
	Name     string `json:"name"`
	Files    []File `json:"files"`
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
		return nil, errors.Wrap(err, "url.Parse")
	}

	switch url_.Scheme {
	case "sqlite3":
		return makeSqlite3Database(url_)

	case "stdout":
		return makeStdoutDatabase(url_)

	case "postgresql":
		return nil, fmt.Errorf("postgresql is not yet supported")

	case "mysql":
		return nil, fmt.Errorf("mysql is not yet supported")

	default:
		return nil, fmt.Errorf("unknown URI scheme: `%s`", url_.Scheme)
	}
}

func NewStatistics() (s *Statistics) {
	s = new(Statistics)
	s.NDiscovered = make(map[string]uint64)
	s.NFiles = make(map[string]uint64)
	s.TotalSize = make(map[string]uint64)
	return
}
