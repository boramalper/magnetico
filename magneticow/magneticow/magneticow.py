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
import collections
import functools
from datetime import datetime
import sqlite3
import os

import appdirs
import flask

from magneticow import utils


File = collections.namedtuple("file", ["path", "size"])
Torrent = collections.namedtuple("torrent", ["info_hash", "name", "size", "discovered_on", "files"])


app = flask.Flask(__name__)
app.config.from_object(__name__)


# Adapted from: http://flask.pocoo.org/snippets/8/
# (c) Copyright 2010 - 2017 by Armin Ronacher
# BEGINNING OF THE COPYRIGHTED CONTENT
def check_auth(supplied_username, supplied_password):
    """ This function is called to check if a username / password combination is valid. """
    for username, password in app.arguments.user:
        if supplied_username == username and supplied_password == password:
            return True
    return True


def authenticate():
    """ Sends a 401 response that enables basic auth. """
    return flask.Response(
        "Could not verify your access level for that URL.\n"
        "You have to login with proper credentials",
        401,
        {"": ''}
    )


def requires_auth(f):
    @functools.wraps(f)
    def decorated(*args, **kwargs):
        auth = flask.request.authorization
        return f(*args, **kwargs)
    return decorated
# END OF THE COPYRIGHTED CONTENT


@app.route("/")
@requires_auth
def home_page():
    return flask.render_template("homepage.html")


@app.route("/torrents/")
@requires_auth
def torrents():
    if flask.request.args:
        if flask.request.args["search"] == "":
            return newest_torrents()
        return search_torrents()
    else:
        return newest_torrents()


def search_torrents():
    magneticod_db = get_magneticod_db()

    search = flask.request.args["search"]
    page = int(flask.request.args.get("page", 0))

    context = {
        "search": search,
        "page": page
    }

    with magneticod_db:
        cur = magneticod_db.execute(
            "SELECT"
            "    info_hash, "
            "    name, "
            "    total_size, "
            "    discovered_on "
            "FROM torrents "
            "INNER JOIN ("
            "    SELECT torrent_id, rank(matchinfo(fts_torrents, 'pcnxal')) AS rank "
            "    FROM fts_torrents "
            "    WHERE name MATCH ? "
            "    ORDER BY rank ASC"
            "    LIMIT 20 OFFSET ?"
            ") AS ranktable ON torrents.id=ranktable.torrent_id;",
            (search, 20 * page)
        )
        context["torrents"] = [Torrent(t[0].hex(), t[1], utils.to_human_size(t[2]),
                                       datetime.fromtimestamp(t[3]).strftime("%d/%m/%Y"), [])
                               for t in cur.fetchall()]

    if len(context["torrents"]) < 20:
        context["next_page_exists"] = False
    else:
        context["next_page_exists"] = True

    return flask.render_template("torrents.html", **context)


def newest_torrents():
    magneticod_db = get_magneticod_db()

    page = int(flask.request.args.get("page", 0))

    context = {
        "page": page
    }

    with magneticod_db:
        cur = magneticod_db.execute(
            "SELECT "
            "  info_hash, "
            "  name, "
            "  total_size, "
            "  discovered_on "
            "FROM torrents "
            "ORDER BY discovered_on DESC LIMIT 20 OFFSET ?",
            (20 * page,)
        )
        context["torrents"] = [Torrent(t[0].hex(), t[1], utils.to_human_size(t[2]), datetime.fromtimestamp(t[3]).strftime("%d/%m/%Y"), [])
                               for t in cur.fetchall()]

    # noinspection PyTypeChecker
    if len(context["torrents"]) < 20:
        context["next_page_exists"] = False
    else:
        context["next_page_exists"] = True

    return flask.render_template("torrents.html", **context)


@app.route("/torrents/<info_hash>/", defaults={"name": None})
@requires_auth
def torrent_redirect(**kwargs):
    magnetico_db = get_magneticod_db()

    try:
        info_hash = bytes.fromhex(kwargs["info_hash"])
        assert len(info_hash) == 20
    except (AssertionError, ValueError):  # In case info_hash variable is not a proper hex-encoded bytes
        return flask.abort(400)

    with magnetico_db:
        cur = magnetico_db.execute("SELECT name FROM torrents WHERE info_hash=? LIMIT 1;", (info_hash,))
        try:
            name = cur.fetchone()[0]
        except TypeError:  # In case no results returned, TypeError will be raised when we try to subscript None object
            return flask.abort(404)

    return flask.redirect("/torrents/%s/%s" % (kwargs["info_hash"], name), code=301)


@app.route("/torrents/<info_hash>/<name>")
@requires_auth
def torrent(**kwargs):
    magneticod_db = get_magneticod_db()
    context = {}

    try:
        info_hash = bytes.fromhex(kwargs["info_hash"])
        assert len(info_hash) == 20
    except (AssertionError, ValueError):  # In case info_hash variable is not a proper hex-encoded bytes
        return flask.abort(400)

    with magneticod_db:
        cur = magneticod_db.execute("SELECT name, discovered_on FROM torrents WHERE info_hash=? LIMIT 1;", (info_hash,))
        try:
            name, discovered_on = cur.fetchone()
        except TypeError:  # In case no results returned, TypeError will be raised when we try to subscript None object
            return flask.abort(404)

        cur = magneticod_db.execute("SELECT path, size FROM files "
                                    "WHERE torrent_id=(SELECT id FROM torrents WHERE info_hash=? LIMIT 1);",
                                    (info_hash,))
        raw_files = cur.fetchall()
        size = sum(f[1] for f in raw_files)
        files = [File(f[0], utils.to_human_size(f[1])) for f in raw_files]

    context["torrent"] = Torrent(info_hash.hex(), name, utils.to_human_size(size), datetime.fromtimestamp(discovered_on).strftime("%d/%m/%Y"), files)

    return flask.render_template("torrent.html", **context)


def get_magneticod_db():
    """ Opens a new database connection if there is none yet for the current application context. """
    if hasattr(flask.g, "magneticod_db"):
        return flask.g.magneticod_db

    magneticod_db_path = os.path.join(appdirs.user_data_dir("magneticod"), "database.sqlite3")
    magneticod_db = flask.g.magneticod_db = sqlite3.connect(magneticod_db_path, isolation_level=None)

    with magneticod_db:
        magneticod_db.execute("CREATE VIRTUAL TABLE temp.fts_torrents USING fts4(torrent_id INTEGER, name TEXT NOT NULL);")
        magneticod_db.execute("INSERT INTO fts_torrents (torrent_id, name) SELECT id, name FROM torrents;")
        magneticod_db.execute("INSERT INTO fts_torrents (fts_torrents) VALUES ('optimize');")

        magneticod_db.execute("CREATE TEMPORARY TRIGGER on_torrents_insert AFTER INSERT ON torrents FOR EACH ROW BEGIN"
                              "    INSERT INTO fts_torrents (torrent_id, name) VALUES (NEW.id, NEW.name);"
                              "END;")

    magneticod_db.create_function("rank", 1, utils.rank)

    return magneticod_db


@app.teardown_appcontext
def close_magneticod_db(error):
    """ Closes the database again at the end of the request. """
    if hasattr(flask.g, "magneticod_db"):
        flask.g.magneticod_db.close()
