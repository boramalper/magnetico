package metadata

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net"
	"time"

	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/pkg/errors"

	"go.uber.org/zap"

	"github.com/boramalper/magnetico/pkg/persistence"
)

const MAX_METADATA_SIZE = 10 * 1024 * 1024

type rootDict struct {
	M            mDict `bencode:"m"`
	MetadataSize int   `bencode:"metadata_size"`
}

type mDict struct {
	UTMetadata int `bencode:"ut_metadata"`
}

type extDict struct {
	MsgType int `bencode:"msg_type"`
	Piece   int `bencode:"piece"`
}

type Leech struct {
	infoHash [20]byte
	peerAddr *net.TCPAddr
	ev       LeechEventHandlers

	conn     *net.TCPConn
	clientID [20]byte

	ut_metadata                    uint8
	metadataReceived, metadataSize uint
	metadata                       []byte

	connClosed bool
}

type LeechEventHandlers struct {
	OnSuccess func(Metadata)        // must be supplied. args: metadata
	OnError   func([20]byte, error) // must be supplied. args: infohash, error
}

func NewLeech(infoHash [20]byte, peerAddr *net.TCPAddr, clientID []byte, ev LeechEventHandlers) *Leech {
	l := new(Leech)
	l.infoHash = infoHash
	l.peerAddr = peerAddr
	copy(l.clientID[:], clientID)
	l.ev = ev

	return l
}

func (l *Leech) writeAll(b []byte) error {
	for len(b) != 0 {
		n, err := l.conn.Write(b)
		if err != nil {
			return err
		}
		b = b[n:]
	}
	return nil
}

func (l *Leech) doBtHandshake() error {
	lHandshake := []byte(fmt.Sprintf(
		"\x13BitTorrent protocol\x00\x00\x00\x00\x00\x10\x00\x01%s%s",
		l.infoHash,
		l.clientID,
	))

	// ASSERTION
	if len(lHandshake) != 68 {
		panic(fmt.Sprintf("len(lHandshake) == %d", len(lHandshake)))
	}

	err := l.writeAll(lHandshake)
	if err != nil {
		return errors.Wrap(err, "writeAll lHandshake")
	}

	rHandshake, err := l.readExactly(68)
	if err != nil {
		return errors.Wrap(err, "readExactly rHandshake")
	}
	if !bytes.HasPrefix(rHandshake, []byte("\x13BitTorrent protocol")) {
		return fmt.Errorf("corrupt BitTorrent handshake received")
	}

	// TODO: maybe check for the infohash sent by the remote peer to double check?

	if (rHandshake[25] & 0x10) == 0 {
		return fmt.Errorf("peer does not support the extension protocol")
	}

	return nil
}

func (l *Leech) doExHandshake() error {
	err := l.writeAll([]byte("\x00\x00\x00\x1a\x14\x00d1:md11:ut_metadatai1eee"))
	if err != nil {
		return errors.Wrap(err, "writeAll lHandshake")
	}

	rExMessage, err := l.readExMessage()
	if err != nil {
		return errors.Wrap(err, "readExMessage")
	}

	// Extension Handshake has the Extension Message ID = 0x00
	if rExMessage[1] != 0 {
		return errors.Wrap(err, "first extension message is not an extension handshake")
	}

	rRootDict := new(rootDict)
	err = bencode.Unmarshal(rExMessage[2:], rRootDict)
	if err != nil {
		return errors.Wrap(err, "unmarshal rExMessage")
	}

	if !(0 < rRootDict.MetadataSize && rRootDict.MetadataSize < MAX_METADATA_SIZE) {
		return fmt.Errorf("metadata too big or its size is less than or equal zero")
	}

	if !(0 < rRootDict.M.UTMetadata && rRootDict.M.UTMetadata < 255) {
		return fmt.Errorf("ut_metadata is not an uint8")
	}

	l.ut_metadata = uint8(rRootDict.M.UTMetadata) // Save the ut_metadata code the remote peer uses
	l.metadataSize = uint(rRootDict.MetadataSize)
	l.metadata = make([]byte, l.metadataSize)

	return nil
}

func (l *Leech) requestAllPieces() error {
	// Request all the pieces of metadata
	nPieces := int(math.Ceil(float64(l.metadataSize) / math.Pow(2, 14)))
	for piece := 0; piece < nPieces; piece++ {
		// __request_metadata_piece(piece)
		// ...............................
		extDictDump, err := bencode.Marshal(extDict{
			MsgType: 0,
			Piece:   piece,
		})
		if err != nil { // ASSERT
			panic(errors.Wrap(err, "marshal extDict"))
		}

		err = l.writeAll([]byte(fmt.Sprintf(
			"%s\x14%s%s",
			toBigEndian(uint(2+len(extDictDump)), 4),
			toBigEndian(uint(l.ut_metadata), 1),
			extDictDump,
		)))
		if err != nil {
			return errors.Wrap(err, "writeAll piece request")
		}
	}

	return nil
}

