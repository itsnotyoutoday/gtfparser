# Commands: Files

Commands that read scripts into tf, save your definitions back out, log a session, move around the file system or run the shell. File names in all of these go through the same expansion as [`filename()`](Functions-Files-and-Streams.md#filenamepath): a leading `~` means a home directory. A relative name is looked for first in the current directory (see `/lcd`), then in each directory of `%TFPATH` (or `%TFLIBDIR` when `%TFPATH` is empty).

Entries marked *Library* are macros defined by tf's standard library `stdlib.tf`, not C builtins; they behave like commands but you can read and override them.

## /load [-q] file

**What it is for:** running a script file, the normal way to put your macros, triggers and settings into tf (typically from `.tfrc`).

**Syntax:** `/load [-q] file`

**Arguments/Returns:** reads `file` line by line and executes each line as a command. Blank lines and lines beginning with `;` or `#` are skipped; leading whitespace is ignored; a line ending in `\` is joined to the next; `%\` makes the backslash literal. `-q` suppresses the `% Loading commands from file` message (including for nested loads). Returns 1 on success, 0 if the file could not be read. A `/exit` command in the file ends the load early.

```tf
/test f := tfopen("/tmp/gtf-ex-hello.tf", "w")
/test tfwrite(f, "; greetings for the lobby")
/test tfwrite(f, "/def hello = /echo Hello from the file, %%1!")
/test tfwrite(f, "/echo (hello.tf loaded)")
/test tfclose(f)
/load -q /tmp/gtf-ex-hello.tf
/hello Bob
/sh rm -f /tmp/gtf-ex-hello.tf
```
```output
(hello.tf loaded)
Hello from the file, Bob!
```
The example writes `%%1` because the typed line is substituted once before `tfwrite()` sees it; the file itself receives `%1`.

**Gotchas:** Lines in a file are *not* substituted when read: `/echo $[1+1]` in a file is stored and executed as written, and only expands when the line is run (as part of a macro, or by `/eval`). A missing file prints `% /load: <name>: no such file` and returns 0 (real tf phrases it `LOAD: <name>: No such file or directory`). Real tf also lets a builtin command be called as a function (`$[load("file")]` gives 1 or 0), which this engine does not support yet; see FINDINGS. `/load` with the same file twice runs it twice; use `/require` for load-once files.

**Status:** see [Status](Status.md)

## /require [-q] file  (and /loaded token)

**What it is for:** loading a library file only once, however many scripts ask for it.

**Syntax:** `/require [-q] file`; and inside the file, as its first command, `/loaded token`.

**Arguments/Returns:** `/require` is `/load` that skips files which have already called `/loaded` with the same token. `token` should be unique and contain no spaces or glob characters; the file's own name is a good choice. When a `/require`d file has been loaded before, its `/loaded` line ends the load (it does not return to the file); `/require` returns 1 on success, including when it did not need to load.

```tf
/test f := tfopen("/tmp/gtf-ex-once.tf", "w")
/test tfwrite(f, "/loaded gtf-ex-once")
/test tfwrite(f, "/echo setting up once")
/test tfwrite(f, "/test uses := uses + 1")
/test tfclose(f)
/set uses=0
/require -q /tmp/gtf-ex-once.tf
/require -q /tmp/gtf-ex-once.tf
/load -q /tmp/gtf-ex-once.tf
/echo loaded body ran %uses time(s)
/sh rm -f /tmp/gtf-ex-once.tf
```
```output
setting up once
setting up once
loaded body ran 2 time(s)
```

**Gotchas:** Only `/require` honours the token: the second `/require` above did nothing, but the plain `/load` ran the whole file again (hence two "setting up once" lines and a count of 2). Both are Library entries in tf: `/loaded` and `/require` are defined by `stdlib.tf`.

**Status:** see [Status](Status.md)

## /save [-a] file [list-options]

**What it is for:** writing macros out to a file in `/def` form so a later `/load` brings them back.

**Syntax:** `/save [-a] file [list-options]`. The `list-options` are those of `/list` (`-mglob`, `-t`, `-h`, `-b`, plus a name pattern), so `/save` can write all macros or any subset. `-a` appends instead of replacing the file.

**Arguments/Returns:** prints `% Writing macros to file` (`Appending ...` with `-a`). Invisible macros (those defined with `-i`, such as the library) are skipped unless `-i` is given. The return value is the number of the last macro listed, or 0 if none matched.

```tf-noverify
/def -mglob -t'You are hungry' hungry = /send eat bread
/def greet = /echo Hello, %1!
/save /tmp/mymacros.tf -mglob greet
/save -a /tmp/mymacros.tf -mglob hungry
```
```output
% Writing macros to /tmp/mymacros.tf
% Appending macros to /tmp/mymacros.tf
```
The file then contains `/def greet = /echo Hello, %1!` followed by `/def -p1 -mglob -t'You are hungry' hungry = /send eat bread`.

**Gotchas:** tf never loads saved files by itself; add the matching `/load` to your `.tfrc`. Saved text is machine-written; if you have many or complex macros, keep them in hand-written files and `/load` those instead. For plain files of text use the library's `/writefile` (`textutil.tf`).

**Status:** see [Status](Status.md)

## /saveworld [-a] [file]

**What it is for:** saving all world definitions (see `/addworld`) so they survive restarts.

**Syntax:** `/saveworld [-a] [file]`. Without `file`, the name given in the `WORLDFILE` macro (`~/tiny.world` by default) is used. `-a` appends.

**Arguments/Returns:** writes one `/test addworld(...)` line per world, including passwords, so protect the file. Reload with `/loadworld`.

```tf-noverify
/addworld -Ttiny Cave cave.example.org 4201
/saveworld /tmp/worlds.tf
```

**Gotchas:** Passwords are stored in plain text; tf warns when a script defining a world with a password is readable by others. The file is not loaded automatically.

**Status:** see [Status](Status.md) (host capability: WorldHost)

## /log [-ligw[world]] [OFF|ON|file]

**What it is for:** recording what you see, or what you type, to a text file.

**Syntax:** `/log [-w[world]|-l|-i|-g] [OFF|ON|file]`

**Arguments/Returns:** the option chooses *what* is logged: `-wworld` one world's output (`-w` alone: the current world), `-l` local output generated by tf, `-i` your keyboard input, `-g` everything (the default when none is given). The argument says what to do: a file name starts logging to it (appending), `ON` uses `${LOGFILE}` (default `tiny.log`), `OFF` stops that log (all logs if no option), and no argument lists the open logs. Several logs may be open at once, and several kinds can share one file. Lines with the `L` (nolog) attribute are not written; wrapping happens in the file only when `%wraplog` is on.

```tf-noverify
/log -wCave ~/logs/cave.log
/log /tmp/everything.log
/log
/log OFF
```
```output
% Logging to file /home/bob/logs/cave.log
% Logging to file /tmp/everything.log
% Logging global output to /tmp/everything.log
% Logging disabled.
```

**Gotchas:** `-w` with no world selected fails with `LOG -w: No world`. Logging is off by default. The library macro `/nolog` is `/log OFF`. A dated per-world log: `/eval /log -w ${world_name}.$[ftime("%F")]`. See [`nlog()`](Functions-Files-and-Streams.md#nlog) for the count of open logs.

**Status:** see [Status](Status.md) (host capability: Notifier)

## /lcd [dir]  (and /cd, /pwd)

**What it is for:** changing, or showing, the directory that relative file names are resolved against.

**Syntax:** `/lcd [dir]`, `/cd [dir]`, `/pwd`.

**Arguments/Returns:** `/lcd dir` changes the working directory; `/lcd` alone prints `% Current directory is ...`. `/cd` is the Library alias for `/lcd`, except that `/cd` with no argument goes to your home directory. `/pwd` is a Library macro that prints the current directory. A directory that does not exist prints an error and changes nothing.

```tf
/lcd /tmp
/cd /
/echo at the root: $[strlen($(/pwd)) == 1]
```
```output
at the root: 1
```

**Gotchas:** On macOS `/tmp` is a symlink, so `/pwd` after `/lcd /tmp` reports `/private/tmp`. The directory is for the whole tf process, not per world.

**Status:** see [Status](Status.md)

## /sh [-q] [command]  (and /sys)

**What it is for:** running a shell command from inside tf.

**Syntax:** `/sh [-q] [command]`; Library `/sys command`.

**Arguments/Returns:** `/sh command` runs the command in `/bin/sh` *interactively*: it may read the terminal and print anything; output goes straight to the screen, not through tf's display. `-q` suppresses the "Executing" notice and the `SHELL` hook (see FINDINGS: this engine's `/sh` does not yet strip `-q`). With no command it starts an interactive shell. Returns the shell's exit status, or -1 if it did not exit normally. `/sys command` is the library's "inline" variant: it runs the command and displays each line of its output through tf (so output can be captured with `%|`/`$()` and logged), but the command cannot ask for input.

```tf
/sh echo from the shell
/sys echo from sys
```
```output
from the shell
!from sys
```

**Gotchas:** Real tf prefixes each `/sys` line with `!` (the library implements it as `/quote -S -decho \\!!command`), and this engine matches that. Never build a `/sh` command from text sent by a game: a trigger doing so lets anyone on the server run commands as you. A client may disable it with `/restrict shell` or the embedder's policy.

**Status:** see [Status](Status.md)

## /quote [options] [prefix] 'file | !command | `tfcommand | #recall  [suffix]

**What it is for:** turning the lines of a file, a shell command, a tf command or the history into a stream of echoed, sent or executed lines: pasting a text file to a game, walking a word list, running a command generated by a command.

**Syntax:** `/quote [-S|-time|-P] [-ddisp] [-wworld] [-ssub] [prefix] 'file[suffix]`; the same with `!command` (shell output), `` `command `` (output of a tf command) or `#args` (a `/recall`). The quote characters may be wrapped in double quotes when a suffix follows.

**Arguments/Returns:** `-ddisp`: `echo` (show it), `send` (send to the socket; the default when there is no prefix) or `exec` (run as a tf command; the default when there is a prefix). `-w[world]` runs lines with that world current. `-time` delays lines (`-0` as fast as possible, `-S` synchronous with no delay, `-P` once per prompt); the default delay is `%ptime`. Each generated line is `prefix` + line + `suffix`. An asynchronous quote returns its process id (0 on error); a synchronous one returns the command's result.

```tf
/test f := tfopen("/tmp/gtf-ex-words.txt", "w")
/test tfwrite(f, "north")
/test tfwrite(f, "east")
/test tfclose(f)
/quote -S -decho '/tmp/gtf-ex-words.txt
/quote -S -decho walk: '/tmp/gtf-ex-words.txt
/quote -S -decho go `/echo up%;/echo down
/quote -S -decho from shell: !echo ok
/sh rm -f /tmp/gtf-ex-words.txt
```
```output
north
east
walk: north
walk: east
go up
down
from shell: ok
```

**Gotchas:** A prefix that contains one of the command characters `'!`#` must escape it with `\`. A synchronous quote of a slow shell command freezes tf until output arrives. The library file `quoter.tf` adds shortcut commands for common quotes (see [Commands-Processes](Commands-Processes.md)). Never trigger `/quote !` from game text.

**Status:** see [Status](Status.md)

## /liststreams

**What it is for:** seeing which streams your scripts left open (a debugging aid for leaked `tfopen()` handles).

**Syntax:** `/liststreams`; returns the number of streams listed.

**Arguments/Returns:** prints a table with columns HANDLE, MODE, FLUSH (autoflush on/off) and NAME for each stream made by `tfopen()` (the standard `i`, `o`, `e` streams are not listed).

```tf-noverify
/test h := tfopen("/tmp/notes.txt", "w")
/liststreams
```
```output
HANDLE MODE FLUSH NAME
     1   w    on  /tmp/notes.txt
```

**Status:** see [Status](Status.md)

## /loaddef, /loadbind, /loadhilite, /loadgag, /loadtrig, /loadhook, /loadworld  *(Library)*

**What they are for:** `/load` with a sensible default file for each kind of definition.

**Syntax:** `/loaddef [file]` etc. With no file, the name held by the matching macro is used: `MACROFILE`, `BINDFILE`, `HILITEFILE`, `GAGFILE`, `TRIGFILE`, `HOOKFILE`, `WORLDFILE`. By default these are `~/tiny.macros`, `~/tiny.bind` and so on on Unix. Extra arguments (such as `-q`) are passed to `/load`.

```tf
/test f := tfopen("/tmp/gtf-ex-mac.tf", "w")
/test tfwrite(f, "/def greet = /echo Hello, %%1!")
/test tfclose(f)
/loaddef /tmp/gtf-ex-mac.tf
/greet Alice
/sh rm -f /tmp/gtf-ex-mac.tf
```
```output
Hello, Alice!
```

**Gotchas:** The default file names are macros (`/eval /def -i MACROFILE = ...`), not variables, so override them by redefining the macro, not with `/set`. A missing default file prints the usual `/load` error.

**Status:** see [Status](Status.md)

## /savedef, /savebind, /savehilite, /savegag, /savetrig, /savehook  *(Library)*

**What they are for:** `/save` with the right filter for one kind of macro.

**Syntax:** `/savedef [file]`: plain named macros with no trigger, hook or binding; `/savebind`: macros with key bindings; `/savehilite`: triggers with attributes other than `-ag`; `/savegag`: gag triggers (`-ag`); `/savetrig`: triggers with no attributes; `/savehook`: hooks. With no file, the matching default file name (as for `/load*`) is used. They overwrite; there is no append form.

```tf-noverify
/def greet = /echo Hello, %1!
/savedef /tmp/macros.tf
```
```output
% Writing macros to /tmp/macros.tf
```

**Gotchas:** Like `/save`, they skip invisible (`-i`) macros, so library macros are not saved.

**Status:** see [Status](Status.md)

## /loadhist [-lig] [-w world] [file]  *(Library)*

**What it is for:** reading an old log file back into a history (the scrollback and recall buffer) at startup.

**Syntax:** `/loadhist [-l|-i|-g] [-wworld] [file]`; without `file` the file named by `LOGFILE` is used. It runs `/quote -S /recordline` over the file; see `/recordline` in [Commands-History-and-Display](Commands-History-and-Display.md).

**Status:** see [Status](Status.md)

## /getfile_MUCK, /getfile_LP, /getfile_UNIX, /putfile_MUCK, /putfile_LP, /putfile_UNIX  *(Library, filexfer.tf)*

**What they are for:** transferring a text file to or from a MUCK, an LPmud or a Unix shell by driving the remote editor.

**Syntax:** after `/require filexfer.tf`: `/getfile_MUCK file [remote-file]` downloads; `/putfile_MUCK file [remote-file]` uploads (`/putfile` is the old name for `/putfile_MUCK`). The `_LP` forms assume a UNIX-like `ed`. If the remote name is omitted the local one is used. Do not use glob characters in the names.

**Gotchas:** On a Unix shell, turn remote echo off first (`stty -echo`) or the download starts with a garbage line. Any open log for the current world is closed by `/getfile`.

```tf-noverify
/require filexfer.tf
/getfile_MUCK ~/mud/rooms.txt rooms
```

**Status:** see [Status](Status.md) (library file, needs a connected world)
