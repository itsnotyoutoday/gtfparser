# Functions: Miscellaneous

The functions that do not belong to a family of their own: output and sending
(`echo`, `send`, `prompt`), information about the running tf (`getpid`,
`gethostname`, `systype`, `isatty`, counters), type inspection (`whatis`),
simulating server text (`fake_recv`), the obsolete `read`, and the optional
scripting bridges (`python`, `calllua`). The library macros `/first`, `/rest`,
`/last`, `/nth`, `/split` and `/escape` are described in
[Functions-Strings](Functions-Strings.md).

## echo(text [, attrs [, inline [, dest]]])

**What it is for:** showing a line of text to the user from inside an expression, with full control over attributes and destination. It is the only way to echo text with leading or trailing spaces.

**Syntax:** `echo(s1)`, `echo(s1, attrs)`, `echo(s1, attrs, inline)`, `echo(s1, attrs, inline, dest)`. `attrs` is an attribute list such as `Cred`; `inline` set to 1 or "on" interprets `@{attr}` markers; `dest` is `"o"` (normal output, the default), `"e"` (the error stream), `"a"` (the alert stream), or `"w<world>"` (a world's own stream).

**Returns:** 1 on success.

```tf
/test echo("  indented text  ")
/test echo(strcat("HP: ", 42, "/", 120))
/echo [$[echo("x")]]
```
```output
  indented text  
HP: 42/120
x
[1]
```

**Gotchas:** the library command `/echo` is the usual front end (`/echo -aCred text`) and shares these options: see Commands-Output-and-Input. Use `echo()` when you need exact spaces: `/echo` trims them. Because `/echo` is a command, `/test echo(...)` is the way to call the function for its side effect only. In the output above the `x` line appears before the `[1]` line because the function prints while the surrounding `/echo` is still being built.

**Status:** see [Status](Status.md). Host capability: `Host` (output).

## send(text [, world [, flags]])

**What it is for:** sending a line to a world from within an expression or macro, as if you had typed it.

**Syntax:** `send(s1)`, `send(s1, s2)`, `send(s1, s2, s3)`: world `s2` (empty or omitted: the current world), flags `s3`: `"h"` runs matching `SEND` hooks first, `"u"` sends the text unterminated (no CR LF).

**Returns:** non-zero if the text was sent, 0 if it was not (no such world, not connected, or an invalid flag).

```tf-noverify
/test send("look")
/test send("say hello", "mymud")
/test send("partial command ", "", "u")
```

**Gotchas:** by default hooks are *not* run (unlike typing). The library command `/send` adds `-w`, `-W` (all worlds) and `-T` (by type) on top. Plain text on a macro line, such as `look`, is sent through the same path. Sending needs a host that owns connections (`WorldHost`); without one, the host's `Send` callback gets the text.

**Status:** see [Status](Status.md). Host capability: `Host` (`Send`), `WorldHost` for named worlds.

## prompt(text)

**What it is for:** setting the prompt of the current connection, so a status line or the input line shows server prompts such as `HP: 42>`.

**Syntax:** `prompt(s1)`.

**Returns:** non-zero if there is a current connection, 0 otherwise.

```tf-noverify
/def -h"PROMPT *> " catch_prompt = /test prompt({*})
```

**Gotchas:** it is most useful from a `PROMPT` hook, as above: any text ending in `>` without a newline becomes the prompt. The command `/prompt` (library) adds `-a` and `-p` for attributes. Needs a host that displays prompts.

**Status:** see [Status](Status.md). Host capability: `Notifier`.

## getpid()

**What it is for:** the process id of tf, for unique temp-file names or to signal tf from a script.

**Syntax:** `getpid()` returns an integer.

```tf
/echo $[getpid() > 0]
/def tmpname = /result strcat("/tmp/tf", getpid(), ".txt")
```
```output
1
```

**Gotchas:** the library macro `/signal` builds on it (`/signal` with no argument lists signal names).

**Status:** see [Status](Status.md).

## gethostname()

**What it is for:** the name of the machine tf runs on, for log headers or per-host configuration.

**Syntax:** `gethostname()` returns a string, empty if the name is unavailable.

```tf
/echo $[strlen(gethostname()) > 0]
```
```output
1
```

**Status:** see [Status](Status.md).

## systype()

**What it is for:** the operating-system family, so one configuration file can behave differently on Unix and elsewhere.

**Syntax:** `systype()` returns `"unix"` (this includes macOS), `"os/2"` or `"cygwin32"`.

```tf
/echo $[systype()]
/echo $[systype() =~ "unix" ? "unix-like" : "other"]
```
```output
unix
unix-like
```

**Status:** see [Status](Status.md).

## isatty()

**What it is for:** telling whether tf is talking to a real terminal, so a script can skip decoration when tf is run in a pipe or a test.

**Syntax:** `isatty()` returns 1 if tf has a terminal, 0 otherwise.

```tf-noverify
/if (isatty()) /echo interactive session%; /else /echo batch run%; /endif
```

**Gotchas:** an embedding program decides what this means; a headless host usually answers 0 (the reference tf run on a pty answers 1).

**Status:** see [Status](Status.md). Host capability: `TermHost`.

## whatis(value)

**What it is for:** finding out the type tf thinks a value has, mostly to debug surprising conversions.

**Syntax:** `whatis(x)` returns a string: `string`, `int`, `decimal` (a real number written as a literal), `float` (a real result of a calculation), `atime`, `dtime`, `enum`, `pos`, or the same prefixed with `id:` when the argument is a variable name and a variable of that name exists (`id:` alone when it does not).

```tf
/set name=Bob
/set n=5
/echo $[whatis(name)] $[whatis("x")] $[whatis(3)] $[whatis(2.5)] $[whatis(n)]
/echo $[whatis(1 + 1)] $[whatis(3 + 0.5)] $[whatis(strcat("a", 1))] $[whatis(nosuchvar)]
```
```output
id:string string int decimal id:string
int decimal string id:
```

**Gotchas:** a value read from a variable is a string until used as a number. `whatis(time())` is `atime` in real tf; the engine reports `int` (FINDINGS.md).

**Status:** see [Status](Status.md).

## nlog()

**What it is for:** how many log files are open, to decide whether to start another.

**Syntax:** `nlog()` returns an integer.

```tf
/echo $[nlog()]
```
```output
0
```

**Gotchas:** see `/log` in the commands reference.

**Status:** see [Status](Status.md). Host capability: `Notifier` (logging is owned by the client).

## nmail()

**What it is for:** how many monitored mail files hold unread mail.

**Syntax:** `nmail()` returns an integer; mail monitoring is controlled by the `MAIL` and `maildelay` variables.

```tf-noverify
/echo $[nmail()]
```
```output
0
```

**Status:** see [Status](Status.md).

## nread()

**What it is for:** telling whether a keyboard read is in progress (a macro waiting on `/read`-style input).

**Syntax:** `nread()` returns a positive number during such a read, 0 otherwise.

```tf-noverify
/echo $[nread()]
```
```output
0
```

**Status:** see [Status](Status.md). Host capability: `Notifier`.

## read()

**What it is for:** reading one line from the keyboard; obsolete. Use `tfread()` ([Functions-Files-and-Streams](Functions-Files-and-Streams.md)).

**Syntax:** `read()` returns the line read, or an empty string.

**Gotchas:** real tf prints a deprecation warning, and a keyboard read only works from a command-line command.

**Status:** see [Status](Status.md).

## fake_recv(text [, world [, flags]])

**What it is for:** making tf behave as if the *server* had sent a line: triggers fire, the line is displayed and logged. Useful for testing macros without a game, or for a script that wants its own messages treated like game output. It is undocumented in the manual.

**Syntax:** `fake_recv(s1)`, `fake_recv(s1, world)`, `fake_recv(s1, world, "r")`; the flag `r` feeds the text as raw network input (it goes through telnet and line splitting).

**Returns:** 1 on success; 0 with the message `no open world` if there is no open connection to receive it.

```tf-noverify
/def -mglob -t'The orc * you' on_orc = /echo Watch out!
/test fake_recv("The orc hits you")
```

The line is displayed like server text and the trigger prints `Watch out!`.

**Gotchas:** it needs an open world; without one real tf refuses. The line goes through the same processing as real server text, including triggers, so take care with macros that send commands in response.

**Status:** see [Status](Status.md). Host capability: `WorldHost`.

## python(expression)

**What it is for:** evaluating a Python expression from tf script and using the answer. It exists only in tf built with the TF Python patch.

**Syntax:** `python(s)`: a single Python *expression* (not a statement).

**Returns:** the value converted to a tf string, number or boolean; 0 on error.

```tf-noverify
/test echo(python("1+1"))
```
```output
2
```

**Gotchas:** quoting is applied twice, once for tf and once for Python. For statements and persistent state use the commands `/python`, `/python_load` and `/python_call`.

**Status:** see [Status](Status.md). Optional bridge; not part of the engine core.

## calllua(function [, args...])

**What it is for:** calling a function defined in a loaded Lua script, from a tf expression. It exists only in tf built with Lua support.

**Syntax:** `calllua(name, arg1, ...)`: the global Lua function `name` is called with the arguments as strings.

**Returns:** the Lua function's result; -1 if no script is loaded or no function name is given; 1 with the message "No such function" if the name is not a function.

```tf-noverify
/loadlua myscript.lua
/test echo(calllua("greet", "Bob"))
```

**Gotchas:** load a script first with `/loadlua`; see also `/calllua` and `/purgelua`.

**Status:** see [Status](Status.md). Optional bridge; not part of the engine core.

## Library

### /sys command (Library)

**What it is for:** running a shell command whose plain-text output should appear in tf's output window, e.g. `/sys date`.

**Syntax:** `/sys shell-command`; runs the command without a terminal and echoes its output. Its value is the shell's exit status (0 means success, the opposite of tf's own truth convention).

