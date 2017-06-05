try:
    from magneticod.bencoder._fast import dumps, loads, loads2
    from magneticod.bencoder._fast import BencodeValueError, BencodeTypeError
except ImportError:
    from magneticod.bencoder._pure import dumps, loads, loads2
    from magneticod.bencoder._pure import BencodeValueError, BencodeTypeError
