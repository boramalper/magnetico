package mainline

import (
	"crypto/rand"
	"crypto/sha1"
	"net"
	"sync"
	"time"

	"go.uber.org/zap"
)

type Protocol struct {
	previousTokenSecret, currentTokenSecret []byte
	tokenLock                               sync.Mutex
	transport                               *Transport
	eventHandlers                           ProtocolEventHandlers
	started                                 bool
}

type ProtocolEventHandlers struct {
	OnPingQuery                  func(*Message, *net.UDPAddr)
	OnFindNodeQuery              func(*Message, *net.UDPAddr)
	OnGetPeersQuery              func(*Message, *net.UDPAddr)
	OnAnnouncePeerQuery          func(*Message, *net.UDPAddr)
	OnGetPeersResponse           func(*Message, *net.UDPAddr)
	OnFindNodeResponse           func(*Message, *net.UDPAddr)
	OnPingORAnnouncePeerResponse func(*Message, *net.UDPAddr)

	// Added by BEP 51
	OnSampleInfohashesQuery    func(*Message, *net.UDPAddr)
	OnSampleInfohashesResponse func(*Message, *net.UDPAddr)

	OnCongestion func()
}

func NewProtocol(laddr string, eventHandlers ProtocolEventHandlers) (p *Protocol) {
	p = new(Protocol)
	p.eventHandlers = eventHandlers
	p.transport = NewTransport(laddr, p.onMessage, p.eventHandlers.OnCongestion)

	p.currentTokenSecret, p.previousTokenSecret = make([]byte, 20), make([]byte, 20)
	_, err := rand.Read(p.currentTokenSecret)
	if err != nil {
		zap.L().Fatal("Could NOT generate random bytes for token secret!", zap.Error(err))
	}
	copy(p.previousTokenSecret, p.currentTokenSecret)

	return
}

func (p *Protocol) Start() {
	if p.started {
		zap.L().Panic("Attempting to Start() a mainline/Protocol that has been already started! (Programmer error.)")
	}
	p.started = true

	p.transport.Start()
	go p.updateTokenSecret()
}

func (p *Protocol) Terminate() {
	if !p.started {
		zap.L().Panic("Attempted to Terminate() a mainline/Protocol that has not been Start()ed! (Programmer error.)")
	}

	p.transport.Terminate()
}

