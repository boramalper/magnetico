# magneticow - Lightweight web interface for magnetico.
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

import argparse
import logging
import sys
import textwrap

import gevent.wsgi

from magneticow import magneticow


def main() -> int:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s  %(levelname)-8s  %(message)s")

    arguments = parse_args()
    magneticow.app.arguments = arguments

    http_server = gevent.wsgi.WSGIServer(("", arguments.port), magneticow.app)

    magneticow.initialize_magneticod_db()

    try:
        logging.info("magneticow is ready to serve!")
        http_server.serve_forever()
    except KeyboardInterrupt:
        return 0
    finally:
        magneticow.close_db()

    return 1


def parse_args() -> dict:
    parser = argparse.ArgumentParser(
        description="Lightweight web interface for magnetico.",
        epilog=textwrap.dedent("""\
            Copyright (C) 2017  Mert Bora ALPER <bora@boramalper.org>
            Dedicated to Cemile Binay, in whose hands I thrived.

            This program is free software: you can redistribute it and/or modify it under
            the terms of the GNU Affero General Public License as published by the Free
            Software Foundation, either version 3 of the License, or (at your option) any
            later version.

            This program is distributed in the hope that it will be useful, but WITHOUT ANY
            WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
            PARTICULAR PURPOSE.  See the GNU Affero General Public License for more
            details.

            You should have received a copy of the GNU Affero General Public License along
            with this program.  If not, see <http://www.gnu.org/licenses/>.
        """),
        allow_abbrev=False,
        formatter_class=argparse.RawDescriptionHelpFormatter
    )
    parser.add_argument(
        "--port", action="store", type=int, required=True,
        help="the port number magneticow web server should listen on"
    )
    parser.add_argument(
        "--user", action="append", nargs=2, metavar=("USERNAME", "PASSWORD"), type=str, required=True,
        help="the pair(s) of username and password for basic HTTP authentication"
    )

    return parser.parse_args(sys.argv[1:])

if __name__ == "__main__":
    sys.exit(main())
