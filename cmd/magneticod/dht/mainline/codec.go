// TODO: This file, as a whole, needs a little skim-through to clear things up, sprinkle a little
//       documentation here and there, and also to make the test coverage 100%.
//       It, most importantly, lacks IPv6 support, if it's not altogether messy and unreliable
//       (hint: it is).

package mainline

import (
	"encoding/binary"
	"fmt"
	"net"

	"github.com/pkg/errors"

	"regexp"

	"github.com/anacrolix/missinggo/iter"
	"github.com/anacrolix/torrent/bencode"
	"github.com/willf/bloom"
)

type Message struct {
	// Query method. One of 5:
	//   - "ping"
	//   - "find_node"
	//   - "get_peers"
	//   - "announce_peer"
	//   - "sample_infohashes" (added by BEP 51)
	Q string `bencode:"q,omitempty"`
	// named QueryArguments sent with a query
	A QueryArguments `bencode:"a,omitempty"`
	// required: transaction ID
	T []byte `bencode:"t"`
	// required: type of the message: q for QUERY, r for RESPONSE, e for ERROR
	Y string `bencode:"y"`
	// RESPONSE type only
	R ResponseValues `bencode:"r,omitempty"`
	// ERROR type only
	E Error `bencode:"e,omitempty"`
}

type QueryArguments struct {
	// ID of the querying Node
	ID []byte `bencode:"id"`
	// InfoHash of the torrent
	InfoHash []byte `bencode:"info_hash,omitempty"`
	// ID of the node sought
	Target []byte `bencode:"target,omitempty"`
	// Token received from an earlier get_peers query
	Token []byte `bencode:"token,omitempty"`
	// Senders torrent port
	Port int `bencode:"port,omitempty"`
	// Use senders apparent DHT port
	ImpliedPort int `bencode:"implied_port,omitempty"`

	// Indicates whether the querying node is seeding the torrent it announces.
	// Defined in BEP 33 "DHT Scrapes" for `announce_peer` queries.
	Seed int `bencode:"seed,omitempty"`

	// If 1, then the responding node should try to fill the `values` list with non-seed items on a
	// best-effort basis."
	// Defined in BEP 33 "DHT Scrapes" for `get_peers` queries.
	NoSeed int `bencode:"noseed,omitempty"`
	// If 1, then the responding node should add two fields to the "r" dictionary in the response:
	//   - `BFsd`: Bloom Filter (256 bytes) representing all stored seeds for that infohash
	//   - `BFpe`: Bloom Filter (256 bytes) representing all stored peers (leeches) for that
	//             infohash
	// Defined in BEP 33 "DHT Scrapes" for `get_peers` queries.
	Scrape int `bencode:"noseed,omitempty"`
}

type ResponseValues struct {
	// ID of the querying node
	ID []byte `bencode:"id"`
	// K closest nodes to the requested target
	Nodes CompactNodeInfos `bencode:"nodes,omitempty"`
	// Token for future announce_peer
	Token []byte `bencode:"token,omitempty"`
	// Torrent peers
	Values []CompactPeer `bencode:"values,omitempty"`

	// The subset refresh interval in seconds. Added by BEP 51.
	Interval int `bencode:"interval,omitempty"`
	// Number of infohashes in storage. Added by BEP 51.
	Num int `bencode:"num,omitempty"`
	// Subset of stored infohashes, N × 20 bytes. Added by BEP 51.
	Samples []byte `bencode:"samples,omitempty"`

	// If `scrape` is set to 1 in the `get_peers` query then the responding node should add the
	// below two fields to the "r" dictionary in the response:
	// Defined in BEP 33 "DHT Scrapes" for responses to `get_peers` queries.
	// Bloom Filter (256 bytes) representing all stored seeds for that infohash:
	BFsd *bloom.BloomFilter `bencode:"BFsd,omitempty"`
	// Bloom Filter (256 bytes) representing all stored peers (leeches) for that infohash:
	BFpe *bloom.BloomFilter `bencode:"BFpe,omitempty"`
	// TODO: write marshallers for those fields above ^^
}

type Error struct {
	Code    int
	Message []byte
}

// Represents peer address in either IPv6 or IPv4 form.
type CompactPeer struct {
	IP   net.IP
	Port int
}

type CompactPeers []CompactPeer

type CompactNodeInfo struct {
	ID   []byte
	Addr net.UDPAddr
}

type CompactNodeInfos []CompactNodeInfo

// This allows bencode.Unmarshal to do better than a string or []byte.
func (cps *CompactPeers) UnmarshalBencode(b []byte) (err error) {
	var bb []byte
	err = bencode.Unmarshal(b, &bb)
	if err != nil {
		return
	}
	*cps, err = UnmarshalCompactPeers(bb)
	return
}