func (p *Protocol) onMessage(msg *Message, addr *net.UDPAddr) {
	switch msg.Y {
	case "q":
		switch msg.Q {
		case "ping":
			if !validatePingQueryMessage(msg) {
				// zap.L().Debug("An invalid ping query received!")
				return
			}
			// Check whether there is a registered event handler for the ping queries, before
			// attempting to call.
			if p.eventHandlers.OnPingQuery != nil {
				p.eventHandlers.OnPingQuery(msg, addr)
			}

		case "find_node":
			if !validateFindNodeQueryMessage(msg) {
				// zap.L().Debug("An invalid find_node query received!")
				return
			}
			if p.eventHandlers.OnFindNodeQuery != nil {
				p.eventHandlers.OnFindNodeQuery(msg, addr)
			}

		case "get_peers":
			if !validateGetPeersQueryMessage(msg) {
				// zap.L().Debug("An invalid get_peers query received!")
				return
			}
			if p.eventHandlers.OnGetPeersQuery != nil {
				p.eventHandlers.OnGetPeersQuery(msg, addr)
			}

		case "announce_peer":
			if !validateAnnouncePeerQueryMessage(msg) {
				// zap.L().Debug("An invalid announce_peer query received!")
				return
			}
			if p.eventHandlers.OnAnnouncePeerQuery != nil {
				p.eventHandlers.OnAnnouncePeerQuery(msg, addr)
			}

		case "vote":
			// Although we are aware that such method exists, we ignore.

		case "sample_infohashes": // Added by BEP 51
			if !validateSampleInfohashesQueryMessage(msg) {
				// zap.L().Debug("An invalid sample_infohashes query received!")
				return
			}
			if p.eventHandlers.OnSampleInfohashesQuery != nil {
				p.eventHandlers.OnSampleInfohashesQuery(msg, addr)
			}

		default:
			// zap.L().Debug("A KRPC query of an unknown method received!", zap.String("method", msg.Q))
			return
		}
	case "r":
		// Query messages have a `q` field which indicates their type but response messages have no such field that we
		// can rely on.
		// The idea is you'd use transaction ID (the `t` key) to deduce the type of a response message, as it must be
		// sent in response to a query message (with the same transaction ID) that we have sent earlier.
		// This approach is, unfortunately, not very practical for our needs since we send up to thousands messages per
		// second, meaning that we'd run out of transaction IDs very quickly (since some [many?] clients assume
		// transaction IDs are no longer than 2 bytes), and we'd also then have to consider retention too (as we might
		// not get a response at all).
		// Our approach uses an ad-hoc pattern matching: all response messages share a subset of fields (such as `t`,
		// `y`) but only one type of them contain a particular field (such as `token` field is unique to `get_peers`
		// responses, `samples` is unique to `sample_infohashes` etc).
		//
		// sample_infohashes > get_peers > find_node > ping / announce_peer
		if len(msg.R.Samples) != 0 { // The message should be a sample_infohashes response.
			if !validateSampleInfohashesResponseMessage(msg) {
				// zap.L().Debug("An invalid sample_infohashes response received!")
				return
			}
			if p.eventHandlers.OnSampleInfohashesResponse != nil {
				p.eventHandlers.OnSampleInfohashesResponse(msg, addr)
			}
		} else if len(msg.R.Token) != 0 { // The message should be a get_peers response.
			if !validateGetPeersResponseMessage(msg) {
				// zap.L().Debug("An invalid get_peers response received!")
				return
			}
			if p.eventHandlers.OnGetPeersResponse != nil {
				p.eventHandlers.OnGetPeersResponse(msg, addr)
			}
		} else if len(msg.R.Nodes) != 0 { // The message should be a find_node response.
			if !validateFindNodeResponseMessage(msg) {
				// zap.L().Debug("An invalid find_node response received!")
				return
			}
			if p.eventHandlers.OnFindNodeResponse != nil {
				p.eventHandlers.OnFindNodeResponse(msg, addr)
			}
		} else { // The message should be a ping or an announce_peer response.
			if !validatePingORannouncePeerResponseMessage(msg) {
				// zap.L().Debug("An invalid ping OR announce_peer response received!")
				return
			}
			if p.eventHandlers.OnPingORAnnouncePeerResponse != nil {
				p.eventHandlers.OnPingORAnnouncePeerResponse(msg, addr)
			}
		}
	case "e":
		// TODO: currently ignoring Server Error 202
		if msg.E.Code != 202 {
			zap.L().Sugar().Debugf("Protocol error received: `%s` (%d)", msg.E.Message, msg.E.Code)
		}
	default:
		/* zap.L().Debug("A KRPC message of an unknown type received!",
		zap.String("type", msg.Y))
		*/
	}
}

func (p *Protocol) SendMessage(msg *Message, addr *net.UDPAddr) {
	p.transport.WriteMessages(msg, addr)
}

func NewPingQuery(id []byte) *Message {
	panic("Not implemented yet!")
}

func NewFindNodeQuery(id []byte, target []byte) *Message {
	return &Message{
		Y: "q",
		T: []byte("aa"),
		Q: "find_node",
		A: QueryArguments{
			ID:     id,
			Target: target,
		},
	}
}

func NewGetPeersQuery(id []byte, infoHash []byte) *Message {
	return &Message{
		Y: "q",
		T: []byte("aa"),
		Q: "get_peers",
		A: QueryArguments{
			ID:       id,
			InfoHash: infoHash,
		},
	}
}

