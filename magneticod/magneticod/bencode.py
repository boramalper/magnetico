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
bencode

Warning:
    Encoders do NOT check for circular objects! (and will NEVER check due to speed concerns).

TODO:
    Add support for integers in scientific notation. (?)
    Please do re-write this as a shared C module so that we can gain a  H U G E  speed & performance gain!

    I M P O R T A N T  //  U R G E N T
    Support bytearrays as well! (Currently, only bytes).
"""


import typing


Types = typing.Union[int, bytes, list, "KRPCDict"]
KRPCDict = typing.Dict[bytes, Types]


def dumps(obj) -> bytes:
    try:
        return __encode[type(obj)](obj)
    except:
        raise BencodeEncodingError()


def loads(bytes_object: bytes) -> Types:
    try:
        return __decoders[bytes_object[0]](bytes_object, 0)[0]
    except Exception as exc:
        raise BencodeDecodingError(exc)


def loads2(bytes_object: bytes) -> typing.Tuple[Types, int]:
    """
    Returns the bencoded object AND the index where the dump of the decoded object ends (exclusive). In less words:

        dump = b"i12eOH YEAH"
        object, i = loads2(dump)
        print(">>>", dump[i:])  # OUTPUT: >>> b'OH YEAH'
    """
    try:
        return __decoders[bytes_object[0]](bytes_object, 0)
    except Exception as exc:
        raise BencodeDecodingError(exc)


def __encode_int(i: int) -> bytes:
    # False positive...
    return b"i%de" % i


def __encode_str(s: typing.ByteString) -> bytes:
    return b"%d:%s" % (len(s), s)


def __encode_list(l: typing.Sequence) -> bytes:
    """ REFERENCE IMPLEMENTATION
    s = bytearray()
    for obj in l:
        s += __encode[type(obj)](obj)
    return b"l%se" % (s,)
    """
    return b"l%se" % b"".join(__encode[type(obj)](obj) for obj in l)


def __encode_dict(d: typing.Dict[typing.ByteString, typing.Any]) -> bytes:
    s = bytearray()
    # Making sure that the keys are in lexicographical order.
    # Source: http://stackoverflow.com/a/7375703/4466589
    items = sorted(d.items(), key=lambda k: (k[0].lower(), k[0]))
    for key, value in items:
        s += __encode_str(key)
        s += __encode[type(value)](value)
    return b"d%se" % (s, )


__encode = {
    int: __encode_int,
    bytes: __encode_str,
    bytearray: __encode_str,
    list: __encode_list,
    dict: __encode_dict
}


def __decode_int(b: bytes, start_i: int) -> typing.Tuple[int, int]:
    end_i = b.find(b"e", start_i)
    assert end_i != -1
    return int(b[start_i + 1: end_i]), end_i + 1


def __decode_str(b: bytes, start_i: int) -> typing.Tuple[bytes, int]:
    separator_i = b.find(b":", start_i)
    assert separator_i != -1
    length = int(b[start_i: separator_i])
    return b[separator_i + 1: separator_i + 1 + length], separator_i + 1 + length


def __decode_list(b: bytes, start_i: int) -> typing.Tuple[list, int]:
    list_ = []
    i = start_i + 1
    while b[i] != 101:  # 101 = ord(b"e")
        item, i = __decoders[b[i]](b, i)
        list_.append(item)
    return list_, i + 1


def __decode_dict(b: bytes, start_i: int) -> typing.Tuple[dict, int]:
    dict_ = {}

    i = start_i + 1
    while b[i] != 101:  # 101 = ord(b"e")
        # Making sure it's between b"0" and b"9" (incl.)
        assert 48 <= b[i] <= 57
        key, end_i = __decode_str(b, i)
        dict_[key], i = __decoders[b[end_i]](b, end_i)

    return dict_, i + 1


__decoders = {
    ord(b"i"): __decode_int,
    ord(b"0"): __decode_str,
    ord(b"1"): __decode_str,
    ord(b"2"): __decode_str,
    ord(b"3"): __decode_str,
    ord(b"4"): __decode_str,
    ord(b"5"): __decode_str,
    ord(b"6"): __decode_str,
    ord(b"7"): __decode_str,
    ord(b"8"): __decode_str,
    ord(b"9"): __decode_str,
    ord(b"l"): __decode_list,
    ord(b"d"): __decode_dict
}


class BencodeEncodingError(Exception):
    pass


class BencodeDecodingError(Exception):
    def __init__(self, original_exc):
        super().__init__()
        self.original_exc = original_exc
