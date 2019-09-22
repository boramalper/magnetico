# magnetico/pkg
[![GoDoc](https://godoc.org/github.com/boramalper/magnetico?status.svg)](https://godoc.org/github.com/boramalper/magnetico)

- The most significant package is `persistence`, that abstracts access to the
  magnetico databases with different engines (currently, SQLite, stdout and partly PostgreSQL).
  
**For REST-ful magneticow API, see [https://app.swaggerhub.com/apis/boramalper/magneticow-api/](https://app.swaggerhub.com/apis/boramalper/magneticow-api/).**

## PostgreSQL database engine (only `magneticod` part implemented)

PostgreSQL database engine uses [PostgreSQL](https://www.postgresql.org/) to store indexed
torrents. It's more performant and flexible than SQLite but requires additional software configuration.

**WARNING:** `pg_trgm` extension required. You need to enable it for your database before starting `magneticod`:

```postgresql
CREATE EXTENSION pg_trgm;
```

Engine usage example:

```shell
magneticod --database=postgres://username:password@127.0.0.1:5432/database?schema=custom_schema_name&sslmode=disable
```

See [pgx repository](https://github.com/jackc/pgx/blob/master/stdlib/sql.go)
for more examples.

Optional parameter `schema` was added to choose which schema will be used to store magnetico tables,
sequences and indexes.

## Beanstalk MQ engine for magneticod

[Beanstalkd](https://beanstalkd.github.io/) is very lightweight and simple MQ server implementation.
You can use it to organize delivery of the indexed data to your application.

Use `beanstalk` URL schema to connect to beanstalkd server. For example:

```shell
magneticod --database=beanstalkd://127.0.0.1:11300/magneticod_tube
```

Don't forget to [set](https://linux.die.net/man/1/beanstalkd) binlog persistence, change maximum job size
and `fsync()` period to be able to reliably save torrents with a large number of files:

```shell
# Example settings (may not work for you)
beanstalkd -z 1048560 -b /var/lib/beanstalkd -f 2400000
```

For job data example see `stdout` engine documentation below as `beanstalk` engine uses the same format.

## Stdout Dummy Database Engine for magneticod

Stdout dummy database engine for **magneticod** prints a new [JSON Line](http://jsonlines.org/)
for each discovered torrent so that you can pipe the stdout of **magneticod** into some other
program to build your own pipelines on the fly!

**Example Output**

```json
{"infoHash":"f84b51f0d2c3455ab5dabb6643b4340234cd036e","name":"Big_Buck_Bunny_1080p_surround_frostclick.com_frostwire.com","files":[{"size":928670754,"path":"Big_Buck_Bunny_1080p_surround_FrostWire.com.avi"},{"size":5008,"path":"PROMOTE_YOUR_CONTENT_ON_FROSTWIRE_01_06_09.txt"},{"size":3456234,"path":"Pressrelease_BickBuckBunny_premiere.pdf"},{"size":180,"path":"license.txt"}]}
```

> **WARNING:**
>
> Please beware that the schema of the object (dictionary) might change in backwards-incompatible ways 
> in the future; although I'll do my best to ensure it won't happen.
