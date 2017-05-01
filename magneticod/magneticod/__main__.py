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
import argparse
import collections
import functools
import logging
import ipaddress
import selectors
import textwrap
import urllib.parse
import itertools
import os
import sys
import time
import typing

import appdirs
import humanfriendly

from .constants import TICK_INTERVAL, MAX_ACTIVE_PEERS_PER_INFO_HASH, DEFAULT_MAX_METADATA_SIZE
from . import __version__
from . import bittorrent
from . import dht
from . import persistence


# Global variables are bad bla bla bla, BUT these variables are used so many times that I think it is justified; else
# the signatures of many functions are literally cluttered.
#
# If you are using a global variable, please always indicate that at the VERY BEGINNING of the function instead of right
# before using the variable for the first time.
selector = selectors.DefaultSelector()
database = None  # type: persistence.Database
node = None
peers = collections.defaultdict(list)  # type: typing.DefaultDict[dht.InfoHash, typing.List[bittorrent.DisposablePeer]]
# info hashes whose metadata is valid & complete (OR complete but deemed to be corrupt) so do NOT download them again:
complete_info_hashes = set()


def main():
    global complete_info_hashes, database, node, peers, selector

    arguments = parse_cmdline_arguments()

    logging.basicConfig(level=arguments.loglevel, format="%(asctime)s  %(levelname)-8s  %(message)s")
    logging.info("magneticod v%d.%d.%d started", *__version__)

    # noinspection PyBroadException
    try:
        path = arguments.database_file
        database = persistence.Database(path)
    except:
        logging.exception("could NOT connect to the database!")
        return 1

    complete_info_hashes = database.get_complete_info_hashes()

    node = dht.SybilNode(arguments.node_addr)

    node.when_peer_found = lambda info_hash, peer_address: on_peer_found(info_hash=info_hash,
                                                                         peer_address=peer_address,
                                                                         max_metadata_size=arguments.max_metadata_size)

    selector.register(node, selectors.EVENT_READ)

    try:
        loop()
    except KeyboardInterrupt:
        logging.critical("Keyboard interrupt received! Exiting gracefully...")
        pass
    finally:
        database.close()
        selector.close()
        node.shutdown()
        for peer in itertools.chain.from_iterable(peers.values()):
            peer.shutdown()

    return 0


def on_peer_found(info_hash: dht.InfoHash, peer_address, max_metadata_size: int=DEFAULT_MAX_METADATA_SIZE) -> None:
    global selector, peers, complete_info_hashes

    if len(peers[info_hash]) > MAX_ACTIVE_PEERS_PER_INFO_HASH or info_hash in complete_info_hashes:
        return

    try:
        peer = bittorrent.DisposablePeer(info_hash, peer_address, max_metadata_size)
    except ConnectionError:
        return

    selector.register(peer, selectors.EVENT_READ | selectors.EVENT_WRITE)
    peer.when_metadata_found = on_metadata_found
    peer.when_error = functools.partial(on_peer_error, peer, info_hash)
    peers[info_hash].append(peer)


def on_metadata_found(info_hash: dht.InfoHash, metadata: bytes) -> None:
    global complete_info_hashes, database, peers, selector

    succeeded = database.add_metadata(info_hash, metadata)
    if not succeeded:
        logging.info("Corrupt metadata for %s! Ignoring.", info_hash.hex())

    # When we fetch the metadata of an info hash completely, shut down all other peers who are trying to do the same.
    for peer in peers[info_hash]:
        selector.unregister(peer)
        peer.shutdown()
    del peers[info_hash]

    complete_info_hashes.add(info_hash)


def on_peer_error(peer: bittorrent.DisposablePeer, info_hash: dht.InfoHash) -> None:
    global peers, selector
    peer.shutdown()
    peers[info_hash].remove(peer)
    selector.unregister(peer)


