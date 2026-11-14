# Commands: System

Commands about tf itself rather than about the game: who may do what
(`/restrict`), leaving and restarting, the version and feature list, help,
the output-flood guards, the optional Python and Lua script hosts, and the
remote-control bridge of the squat build. "Library" entries are macros defined
by tf's `stdlib.tf`, not C builtins. For what this build of the engine
supports see [Status](Status.md); many of these commands need the embedding
program (the host) to give them meaning - capabilities are named on each entry
(`Notifier`, `HelpHost`, `WorldHost`; see `host.go`).

Messages beginning with `%` are printed to the message stream and are not part
of the verified `output` blocks.

Contents: [/restrict](#restrict-shellfileworld) - [/quit](#quit--y) -
[/reboot](#reboot--y) - [/suspend](#suspend) - [/watchdog](#watchdog-offonn1-n2) -
[/watchname](#watchname-offonn1-n2) - [/features](#features-name) -
[/help](#help-topic) - [/version](#version) - [/core](#core) -
[/squat](#squat-subcommand) - [/control](#control-world-tfremoteboth) -
[/python and friends](#python-command) - [Lua](#calllua-function-args) -
Library: [/ver](#library-ver) - [/vercmp](#library-vercmp-v1-v2) -
[/changes](#library-changes-version) - [/man](#library-man) - [/sys](#library-sys-command) -
[/signal](#library-signal-sig) - [/time](#library-time-format) -
[/runtime](#library-runtime-command)

---

## /restrict [shell|file|world]

**What it is for:** locking tf down so scripts (or users on a shared account)
cannot reach outside it. The level only goes up, never down.

**Syntax:** `/restrict [none|shell|file|world|0|1|2|3]`

**Levels.**

| level | name | effect |
|---|---|---|
| 0 | none | no restrictions |
| 1 | shell | no `/sh`, no `/quote !`, no uncompressing during `/load` and `/help` |
| 2 | file | also no `/load`, `/save`, `/saveworld`, `/lcd`, `/log`, `/quote '`, `tfopen()`, the `sockmload` feature (implies `shell`) |
| 3 | world | also no new user-defined connections: `/addworld` and `/connect host port` are disabled (implies `file`) |

With no argument it reports the current level (`% restriction level: none`).
An unknown word is rejected with the list of valid values. A site
administrator would normally put `/restrict` in `local.tf`. The return value
is 1 on success and 0 when refused (for example when trying to lower the
level).

```tf
/restrict shell
/sh echo not executed
/echo /sh returned %?
/restrict none
/echo lowering returned %?
```
```output
/sh returned 0
lowering returned 0
```

**Gotchas.**
- Lowering fails loudly (`restriction level cannot be lowered`) and keeps the
  higher level; restrict is a one-way door for the life of the interpreter.
- A host can be told when the level changes (it receives a `restrict` event)
  and may want to apply the same limit to its own file and network access.
- The squat build also has a per-script sandbox, see [/squat](#squat-subcommand).

**Status:** see [Status](Status.md). Capability: Notifier (change events).

---

## /quit [-y]

**What it is for:** leaving tf.

**Syntax:** `/quit [-y]`. In an interactive session with unseen text in some
world, `/quit` first asks for confirmation (anything but `y` cancels); `-y` quits
without asking. Quitting closes all connections and logs, kills `/repeat` and
`/quote` processes and discards history and unsaved macros and variables. Return
value is 1 when quitting was requested, 0 if the user said no.

To never be asked: `/def quit = /@quit -y`.

```tf-noverify
/def -hDISCONNECT = /echo Left %1.
/quit -y
```

**Gotchas.** An embedding program decides what "quit" means for it: this
engine records the request and the host acts on it, so a script that keeps
running after `/quit` is still executing in that run.

**Status:** see [Status](Status.md).

---

## /reboot [-y]

**What it is for:** restarting the tf program in place (re-executing the
binary, for upgrades) while keeping the open connections where the system
allows it - an "online reboot".

**Syntax:** `/reboot [-y]`. Without `-y`, if there are active sessions in an
interactive tf it asks `Online reboot now? Active sessions will be preserved
when possible. (y/N)`. An extra argument is an error. Unix builds only.

**Gotchas.** This is a feature of the squat-patched tf. An embedding program
has to supply its own restart mechanism; without one the command fails loudly.

**Status:** see [Status](Status.md). Capability: Notifier.

---

## /suspend

**What it is for:** stopping the tf process and returning to the shell, like
typing `^Z`, if the system has job control.

**Syntax:** `/suspend`. When tf is resumed it redraws the screen and then runs
the `/repeat` and `/quote` work that came due and processes text received
meanwhile.

**See also:** `/signal`, the `SIGTSTP`-related hooks.

**Status:** see [Status](Status.md). Capability: Notifier (the host owns the
terminal and decides what suspending means).

---

## /watchdog [off|on|n1 n2]

**What it is for:** suppressing runaway repeated lines (a spamming object or a
stuck loop): a line that has appeared `n1` times among the last `n2` lines is
gagged.

**Syntax:** `/watchdog` (report), `/watchdog on`, `/watchdog off`,
`/watchdog n1 n2` (set thresholds and turn on). It sets the flag `%watchdog`,
default off.

Real tf prints `% watchdog enabled, searching for 3 out of 6 lines` and
`% watchdog disabled.`. The thresholds are independent from `/watchname`'s.

**Gotchas.** Despite the manual saying `n1` defaults to 2 and `n2` to 5, the
real command needs *both* numbers: `/watchdog 3` fails with `invalid or missing
numeric argument`. A bare `/watchdog` only reports the state. The guard
applies only to lines subject to gagging (it does nothing when `%gag` is off).

```tf-noverify
/watchdog 3 6
/watchdog off
```

**Status:** see [Status](Status.md). Capability: WorldHost (it works on each
world's incoming text).

---

## /watchname [off|on|n1 n2]

**What it is for:** gagging a person who floods you: if the first word of a
line (a player's name) begins `n1` of the last `n2` lines, tf defines a gag
`{Name}*` for that name and says so (`Watchname: gagging "{Name}*"`).

**Syntax:** same shape as `/watchdog`. Flag `%watchname`, default off;
defaults when turned on without numbers are 4 out of 5.

Real tf prints `% watchname enabled, searching for 4 out of 5 lines` for
`/watchname on`. The same both-numbers requirement as `/watchdog` applies to
the two-argument form (`/watchname 2` fails).

**Gotchas.** The created gag is an ordinary macro with priority from `%gpri`; list
or remove it with `/listdef -mglob`/`/undef`.

**Status:** see [Status](Status.md). Capability: WorldHost.

---

## /features [name]

**What it is for:** finding out which optional parts this tf was built with, so
a script can degrade gracefully.

**Syntax:** `/features` prints every feature prefixed `+` (on) or `-` (off);
`/features name` returns 1 or 0 (case-insensitive).

Features: `256colors`, `core`, `float`, `ftime`, `history`, `IPv6`, `locale`,
`MCCPv1`, `MCCPv2`, `process` (`/repeat` and `/quote`), `SOCKS`, `ssl`,
`subsecond`, `TZ`.

Real tf (one build) prints:

```tf-noverify
/features
```
```output
+256colors -core +float +ftime +history -IPv6 +locale +MCCPv1 +MCCPv2 +process -SOCKS -ssl +subsecond +TZ
```
and as a function: `/if (!features("ssl")) /echo -e warning: not secure%; /endif`.

**Gotchas.** The answer describes this build, so an embedding program reports
what it provides; do not hard-code the real tf list.

**Status:** see [Status](Status.md).

---

## /help [topic]

**What it is for:** showing a help topic.

**Syntax:** `/help [topic]`; with no topic it shows the summary of topics.
Punctuation in a topic is optional where unambiguous: `def` finds `/def`,
`%MAIL` is the variable but `MAIL` is the hook. Topics also exist for syntax
(`%{}`, `$()`). Real tf reads the file named by `%TFHELP` (and its index) and
prints `% Help on subject X not found.` for an unknown topic.

**Gotchas.** The engine has no help text of its own: it hands the topic to
the host. Without a help-capable host the command fails loudly instead of
printing nothing. `/man` is the same command.

**Status:** see [Status](Status.md). Capability: HelpHost.

---

## /version

**What it is for:** showing which tf this is: version, copyright, patch credits,
and the system it was built for. Return value 1.

**Syntax:** `/version`. Real tf prints four `%` lines (version, copyright,
contributors/mods, "Built for ..."). The library macro `/ver` extracts just the
version number from this output.

```tf-noverify
/version
```

**Gotchas.** The exact text is the host's identity (this engine reports its
own name and version), and anything parsing it should use `/ver` and
`/vercmp` rather than the text.

**Status:** see [Status](Status.md).

---

## /core

**What it is for:** debugging only: deliberately makes tf dump core (it
reports an internal error and calls the core-dump routine). Do not use it in
scripts.

**Syntax:** `/core [message]`. Whether a core file is actually written depends on the `core` feature (see `/features`).

**Gotchas.** In an embedding program a host has no reason to implement
it; calling it should simply fail.

**Status:** see [Status](Status.md).

---

## /squat [subcommand]

**What it is for:** (squat-patched tf only) pairing the running tf with a
remote client (SquatClient) through a helper process, `tf-squatterd`, so the
remote client can see all connections, type into them and run triggers.
Nothing in tf itself listens on the network; the helper is tf's child.

**Syntax.**

| command | meaning |
|---|---|
| `/squat` or `/squat status` | report `on`/`off`, bridge state, sessions |
| `/squat on` | start the bridge (default `127.0.0.1:10328`, TLS) |
| `/squat off` | stop it (`already off` if not running) |
| `/squat pair [viewer\|operator\|admin]` | print a one-time pairing code and TLS fingerprint (default `operator`) |
| `/squat devices` / `/squat revoke <id>` | list paired devices / revoke one |

Anything else prints a usage line. Subcommands other than `status`/`on`/`off`
need the bridge running (`squat is off; use /squat on`).

**Settings** (ordinary variables, read when `/squat on` runs unless noted):
`squat_bin` (helper, default `tf-squatterd`), `squat_port` (10328),
`squat_bind` (127.0.0.1), `squat_dir` (`~/.squatterd/tf`: keys, certificate,
devices, log), `squat_plain` (non-zero: plain `ws://`, loopback only),
`squat_log` (`info`/`debug`), `squat_takeover` (`auto`/`ask`/`off`),
`squat_ask_timeout` (30 s), `squat_exec` (`off`/`allow`/`on`: remote
commands for admin devices), `squat_exec_allow` (command names permitted under
`allow`, default `version listworlds listsockets listdef listvar recall help`),
`squat_scripts` (`off`/`safe`/`on`: uploaded scripts), `squat_disp`, `squat_mtts`.
`squat_status` shows, e.g., `sq:3 R1 A1 ?1` (sessions, remote-input worlds,
client-automation worlds, takeover requests waiting).

**Security notes.** `squat_exec=on` is equivalent to a shell on the account;
use `allow`. `squat_scripts=safe` loads an uploaded script as under
`/restrict world` (plus no Lua and no `/restrict`, `/squat`, `/control`, or
changing `squat_*`) both while it loads and whenever a macro it defined runs.
Text a remote device types into a game is sent verbatim and is never run as a
tf command. While sandboxed, `/squat` answers `restricted`.

```tf-noverify
/squat on
/squat pair operator
/squat status
```

**Status:** see [Status](Status.md). Capability: WorldHost (and a bridge
implementation supplied by the program).

---

## /control [world|* tf|remote|both]

**What it is for:** (squat build) deciding, per world, who types into it and
who runs automation when a remote client is attached.

**Syntax.**

| command | meaning |
|---|---|
| `/control` | list each world's `input=` and `automation=` (and waiting requests) |
| `/control <world>\|* tf\|remote\|both` | who may type: only the tf keyboard, only the remote client (tf's keyboard locked for that world), or both |
| `/control <world>\|* automation tf\|client` | whether tf's triggers run, or tf skips trigger matching for that world while a client owns it |
| `/control pending` | list takeover requests waiting for an answer |
| `/control allow [n]` / `/control deny [n]` | answer request number `n` (default 1) |

Both switches revert to `tf` when the client disconnects or the bridge stops;
`/control <world> tf` always takes a world back. An unknown world gives `no such
world`. Remote text goes to the game verbatim and the window size the game is
told follows the controller.

```tf-noverify
/control
/control Cave remote
/control * automation tf
```

**Status:** see [Status](Status.md). Capability: WorldHost.

---

## /python [command]

**What it is for:** (Python-patched tf only) running a Python statement in a
persistent embedded interpreter, mainly from `tfrc` setup; `python(expr)` is
the function form that returns the value of an *expression*.

**Syntax:** `/python statement`. The environment persists between calls
(`/python import glob`). stdout is discarded unless you set
`sys.stdout.output = tf.out`; stderr goes to tf's error output. The `tf`
module lets Python call back (`tf.out`, `tf.eval`).

**Gotchas.** The command is recompiled every time, so use `/python_load` and
`/python_call` for anything repeated. Quoting is two-layered when called from
`python()`: once for tf, once for Python. The reference tf used to verify these
docs was built without Python.

**Status:** see [Status](Status.md). Not in the engine's own scope: an embedding
program that wants scripting in Python would register these through the extension
API.

---

## /python_load module

**What it is for:** (re)importing a Python module from `PYTHONPATH`, `TFPATH`
and `TFLIBDIR`, reloading it if it changed, so its functions can be used with
`/python_call`.

**Syntax:** `/python_load module`.

**Status:** see [Status](Status.md).

---

## /python_call module.function argument

**What it is for:** calling a function of a loaded module with the entire rest
of the line as one string argument - the fast, quoting-free way to run Python
from a trigger.

**Syntax:** `/python_call module.function argument`.

```tf-noverify
/python_load mymodule
/def -mglob -p9 -Fq -t'howdy *' howdy = /python_call mymodule.howdy %{*}
```

**Status:** see [Status](Status.md).

---

## /python_kill

**What it is for:** the dead-man switch: throws away the Python interpreter and
every loaded module; the next Python command starts a fresh one.

**Syntax:** `/python_kill`.

**Status:** see [Status](Status.md).

---

## /loadlua file

**What it is for:** (Lua-patched tf) loading a Lua script into the embedded Lua
state (created on first use), so its functions can be called with `/calllua`.

**Syntax:** `/loadlua filename`. Returns 0 on success, 1 on error (`Cannot
load script`). Scripts get these functions: `tf_eval(cmd)`, `tf_getvar(name)`,
`tf_setvar(name, value)`, `tf_unsetvar(name)`, `tf_sendstr(text[, world[, flags]])`,
`tf_sendvar(name[, world[, flags]])`.

**Gotchas.** Refused (`restricted`) while a squat sandbox is active. The
reference tf used for these docs was built without it.

**Status:** see [Status](Status.md).

---

## /calllua function [args]

**What it is for:** calling a global Lua function by name, typically from a
trigger, passing the text of the line.

**Syntax:** `/calllua name [text]`. With text, the function is called with three
arguments: the text string, a table of per-character attribute codes, and the
attribute code of the whole line; without text it is called with none. Returns 0
on success, 1 on `No script loaded`, `No such function` or a Lua error.
`calllua("name", ...)` is the function form.

```tf-noverify
/loadlua ~/tf/triggers.lua
/def -mglob -t'* tells you: *' tell = /calllua on_tell %*
```

**Status:** see [Status](Status.md).

---

## /purgelua

**What it is for:** closing the Lua state and forgetting every loaded script.

**Syntax:** `/purgelua`. Prints `All LUA scripts forgotten` and returns 1, or
`No LUA scripts loaded` and returns 0.

**Status:** see [Status](Status.md).

---

## Library: /ver

**What it is for:** the bare version number, for comparing versions.

**Syntax:** `/ver` (library macro; result is the version string, e.g. `5.0
beta 8`). It parses the first line of `/version`, so it relies on that line's
exact shape.

```tf-noverify
/echo running tf $(/ver)
```

**Status:** library macro. See FINDINGS (this engine's `/version` text).

---

## Library: /vercmp v1 v2

**What it is for:** comparing two tf version strings of the form `5.0 beta 8`.

**Syntax:** `/vercmp "5.0 beta 8" "5.0 beta 7"` returns negative, 0 or positive
like `strcmp`; major and minor numbers are numeric, the level
(`alpha` < `beta` < `gamma` < `stable`) is alphabetical, and the last number is the
revision. A malformed string prints an error and returns -2.

```tf
/echo $[vercmp("5.0 beta 8", "5.0 beta 7")]
/echo $[vercmp("4.0 stable 1", "4.0 stable 1")]
/echo [$[vercmp("5.0 alpha 8", "5.0 beta 1") < 0]]
/echo [$[vercmp("x", "4.0 stable 1")]]
```
```output
1
0
[1]
% vercmp: Bad version format "x"
[-2]
```

**Gotchas.** `/echo $[...]` of a bare negative number would be read as an option, so the
examples wrap results in brackets or test the sign. The error line is an
`/echo -e` message and a host may route it to its message area.

**Status:** library macro.

---

## Library: /changes [version]

**What it is for:** listing what changed in a tf version, read from
`%TFLIBDIR/CHANGES`; the default is the running version (`$(/ver)`).

**Syntax:** `/changes [5.0|5.0 beta 7]`.

**Status:** library macro (`lib/tf/changes.tf`, loaded by `stdlib.tf`).

---

## Library: /man

**What it is for:** an alias: `/man topic` is `/help topic`.

**Status:** library macro; see [/help](#help-topic).

---

## Library: /sys command

**What it is for:** running a non-interactive shell command and showing its
output in the tf window, without the extra messages and screen redraw of `/sh`.

**Syntax:** `/sys shell-command` (it is `/quote -S -decho !shell-command`).
Synchronous; returns the shell's exit status when it exited normally and -1
otherwise - note that this is the Unix convention (0 = success), the opposite
of tf's usual truth. For background work use `/quote` without `-S`.

**Gotchas.** Never build the command from game text in a trigger: anyone on the
server could run commands on your account. `/restrict shell` disables it.

**Status:** library macro (needs `/quote`).

---

## Library: /signal [sig]

**What it is for:** sending a signal to the tf process; with no argument,
listing the signal names.

**Syntax:** `/signal [name]` (it runs `kill -SIG <pid>` via `/quote`; `/signal`
alone shows `kill -l`'s list). Typical names: `HUP INT QUIT KILL SEGV TERM USR1
USR2 TSTP`. Hooks exist for `SIGHUP`, `SIGTERM`, `SIGUSR1` and `SIGUSR2`.

**Status:** library macro.

---

## Library: /time [format]

**What it is for:** printing the current time; the format is an `ftime()`
format (default `%time_format`).

**Syntax:** `/time [format]`. Mind that `%` is the substitution character: type
`/time %%H:%%M` to pass `%H:%M`. The return value is the time as a number.

```tf
/time Format codes would go here
```
```output
Format codes would go here
```

**See also:** `time()`, `ftime()`, `mktime()` in [Functions-Time](Functions-Time.md).

**Status:** library macro.

---

## Library: /runtime command

**What it is for:** timing a command: runs it and prints `real=<seconds>
cpu=<seconds>`; the return value is the command's.

**Syntax:** `/runtime command` (the command gets no extra substitution).

```tf-noverify
/runtime /repeat -0 1 /echo done
```
Output is like `real=0.001234 cpu=0.000211`.

**Status:** library macro (uses `time()` and `cputime()`).
