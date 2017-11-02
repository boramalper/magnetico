package bittorrent

import (
	"crypto/rand"
	"fmt"
	"net"
	"strings"
	"time"

	"go.uber.org/zap"

	"magnetico/magneticod/dht/mainline"
	"magnetico/persistence"
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
	// TODO: remove this
	if len(ms.clientID) != 20 {
		panic("CLIENT ID NOT 20!")
	}

	ms.deadline = deadline
	ms.drain = make(chan Metadata)
	ms.termination = make(chan interface{})
	return ms
}

func (ms *MetadataSink) Sink(res mainline.TrawlingResult) {
	if ms.terminated {
		zap.L().Panic("Trying to Sink() an already closed MetadataSink!")
	}

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
	}
}
