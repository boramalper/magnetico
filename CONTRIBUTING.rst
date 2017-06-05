=========================
Contributing to magnetico
=========================

Hello there! We would be glad to accept your contributions and this is document is a guide to help you throughout the
process of contributing.

Kinds of Contributions
======================
There are several kinds of contributions and feel free to choose ones that you feel most comfortable with:

1. Programming

   If you are a **Python** programmer, great! **JavaScript** programmers are too welcome. **magneticod** and
   **magneticow** are programmed in Python 3, and we use a tiny tiny bit of JavaScript for the web interface offered by
   **magneticow**. We are trying to keep the amount of JavaScript used to the bare minimum (so that users with NoScript
   and JavaScript disabled can still use it), so if your contribution is refused, please do not take it personally.

2. Testing

   **magnetico** is still a pre-v1.0 software, hence is considered unstable. Currently, developers are testing
   **magnetico** themselves before each version and this is a very tiresome task. Also, due to lack of resources and
   diversity of different setups we have, we cannot test **magnetico** extensively. If you would like to test
   **magnetico** for us, we would be grateful.

3. User Interface Design & User Experience

   **magnetico** is not the first DHT search engine, but "the first autonomous (self-hosted) BitTorrent DHT search
   engine suite that is designed **for end-users**." We care about *end-users* and value their experience while using
   **magnetico** suite. Ease of installation, and of use are our primary concerns and any contributions to improve these
   experiences are much welcome.

Things Every Contributor Should Know
====================================
* Join `magnetico-dev gitter channel <https://gitter.im/magnetico-dev/magnetico-dev>`_ to join the conversation. We
  value every opinion.
* Let people know what you are planning to do, especially before undertaking a huge task. It is very discouraging to
  spend your time on a daunting task and to see it refused. Asking people before acting would prevent such situations
  and easier for both parties to be prepared.
* Do not argue against the first principles of the project. The **magnetico** project is not just a piracy tool or
  whatever you think it to be, but it is a **self-hosted** DHT search engine, to serve as another core component of the
  BitTorrent network, to resist censorship and protect users' privacy. Any attempt to violate these principles in favour
  of anything else (which you might think to be more *practical*) will be firmly refused.

Python Coding Guidelines
========================
.. image:: https://api.travis-ci.org/boramalper/magnetico.svg?branch=master
   :target: https://travis-ci.org/boramalper/magnetico

* In general, we follow `PEP 8 <https://www.python.org/dev/peps/pep-0008/>`_ and
  `Google Python Style Guide <https://google.github.io/styleguide/pyguide.html>`_.
* Prefer this document over Google Python Style Guide over PEP 8 in case of conflict.



* Maximum line length is 120 characters.
* Do not abbreviate variable names, unless the abbreviation is famous (even though it might be *obvious* in its code
  context). For instance, `HTTP` is accepted but `_f` for *futures* and `p_` for *parent* are NOT.
  * A possible exception of this rule is to use shorter names in case of complex symbolic manipulation and/or
  operations, for instance in complex for-loops, functions etc. Shorter names would allow the programmer to follow the
  flow of the operations and logic behind the manipulation more easily and hence justified. But, comments are required
  in those cases.
* Do NOT use shebang line to specify the interpreter or the encoding of the file. The former is handled by the Python's
  setuptools and latter is UTF-8, for every and each source file.
* Prefer Python standard library over 3rd party solutions, unless justified.



* We use `Travis CI <https://travis-ci.org/boramalper/magnetico>`_ to automatically run tests on the latest code. We use

  * `pylint <https://www.pylint.org/>`_
  * `mypy <http://mypy-lang.org/>`_

  for overall code quality and static type checking. Both are very powerful tools and we depend on them a lot to prevent
  unforeseen bugs at "compile" time. Please make sure that your changes do NOT introduce new warnings. Fixing the old
  code that caused warnings are also much welcome.
* **Type-annotate all function signatures (all arguments and return values).**

Testing and User Interface Design & User Experience
===================================================
As we lack both experience and contributors, we cannot even write contributions guidelines. =)

Just shoot us a message if you are interested in and let's discuss.

----

**P.S.** If you feel overwhelmed after seeing this document, don't. Send your first contributions and I'll personally
help you through your first contribution. =)

Bora, <bora@boramalper.org>
