# Functions: Terminal, Input Line and Status Bar

These functions look at, and edit, the things a full-screen client owns: the size of the screen, the text currently typed on the input line, the "more" pager and the status bar. They are what keybinding macros (see the `kb-*.tf` libraries shipped with tf) are built from.

The engine keeps this state itself: an input buffer you can fill with `/input` (or that a client fills as the user types), the status-bar layout, and per-world pager counters. A client draws from that state. Only a few things need the embedding program: `TermHost` (screen size), `KeyHost` (terminal key sequences), and the optional `MoreHost`, `WindowHost`, `StatusHost` and `PromptHost` overrides that let a client that owns the screen answer the pager and status questions itself. Without them the engine uses the documented defaults noted per entry. The example blocks here run in a bare interpreter (80x24, empty input line), so they type text with `/input` first. See `docs/engine/terminal-builtins.md` for the client side.

## columns()

**What it is for:** laying out text to the width of the screen (tables, centred banners).

**Syntax:** `columns()` returns an integer.

**Arguments/Returns:** the number of columns of the screen. Without a `TermHost` this engine answers the documented default of 80.

```tf
/echo width: $[columns()]
/echo rule: $[strrep("-", columns() / 8)]
```
```output
width: 80
rule: ----------
```

**Gotchas:** The value changes when the window is resized (a `RESIZE` hook fires); do not cache it for long. Real tf also keeps it in the variable `%COLUMNS`.

**Status:** see [Status](Status.md) (host capability: TermHost; defaults to 80x24 without it)

## lines()

**What it is for:** the height of the whole screen, including status bar and input line.

