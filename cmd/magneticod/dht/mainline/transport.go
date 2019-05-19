package mainline

import (
	"net"

	"github.com/anacrolix/torrent/bencode"
	sockaddr "github.com/libp2p/go-sockaddr/net"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

type Transport struct {
	fd      int
	laddr   *net.UDPAddr
	started bool
	buffer  []byte

	// OnMessage is the function that will be called when Transport receives a packet that is
	// successfully unmarshalled as a syntactically correct Message (but -of course- the checking
	// the semantic correctness of the Message is left to Protocol).
	onMessage func(*Message, *net.UDPAddr)
	// OnCongestion
	onCongestion func()
}

func NewTransport(laddr string, onMessage func(*Message, *net.UDPAddr), onCongestion func()) *Transport {
	t := new(Transport)
	/*   The field size sets a theoretical limit of 65,535 bytes (8 byte header + 65,527 bytes of
	 * data) for a UDP datagram. However the actual limit for the data length, which is imposed by
	 * the underlying IPv4 protocol, is 65,507 bytes (65,535 − 8 byte UDP header − 20 byte IP
	 * header).
	 *
	 *   In IPv6 jumbograms it is possible to have UDP packets of size greater than 65,535 bytes.
	 * RFC 2675 specifies that the length field is set to zero if the length of the UDP header plus
	 * UDP data is greater than 65,535.
	 *
	 * https://en.wikipedia.org/wiki/User_Datagram_Protocol
	 */
	t.buffer = make([]byte, 65507)
	t.onMessage = onMessage
	t.onCongestion = onCongestion

	var err error
	t.laddr, err = net.ResolveUDPAddr("udp", laddr)
	if err != nil {
		zap.L().Panic("Could not resolve the UDP address for the trawler!", zap.Error(err))
	}
	if t.laddr.IP.To4() == nil {
		zap.L().Panic("IP address is not IPv4!")
	}

	return t
}

func (t *Transport) Start() {
	// Why check whether the Transport `t` started or not, here and not -for instance- in
	// t.Terminate()?
	// Because in t.Terminate() the programmer (i.e. you & me) would stumble upon an error while
	// trying close an uninitialised net.UDPConn or something like that: it's mostly harmless
	// because its effects are immediate. But if you try to start a Transport `t` for the second
	// (or the third, 4th, ...) time, it will keep spawning goroutines and any small mistake may
	// end up in a debugging horror.
	//                                                                   Here ends my justification.
	if t.started {
		zap.L().Panic("Attempting to Start() a mainline/Transport that has been already started! (Programmer error.)")
	}
	t.started = true

	var err error
	t.fd, err = unix.Socket(unix.SOCK_DGRAM, unix.AF_INET, 0)
	if err != nil {
		zap.L().Fatal("Could NOT create a UDP socket!", zap.Error(err))
	}

	var ip [4]byte
	copy(ip[:], t.laddr.IP.To4())
	err = unix.Bind(t.fd, &unix.SockaddrInet4{Addr: ip, Port: t.laddr.Port})
	if err != nil {
		zap.L().Fatal("Could NOT bind the socket!", zap.Error(err))
	}

	go t.readMessages()
}

func (t *Transport) Terminate() {
	unix.Close(t.fd)
}

// readMessages is a goroutine!
func (t *Transport) readMessages() {
	for {
		n, fromSA, err := unix.Recvfrom(t.fd, t.buffer, 0)
		if err == unix.EPERM || err == unix.ENOBUFS { // todo: are these errors possible for recvfrom?
			zap.L().Warn("READ CONGESTION!", zap.Error(err))
			t.onCongestion()
		} else if err != nil {
			// Socket is probably closed
			break
		}

		if n == 0 {
			/* Datagram sockets in various domains  (e.g., the UNIX and Internet domains) permit
			 * zero-length datagrams. When such a datagram is received, the return value (n) is 0.
			 */
			continue
		}

		from := sockaddr.SockaddrToUDPAddr(fromSA)
		if from == nil {
			zap.L().Panic("dht mainline transport SockaddrToUDPAddr: nil")
		}

		var msg Message
		err = bencode.Unmarshal(t.buffer[:n], &msg)
		if err != nil {
			// couldn't unmarshal packet data
			continue
		}

		t.onMessage(&msg, from)
	}
}

func (t *Transport) WriteMessages(msg *Message, addr *net.UDPAddr) {
	data, err := bencode.Marshal(msg)
	if err != nil {
		zap.L().Panic("Could NOT marshal an outgoing message! (Programmer error.)")
	}

	addrSA := sockaddr.NetAddrToSockaddr(addr)
	if addrSA == nil {
		zap.L().Debug("Wrong net address for the remote peer!",
			zap.String("addr", addr.String()))
		return
	}

	err = unix.Sendto(t.fd, data, 0, addrSA)
	if err == unix.EPERM || err == unix.ENOBUFS {
		/*   EPERM (errno: 1) is kernel's way of saying that "you are far too fast, chill". It is
		 * also likely that we have received a ICMP source quench packet (meaning, that we *really*
		 * need to slow down.
		 *
		 * Read more here: http://www.archivum.info/comp.protocols.tcp-ip/2009-05/00088/UDP-socket-amp-amp-sendto-amp-amp-EPERM.html
		 *
		 * >   Note On BSD systems (OS X, FreeBSD, etc.) flow control is not supported for
		 * > DatagramProtocol, because send failures caused by writing too many packets cannot be
		 * > detected easily. The socket always appears ‘ready’ and excess packets are dropped; an
		 * > OSError with errno set to errno.ENOBUFS may or may not be raised; if it is raised, it
		 * > will be reported to DatagramProtocol.error_received() but otherwise ignored.
		 *
		 * Source: https://docs.python.org/3/library/asyncio-protocol.html#flow-control-callbacks
		 */
		zap.L().Warn("WRITE CONGESTION!", zap.Error(err))
		if t.onCongestion != nil {
			t.onCongestion()
		}
	} else if err != nil {
		zap.L().Warn("Could NOT write an UDP packet!", zap.Error(err))
	}
}
