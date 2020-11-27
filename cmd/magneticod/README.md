# magneticod
*Autonomous BitTorrent DHT crawler and metadata fetcher.*

**magneticod** is the daemon that crawls the BitTorrent DHT network in the background to discover info hashes and
fetches metadata from the peers.

## Installation

### Requirements
- Decent Internet access (IPv4)

  **magneticod** uses UDP protocol to communicate with the nodes in the DHT network, and TCP to communicate with the
  peers while fetching metadata. **Please make sure you have a healthy connection;** you can confirm this by checking at
  the *connection status indicator* of your BitTorrent client: if it does not indicate any error (*e.g.* a misconfigured NAT),
  **magneticod** should just work fine.

### Installing the Pre-Compiled Static Binary
You can find the latest pre-compiled static binaries on [GitHub](https://github.com/boramalper/magnetico/releases)
for versions from v0.7.0 onwards. 

### Installing the Docker Image
Docker images are provided on [Docker Hub](https://hub.docker.com/r/boramalper/magnetico/tags/) at
the repository `boramalper/magnetico`. Images are tagged as `d-vMAJOR.MINOR.PATCH`.

## Setup
1. (Optional, **requires root**) Disable iptables for a specified port:
   
   ```bash
   iptables -I OUTPUT -t raw -p udp --sport PORT_NUMBER -j NOTRACK
   iptables -I PREROUTING -t raw -p udp --dport PORT_NUMBER -j NOTRACK
   ```
   
   This is to prevent excessive number of ``EPERM`` "Operation not permitted" errors, which also has a negative impact
   on the performance.

## Usage
### Database
**magneticod** is designed to be able to use different database engines to store its data, but
currently only SQLite 3 and PostgreSQL 9+ are supported.

#### SQLite

The database file can be found in:

- **On Linux**

      ~/.local/share/magneticod/

**magneticod** uses write-ahead logging (WAL) for its database, so there might be multiple
files while it is operating, but ``database.sqlite3`` is *the database*.

#### More engines (PostgreSQL and others)

You can read about other supported persistence engines [here](pkg/README.md).

### Using the Docker Image
You need to mount

- the data directory (`~/.local/share/magneticod/` on Linux, see the previous sections)
- the configuration file at (`~/.config/magneticod/configuration.toml` on Linux, see the previous sections)

hence run:

  ```bash
  docker run -it --rm \
    -v ~/.local/share/magneticod:/root/.local/share/magneticod/ \
    -v ~/.config/magneticod/configuration.toml:/root/.config/magneticod/configuration.toml \
    boramalper/magneticod
  ```
  
### Remark About the Network Usage
**magneticod** does *not* have any built-in rate limiter *yet*, and it will literally suck the hell out of your
bandwidth. Unless you are running **magneticod** on a separate machine dedicated for it, you might want to consider
starting it manually only when network load is low (e.g. when you are at work or sleeping at night).
