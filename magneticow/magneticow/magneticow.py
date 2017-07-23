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
import datetime as dt
from datetime import datetime
import logging
import sqlite3
import os

import appdirs
import flask

from magneticow import utils
from magneticow.authorization import requires_auth, generate_feed_hash


File = collections.namedtuple("file", ["path", "size"])
Torrent = collections.namedtuple("torrent", ["info_hash", "name", "size", "discovered_on", "files"])


app = flask.Flask(__name__)
app.config.from_object(__name__)

# TODO: We should have been able to use flask.g but it does NOT persist across different requests so we resort back to
# this. Investigate the cause and fix it (I suspect of Gevent).
magneticod_db = None

@app.route("/")
@requires_auth
def home_page():
    with magneticod_db:
        # COUNT(ROWID) is much more inefficient since it scans the whole table, so use MAX(ROWID)
        cur = magneticod_db.execute("SELECT MAX(ROWID) FROM torrents ;")
        n_torrents = cur.fetchone()[0] or 0

    return flask.render_template("homepage.html", n_torrents=n_torrents)


@app.route("/torrents/")
@requires_auth
def torrents():
    search = flask.request.args.get("search")
    page = int(flask.request.args.get("page", 0))

    context = {
        "search": search,
        "page": page
    }

    SQL_query = """
        SELECT
            info_hash,
            name,
            total_size,
            discovered_on
        FROM torrents
    """
    if search:
        SQL_query += """
            INNER JOIN (
                SELECT docid AS id, rank(matchinfo(fts_torrents, 'pcnxal')) AS rank
                FROM fts_torrents
                WHERE name MATCH ?
            ) AS ranktable USING(id)
        """
    SQL_query += """
        ORDER BY {}
        LIMIT 20 OFFSET ?
    """

    sort_by = flask.request.args.get("sort_by")
    allowed_sorts = [
        None,
        "name ASC",
        "name DESC",
        "total_size ASC",
        "total_size DESC",
        "discovered_on ASC",
        "discovered_on DESC"
    ]
    if sort_by not in allowed_sorts:
        return flask.Response("Invalid value for `sort_by! (Allowed values are %s)" % (allowed_sorts, ), 400)

    if search:
        if sort_by:
            SQL_query = SQL_query.format(sort_by + ", " + "rank ASC")
        else:
            SQL_query = SQL_query.format("rank ASC")
    else:
        if sort_by:
            SQL_query = SQL_query.format(sort_by + ", " + "id DESC")
        else:
            SQL_query = SQL_query.format("id DESC")

    with magneticod_db:
        if search:
            cur = magneticod_db.execute(SQL_query, (search, 20 * page))
        else:
            cur = magneticod_db.execute(SQL_query, (20 * page, ))
        context["torrents"] = [Torrent(t[0].hex(), t[1], utils.to_human_size(t[2]),
                                       datetime.fromtimestamp(t[3]).strftime("%d/%m/%Y"), [])
                               for t in cur.fetchall()]

    if len(context["torrents"]) < 20:
        context["next_page_exists"] = False
    else:
        context["next_page_exists"] = True

    if app.arguments.noauth:
        context["subscription_url"] = "/feed/?filter%s" % search
    else:
        username, password = flask.request.authorization.username, flask.request.authorization.password
        context["subscription_url"] = "/feed?filter=%s&hash=%s" % (
            search, generate_feed_hash(username, password, search))

    if sort_by:
        context["sorted_by"] = sort_by

    return flask.render_template("torrents.html", **context)


@app.route("/torrents/<info_hash>/", defaults={"name": None})
@requires_auth
def torrent_redirect(**kwargs):
    try:
        info_hash = bytes.fromhex(kwargs["info_hash"])
        assert len(info_hash) == 20
    except (AssertionError, ValueError):  # In case info_hash variable is not a proper hex-encoded bytes
        return flask.abort(400)

    with magneticod_db:
        cur = magneticod_db.execute("SELECT name FROM torrents WHERE info_hash=? LIMIT 1;", (info_hash,))
        try:
            name = cur.fetchone()[0]
        except TypeError:  # In case no results returned, TypeError will be raised when we try to subscript None object
            return flask.abort(404)

    return flask.redirect("/torrents/%s/%s" % (kwargs["info_hash"], name), code=301)


@app.route("/torrents/<info_hash>/<name>")
@requires_auth
def torrent(**kwargs):
    context = {}

    try:
        info_hash = bytes.fromhex(kwargs["info_hash"])
        assert len(info_hash) == 20
    except (AssertionError, ValueError):  # In case info_hash variable is not a proper hex-encoded bytes
        return flask.abort(400)

    with magneticod_db:
        cur = magneticod_db.execute("SELECT id, name, discovered_on FROM torrents WHERE info_hash=? LIMIT 1;",
                                    (info_hash,))
        try:
            torrent_id, name, discovered_on = cur.fetchone()
        except TypeError:  # In case no results returned, TypeError will be raised when we try to subscript None object
            return flask.abort(404)

        cur = magneticod_db.execute("SELECT path, size FROM files WHERE torrent_id=?;", (torrent_id,))
        raw_files = cur.fetchall()
        size = sum(f[1] for f in raw_files)
        files = [File(f[0], utils.to_human_size(f[1])) for f in raw_files]

    context["torrent"] = Torrent(info_hash.hex(), name, utils.to_human_size(size), datetime.fromtimestamp(discovered_on).strftime("%d/%m/%Y"), files)

    return flask.render_template("torrent.html", **context)


