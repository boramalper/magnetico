package bittorrent

import (
	"net"
	"path"
	"time"

	"github.com/anacrolix/dht"
	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/storage"
	"github.com/Wessie/appdirs"
	"go.uber.org/zap"
)

type FileRequest struct {
	InfoHash []byte
	Path string
	Peers []torrent.Peer
}

type FileResult struct {
	// Request field is the original Request
	Request  *FileRequest
	FileData []byte
}

type FileSink struct {
	baseDownloadDir string
	client *torrent.Client
	drain chan FileResult
	terminated bool
	termination chan interface{}

	timeoutDuration time.Duration
}

// NewFileSink creates a new FileSink.
//
//   cAddr : client address
//   mlAddr: mainline DHT node address
func NewFileSink(cAddr, mlAddr string, timeoutDuration time.Duration) *FileSink {
	fs := new(FileSink)

	mlUDPAddr, err := net.ResolveUDPAddr("udp", mlAddr)
	if err != nil {
		zap.L().Fatal("Could NOT resolve UDP addr!", zap.Error(err))
		return nil
	}

	// Make sure to close the mlUDPConn before returning from this function in case of an error.
	mlUDPConn, err := net.ListenUDP("udp", mlUDPAddr)
	if err != nil {
		zap.L().Fatal("Could NOT listen UDP (file sink)!", zap.Error(err))
		return nil
	}

	fs.baseDownloadDir = path.Join(
		appdirs.UserCacheDir("magneticod", "", "", true),
		"downloads",
	)

	fs.client, err = torrent.NewClient(&torrent.Config{
		ListenAddr: cAddr,
		DisableTrackers: true,
		DHTConfig: dht.ServerConfig{
			Conn:       mlUDPConn,
			Passive:    true,
			NoSecurity: true,
		},
		DefaultStorage: storage.NewFileByInfoHash(fs.baseDownloadDir),
	})
	if err != nil {
		zap.L().Fatal("Leech could NOT create a new torrent client!", zap.Error(err))
		mlUDPConn.Close()
		return nil
	}

	fs.drain = make(chan FileResult)
	fs.termination = make(chan interface{})
	fs.timeoutDuration = timeoutDuration

	return fs
}

// peer field is optional and might be nil.
func (fs *FileSink) Sink(infoHash []byte, path string, peers []torrent.Peer) {
	if fs.terminated {
		zap.L().Panic("Trying to Sink() an already closed FileSink!")
	}
	go fs.awaitFile(&FileRequest{
		InfoHash: infoHash,
		Path: path,
		Peers: peers,
	})
}

func (fs *FileSink) Drain() <-chan FileResult {
	if fs.terminated {
		zap.L().Panic("Trying to Drain() an already closed FileSink!")
	}
	return fs.drain
}


func (fs *FileSink) Terminate() {
	fs.terminated = true
	close(fs.termination)
	fs.client.Close()
	close(fs.drain)
}


func (fs *FileSink) flush(result FileResult) {
	if !fs.terminated {
		fs.drain <- result
	}
}
