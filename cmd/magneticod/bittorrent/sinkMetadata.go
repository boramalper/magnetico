package bittorrent

import (
	"crypto/rand"
	"fmt"
	"net"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/izolight/magnetico/cmd/magneticod/dht/mainline"
	"github.com/izolight/magnetico/pkg/persistence"
)

type Metadata struct {
	InfoHash []byte
	// Name should be thought of "Title" of the torrent. For single-file torrents, it is the name
	// of the file, and for multi-file torrents, it is the name of the root directory.
	Name         string
	TotalSize    uint64
	DiscoveredOn int64
	// Files must be populated for both single-file and multi-file torrents!
	Files []persistence.File
}

type Peer struct {
	Addr *net.TCPAddr
}

type MetadataSink struct {
	clientID    []byte
	deadline    time.Duration
	drain       chan Metadata
	incomingInfoHashes map[[20]byte]struct{}
	terminated  bool
	termination chan interface{}
}

func NewMetadataSink(deadline time.Duration) *MetadataSink {
	ms := new(MetadataSink)

	ms.clientID = make([]byte, 20)
	_, err := rand.Read(ms.clientID)
	if err != nil {
		zap.L().Panic("sinkMetadata couldn't read 20 random bytes for client ID!", zap.Error(err))
	}
	ms.deadline = deadline
	ms.drain = make(chan Metadata)
	ms.incomingInfoHashes = make(map[[20]byte]struct{})
	ms.termination = make(chan interface{})
	return ms
}

func (ms *MetadataSink) Sink(res mainline.TrawlingResult) {
	if ms.terminated {
		zap.L().Panic("Trying to Sink() an already closed MetadataSink!")
	}

	if _, exists := ms.incomingInfoHashes[res.InfoHash]; exists {
		return
	}
	// BEWARE!
	// Although not crucial, the assumption is that MetadataSink.Sink() will be called by only one
	// goroutine (i.e. it's not thread-safe), lest there might be a race condition between where we
	// check whether res.infoHash exists in the ms.incomingInfoHashes, and where we add the infoHash
	// to the incomingInfoHashes at the end of this function.

	IPs := res.PeerIP.String()
	var rhostport string
	if IPs == "<nil>" {
		zap.L().Debug("MetadataSink.Sink: Peer IP is nil!")
		return
	} else if IPs[0] == '?' {
		zap.L().Debug("MetadataSink.Sink: Peer IP is invalid!")
		return
	} else if strings.ContainsRune(IPs, ':') { // IPv6
		rhostport = fmt.Sprintf("[%s]:%d", IPs, res.PeerPort)
	} else { // IPv4
		rhostport = fmt.Sprintf("%s:%d", IPs, res.PeerPort)
	}

	raddr, err := net.ResolveTCPAddr("tcp", rhostport)
	if err != nil {
		zap.L().Debug("MetadataSink.Sink: Couldn't resolve peer address!", zap.Error(err))
		return
	}

	go ms.awaitMetadata(res.InfoHash, Peer{Addr: raddr})

	ms.incomingInfoHashes[res.InfoHash] = struct{}{}
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
	close(ms.drain)
}

func (ms *MetadataSink) flush(result Metadata) {
	if !ms.terminated {
		ms.drain <- result
		// Delete the infoHash from ms.incomingInfoHashes ONLY AFTER once we've flushed the
		// metadata!
		var infoHash [20]byte
		copy(infoHash[:], result.InfoHash)
		delete(ms.incomingInfoHashes, infoHash)
	}
}
