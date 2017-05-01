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
import array
import collections
import zlib
import logging
import socket
import typing
import os

from .constants import BOOTSTRAPPING_NODES, DEFAULT_MAX_METADATA_SIZE
from . import bencode

NodeID = bytes
NodeAddress = typing.Tuple[str, int]
PeerAddress = typing.Tuple[str, int]
InfoHash = bytes


class SybilNode:
    def __init__(self, address: typing.Tuple[str, int]):
        self.__true_id = self.__random_bytes(20)

        self.__socket = socket.socket(type=socket.SOCK_DGRAM)
        self.__socket.bind(address)
        self.__socket.setblocking(False)

        self.__incoming_buffer = array.array("B", (0 for _ in range(65536)))
        self.__outgoing_queue = collections.deque()

        self.__routing_table = {}  # type: typing.Dict[NodeID, NodeAddress]

        self.__token_secret = self.__random_bytes(4)
        # Maximum number of neighbours (this is a THRESHOLD where, once reached, the search for new neighbours will
        # stop; but until then, the total number of neighbours might exceed the threshold).
        self.__n_max_neighbours = 2000

        logging.info("SybilNode %s on %s initialized!", self.__true_id.hex().upper(), address)

    @staticmethod
    def when_peer_found(info_hash: InfoHash, peer_addr: PeerAddress) -> None:
        raise NotImplementedError()

    def on_tick(self) -> None:
        self.__bootstrap()
        self.__make_neighbours()
        self.__routing_table.clear()

    def on_receivable(self) -> None:
        buffer = self.__incoming_buffer
        while True:
            try:
                _, addr = self.__socket.recvfrom_into(buffer, 65536)
                data = buffer.tobytes()
            except BlockingIOError:
                break
            except ConnectionResetError:
                continue
            except ConnectionRefusedError:
                continue

            # Ignore nodes that uses port 0 (assholes).
            if addr[1] == 0:
                continue

            try:
                message = bencode.loads(data)
            except bencode.BencodeDecodingError:
                continue

            if isinstance(message.get(b"r"), dict) and type(message[b"r"].get(b"nodes")) is bytes:
                self.__on_FIND_NODE_response(message)
            elif message.get(b"q") == b"get_peers":
                self.__on_GET_PEERS_query(message, addr)
            elif message.get(b"q") == b"announce_peer":
                self.__on_ANNOUNCE_PEER_query(message, addr)

    def on_sendable(self) -> None:
        congestion = None
        while True:
            try:
                addr, data = self.__outgoing_queue.pop()
            except IndexError:
                break

            try:
                self.__socket.sendto(data, addr)
            except BlockingIOError:
                self.__outgoing_queue.appendleft((addr, data))
                break
            except PermissionError:
                # This exception (EPERM errno: 1) is kernel's way of saying that "you are far too fast, chill".
                # It is also likely that we have received a ICMP source quench packet (meaning, that we really need to
                # slow down.
                #
                # Read more here: http://www.archivum.info/comp.protocols.tcp-ip/2009-05/00088/UDP-socket-amp-amp-sendto
                # -amp-amp-EPERM.html
                congestion = True
                break
            except OSError:
                # Pass in case of trying to send to port 0 (it is much faster to catch exceptions than using an
                # if-statement).
                pass

        if congestion:
            self.__outgoing_queue.clear()
            # In case of congestion, decrease the maximum number of nodes to the 90% of the current value.
            if self.__n_max_neighbours < 200:
                logging.warning("Maximum number of neighbours are now less than 200 due to congestion!")
            else:
                self.__n_max_neighbours = self.__n_max_neighbours * 9 // 10
        else:
            # In case of the lack of congestion, increase the maximum number of nodes by 1%.
            self.__n_max_neighbours = self.__n_max_neighbours * 101 // 100

    def would_send(self) -> bool:
        """ Whether node is waiting to write on its socket or not. """
        return bool(self.__outgoing_queue)

    def shutdown(self) -> None:
        self.__socket.close()

    def __on_FIND_NODE_response(self, message: bencode.KRPCDict) -> None:
        try:
            nodes_arg = message[b"r"][b"nodes"]
            assert type(nodes_arg) is bytes and len(nodes_arg) % 26 == 0
        except (TypeError, KeyError, AssertionError):
            return

        try:
            nodes = self.__decode_nodes(nodes_arg)
        except AssertionError:
            return

        # Add new found nodes to the routing table, assuring that we have no more than n_max_neighbours in total.
        if len(self.__routing_table) < self.__n_max_neighbours:
            self.__routing_table.update(nodes)

    def __on_GET_PEERS_query(self, message: bencode.KRPCDict, addr: NodeAddress) -> None:
        try:
            transaction_id = message[b"t"]
            assert type(transaction_id) is bytes and transaction_id
            info_hash = message[b"a"][b"info_hash"]
            assert type(info_hash) is bytes and len(info_hash) == 20
        except (TypeError, KeyError, AssertionError):
            return

        # appendleft to prioritise GET_PEERS responses as they are the most fruitful ones!
        self.__outgoing_queue.appendleft((addr, bencode.dumps({
            b"y": b"r",
            b"t": transaction_id,
            b"r": {
                b"id": info_hash[:15] + self.__true_id[:5],
                b"nodes": b"",
                b"token": self.__calculate_token(addr, info_hash)
            }
        })))

    def __on_ANNOUNCE_PEER_query(self, message: bencode.KRPCDict, addr: NodeAddress) -> None:
        try:
            node_id = message[b"a"][b"id"]
            assert type(node_id) is bytes and len(node_id) == 20
            transaction_id = message[b"t"]
            assert type(transaction_id) is bytes and transaction_id
            token = message[b"a"][b"token"]
            assert type(token) is bytes
            info_hash = message[b"a"][b"info_hash"]
            assert type(info_hash) is bytes and len(info_hash) == 20
            if b"implied_port" in message[b"a"]:
                implied_port = message[b"a"][b"implied_port"]
                assert implied_port in (0, 1)
            else:
                implied_port = None
            port = message[b"a"][b"port"]

            assert type(port) is int and 0 < port < 65536
        except (TypeError, KeyError, AssertionError):
            return

        self.__outgoing_queue.append((addr, bencode.dumps({
            b"y": b"r",
            b"t": transaction_id,
            b"r": {
                b"id": node_id[:15] + self.__true_id[:5]
            }
        })))

        if implied_port:
            peer_addr = (addr[0], addr[1])
        else:
            peer_addr = (addr[0], port)

        self.when_peer_found(info_hash, peer_addr)

    def fileno(self) -> int:
        return self.__socket.fileno()

    def __bootstrap(self) -> None:
        for addr in BOOTSTRAPPING_NODES:
            self.__outgoing_queue.append((addr, self.__build_FIND_NODE_query(self.__true_id)))

    def __make_neighbours(self) -> None:
        for node_id, addr in self.__routing_table.items():
            self.__outgoing_queue.append((addr, self.__build_FIND_NODE_query(node_id[:15] + self.__true_id[:5])))

    @staticmethod
    def __decode_nodes(infos: bytes) -> typing.List[typing.Tuple[NodeID, NodeAddress]]:
        """ REFERENCE IMPLEMENTATION
        nodes = []
        for i in range(0, len(infos), 26):
            info = infos[i: i + 26]
            node_id = info[:20]
            node_host = socket.inet_ntoa(info[20:24])
            node_port = int.from_bytes(info[24:], "big")
            nodes.append((node_id, (node_host, node_port)))
        return nodes
        """

        """ Optimized Version """
        inet_ntoa = socket.inet_ntoa
        int_from_bytes = int.from_bytes
        return [
            (infos[i:i+20], (inet_ntoa(infos[i+20:i+24]), int_from_bytes(infos[i+24:i+26], "big")))
            for i in range(0, len(infos), 26)
        ]

    def __calculate_token(self, addr: NodeAddress, info_hash: InfoHash):
        # Believe it or not, faster than using built-in hash (including conversion from int -> bytes of course)
        return zlib.adler32(b"%s%s%d%s" % (self.__token_secret, socket.inet_aton(addr[0]), addr[1], info_hash))

    @staticmethod
    def __random_bytes(n: int) -> bytes:
        return os.urandom(n)

    def __build_FIND_NODE_query(self, id_: bytes) -> bytes:
        """ BENCODE IMPLEMENTATION
        bencode.dumps({
            b"y": b"q",
            b"q": b"find_node",
            b"t": self.__random_bytes(2),
            b"a": {
                b"id": id_,
                b"target": self.__random_bytes(20)
            }
        })
        """

        """ Optimized Version """
        return b"d1:ad2:id20:%s6:target20:%se1:q9:find_node1:t2:aa1:y1:qe" % (
            id_,
            self.__random_bytes(20)
        )
