# Commands: Worlds and Connections

A *world* is a saved definition (name, host, port, character, password, type); a *socket* is an open connection to one. At any moment at most one socket is in the *foreground*, its text is on screen and your typing goes to it; others run in the *background* and queue their output. These commands define worlds, connect and disconnect, and switch between sockets.

The world database is kept by the engine, so defining, listing, renaming and removing worlds works in any embedding. Connections belong to the embedding program, which supplies them through `WorldHost` (`/connect`, `/dc`, `/fg`) and the optional `ConnectHost`, `FgHost` and `SocketHost` (options and the socket list); see `docs/engine/state-builtins.md`. With no socket support the commands report that there are no sockets, and `/connect` itself says it is not supported by this client. The examples below run in a bare interpreter, so they cover the database and the no-socket answers. Entries marked *Library* are macros from tf's `stdlib.tf`.

## /addworld [-pxe] [-Ttype] [-ssrchost] name [char pass] host port [file]  *(Library)*

**What it is for:** defining a world: once defined, `/connect name` (or `/world name`) opens it, and the type and login fields drive automatic login.

**Syntax:**

- `/addworld [-pxe] [-Ttype] [-ssrchost] name [char pass] host port [file]`
- `/addworld [-Ttype] [-ssrchost] name` (a connectionless world: no host or port)
- `/addworld [-Ttype] DEFAULT [char pass [file]]`

**Arguments/Returns:** `name` has no spaces and (for a new world) does not begin with `(`. `type` is any label; the library's login hooks recognise `tiny` (`connect char pass`), `lp`/`diku`/`aber` (name and password on separate lines, `lp` on), `lpp`, and `telnet` (answers `login:`/`password:` prompts), each also as a prefix `type.something`. `-p` ignore `%proxy_host`; `-x` use SSL; `-e` echo sent text back as if received (handy for connectionless worlds); `-ssrchost` the local address to bind. A world called `DEFAULT` supplies the character, password, type and file for worlds that leave them out. The command is a library macro that calls the [`addworld()`](Functions-World-and-Connection.md#addworldname-type-host-port-char-pass-file-flags-srchost) function.

```tf
/addworld -Ttiny Cave cave.example.org 4201
/addworld -Ttiny Hub bob swordfish hub.example.org 4000
/addworld Scratch
/listworlds -s
```
```output
Cave
Hub
Scratch
```

**Gotchas:** Redefining a world replaces all of its fields (`% addworld: Redefined world Cave`). Types are matched by glob, so choose `tiny.muck.` style names with a trailing dot if you want `-T'tiny.muck.*'` triggers and the stock `tiny*` hooks to both fire. Passwords in a file readable by others earn a warning.

**Status:** see [Status](Status.md) (no host capability needed)

## /addtiny, /addlp, /addlpp, /adddiku, /addtelnet  *(Library)*

**What they are for:** `/addworld` with the type already filled in: `-Ttiny`, `-Tlp`, `-Tlpp`, `-Tdiku`, `-Ttelnet`.

**Syntax:** the same arguments as `/addworld`, e.g. `/addtiny Cave bob swordfish cave.example.org 4201`.

```tf
/addtelnet Library library.example.org 23
/echo $[world_info("Library", "type")]
```
```output
telnet
```

**Status:** see [Status](Status.md) (no host capability needed: the engine keeps the world database)

## /unworld name...  (and /purgeworld)

**What it is for:** forgetting world definitions.

**Syntax:** `/unworld name...`; Library `/purgeworld pattern...` removes every world whose name matches a glob pattern.

**Arguments/Returns:** returns the number of worlds removed. An unknown name prints `UNWORLD: No world name` and is skipped. The removed world's history is deleted (lines may survive in the global history).

```tf
/addworld Scratch
/addworld Tmp1
/addworld Tmp2
/unworld Scratch
/purgeworld Tmp*
/listworlds -s
/echo [$[world_info("Tmp1", "host")]]
```
```output
[]
```

**Status:** see [Status](Status.md) (no host capability needed: the engine keeps the world database)

## /renameworld old new

**What it is for:** giving a world a new name without redefining it.

**Syntax:** `/renameworld old new`; prints `% World renamed.` on success.

```tf
/addworld Hub hub.example.org 4000
/renameworld Hub Hub2
/listworlds -s
```
```output
% World renamed.
Hub2
```

**Status:** see [Status](Status.md) (no host capability needed: the engine keeps the world database)

## /listworlds [-cus] [-mstyle] [-Sfield] [-Ttype] [name]

**What it is for:** reviewing what you have defined; also the standard way to turn the world list into a list a macro can loop over.

**Syntax:** `/listworlds [-cus] [-mstyle] [-Sfield] [-Ttype] [name]`

