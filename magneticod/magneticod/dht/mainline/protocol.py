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
import enum
import functools
import typing

import cerberus

from . import transport


class Protocol:
    def __init__(self, *, client_version: bytes=b"mc00"):
        self.client_version = client_version
        self.transport = transport.Transport()

        self.transport.on_message = functools.partial(self.__when_message, self)

    async def launch(self, address: transport.Address):
        await asyncio.get_event_loop().create_datagram_endpoint(lambda: self.transport, local_addr=address)

    # Offered Functionality
    # =====================
    @staticmethod
    def on_ping_query(query: PingQuery) -> typing.Optional[typing.Union[PingResponse, Error]]:
        pass

    @staticmethod
    def on_find_node_query(query: FindNodeQuery) -> typing.Optional[typing.Union[FindNodeResponse, Error]]:
        pass

    @staticmethod
    def on_get_peers_query(query: GetPeersQuery) -> typing.Optional[typing.Union[GetPeersQuery, Error]]:
        pass

    @staticmethod
    def on_announce_peer_query(query: AnnouncePeerQuery) -> typing.Optional[typing.Union[AnnouncePeerResponse, Error]]:
        pass

    @staticmethod
    def on_ping_OR_announce_peer_response(response: PingResponse) -> None:
        pass

    @staticmethod
    def on_find_node_response(response: FindNodeResponse) -> None:
        pass

    @staticmethod
    def on_get_peers_response(response: GetPeersResponse) -> None:
        pass

    @staticmethod
    def on_error(error: Error) -> None:
        pass

    # Private Functionality
    # =====================
    def __when_message(self, message: typing.Dict[bytes, typing.Any], address: transport.Address) -> None:
        # We need to ignore unknown fields in the messages, in consideration of forward-compatibility, but that also
        # requires us to be careful about the "order" we are following. For instance, every single query can also be
        # misunderstood as a ping query, since they all have `id` as an argument. Hence, we start validating against the
        # query/response type that is most distinguishing against all other.

        if BaseQuery.validate_message(message):
            args = message[b"a"]
            if AnnouncePeerQuery.validate_message(message):
                response = self.on_announce_peer_query(AnnouncePeerQuery(
                    args[b"id"], args[b"info_hash"], args[b"port"], args[b"token"], args[b"implied_port"]
                ))
            elif GetPeersQuery.validate_message(message):
                response = self.on_get_peers_query(GetPeersQuery(args[b"id"], args[b"info_hash"]))
            elif FindNodeQuery.validate_message(message):
                response = self.on_find_node_query(FindNodeQuery(args[b"id"], args[b"target"]))
            elif PingQuery.validate_message(message):
                response = self.on_ping_query(PingQuery(args[b"id"]))
            else:
                # Unknown Query received!
                response = None
            if response:
                self.transport.send_message(response.to_message(message[b"t"], self.client_version), address)

        elif BaseResponse.validate_message(message):
            return_values = message[b"r"]
            if GetPeersResponse.validate_message(message):
                if b"nodes" in return_values:
                    self.on_get_peers_response(GetPeersResponse(
                        return_values[b"id"], return_values[b"token"], nodes=return_values[b"nodes"]
                    ))
                else:
                    self.on_get_peers_response(GetPeersResponse(
                        return_values[b"id"], return_values[b"token"], values=return_values[b"values"]
                    ))
            elif FindNodeResponse.validate_message(message):
                self.on_find_node_response(FindNodeResponse(return_values[b"id"], return_values[b"nodes"]))
            elif PingResponse.validate_message(message):
                self.on_ping_OR_announce_peer_response(PingResponse(return_values[b"id"]))
            else:
                # Unknown Response received!
                pass

        elif Error.validate_message(message):
            if Error.validate_message(message):
                self.on_error(Error(message[b"e"][0], message[b"e"][1]))
            else:
                # Erroneous Error received!
                pass

        else:
            # Unknown message received!
            pass


NodeID = typing.NewType("NodeID", bytes)
InfoHash = typing.NewType("InfoHash", bytes)
NodeInfo = typing.NamedTuple("NodeInfo", [
    ("id", NodeID),
    ("address", transport.Address),
])


