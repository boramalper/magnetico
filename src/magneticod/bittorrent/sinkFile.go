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


type File struct{
	torrentInfoHash []byte
	path string
	data []byte
}


type FileSink struct {
	client *torrent.Client
	drain chan File
	terminated bool
	termination chan interface{}

	timeout time.Duration
}

// NewFileSink creates a new FileSink.
//
//   cAddr : client address
//   mlAddr: mainline DHT node address
func NewFileSink(cAddr, mlAddr string, timeout time.Duration) *FileSink {
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

	fs.client, err = torrent.NewClient(&torrent.Config{
		ListenAddr: cAddr,
		DisableTrackers: true,
		DHTConfig: dht.ServerConfig{
			Conn:       mlUDPConn,
			Passive:    true,
			NoSecurity: true,
		},
		DefaultStorage: storage.NewFileByInfoHash(path.Join(
			appdirs.UserCacheDir("magneticod", "", "", true),
			"downloads",
		)),
	})
	if err != nil {
		zap.L().Fatal("Leech could NOT create a new torrent client!", zap.Error(err))
		mlUDPConn.Close()
		return nil
	}

	fs.drain = make(chan File)
	fs.termination = make(chan interface{})
	fs.timeout = timeout

	return fs
}


// peer might be nil
func (fs *FileSink) Sink(infoHash []byte, filePath string, peer *torrent.Peer) {
	go fs.awaitFile(infoHash, filePath, peer)
}


func (fs *FileSink) Drain() <-chan File {
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


func (fs *FileSink) flush(result File) {
	if !fs.terminated {
		fs.drain <- result
	}
}
