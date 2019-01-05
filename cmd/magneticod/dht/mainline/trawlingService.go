package mainline

import (
	"crypto/rand"
	"net"
	"sync"
	"time"

	"go.uber.org/zap"
)

type TrawlingService struct {
	// Private
	protocol      *Protocol
	started       bool
	interval      time.Duration
	eventHandlers TrawlingServiceEventHandlers

	trueNodeID []byte
	// []byte type would be a much better fit for the keys but unfortunately (and quite
	// understandably) slices cannot be used as keys (since they are not hashable), and using arrays
	// (or even the conversion between each other) is a pain; hence map[string]net.UDPAddr
	//                                                                  ^~~~~~
	routingTable      map[string]*net.UDPAddr
	routingTableMutex *sync.Mutex
	maxNeighbors      uint
}

type TrawlingServiceEventHandlers struct {
	OnResult func(TrawlingResult)
}

type TrawlingResult struct {
	infoHash [20]byte
	peerAddr *net.TCPAddr
}

func (tr TrawlingResult) InfoHash() [20]byte {
	return tr.infoHash
}

func (tr TrawlingResult) PeerAddr() *net.TCPAddr {
	return tr.peerAddr
}

func NewTrawlingService(laddr string, initialMaxNeighbors uint, interval time.Duration, eventHandlers TrawlingServiceEventHandlers) *TrawlingService {
	service := new(TrawlingService)
	service.interval = interval
	service.protocol = NewProtocol(
		laddr,
		ProtocolEventHandlers{
			OnGetPeersQuery:     service.onGetPeersQuery,
			OnAnnouncePeerQuery: service.onAnnouncePeerQuery,
			OnFindNodeResponse:  service.onFindNodeResponse,
			OnCongestion:        service.onCongestion,
		},
	)
	service.trueNodeID = make([]byte, 20)
	service.routingTable = make(map[string]*net.UDPAddr)
	service.routingTableMutex = new(sync.Mutex)
	service.eventHandlers = eventHandlers
	service.maxNeighbors = initialMaxNeighbors

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
	for range time.Tick(s.interval) {
		// TODO
		// For some reason, we can't still detect congestion and this keeps increasing...
		// Disable for now.
		// s.maxNeighbors = uint(float32(s.maxNeighbors) * 1.001)

		s.routingTableMutex.Lock()
		if len(s.routingTable) == 0 {
			s.bootstrap()
		} else {
			zap.L().Info("Latest status:", zap.Int("n", len(s.routingTable)),
				zap.Uint("maxNeighbors", s.maxNeighbors))
			s.findNeighbors()
			s.routingTable = make(map[string]*net.UDPAddr)
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
			continue
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

func (s *TrawlingService) onGetPeersQuery(query *Message, addr *net.UDPAddr) {
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

func (s *TrawlingService) onAnnouncePeerQuery(query *Message, addr *net.UDPAddr) {
	/* BEP 5
	 *
	 * There is an optional argument called implied_port which value is either 0 or 1. If it is
	 * present and non-zero, the port argument should be ignored and the source port of the UDP
	 * packet should be used as the peer's port instead. This is useful for peers behind a NAT that
	 * may not know their external port, and supporting uTP, they accept incoming connections on the
	 * same port as the DHT port.
	 */
	var peerPort int
	if query.A.ImpliedPort != 0 {
		// TODO: Peer uses uTP, ignore for now
		// return
		peerPort = addr.Port
	} else {
		peerPort = query.A.Port
		// return
	}

	// TODO: It looks ugly, am I doing it right?  --Bora
	// (Converting slices to arrays in Go shouldn't have been such a pain...)
	var infoHash [20]byte
	copy(infoHash[:], query.A.InfoHash)
	s.eventHandlers.OnResult(TrawlingResult{
		infoHash: infoHash,
		peerAddr: &net.TCPAddr{
			IP:   addr.IP,
			Port: peerPort,
		},
	})

	s.protocol.SendMessage(
		NewAnnouncePeerResponse(
			query.T,
			append(query.A.ID[:15], s.trueNodeID[:5]...),
		),
		addr,
	)
}

func (s *TrawlingService) onFindNodeResponse(response *Message, addr *net.UDPAddr) {
	s.routingTableMutex.Lock()
	defer s.routingTableMutex.Unlock()

	for _, node := range response.R.Nodes {
		if uint(len(s.routingTable)) >= s.maxNeighbors {
			break
		}
		if node.Addr.Port == 0 { // Ignore nodes who "use" port 0.
			continue
		}

		s.routingTable[string(node.ID)] = &node.Addr
	}
}

func (s *TrawlingService) onCongestion() {
	/* The Congestion Prevention Strategy:
	 *
	 * In case of congestion, decrease the maximum number of nodes to the 90% of the current value.
	 */
	if s.maxNeighbors < 200 {
		zap.L().Warn("Max. number of neighbours are < 200 and there is still congestion!" +
			"(check your network connection if this message recurs)")
		return
	}

	s.maxNeighbors = uint(float32(s.maxNeighbors) * 0.9)
}
