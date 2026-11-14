# Functions: Worlds and Connections

A *world* is a named definition of a place tf can connect to (host, port, login, type); a *socket* is an open connection to one. These functions inspect and define worlds and sockets so a script can adapt to where it is running: "am I connected?", "which world is in front?", "how long since the server said something?".

In tf, a function with an optional world argument uses the *current* world (the one the running trigger or command belongs to) when it is omitted, which is often, but not always, the foreground world. The world database (`addworld()`, `world_info()`) lives in the engine and works anywhere. What is *open* or *connected* is owned by the embedding program, which reports its sockets through the optional `WorldHost` / `SocketHost` interfaces (see `host.go` and `docs/engine/state-builtins.md`). A host that provides none of them is treated as having no sockets: `is_open()`, `is_connected()`, `nactive()` answer 0 and the idle timers 0.0, so the examples below, which run in a bare interpreter, show the no-socket answers.

## addworld(name, type, host, port, char, pass, file, flags, srchost)

**What it is for:** defining (or redefining) a world from a script, usually when generating a world list from data.

**Syntax:** `addworld(name, type [, host, port [, char, pass [, file [, flags [, srchost]]]]])` returns 1 on success, 0 on failure.

**Arguments/Returns:** `name` may not contain spaces and, for a new world, may not start with `(`. `type` is a free-form label matched by hooks and triggers `-T` (see [/addworld](Commands-Worlds-and-Connections.md#addworld)). `host`/`port` may both be empty for a "connectionless" world. `file` is an optional macro file loaded on connect. `flags` is a string of letters: `p` (ignore `%proxy_host`), `x` (use SSL), `e` (echo sent text back as if received). Redefining a world replaces all its fields.

```tf
/echo $[addworld("Cave", "tiny", "cave.example.org", 4201, "bob", "swordfish")]
/echo $[world_info("Cave", "host")]:$[world_info("Cave", "port")]
/echo $[world_info("Cave", "character")] on a $[world_info("Cave", "type")] world
```
```output
1
cave.example.org:4201
bob on a tiny world
```

**Gotchas:** Redefining an existing world prints `% addworld: Redefined world Cave`. A password in a script file readable by others also triggers a warning. A world named `default` provides the character, password, type and file for worlds that omit them. The command form `/addworld` is a library macro that calls this function.

**Status:** see [Status](Status.md) (no host capability needed)

## world_info([world,] field)

**What it is for:** reading one property of a world definition.

**Syntax:** `world_info(world, field)`, `world_info(field)` (current world), or `world_info()` (name of the current world).

**Arguments/Returns:** `field` is one of `name`, `type`, `character`, `password`, `host`, `port`, `mfile`, `login` (`1` if auto-login is on for the open socket), `proxy` (`1` if connected through a proxy), `src` (local address), `cipher` (SSL cipher name or empty). Returns a string. An unknown world gives an empty string without an error; an unknown field prints `world_info: illegal field name` and gives an empty string. With no current world (nothing connected), `world_info()` is empty.

```tf
/addworld -Ttiny Cave cave.example.org 4201
/echo $[world_info("Cave", "type")] $[world_info("Cave", "port")]
/echo [$[world_info("NoSuchWorld", "host")]]
/echo [$[world_info()]]
```
```output
tiny 4201
[]
[]
```

**Gotchas:** The field names are exactly those above; `flags` and `srchost` (the names used by `addworld()`) are not accepted by `world_info()`. With no world argument, `host`/`port` describe the *socket*, so an unconnected current world gives errors or empty text.

**Status:** see [Status](Status.md) (no host capability for stored fields; `host`, `port`, `login`, `cipher` of an open socket come from `SocketHost`)

## fg_world()

**What it is for:** finding out which world is in the foreground (the one whose text you are looking at and whose input line is active).

**Syntax:** `fg_world()` returns the world's name, or an empty string when no socket is in the foreground.

```tf
/echo foreground: [$[fg_world()]]
```
```output
foreground: []
```

With a host that tracks sockets, `/fg Cave` makes it `Cave` (see [/fg](Commands-Worlds-and-Connections.md#fg--bg)) and `/bg` makes it empty again.

**Gotchas:** Do not confuse this with `world_info()` (no arguments), which names the *current* world: inside a trigger firing for a background world, `world_info()` is that world and `fg_world()` is whatever you happen to be viewing.

**Status:** see [Status](Status.md) (host capability: SocketHost/WorldHost for the socket state)

## is_connected([world])

**What it is for:** guarding a send or a login macro so it only runs when a live connection exists.

**Syntax:** `is_connected()` (current socket) or `is_connected(world)`; returns `1` or `0`.

**Arguments/Returns:** `1` only for a fully established connection. A connection still being attempted, a closed one, and a connectionless socket give `0` (see `is_open()` for the broader test).

```tf
/addworld Cave cave.example.org 4201
/def keepalive = /if (is_connected("Cave")) /send ping%; /else /echo not connected%; /endif
/keepalive
```
```output
not connected
```

**Gotchas:** A connectionless world opened with `/connect` is "open" but never "connected".

**Status:** see [Status](Status.md) (host capability: SocketHost/WorldHost for the socket state)

## is_open([world])

**What it is for:** the broader sibling of `is_connected()`: true when a socket exists for the world, whether or not it is a real network connection.

**Syntax:** `is_open()` or `is_open(world)`; returns `1` or `0`.

**Arguments/Returns:** `1` for any open socket, including connectionless ones and ones still connecting. Real tf's `/world` macro uses `is_open()` to choose between `/fg` and `/connect`.

```tf
/addworld Scratch
/echo defined but not opened: $[is_open("Scratch")] $[is_connected("Scratch")]
/echo no current socket: $[is_open()] $[is_connected()]
```
```output
defined but not opened: 0 0
no current socket: 0 0
```

After `/connect Scratch` on a host that supports it, `is_open("Scratch")` is `1` while `is_connected("Scratch")` stays `0` for a connectionless world.

**Status:** see [Status](Status.md) (host capability: SocketHost/WorldHost for the socket state)

## nactive([world])

**What it is for:** counting how much unseen output is waiting in background worlds, for status displays and "something happened" alerts.

**Syntax:** `nactive()` returns the number of worlds that have unseen text; `nactive(world)` returns the number of unseen lines in that world.

**Arguments/Returns:** `nactive(world)` is the same as `moresize("n", world)`. Called from a trigger, the line that fired the trigger is not yet counted.

```tf
/echo worlds waiting: $[nactive()]; Cave has $[nactive("Cave")] new lines
```
```output
worlds waiting: 0; Cave has 0 new lines
```

**Gotchas:** A world with no socket yields `0`, not an error.

**Status:** see [Status](Status.md) (host capability: SocketHost/WorldHost for the socket state)

## idle([world])

**What it is for:** measuring inactivity: "how long since I last touched the keyboard?" or "how long since this server spoke?"

**Syntax:** `idle()` gives seconds since the last keypress; `idle(world)` gives seconds since text was last *received* from that world, or `-1` on error. For a world with no socket the answer is `0.0` here and in the reference build. The result has microsecond precision (e.g. `12.345678`).

```tf
/addworld Cave cave.example.org 4201
/echo quiet for $[idle("Cave")] seconds
```
```output
quiet for 0.0 seconds
```

A typical use is an away macro: `/repeat -300 i /if (idle() > 1800) /send afk%; /endif`.

**Gotchas:** `idle()` with no argument tracks the *keyboard*; to ask about the server you must pass the world name.

**Status:** see [Status](Status.md) (host capability: SocketHost/WorldHost for the socket state)

## sidle([world])

**What it is for:** the counterpart of `idle(world)` for output: seconds since you last *sent* a line, for anti-idle keepalives.

**Syntax:** `sidle()` (current socket) or `sidle(world)`; returns seconds as a real, or `-1` on error.

```tf
/addworld Cave cave.example.org 4201
/echo last sent $[sidle("Cave")] seconds ago
```
```output
last sent 0.0 seconds ago
```

An anti-idle macro: `/repeat -w -120 i /if (sidle() > 240) /send look%; /endif`.

**Status:** see [Status](Status.md) (host capability: SocketHost/WorldHost for the socket state)

## fake_recv(text [, world [, flags]])

**What it is for:** pretending the server sent a line, so triggers, hooks and display run exactly as if it came from the network. Invaluable for testing triggers without a live game.

**Syntax:** `fake_recv(text)`, `fake_recv(text, world)`, `fake_recv(text, world, "r")`; returns `1` on success, `0` on failure.

**Arguments/Returns:** `text` is processed as one received line (flag `r` feeds it as raw bytes, so telnet bytes and partial lines are interpreted too). The world must be connected or open; otherwise `fake_recv: no open world` and `0`. Use [`/trigger`](Commands-Macros-and-Variables.md) when you only want trigger matching without touching a socket.

```tf
/addworld Cave cave.example.org 4201
/echo no open world, so: $[fake_recv("Alice tells you: hi there", "Cave")]
```
```output
no open world, so: 0
```

With the world open (host-dependent), the line runs through triggers as if received: after `/def -t'* tells you: *' reply = /send say %-2`, `/test fake_recv("Alice tells you: hi there", "Cave")` sends `say hi there`. To exercise triggers without any socket, use [`/trigger`](Commands-Macros-and-Variables.md).

**Gotchas:** The `r` flag is the only one defined; anything else prints `invalid flag` and returns `0`.

**Status:** see [Status](Status.md) (host capability: SocketHost/WorldHost for the socket state)

## prompt(text)

**What it is for:** setting the prompt of the current socket from a script, as if the server had sent an unterminated prompt line.

**Syntax:** `prompt(text)` returns nonzero if there is a current socket. The library also defines a `/prompt` macro that wraps it.

```tf
/echo $[prompt("HP: 120> ")]
```
```output
0
```
(with no current world there is no socket whose prompt could be set)

**Gotchas:** Prompt handling interacts with `%lp` and the `PROMPT` hook; see the `prompts` topic of the tf manual.

**Status:** see [Status](Status.md) (host capability: PromptHost, optional; needs a current world)

## Related

- [Commands-Worlds-and-Connections](Commands-Worlds-and-Connections.md): `/connect`, `/dc`, `/fg`, `/listworlds`, `/listsockets`.
- Telnet-level data from a connected world: [Functions-Protocol](Functions-Protocol.md) (`gmcp()`, `msdp()`, `atcp()`, `option102()`).