// readMessage returns a BitTorrent message, sans the first 4 bytes indicating its length.
func (l *Leech) readMessage() ([]byte, error) {
	rLengthB, err := l.readExactly(4)
	if err != nil {
		return nil, errors.Wrap(err, "readExactly rLengthB")
	}

	rLength := uint(binary.BigEndian.Uint32(rLengthB))

	// Some malicious/faulty peers say that they are sending a very long
	// message, and hence causing us to run out of memory.
	// This is a crude check that does not let it happen (i.e. boundary can probably be
	// tightened a lot more.)
	if rLength > MAX_METADATA_SIZE {
		return nil, errors.New("message is longer than max allowed metadata size")
	}

	rMessage, err := l.readExactly(rLength)
	if err != nil {
		return nil, errors.Wrap(err, "readExactly rMessage")
	}

	return rMessage, nil
}

// readExMessage returns an *extension* message, sans the first 4 bytes indicating its length.
//
// It will IGNORE all non-extension messages!
func (l *Leech) readExMessage() ([]byte, error) {
	for {
		rMessage, err := l.readMessage()
		if err != nil {
			return nil, errors.Wrap(err, "readMessage")
		}

		// Every extension message has at least 2 bytes.
		if len(rMessage) < 2 {
			continue
		}

		// We are interested only in extension messages, whose first byte is always 20
		if rMessage[0] == 20 {
			return rMessage, nil
		}
	}
}

// readUmMessage returns an ut_metadata extension message, sans the first 4 bytes indicating its
// length.
//
// It will IGNORE all non-"ut_metadata extension" messages!
func (l *Leech) readUmMessage() ([]byte, error) {
	for {
		rExMessage, err := l.readExMessage()
		if err != nil {
			return nil, errors.Wrap(err, "readExMessage")
		}

		if rExMessage[1] == 0x01 {
			return rExMessage, nil
		}
	}
}

func (l *Leech) connect(deadline time.Time) error {
	var err error

	x, err := net.DialTimeout("tcp4", l.peerAddr.String(), 1*time.Second)
	if err != nil {
		return errors.Wrap(err, "dial")
	}
	l.conn = x.(*net.TCPConn)

	// > If sec == 0, operating system discards any unsent or unacknowledged data [after Close()
	// > has been called].
	err = l.conn.SetLinger(0)
	if err != nil {
		if err := l.conn.Close(); err != nil {
			zap.L().Panic("couldn't close leech connection!", zap.Error(err))
		}
		return errors.Wrap(err, "SetLinger")
	}

	err = l.conn.SetNoDelay(true)
	if err != nil {
		if err := l.conn.Close(); err != nil {
			zap.L().Panic("couldn't close leech connection!", zap.Error(err))
		}
		return errors.Wrap(err, "NODELAY")
	}

	err = l.conn.SetDeadline(deadline)
	if err != nil {
		if err := l.conn.Close(); err != nil {
			zap.L().Panic("couldn't close leech connection!", zap.Error(err))
		}
		return errors.Wrap(err, "SetDeadline")
	}

	return nil
}

func (l *Leech) closeConn() {
	if l.connClosed {
		return
	}

	if err := l.conn.Close(); err != nil {
		zap.L().Panic("couldn't close leech connection!", zap.Error(err))
		return
	}

	l.connClosed = true
}