func (cps CompactPeers) MarshalBinary() (ret []byte, err error) {
	ret = make([]byte, len(cps)*6)
	for i, cp := range cps {
		copy(ret[6*i:], cp.IP.To4())
		binary.BigEndian.PutUint16(ret[6*i+4:], uint16(cp.Port))
	}
	return
}

func (cp CompactPeer) MarshalBencode() (ret []byte, err error) {
	ip := cp.IP
	if ip4 := ip.To4(); ip4 != nil {
		ip = ip4
	}
	ret = make([]byte, len(ip)+2)
	copy(ret, ip)
	binary.BigEndian.PutUint16(ret[len(ip):], uint16(cp.Port))
	return bencode.Marshal(ret)
}

func (cp *CompactPeer) UnmarshalBinary(b []byte) error {
	switch len(b) {
	case 18:
		cp.IP = make([]byte, 16)
	case 6:
		cp.IP = make([]byte, 4)
	default:
		return fmt.Errorf("bad compact peer string: %q", b)
	}
	copy(cp.IP, b)
	b = b[len(cp.IP):]
	cp.Port = int(binary.BigEndian.Uint16(b))
	return nil
}

func (cp *CompactPeer) UnmarshalBencode(b []byte) (err error) {
	var _b []byte
	err = bencode.Unmarshal(b, &_b)
	if err != nil {
		return
	}
	return cp.UnmarshalBinary(_b)
}

func UnmarshalCompactPeers(b []byte) (ret []CompactPeer, err error) {
	num := len(b) / 6
	ret = make([]CompactPeer, num)
	for i := range iter.N(num) {
		off := i * 6
		err = ret[i].UnmarshalBinary(b[off : off+6])
		if err != nil {
			return
		}
	}
	return
}

// This allows bencode.Unmarshal to do better than a string or []byte.
func (cnis *CompactNodeInfos) UnmarshalBencode(b []byte) (err error) {
	var bb []byte
	err = bencode.Unmarshal(b, &bb)
	if err != nil {
		return
	}
	*cnis, err = UnmarshalCompactNodeInfos(bb)
	return
}

func UnmarshalCompactNodeInfos(b []byte) (ret []CompactNodeInfo, err error) {
	if len(b)%26 != 0 {
		err = fmt.Errorf("compact node is not a multiple of 26")
		return
	}

	num := len(b) / 26
	ret = make([]CompactNodeInfo, num)
	for i := range iter.N(num) {
		off := i * 26
		ret[i].ID = make([]byte, 20)
		err = ret[i].UnmarshalBinary(b[off : off+26])
		if err != nil {
			return
		}
	}
	return
}

func (cni *CompactNodeInfo) UnmarshalBinary(b []byte) error {
	copy(cni.ID[:], b)
	b = b[len(cni.ID):]
	cni.Addr.IP = make([]byte, 4)
	copy(cni.Addr.IP, b)
	b = b[len(cni.Addr.IP):]
	cni.Addr.Port = int(binary.BigEndian.Uint16(b))
	cni.Addr.Zone = ""
	return nil
}

func (cnis CompactNodeInfos) MarshalBencode() ([]byte, error) {
	var ret []byte

	if len(cnis) == 0 {
		return []byte("0:"), nil
	}

	for _, cni := range cnis {
		ret = append(ret, cni.MarshalBinary()...)
	}

	return bencode.Marshal(ret)
}

func (cni CompactNodeInfo) MarshalBinary() []byte {
	ret := make([]byte, 20)

	copy(ret, cni.ID)
	ret = append(ret, cni.Addr.IP.To4()...)

	portEncoding := make([]byte, 2)
	binary.BigEndian.PutUint16(portEncoding, uint16(cni.Addr.Port))
	ret = append(ret, portEncoding...)

	return ret
}

func (e Error) MarshalBencode() ([]byte, error) {
	return []byte(fmt.Sprintf("li%de%d:%se", e.Code, len(e.Message), e.Message)), nil
}

func (e *Error) UnmarshalBencode(b []byte) (err error) {
	var code, msgLen int

	result := regexp.MustCompile(`li([0-9]+)e([0-9]+):(.+)e`).FindAllSubmatch(b, 1)
	if len(result) == 0 {
		return fmt.Errorf("could not parse the error list")
	}

	matches := result[0][1:]
	if _, err := fmt.Sscanf(string(matches[0]), "%d", &code); err != nil {
		return errors.Wrap(err, "could not parse error code")
	}
	if _, err := fmt.Sscanf(string(matches[1]), "%d", &msgLen); err != nil {
		return errors.Wrap(err, "could not parse error msg length")
	}

	if len(matches[2]) != msgLen {
		return fmt.Errorf("error message have different lengths (%d vs %d) \"%s\"", len(matches[2]), msgLen, matches[2])
	}

	e.Code = code
	e.Message = matches[2]

	return nil
}