# TODO:
# Consider whether time.monotonic() is a good choice. Maybe we should use CLOCK_MONOTONIC_RAW as its not affected by NTP
# adjustments, and all we need is how many seconds passed since a certain point in time.
def loop() -> None:
    global selector, node, peers

    t0 = time.monotonic()
    while True:
        keys_and_events = selector.select(timeout=TICK_INTERVAL)

        # Check if it is time to tick
        delta = time.monotonic() - t0
        if delta >= TICK_INTERVAL:
            if not (delta < 2 * TICK_INTERVAL):
                logging.warning("Belated TICK! (Δ = %d)", delta)

            node.on_tick()
            for peer_list in peers.values():
                for peer in peer_list:
                    peer.on_tick()

            t0 = time.monotonic()

        for key, events in keys_and_events:
            if events & selectors.EVENT_READ:
                key.fileobj.on_receivable()
            if events & selectors.EVENT_WRITE:
                key.fileobj.on_sendable()

        # Check for entities that would like to write to their socket
        keymap = selector.get_map()
        for fd in keymap:
            fileobj = keymap[fd].fileobj
            if fileobj.would_send():
                selector.modify(fileobj, selectors.EVENT_READ | selectors.EVENT_WRITE)
            else:
                selector.modify(fileobj, selectors.EVENT_READ)


def parse_ip_port(netloc) -> typing.Optional[typing.Tuple[str, int]]:
    # In case no port supplied
    try:
        return str(ipaddress.ip_address(netloc)), 0
    except ValueError:
        pass

    # In case port supplied
    try:
        parsed = urllib.parse.urlparse("//{}".format(netloc))
        ip = str(ipaddress.ip_address(parsed.hostname))
        port = parsed.port
        if port is None:
            raise argparse.ArgumentParser("Invalid node address supplied!")
    except ValueError:
        raise argparse.ArgumentParser("Invalid node address supplied!")

    return ip, port


def parse_size(value: str) -> int:
    try:
        return humanfriendly.parse_size(value)
    except humanfriendly.InvalidSize as e:
        raise argparse.ArgumentTypeError("Invalid argument. {}".format(e))


def parse_cmdline_arguments() -> typing.Optional[argparse.Namespace]:
    parser = argparse.ArgumentParser(
        description="Autonomous BitTorrent DHT crawler and metadata fetcher.",
        epilog=textwrap.dedent("""\
            Copyright (C) 2017  Mert Bora ALPER <bora@boramalper.org>
            Dedicated to Cemile Binay, in whose hands I thrived.

            This program is free software: you can redistribute it and/or modify it under
            the terms of the GNU Affero General Public License as published by the Free
            Software Foundation, either version 3 of the License, or (at your option) any
            later version.

            This program is distributed in the hope that it will be useful, but WITHOUT ANY
            WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
            PARTICULAR PURPOSE.  See the GNU Affero General Public License for more
            details.

            You should have received a copy of the GNU Affero General Public License along
            with this program.  If not, see <http://www.gnu.org/licenses/>.
        """),
        allow_abbrev=False,
        formatter_class=argparse.RawDescriptionHelpFormatter
    )

    parser.add_argument(
        "--node-addr", action="store", type=parse_ip_port, required=False, default="0.0.0.0:0",
        help="the address of the (DHT) node magneticod will use"
    )

    parser.add_argument(
        "--max-metadata-size", type=parse_size, default=DEFAULT_MAX_METADATA_SIZE,
        help="Limit metadata size to protect memory overflow. Provide in human friendly format eg. 1 M, 1 GB"
    )

    default_database_dir = os.path.join(appdirs.user_data_dir("magneticod"), "database.sqlite3")
    parser.add_argument(
        "--database-file", type=str, default=default_database_dir,
        help="Path to database file (default: {})".format(humanfriendly.format_path(default_database_dir))
    )
    parser.add_argument(
        '-d', '--debug',
        action="store_const", dest="loglevel", const=logging.DEBUG, default=logging.INFO,
        help="Print debugging information in addition to normal processing.",
    )
    return parser.parse_args(sys.argv[1:])


if __name__ == "__main__":
    sys.exit(main())
