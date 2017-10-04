package bittorrent

import (
	"go.uber.org/zap"
	"github.com/anacrolix/torrent"

	"magneticod/dht/mainline"
	"persistence"
)


type Metadata struct {
	InfoHash []byte
	// Name should be thought of "Title" of the torrent. For single-file torrents, it is the name
	// of the file, and for multi-file torrents, it is the name of the root directory.
	Name string
	TotalSize uint64
	DiscoveredOn int64
	// Files must be populated for both single-file and multi-file torrents!
	Files []persistence.File
	// Peers is the list of the "active" peers at the time of fetching metadata. Currently, it's
	// always nil as anacrolix/torrent does not support returning list of peers for a given torrent,
	// but in the future, this information can be useful for the CompletingCoordinator which can use
	// those Peers to download the README file (if any found).
	Peers []torrent.Peer

}


type MetadataSink struct {
	client *torrent.Client
	drain chan Metadata
	terminated bool
	termination chan interface{}
}


func NewMetadataSink(laddr string) *MetadataSink {
	ms := new(MetadataSink)
	var err error
	ms.client, err = torrent.NewClient(&torrent.Config{
		ListenAddr: laddr,
		DisableTrackers: true,
		DisablePEX: true,
		// TODO: Should we disable DHT to force the client to use the peers we supplied only, or not?
		NoDHT: true,
		Seed: false,


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


func (ms *MetadataSink) flush(result Metadata) {
	if !ms.terminated {
		ms.drain <- result
	}
}