class BaseQuery:
    method_name = b""
    _arguments_schema = {
        b"id": {"type": "binary", "minlength": 20, "maxlength": 20, "required": True}
    }
    __validator = cerberus.Validator()

    def __init__(self, id_: NodeID):
        self.id = id_

    def to_message(self, transaction_id: bytes, client_version: bytes) -> typing.Dict[bytes, typing.Any]:
        return {
            b"t": transaction_id,
            b"y": b"q",
            b"v": client_version,
            b"q": self.method_name,
            b"a": self.__dict__
        }

    @classmethod
    def validate_message(cls, message: typing.Dict[bytes, typing.Any]) -> bool:
        if cls.__validator.schema is None:
            cls.__validator.schema = cls.__get_message_schema()

        return cls.__validator.validate(message)

    @classmethod
    def __get_message_schema(cls):
        return {
            b"t": {"type": "binary", "empty": False, "required": True},
            b"y": {"type": "binary", "empty": False, "required": True},
            b"v": {"type": "binary", "empty": False, "required": False},
            b"q": {"type": "binary", "empty": False, "required": True},
            b"a": cls._arguments_schema
        }


class PingQuery(BaseQuery):
    method_name = b"ping"

    def __init__(self, id_: NodeID):
        super().__init__(id_)


class FindNodeQuery(BaseQuery):
    method_name = b"find_node"
    _arguments_schema = {
        **super()._arguments_schema,
        b"target": {"type": "binary", "minlength": 20, "maxlength": 20, "required": True}
    }

    def __init__(self, id_: NodeID, target: NodeID):
        super().__init__(id_)
        self.target = target


class GetPeersQuery(BaseQuery):
    method_name = b"get_peers"
    _arguments_schema = {
        **super()._arguments_schema,
        b"info_hash": {"type": "binary", "minlength": 20, "maxlength": 20, "required": True}
    }

    def __init__(self, id_: NodeID, info_hash: InfoHash):
        super().__init__(id_)
        self.info_hash = info_hash


class AnnouncePeerQuery(BaseQuery):
    method_name = b"announce_peer"
    _arguments_schema = {
        **super()._arguments_schema,
        b"info_hash": {"type": "binary", "minlength": 20, "maxlength": 20, "required": True},
        b"port": {"type": "integer", "min": 1, "max": 2**16 - 1, "required": True},
        b"token": {"type": "binary", "empty": False, "required": True},
        b"implied_port": {"type": "integer", "required": False}
    }

    def __init__(self, id_: NodeID, info_hash: InfoHash, port: int, token: bytes, implied_port: int=0):
        super().__init__(id_)
        self.info_hash = info_hash
        self.port = port
        self.token = token
        self.implied_port = implied_port


class BaseResponse:
    _return_values_schema = {
        b"id": {"type": "binary", "minlength": 20, "maxlength": 20, "required": True}
    }
    __validator = cerberus.Validator()
    
    def __init__(self, id_: NodeID):
        self.id = id_

    def to_message(self, transaction_id: bytes, client_version: bytes) -> typing.Dict[bytes, typing.Any]:
        return {
            b"t": transaction_id,
            b"y": b"r",
            b"v": client_version,
            b"r": self._return_values()
        }

    @classmethod
    def validate_message(cls, message: typing.Dict[bytes, typing.Any]) -> bool:
        if cls.__validator.schema is None:
            cls.__validator.schema = cls.__get_message_schema()

        return cls.__validator.validate(message)

    def _return_values(self) -> typing.Dict[bytes, typing.Any]:
        return {b"id": self.id}

    @classmethod
    def __get_message_schema(cls):
        return {
            b"t": {"type": "binary", "empty": False, "required": True},
            b"y": {"type": "binary", "empty": False, "required": True},
            b"v": {"type": "binary", "empty": False, "required": False},
            b"r": cls._return_values_schema
        }


class PingResponse(BaseResponse):
    def __init__(self, id_: NodeID):
        super().__init__(id_)


