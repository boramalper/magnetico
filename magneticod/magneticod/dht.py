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
import errno
import zlib
import logging
import socket
import typing
import os

from .constants import BOOTSTRAPPING_NODES, MAX_ACTIVE_PEERS_PER_INFO_HASH, PEER_TIMEOUT, TICK_INTERVAL
from . import bencode
from . import bittorrent

NodeID = bytes
NodeAddress = typing.Tuple[str, int]
PeerAddress = typing.Tuple[str, int]
InfoHash = bytes
Metadata = bytes


class SybilNode(asyncio.DatagramProtocol):
    def __init__(self, is_infohash_new, max_metadata_size):
        self.__true_id = os.urandom(20)

        self._routing_table = {}  # type: typing.Dict[NodeID, NodeAddress]

        self.__token_secret = os.urandom(4)
        # Maximum number of neighbours (this is a THRESHOLD where, once reached, the search for new neighbours will
        # stop; but until then, the total number of neighbours might exceed the threshold).
        self.__n_max_neighbours = 2000
        self.__parent_futures = {}  # type: typing.Dict[InfoHash, asyncio.Future]
        self._is_inforhash_new = is_infohash_new
        self.__max_metadata_size = max_metadata_size
        # Complete metadatas will be added to the queue, to be retrieved and committed to the database.
        self.__metadata_queue = asyncio.Queue()  # typing.Collection[typing.Tuple[InfoHash, Metadata]]
        self._is_writing_paused = False
        self._tick_task = None

        logging.info("SybilNode %s initialized!", self.__true_id.hex().upper())

    def metadata_q(self):
        return self.__metadata_queue

    async def launch(self, address):
        await asyncio.get_event_loop().create_datagram_endpoint(lambda: self, local_addr=address)
        logging.info("SybliNode is launched on %s!", address)

    def connection_made(self, transport: asyncio.DatagramTransport) -> None:
        self._tick_task = asyncio.get_event_loop().create_task(self.tick_periodically())
        self._transport = transport

    def connection_lost(self, exc) -> None:
        logging.critical("SybilNode's connection is lost.")
        self._is_writing_paused = True

    def pause_writing(self) -> None:
        self._is_writing_paused = True
        # In case of congestion, decrease the maximum number of nodes to the 90% of the current value.
        self.__n_max_neighbours = self.__n_max_neighbours * 9 // 10
        logging.debug("Maximum number of neighbours now %d", self.__n_max_neighbours)

    def resume_writing(self) -> None:
        self._is_writing_paused = False

    def sendto(self, data, addr) -> None:
        if not self._is_writing_paused:
            self._transport.sendto(data, addr)

    def error_received(self, exc: Exception) -> None:
        if isinstance(exc, PermissionError) or (isinstance(exc, OSError) and exc.errno == errno.ENOBUFS):
            # This exception (EPERM errno: 1) is kernel's way of saying that "you are far too fast, chill".
            # It is also likely that we have received a ICMP source quench packet (meaning, that we really need to
            # slow down.
            #
            # Read more here: http://www.archivum.info/comp.protocols.tcp-ip/2009-05/00088/UDP-socket-amp-amp-sendto
            #                 -amp-amp-EPERM.html

            # > Note On BSD systems (OS X, FreeBSD, etc.) flow control is not supported for DatagramProtocol, because
            # > send failures caused by writing too many packets cannot be detected easily. The socket always appears
            # > ‘ready’ and excess packets are dropped; an OSError with errno set to errno.ENOBUFS may or may not be
            # > raised; if it is raised, it will be reported to DatagramProtocol.error_received() but otherwise ignored.
            # Source: https://docs.python.org/3/library/asyncio-protocol.html#flow-control-callbacks

            # In case of congestion, decrease the maximum number of nodes to the 90% of the current value.
            if self.__n_max_neighbours < 200:
                logging.warning("Max. number of neighbours are < 200 and there is still congestion! (check your network "
                                "connection if this message recurs)")
            else:
                self.__n_max_neighbours = self.__n_max_neighbours * 9 // 10
                logging.debug("Maximum number of neighbours now %d", self.__n_max_neighbours)
        else:
            # The previous "exception" was kind of "unexceptional", but we should log anything else.
            logging.error("SybilNode operational error: `%s`", exc)

    async def tick_periodically(self) -> None:
        while True:
            await asyncio.sleep(TICK_INTERVAL)
            # Bootstrap (by querying the bootstrapping servers) ONLY IF the routing table is empty (i.e. we don't have
            # any neighbours). Otherwise we'll increase the load on those central servers by querying them every second.
            if not self._routing_table:
                await self.__bootstrap()
            self.__make_neighbours()
            self._routing_table.clear()
            if not self._is_writing_paused:
                self.__n_max_neighbours = self.__n_max_neighbours * 101 // 100
            logging.debug("fetch metadata task count: %d", sum(
                x.child_count for x in self.__parent_futures.values()))
            logging.debug("asyncio task count: %d", len(asyncio.Task.all_tasks()))

    def datagram_received(self, data, addr) -> None:
        # Ignore nodes that "uses" port 0, as we cannot communicate with them reliably across the different systems.
        # See https://tools.cisco.com/security/center/viewAlert.x?alertId=19935 for slightly more details
        if addr[1] == 0:
            return

        if self._transport.is_closing():
            return

        try:
            message = bencode.loads(data)
        except bencode.BencodeDecodingError:
            return

        if isinstance(message.get(b"r"), dict) and type(message[b"r"].get(b"nodes")) is bytes:
            self.__on_FIND_NODE_response(message)
        elif message.get(b"q") == b"get_peers":
            self.__on_GET_PEERS_query(message, addr)
        elif message.get(b"q") == b"announce_peer":
            self.__on_ANNOUNCE_PEER_query(message, addr)

    async def shutdown(self) -> None:
        parent_futures = list(self.__parent_futures.values())
        for pf in parent_futures:
            pf.set_result(None)
        self._tick_task.cancel()
        await asyncio.wait([self._tick_task])
        self._transport.close()

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

        # Ignore nodes with port 0.
        nodes = [n for n in nodes if n[1][1] != 0]

        # Add new found nodes to the routing table, assuring that we have no more than n_max_neighbours in total.
        if len(self._routing_table) < self.__n_max_neighbours:
            self._routing_table.update(nodes[:self.__n_max_neighbours - len(self._routing_table)])

    def __on_GET_PEERS_query(self, message: bencode.KRPCDict, addr: NodeAddress) -> None:
        try:
            transaction_id = message[b"t"]
            assert type(transaction_id) is bytes and transaction_id
            info_hash = message[b"a"][b"info_hash"]
            assert type(info_hash) is bytes and len(info_hash) == 20
        except (TypeError, KeyError, AssertionError):
            return

        data = self.__build_GET_PEERS_query(
            info_hash[:15] + self.__true_id[:5], transaction_id, self.__calculate_token(addr, info_hash)
        )

        # TODO:
        # We would like to prioritise GET_PEERS responses as they are the most fruitful ones, i.e., that leads to the
        # discovery of an info hash & metadata! But there is no easy way to do this with asyncio...
        # Maybe use priority queues to prioritise certain messages and let them accumulate, and dispatch them to the
        # transport at every tick?
        self.sendto(data, addr)

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

        data = self.__build_ANNOUNCE_PEER_query(node_id[:15] + self.__true_id[:5], transaction_id)
        self.sendto(data, addr)

        if implied_port:
            peer_addr = (addr[0], addr[1])
        else:
            peer_addr = (addr[0], port)

        if not self._is_inforhash_new(info_hash):
            return

        event_loop = asyncio.get_event_loop()

        # A little clarification about parent and child futures might be really useful here:
        # For every info hash we are interested in, we create ONE parent future and save it under self.__tasks
        # (info_hash -> task) dictionary.
        # For EVERY DisposablePeer working to fetch the metadata of that info hash, we create a child future. Hence, for
        # every parent future, there should be *at least* one child future.
        #
        # Parent and child futures are "connected" to each other through `add_done_callback` functionality:
        #     When a child is successfully done, it sets the result of its parent (`set_result()`), and if it was
        #   unsuccessful to fetch the metadata, it just checks whether there are any other child futures left and if not
        #   it terminates the parent future (by setting its result to None) and quits.
        #     When a parent future is successfully done, (through the callback) it adds the info hash to the set of
        #   completed metadatas and puts the metadata in the queue to be committed to the database.

        # create the parent future
        if info_hash not in self.__parent_futures:
            parent_f = event_loop.create_future()
            parent_f.child_count = 0
            parent_f.add_done_callback(lambda f: self._parent_task_done(f, info_hash))
            self.__parent_futures[info_hash] = parent_f

        parent_f = self.__parent_futures[info_hash]

        if parent_f.done():
            return
        if parent_f.child_count > MAX_ACTIVE_PEERS_PER_INFO_HASH:
            return

        task = asyncio.ensure_future(bittorrent.fetch_metadata_from_peer(
            info_hash, peer_addr, self.__max_metadata_size, timeout=PEER_TIMEOUT))
        task.add_done_callback(lambda task: self._got_child_result(parent_f, task))
        parent_f.child_count += 1
        parent_f.add_done_callback(lambda f: task.cancel())

    def _got_child_result(self, parent_task, child_task):
        parent_task.child_count -= 1
        try:
            metadata = child_task.result()
            # Bora asked:
            #     Why do we check for parent_task being done here when a child got result? I mean, if parent_task is
            #     done before, and successful, all of its childs will be terminated and this function cannot be called
            #     anyway.
            #
            # --- https://github.com/boramalper/magnetico/pull/76#discussion_r119555423
            #
            #     Suppose two child tasks are fetching the same metadata for a parent and they finish at the same time
            #     (or very close). The first one wakes up, sets the parent_task result which will cause the done
            #     callback to be scheduled. The scheduler might still then chooses the second child task to run next
            #     (why not? It's been waiting longer) before the parent has a chance to cancel it.
            #
            # Thus spoke Richard.
            if metadata and not parent_task.done():
                parent_task.set_result(metadata)
        except asyncio.CancelledError:
            pass
        except Exception:
            logging.exception("child result is exception")
        if parent_task.child_count <= 0 and not parent_task.done():
            parent_task.set_result(None)

    def _parent_task_done(self, parent_task, info_hash):
        try:
            metadata = parent_task.result()
            if metadata:
                self.__metadata_queue.put_nowait((info_hash, metadata))
        except asyncio.CancelledError:
            pass
        del self.__parent_futures[info_hash]

    async def __bootstrap(self) -> None:
        event_loop = asyncio.get_event_loop()
        for node in BOOTSTRAPPING_NODES:
            try:
                # AF_INET means ip4 only
                responses = await event_loop.getaddrinfo(*node, family=socket.AF_INET)
                for (family, type, proto, canonname, sockaddr) in responses:
                    data = self.__build_FIND_NODE_query(self.__true_id)
                    self.sendto(data, sockaddr)
            except Exception:
                logging.exception("An exception occurred during bootstrapping!")

    def __make_neighbours(self) -> None:
        for node_id, addr in self._routing_table.items():
            self.sendto(self.__build_FIND_NODE_query(node_id[:15] + self.__true_id[:5]), addr)

    @staticmethod
    def __decode_nodes(infos: bytes) -> typing.List[typing.Tuple[NodeID, NodeAddress]]:
        """ Reference Implementation:
        nodes = []
        for i in range(0, len(infos), 26):
            info = infos[i: i + 26]
            node_id = info[:20]
            node_host = socket.inet_ntoa(info[20:24])
            node_port = int.from_bytes(info[24:], "big")
            nodes.append((node_id, (node_host, node_port)))
        return nodes
        """
        """ Optimized Version: """
        # Because dot-access also has a cost
        inet_ntoa = socket.inet_ntoa
        int_from_bytes = int.from_bytes
        return [
            (infos[i:i+20], (inet_ntoa(infos[i+20:i+24]), int_from_bytes(infos[i+24:i+26], "big")))
            for i in range(0, len(infos), 26)
        ]

    def __calculate_token(self, addr: NodeAddress, info_hash: InfoHash) -> bytes:
        # Believe it or not, faster than using built-in hash (including conversion from int -> bytes of course)
        checksum = zlib.adler32(b"%s%s%d%s" % (self.__token_secret, socket.inet_aton(addr[0]), addr[1], info_hash))
        return checksum.to_bytes(4, "big")

    @staticmethod
    def __build_FIND_NODE_query(id_: bytes) -> bytes:
        """ Reference Implementation:
        bencode.dumps({
            b"y": b"q",
            b"q": b"find_node",
            b"t": b"aa",
            b"a": {
                b"id": id_,
                b"target": self.__random_bytes(20)
            }
        })
        """
        """ Optimized Version: """
        return b"d1:ad2:id20:%s6:target20:%se1:q9:find_node1:t2:aa1:y1:qe" % (
            id_,
            os.urandom(20)
        )

    @staticmethod
    def __build_GET_PEERS_query(id_: bytes, transaction_id: bytes, token: bytes) -> bytes:
        """ Reference Implementation:

        bencode.dumps({
            b"y": b"r",
            b"t": transaction_id,
            b"r": {
                b"id": info_hash[:15] + self.__true_id[:5],
                b"nodes": b"",
                b"token": self.__calculate_token(addr, info_hash)
            }
        })
        """
        """ Optimized Version: """
        return b"d1:rd2:id20:%s5:nodes0:5:token%d:%se1:t%d:%s1:y1:re" % (
            id_, len(token), token, len(transaction_id), transaction_id
        )

    @staticmethod
    def __build_ANNOUNCE_PEER_query(id_: bytes, transaction_id: bytes) -> bytes:
        """ Reference Implementation:

        bencode.dumps({
            b"y": b"r",
            b"t": transaction_id,
            b"r": {
                b"id": node_id[:15] + self.__true_id[:5]
            }
        })
        """
        """ Optimized Version: """
        return b"d1:rd2:id20:%se1:t%d:%s1:y1:re" % (id_, len(transaction_id), transaction_id)