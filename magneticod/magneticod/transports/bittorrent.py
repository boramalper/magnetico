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
import typing

from magneticod import types


class TCPTransport(asyncio.Protocol):
    def __init__(self):
        self._stream_transport = asyncio.Transport()

        self._incoming = bytearray()

        self._awaiting_the_bittorrent_handshake = True

    async def launch(self):
        await asyncio.get_event_loop().create_connection(lambda: self, "0.0.0.0", 0)

    # Offered Functionality
    # =====================
    def initiate_the_bittorrent_handshake(self, reserved: bytes, info_hash: types.InfoHash, peer_id: types.PeerID) -> None:
        self._stream_transport.write(b"\x13BitTorrent protocol%s%s%s" % (
            reserved,
            info_hash,
            peer_id
        ))

    def send_keepalive(self) -> None:
        self._stream_transport.write(b"\x00\x00\x00\x00")

    def send_message(self, type_: bytes, payload: typing.ByteString) -> None:
        if len(type_) != 1:
            raise ValueError("Argument `type_` must be a single byte!")
        length = 1 + len(payload)

    @staticmethod
    def on_keepalive() -> None:
        pass

    @staticmethod
    def on_message(type_: bytes, payload: typing.ByteString) -> None:
        pass

    @staticmethod
    def on_the_bittorrent_handshake_completed(
            reserved: typing.ByteString,
            info_hash: types.InfoHash,
            peer_id: types.PeerID
    ) -> None:
        pass

    # Private Functionality
    # =====================
    def connection_made(self, transport: asyncio.Transport) -> None:
        self._stream_transport = transport

    def data_received(self, data: typing.ByteString) -> None:
        self._incoming += data

        if self._awaiting_the_bittorrent_handshake:
            if len(self._incoming) >= 68:
                assert self._incoming.startswith(b"\x13BitTorrent protocol")
                self.on_the_bittorrent_handshake_completed(
                    reserved=self._incoming[20:28],
                    info_hash=self._incoming[28:48],
                    peer_id=self._incoming[48:68]
                )
                self._incoming = self._incoming[68:]
                self._awaiting_the_bittorrent_handshake = False
            else:
                return

        # Continue or Start the "usual" processing from here below

        if len(self._incoming) >= 4 and len(self._incoming) - 1 >= int.from_bytes(self._incoming[:4], "big"):
            if int.from_bytes(self._incoming[:4], "big"):
                self.on_keepalive()
            else:
                self.on_message(self._incoming[4], self._incoming[5:])

    def eof_received(self):
        pass

    def connection_lost(self, exc: Exception) -> None:
        pass


class UTPTransport:
    pass
