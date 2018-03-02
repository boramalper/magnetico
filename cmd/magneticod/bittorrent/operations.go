package bittorrent

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"time"

	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"

	"go.uber.org/zap"

	"magnetico/persistence"
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

func (ms *MetadataSink) awaitMetadata(infoHash metainfo.Hash, peer Peer) {
	conn, err := net.DialTCP("tcp", nil, peer.Addr)
	if err != nil {
		zap.L().Debug(
			"awaitMetadata couldn't connect to the peer!",
			zap.ByteString("infoHash", infoHash[:]),
			zap.String("remotePeerAddr", peer.Addr.String()),
			zap.Error(err),
		)
		return
	}
	defer conn.Close()

	err = conn.SetNoDelay(true)
	if err != nil {
		zap.L().Panic(
			"Couldn't set NODELAY!",
			zap.ByteString("infoHash", infoHash[:]),
			zap.String("remotePeerAddr", peer.Addr.String()),
			zap.Error(err),
		)
		return
	}
	err = conn.SetDeadline(time.Now().Add(ms.deadline))
	if err != nil {
		zap.L().Panic(
			"Couldn't set the deadline!",
			zap.ByteString("infoHash", infoHash[:]),
			zap.String("remotePeerAddr", peer.Addr.String()),
			zap.Error(err),
		)
		return
	}

	// State Variables
	var isExtHandshakeDone, done bool
	var ut_metadata, metadataReceived, metadataSize int
	var metadata []byte

	lHandshake := []byte(fmt.Sprintf(
		"\x13BitTorrent protocol\x00\x00\x00\x00\x00\x10\x00\x01%s%s",
		infoHash[:],
		ms.clientID,
	))
	if len(lHandshake) != 68 {
		zap.L().Panic(
			"Generated BitTorrent handshake is not of length 68!",
			zap.ByteString("infoHash", infoHash[:]),
			zap.Int("len_lHandshake", len(lHandshake)),
		)
	}
	err = writeAll(conn, lHandshake)
	if err != nil {
		zap.L().Debug(
			"Couldn't write BitTorrent handshake!",
			zap.ByteString("infoHash", infoHash[:]),
			zap.String("remotePeerAddr", peer.Addr.String()),
			zap.Error(err),
		)
		return
	}

	zap.L().Debug("BitTorrent handshake sent, waiting for the remote's...")

	rHandshake, err := readExactly(conn, 68)
	if err != nil {
		zap.L().Debug(
			"Couldn't read remote BitTorrent handshake!",
			zap.ByteString("infoHash", infoHash[:]),
			zap.String("remotePeerAddr", peer.Addr.String()),
			zap.Error(err),
		)
		return
	}
	if !bytes.HasPrefix(rHandshake, []byte("\x13BitTorrent protocol")) {
		zap.L().Debug(
			"Remote BitTorrent handshake is not what it is supposed to be!",
			zap.ByteString("infoHash", infoHash[:]),
			zap.String("remotePeerAddr", peer.Addr.String()),
			zap.ByteString("rHandshake[:20]", rHandshake[:20]),
		)
		return
	}

	// __on_bt_handshake
	// ================
	if rHandshake[25] != 16 { // TODO (later): do *not* compare the whole byte, check the bit instead! (0x10)
		zap.L().Debug(
			"Peer does not support the extension protocol!",
			zap.ByteString("infoHash", infoHash[:]),
			zap.String("remotePeerAddr", peer.Addr.String()),
		)
		return
	}

	writeAll(conn, []byte("\x00\x00\x00\x1a\x14\x00d1:md11:ut_metadatai1eee"))
	zap.L().Debug(
		"Extension handshake sent, waiting for the remote's...",
		zap.ByteString("infoHash", infoHash[:]),
		zap.String("remotePeerAddr", peer.Addr.String()),
	)

	// the loop!
	// =========
	for !done {
		rLengthB, err := readExactly(conn, 4)
		if err != nil {
			zap.L().Debug(
				"Couldn't read the first 4 bytes from the remote peer in the loop!",
				zap.ByteString("infoHash", infoHash[:]),
				zap.String("remotePeerAddr", peer.Addr.String()),
				zap.Error(err),
			)
			return
		}

		// The messages we are interested in have the length of AT LEAST two bytes
		// (TODO: actually a bit more than that but SURELY when it's less than two bytes, the
		//        program panics)
		rLength := bigEndianToInt(rLengthB)
		if rLength < 2 {
			continue
		}

		rMessage, err := readExactly(conn, rLength)
		if err != nil {
			zap.L().Debug(
				"Couldn't read the rest of the message from the remote peer in the loop!",
				zap.ByteString("infoHash", infoHash[:]),
				zap.String("remotePeerAddr", peer.Addr.String()),
				zap.Error(err),
			)
			return
		}

		// __on_message
		// ------------
		if rMessage[0] != 0x14 { // We are interested only in extension messages, whose first byte is always 0x14
			zap.L().Debug(
				"Ignoring the non-extension message.",
				zap.ByteString("infoHash", infoHash[:]),
			)
			continue
		}

		if rMessage[1] == 0x00 { // Extension Handshake has the Extension Message ID = 0x00
			// __on_ext_handshake_message(message[2:])
			// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

			// TODO: continue editing log messages from here

			if isExtHandshakeDone {
				return
			}

			rRootDict := new(rootDict)
			err := bencode.Unmarshal(rMessage[2:], rRootDict)
			if err != nil {
				zap.L().Debug("Couldn't unmarshal extension handshake!", zap.Error(err))
				return
			}

			if rRootDict.MetadataSize <= 0 || rRootDict.MetadataSize > MAX_METADATA_SIZE {
				zap.L().Debug("Unacceptable metadata size!", zap.Int("metadata_size", rRootDict.MetadataSize))
				return
			}

			ut_metadata = rRootDict.M.UTMetadata // Save the ut_metadata code the remote peer uses
			metadataSize = rRootDict.MetadataSize
			metadata = make([]byte, metadataSize)
			isExtHandshakeDone = true

			zap.L().Debug("GOT EXTENSION HANDSHAKE!", zap.Int("ut_metadata", ut_metadata), zap.Int("metadata_size", metadataSize))

			// Request all the pieces of metadata
			n_pieces := int(math.Ceil(float64(metadataSize) / math.Pow(2, 14)))
			for piece := 0; piece < n_pieces; piece++ {
				// __request_metadata_piece(piece)
				// ...............................
				extDictDump, err := bencode.Marshal(extDict{
					MsgType: 0,
					Piece:   piece,
				})
				if err != nil {
					zap.L().Warn("Couldn't marshal extDictDump!", zap.Error(err))
					return
				}
				writeAll(conn, []byte(fmt.Sprintf(
					"%s\x14%s%s",
					intToBigEndian(2+len(extDictDump), 4),
					intToBigEndian(ut_metadata, 1),
					extDictDump,
				)))
			}

			zap.L().Warn("requested all metadata pieces!")

		} else if rMessage[1] == 0x01 {
			// __on_ext_message(message[2:])
			// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

			// Run TestDecoder() function in operations_test.go in case you have any doubts.
			rMessageBuf := bytes.NewBuffer(rMessage[2:])
			rExtDict := new(extDict)
			err := bencode.NewDecoder(rMessageBuf).Decode(rExtDict)
			if err != nil {
				zap.L().Warn("Couldn't decode extension message in the loop!", zap.Error(err))
				return
			}

			if rExtDict.MsgType == 1 { // data
				// Get the unread bytes!
				metadataPiece := rMessageBuf.Bytes()
				piece := rExtDict.Piece
				// metadata[piece * 2**14: piece * 2**14 + len(metadataPiece)] = metadataPiece is how it'd be done in Python
				copy(metadata[piece*int(math.Pow(2, 14)):piece*int(math.Pow(2, 14))+len(metadataPiece)], metadataPiece)
				metadataReceived += len(metadataPiece)
				done = metadataReceived == metadataSize

				// BEP 9 explicitly states:
				//   > If the piece is the last piece of the metadata, it may be less than 16kiB. If
				//   > it is not the last piece of the metadata, it MUST be 16kiB.
				//
				// Hence...
				//   ... if the length of @metadataPiece is more than 16kiB, we err.
				if len(metadataPiece) > 16*1024 {
					zap.L().Debug(
						"metadataPiece is bigger than 16kiB!",
						zap.Int("len_metadataPiece", len(metadataPiece)),
						zap.Int("metadataReceived", metadataReceived),
						zap.Int("metadataSize", metadataSize),
						zap.Int("metadataPieceIndex", bytes.Index(rMessage, metadataPiece)),
					)
					return
				}

				// ... if the length of @metadataPiece is less than 16kiB AND metadata is NOT
				// complete (!done) then we err.
				if len(metadataPiece) < 16*1024 && !done {
					zap.L().Debug(
						"metadataPiece is less than 16kiB and metadata is incomplete!",
						zap.Int("len_metadataPiece", len(metadataPiece)),
						zap.Int("metadataReceived", metadataReceived),
						zap.Int("metadataSize", metadataSize),
						zap.Int("metadataPieceIndex", bytes.Index(rMessage, metadataPiece)),
					)
					return
				}

				if metadataReceived > metadataSize {
					zap.L().Debug(
						"metadataReceived is greater than metadataSize!",
						zap.Int("len_metadataPiece", len(metadataPiece)),
						zap.Int("metadataReceived", metadataReceived),
						zap.Int("metadataSize", metadataSize),
						zap.Int("metadataPieceIndex", bytes.Index(rMessage, metadataPiece)),
					)
					return
				}

				zap.L().Debug(
					"Fetching...",
					zap.ByteString("infoHash", infoHash[:]),
					zap.String("remotePeerAddr", conn.RemoteAddr().String()),
					zap.Int("metadataReceived", metadataReceived),
					zap.Int("metadataSize", metadataSize),
				)
			} else if rExtDict.MsgType == 2 { // reject
				zap.L().Debug(
					"Remote peer rejected sending metadata!",
					zap.ByteString("infoHash", infoHash[:]),
					zap.String("remotePeerAddr", conn.RemoteAddr().String()),
				)
				return
			}

		} else {
			zap.L().Debug(
				"Message is not an ut_metadata message! (ignoring)",
				zap.ByteString("msg", rMessage[:100]),
			)
			// no return!
		}
	}

	zap.L().Debug(
		"Metadata is complete, verifying the checksum...",
		zap.ByteString("infoHash", infoHash[:]),
	)

	sha1Sum := sha1.Sum(metadata)
	if !bytes.Equal(sha1Sum[:], infoHash[:]) {
		zap.L().Debug(
			"Info-hash mismatch!",
			zap.ByteString("expectedInfoHash", infoHash[:]),
			zap.ByteString("actualInfoHash", sha1Sum[:]),
		)
		return
	}

	zap.L().Debug(
		"Checksum verified, checking the info dictionary...",
		zap.ByteString("infoHash", infoHash[:]),
	)

	info := new(metainfo.Info)
	err = bencode.Unmarshal(metadata, info)
	if err != nil {
		zap.L().Debug(
			"Couldn't unmarshal info bytes!",
			zap.ByteString("infoHash", infoHash[:]),
			zap.Error(err),
		)
		return
	}
	err = validateInfo(info)
	if err != nil {
		zap.L().Debug(
			"Bad info dictionary!",
			zap.ByteString("infoHash", infoHash[:]),
			zap.Error(err),
		)
		return
	}

	var files []persistence.File
	for _, file := range info.Files {
		if file.Length < 0 {
			zap.L().Debug(
				"File size is less than zero!",
				zap.ByteString("infoHash", infoHash[:]),
				zap.String("filePath", file.DisplayPath(info)),
				zap.Int64("fileSize", file.Length),
			)
			return
		}

		files = append(files, persistence.File{
			Size: file.Length,
			Path: file.DisplayPath(info),
		})
	}

	var totalSize uint64
	for _, file := range files {
		totalSize += uint64(file.Size)
	}

	zap.L().Debug(
		"Flushing metadata...",
		zap.ByteString("infoHash", infoHash[:]),
	)

	ms.flush(Metadata{
		InfoHash:     infoHash[:],
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

func writeAll(c *net.TCPConn, b []byte) error {
	for len(b) != 0 {
		n, err := c.Write(b)
		if err != nil {
			return err
		}
		b = b[n:]
	}
	return nil
}

func readExactly(c *net.TCPConn, n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := io.ReadFull(c, b)
	return b, err
}

// TODO: add bounds checking!
func intToBigEndian(i int, n int) []byte {
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

func bigEndianToInt(b []byte) int {
	switch len(b) {
	case 1:
		return int(b[0])

	case 2:
		return int(binary.BigEndian.Uint16(b))

	case 4:
		return int(binary.BigEndian.Uint32(b))

	default:
		panic(fmt.Sprintf("bigEndianToInt: b is too long! (%d bytes)", len(b)))
	}
}
