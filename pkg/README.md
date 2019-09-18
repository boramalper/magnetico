# magnetico/pkg
[![GoDoc](https://godoc.org/github.com/boramalper/magnetico?status.svg)](https://godoc.org/github.com/boramalper/magnetico)

- The most significant package is `persistence`, that abstracts access to the
  magnetico databases with different engines (currently, only SQLite).
  
**For REST-ful magneticow API, see [https://app.swaggerhub.com/apis/boramalper/magneticow-api/](https://app.swaggerhub.com/apis/boramalper/magneticow-api/).**

## Stdout Dummy Database Engine for magneticod

Stdout dummy database engine for **magneticod** prints a new [JSON Line](http://jsonlines.org/)
for each discovered torrent so that you can pipe the stdout of **magneticod** into some other
program to build your own pipelines on the fly!

**Example Output**

```json

```
