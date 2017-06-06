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
import logging
import sqlite3
import time
import typing
import os

from magneticod import bencode

from .constants import PENDING_INFO_HASHES


class Database:
    def __init__(self, database) -> None:
        self.__db_conn = self.__connect(database)

        # We buffer metadata to flush many entries at once, for performance reasons.
        # list of tuple (info_hash, name, total_size, discovered_on)
        self.__pending_metadata = []  # type: typing.List[typing.Tuple[bytes, str, int, int]]
        # list of tuple (info_hash, size, path)
        self.__pending_files = []  # type: typing.List[typing.Tuple[bytes, int, bytes]]

    @staticmethod
    def __connect(database) -> sqlite3.Connection:
        os.makedirs(os.path.split(database)[0], exist_ok=True)
        db_conn = sqlite3.connect(database, isolation_level=None)

        db_conn.execute("PRAGMA journal_mode=WAL;")
        db_conn.execute("PRAGMA temp_store=1;")
        db_conn.execute("PRAGMA foreign_keys=ON;")

        with db_conn:
            db_conn.execute("CREATE TABLE IF NOT EXISTS torrents ("
                            "id             INTEGER PRIMARY KEY AUTOINCREMENT,"
                            "info_hash      BLOB NOT NULL UNIQUE,"
                            "name           TEXT NOT NULL,"
                            "total_size     INTEGER NOT NULL CHECK(total_size > 0),"
                            "discovered_on  INTEGER NOT NULL CHECK(discovered_on > 0)"
                            ");")
            db_conn.execute("CREATE INDEX IF NOT EXISTS info_hash_index ON torrents (info_hash);")
            db_conn.execute("CREATE TABLE IF NOT EXISTS files ("
                            "id          INTEGER PRIMARY KEY,"
                            "torrent_id  INTEGER REFERENCES torrents ON DELETE CASCADE ON UPDATE RESTRICT,"
                            "size        INTEGER NOT NULL,"
                            "path        TEXT NOT NULL"
                            ");")
            db_conn.execute("CREATE INDEX IF NOT EXISTS file_info_hash_index ON files (torrent_id);")

        return db_conn

    def add_metadata(self, info_hash: bytes, metadata: bytes) -> bool:
        files = []
        discovered_on = int(time.time())
        try:
            info = bencode.loads(metadata)  # type: dict

            assert b"/" not in info[b"name"]
            name = info[b"name"].decode("utf-8")

            if b"files" in info:  # Multiple File torrent:
                for file in info[b"files"]:
                    assert type(file[b"length"]) is int
                    # Refuse trailing slash in any of the path items
                    assert not any(b"/" in item for item in file[b"path"])
                    path = "/".join(i.decode("utf-8") for i in file[b"path"])
                    files.append((info_hash, file[b"length"], path))
            else:  # Single File torrent:
                assert type(info[b"length"]) is int
                files.append((info_hash, info[b"length"], name))
        # TODO: Make sure this catches ALL, AND ONLY operational errors
        except (bencode.BencodeDecodingError, AssertionError, KeyError, AttributeError, UnicodeDecodeError, TypeError):
            return False

        self.__pending_metadata.append((info_hash, name, sum(f[1] for f in files), discovered_on))
        self.__pending_files += files

        logging.info("Added: `%s`", name)

        # Automatically check if the buffer is full, and commit to the SQLite database if so.
        if len(self.__pending_metadata) >= PENDING_INFO_HASHES:
            self.__commit_metadata()

        return True

    def is_infohash_new(self, info_hash):
        if info_hash in [x[0] for x in self.__pending_metadata]:
            return False
        cur = self.__db_conn.cursor()
        try:
            cur.execute("SELECT count(info_hash) FROM torrents where info_hash = ?;", [info_hash])
            x, = cur.fetchone()
            return x == 0
        finally:
            cur.close()

    def __commit_metadata(self) -> None:
        cur = self.__db_conn.cursor()
        cur.execute("BEGIN;")
        # noinspection PyBroadException
        try:
            cur.executemany(
                "INSERT INTO torrents (info_hash, name, total_size, discovered_on) VALUES (?, ?, ?, ?);",
                self.__pending_metadata
            )
            cur.executemany(
                "INSERT INTO files (torrent_id, size, path) "
                "VALUES ((SELECT id FROM torrents WHERE info_hash=?), ?, ?);",
                self.__pending_files
            )
            cur.execute("COMMIT;")
            logging.info("%d metadata (%d files) are committed to the database.",
                          len(self.__pending_metadata), len(self.__pending_files))
            self.__pending_metadata.clear()
            self.__pending_files.clear()
        except:
            cur.execute("ROLLBACK;")
            logging.exception("Could NOT commit metadata to the database! (%d metadata are pending)",
                              len(self.__pending_metadata))
        finally:
            cur.close()

    def close(self) -> None:
        if self.__pending_metadata:
            self.__commit_metadata()
        self.__db_conn.close()
