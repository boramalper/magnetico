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
import hashlib
import os
import socket
import typing

from magneticod import constants
from . import protocol


class PeerService:
    def __init__(self):
        self._protocol = protocol.Protocol(b"mc00")

        self._protocol.on_get_peers_query = self._when_get_peers_query
        self._protocol.on_announce_peer_query = self._when_announce_peer_query
        self._protocol.on_find_node_response = self._when_find_node_response

        self._true_node_id = os.urandom(20)
        self._token_secret = os.urandom(4)
        self._routing_table = {}  # typing.Dict[protocol.NodeID, protocol.transport.Address]

        self._tick_task = None

    async def launch(self, address: protocol.transport.Address):
        await self._protocol.launch(address)
        self._tick_task = asyncio.ensure_future(self._tick_periodically())

    # Offered Functionality
    # =====================
    @staticmethod
    def on_peer(info_hash: protocol.InfoHash, address: protocol.transport.Address) -> None:
        pass

    # Private Functionality
    # =====================
    async def _tick_periodically(self) -> None:
        while True:
            if not self._routing_table:
                await self._bootstrap()
            else:
                self._make_neighbors()
                self._routing_table.clear()
            await asyncio.sleep(constants.TICK_INTERVAL)

    async def _bootstrap(self) -> None:
        event_loop = asyncio.get_event_loop()
        for node in constants.BOOTSTRAPPING_NODES:
            for *_, address in await event_loop.getaddrinfo(*node, family=socket.AF_INET):
                self._protocol.make_query(protocol.FindNodeQuery(self._true_node_id, os.urandom(20)), address)

    def _make_neighbors(self) -> None:
        for id_, address in self._routing_table.items():
            self._protocol.make_query(
                protocol.FindNodeQuery(id_[:15] + self._true_node_id[:5], os.urandom(20)),
                address
            )

    def _when_get_peers_query(self, query: protocol.GetPeersQuery, address: protocol.transport.Address) \
    -> typing.Optional[typing.Union[protocol.GetPeersResponse, protocol.Error]]:
        return protocol.GetPeersResponse(query.info_hash[:15] + self._true_node_id[:5], self._calculate_token(address))

    def _when_announce_peer_query(self, query: protocol.AnnouncePeerQuery, address: protocol.transport.Address) \
    -> typing.Optional[typing.Union[protocol.AnnouncePeerResponse, protocol.Error]]:
        if query.implied_port:
            peer_address = (address[0], address[1])
        else:
            peer_address = (address[0], query.port)
        self.on_info_hash_and_peer(query.info_hash, peer_address)

        return protocol.AnnouncePeerResponse(query.info_hash[:15] + self._true_node_id[:5])

    def _when_find_node_response(self, response: protocol.FindNodeResponse, address: protocol.transport.Address) \
    -> None:
        self._routing_table.update({node.id: node.address for node in response.nodes if node.address != 0})

    def _calculate_token(self, address: protocol.transport.Address) -> bytes:
        return hashlib.sha1(b"%s%d" % (socket.inet_aton(address[0]), socket.htons(address[1]))).digest()