**Arguments/Returns:** with no options it prints a table (passwords hidden). `-s` prints names only; `-c` prints the commands that would recreate each world (passwords included); `-u` also lists unnamed temporary worlds (`(unnamed1)`). `-Sfield` sorts by `name` (default), `type`, `character`, `host`, `port`, or `-` for no sorting (first letter is enough). `-Ttype` and `name` are patterns (style from `-m`, default `%matching`). Returns the number of worlds listed.

```tf
/addworld -Ttiny Cave cave.example.org 4201
/addworld -Ttelnet Library library.example.org 23
/listworlds -s -Ttiny
/listworlds -c Cave
/listworlds
```
```output
Cave
/test addworld("Cave", "tiny", "cave.example.org", "4201")
NAME             TYPE        HOST                                           PORT
Cave             tiny        cave.example.org                               4201
Library          telnet      library.example.org                              23
```

**Gotchas:** `DEFAULT` shows up in the list like any world. `-c` output is what `/saveworld` writes. Used by the library: `/world` with no argument connects the first world returned by `/listworlds -s`.

**Status:** see [Status](Status.md) (no host capability needed: the engine keeps the world database)

## /connect [-lqxbf] [world]  and  /connect host port

**What it is for:** opening a connection.

**Syntax:** `/connect [-lqxbf] [world]` or `/connect host port`.

**Arguments/Returns:** connects to a world defined with `/addworld` that is not already open; with no argument the first defined world. If the world has no host and port it creates a "connectionless" socket (a local window). The `host port` form creates a temporary world named `(unnamedN)`, forgotten when closed. Options: `-l` no auto-login, `-q` quiet login, `-x` SSL, `-f` bring to the foreground, `-b` stay in the background (default: foreground when typed at the keyboard, background when run from a background hook). Returns `0` on failure, `1` on immediate success, `2` if name lookup or the network connection is still in progress.

After a successful connection tf loads the world's file (if any), runs the `CONNECT` hook, and, if `%login` is on and a character and password are defined, the `LOGIN` hook; the stock library hooks implement `tiny`/`lp`/`telnet` logins from the world's type. Failures run `CONFAIL` (and `ICONFAIL` for each failed address when a host has several).

```tf-noverify
/addworld -Ttiny Cave bob swordfish cave.example.org 4201
/connect Cave
```
```output
% Trying to connect to Cave: 192.0.2.7 4201
% Connected to Cave.
```

**Gotchas:** A socket opened in the background does not jump to the foreground when it connects; define a `CONNECT` hook running `/fg %{1}` if you want that. `/connect` to an already open world is an error. Related variables: `%gethostbyname`, `%connect` (blocking or nonblocking). The embedding program owns the network: this command defines the temporary world if needed and asks the host to connect (`WorldHost.Connect`; `ConnectHost.ConnectWith` for the options). With no `WorldHost` it prints `/connect: not supported by this client`.

**Status:** see [Status](Status.md) (host capability: WorldHost)

## /dc [world | -all]

**What it is for:** closing a connection.

**Syntax:** `/dc` (current world), `/dc world`, `/dc -all`.

**Arguments/Returns:** disconnects the named world, the current world, or all of them. With `%quitdone` on, disconnecting the last socket quits tf. The `DISCONNECT` hook is *not* run by `/dc` (it runs when the *server* closes the connection). Real tf prints `% Connection to name closed.`

```tf-noverify
/dc Cave
/dc -all
```

With no sockets, `/dc` complains that there is no current socket, and `/dc Cave` says `Not connected to Cave`.

**Status:** see [Status](Status.md) (host capability: WorldHost; `SocketHost` for `-all`)

## /fg [-nsq<>l] [-cN] [world]  and  /bg

**What it is for:** choosing which open socket you are looking at.

**Syntax:** `/fg world`, `/fg -n`, `/fg -<`, `/fg ->`, `/bg`.

**Arguments/Returns:** `/fg world` brings an already-connected world's socket to the foreground; lines that arrived while it was in the background are shown or counted at the "more" prompt (`-q` jumps straight to the end instead). `-n` puts every socket in the background (so does `/bg`, a library alias for `/fg -n`); `-<` and `->` go to the previous or next socket in the cycle (`-cN` repeats N times); `-s` suppresses errors. Returns nonzero and runs the `WORLD` hook on success, `0` otherwise. `%textdiv` controls the dividing line between old and new text.

```tf-noverify
/fg Cave
/echo $[fg_world()]
/bg
/echo [$[fg_world()]]
```
```output
Cave
[]
```

**Gotchas:** `/bg` is bound to `^]` by default. `/fg` on an unconnected world fails; use `/world` to connect-or-switch (`no socket named 'Cave'` is what a host without sockets reports).

**Status:** see [Status](Status.md) (host capability: WorldHost; `FgHost` for `-q`, `SocketHost` for `-<`/`->`)

