# Commands: Output and Input

How a script talks to you (`/echo`), to the game (`/send`), and to the input
line (`/input`, `/grab`), plus the small display toggles that belong with them.
In real tf several of these (`/echo`, `/send`, `/prompt`, `/substitute`, `/cat`,
`/paste`, `/time`, `/rand`, `/kecho`...) are macros in `stdlib.tf` that call
builtin *functions* (`echo()`, `send()`, `prompt()`...); they are marked
**Library**. See [Status](Status.md) for which of them this build provides.

Examples run with full substitution of each line, as if `/set sub=full` (in real tf a typed line is only expanded in that mode; inside macros and `/eval` it always is).

Output examples in this page run with the standard library loaded. A line sent
to the game by the test runner appears as `[send] text`.

## /echo [-aattrs] [-p] [-e|-A|-wworld] text

**Library.** Shows a line on your screen (the `tfout` stream). Nothing is sent to
the game and the text is not matched against triggers; use `/trigger` to feed
text to your own triggers.

**Syntax:** `/echo [-aattrs] [-p] [-e] [-A] [-w[world]] text`

**Options**

| Option | Meaning |
|---|---|
| `-aattrs` | display the line with attributes (`u` underline, `B` bold, `C`+colour such as `Cred`, `g` gag ...) |
| `-p` | interpret inline `@{u}...@{n}` attribute codes in the text |
| `-e` | write to the error stream instead of the screen |
| `-A` | write to the alert stream |
| `-wworld` | echo into that world's output (current world if the name is empty) |

```tf
/echo Your shield glows.
/echo -- -- a line that starts with dashes
/set hp=17
/echo HP: %hp / $[hp * 2]
```
```output
Your shield glows.
-- a line that starts with dashes
HP: 17 / 34
```

**Gotchas**

- `--` ends option parsing, so text beginning with `-` needs `/echo -- -text`. (A lone `-`, which the real docs also mention, is printed literally by both real tf and this engine.)
- Leading and trailing spaces are trimmed by the command form; the function form
  keeps them: `/test echo("  padded  ")`.
- `/echo` is a macro in the library; `/_echo text` is the faster builtin form for
  hot paths, with no options.
- Attribute handling (`-a`, `-p`) relies on `decode_attr()`; see Status for whether
  this build renders attributes or shows the raw `@{...}` text.

**Status:** see [Status](Status.md). Capability: `Host` (echo).

## /_echo text

**Library.** The plain, option-free `/echo`: prints `text` and nothing else. Use it
inside macros that fire on every line of game output.

```tf
/_echo 100%% sure
```
```output
100% sure
```

**Status:** see [Status](Status.md). Capability: `Host`.

## /send [-W] [-Ttype] [-wworld] [-n] [-h] text

**Library.** Sends a line to a world as if you had typed it, *without* the
`SEND` hook and without macro or `/` interpretation of the text (unlike typing
it at the keyboard).

**Syntax:** `/send [-W] [-Ttype] [-wworld] [-n] [-h] text`

**Options:** `-wworld` sends to a named world (default: the current one); `-W`
to every connected world; `-Ttype` to every connected world of that type; `-n`
sends with no end-of-line; `-h` first runs matching `SEND` hooks.

```tf
/send look
/def heal = /send cast cure %1
/heal bob
/send -w look
```
```output
[send] look
[send] cast cure bob
[send] look
```

**Gotchas**

- The return value is non-zero only if the line was actually sent, so
  `/if /send ...` tests for "there is a connection", not for the game's answer.
- Plain text on a command line (no leading `/`) is also sent; `/send` is for the
  cases where text begins with `/` or `%`, or must go to a specific world.
- Also available as the function `send(text [, world [, flags]])`; flag `h` runs
  hooks and `u` omits the newline.

**Status:** see [Status](Status.md). Capability: `Host` (send), `WorldHost` (named worlds).

## /input text

**What it is for:** inserting text into the input line as if you had typed it,
keeping what is already there. Together with `/bind` it makes key shortcuts that
expand to longer text.

**Syntax:** `/input text`

```tf-noverify
/bind ^[oj = /input OliverJones
; typing "page ^[oj = hi" now leaves "page OliverJones = hi" on the input line
```

**Gotchas.** Nothing is sent until you press Enter; with no input line (a headless
host) there is nowhere to put the text.

**Status:** see [Status](Status.md). Capability: `Notifier`/input host.

## /grab text

**Library.** Replaces the whole input line with `text` (compare `/input`, which
inserts). Mostly used from a macro that prepares an edit for you.

```tf-noverify
/def reedit = /grab /edit %1 = $%1
/reedit flail
; the input line now holds:  /edit flail = :flails at his keyboard
```

**Status:** see [Status](Status.md). It uses `kblen()`, `dokey()`/`/dokey dline`
and `/input`.

## /beep [number|on|off]

**What it is for:** ringing the terminal bell, e.g. from a trigger to alert you.

**Syntax:** `/beep [number|ON|OFF]`

**Arguments.** `number` bells (default 3). `off` silences later `/beep`s until
`/beep on`. Many terminals merge back-to-back bells; space them with
`/repeat -0.2 5 /beep`.

```tf-noverify
/def -t'* pages you*' pagebeep = /beep 2
```

**Status:** see [Status](Status.md). Capability: `Notifier` (kind `beep`).

## /localecho [on|off]

