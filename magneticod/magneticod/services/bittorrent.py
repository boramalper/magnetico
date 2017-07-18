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
import logging

from ..protocols import bittorrent as bt_protocol
from magneticod import types


class MetadataService:
    def __init__(self, peer_id: types.PeerID, info_hash: types.InfoHash):
        self._protocol = bt_protocol.Protocol()

        self._protocol.on_the_bittorrent_handshake_completed = self._when_the_bittorrent_handshake_completed
        self._protocol.on_extension_handshake_completed = self._when_extension_handshake_completed

        self._peer_id = peer_id
        self._info_hash = info_hash

    async def launch(self) -> None:
        await self._protocol.launch()

        self._protocol.initiate_the_bittorrent_handshake(
            bt_protocol.reserved_feature_set_to_reserved({
                bt_protocol.ReservedFeature.EXTENSION_PROTOCOL,
                bt_protocol.ReservedFeature.DHT
            }),
            self._info_hash,
            self._peer_id
        )

    # Offered Functionality
    # =====================
    @staticmethod
    def on_fatal_failure() -> None:
        pass

    # Private Functionality
    # =====================
    def _when_the_bittorrent_handshake_completed(
        self,
        reserved: bytes,
        info_hash: types.InfoHash,
        peer_id: types.PeerID
    ) -> None:
        if bt_protocol.ReservedFeature.EXTENSION_PROTOCOL not in bt_protocol.reserved_to_reserved_feature_set(reserved):
            logging.info("Peer does NOT support the extension protocol.")
            self.on_fatal_failure()
        self._protocol.send_extension_handshake({bt_protocol.ExtensionType.UT_METADATA})

    def _when_extension_handshake_completed(self, payload: types.Dictionary) -> None:
        pass
