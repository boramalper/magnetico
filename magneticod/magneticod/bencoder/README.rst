This sub-module is a fork of Krzysztof Kosyl's [better-bencode](https://github.com/kosqx/better-bencode/) for the
specific needs of **magetico**.

The original repository is forked at commit `46bdc09f1b3003b39aa4263e0a052883a5209c2a`.

Key Differenes from *better-bencode*:

* Python 2 support is removed.
* Removed `dump` and `load` functions, as they are not used and most likely will not be maintained. It's better not to
  have them than to have two different set of functions with inconsistent, confusing behaviour.
