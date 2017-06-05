"""
Pure Python implementation of Bencode serialization format.
To be used when fast C Extension cannot be compiled.
"""
from io import BytesIO as StringIO


INTEGER_TYPES = (int,)
BINARY_TYPES = (bytes, )
int_to_binary = lambda val: bytes(str(val), 'ascii')


class BencodeValueError(ValueError):
    pass


class BencodeTypeError(TypeError):
    pass


def _dump_implementation(obj, write, path, cast):
    """ dump()/dumps() implementation """

    t = type(obj)

    if id(obj) in path:
        raise BencodeValueError('circular reference detected')

    if t in INTEGER_TYPES:
        write(b'i')
        write(int_to_binary(obj))
        write(b'e')
    elif t in BINARY_TYPES:
        write(int_to_binary(len(obj)))
        write(b':')
        write(obj)
    elif t is list or (cast and issubclass(t, (list, tuple))):
        write(b'l')
        for item in obj:
            _dump_implementation(item, write, path + [id(obj)], cast)
        write(b'e')
    elif t is dict:
        write(b'd')

        data = sorted(obj.items())
        for key, val in data:
            _dump_implementation(key, write, path + [id(obj)], cast)
            _dump_implementation(val, write, path + [id(obj)], cast)
        write(b'e')
    elif cast and t is bool:
        write(b'i')
        write(int_to_binary(int(obj)))
        write(b'e')
    else:
        raise BencodeTypeError(
            'type %s is not Bencode serializable' % type(obj).__name__
        )


def dumps(obj, cast=False):
    """Serialize ``obj`` to a Bencode formatted ``str``."""

    fp = []
    _dump_implementation(obj, fp.append, [], cast)
    return b''.join(fp)


def _read_until(delimiter, read):
    """ Read char by char until ``delimiter`` occurs. """

    result = b''
    ch = read(1)
    if not ch:
        raise BencodeValueError('unexpected end of data')
    while ch != delimiter:
        result += ch
        ch = read(1)
        if not ch:
            raise BencodeValueError('unexpected end of data')
    return result


def _load_implementation(read):
    """ load()/loads() implementation """

    first = read(1)

    if first == b'e':
        return StopIteration
    elif first == b'i':
        value = b''
        ch = read(1)
        while (b'0' <= ch <= b'9') or (ch == b'-'):
            value += ch
            ch = read(1)
        if ch == b'' or (ch == b'e' and value in (b'', b'-')):
            raise BencodeValueError('unexpected end of data')
        if ch != b'e':
            raise BencodeValueError('unexpected byte 0x%.2x' % ord(ch))
        return int(value)
    elif b'0' <= first <= b'9':
        size = 0
        while b'0' <= first <= b'9':
            size = size * 10 + (ord(first) - ord('0'))
            first = read(1)
            if first == b'':
                raise BencodeValueError('unexpected end of data')
        if first != b':':
            raise BencodeValueError('unexpected byte 0x%.2x' % ord(first))
        data = read(size)
        if len(data) != size:
            raise BencodeValueError('unexpected end of data')
        return data
    elif first == b'l':
        result = []
        while True:
            val = _load_implementation(read)
            if val is StopIteration:
                return result
            result.append(val)
    elif first == b'd':
        result = {}
        while True:
            this = read(1)
            if this == b'e':
                return result
            elif this == b'':
                raise BencodeValueError('unexpected end of data')
            elif not this.isdigit():
                raise BencodeValueError('unexpected byte 0x%.2x' % ord(this))
            size = int(this + _read_until(b':', read))
            key = read(size)
            val = _load_implementation(read)
            result[key] = val
    elif first == b'':
        raise BencodeValueError('unexpected end of data')
    else:
        raise BencodeValueError('unexpected byte 0x%.2x' % ord(first))


def loads(data):
    """Deserialize ``s`` to a Python object."""

    fp = StringIO(data)
    return _load_implementation(fp.read)


def loads2(data):
    """Deserialize ``s`` to a Python object."""

    fp = StringIO(data)
    return _load_implementation(fp.read), fp.tell()
