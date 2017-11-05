# magnetico
*Autonomous (self-hosted) BitTorrent DHT search engine suite.*

[![chat on gitter](https://badges.gitter.im/gitterHQ/gitter.png)](https://gitter.im/magnetico-dev/magnetico-dev)

magnetico is the first autonomous (self-hosted) BitTorrent DHT search engine suite that is *designed
for end-users*. The suite consists of two packages:

- **magneticod:** Autonomous BitTorrent DHT crawler and metadata fetcher.
- **magneticow:** Lightweight web interface for magnetico.

Both programs, combined together, allows anyone with a decent Internet connection to access the vast
amount of torrents waiting to be discovered within the BitTorrent DHT space, *without relying on any
central entity*.

**magnetico** liberates BitTorrent from the yoke of centralised trackers & web-sites and makes it
*truly decentralised*. Finally!

## Features
- Easy installation & minimal requirements:
  - Python 3.5+ and a few Python packages that is available on PyPI.
  - Root access is *not* required to install.
- Near-zero configuration:
  - magneticod works out of the box, and magneticow requires minimal configuration to work with the
    web server you choose.
  - Detailed, step-by-step manual to guide you through the installation.
- No reliance on any centralised entity:
  - **magneticod** crawls the BitTorrent DHT by "going" from one node to another, and fetches the
    metadata using the nodes without using trackers.
- Resilience:
  - Unlike client-server model that web applications use, P2P networks are *chaotic* and
    **magneticod** is designed to handle all the operational errors accordingly.
- High performance implementation:
  - **magneticod** utilizes every bit of your bandwidth to discover as many infohashes & metadata as
    possible.
- Built-in lightweight web interface:
  - **magneticow** features a lightweight web interface to help you access the database without
    getting on your way.

### Screenshots
*Click on the images to view full-screen.*

<!-- Use https://www.tablesgenerator.com/markdown_tables -->
| ![The Homepage](https://camo.githubusercontent.com/488606a87a3e1d7238c0539c6b9cf8429e2c8f16/68747470733a2f2f696d6775722e636f6d2f3634794433714e2e706e67) | ![Searching for torrents](https://camo.githubusercontent.com/0b6def355a17b944de163a11f77c17c1c622280c/68747470733a2f2f696d6775722e636f6d2f34786a733335382e706e67) | ![ss](https://camo.githubusercontent.com/0bd679ad8bbf038b50c082d80a8e0e37516c813e/68747470733a2f2f696d6775722e636f6d2f6c3354685065692e706e67) |
|:-------------------------------------------------------------------------------------------------------------------------------------------------------:|:-----------------------------------------------------------------------------------------------------------------------------------------------------------------:|:---------------------------------------------------------------------------------------------------------------------------------------------:|
|                                                                     __The Homepage__                                                                    |                                                                     __Searching for torrents__                                                                    |                                                     __Viewing the metadata of a torrent__                                                     |

## Why?
BitTorrent, being a distributed P2P file sharing protocol, has long suffered because of the
centralised entities that people depended on for searching torrents (websites) and for discovering
other peers (trackers). Introduction of DHT (distributed hash table) eliminated the need for
trackers, allowing peers to discover each other through other peers and to fetch metadata from the
leechers & seeders in the network. **magnetico** is the finishing move that allows users to search
for torrents in the network, hence removing the need for centralised torrent websites.

## Installation Instructions
> **WARNING:**
>
> **magnetico** is still under active construction, and is considered *pre-alpha* software. Please
> use **magnetico** suite with care and follow the installation instructions carefully to install
> it & secure the installation. Feel perfectly free to send bug reports, suggestions, or whatever
> comes to your mind to send to us through GitHub or personal e-mail.


> **WARNING:**
>
> **magnetico** currently does NOT have any filtering system NOR it allows individual torrents to be
> removed from the database, and BitTorrent DHT network is full of the materials that are considered
> illegal in many countries (violence, pornography, copyright infringing content, and even
> child-pornography). If you are afraid of the legal consequences, or simply morally against
> (indirectly) assisting those content to spread around, follow the **magneticow** installation
> instructions carefully to password-protect the web-interface from others.

> **WARNING:**
>
> Do NOT clone the [repository](https://github.com/boramalper/magnetico) to install **magnetico**,
> as it is never meant to be stable (except
> [releases](https://github.com/boramalper/magnetico/releases) of course).

1. Install **magneticod** first by following its [installation instructions](magneticod/README.md).
2. Install **magneticow** afterwards by following its
   [installation instructions](magneticow/README.rst).

## License

All the code is licensed under AGPLv3, unless otherwise stated in the source specific source. See
`COPYING` file for the full license text.

----

Dedicated to Cemile Binay, in whose hands I thrived.

Bora M. ALPER <[bora@boramalper.org](mailto:bora@boramalper.org)>
