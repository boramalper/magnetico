package mainline

import (
	"math/rand"
	"net"
	"sync"
	"time"

	"go.uber.org/zap"
)

type IndexingService struct {
	// Private
	protocol      *Protocol
	started       bool
	interval      time.Duration
	eventHandlers IndexingServiceEventHandlers

	nodeID []byte
	// []byte type would be a much better fit for the keys but unfortunately (and quite
	// understandably) slices cannot be used as keys (since they are not hashable), and using arrays
	// (or even the conversion between each other) is a pain; hence map[string]net.UDPAddr
	//                                                                  ^~~~~~
	routingTable      map[string]*net.UDPAddr
	routingTableMutex *sync.Mutex
	maxNeighbors      uint

	counter          uint16
	getPeersRequests map[[2]byte][20]byte // GetPeersQuery.`t` -> infohash
}

type IndexingServiceEventHandlers struct {
	OnResult func(IndexingResult)
}

type IndexingResult struct {
	infoHash  [20]byte
	peerAddrs []net.TCPAddr
}

func (ir IndexingResult) InfoHash() [20]byte {
	return ir.infoHash
}

func (ir IndexingResult) PeerAddrs() []net.TCPAddr {
	return ir.peerAddrs
}

func NewIndexingService(laddr string, interval time.Duration, maxNeighbors uint, eventHandlers IndexingServiceEventHandlers) *IndexingService {
	service := new(IndexingService)
	service.interval = interval
	service.protocol = NewProtocol(
		laddr,
		ProtocolEventHandlers{
			OnFindNodeResponse:         service.onFindNodeResponse,
			OnGetPeersResponse:         service.onGetPeersResponse,
			OnSampleInfohashesResponse: service.onSampleInfohashesResponse,
		},
	)
	service.nodeID = make([]byte, 20)
	service.routingTable = make(map[string]*net.UDPAddr)
	service.routingTableMutex = new(sync.Mutex)
	service.maxNeighbors = maxNeighbors
	service.eventHandlers = eventHandlers

	service.getPeersRequests = make(map[[2]byte][20]byte)

	return service
}

func (is *IndexingService) Start() {
	if is.started {
		zap.L().Panic("Attempting to Start() a mainline/IndexingService that has been already started! (Programmer error.)")
	}
	is.started = true

	is.protocol.Start()
	go is.index()

	zap.L().Info("Indexing Service started!")
}

func (is *IndexingService) Terminate() {
	is.protocol.Terminate()
}

func (is *IndexingService) index() {
	for range time.Tick(is.interval) {
		is.routingTableMutex.Lock()
		if len(is.routingTable) == 0 {
			is.bootstrap()
		} else {
			zap.L().Info("Latest status:", zap.Int("n", len(is.routingTable)),
				zap.Uint("maxNeighbors", is.maxNeighbors))
			//TODO
			is.findNeighbors()
			is.routingTable = make(map[string]*net.UDPAddr)
		}
		is.routingTableMutex.Unlock()
	}
}

func (is *IndexingService) bootstrap() {
	bootstrappingNodes := []string{
		"router.bittorrent.com:6881",
		"dht.transmissionbt.com:6881",
		"dht.libtorrent.org:25401",
	}

	zap.L().Info("Bootstrapping as routing table is empty...")
	for _, node := range bootstrappingNodes {
		target := make([]byte, 20)
		_, err := rand.Read(target)
		if err != nil {
			zap.L().Panic("Could NOT generate random bytes during bootstrapping!")
		}

		addr, err := net.ResolveUDPAddr("udp", node)
		if err != nil {
			zap.L().Error("Could NOT resolve (UDP) address of the bootstrapping node!",
				zap.String("node", node))
			continue
		}

		is.protocol.SendMessage(NewFindNodeQuery(is.nodeID, target), addr)
	}
}

func (is *IndexingService) findNeighbors() {
	target := make([]byte, 20)
	for _, addr := range is.routingTable {
		_, err := rand.Read(target)
		if err != nil {
			zap.L().Panic("Could NOT generate random bytes during bootstrapping!")
		}

		is.protocol.SendMessage(
			NewSampleInfohashesQuery(is.nodeID, []byte("aa"), target),
			addr,
		)
	}
}

func (is *IndexingService) onFindNodeResponse(response *Message, addr *net.UDPAddr) {
	is.routingTableMutex.Lock()
	defer is.routingTableMutex.Unlock()

	for _, node := range response.R.Nodes {
		if uint(len(is.routingTable)) >= is.maxNeighbors {
			break
		}
		if node.Addr.Port == 0 { // Ignore nodes who "use" port 0.
			continue
		}

		is.routingTable[string(node.ID)] = &node.Addr

		target := make([]byte, 20)
		_, err := rand.Read(target)
		if err != nil {
			zap.L().Panic("Could NOT generate random bytes!")
		}
		is.protocol.SendMessage(
			NewSampleInfohashesQuery(is.nodeID, []byte("aa"), target),
			&node.Addr,
		)
	}
}

func (is *IndexingService) onGetPeersResponse(msg *Message, addr *net.UDPAddr) {
	var t [2]byte
	copy(t[:], msg.T)

	infoHash := is.getPeersRequests[t]
	// We got a response, so free the key!
	delete(is.getPeersRequests, t)

	// BEP 51 specifies that
	//     The new sample_infohashes remote procedure call requests that a remote node return a string of multiple
	//     concatenated infohashes (20 bytes each) FOR WHICH IT HOLDS GET_PEERS VALUES.
	//                                                                          ^^^^^^
	// So theoretically we should never hit the case where `values` is empty, but c'est la vie.
	if len(msg.R.Values) == 0 {
		return
	}

	peerAddrs := make([]net.TCPAddr, 0)
	for _, peer := range msg.R.Values {
		if peer.Port == 0 {
			continue
		}

		peerAddrs = append(peerAddrs, net.TCPAddr{
			IP:   peer.IP,
			Port: peer.Port,
		})
	}

	is.eventHandlers.OnResult(IndexingResult{
		infoHash:  infoHash,
		peerAddrs: peerAddrs,
	})
}

func (is *IndexingService) onSampleInfohashesResponse(msg *Message, addr *net.UDPAddr) {
	// request samples
	for i := 0; i < len(msg.R.Samples)/20; i++ {
		var infoHash [20]byte
		copy(infoHash[:], msg.R.Samples[i:(i+1)*20])

		msg := NewGetPeersQuery(is.nodeID, infoHash[:])
		t := uint16BE(is.counter)
		msg.T = t[:]

		is.protocol.SendMessage(msg, addr)

		is.getPeersRequests[t] = infoHash
		is.counter++
	}

	// iterate
	for _, node := range msg.R.Nodes {
		if uint(len(is.routingTable)) >= is.maxNeighbors {
			break
		}
		if node.Addr.Port == 0 { // Ignore nodes who "use" port 0.
			continue
		}
		is.routingTable[string(node.ID)] = &node.Addr

		// TODO
		/*
			target := make([]byte, 20)
			_, err := rand.Read(target)
			if err != nil {
				zap.L().Panic("Could NOT generate random bytes!")
			}
			is.protocol.SendMessage(
				NewSampleInfohashesQuery(is.nodeID, []byte("aa"), target),
				&node.Addr,
			)
		*/
	}
}

func uint16BE(v uint16) (b [2]byte) {
	b[0] = byte(v >> 8)
	b[1] = byte(v)
	return
}