```tf-noverify
/sys echo hello
```
```output
hello
```

**Gotchas:** it is synchronous and cannot run interactive programs (use `/sh` for those). **Never** build a `/sys` command from server text without quoting it; a hostile line could run commands on your account. The engine's `/sys` rests on `/quote -S` and may be restricted by `/restrict`.

**Status:** library macro from `stdlib.tf`; see [Status](Status.md).

### /cat (Library)

**What it is for:** typing several lines straight into tf, such as a multi-line macro definition, ending with a line containing just `.`.

**Syntax:** `/cat`, then lines, then `.`. The collected text is run as one command line (lines joined with `%;`). Needs keyboard input, so it works only interactively.

**Status:** library macro from `stdlib.tf`; see [Status](Status.md). Host capability: `Notifier` (keyboard reading).

### /ver and /vercmp (Library)

**What they are for:** reading the running tf version number and comparing version strings, so a script can check for features.

**Syntax:** `/ver` returns the short version (e.g. `5.0 beta 8`) via `/result`; `/vercmp v1 v2` compares two versions in the form `5.0 beta 8` (`major.minor level revision`, level one of `alpha`, `beta`, `gamma`, `stable`) and returns a negative, zero or positive number.

```tf
/def newer = /echo $[vercmp("5.0 beta 9", "5.0 beta 8") > 0]
/newer
/echo $[vercmp("5.1 stable 0", "5.0 beta 9") > 0]
```
```output
1
1
```

**Gotchas:** pass the versions as *function arguments* (with quotes) or as single words; as command arguments each space-separated word is a parameter. A malformed version prints `Bad version format` and returns -2.

**Status:** library macros from `stdlib.tf`; see [Status](Status.md).

### /signal (Library)

**What it is for:** sending a Unix signal to tf itself, for example to test its signal handling.

**Syntax:** `/signal [name]`: with no argument it lists the signal names (`kill -l`); otherwise it runs `kill -name <tf's pid>` through `/quote`.

**Status:** library macro from `stdlib.tf`; see [Status](Status.md).
