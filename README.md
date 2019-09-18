# magnetico
*Autonomous (self-hosted) BitTorrent DHT search engine suite.*

[![chat on gitter](https://badges.gitter.im/gitterHQ/gitter.png)](https://gitter.im/magnetico-dev/magnetico-dev)&emsp;[![Build Status on Travis](https://travis-ci.org/boramalper/magnetico.svg?branch=master)](https://travis-ci.org/boramalper/magnetico)&emsp;[![Build status on AppVeyor](https://ci.appveyor.com/api/projects/status/u2jtbe6jutya7p0x/branch/master?svg=true)](https://ci.appveyor.com/project/boramalper/magnetico/branch/master)&emsp;[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/1029/badge)](https://bestpractices.coreinfrastructure.org/projects/1029)

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
  - [Pre-compiled static binaries](https://github.com/boramalper/magnetico/releases) and [Docker images](https://hub.docker.com/u/boramalper) are provided.
  - Root access is *not* required to install or to use.
- Near-zero configuration:
  - Both programs work out of the box, and **magneticow** can be used without a web-server too.
  - Detailed, step-by-step manual to guide you through the installation.
- No reliance on any centralised entity:
  - **magneticod** trawls the BitTorrent DHT by "going" from one node to another, and fetches the
    metadata using the nodes without using trackers.
- Resilience:
  - Unlike client-server model that web applications use, P2P networks are *chaotic* and
    **magneticod** is designed to handle all the operational errors accordingly.
    - Currently on paper, wait for the v1.0!
- High performance implementation in Go:
  - **magneticod** utilizes every bit of your resources to discover as many infohashes & metadata as
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
> **magnetico** is still under active construction, and is considered *alpha* software. Please
> use **magnetico** suite with care and follow the installation instructions carefully to install
> it & secure the installation. Feel perfectly free to send bug reports, suggestions, or whatever
> comes to your mind to send to us through GitHub or personal e-mail.

> **WARNING:**
>
> Do NOT clone the [repository](https://github.com/boramalper/magnetico) to install **magnetico**,
> as it is never meant to be stable (except
> [releases](https://github.com/boramalper/magnetico/releases) of course).

1. Install **magneticod** first by following its [installation instructions](cmd/magneticod/README.md).
2. Install **magneticow** afterwards by following its
   [installation instructions](cmd/magneticow/README.md).

*Alternatively*, just grab it from [Docker Hub](https://hub.docker.com/u/boramalper)!

## License

All the code is licensed under AGPLv3, unless stated otherwise specifically. See `COPYING` for
details.

## Donations
### Patreon
https://www.patreon.com/boramalper

### PayPal
https://paypal.me/boramalper

### Cryptocurrencies (Coinbase)
- **BTC:** `3BLWjamWug3QQzcDDGwYLwuCqJyjcfYJB8`
- **LTC:** `MRWX5SGCF7EvN15gpzT5b3KQD3Z91gH8qi`
- **BCH:** `qqn07a58hax9l8pckq9j8ys6dsh2cnu4rsyztw2kj9`
- **ETH:** `0xe5A8e80bAA6129DF7eBB1B5302F9e2Ef4C6f6E62`
- **ETC:** `0x8964EcC86eaf043Bff2CdfE875E73D8095c26a58`

----

Dedicated to Cemile Binay, in whose hands I thrived.

Bora M. ALPER <bora@boramalper.org>
