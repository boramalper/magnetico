package mainline

import (
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"go.uber.org/zap"
)

type TrawlingResult struct {
	InfoHash metainfo.Hash
	Peer     torrent.Peer
	PeerIP   net.IP
	PeerPort int
}

type TrawlingService struct {
	// Private
	protocol      *Protocol
	started       bool
	eventHandlers TrawlingServiceEventHandlers

	trueNodeID []byte
	// []byte type would be a much better fit for the keys but unfortunately (and quite
	// understandably) slices cannot be used as keys (since they are not hashable), and using arrays
	// (or even the conversion between each other) is a pain; hence map[string]net.UDPAddr
	//                                                                  ^~~~~~
	routingTable      map[string]net.Addr
	routingTableMutex *sync.Mutex
}

type TrawlingServiceEventHandlers struct {
	OnResult func(TrawlingResult)
}

func NewTrawlingService(laddr string, eventHandlers TrawlingServiceEventHandlers) *TrawlingService {
	service := new(TrawlingService)
	service.protocol = NewProtocol(
		laddr,
		ProtocolEventHandlers{
			OnGetPeersQuery:     service.onGetPeersQuery,
			OnAnnouncePeerQuery: service.onAnnouncePeerQuery,
			OnFindNodeResponse:  service.onFindNodeResponse,
		},
	)
	service.trueNodeID = make([]byte, 20)
	service.routingTable = make(map[string]net.Addr)
	service.routingTableMutex = new(sync.Mutex)
	service.eventHandlers = eventHandlers

	_, err := rand.Read(service.trueNodeID)
	if err != nil {
		zap.L().Panic("Could NOT generate random bytes for node ID!")
	}

	return service
}

func (s *TrawlingService) Start() {
	if s.started {
		zap.L().Panic("Attempting to Start() a mainline/TrawlingService that has been already started! (Programmer error.)")
	}
	s.started = true

	s.protocol.Start()
	go s.trawl()

	zap.L().Info("Trawling Service started!")
}

func (s *TrawlingService) Terminate() {
	s.protocol.Terminate()
}

func (s *TrawlingService) trawl() {
	for range time.Tick(3 * time.Second) {
		s.routingTableMutex.Lock()
		if len(s.routingTable) == 0 {
			s.bootstrap()
		} else {
			zap.L().Debug("Latest status:", zap.Int("n", len(s.routingTable)))
			s.findNeighbors()
			s.routingTable = make(map[string]net.Addr)
		}
		s.routingTableMutex.Unlock()
	}
}

func (s *TrawlingService) bootstrap() {
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
		}

		s.protocol.SendMessage(NewFindNodeQuery(s.trueNodeID, target), addr)
	}
}

func (s *TrawlingService) findNeighbors() {
	target := make([]byte, 20)
	for nodeID, addr := range s.routingTable {
		_, err := rand.Read(target)
		if err != nil {
			zap.L().Panic("Could NOT generate random bytes during bootstrapping!")
		}

		s.protocol.SendMessage(
			NewFindNodeQuery(append([]byte(nodeID[:15]), s.trueNodeID[:5]...), target),
			addr,
		)
	}
}

func (s *TrawlingService) onGetPeersQuery(query *Message, addr net.Addr) {
	s.protocol.SendMessage(
		NewGetPeersResponseWithNodes(
			query.T,
			append(query.A.ID[:15], s.trueNodeID[:5]...),
			s.protocol.CalculateToken(net.ParseIP(addr.String()))[:],
			[]CompactNodeInfo{},
		),
		addr,
	)
}

func (s *TrawlingService) onAnnouncePeerQuery(query *Message, addr net.Addr) {
	var peerPort int
	if query.A.ImpliedPort != 0 {
		peerPort = addr.(*net.UDPAddr).Port
	} else {
		peerPort = query.A.Port
	}

	// TODO: It looks ugly, am I doing it right?  --Bora
	// (Converting slices to arrays in Go shouldn't have been such a pain...)
	var peerId, infoHash [20]byte
	copy(peerId[:], []byte("\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00"))
	copy(infoHash[:], query.A.InfoHash)
	s.eventHandlers.OnResult(TrawlingResult{
		InfoHash: infoHash,
		Peer: torrent.Peer{
			// As we don't know the ID of the remote peer, set it empty.
			Id:   peerId,
			IP:   addr.(*net.UDPAddr).IP,
			Port: peerPort,
			// "Ha" indicates that we discovered the peer through DHT Announce Peer (query); not
			// sure how anacrolix/torrent utilizes that information though.
			Source: "Ha",
			// We don't know whether the remote peer supports encryption either, but let's pretend
			// that it doesn't.
			SupportsEncryption: false,
		},
		PeerIP:   addr.(*net.UDPAddr).IP,
		PeerPort: peerPort,
	})

	s.protocol.SendMessage(
		NewAnnouncePeerResponse(
			query.T,
			append(query.A.ID[:15], s.trueNodeID[:5]...),
		),
		addr,
	)
}

func (s *TrawlingService) onFindNodeResponse(response *Message, addr net.Addr) {
	s.routingTableMutex.Lock()
	defer s.routingTableMutex.Unlock()

	for _, node := range response.R.Nodes {
		if node.Addr.Port != 0 { // Ignore nodes who "use" port 0.
			if len(s.routingTable) < 8000 {
				s.routingTable[string(node.ID)] = &node.Addr
			}
		}
	}
}
