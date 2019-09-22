package persistence

import (
	"encoding/hex"
	"encoding/json"
	"go.uber.org/zap"
	"net/url"
	"strings"
	"time"

	"github.com/iwanbk/gobeanstalk"
	"github.com/pkg/errors"
)

func makeBeanstalkDatabase(url_ *url.URL) (Database, error) {
	s := new(beanstalkd)

	var err error
	s.bsQueue, err = gobeanstalk.Dial(url_.Hostname() + ":" + url_.Port())
	if err != nil {
		return nil, errors.Wrap(err, "Beanstalkd connection error")
	}

	tubeName := strings.TrimPrefix(url_.Path, "/")

	err = s.bsQueue.Use(tubeName)
	if err != nil {
		return nil, errors.Wrap(err, "Beanstalkd tube set error")
	}

	zap.L().Info(
		"Beanstalkd connection created",
		zap.String("host", url_.Hostname()),
		zap.String("port", url_.Port()),
		zap.String("tube", tubeName),
	)

	return s, nil
}

type beanstalkd struct {
	bsQueue *gobeanstalk.Conn
}

func (s *beanstalkd) Engine() databaseEngine {
	return Beanstalkd
}

func (s *beanstalkd) DoesTorrentExist(infoHash []byte) (bool, error) {
	// Always say that "No the torrent does not exist" because we do not have
	// a way to know if we have seen it before or not.
	return false, nil
}

func (s *beanstalkd) AddNewTorrent(infoHash []byte, name string, files []File) error {
	payloadJson, err := json.Marshal(SimpleTorrentSummary{
		InfoHash: hex.EncodeToString(infoHash),
		Name:     name,
		Files:    files,
	})

	if err != nil {
		return errors.Wrap(err, "DB engine beanstalkd encode error")
	}

	jobId, err := s.bsQueue.Put(payloadJson, 0, 0, 30*time.Second)

	if err != nil {
		return errors.Wrap(err, "DB engine beanstalkd Put() error")
	}

	zap.L().Debug("New item put into the queue", zap.Uint64("job_id", jobId))

	return nil
}

func (s *beanstalkd) Close() error {
	s.bsQueue.Quit()
	return nil
}

func (s *beanstalkd) GetNumberOfTorrents() (uint, error) {
	return 0, NotImplementedError
}

func (s *beanstalkd) QueryTorrents(
	query string,
	epoch int64,
	orderBy OrderingCriteria,
	ascending bool,
	limit uint,
	lastOrderedValue *float64,
	lastID *uint64,
) ([]TorrentMetadata, error) {
	return nil, NotImplementedError
}

func (s *beanstalkd) GetTorrent(infoHash []byte) (*TorrentMetadata, error) {
	return nil, NotImplementedError
}

func (s *beanstalkd) GetFiles(infoHash []byte) ([]File, error) {
	return nil, NotImplementedError
}

func (s *beanstalkd) GetStatistics(from string, n uint) (*Statistics, error) {
	return nil, NotImplementedError
}
