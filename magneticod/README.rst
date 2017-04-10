==========
magneticod
==========
*Autonomous BitTorrent DHT crawler and metadata fetcher.*

**magneticod** is the daemon that crawls the BitTorrent DHT network in the background to discover info hashes and
fetches metadata from the peers. It uses SQLite 3 that is built-in your Python 3.x distribution to persist data.

Installation
============
Requirements
------------
- Python 3.5 or above.

    **WARNING:**

    Python 3.6.0 and 3.6.1 suffer from a bug (`issue #29714 <http://bugs.python.org/issue29714>`_) that causes
    magneticod to fail. As it is an interpreter bug that I have no control on, please make sure that you are not using
    any of those Python 3 versions to run magneticod.

- Decent Internet access (IPv4)

  **magneticod** uses UDP protocol to communicate with the nodes in the DHT network, and TCP to communicate with the
  peers while fetching metadata. **Please make sure you have a healthy connection;** you can confirm this by checking at
  the *connection status indicator* of your BitTorrent client: if it does not indicate any error, **magneticod** should
  just work fine.

Instructions
------------
1. Download the latest version of **magneticod** from PyPI using pip3: ::

       pip3 install magneticod --user

2. Add installation path to the ``$PATH``; append the following line to your ``~/.profile`` ::

       export PATH=$PATH:~/.local/bin
       
   **or if you are on macOS**, (assuming that you are using Python 3.5): ::
   
        export PATH="${PATH}:${HOME}/Library/Python/3.5/bin/"

3. Activate the changes to ``$PATH``: ::

       source ~/.profile

4. Confirm that it is running: ::

       magneticod

   Within maximum 5 minutes (and usually under a minute) **magneticod** will discover a few torrents! This, of course,
   depends on your bandwidth, and your network configuration (existence of a firewall, misconfigured NAT, etc.).

5. *(only for systemd users, skip the rest of the steps and proceed to the* `Using`_ *section if you are not a systemd
   user or want to use a different solution)*

   Download the magneticod systemd service file (at
   `magneticod/systemd/magneticod.service <systemd/magneticod.service>`_) and change the tilde symbol with
   the path of your home directory. For example, if my username is ``bora``, this line ::

       ExecStart=~/.local/bin/magneticod

   should become this: ::

       ExecStart=/home/bora/.local/bin/magneticod

   Here, tilde (``~``) is replaced with ``/home/bora``. Run ``echo ~`` to see the path of your own home directory, if
   you do not already know.

6. Copy the magneticod systemd service file to your local systemd configuration directory: ::

       cp magneticod.service ~/.config/systemd/user/

   You might need to create intermediate directories (``.config``, ``systemd``, and ``user``) if not exists.

7. Start **magneticod**: ::

       systemctl --user start magneticod

   **magneticod** should now be running under the supervision of systemd and it should also be automatically started
   whenever you boot your machine.

   You can check its status and most recent log entries using the following command: ::

       systemctl --user status magneticod

   To stop **magneticod**, issue the following: ::

       systemctl --user stop magneticod
\

    **Suggestion:**

    Keep **magneticod** running so that when you finish installing **magneticow**, database will be populated and you
    can see some results.

Using
=====
**magneticod** does not require user interference to operate, once it starts running. Hence, there is no "user manual",
although you should beware of these points:

1. **Network Usage:**

   **magneticod** does *not* have any built-in rate limiter *yet*, and it will literally suck the hell out of your
   bandwidth. Unless you are running **magneticod** on a separate machine dedicated for it, you might want to consider
   starting it manually only when network load is low (e.g. when you are at work or sleeping at night).

2. **Pre-Alpha Bugs:**

   **magneticod** is *supposed* to work "just fine", but as being at pre-alpha stage, it's likely that you might find
   some bugs. It will be much appreciated if you can report those bugs, so that **magneticod** can be improved. See the
   next sub-section for how to mitigate the issue if you are *not* using systemd.

Automatic Restarting
--------------------
Due to minor bugs at this stage of its development, **magneticod** should be supervised by another program to be ensured
that it's running, and should be restarted if not. systemd service file supplied by **magneticod** implements that,
although (if you wish) you can also use a much more primitive approach using GNU screen (which comes pre-installed in
many GNU/Linux distributions):

1. Start screen session named ``magneticod``: ::

       screen -S magneticod

2. Run **magneticod** forever: ::

       until magneticod; do echo "restarting..."; sleep 5; done;

   This will keep restarting **magneticod** after five seconds in case if it fails.

3. Detach the session by pressing Ctrl+A and after Ctrl+D.

4. If you wish to see the logs, or to kill **magneticod**, ``screen -r magneticod`` will attach the original screen
   session back. **magneticod** will exit gracefully upon keyboard interrupt (Ctrl+C) [SIGINT].

Database
--------
**magneticod** uses SQLite 3 that is built-in by default in almost all Python distributions.
`appdirs <https://pypi.python.org/pypi/appdirs/>`_ package is used to determine user data directory, which is often
``~/.local/share/magneticod``. **magneticod** uses write-ahead logging for its database, so there might be multiple
files while it is operating, but ``database.sqlite3`` is *the main database where every torrent metadata is stored*.

License
=======
All the code is licensed under AGPLv3, unless otherwise stated in the source specific source. See ``COPYING`` file
in ``magnetico`` directory for the full license text.

----

Dedicated to Cemile Binay, in whose hands I thrived.

Bora M. ALPER <bora@boramalper.org>