**Syntax:** `lines()` returns an integer; 24 without a `TermHost`. For the height of just the output window use [`winlines()`](#winlines).

```tf
/echo screen height: $[lines()]
```
```output
screen height: 24
```

**Status:** see [Status](Status.md) (host capability: TermHost; defaults to 80x24 without it)

## winlines()

**What it is for:** the number of lines in the output window (screen height minus status area and input window), useful for choosing how many lines to show or scroll by.

**Syntax:** `winlines()` returns an integer.

```tf
/echo screen $[lines()], window $[winlines()], page $[winlines() - 1]
```
```output
screen 24, window 23, page 22
```
In the default (non-visual) layout the window is one line shorter than the screen (the input line); in visual mode it also loses `%isize` input lines and `%status_height` status rows.

**Status:** see [Status](Status.md) (host capability: WindowHost, optional: the core answers from the screen size)

## keycode(name)

**What it is for:** finding the exact bytes your terminal sends for a named key ("up", "F1", "HOME") so that you can bind them portably with `/bind` or `/def -b`.

**Syntax:** `keycode(name)` returns a string, or the empty string when the terminal defines no such key.

**Arguments/Returns:** `name` is a termcap/terminfo key label (`kcuu1`-style capability names are accepted by real tf; see the `keybindings` topic). The answer comes from the terminal database selected by `%TERM`. Without a `KeyHost` this engine returns an empty string for every name.

```tf
/echo [$[keycode("F1")]]
```
```output
[]
```

**Gotchas:** An empty result is not an error; bind only if the string is non-empty, e.g. `/if ((key := keycode("kcuu1")) !~ "") /def -b'%{key}' ...`.

**Status:** see [Status](Status.md) (host capability: KeyHost)

## kblen()

**What it is for:** how long the line the user is typing is, for macros that edit it.

**Syntax:** `kblen()` returns the number of bytes in the input buffer.

```tf
/input look at (the) dragon
/echo input is $[kblen()] characters long
```
```output
input is 20 characters long
```

**Gotchas:** Input-line functions are only meaningful while the user is at the keyboard (from a key binding, a `/dokey` macro, or a command typed on the line); from a trigger they see whatever is half-typed. The unit is bytes, not display columns.

**Status:** see [Status](Status.md) (host capability: none needed: the core owns the input buffer and the client feeds it)

## kbpoint()

**What it is for:** where the cursor is within the input line.

**Syntax:** `kbpoint()` returns the cursor position, counting from 0.

```tf
/input hello world
/test kbgoto(5)
/echo cursor at $[kbpoint()] of $[kblen()]
```
```output
cursor at 5 of 11
```

**Status:** see [Status](Status.md) (host capability: none needed: the core owns the input buffer and the client feeds it)

## kbhead()

**What it is for:** getting the text before the cursor.

**Syntax:** `kbhead()` returns a string: the input line up to (not including) the cursor.

```tf
/input look at the dragon
/test kbgoto(8)
/echo typed so far: "$[kbhead()]"
```
```output
typed so far: "look at "
```

**Status:** see [Status](Status.md) (host capability: none needed: the core owns the input buffer and the client feeds it)

## kbtail()

**What it is for:** the text after the cursor; with `kbhead()` it splits the line in two for completion-style macros.

**Syntax:** `kbtail()` returns a string.

```tf
/input look at the dragon
/test kbgoto(8)
/echo before=[$[kbhead()]] after=[$[kbtail()]]
```
```output
before=[look at ] after=[the dragon]
```

**Status:** see [Status](Status.md) (host capability: none needed: the core owns the input buffer and the client feeds it)

## kbgoto(position)

**What it is for:** moving the cursor to an exact position in the input line.

**Syntax:** `kbgoto(i)` returns the new position, which differs from `i` if `i` was outside the buffer (it is clamped).

```tf
/input abc
/echo $[kbgoto(1)] $[kbgoto(99)] $[kbgoto(0)]
```
```output
1 3 0
```

**Gotchas:** Use `/test kbgoto(...)` to call it for its side effect and ignore the result.

**Status:** see [Status](Status.md) (host capability: none needed: the core owns the input buffer and the client feeds it)

## kbdel(position)

**What it is for:** deleting from the cursor to a position: the primitive behind "delete word" and "kill to end of line" bindings.

**Syntax:** `kbdel(i)` deletes the text between the cursor and position `i` (in either direction) and returns the new cursor position.

```tf
/input look at the dragon
/test kbdel(kbwordleft())
/echo [$[kbhead()]] cursor $[kbpoint()]
```
```output
[look at the ] cursor 12
```

The stock `^W` binding is built from exactly this pair.

**Gotchas:** A position outside the buffer deletes nothing.

**Status:** see [Status](Status.md) (host capability: none needed: the core owns the input buffer and the client feeds it)

## kbmatch([position])

**What it is for:** parenthesis matching: finds the partner of a bracket in the input line.

**Syntax:** `kbmatch()` or `kbmatch(i)`: looks from `i` (default: the cursor) rightwards for one of `(){}[]` and returns the position of its mate, or `-1` if there is none.

```tf
/input say (hi there) now
/echo $[kbmatch(4)] $[kbmatch(14)] $[kbmatch(0)] $[kbmatch(17)]
```
```output
13 -1 13 -1
```

**Status:** see [Status](Status.md) (host capability: none needed: the core owns the input buffer and the client feeds it)

## kbwordleft([position]) and kbwordright([position])

**What they are for:** word-wise cursor movement. Words are runs of letters, digits and the characters in `%wordpunct`.

**Syntax:** `kbwordleft(i)` is the position of the start of the word at or left of `i`; `kbwordright(i)` the position just past the end of the word right of `i`. `i` defaults to the cursor.

```tf
/input one two  three
/test kbgoto(6)
/echo $[kbwordleft()] $[kbwordright()] $[kbwordleft(0)] $[kbwordright(13)]
```
```output
4 7 0 14
```

Bound to keys with `/def -b'^[b' = /test kbgoto(kbwordleft())` and `/def -b'^[f' = /test kbgoto(kbwordright())`.

**Status:** see [Status](Status.md) (host capability: none needed: the core owns the input buffer and the client feeds it)

## kbcharback(n) and kbcharfwd(n)

**What they are for:** character-wise cursor arithmetic that respects multibyte (UTF-8) characters.

**Syntax:** `kbcharback(n)` returns the byte position `n` characters before the cursor (never below 0); `kbcharfwd(n)` the position `n` characters after (never beyond the end); the argument is required. They only *compute* a position; combine with `kbgoto()` to move.

```tf
/input héllo
/echo $[kblen()] $[kbcharback(1)] $[kbcharback(2)]
/test kbgoto(1)
/echo $[kbcharfwd(1)] $[kbcharfwd(9)]
```
```output
6 5 4
3 6
```

The input is 6 bytes (`é` takes two), so one character back from the end is byte 5 and two back is byte 4 (the `é` is not split). `kbcharfwd` never goes past the end. Both functions require their argument in this engine as in tf's function table, even though the manual shows it optional.

**Status:** see [Status](Status.md) (host capability: none needed: the core owns the input buffer and the client feeds it)

## morepaused([world])

**What it is for:** knowing whether the pager is holding back output (the "More" prompt).

**Syntax:** `morepaused()` or `morepaused(world)`; returns `1` if that world's output is paused by `/more` or by `/dokey pause`, else `0`.

```tf
/echo paused: $[morepaused()]
```
```output
paused: 0
```

**Status:** see [Status](Status.md) (host capability: MoreHost, optional: the core keeps per-world counters)

## moresize([flags [, world]])

**What it is for:** how many lines are queued behind the pager.

**Syntax:** `moresize()` returns the physical lines (after wrapping) below the bottom of the output window. Flags: `n` count only lines never seen, `l` count only lines matching the current `/limit`, `a` also count lines with the "A" (noactivity) attribute. Flags may be combined (`"nl"`). An unknown flag prints `moresize: illegal flag 'x'`.

```tf
/echo $[moresize()] queued, $[moresize("n")] never seen, $[moresize("nl")] unseen and matching /limit
```
```output
0 queued, 0 never seen, 0 unseen and matching /limit
```

**Gotchas:** `0` does not mean output is not paused: use `morepaused()` for that.

**Status:** see [Status](Status.md) (host capability: MoreHost, optional: the core keeps per-world counters)

## morescroll(n)

**What it is for:** scrolling the output window from a macro, as the pager keys do.

**Syntax:** `morescroll(n)`: if `n` is positive, scroll `n` lines forward (revealing queued text); if negative, scroll `abs(n)` lines back into scrollback. Returns the number of lines actually scrolled (`0` when there is nothing to scroll).

```tf
/echo scrolled: $[morescroll(-(winlines() - 1))]
```
```output
scrolled: 0
```

A page-up key binding is `/def -b'^[v' = /test morescroll(-(winlines() - 1))`.

**Status:** see [Status](Status.md) (host capability: MoreHost, optional: the core keeps per-world counters)

## status_fields([row])

**What it is for:** reading the layout of the status bar so a script can check or save it.

**Syntax:** `status_fields()` or `status_fields(row)` returns the space-separated field list of that status row (row 0 by default), in `/status_add` syntax: `name[:width[:attributes]]`.

```tf
/echo $[status_fields(0)]
/echo [$[status_fields(1)]]
```
```output
@more:8:rB :1 @world :1 @read:6 :1 @active:11 :1 @log:5 :1 @mail:6 :1 insert:6 :1 kbnum:4 :1 @clock:5
[]
```
(the default row of real tf; row 1 is empty until you `/status_add -r1`)

**Gotchas:** A row that does not exist yields an empty string.

**Status:** see [Status](Status.md) (host capability: StatusHost, optional: the core owns the layout)

## status_width(fieldname)

**What it is for:** the current width, in columns, of a named status field, when you want to line another display up with it.

**Syntax:** `status_width(name)` returns an integer. It takes exactly one argument; calling it with none is a usage error.

```tf
/echo clock field is $[status_width("@clock")] columns wide
```
```output
clock field is 5 columns wide
```

**Gotchas:** A name that is not a field of the status bar warns and returns `0` here; the reference tf build dereferences a null pointer and aborts, so check with `status_fields()` first if you also run on real tf.

**Status:** see [Status](Status.md) (host capability: StatusHost, optional: the core owns the layout)

## isatty()

**What it is for:** telling an interactive session apart from a batch run (`tf < script`, a bot, a test rig).

**Syntax:** `isatty()` returns `1` when tf is running on a terminal, `0` otherwise.

```tf-noverify
/if (isatty()) /echo interactive%; /else /echo batch%; /endif
```

**Status:** see [Status](Status.md) (host capability: TermHost)

## nmail()

**What it is for:** the number of watched mail files with unread mail, the number behind the `(Mail)` status indicator.

**Syntax:** `nmail()` returns an integer. Which files are watched is set by `%MAIL` / `%MAILPATH`.

```tf-noverify
/echo unread mailboxes: $[nmail()]
```
```output
unread mailboxes: 0
```

**Status:** see [Status](Status.md) (host capability: Notifier)

## read()

**What it is for:** obsolete: reads a line from the keyboard. Use [`tfread()`](Functions-Files-and-Streams.md#tfreadhandle-variable).

**Syntax:** `read()` returns the line as a string. Real tf prints `read: Warning: read() is deprecated. Use tfread() instead.` and, like `tfread()`, fails with `keyboard can only be read from a command line command` when called from a trigger or macro running in the background.

**Status:** see [Status](Status.md) (host capability: Notifier)

## Related

- Commands: [Commands-History-and-Display](Commands-History-and-Display.md) (`/more`, `/status_add`, `/limit`), [Commands-Output-and-Input](Commands-Output-and-Input.md) (`/input`, `/dokey`).
