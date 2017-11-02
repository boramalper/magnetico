package mainline

import (
	"net"
	"testing"
)

var protocolTest_validInstances = []struct {
	validator func(*Message) bool
	msg       Message
}{
	// ping Query:
	{
		validator: validatePingQueryMessage,
		msg: Message{
			T: []byte("aa"),
			Y: "q",
			Q: "ping",
			A: QueryArguments{
				ID: []byte("abcdefghij0123456789"),
			},
		},
	},
	// ping or announce_peer Response:
	// Also, includes NUL and EOT characters as transaction ID (`t`).
	{
		validator: validatePingORannouncePeerResponseMessage,
		msg: Message{
			T: []byte("\x00\x04"),
			Y: "r",
			R: ResponseValues{
				ID: []byte("mnopqrstuvwxyz123456"),
			},
		},
	},
	// find_node Query:
	{
		validator: validateFindNodeQueryMessage,
		msg: Message{
			T: []byte("\x09\x0a"),
			Y: "q",
			Q: "find_node",
			A: QueryArguments{
				ID:     []byte("abcdefghij0123456789"),
				Target: []byte("mnopqrstuvwxyz123456"),
			},
		},
	},
	// find_node Response with no nodes (`nodes` key still exists):
	{
		validator: validateFindNodeResponseMessage,
		msg: Message{
			T: []byte("aa"),
			Y: "r",
			R: ResponseValues{
				ID:    []byte("0123456789abcdefghij"),
				Nodes: []CompactNodeInfo{},
			},
		},
	},
	// find_node Response with a single node:
	{
		validator: validateFindNodeResponseMessage,
		msg: Message{
			T: []byte("aa"),
			Y: "r",
			R: ResponseValues{
				ID: []byte("0123456789abcdefghij"),
				Nodes: []CompactNodeInfo{
					{
						ID:   []byte("abcdefghijklmnopqrst"),
						Addr: net.UDPAddr{IP: []byte("\x8b\x82\x8e\xf5"), Port: 3169, Zone: ""},
					},
				},
			},
		},
	},
	// find_node Response with 8 nodes (all the same except the very last one):
	{
		validator: validateFindNodeResponseMessage,
		msg: Message{
			T: []byte("aa"),
			Y: "r",
			R: ResponseValues{
				ID: []byte("0123456789abcdefghij"),
				Nodes: []CompactNodeInfo{
					{
						ID:   []byte("abcdefghijklmnopqrst"),
						Addr: net.UDPAddr{IP: []byte("\x8b\x82\x8e\xf5"), Port: 3169, Zone: ""},
					},
					{
						ID:   []byte("abcdefghijklmnopqrst"),
						Addr: net.UDPAddr{IP: []byte("\x8b\x82\x8e\xf5"), Port: 3169, Zone: ""},
					},
					{
						ID:   []byte("abcdefghijklmnopqrst"),
						Addr: net.UDPAddr{IP: []byte("\x8b\x82\x8e\xf5"), Port: 3169, Zone: ""},
					},
					{
						ID:   []byte("abcdefghijklmnopqrst"),
						Addr: net.UDPAddr{IP: []byte("\x8b\x82\x8e\xf5"), Port: 3169, Zone: ""},
					},
					{
						ID:   []byte("abcdefghijklmnopqrst"),
						Addr: net.UDPAddr{IP: []byte("\x8b\x82\x8e\xf5"), Port: 3169, Zone: ""},
					},
					{
						ID:   []byte("abcdefghijklmnopqrst"),
						Addr: net.UDPAddr{IP: []byte("\x8b\x82\x8e\xf5"), Port: 3169, Zone: ""},
					},
					{
						ID:   []byte("abcdefghijklmnopqrst"),
						Addr: net.UDPAddr{IP: []byte("\x8b\x82\x8e\xf5"), Port: 3169, Zone: ""},
					},
					{
						ID:   []byte("zyxwvutsrqponmlkjihg"),
						Addr: net.UDPAddr{IP: []byte("\xf5\x8e\x82\x8b"), Port: 6931, Zone: ""},
					},
				},
			},
		},
	},
	// get_peers Query:
	{
		validator: validateGetPeersQueryMessage,
		msg: Message{
			T: []byte("aa"),
			Y: "q",
			Q: "get_peers",
			A: QueryArguments{
				ID:       []byte("abcdefghij0123456789"),
				InfoHash: []byte("mnopqrstuvwxyz123456"),
			},
		},
	},
	// get_peers Response with 2 peers (`values`):
	{
		validator: validateGetPeersResponseMessage,
		msg: Message{
			T: []byte("aa"),
			Y: "r",
			R: ResponseValues{
				ID:    []byte("abcdefghij0123456789"),
				Token: []byte("aoeusnth"),
				Values: []CompactPeer{
					{IP: []byte("axje"), Port: 11893},
					{IP: []byte("idht"), Port: 28269},
				},
			},
		},
	},
	// get_peers Response with 2 closest nodes (`nodes`):
	{
		validator: validateGetPeersResponseMessage,
		msg: Message{
			T: []byte("aa"),
			Y: "r",
			R: ResponseValues{
				ID:    []byte("abcdefghij0123456789"),
				Token: []byte("aoeusnth"),
				Nodes: []CompactNodeInfo{
					{
						ID:   []byte("abcdefghijklmnopqrst"),
						Addr: net.UDPAddr{IP: []byte("\x8b\x82\x8e\xf5"), Port: 3169, Zone: ""},
					},
					{
						ID:   []byte("zyxwvutsrqponmlkjihg"),
						Addr: net.UDPAddr{IP: []byte("\xf5\x8e\x82\x8b"), Port: 6931, Zone: ""},
					},
				},
			},
		},
	},
	// announce_peer Query without optional `implied_port` argument:
	{
		validator: validateAnnouncePeerQueryMessage,
		msg: Message{
			T: []byte("aa"),
			Y: "q",
			Q: "announce_peer",
			A: QueryArguments{
				ID:       []byte("abcdefghij0123456789"),
				InfoHash: []byte("mnopqrstuvwxyz123456"),
				Port:     6881,
				Token:    []byte("aoeusnth"),
			},
		},
	},
	// TODO: Add announce_peer Query with optional `implied_port` argument.
}

func TestValidators(t *testing.T) {
	for i, instance := range protocolTest_validInstances {
		if isValid := instance.validator(&instance.msg); !isValid {
			t.Errorf("False-positive for valid msg #%d!", i+1)
		}
	}
}

func TestNewFindNodeQuery(t *testing.T) {
	if !validateFindNodeQueryMessage(NewFindNodeQuery([]byte("qwertyuopasdfghjklzx"), []byte("xzlkjhgfdsapouytrewq"))) {
		t.Errorf("NewFindNodeQuery returned an invalid message!")
	}
}

func TestNewPingResponse(t *testing.T) {
	if !validatePingORannouncePeerResponseMessage(NewPingResponse([]byte("tt"), []byte("qwertyuopasdfghjklzx"))) {
		t.Errorf("NewPingResponse returned an invalid message!")
	}
}

func TestNewGetPeersResponseWithNodes(t *testing.T) {
	if !validateGetPeersResponseMessage(NewGetPeersResponseWithNodes([]byte("tt"), []byte("qwertyuopasdfghjklzx"), []byte("token"), []CompactNodeInfo{})) {
		t.Errorf("NewGetPeersResponseWithNodes returned an invalid message!")
	}
}
