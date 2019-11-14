package persistence

import (
	"encoding/hex"
	"encoding/json"
	"net/url"
	"os"

	"github.com/pkg/errors"
)

func makeStdoutDatabase(_ *url.URL) (Database, error) {
	s := new(stdout)
	s.encoder = json.NewEncoder(os.Stdout)
	return s, nil
}

type stdout struct {
	WriteOnlyDatabase
	encoder *json.Encoder
}

func (s *stdout) Engine() databaseEngine {
	s.kind = Stdout
	return s.kind
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
	err := s.encoder.Encode(SimpleTorrentSummary{
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