@app.route("/statistics")
@requires_auth
def statistics():
    # Ahhh...
    # Time is hard, really. magneticod used time.time() to save when a torrent is discovered, unaware that none of the
    # specifications say anything about the timezones (or their irrelevance to the UNIX time) and about leap seconds in
    # a year.
    # Nevertheless, we still use it. In future, before v1.0.0, we may change it as we wish, offering a migration
    # solution for the current users. But in the meanwhile, be aware that all your calculations will be a bit lousy,
    # though within tolerable limits for a torrent search engine.

    with magneticod_db:
        # latest_today is the latest UNIX timestamp of today, the very last second.
        latest_today = int((dt.date.today() + dt.timedelta(days=1) - dt.timedelta(seconds=1)).strftime("%s"))
        # Retrieve all the torrents discovered in the past 30 days (30 days * 24 hours * 60 minutes * 60 seconds...)
        # Also, see http://www.sqlite.org/lang_datefunc.html for details of `date()`.
        #     Function          Equivalent strftime()
        #     date(...) 		strftime('%Y-%m-%d', ...)
        cur = magneticod_db.execute(
            "SELECT date(discovered_on, 'unixepoch') AS day, count() FROM torrents WHERE discovered_on >= ? "
            "GROUP BY day;",
            (latest_today - 30 * 24 * 60 * 60, )
        )
        results = cur.fetchall()  # for instance, [('2017-04-01', 17428), ('2017-04-02', 28342)]

    return flask.render_template("statistics.html", **{
        # We directly substitute them in the JavaScript code.
        "dates": str([t[0] for t in results]),
        "amounts": str([t[1] for t in results])
    })


@app.route("/feed")
def feed():
    filter_ = flask.request.args["filter"]
    # Check for all possible users who might be requesting.
    # pylint disabled: because we do monkey-patch! [in magneticow.__main__.py:main()]
    if not app.arguments.noauth:
        hash_ = flask.request.args["hash"]
        for username, password in app.arguments.user:  # pylint: disable=maybe-no-member
            if generate_feed_hash(username, password, filter_) == hash_:
                break
        else:
            return flask.Response(
                "Could not verify your access level for that URL (wrong hash).\n",
                401
            )

    context = {}

    if filter_:
        context["title"] = "`%s` - magneticow" % (filter_,)
        with magneticod_db:
            cur = magneticod_db.execute(
                "SELECT "
                "    name, "
                "    info_hash "
                "FROM torrents "
                "INNER JOIN ("
                "    SELECT docid AS id, rank(matchinfo(fts_torrents, 'pcnxal')) AS rank "
                "    FROM fts_torrents "
                "    WHERE name MATCH ? "
                "    ORDER BY rank ASC"
                "    LIMIT 50"
                ") AS ranktable USING(id);",
                (filter_, )
            )
            context["items"] = [{"title": r[0], "info_hash": r[1].hex()} for r in cur]
    else:
        context["title"] = "The Newest Torrents - magneticow"
        with magneticod_db:
            cur = magneticod_db.execute(
                "SELECT "
                "    name, "
                "    info_hash "
                "FROM torrents "
                "ORDER BY id DESC LIMIT 50"
            )
            context["items"] = [{"title": r[0], "info_hash": r[1].hex()} for r in cur]

    return flask.render_template("feed.xml", **context), 200, {"Content-Type": "application/rss+xml; charset=utf-8"}


def initialize_magneticod_db() -> None:
    global magneticod_db

    logging.info("Connecting to magneticod's database...")

    magneticod_db_path = os.path.join(appdirs.user_data_dir("magneticod"), "database.sqlite3")
    magneticod_db = sqlite3.connect(magneticod_db_path, isolation_level=None)

    logging.info("Preparing for the full-text search (this might take a while)...")
    with magneticod_db:
        magneticod_db.execute("PRAGMA journal_mode=WAL;")

        magneticod_db.execute("CREATE INDEX IF NOT EXISTS discovered_on_index ON torrents (discovered_on);")
        magneticod_db.execute("CREATE INDEX IF NOT EXISTS info_hash_index ON torrents (info_hash);")
        magneticod_db.execute("CREATE INDEX IF NOT EXISTS file_info_hash_index ON files (torrent_id);")

        magneticod_db.execute("CREATE VIRTUAL TABLE temp.fts_torrents USING fts4(name);")
        magneticod_db.execute("INSERT INTO fts_torrents (docid, name) SELECT id, name FROM torrents;")
        magneticod_db.execute("INSERT INTO fts_torrents (fts_torrents) VALUES ('optimize');")

        magneticod_db.execute("CREATE TEMPORARY TRIGGER on_torrents_insert AFTER INSERT ON torrents FOR EACH ROW BEGIN"
                              "    INSERT INTO fts_torrents (docid, name) VALUES (NEW.id, NEW.name);"
                              "END;")

    magneticod_db.create_function("rank", 1, utils.rank)


def close_db() -> None:
    logging.info("Closing magneticod database...")
    if magneticod_db is not None:
        magneticod_db.close()
