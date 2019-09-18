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
{"infoHash":"f84b51f0d2c3455ab5dabb6643b4340234cd036e","name":"Big_Buck_Bunny_1080p_surround_frostclick.com_frostwire.com","files":[{"size":928670754,"path":"Big_Buck_Bunny_1080p_surround_FrostWire.com.avi"},{"size":5008,"path":"PROMOTE_YOUR_CONTENT_ON_FROSTWIRE_01_06_09.txt"},{"size":3456234,"path":"Pressrelease_BickBuckBunny_premiere.pdf"},{"size":180,"path":"license.txt"}]}
```

> **WARNING:**
>
> Please beware that the schema of the object (dictionary) might change in backwards-incompatible ways 
> in the future; although I'll my best to ensure it won't happen.
