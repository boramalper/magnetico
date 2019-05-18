package metadata

import (
	"math/rand"
	"net"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/boramalper/magnetico/cmd/magneticod/dht"
	"github.com/boramalper/magnetico/pkg/persistence"
	"github.com/boramalper/magnetico/pkg/util"
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

type Sink struct {
	PeerID      []byte
	deadline    time.Duration
	maxNLeeches int
	drain       chan Metadata

	incomingInfoHashes   map[[20]byte][]net.TCPAddr
	incomingInfoHashesMx sync.Mutex

	terminated  bool
	termination chan interface{}

	deleted int
}

func randomID() []byte {
	/* > The peer_id is exactly 20 bytes (characters) long.
	 * >
	 * > There are mainly two conventions how to encode client and client version information into the peer_id,
	 * > Azureus-style and Shadow's-style.
	 * >
	 * > Azureus-style uses the following encoding: '-', two characters for client id, four ascii digits for version
	 * > number, '-', followed by random numbers.
	 * >
	 * > For example: '-AZ2060-'...
	 *
	 * https://wiki.theory.org/index.php/BitTorrentSpecification
	 *
	 * We encode the version number as:
	 *  - First two digits for the major version number
	 *  - Last two digits for the minor version number
	 *  - Patch version number is not encoded.
	 */
	prefix := []byte("-MC0008-")

	var rando []byte
	for i := 20 - len(prefix); i >= 0; i-- {
		rando = append(rando, randomDigit())
	}

	return append(prefix, rando...)
}

func randomDigit() byte {
	var max, min int
	max, min = '9', '0'
	return byte(rand.Intn(max-min) + min)
}

func NewSink(deadline time.Duration, maxNLeeches int) *Sink {
	ms := new(Sink)

	ms.PeerID = randomID()
	ms.deadline = deadline
	ms.maxNLeeches = maxNLeeches
	ms.drain = make(chan Metadata, 10)
	ms.incomingInfoHashes = make(map[[20]byte][]net.TCPAddr)
	ms.termination = make(chan interface{})

	go func() {
		for range time.Tick(deadline) {
			ms.incomingInfoHashesMx.Lock()
			l := len(ms.incomingInfoHashes)
			ms.incomingInfoHashesMx.Unlock()
			zap.L().Info("Sink status",
				zap.Int("activeLeeches", l),
				zap.Int("nDeleted", ms.deleted),
				zap.Int("drainQueue", len(ms.drain)),
			)
			ms.deleted = 0
		}
	}()

	return ms
}

func (ms *Sink) Sink(res dht.Result) {
	if ms.terminated {
		zap.L().Panic("Trying to Sink() an already closed Sink!")
	}
	ms.incomingInfoHashesMx.Lock()
	defer ms.incomingInfoHashesMx.Unlock()

	// cap the max # of leeches
	if len(ms.incomingInfoHashes) >= ms.maxNLeeches {
		return
	}

	infoHash := res.InfoHash()
	peerAddrs := res.PeerAddrs()

	if _, exists := ms.incomingInfoHashes[infoHash]; exists {
		return
	} else if len(peerAddrs) > 0 {
		peer := peerAddrs[0]
		ms.incomingInfoHashes[infoHash] = peerAddrs[1:]

		go NewLeech(infoHash, &peer, ms.PeerID, LeechEventHandlers{
			OnSuccess: ms.flush,
			OnError:   ms.onLeechError,
		}).Do(time.Now().Add(ms.deadline))
	}

	zap.L().Debug("Sunk!", zap.Int("leeches", len(ms.incomingInfoHashes)), util.HexField("infoHash", infoHash[:]))
}

func (ms *Sink) Drain() <-chan Metadata {
	if ms.terminated {
		zap.L().Panic("Trying to Drain() an already closed Sink!")
	}
	return ms.drain
}

func (ms *Sink) Terminate() {
	ms.terminated = true
	close(ms.termination)
	close(ms.drain)
}

func (ms *Sink) flush(result Metadata) {
	if ms.terminated {
		return
	}

	ms.drain <- result
	// Delete the infoHash from ms.incomingInfoHashes ONLY AFTER once we've flushed the
	// metadata!
	ms.incomingInfoHashesMx.Lock()
	defer ms.incomingInfoHashesMx.Unlock()

	var infoHash [20]byte
	copy(infoHash[:], result.InfoHash)
	delete(ms.incomingInfoHashes, infoHash)
}

func (ms *Sink) onLeechError(infoHash [20]byte, err error) {
	zap.L().Debug("leech error", util.HexField("infoHash", infoHash[:]), zap.Error(err))

	ms.incomingInfoHashesMx.Lock()
	defer ms.incomingInfoHashesMx.Unlock()

	if len(ms.incomingInfoHashes[infoHash]) > 0 {
		peer := ms.incomingInfoHashes[infoHash][0]
		ms.incomingInfoHashes[infoHash] = ms.incomingInfoHashes[infoHash][1:]
		go NewLeech(infoHash, &peer, ms.PeerID, LeechEventHandlers{
			OnSuccess: ms.flush,
			OnError:   ms.onLeechError,
		}).Do(time.Now().Add(ms.deadline))
	} else {
		ms.deleted++
		delete(ms.incomingInfoHashes, infoHash)
	}
}
