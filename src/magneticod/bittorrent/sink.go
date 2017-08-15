package bittorrent

import (
	"go.uber.org/zap"
	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"

	"magneticod/dht/mainline"
	"net"
)


type Metadata struct {
	InfoHash []byte
	// Name should be thought of "Title" of the torrent. For single-file torrents, it is the name
	// of the file, and for multi-file torrents, it is the name of the root directory.
	Name string
	TotalSize uint64
	DiscoveredOn int64
	// Files must be populated for both single-file and multi-file torrents!
	Files []metainfo.FileInfo
}


type MetadataSink struct {
	activeInfoHashes []metainfo.Hash
	client *torrent.Client
	drain chan Metadata
	terminated bool
	termination chan interface{}
}


func NewMetadataSink(laddr net.TCPAddr) *MetadataSink {
	ms := new(MetadataSink)
	var err error
	ms.client, err = torrent.NewClient(&torrent.Config{
		ListenAddr: laddr.String(),
		DisableTrackers: true,
		DisablePEX: true,
		// TODO: Should we disable DHT to force the client to use the peers we supplied only, or not?
		NoDHT: true,
		PreferNoEncryption: true,

	})
	if err != nil {
		zap.L().Fatal("Fetcher could NOT create a new torrent client!", zap.Error(err))
	}
	ms.drain = make(chan Metadata)
	ms.termination = make(chan interface{})
	return ms
}


func (ms *MetadataSink) Sink(res mainline.TrawlingResult) {
	if ms.terminated {
		zap.L().Panic("Trying to Sink() an already closed MetadataSink!")
	}

	ms.activeInfoHashes = append(ms.activeInfoHashes, res.InfoHash)
	go ms.awaitMetadata(res.InfoHash, res.Peer)
}


func (ms *MetadataSink) Drain() <-chan Metadata {
	if ms.terminated {
		zap.L().Panic("Trying to Drain() an already closed MetadataSink!")
	}

	return ms.drain
}


func (ms *MetadataSink) Terminate() {
	ms.terminated = true
	close(ms.termination)
	ms.client.Close()
	close(ms.drain)
}


func (ms *MetadataSink) flush(metadata Metadata) {
	if !ms.terminated {
		ms.drain <- metadata
	}
}
