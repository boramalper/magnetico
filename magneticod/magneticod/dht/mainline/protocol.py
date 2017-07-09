import typing

from . import transport


class Protocol:
    def __init__(self):
        pass

    # Offered Functionality
    # =====================
    def on_ping_query(self, query: PingQuery) -> typing.Optional[typing.Union[PingResponse, Error]]:
        pass

    def on_find_node_query(self, query: FindNodeQuery) -> typing.Optional[typing.Union[FindNodeResponse, Error]]:
        pass

    def on_get_peers_query(self, query: GetPeersQuery) -> typing.Optional[typing.Union[GetPeersQuery, Error]]:
        pass

    def on_announce_peer_query(self, query: AnnouncePeerQuery) -> typing.Optional[typing.Union[AnnouncePeerResponse, Error]]:
        pass

    def on_ping_OR_announce_peer_response(self, response: PingResponse) -> None:
        pass

    def on_find_node_response(self, response: FindNodeResponse) -> None:
        pass

    def on_get_peers_response(self, response: GetPeersResponse) -> None:
        pass

    def on_error(self, response: Error) -> None:
        pass

    # Private Functionality
    # =====================
    def when_message_received(self, message):
        pass


NodeID = typing.NewType("NodeID", bytes)
InfoHash = typing.NewType("InfoHash", bytes)
NodeInfo = typing.NamedTuple("NodeInfo", [
    ("id", NodeID),
    ("address", transport.Address),
])


class BaseQuery:
    method_name = b""

    def __init__(self, id_: NodeID):
        self.id = id_

    def to_message(self, *, transaction_id: bytes, client_version: bytes=b"") -> typing.Dict[bytes, typing.Any]:
        return {
            b"t": transaction_id,
            b"y": b"q",
            b"v": client_version,
            b"q": self.method_name,
            b"a": self.__dict__
        }


class PingQuery(BaseQuery):
    method_name = b"ping"

    def __init__(self, id_: NodeID):
        super().__init__(id_)


class FindNodeQuery(BaseQuery):
    method_name = b"find_node"

    def __init__(self, id_: NodeID, target: NodeID):
        super().__init__(id_)
        self.target = target


class GetPeersQuery(BaseQuery):
    method_name = b"get_peers"

    def __init__(self, id_: NodeID, info_hash: InfoHash):
        super().__init__(id_)
        self.info_hash = info_hash


class AnnouncePeerQuery(BaseQuery):
    method_name = b"announce_peer"

    def __init__(self, id_: NodeID, info_hash: InfoHash, port: int, token: bytes, implied_port: int=0):
        super().__init__(id_)
        self.info_hash = info_hash
        self.port = port
        self.token = token
        self.implied_port = implied_port


class BaseResponse:
    def __init__(self, id_: NodeID):
        self.id = id_

    def to_message(self, *, transaction_id: bytes, client_version: bytes = b"") -> typing.Dict[bytes, typing.Any]:
        return {
            b"t": transaction_id,
            b"y": b"r",
            b"v": client_version,
            b"r": self._return_values()
        }

    def _return_values(self) -> typing.Dict[bytes, typing.Any]:
        return {b"id": self.id}


class PingResponse(BaseResponse):
    def __init__(self, id_: NodeID):
        super().__init__(id_)


class FindNodeResponse(BaseResponse):
    def __init__(self, id_: NodeID, nodes: typing.List[NodeInfo]):
        super().__init__(id_)
        self.nodes = nodes

    def _return_values(self) -> typing.Dict[bytes, typing.Any]:
        d = super()._return_values()
        d.update({
            b"nodes": self.nodes  # TODO: this is not right obviously, encode & decode!
        })
        return d


class GetPeersResponse(BaseResponse):
    def __init__(self, id_: NodeID, token: bytes, *, values, nodes: typing.Optional[typing.List[NodeInfo]]=None):
        assert bool(values) ^ bool(nodes)

        super().__init__(id_)
        self.token = token
        self.values = values,
        self.nodes = nodes


class AnnouncePeerResponse(BaseResponse):
    def __init__(self, id_: NodeID):
        super().__init__(id_)