func NewAnnouncePeerQuery(id []byte, implied_port bool, info_hash []byte, port uint16, token []byte) *Message {
	panic("Not implemented yet!")
}

func NewSampleInfohashesQuery(id []byte, t []byte, target []byte) *Message {
	return &Message{
		Y: "q",
		T: t,
		Q: "sample_infohashes",
		A: QueryArguments{
			ID:     id,
			Target: target,
		},
	}
}

func NewPingResponse(t []byte, id []byte) *Message {
	return &Message{
		Y: "r",
		T: t,
		R: ResponseValues{
			ID: id,
		},
	}
}

func NewFindNodeResponse(t []byte, id []byte, nodes []CompactNodeInfo) *Message {
	panic("Not implemented yet!")
}

func NewGetPeersResponseWithValues(t []byte, id []byte, token []byte, values []CompactPeer) *Message {
	panic("Not implemented yet!")
}

func NewGetPeersResponseWithNodes(t []byte, id []byte, token []byte, nodes []CompactNodeInfo) *Message {
	return &Message{
		Y: "r",
		T: t,
		R: ResponseValues{
			ID:    id,
			Token: token,
			Nodes: nodes,
		},
	}
}

func NewAnnouncePeerResponse(t []byte, id []byte) *Message {
	// Because they are indistinguishable.
	return NewPingResponse(t, id)
}

func (p *Protocol) CalculateToken(address net.IP) []byte {
	p.tokenLock.Lock()
	defer p.tokenLock.Unlock()
	sum := sha1.Sum(append(p.currentTokenSecret, address...))
	return sum[:]
}

func (p *Protocol) VerifyToken(address net.IP, token []byte) bool {
	p.tokenLock.Lock()
	defer p.tokenLock.Unlock()
	// TODO: implement VerifyToken()
	panic("VerifyToken() not implemented yet!")
}

func (p *Protocol) updateTokenSecret() {
	for range time.Tick(10 * time.Minute) {
		p.tokenLock.Lock()
		copy(p.previousTokenSecret, p.currentTokenSecret)
		_, err := rand.Read(p.currentTokenSecret)
		if err != nil {
			p.tokenLock.Unlock()
			zap.L().Fatal("Could NOT generate random bytes for token secret!", zap.Error(err))
		}
		p.tokenLock.Unlock()
	}
}

func validatePingQueryMessage(msg *Message) bool {
	return len(msg.A.ID) == 20
}

func validateFindNodeQueryMessage(msg *Message) bool {
	return len(msg.A.ID) == 20 &&
		len(msg.A.Target) == 20
}

func validateGetPeersQueryMessage(msg *Message) bool {
	return len(msg.A.ID) == 20 &&
		len(msg.A.InfoHash) == 20
}

func validateAnnouncePeerQueryMessage(msg *Message) bool {
	return len(msg.A.ID) == 20 &&
		len(msg.A.InfoHash) == 20 &&
		msg.A.Port > 0 &&
		len(msg.A.Token) > 0
}

func validateSampleInfohashesQueryMessage(msg *Message) bool {
	return len(msg.A.ID) == 20 &&
		len(msg.A.Target) == 20
}

func validatePingORannouncePeerResponseMessage(msg *Message) bool {
	return len(msg.R.ID) == 20
}

func validateFindNodeResponseMessage(msg *Message) bool {
	if len(msg.R.ID) != 20 {
		return false
	}

	// TODO: check nodes field

	return true
}

func validateGetPeersResponseMessage(msg *Message) bool {
	return len(msg.R.ID) == 20 &&
		len(msg.R.Token) > 0

	// TODO: check for values or nodes
}

func validateSampleInfohashesResponseMessage(msg *Message) bool {
	return len(msg.R.ID) == 20 &&
		msg.R.Interval >= 0 &&
		// TODO: check for nodes
		msg.R.Num >= 0 &&
		len(msg.R.Samples)%20 == 0
}
