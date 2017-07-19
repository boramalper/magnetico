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
import functools
import hashlib

import flask


# Adapted from: http://flask.pocoo.org/snippets/8/
# (c) Copyright 2010 - 2017 by Armin Ronacher
# BEGINNING OF THE 3RD PARTY COPYRIGHTED CONTENT
def is_authorized(supplied_username, supplied_password):
    """ This function is called to check if a username / password combination is valid. """
    # Because we do monkey-patch! [in magneticow.__main__.py:main()]
    app = flask.current_app
    for username, password in app.arguments.user:  # pylint: disable=maybe-no-member
        if supplied_username == username and supplied_password == password:
            return True
    return False


def authenticate():
    """ Sends a 401 response that enables basic auth. """
    return flask.Response(
        "Could not verify your access level for that URL.\n"
        "You have to login with proper credentials",
        401,
        {"WWW-Authenticate": 'Basic realm="Login Required"'}
    )


def requires_auth(f):
    @functools.wraps(f)
    def decorated(*args, **kwargs):
        auth = flask.request.authorization
        if not flask.current_app.arguments.noauth:
            if not auth or not is_authorized(auth.username, auth.password):
                return authenticate()
        return f(*args, **kwargs)
    return decorated
# END OF THE 3RD PARTY COPYRIGHTED CONTENT


def generate_feed_hash(username: str, password: str, filter_: str) -> str:
    """
    Deterministically generates the feed hash from given username, password, and filter.
    Hash is the hex encoding of the SHA256 sum.
    """
    return hashlib.sha256((username + "\0" + password + "\0" + filter_).encode()).digest().hex()
