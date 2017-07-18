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
import enum
import typing

import cerberus

from magneticod import types
from ..codecs import bencode
from .. import transports


class Protocol:
    def __init__(self):
        self._transport = transports.bittorrent.TCPTransport()

        self._transport.on_the_bittorrent_handshake_completed = self._when_the_bittorrent_handshake_completed
        self._transport.on_message = self._when_message
        self._transport.on_keepalive = self._when_keepalive

        # When we initiate extension handshake, we set the keys of this dictionary with the ExtensionType's we show
        # interest in, and if the remote peer is also interested in them, we record which type code the remote peer
        # prefers for them in this dictionary, but until then, the keys have the None value. If our interest for an
        # ExtensionType is not mutual, we remove the key from the dictionary.
        self._enabled_extensions = {}  # typing.Dict[ExtensionType, typing.Optional[int]]

    async def launch(self) -> None:
        await self._transport.launch()

    # Offered Functionality
    # =====================
    def initiate_the_bittorrent_handshake(self, reserved: bytes, info_hash: types.InfoHash, peer_id: types.PeerID) \
    -> None:
        self._transport.initiate_the_bittorrent_handshake(reserved, info_hash, peer_id)

    def send_keepalive(self) -> None:
        pass

    def send_choke_message(self) -> None:
        raise NotImplementedError()

    def send_unchoke_message(self) -> None:
        raise NotImplementedError()

    def send_interested_message(self) -> None:
        raise NotImplementedError()

    def send_not_interested_message(self) -> None:
        raise NotImplementedError()

    def send_have_message(self) -> None:
        raise NotImplementedError()

    def send_bitfield_message(self) -> None:
        raise NotImplementedError()

    def send_request_message(self) -> None:
        raise NotImplementedError()

    def send_piece_message(self) -> None:
        raise NotImplementedError()

    def send_cancel_message(self) -> None:
        raise NotImplementedError()

    def send_extension_handshake(
        self,
        supported_extensions: typing.Set[ExtensionType],
        local_port: int=-1,
        name_and_version: str="magneticod 0.x.x"
    ) -> None:
        pass

    @staticmethod
    def on_the_bittorrent_handshake_completed(reserved: bytes, info_hash: types.InfoHash, peer_id: types.PeerID) \
    -> None:
        pass

    @staticmethod
    def on_keepalive() -> None:
        pass

    @staticmethod
    def on_choke_message() -> None:
        pass

    @staticmethod
    def on_unchoke_message() -> None:
        pass

    @staticmethod
    def on_interested_message() -> None:
        pass

    @staticmethod
    def on_not_interested_message() -> None:
        pass

    @staticmethod
    def on_have_message(index: int) -> None:
        pass

    @staticmethod
    def on_bitfield_message() -> None:
        raise NotImplementedError()

    @staticmethod
    def on_request_message(index: int, begin: int, length: int) -> None:
        pass

    @staticmethod
    def on_piece_message(index: int, begin: int, piece: int) -> None:
        pass

    @staticmethod
    def on_cancel_message(index: int, begin: int, length: int) -> None:
        pass

    @staticmethod
    def on_extension_handshake_completed(payload: types.Dictionary) -> None:
        pass

    # Private Functionality
    # =====================
    def _when_the_bittorrent_handshake_completed(
            self,
            reserved: bytes,
            info_hash: types.InfoHash,
            peer_id: types.PeerID
    ) -> None:
        self.on_the_bittorrent_handshake_completed(reserved, info_hash, peer_id)

    def _when_keepalive(self) -> None:
        self.on_keepalive()

    def _when_message(self, type_: bytes, payload: typing.ByteString) -> None:
        if type_ == MessageTypes.CHOKE:
            self.on_choke_message()
        elif type_ == MessageTypes.UNCHOKE:
            self.on_unchoke_message()
        elif type_ == MessageTypes.INTERESTED:
            self.on_interested_message()
        elif type_ == MessageTypes.NOT_INTERESTED:
            self.on_not_interested_message()
        elif type_ == MessageTypes.HAVE:
            index = int.from_bytes(payload[:4], "big")
            self.on_have_message(index)
        elif type_ == MessageTypes.BITFIELD:
            raise NotImplementedError()
        elif type_ == MessageTypes.REQUEST:
            index = int.from_bytes(payload[:4], "big")
            begin = int.from_bytes(payload[4:8], "big")
            length = int.from_bytes(payload[8:12], "big")
            self.on_request_message(index, begin, length)
        elif type_ == MessageTypes.PIECE:
            index = int.from_bytes(payload[:4], "big")
            begin = int.from_bytes(payload[4:8], "big")
            piece = int.from_bytes(payload[8:12], "big")
            self.on_piece_message(index, begin, piece)
        elif type_ == MessageTypes.CANCEL:
            index = int.from_bytes(payload[:4], "big")
            begin = int.from_bytes(payload[4:8], "big")
            length = int.from_bytes(payload[8:12], "big")
            self.on_cancel_message(index, begin, length)
        elif type_ == MessageTypes.EXTENDED:
            self._when_extended_message(type_=payload[:1], payload=payload[1:])
        else:
            pass

    def _when_extended_message(self, type_: bytes, payload: typing.ByteString) -> None:
        if type_ == 0:
            self._when_extension_handshake(payload)
        elif type_ == ExtensionType.UT_METADATA.value:
            pass
        else:
            pass

    def _when_extension_handshake(self, payload: typing.ByteString) -> None:
        dictionary_schema = {
            b"m": {
                "type": "dict",
                "keyschema": {"type": "binary", "empty": False},
                "valueschema": {"type": "integer", "min": 0},
                "required": True,
            },
            b"p": {
                "type": "integer",
                "min": 1,
                "max": 2**16 - 1,
                "required": False
            },
            b"v": {
                "type": "binary",
                "empty": False,
                "required": False
            },
            b"yourip": {
                "type": "binary",
                # It's actually EITHER 4 OR 16, not anything in-between. We need to validate this ourselves!
                "minlength": 4,
                "maxlength": 16,
                "required": False
            },
            b"ipv6": {
                "type": "binary",
                "minlength": 16,
                "maxlength": 16,
                "required": False
            },
            b"ipv4": {
                "type": "binary",
                "minlength": 4,
                "maxlength": 4,
                "required": False
            },
            b"reqq": {
                "type": "integer",
                "min": 0,
                "required": False
            }
        }

        try:
            dictionary = bencode.decode(payload)
        except bencode.DecodeError:
            return

        if not cerberus.Validator(dictionary_schema).validate(dictionary):
            return

        # Check which extensions, that we show interest in, are enabled by the remote peer.
        if ExtensionType.UT_METADATA in self._enabled_extensions and b"ut_metadata" in dictionary[b"m"]:
            self._enabled_extensions[ExtensionType.UT_METADATA] = dictionary[b"m"][b"ut_metadata"]

        # As there can be multiple SUBSEQUENT extension-handshake-messages, check for the existence of b"metadata_size"
        # in the top level dictionary ONLY IF b"ut_metadata" exists in b"m". `ut_metadata` might be enabled before,
        # and other subsequent extension-handshake-messages do not need to include 'metadata_size` field in the top
        # level dictionary whilst enabling-and/or-disabling other extension features.
        if (b"ut_metadata" in dictionary[b"m"]) ^ (b"metadata_size" not in dictionary):
            return

        self.on_extension_handshake_completed(dictionary)

