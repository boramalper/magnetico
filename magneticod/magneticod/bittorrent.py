# magneticod - Autonomous BitTorrent DHT crawler and metadata fetcher.
# Copyright (C) 2017  Mert Bora ALPER <bora@boramalper.org>
# Dedicated to Cemile Binay, in whose hands I thrived.
#
# This program is free software: you can redistribute it and/or modify it under the terms of the GNU Affero General
# Public License as published by the Free Software Foundation, either version 3 of the License, or (at your option) any
# later version.
#
# This program is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY; without even the implied
# warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU Affero General Public License for more
# details.
#
# You should have received a copy of the GNU Affero General Public License along with this program.  If not, see
# <http://www.gnu.org/licenses/>.
import errno
import logging
import hashlib
import math
import socket
import typing
import os

from . import bencode
from .constants import DEFAULT_MAX_METADATA_SIZE

InfoHash = bytes
PeerAddress = typing.Tuple[str, int]


class DisposablePeer:
    def __init__(self, info_hash: InfoHash, peer_addr: PeerAddress, max_metadata_size: int= DEFAULT_MAX_METADATA_SIZE):
        self.__socket = socket.socket()
        self.__socket.setblocking(False)
        # To reduce the latency:
        self.__socket.setsockopt(socket.IPPROTO_TCP, socket.TCP_NODELAY, True)
        if hasattr(socket, "TCP_QUICKACK"):
            self.__socket.setsockopt(socket.IPPROTO_TCP, socket.TCP_QUICKACK, True)

        res = self.__socket.connect_ex(peer_addr)
        if res != errno.EINPROGRESS:
            raise ConnectionError()

        self.__peer_addr = peer_addr
        self.__info_hash = info_hash

        self.__max_metadata_size = max_metadata_size

        self.__incoming_buffer = bytearray()
        self.__outgoing_buffer = bytearray()

        self.__bt_handshake_complete = False  # BitTorrent Handshake
        self.__ext_handshake_complete = False  # Extension Handshake
        self.__ut_metadata = None  # Since we don't know ut_metadata code that remote peer uses...

        self.__metadata_size = None
        self.__metadata_received = 0  # Amount of metadata bytes received...
        self.__metadata = None

        # To prevent double shutdown
        self.__shutdown = False

        # After 120 ticks passed, a peer should report an error and shut itself down due to being stall.
        self.__ticks_passed = 0

        # Send the BitTorrent handshake message (0x13 = 19 in decimal, the length of the handshake message)
        self.__outgoing_buffer += b"\x13BitTorrent protocol%s%s%s" % (
            b"\x00\x00\x00\x00\x00\x10\x00\x01",
            self.__info_hash,
            self.__random_bytes(20)
        )

    @staticmethod
    def when_error() -> None:
        raise NotImplementedError()

    @staticmethod
    def when_metadata_found(info_hash: InfoHash, metadata: bytes) -> None:
        raise NotImplementedError()

    def on_tick(self):
        self.__ticks_passed += 1

        if self.__ticks_passed == 120:
            logging.debug("Peer failed to fetch metadata in time for info hash %s!", self.__info_hash.hex())
            self.when_error()

    def on_receivable(self) -> None:
        while True:
            try:
                received = self.__socket.recv(8192)
            except BlockingIOError:
                break
            except ConnectionResetError:
                self.when_error()
                return
            except ConnectionRefusedError:
                self.when_error()
                return
            except OSError:  # TODO: check for "no route to host 113" error
                self.when_error()
                return

            if not received:
                self.when_error()
                return

            self.__incoming_buffer += received
        # Honestly speaking, BitTorrent protocol might be one of the most poorly documented and (not the most but) badly
        # designed protocols I have ever seen (I am 19 years old so what I could have seen?).
        #
        # Anyway, all the messages EXCEPT the handshake are length-prefixed by 4 bytes in network order, BUT the
        # size of the handshake message is the 1-byte length prefix + 49 bytes, but luckily, there is only one canonical
        # way of handshaking in the wild.
        if not self.__bt_handshake_complete:
            if len(self.__incoming_buffer) < 68:
                # We are still receiving the handshake...
                return

            if self.__incoming_buffer[1:20] != b"BitTorrent protocol":
                # Erroneous handshake, possibly unknown version...
                logging.debug("Erroneous BitTorrent handshake!  %s", self.__incoming_buffer[:68])
                self.when_error()
                return

            self.__on_bt_handshake(self.__incoming_buffer[:68])

            self.__bt_handshake_complete = True
            self.__incoming_buffer = self.__incoming_buffer[68:]

        while len(self.__incoming_buffer) >= 4:
            # Beware that while there are still messages in the incoming queue/buffer, one of previous messages might
            # have caused an error that necessitates us to quit.
            if self.__shutdown:
                break

            length = int.from_bytes(self.__incoming_buffer[:4], "big")
            if len(self.__incoming_buffer) - 4 < length:
                # Message is still incoming...
                return

            self.__on_message(self.__incoming_buffer[4:4+length])
            self.__incoming_buffer = self.__incoming_buffer[4+length:]

    def on_sendable(self) -> None:
        while self.__outgoing_buffer:
            try:
                n_sent = self.__socket.send(self.__outgoing_buffer)
                assert n_sent
                self.__outgoing_buffer = self.__outgoing_buffer[n_sent:]
            except BlockingIOError:
                break
            except OSError:
                # In case -while looping- on_sendable is called after socket is closed (mostly because of an error)
                return

    def __on_message(self, message: bytes) -> None:
        length = len(message)

        if length < 2:
            # An extension message has minimum length of 2.
            return

        # Every extension message has BitTorrent Message ID = 20
        if message[0] != 20:
            # logging.debug("Message is NOT an EXTension message!  %s", message[:200])
            return

        # Extension Handshake has the Extension Message ID = 0
        if message[1] == 0:
            self.__on_ext_handshake_message(message[2:])
            return

        # ut_metadata extension messages has the Extension Message ID = 1  (as we arbitrarily decided!)
        if message[1] != 1:
            logging.debug("Message is NOT an ut_metadata message!  %s", message[:200])
            return

        # Okay, now we are -almost- sure that this is an extension message, a kind we are most likely interested in...
        self.__on_ext_message(message[2:])

    def __on_bt_handshake(self, message: bytes) -> None:
        """ on BitTorrent Handshake... send the extension handshake! """
        if message[25] != 16:
            logging.info("Peer does NOT support the extension protocol")

        msg_dict_dump = bencode.dumps({
            b"m": {
                b"ut_metadata": 1
            }
        })
        # In case you cannot read_file hex:
        #   0x14 = 20  (BitTorrent ID indicating that it's an extended message)
        #   0x00 =  0  (Extension ID indicating that it's the handshake message)
        self.__outgoing_buffer += b"%s\x14\x00%s" % (
            (2 + len(msg_dict_dump)).to_bytes(4, "big"),
            msg_dict_dump
        )

    def __on_ext_handshake_message(self, message: bytes) -> None:
        if self.__ext_handshake_complete:
            return

        try:
            msg_dict = bencode.loads(bytes(message))
        except bencode.BencodeDecodingError:
            # One might be tempted to close the connection, but why care? Any DisposableNode will be disposed
            # automatically anyway (after a certain amount of time if the metadata is still not complete).
            logging.debug("Could NOT decode extension handshake message! %s", message[:200])
            return

        try:
            # Just to make sure that the remote peer supports ut_metadata extension:
            ut_metadata = msg_dict[b"m"][b"ut_metadata"]
            metadata_size = msg_dict[b"metadata_size"]
            assert metadata_size > 0, "Invalid (empty) metadata size"
            assert metadata_size < self.__max_metadata_size, "Malicious or malfunctioning peer {}:{} tried send above" \
                                                             " {} max metadata size".format(self.__peer_addr[0],
                                                                                            self.__peer_addr[1],
                                                                                            self.__max_metadata_size)
        except KeyError:
            self.when_error()
            return
        except AssertionError as e:
            logging.debug(str(e))
            self.when_error()
            return

        self.__ut_metadata = ut_metadata
        try:
            self.__metadata = bytearray(metadata_size)
        except MemoryError:
            logging.exception("Could not allocate %.1f KiB for the metadata!", metadata_size / 1024)
            self.when_error()
            return

        self.__metadata_size = metadata_size
        self.__ext_handshake_complete = True

        # After the handshake is complete, request all the pieces of metadata
        n_pieces = math.ceil(self.__metadata_size / (2 ** 14))
        for piece in range(n_pieces):
            self.__request_metadata_piece(piece)

    def __on_ext_message(self, message: bytes) -> None:
        try:
            msg_dict, i = bencode.loads2(bytes(message))
        except bencode.BencodeDecodingError:
            # One might be tempted to close the connection, but why care? Any DisposableNode will be disposed
            # automatically anyway (after a certain amount of time if the metadata is still not complete).
            logging.debug("Could NOT decode extension message!  %s", message[:200])
            return

        try:
            msg_type = msg_dict[b"msg_type"]
            piece = msg_dict[b"piece"]
        except KeyError:
            logging.debug("Missing EXT keys!  %s", msg_dict)
            return

        if msg_type == 1:  # data
            metadata_piece = message[i:]
            self.__metadata[piece * 2**14: piece * 2**14 + len(metadata_piece)] = metadata_piece
            self.__metadata_received += len(metadata_piece)

            # self.__metadata += metadata_piece

            # logging.debug("PIECE %d RECEIVED  %s", piece, metadata_piece[:200])

            if self.__metadata_received == self.__metadata_size:
                if hashlib.sha1(self.__metadata).digest() == self.__info_hash:
                    self.when_metadata_found(self.__info_hash, bytes(self.__metadata))
                else:
                    logging.debug("Invalid Metadata! Ignoring.")

        elif msg_type == 2:  # reject
            logging.info("Peer rejected us.")
            self.when_error()

    def __request_metadata_piece(self, piece: int) -> None:
        msg_dict_dump = bencode.dumps({
            b"msg_type": 0,
            b"piece": piece
        })
        # In case you cannot read_file hex:
        #   0x14 = 20  (BitTorrent ID indicating that it's an extended message)
        #   0x03 =  3  (Extension ID indicating that it's an ut_metadata message)
        self.__outgoing_buffer += b"%b\x14%s%s" % (
            (2 + len(msg_dict_dump)).to_bytes(4, "big"),
            self.__ut_metadata.to_bytes(1, "big"),
            msg_dict_dump
        )

    def shutdown(self) -> None:
        if self.__shutdown:
            return
        try:
            self.__socket.shutdown(socket.SHUT_RDWR)
        except OSError:
            # OSError might be raised in case the connection to the remote peer fails: nevertheless, when_error should
            # be called, and the supervisor will try to shutdown the peer, and ta da: OSError!
            pass
        self.__socket.close()
        self.__shutdown = True

    def would_send(self) -> bool:
        return bool(len(self.__outgoing_buffer))

    def fileno(self) -> int:
        return self.__socket.fileno()

    @staticmethod
    def __random_bytes(n: int) -> bytes:
        return os.urandom(n)
