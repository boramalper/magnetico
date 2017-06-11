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
import asyncio
import logging
import hashlib
import math
import typing
import os

from . import bencode

InfoHash = bytes
PeerAddress = typing.Tuple[str, int]


async def fetch_metadata_from_peer(info_hash: InfoHash, peer_addr: PeerAddress, max_metadata_size: int, timeout=None):
    try:
        return await asyncio.wait_for(DisposablePeer(info_hash, peer_addr, max_metadata_size).run(), timeout=timeout)
    except asyncio.TimeoutError:
        return None


class ProtocolError(Exception):
    pass


class DisposablePeer:
    def __init__(self, info_hash: InfoHash, peer_addr: PeerAddress, max_metadata_size: int) -> None:
        self.__peer_addr = peer_addr
        self.__info_hash = info_hash

        self.__ext_handshake_complete = False  # Extension Handshake
        self.__ut_metadata = None  # Since we don't know ut_metadata code that remote peer uses...

        self.__max_metadata_size = max_metadata_size
        self.__metadata_size = None
        self.__metadata_received = 0  # Amount of metadata bytes received...
        self.__metadata = None

        self._run_task = None
        self._writer = None


    async def run(self):
        event_loop = asyncio.get_event_loop()
        self._metadata_future = event_loop.create_future()

        try:
            self._reader, self._writer = await asyncio.open_connection(*self.__peer_addr, loop=event_loop)
            # Send the BitTorrent handshake message (0x13 = 19 in decimal, the length of the handshake message)
            self._writer.write(b"\x13BitTorrent protocol%s%s%s" % (
                b"\x00\x00\x00\x00\x00\x10\x00\x01",
                self.__info_hash,
                os.urandom(20)
            ))
            # Honestly speaking, BitTorrent protocol might be one of the most poorly documented and (not the most but)
            # badly designed protocols I have ever seen (I am 19 years old so what I could have seen?).
            #
            # Anyway, all the messages EXCEPT the handshake are length-prefixed by 4 bytes in network order, BUT the
            # size of the handshake message is the 1-byte length prefix + 49 bytes, but luckily, there is only one
            # canonical way of handshaking in the wild.
            message = await self._reader.readexactly(68)
            if message[1:20] != b"BitTorrent protocol":
                # Erroneous handshake, possibly unknown version...
                raise ProtocolError("Erroneous BitTorrent handshake!  %s" % message)

            self.__on_bt_handshake(message)

            while not self._metadata_future.done():
                buffer = await self._reader.readexactly(4)
                length = int.from_bytes(buffer, "big")
                message = await self._reader.readexactly(length)
                self.__on_message(message)
        except Exception:
            logging.debug("closing %s to %s", self.__info_hash.hex(), self.__peer_addr)
        finally:
            if not self._metadata_future.done():
                self._metadata_future.set_result(None)
            if self._writer:
                self._writer.close()
        return self._metadata_future.result()

    def __on_message(self, message: bytes) -> None:
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
        # In case you cannot read hex:
        #   0x14 = 20  (BitTorrent ID indicating that it's an extended message)
        #   0x00 =  0  (Extension ID indicating that it's the handshake message)
        self._writer.write(b"%b\x14%s" % (
            (2 + len(msg_dict_dump)).to_bytes(4, "big"),
            b'\0' + msg_dict_dump
        ))

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
        except AssertionError as e:
            logging.debug(str(e))
            raise

        self.__ut_metadata = ut_metadata
        try:
            self.__metadata = bytearray(metadata_size)
        except MemoryError:
            logging.exception("Could not allocate %.1f KiB for the metadata!", metadata_size / 1024)
            raise

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
                    if not self._metadata_future.done():
                        self._metadata_future.set_result(bytes(self.__metadata))
                else:
                    logging.debug("Invalid Metadata! Ignoring.")

        elif msg_type == 2:  # reject
            logging.info("Peer rejected us.")

    def __request_metadata_piece(self, piece: int) -> None:
        msg_dict_dump = bencode.dumps({
            b"msg_type": 0,
            b"piece": piece
        })
        # In case you cannot read_file hex:
        #   0x14 = 20  (BitTorrent ID indicating that it's an extended message)
        #   0x03 =  3  (Extension ID indicating that it's an ut_metadata message)
        self._writer.write(b"%b\x14%s%s" % (
            (2 + len(msg_dict_dump)).to_bytes(4, "big"),
            self.__ut_metadata.to_bytes(1, "big"),
            msg_dict_dump
        ))
