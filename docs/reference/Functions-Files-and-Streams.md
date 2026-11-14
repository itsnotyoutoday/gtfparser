# Functions: Files and Streams

tf does its I/O through *streams* (the `tfio` system). Three streams always exist and have one-letter handles: `"i"` (tfin, the keyboard), `"o"` (tfout, the screen) and `"e"` (tferr, the screen, never redirected). Streams you open yourself with `tfopen()` get small positive integer handles. These functions open, read, write and close them, and are how a script keeps notes, logs or settings on disk.

All the functions return `-1` (or `0` for the writers, see each entry) on failure instead of raising an error, so check the result when it matters. Remember when testing from the command line that a result of `-1` at the start of `/echo` text looks like an option; write `/echo r=$[...]`.

## tfopen([name, mode])

**What it is for:** getting a handle on a file or shell pipe so you can read or write it line by line.

**Syntax:** `tfopen(name, mode)` returns an integer handle; `tfopen()` with no arguments opens an unnamed queue.

**Arguments/Returns:** `name` is a file name (after [`filename()`](#filenamename) expansion, so `~/` works) or, for mode `"p"`, a shell command (for `"q"` it is only a label shown by `/liststreams`). `mode` is one of `"r"` (read), `"w"` (create or truncate for writing), `"a"` (append), `"p"` (read the output of a shell command), or `"q"` (an in-memory queue you can write to and read back). Returns a positive handle, or `-1` if the file cannot be opened, the mode is invalid, or the operation is restricted.

```tf
/test h := tfopen("/tmp/gtf-ex-notes.txt", "w")
/echo handle ok: $[h > 0]
/test tfwrite(h, "remember: buy lamp oil")
/test tfclose(h)
/echo missing file: $[tfopen("/tmp/gtf-ex-no-such-dir/x", "r")]
/sh rm -f /tmp/gtf-ex-notes.txt
```
```output
handle ok: 1
missing file: -1
```

**Gotchas:** Handles count up from 1 and are never reused within a session. Opening for `"w"` truncates immediately. The argument order is name, then mode, in both real tf and this engine. Mode `"q"` opens a queue: a FIFO of lines you `tfwrite()` and `tfread()` back (a bare `tfopen()` opens an unnamed queue), and `"p"` runs a shell command and reads its output as lines; both work as in real tf (see the example below). `/restrict` at the file level or higher forbids `tfopen()` entirely (returns `-1`).

A queue and a pipe (the queue's lines come back in order, and a read past the end returns `-1`):

```tf
/test q := tfopen("jobs", "q")
/echo wrote $[tfwrite(q, "kill orc")] then $[tfwrite(q, "loot orc")]
/echo $[tfread(q, job)] [%{job}]
/echo $[tfread(q, job)] [%{job}]
/echo empty: $[tfread(q, job)]
/test p := tfopen("echo hi; echo there", "p")
/echo $[tfread(p, l)] [%l] $[tfread(p, l)] [%l] $[tfread(p, l)]
/echo closed: $[tfclose(q)] $[tfclose(p)]
```
```output
wrote 1 then 1
8 [kill orc]
8 [loot orc]
empty: -1
2 [hi] 5 [there] -1
closed: 0 0
```

Bad handles warn and return `-1`; note that `/echo -1` is read as an option, so give the number a prefix:

```tf
/test h := tfopen("/tmp/gtfdoc-h.txt", "w")
/echo r=$[tfclose(h)] r=$[tfclose(h)] r=$[tfflush(12345)] r=$[tfwrite(12345, "x")]
/sh rm -f /tmp/gtfdoc-h.txt
```
```output
r=0 r=-1 r=-1 r=-1
```

**Status:** see [Status](Status.md)

## tfclose(handle)

**What it is for:** finishing with a stream: flushes pending output and frees the handle.

**Syntax:** `tfclose(handle)` returns an integer.

**Arguments/Returns:** `handle` is a number from `tfopen()` or one of `"i"`, `"o"`. Real tf returns `0` on success and `-1` for a handle that is not open; it also lets a macro close `"o"` to silence the rest of that macro's output, closes `"i"` to stop keyboard reads, and refuses to close `"e"` (tferr cannot be closed; returns `-1`).

```tf
/test h := tfopen("/tmp/gtf-ex-close.txt", "w")
/test tfwrite(h, "saved before closing")
/test tfclose(h)
/test r := tfopen("/tmp/gtf-ex-close.txt", "r")
/test tfread(r, back)
/echo read back: %back
/test tfclose(r)
/sh rm -f /tmp/gtf-ex-close.txt
```
```output
read back: saved before closing
```

**Gotchas:** Using a handle after closing it does not stop the script; the call just fails (real tf: `-1` plus a `bad handle` message). Always close streams you opened for writing if another program is going to read the file. Handle numbers are not reused.

**Status:** see [Status](Status.md)

## tfwrite([handle,] text)

**What it is for:** writing one line to a stream; with no handle it is the same as `echo(text)`.

**Syntax:** `tfwrite(handle, text)` or `tfwrite(text)`.

**Arguments/Returns:** `text` is written followed by a newline. Display attributes in `text` are stripped when it goes to a file or pipe. Returns `1` on success. If the handle is unusable (closed, unknown, or opened read-only) it prints a warning, returns `-1` and writes nothing. Writing to `"e"` (tferr) shows the text as an error line and returns `1`.

```tf
/test log := tfopen("/tmp/gtf-ex-log.txt", "a")
/test tfwrite(log, "12:01 Bob arrives from the north")
/test tfwrite(log, "12:02 Bob says, \"hello\"")
/test tfclose(log)
/test tfwrite("announcement to the screen")
/test tfwrite("o", "same, with an explicit handle")
/sh rm -f /tmp/gtf-ex-log.txt
```
```output
announcement to the screen
same, with an explicit handle
```

**Gotchas:** Files opened with `"w"` or `"a"` are flushed after every line by default; turn that off with [`tfflush()`](#tfflushhandle-flag) when you write thousands of lines. Writing to `"e"` produces a tferr message.

**Status:** see [Status](Status.md)

## tfread([handle,] variable)

**What it is for:** reading the next line of a stream into a variable, the building block of every "parse this file" script.

**Syntax:** `tfread(handle, varname)` or `tfread(varname)`. The second argument is the *name* of the variable to set, not its value: pass it as a bare variable name in an expression (`tfread(h, line)` assigns `%line`).

**Arguments/Returns:** returns the length of the line read (so an empty line returns 0, which is still success), or `-1` at end of file / error. If the variable does not exist it is created globally, as if by `/set`. After `-1` at the end of a file the only valid operation is `tfclose()`.

```tf
/test w := tfopen("/tmp/gtf-ex-rooms.txt", "w")
/test tfwrite(w, "The Great Hall")
/test tfwrite(w, "")
/test tfwrite(w, "Armory")
/test tfclose(w)
/test r := tfopen("/tmp/gtf-ex-rooms.txt", "r")
/while (tfread(r, room) >= 0) /echo room: [%{room}]%; /done
/test tfclose(r)
/sh rm -f /tmp/gtf-ex-rooms.txt
```
```output
room: [The Great Hall]
room: []
room: [Armory]
```

**Gotchas:** Do not use `%l1` style names for the variable; `%l1` is read as the special substitution `%L1`. Reading from the keyboard (`tfread(var)` with no handle) is only allowed from a command-line command, never from a trigger or hook (it returns `-1`). A read-only stream opened with `"w"` returns `-1`. Also see the obsolete [`read()`](Functions-Terminal-and-Input.md#read).

**Status:** see [Status](Status.md)

## tfreadable(handle)

**What it is for:** asking whether `tfread()` would answer *immediately*: it matters for pipes (`"p"`) and the keyboard, where a read can otherwise wait.

**Syntax:** `tfreadable(handle)` returns 1 or 0.

**Arguments/Returns:** `1` when a `tfread()` would return at once (a line, or end of file, or an error), `0` when it would have to wait: the keyboard stream `"i"` while no read is pending, or a shell pipe that has produced nothing yet. For ordinary files and queues it is always `1`.

```tf
/test w := tfopen("/tmp/gtf-ex-rd.txt", "w")
/test tfwrite(w, "only line")
/test tfclose(w)
/test r := tfopen("/tmp/gtf-ex-rd.txt", "r")
/echo file readable? $[tfreadable(r)]
/test tfread(r, got)
/echo after reading: $[tfreadable(r)]
/test tfclose(r)
/sh rm -f /tmp/gtf-ex-rd.txt
```
```output
file readable? 1
after reading: 1
```

**Gotchas:** It does not mean "has unread lines": after the last line it still says `1` (the next `tfread()` returns `-1`). Loop on the return value of `tfread()`, and use `tfreadable()` only to avoid blocking on a pipe or the keyboard. It says `0` for `"i"` when no key is waiting; for write-only, closed or unknown handles it warns and returns `1`, exactly as real tf does.

**Status:** see [Status](Status.md)

## tfflush(handle [, flag])

**What it is for:** forcing buffered output to disk, or turning per-line automatic flushing on or off for a stream.

**Syntax:** `tfflush(handle)` flushes now; `tfflush(handle, flag)` sets autoflush (`0`/`"off"` or `1`/`"on"`).

**Arguments/Returns:** returns nonzero on success. It has no effect on `"r"`, `"p"` and `"q"` streams.

```tf
/test h := tfopen("/tmp/gtf-ex-flush.txt", "w")
/test tfflush(h, "off")
/test tfwrite(h, "line 1")
/test tfwrite(h, "line 2")
/echo flush: $[tfflush(h)]
/test tfclose(h)
/sh rm -f /tmp/gtf-ex-flush.txt
```
```output
flush: 1
```

**Gotchas:** `tfclose()` flushes too, so explicit flushing only matters if something else reads the file while you still have it open (a `tail -f`, a second `tfopen(..., "r")`).

**Status:** see [Status](Status.md)

## fwrite(file, text)

**What it is for:** the one-shot way to append a single line to a file without managing a handle.

**Syntax:** `fwrite(filename, text)` returns an integer.

**Arguments/Returns:** opens `filename` (with `~` expansion) for appending, writes `text` and a newline, and closes it. Display attributes are not written. Real tf returns nonzero on success and `0` if the file cannot be opened.

```tf
/echo $[fwrite("/tmp/gtfdoc-kills.txt", "orc, 12:03")] $[fwrite("/tmp/gtfdoc-kills.txt", "troll, 12:09")]
/test h := tfopen("/tmp/gtfdoc-kills.txt", "r")
/echo $[tfread(h, a)] [%a] $[tfread(h, b)] [%b] $[tfread(h, c)]
/test tfclose(h)
/sh rm -f /tmp/gtfdoc-kills.txt
```
```output
1 1
10 [orc, 12:03] 12 [troll, 12:09] -1
```

**Gotchas:** One `fwrite()` per line costs an open and close each time; for many lines use `tfopen`/`tfwrite`/`tfclose`. Appends, never truncates.

**Status:** see [Status](Status.md)

## filename(path)

**What it is for:** applying tf's own file-name expansion to a string, for building paths in scripts.

**Syntax:** `filename(path)` returns a string.

**Arguments/Returns:** a leading `~` is replaced with your home directory (the `HOME` of the environment), and `~user` with that user's home directory; everything else is returned unchanged. Every function and command that takes a file name already does this expansion; call `filename()` when you need the expanded text itself.

```tf
/echo expanded: $[filename("~/logs/cave.log") !~ "~/logs/cave.log"]
/echo untouched: $[filename("/var/tmp/x")]
```
```output
expanded: 1
untouched: /var/tmp/x
```

**Gotchas:** `$HOME` and other shell-style variables are not expanded. A `~user` that does not exist is left as written.

**Status:** see [Status](Status.md)

## read()

**What it is for:** obsolete spelling of reading a line from the keyboard; use `tfread()`.

**Syntax:** `read()` returns a string.

**Arguments/Returns:** reads a line from the user's keyboard buffer. Real tf prints a deprecation warning and, like `tfread()`, only works from a command-line command. See [Functions-Terminal-and-Input](Functions-Terminal-and-Input.md#read).

**Status:** see [Status](Status.md) (host capability: Notifier)

## nread()

**What it is for:** telling whether a keyboard read (`tfread()` from the keyboard) is currently waiting for the user.

**Syntax:** `nread()` returns a positive integer while a read is in progress, `0` otherwise. A status-line indicator `@read` is driven by this.

```tf
/echo reading? $[nread()]
```
```output
reading? 0
```

**Status:** see [Status](Status.md) (host capability: WorldHost)

## nlog()

**What it is for:** counting open log files (see [/log](Commands-Files.md#log)); it feeds the `(Log)` status-bar indicator.

**Syntax:** `nlog()` returns an integer.

```tf
/echo logs open: $[nlog()]
/log /tmp/gtf-ex-session.log
/echo logs open: $[nlog()]
/log off
/echo logs open: $[nlog()]
/sh rm -f /tmp/gtf-ex-session.log
```
```output
logs open: 0
% Logging to file /tmp/gtf-ex-session.log
logs open: 1
logs open: 0
```

**Gotchas:** Several `/log` commands that write to the same file still count as separate logs.

**Status:** see [Status](Status.md) (no host capability needed: the core counts its own logs)

## Related

- [Commands-Files](Commands-Files.md): `/load`, `/save`, `/log`, `/liststreams`.
- Redirection: `%|` pipes, `$(...)` and `/quote` use `"q"` queues internally; see the tf topic `redirection`.