## /world [-lqnxfb] [world]  and  /world host port  *(Library)*

**What it is for:** the do-what-I-mean command: switch to the world if it is already open, otherwise connect to it.

**Syntax:** `/world [-lqnxfb] [world]`, `/world host port`.

**Arguments/Returns:** if `world` is open (per `is_open()`), runs `/fg` (passing along `-n` and `-q` if given); otherwise `/connect` (passing along any of `-l -q -x -f -b`). With no argument it picks the first world in `/listworlds -s` (skipping `DEFAULT`).

```tf-noverify
/world Cave
```

**Status:** see [Status](Status.md) (host capability: WorldHost)

## /listsockets [-sn] [-mstyle] [-Sfield] [-Ttype] [name]

**What it is for:** seeing your open connections, which is in the foreground, and how much unseen text each has.

**Syntax:** `/listsockets [-sn] [-mstyle] [-Sfield] [-Ttype] [name]`

**Arguments/Returns:** `-s` prints world names only (the form to use in macros); `-n` shows addresses numerically. Sort with `-Sfield`: `name`, `type`, `character`, `host`, `port`, `lines`, `idle`. Without `-s` a table is printed:

```tf-noverify
/listsockets
```
```output
       LINES IDLE TYPE      NAME            HOST                       PORT
  10+  48  13h tiny.muck Cave            tcp.com                    2283
*  foregnd   1m tiny.mush DeepSeas        muds.okstate.edu           6250
 ?       0  15s tiny      SlowMUD         slow.machine.com           4201
```

`*` marks the current socket. The next column is the state: `!` dead, `?` connecting, `C` connected, `S` in telnet subnegotiation, `X` SSL, `O` connectionless (lower case when MCCP compression is on); a `P` after it means the connection is proxied. LINES is old+new unseen lines for background sockets, or `foregnd`. Returns the number of sockets listed. With no sockets it prints `Not connected to any sockets.`

**Status:** see [Status](Status.md) (host capability: SocketHost)

## /telnet host [port]  *(Library)*

**What it is for:** a quick line-based telnet session to any host.

**Syntax:** `/telnet host [port]` (default port 23): defines a temporary telnet-type world named `host,port` and connects to it.

**Gotchas:** tf is line-oriented; full-screen or single-key programs on the remote side will not work. TF answers the `TTYPE` request with `TINYFUGUE`, `ANSI-ATTR`, `ANSI`, `UNKNOWN`.

```tf-noverify
/telnet towel.blinkenlights.nl
```

**Status:** see [Status](Status.md) (host capability: WorldHost)

## /retry world [delay], /retry_off [world]  *(Library)*

**What it is for:** persistently reconnecting to a flaky game.

**Syntax:** `/retry world [delay]` connects now and, whenever the connection attempt fails (`CONFAIL`), tries again after `delay` seconds (default 60) until it succeeds. `/retry_off [world]` cancels the retrying for that world (all worlds if omitted).

```tf-noverify
/retry Cave 30
```

**Gotchas:** The retry is implemented with hooks and a `/repeat -w`; `/retry_off` removes them by name (`~retry_fail_<world>`, `~retry_succ_<world>`).

**Status:** see [Status](Status.md) (host capability: WorldHost)

## /watchdog [OFF|ON] and /watchdog n1 n2

**What it is for:** suppressing spam: a line repeated `n1` times in the last `n2` lines is gagged.

**Syntax:** `/watchdog on|off`, `/watchdog n1 n2` (both numbers are required in real tf and here, despite the manual's stated defaults). Sets the flag variable `%watchdog` (off by default; it reads back as `on` or `off`).

```tf
/watchdog 3 6
/echo flag is $[watchdog]
```
```output
% watchdog enabled, searching for 3 out of 6 lines
flag is on
```

**Gotchas:** Its counts are independent of `/watchname`'s. Plain `/watchdog` reports `% watchdog disabled.` when off.

**Status:** see [Status](Status.md) (the counters are kept by the engine; the host feeds each received line through `CheckWatch`)

## /watchname [OFF|ON] and /watchname n1 n2

**What it is for:** muting a player who floods you: a name that starts `n1` of the last `n2` lines is gagged (with a message).

**Syntax:** `/watchname on|off`, `/watchname n1 n2` (both required, as for `/watchdog`). Sets `%watchname`.

```tf
/watchname on
/echo flag is $[watchname]
```
```output
% watchname enabled, searching for 4 out of 5 lines
flag is on
```

**Status:** see [Status](Status.md) (the counters are kept by the engine; the host feeds each received line through `CheckWatch`)

## Related

- Functions: [Functions-World-and-Connection](Functions-World-and-Connection.md).
- Protocol data once connected: [Functions-Protocol](Functions-Protocol.md).
