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

"""
Warning:
    Encoders do NOT check for circular objects! (and will NEVER check due to speed concerns).
"""

import typing
from io import BytesIO

import better_bencode

Types = typing.Union[int, bytes, list, "KRPCDict"]
KRPCDict = typing.Dict[bytes, Types]


def dumps(obj) -> bytes:
    try:
        return better_bencode.dumps(obj)
    except:
        raise BencodeEncodingError()


def loads(bytes_object: bytes) -> Types:
    try:
        return better_bencode.loads(bytes_object)
    except Exception as exc:
        raise BencodeDecodingError(exc)


def loads2(bytes_object: bytes) -> typing.Tuple[Types, int]:
    """
    Returns the bencoded object AND the index where the dump of the decoded object ends (exclusive). In less words:

        dump = b"i12eOH YEAH"
        object, i = loads2(dump)
        print(">>>", dump[i:])  # OUTPUT: >>> b'OH YEAH'
    """
    bio = BytesIO(bytes_object)
    try:
        return better_bencode.load(bio), bio.tell()
    except Exception as exc:
        raise BencodeDecodingError(exc)


class BencodeEncodingError(Exception):
    pass


class BencodeDecodingError(Exception):
    def __init__(self, original_exc):
        super().__init__()
        self.original_exc = original_exc
