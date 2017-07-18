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
import typing
import io

import better_bencode


Message = typing.Dict[bytes, typing.Any]


def encode(message: Message) -> bytes:
    try:
        return better_bencode.dumps(message)
    except Exception as exc:
        raise EncodeError(exc)


def decode(data: typing.ByteString) -> Message:
    try:
        return better_bencode.loads(data)
    except Exception as exc:
        raise DecodeError(exc)


def decode_prefix(data: typing.ByteString) -> typing.Tuple[Message, int]:
    """
    Returns the bencoded object AND the index where the dump of the decoded object ends (exclusive). In less words:

        dump = b"i12eOH YEAH"
        object, i = decode_prefix(dump)
        print(">>>", dump[i:])  # OUTPUT: >>> b'OH YEAH'
    """
    bio = io.BytesIO(data)
    try:
        return better_bencode.load(bio), bio.tell()
    except Exception as exc:
        raise DecodeError(exc)


class BaseCodecError(Exception):
    def __init__(self, original_exception: Exception):
        self.original_exception = original_exception


class EncodeError(BaseCodecError):
    pass


class DecodeError(BaseCodecError):
    pass

