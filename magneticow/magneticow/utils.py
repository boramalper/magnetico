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
from math import log10
from struct import unpack_from


# Source: http://stackoverflow.com/a/1094933
# (primarily: https://web.archive.org/web/20111010015624/http://blogmag.net/blog/read/38/Print_human_readable_file_size)
def to_human_size(num, suffix='B'):
    for unit in ['', 'Ki', 'Mi', 'Gi', 'Ti', 'Pi', 'Ei', 'Zi']:
        if abs(num) < 1024:
            return "%3.1f %s%s" % (num, unit, suffix)
        num /= 1024
    return "%.1f %s%s" % (num, 'Yi', suffix)


def rank(blob):
    # TODO: is there a way to futher optimize this?
    p, c, n = unpack_from("=LLL", blob, 0)

    x = []  # list of tuples
    for i in range(12, 12 + 3*c*p*4, 3*4):
        x0, x1, x2 = unpack_from("=LLL", blob, i)
        if x1 != 0:  # skip if it's index column
            x.append((x0, x1, x2))

    # Ignore the first column (torrent_id)
    avgdl = unpack_from("=L", blob, 12 + 3*c*p*4)[0]

    # Ignore the first column (torrent_id)
    l = unpack_from("=L", blob, (12 + 3*c*p*4) + 4*c)[0]

    # Multiply by -1 so that sorting in the ASC order would yield the best match first
    return -1 * okapi_bm25(term_frequencies=[X[0] for X in x], dl=l, avgdl=avgdl, N=n, nq=[X[2] for X in x])


# TODO: check if I got it right =)
def okapi_bm25(term_frequencies, dl, avgdl, N, nq, k1=1.2, b=0.75):
    """

    :param term_frequencies: List of frequencies of each term in the document.
    :param dl: Length of the document in words.
    :param avgdl: Average document length in the collection.
    :param N: Total number of documents in the collection.
    :param nq: List of each numbers of documents containing term[i] for each term.
    :param k1: Adjustable constant; = 1.2 in FTS5 extension of SQLite3.
    :param b: Adjustable constant; = 0.75 in FTS5 extension of SQLite3.
    :return:
    """
    return sum(
        log10((N - nq[i] + 0.5) / (nq[i] + 0.5)) *
        (
            (term_frequencies[i] * (k1 + 1)) /
            (term_frequencies[i] + k1 * (1 - b + b * dl / avgdl))
        )
        for i in range(len(term_frequencies))
    )