@enum.unique
class MessageTypes(enum.IntEnum):
    CHOKE = 0
    UNCHOKE = 1
    INTERESTED = 2
    NOT_INTERESTED = 3
    HAVE = 4
    BITFIELD = 5
    REQUEST = 6
    PIECE = 7
    CANCEL = 8
    EXTENDED = 20


@enum.unique
class ReservedFeature(enum.Enum):
    # What do values mean?
    # The first number is the offset, and the second number is the bit to set. For instance,
    # EXTENSION_PROTOCOL = (5, 0x10) means that reserved[5] & 0x10 should be true.
    DHT = (7, 0x01)
    EXTENSION_PROTOCOL = (5, 0x10)


def reserved_feature_set_to_reserved(reserved_feature_set: typing.Set[ReservedFeature]) -> bytes:
    reserved = 8 * b"\x00"
    for reserved_feature in reserved_feature_set:
        reserved[reserved_feature.value[0]] |= reserved_feature.value[1]
    return reserved


def reserved_to_reserved_feature_set(reserved: bytes) -> typing.Set[ReservedFeature]:
    return {
        reserved_feature
        for reserved_feature in ReservedFeature
        if reserved[reserved_feature.value[0]] & reserved_feature.value[1]
    }


@enum.unique
class ExtensionType(enum.IntEnum):
    UT_METADATA = 1
