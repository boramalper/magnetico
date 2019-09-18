package persistence

import (
	"encoding/hex"
	"encoding/json"
	"net/url"
	"os"

	"github.com/pkg/errors"
)

type out struct {
	InfoHash string `json:"infoHash"`
	Name     string `json:"name"`
	Files    []File `json:"files"`
}

var notSupportedError = errors.New("This dummy database engine (\"stdout\") does not support any sort of queries")

func makeStdoutDatabase(_ *url.URL) (Database, error) {
	s := new(stdout)
	s.encoder = json.NewEncoder(os.Stdout)
	return s, nil
}

type stdout struct {
	encoder *json.Encoder
}

func (s *stdout) Engine() databaseEngine {
	return Stdout
}

func (s *stdout) DoesTorrentExist(infoHash []byte) (bool, error) {
	// Always say that "No the torrent does not exist" because we do not have
	// a way to know if we have seen it before or not.
	// TODO:
	// A possible improvement would be using bloom filters (with low false positive
	// probabilities) to apply some reasonable filtering.
	return false, nil
}

func (s *stdout) AddNewTorrent(infoHash []byte, name string, files []File) error {
	err := s.encoder.Encode(out{
		InfoHash: hex.EncodeToString(infoHash),
		Name:     name,
		Files:    files,
	})
	if err != nil {
		return errors.Wrap(err, "DB engine stdout encode error")
	}

	return nil
}

func (s *stdout) Close() error {
	return os.Stdout.Sync()
}

func (s *stdout) GetNumberOfTorrents() (uint, error) {
	return 0, notSupportedError
}

func (s *stdout) QueryTorrents(
	query string,
	epoch int64,
	orderBy OrderingCriteria,
	ascending bool,
	limit uint,
	lastOrderedValue *float64,
	lastID *uint64,
) ([]TorrentMetadata, error) {
	return nil, notSupportedError
}

func (s *stdout) GetTorrent(infoHash []byte) (*TorrentMetadata, error) {
	return nil, notSupportedError
}

func (s *stdout) GetFiles(infoHash []byte) ([]File, error) {
	return nil, notSupportedError
}

func (s *stdout) GetStatistics(from string, n uint) (*Statistics, error) {
	return nil, notSupportedError
}