**What it is for:** controlling whether tf echoes what you type, which matters at
password prompts. Library hooks call it for you; you rarely do.

**Syntax:** `/localecho [on|off]`

**Arguments.** No argument returns 1 when local echo is on for the current
socket. `on` tells the server not to echo (so tf must); `off` asks the server to
echo. It only does something on connections that speak TELNET and the change
happens when the server agrees.

**Status:** see [Status](Status.md). Capability: `Notifier` (kind `localecho`).

## /prompt [-aattrs] [-p] text

**Library.** Sets the current socket's prompt line to `text`, replacing the old
one. It is meant for `PROMPT` hooks that capture a MUD prompt such as
`HP: 40>` so it stays at the bottom of the screen.

**Syntax:** `/prompt [-aattrs] [-p] text`

```tf-noverify
/def -h'PROMPT *> ' catch_prompt = /test prompt({*})
```

**Gotchas.** Also the function `prompt(text)`; the command passes `text`
through `decode_attr()`.

**Status:** see [Status](Status.md). Capability: `Notifier`.

## /substitute [-aattrs] [-p] text

**Library.** Called from a trigger, replaces the *entire matched line* with
`text` before it is displayed or tested by lower-priority triggers. Typical use:
strip a decoration or rewrite a line.

**Syntax:** `/substitute [-aattrs] [-p] text`

```tf
/def -F -p5 -t'hello*' sub1 = /substitute -- HI %-1
/def -p1 -t'HI*' second = /echo second saw: %*
/trigger hello world
```
```output
second saw: HI world
```

The fall-through trigger rewrites the line; the lower-priority trigger then matches
and sees the new text.

**Gotchas**

- Only meaningful while a trigger is running; the original line is lost.
- On a `fall-thru` trigger (`-F`), lower-priority triggers see the new text.
- Also the function `substitute(text [, attrs [, inline]])`.

**Status:** see [Status](Status.md).

## /cat [%]

**Library.** Collects typed lines until a line holding a single `.`, then
executes them as one. With `%`, a `%;` is appended between lines so the result is
a command list.

```tf-noverify
/cat %
:waves
:bows
.
```

**Gotchas.** Reads from the keyboard (`tfread`), so it needs a host that provides
an input line.

**Status:** see [Status](Status.md).

## /paste [-pnxsqth] [-wworld] [-eend] [-aabort] [prefix]  /endpaste

**Library.** Enters a mode in which every typed (or pasted) line, including lines
that start with `/`, is sent to the world with `prefix` in front (default
`:|`, from `%paste_prefix`). Ends with `/endpaste` or `.` on a line by itself;
`/abort` cancels.

**Options:** `-p` joins adjacent non-blank lines into paragraphs; `-n` no prefix;
`-x` executes each line as a command instead of sending it; `-s` strips trailing
spaces; `-q` quiet; `-h` runs `SEND` hooks per line.

```tf-noverify
/paste -p
This is one long paragraph that was pasted
from a web page and wrapped at 40 columns.
.
```

**Status:** see [Status](Status.md).

## /more [on|off]

**Library.** Turns the pager on or off: with `more` on, output stops with a "More"
prompt when the screen fills, and TAB scrolls. It is the library wrapper that
sets the `more` variable (and shows a one-time hint).

```tf
/more off
/echo more is %more
/more 0
/echo more is %more
```
```output
more is off
more is off
```

**Status:** see [Status](Status.md). The pager itself belongs to the display host
(`moresize()`, `morepaused()`, `morescroll()`).

## /wrap [size|on|off]  and  /nowrap

**Library.** `/wrap` turns line wrapping on or off; a number is the column count
and also switches wrapping on. `/nowrap` is `/wrap off`.

```tf
/wrap 50
/echo wrap=%wrap size=%wrapsize
/wrap off
/echo wrap=%wrap
/wrap on
/nowrap
/echo wrap=%wrap
```
```output
wrap=on size=50
wrap=off
wrap=off
```

**Status:** see [Status](Status.md).

## /kecho, /mecho, /qecho [on|off|all|prefix]

**Library.** Control echoing of the *results of* keyboard-typed commands
(`/kecho`), macro commands (`/mecho`) and `/quote` output (`/qecho`).
Given on/off (or `1`/`0`) they set `kecho` / `mecho` / `qecho`; `/mecho all`
(or `2`) also echoes nested macro commands. Any other argument becomes the
echo *prefix* (`kprefix`, `mprefix`, `qprefix`) and turns echoing on.

```tf
/kecho on
/echo kecho=%kecho
/mecho all
/echo mecho=%mecho
/qecho off
/echo qecho=%qecho
```
```output
kecho=on
mecho=all
qecho=off
```

**Gotchas.** `/kecho >>` (a prefix) additionally prints `% kprefix=>>`, the output
of `/set kprefix`.

**Status:** see [Status](Status.md).

## /time [format]

**Library.** Prints the current time using an `ftime()` format (default
`%{time_format}`).

```tf-noverify
/time %A %H:%M
```

**Status:** see [Status](Status.md).

## /rand [n [m]]

**Library.** Prints a random number: none, `[0, n-1]` with one argument, or
`[n, m]` with two. It is `echo(rand(...))`.

```tf
/rand 5 5
/echo $[rand(1,1)]
```
```output
5
1
```

**Gotchas.** More than two arguments is an error.

**Status:** see [Status](Status.md).
