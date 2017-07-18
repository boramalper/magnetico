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

from .dht import mainline
from . import bittorrent


class Coordinator:
    def __init__(self):
        self._peer_service = mainline.service.PeerService()

        self._metadata_service_tasks = {}

    async def run(self):
        await self._peer_service.launch(("0.0.0.0", 0))

    # Private Functionality
    # =====================
    def _when_peer(self, info_hash: mainline.protocol.InfoHash, address: mainline.transport.Address) \
    -> None:
        if info_hash in self._metadata_service_tasks:
            return

        self._metadata_service_tasks[info_hash] = asyncio.ensure_future()



    def _when_metadata(self, info_hash: mainline.protocol.InfoHash, address: mainline.transport.Address) -> None:
        pass
