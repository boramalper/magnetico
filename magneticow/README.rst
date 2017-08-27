==========
magneticow
==========
*Lightweight web interface for magnetico.*

**magneticow** is a lightweight web interface to search and to browse the torrents that its counterpart (**magneticod**)
discovered. It allows fast full text search of the names of the torrents, by correctly parsing them into their elements.

Installation
============
**magneticow** uses `gevent <http://www.gevent.org/>`_ as a "standalone WSGI container" (you can think of it as an
embedded HTTP server), and connects to the same SQLite 3 database that **magneticod** writes. Hence, **root or sudo
access is NOT required at any stage, during or after the installation process.**

Requirements
------------
- Python 3.5 or above.

Instructions
------------
    **WARNING:**

    **magnetico** currently does NOT have any filtering system NOR it allows individual torrents to be removed from the
    database, and BitTorrent DHT network is full of the materials that are considered illegal in many countries
    (violence, pornography, copyright infringing content, and even child-pornography). If you are afraid of the legal
    consequences, or simply morally against (indirectly) assisting those content to spread around, follow the
    **magneticow** installation instructions carefully to password-protect the web-interface from others.
\

    **WARNING:**

    **magneticow** is *NOT* designed to scale, and will fail miserably if you try to use it like a public torrent
    website. This is a *deliberate* technical decision, not a bug or something to be fixed; another web interface with
    more features to support such use cases and scalability *might* be developed, but **magneticow** will NEVER be the
    case.

1. Download the latest version of **magneticow** from PyPI: ::

       pip3 install magneticow --user

2. Add installation path to the ``$PATH``; append the following line to your ``~/.profile`` if you are using bash
   *(you can skip to step 4 if you installed magneticod first as advised)* ::

       export PATH=$PATH:~/.local/bin
       
   **or if you are on macOS**, (assuming that you are using Python 3.5): ::
   
        export PATH="${PATH}:${HOME}/Library/Python/3.5/bin/"

3. Activate the changes to ``$PATH`` (again, if you are using bash): ::

       source ~/.profile

4. Confirm that it is running: ::

       magneticow  --port 8080 --user username_1 password_1 --user username_2 password_2

   Do not forget to actually visit the website, and run a search without any keywords (i.e. simply press the enter
   button); this should return a list of most recently discovered torrents. If **magneticod** has not been running long
   enough, database might be completely empty and you might see no results (5 minutes should suffice to discover more
   than a dozen torrents).

5. *(only for systemd users, skip the rest of the steps and proceed to the* `Using`_ *section if you are not a systemd
   user or want to use a different solution)*

   Download the magneticow systemd service file (at
   `magneticow/systemd/magneticow.service <systemd/magneticow.service>`_) and expand the tilde symbol with the path of
   your home directory. Also, choose a port (> 1024) for **magneticow** to listen on, and supply username(s) and
   password(s).

   For example, if my home directory is ``/home/bora``, and I want to create two users named ``bora`` and ``tolga`` with
   passwords ``staatsangehörigkeit`` and ``bürgerschaft``, and then **magneticow** to listen on port 8080, this line ::

       ExecStart=~/.local/bin/magneticow --port PORT --user USERNAME PASSWORD

   should become this: ::

       ExecStart=/home/bora/.local/bin/magneticow --port 8080 --user bora staatsangehörigkeit --user tolga bürgerschaft

   Run ``echo ~`` to see the path of your own home directory, if you do not already know.

       **WARNING:**

       **At least one username and password MUST be supplied.** This is to protect the privacy of your **magneticow**
       installation, although **beware that this does NOT encrypt the communication between your browser and the
       server!**

6. Copy the magneticow systemd service file to your local systemd configuration directory: ::

       cp magneticow.service ~/.config/systemd/user/

7. Start **magneticow**: ::

       systemctl --user enable magneticow --now

   **magneticow** should now be running under the supervision of systemd and it should also be automatically started
   whenever you boot your machine.

   You can check its status and most recent log entries using the following command: ::

       systemctl --user status magneticow

   To stop **magneticow**, issue the following: ::

       systemctl --user stop magneticow

Alternate instructions
======================