func (l *Leech) Do(deadline time.Time) {
	err := l.connect(deadline)
	if err != nil {
		l.OnError(errors.Wrap(err, "connect"))
		return
	}
	defer l.closeConn()

	err = l.doBtHandshake()
	if err != nil {
		l.OnError(errors.Wrap(err, "doBtHandshake"))
		return
	}

	err = l.doExHandshake()
	if err != nil {
		l.OnError(errors.Wrap(err, "doExHandshake"))
		return
	}

	err = l.requestAllPieces()
	if err != nil {
		l.OnError(errors.Wrap(err, "requestAllPieces"))
		return
	}

	for l.metadataReceived < l.metadataSize {
		rUmMessage, err := l.readUmMessage()
		if err != nil {
			l.OnError(errors.Wrap(err, "readUmMessage"))
			return
		}

		// Run TestDecoder() function in leech_test.go in case you have any doubts.
		rMessageBuf := bytes.NewBuffer(rUmMessage[2:])
		rExtDict := new(extDict)
		err = bencode.NewDecoder(rMessageBuf).Decode(rExtDict)
		if err != nil {
			l.OnError(errors.Wrap(err, "could not decode ext msg in the loop"))
			return
		}

		if rExtDict.MsgType == 2 { // reject
			l.OnError(fmt.Errorf("remote peer rejected sending metadata"))
			return
		}

		if rExtDict.MsgType == 1 { // data
			// Get the unread bytes!
			metadataPiece := rMessageBuf.Bytes()

			// BEP 9 explicitly states:
			//   > If the piece is the last piece of the metadata, it may be less than 16kiB. If
			//   > it is not the last piece of the metadata, it MUST be 16kiB.
			//
			// Hence...
			//   ... if the length of @metadataPiece is more than 16kiB, we err.
			if len(metadataPiece) > 16*1024 {
				l.OnError(fmt.Errorf("metadataPiece > 16kiB"))
				return
			}

			piece := rExtDict.Piece
			// metadata[piece * 2**14: piece * 2**14 + len(metadataPiece)] = metadataPiece is how it'd be done in Python
			copy(l.metadata[piece*int(math.Pow(2, 14)):piece*int(math.Pow(2, 14))+len(metadataPiece)], metadataPiece)
			l.metadataReceived += uint(len(metadataPiece))

			// ... if the length of @metadataPiece is less than 16kiB AND metadata is NOT
			// complete then we err.
			if len(metadataPiece) < 16*1024 && l.metadataReceived != l.metadataSize {
				l.OnError(fmt.Errorf("metadataPiece < 16 kiB but incomplete"))
				return
			}

			if l.metadataReceived > l.metadataSize {
				l.OnError(fmt.Errorf("metadataReceived > metadataSize"))
				return
			}
		}
	}

	// We are done with the transfer, close socket as soon as possible (i.e. NOW) to avoid hitting "too many open files"
	// error.
	l.closeConn()

	// Verify the checksum
	sha1Sum := sha1.Sum(l.metadata)
	if !bytes.Equal(sha1Sum[:], l.infoHash[:]) {
		l.OnError(fmt.Errorf("infohash mismatch"))
		return
	}

	// Check the info dictionary
	info := new(metainfo.Info)
	err = bencode.Unmarshal(l.metadata, info)
	if err != nil {
		l.OnError(errors.Wrap(err, "unmarshal info"))
		return
	}
	err = validateInfo(info)
	if err != nil {
		l.OnError(errors.Wrap(err, "validateInfo"))
		return
	}

	var files []persistence.File
	// If there is only one file, there won't be a Files slice. That's why we need to add it here
	if len(info.Files) == 0 {
		files = append(files, persistence.File{
			Size: info.Length,
			Path: info.Name,
		})
	} else {
		for _, file := range info.Files {
			files = append(files, persistence.File{
				Size: file.Length,
				Path: file.DisplayPath(info),
			})
		}
	}

	var totalSize uint64
	for _, file := range files {
		if file.Size < 0 {
			l.OnError(fmt.Errorf("file size less than zero"))
			return
		}

		totalSize += uint64(file.Size)
	}

	l.ev.OnSuccess(Metadata{
		InfoHash:     l.infoHash[:],
		Name:         info.Name,
		TotalSize:    totalSize,
		DiscoveredOn: time.Now().Unix(),
		Files:        files,
	})
}

// COPIED FROM anacrolix/torrent
func validateInfo(info *metainfo.Info) error {
	if len(info.Pieces)%20 != 0 {
		return errors.New("pieces has invalid length")
	}
	if info.PieceLength == 0 {
		if info.TotalLength() != 0 {
			return errors.New("zero piece length")
		}
	} else {
		if int((info.TotalLength()+info.PieceLength-1)/info.PieceLength) != info.NumPieces() {
			return errors.New("piece count and file lengths are at odds")
		}
	}
	return nil
}

func (l *Leech) readExactly(n uint) ([]byte, error) {
	b := make([]byte, n)
	_, err := io.ReadFull(l.conn, b)
	return b, err
}

func (l *Leech) OnError(err error) {
	l.ev.OnError(l.infoHash, err)
}

// TODO: add bounds checking!
func toBigEndian(i uint, n int) []byte {
	b := make([]byte, n)
	switch n {
	case 1:
		b = []byte{byte(i)}

	case 2:
		binary.BigEndian.PutUint16(b, uint16(i))

	case 4:
		binary.BigEndian.PutUint32(b, uint32(i))

	default:
		panic(fmt.Sprintf("n must be 1, 2, or 4!"))
	}

	if len(b) != n {
		panic(fmt.Sprintf("postcondition failed: len(b) != n in intToBigEndian (i %d, n %d, len b %d, b %s)", i, n, len(b), b))
	}

	return b
}
