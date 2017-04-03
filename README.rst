=========
magnetico
=========
*Autonomous (self-hosted) BitTorrent DHT search engine suite.*

magnetico is the first autonomous (self-hosted) BitTorrent DHT search engine suite that is *designed for end-users*.
The suite consists of two packages:

* **magneticod:** Autonomous BitTorrent DHT crawler and metadata fetcher.
* **magneticow:** Lightweight web interface for magnetico.

Both programs, combined together, allows anyone with a decent Internet connection to access the vast amount of torrents
waiting to be discovered within the BitTorrent DHT space, *without relying on any central entity*.

**magnetico** liberates BitTorrent from the yoke of centralised trackers & web-sites and makes it *truly
decentralised*. Finally!

Features
========
- Easy installation & minimal requirements:

  - Python 3.5+ and a few Python packages that is available on PyPI.
  - Root access is *not* required to install.
- Near-zero configuration:

  - magneticod works out of the box, and magneticow requires minimal configuration to work with the web server you choose.
  - Detailed, step-by-step manual to guide you through the installation.
- No reliance on any centralised entity:

  - **magneticod** crawls the BitTorrent DHT by "going" from one node to another, and fetches the metadata using the nodes without using trackers.
- Resilience:

  - Unlike client-server model that web applications use, P2P networks are *chaotic* and **magneticod** is designed to handle all the operational errors accordingly.

- High performance implementation:

  - **magneticod** utilizes every bit of your bandwidth to discover as many infohashes & metadata as possible.
- Built-in lightweight web interface:

  - **magneticow** features a lightweight web interface to help you access the database without getting on your way.

Screenshots
-----------
*Click on the images to view full-screen.*

+----------------------------------------------------------------------------------------------------------------------------------------------------+----------------------------------------------------------------------------------------------------------------------------------------------------+----------------------------------------------------------------------------------------------------------------------------------------------------+
|                                                                                                                                                    |                                                                                                                                                    |                                                                                                                                                    |
| .. image:: https://camo.githubusercontent.com/92fbb5a3d5a6310e1c6f415ec33815c5d119fe07/68747470733a2f2f696d6775722e636f6d2f705945474968612e706e67  | .. image:: https://camo.githubusercontent.com/3b8e75d9971fcff877411a951112001d0082fe1c/68747470733a2f2f696d6775722e636f6d2f58337473736a482e706e67  | .. image:: https://camo.githubusercontent.com/227ed6dc241a3c6939ca0e8a441c1b6153f7ba12/68747470733a2f2f696d6775722e636f6d2f74437a33714a6a2e706e67  |
|    :target: https://camo.githubusercontent.com/92fbb5a3d5a6310e1c6f415ec33815c5d119fe07/68747470733a2f2f696d6775722e636f6d2f705945474968612e706e67 |    :target: https://camo.githubusercontent.com/3b8e75d9971fcff877411a951112001d0082fe1c/68747470733a2f2f696d6775722e636f6d2f58337473736a482e706e67 |    :target: https://camo.githubusercontent.com/227ed6dc241a3c6939ca0e8a441c1b6153f7ba12/68747470733a2f2f696d6775722e636f6d2f74437a33714a6a2e706e67 |
|                                                                                                                                                    |                                                                                                                                                    |                                                                                                                                                    |
+----------------------------------------------------------------------------------------------------------------------------------------------------+----------------------------------------------------------------------------------------------------------------------------------------------------+----------------------------------------------------------------------------------------------------------------------------------------------------+
|                                                                                                                                                    |                                                                                                                                                    |                                                                                                                                                    |
| The Homepage                                                                                                                                       |  Searching for torrents                                                                                                                            | Viewing the metadata of a torrent                                                                                                                  |
|                                                                                                                                                    |                                                                                                                                                    |                                                                                                                                                    |
+----------------------------------------------------------------------------------------------------------------------------------------------------+----------------------------------------------------------------------------------------------------------------------------------------------------+----------------------------------------------------------------------------------------------------------------------------------------------------+


Why?
====
BitTorrent, being a distributed P2P file sharing protocol, has long suffered because of the centralised entities that
people dependent on for searching torrents (websites) and for discovering other peers (trackers). Introduction of DHT
(distributed hash table) eliminated the need for trackers, allowing peers to discover peers through other peers and to
fetch metadata from the leechers & seeders in the network. **magnetico** is the finishing move that allows users to
search for torrents in the network & removes the need for torrent websites.

Installation Instructions
=========================
    **WARNING:**

    **magnetico** is still under active construction, and is considered *pre-alpha* software. Please use **magnetico**
    suite with care and follow the installation instructions carefully to install it & secure the installation. Feel
    perfectly free to send bug reports, suggestions, or whatever comes to your mind to send to us through GitHub or
    personal e-mail.
\

    **WARNING:**

    **magnetico** currently does NOT have any filtering system NOR it allows individual torrents to be removed from the
    database, and BitTorrent DHT network is full of the materials that are considered illegal in many countries
    (violence, pornography, copyright infringing content, and even child-pornography). If you are afraid of the legal
    consequences, or simply morally against (indirectly) assisting those content to spread around, follow the
    **magneticow** installation instructions carefully to password-protect the web-interface from others.

1. Install **magneticod** first by following its
   `installation instruction <magneticod/README.rst>`_.
2. Install **magneticow** first by following its
   `installation instruction <magneticow/README.rst>`_.


License
=======
All the code is licensed under AGPLv3, unless otherwise stated in the source specific source. See ``COPYING`` file for
the full license text.

----

Dedicated to Cemile Binay, in whose hands I thrived.

Bora M. ALPER <bora@boramalper.org>