For building standalone static **magneticow** binaries (using pyinstaller): ::

       docker build -t magneticow_builder -f Dockerfile.build .
       docker run --rm -t magneticow_builder | base64 -d > magneticow
       chmod +x magneticow

You can now start **magneticow** by executing ``./magneticow``, as you normally would. If you get the ``magneticow is a folder``, try to move to a different directory with no **magneticow** folder.

Using
=====
**magneticow** does not require user interference to operate, once it starts running. Hence, there is no "user manual",
although you should beware of these points:

1. **Resource Usage:**

   To repeat it for the last time, **magneticow** is a lightweight web interface for magnetico and is not suitable for
   handling many users simultaneously. Misusing **magneticow** will likely to lead high processor usage and increased
   response times. If that is the case, you might consider lowering the priority of **magneticow** using ``renice``
   command.

2. **Pre-Alpha Bugs:**

   **magneticow** is *supposed* to work "just fine", but as being at pre-alpha stage, it's likely that you might find
   some bugs. It will be much appreciated if you can report those bugs, so that **magneticow** can be improved. See the
   next sub-section for how to mitigate the issue if you are *not* using systemd.

Automatic Restarting
--------------------
Due to minor bugs at this stage of its development, **magneticow** should be supervised by another program to be ensured
that it's running, and should be restarted if not. systemd service file supplied by **magneticow** implements that,
although (if you wish) you can also use a much more primitive approach using GNU screen (which comes pre-installed in
many GNU/Linux distributions):

1. Start screen session named ``magneticow``: ::

       screen -S magneticow

2. Run **magneticow** forever: ::

       until magneticow; do echo "restarting..."; sleep 5; done;

   This will keep restarting **magneticow** after five seconds in case if it fails.

3. Detach the session by pressing Ctrl+A and after Ctrl+D.

4. If you wish to see the logs, or to kill **magneticow**, ``screen -r magneticow`` will attach the original screen
   session back. **magneticow** will exit gracefully upon keyboard interrupt (Ctrl+C) [SIGINT].

Searching
---------
* Only the **titles** of the torrents are being searched.
* Search is case-insensitive.
* Titles that includes terms that are separated by space are returned from the search:

  Example: ``king bad`` returns ``Stephen King - The Bazaar of Bad Dreams``

  * If you would like terms to appear in the exact order you wrote them, enclose them in double quotes:

    Example: ``"king bad"`` returns ``George Benson - Good King Bad``
* Use asteriks (``*``) to denote prefixes:

  Example: ``The Godf*`` returns ``Francis Ford Coppola - The Godfather``

  Asteriks works inside the double quotes too!
* Use caret (``^``) to indicate that the term it prefixes must be the first word in the title:

  Example: ``linux`` returns ``Arch Linux`` while ``^linux`` would return ``Linux Mint``

  * Caret works **inside** the double quotes **but not outside**:

    Right: ``"^ubuntu linux"``

    Wrong: ``^"ubuntu linux"``
* You can use ``AND``, ``OR`` and ``NOT`` and also parentheses for more complex queries:

  Example: ``programming NOT (windows OR "os x" OR macos)``

  Beware that the terms are all-caps and MUST be so.

  =======================    =======================================
  Operator Enhanced Query    Syntax Precedence
  =======================    =======================================
  NOT                        Highest precedence (tightest grouping).
  AND
  OR                         Lowest precedence (loosest grouping).
  =======================    =======================================

REST-ful HTTP API
=================
    **magneticow** offers a REST-ful HTTP API for 3rd-party applications to interact with **magnetico** setups. Examples
    would be an Android app for searching torrents **magnetico** discovered and queueing them on your seedbox, or a
    custom data analysis/statistics application developed for a research project on BitTorrent network. Nevertheless, it
    is up to you what to do with it at the end of the day.

See `API documentation <./docs/API/README.md>`_ for more details.

License
=======
All the code is licensed under AGPLv3, unless otherwise stated in the source specific source. See ``COPYING`` file
in ``magnetico`` directory for the full license text.


----

Dedicated to Cemile Binay, in whose hands I thrived.

Bora M. ALPER <bora@boramalper.org>
