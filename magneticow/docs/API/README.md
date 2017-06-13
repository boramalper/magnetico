# magnetico<sup>w</sup> API Documentation

**magneticow** offers a REST-ful HTTP API for 3rd-party applications to interact with **magnetico** setups. Examples
would be an Android app for searching torrents **magnetico** discovered and queueing them on your seedbox, or a custom
data analysis/statistics application developed for a research project on BitTorrent network. Nevertheless, it is up to
you what to do with it at the end of the day.

The rules stated above below to the API as a whole and across the all versions:

* The API root is `/api`.
* Right after the API root MUST come the API version in the format `vX` (*e.g.* `/api/v1`).
* Different API versions MAY be backwards-incompatible, but any changes within the same version of the API MUST NOT
  break the backwards-compatibility.
* Version 0 (zero) of the API is considered to be experimental and MAY be backwards-incompatible.
* API documentation MUST be considered as a contract between the developers of **magnetico** and **magneticow**, and of
  3rd party application developers, and MUST be respected as such.

The documentation for the API is organised as described below:

* Each version of the API MUST be documented in a separate document named `vX.md`. Everything (*i.e.* each
  functionality, status codes, etc.) MUST be clearly indicated when they are introduced.
* Each document MUST clearly indicate at the beginning whether it is *finalized* or not. Not-finalised documents (called
  *Draft*) CAN be changed, and finalised , but once finalised documents MUST NOT be modified afterwards.
* Documentation for the version 0 (zero) of the API MUST be considered free from the rules above, and always considered
  a *draft*.
* Each document MUST be self-standing, that is, MUST be completely understandable and unambiguous without requiring to
  refer another document.
  * Hence, use quotations when necessary and reference them.

Remarks:

* Use British English, and serial comma.
* Documents should be formatted in GitHub Flavoured Markdown.