class FindNodeResponse(BaseResponse):
    _return_values_schema = {
        **super()._return_values_schema,
        b"nodes": {"type": "binary", "required": True}
    }
    __validator = cerberus.Validator()

    def __init__(self, id_: NodeID, nodes: typing.List[NodeInfo]):
        super().__init__(id_)
        self.nodes = nodes

    @classmethod
    def validate_message(cls, message: typing.Dict[bytes, typing.Any]) -> bool:
        if cls.__validator.schema is None:
            cls.__validator.schema = cls.__get_message_schema()

        if not cls.__validator.validate(message):
            return False

        # Unfortunately, Cerberus cannot check some fine details.
        # For instance, the length of the `nodes` field in the return values of the response message has to be a
        # multiple of 26, as "contact information for nodes is encoded as a 26-byte string" (BEP 5).
        if not message[b"r"][b"nodes"] % 26 == 0:
            return False

        return True

    def _return_values(self) -> typing.Dict[bytes, typing.Any]:
        return {
            **super()._return_values(),
            b"nodes": self.nodes  # TODO: this is not right obviously, encode & decode!
        }


class GetPeersResponse(BaseResponse):
    _return_values_schema = {
        **super()._return_values_schema,
        b"token":  {"type": "binary", "empty": False, "required": True},
        b"values": {
            "type": "list",
            "schema": {"type": "binary", "minlength": 6, "maxlength": 6},
            "excludes": b"nodes",
            "empty": False,
            "require": True
        },
        b"nodes": {"type": "binary", "excludes": b"values", "empty": True, "require": True}
    }
    __validator = cerberus.Validator()

    def __init__(self, id_: NodeID, token: bytes, *, values: typing.Optional[typing.List[bytes]]=None,
                 nodes: typing.Optional[typing.List[NodeInfo]]=None
    ):
        if not bool(values) ^ bool(nodes):
            raise ValueError("Supply either `values` or `nodes` but not both or neither.")

        super().__init__(id_)
        self.token = token
        self.values = values,
        self.nodes = nodes

    @classmethod
    def validate_message(cls, message: typing.Dict[bytes, typing.Any]) -> bool:
        if cls.__validator.schema is None:
            cls.__validator.schema = cls.__get_message_schema()

        if not cls.__validator.validate(message):
            return False

        # Unfortunately, Cerberus cannot check some fine details.
        # For instance, the length of the `nodes` field in the return values of the response message has to be a
        # multiple of 26, as "contact information for nodes is encoded as a 26-byte string" (BEP 5).
        if b"nodes" in message[b"r"] ^ message[b"r"][b"nodes"] % 26 == 0:
            return False

        return True


class AnnouncePeerResponse(BaseResponse):
    def __init__(self, id_: NodeID):
        super().__init__(id_)


@enum.unique
class ErrorCodes(enum.IntEnum):
    GENERIC = 201
    SERVER = 202
    PROTOCOL = 203
    METHOD_UNKNOWN = 204


class Error:
    __validator = cerberus.Validator()

    def __init__(self, code: ErrorCodes, error_message: bytes):
        self.code = code
        self.error_message = error_message

    def to_message(self, transaction_id: bytes, client_version: bytes) -> typing.Dict[bytes, typing.Any]:
        return {
            b"t": transaction_id,
            b"y": b"e",
            b"v": client_version,
            b"e": [self.code, self.error_message]
        }

    @classmethod
    def validate_message(cls, message: typing.Dict[bytes, typing.Any]) -> bool:
        if cls.__validator.schema is None:
            cls.__validator.schema = cls.__get_message_schema()

        if not cls.__validator.validate(message):
            return False

        # Unfortunately, Cerberus cannot check some fine details.
        # For instance, the `e` field of the error message should be an array with first element being an integer, and
        # the second element being a (binary) string.
        if not (isinstance(message[b"e"], int) and isinstance(message[b"e"], bytes)):
            return False

        return True

    @classmethod
    def __get_message_schema(cls):
        return {
            b"t": {"type": "binary", "empty": False, "required": True},
            b"y": {"type": "binary", "empty": False, "required": True},
            b"v": {"type": "binary", "empty": False, "required": False},
            b"e": {"type": "list", "minlength": 2, "maxlength": 2, "required": True}
        }
