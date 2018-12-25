package metadata

import (
	"math/rand"
	"sync"
	"time"

	"github.com/boramalper/magnetico/pkg/util"
	"go.uber.org/zap"

	"github.com/boramalper/magnetico/cmd/magneticod/dht/mainline"
	"github.com/boramalper/magnetico/pkg/persistence"
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
	PeerID               []byte
	deadline             time.Duration
	drain                chan Metadata
	incomingInfoHashes   map[[20]byte]struct{}
	incomingInfoHashesMx sync.Mutex
	terminated           bool
	termination          chan interface{}
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
	prefix := []byte("-MC0007-")

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

func NewSink(deadline time.Duration) *Sink {
	ms := new(Sink)

	ms.PeerID = randomID()
	ms.deadline = deadline
	ms.drain = make(chan Metadata)
	ms.incomingInfoHashes = make(map[[20]byte]struct{})
	ms.termination = make(chan interface{})

	return ms
}

func (ms *Sink) Sink(res mainline.TrawlingResult) {
	if ms.terminated {
		zap.L().Panic("Trying to Sink() an already closed Sink!")
	}
	ms.incomingInfoHashesMx.Lock()
	defer ms.incomingInfoHashesMx.Unlock()

	if _, exists := ms.incomingInfoHashes[res.InfoHash]; exists {
		return
	}
	// BEWARE!
	// Although not crucial, the assumption is that Sink.Sink() will be called by only one
	// goroutine (i.e. it's not thread-safe), lest there might be a race condition between where we
	// check whether res.infoHash exists in the ms.incomingInfoHashes, and where we add the infoHash
	// to the incomingInfoHashes at the end of this function.

	zap.L().Info("Sunk!", zap.Int("leeches", len(ms.incomingInfoHashes)), util.HexField("infoHash", res.InfoHash[:]))

	go NewLeech(res.InfoHash, res.PeerAddr, ms.PeerID, LeechEventHandlers{
		OnSuccess: ms.flush,
		OnError:   ms.onLeechError,
	}).Do(time.Now().Add(ms.deadline))

	ms.incomingInfoHashes[res.InfoHash] = struct{}{}
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
	var infoHash [20]byte
	copy(infoHash[:], result.InfoHash)
	ms.incomingInfoHashesMx.Lock()
	delete(ms.incomingInfoHashes, infoHash)
	ms.incomingInfoHashesMx.Unlock()
}

func (ms *Sink) onLeechError(infoHash [20]byte, err error) {
	zap.L().Debug("leech error", util.HexField("infoHash", infoHash[:]), zap.Error(err))
	ms.incomingInfoHashesMx.Lock()
	delete(ms.incomingInfoHashes, infoHash)
	ms.incomingInfoHashesMx.Unlock()
}
