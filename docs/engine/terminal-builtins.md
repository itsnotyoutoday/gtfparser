# Keyboard, status line and terminal builtins

Everything about the keyboard, the status line, paging and prompts that can be
computed without a terminal lives in `gtfparser` as per-`Interp` state. A
client (a terminal UI, a web front end, squatterd) feeds that state and draws
from it. Behaviour that really needs a client goes through `Notifier` events or
small optional host interfaces; a builtin that needs a capability the host lacks
prints `% /name: not supported by this client` and fails (it never pretends to
work).

Source files: `inputbuf.go` (input buffer, input history, `kb*()`, `/input`),
`keys.go` (key sequences, bindings, `KeyPressed`, `/dokey`, `keycode()`),
`status.go` (status fields, `/status_*`, rendering), `terminal.go` (shared
state, variable-change hooks, `/beep`, `/localecho`, `/suspend`, paging,
`prompt()`, `fake_recv()`, optional host interfaces). Tests:
`terminal_test.go`, `terminal_screen_test.go`, and the differential corpora
`testdata/cases/{kb,status,term}.tf`.

All of it is single-owner like the rest of `Interp`: call it from the goroutine
that owns the interpreter (with a `Runner`: `r.Do(func(in *gtfparser.Interp){ in.KeyPressed(s) })`).

## Where the state lives

`Interp` is declared in `interp.go`, so the terminal state hangs off a
weak-pointer registry (`ts()` in `terminal.go`): one `termState` per `Interp`,
dropped automatically when the `Interp` is garbage collected, or explicitly with
`in.ReleaseTerminal()`. No terminal state is shared between interpreters.
Closures a client stores in that state (`OnVarChange`, `InputBuffer.OnChange`)
must not capture the `Interp` strongly if you rely on the automatic release.
An integrator who prefers a plain field can replace `ts()` with
`return in.term` (see `terminal-hooks.md`).

## Input buffer

```go
b := in.Input()          // *InputBuffer: the line being edited
b.Text(); b.Cursor(); b.Len()   // text, byte offset of the cursor, length
b.Set(text, cursor)      // replace (cursor < 0: at the end); b.Clear()
b.OnChange = func() {}   // called after every change: redraw the input line
in.TypeText(s)           // type s as the keyboard would: %insert (insert/overwrite),
                         // %kbnum repeat count, input-history sync
```

Positions are **byte offsets** (as in tf); `kbcharback/kbcharfwd` and the editing
operations step by UTF-8 character; overwrite mode replaces whole characters.

Script side: `kbhead() kbtail() kbpoint() kbgoto(i) kbdel(i) kblen() kbmatch([i])
kbwordleft([i]) kbwordright([i]) kbcharback(n) kbcharfwd(n)` and `/input text`.
They check argument counts like tf (`kbcharback()` with no argument is an error;
the docs say otherwise but tf's function table says exactly one). Movement that
does not move, and deletions that delete nothing, ring the bell (`beep` event,
see below). Word boundaries follow `%wordpunct`.

### Input history

`in.SubmitInput()` is Enter: it handles `^old^new` substitution (`% No match.`
on failure), `%kecho` (shown with `%kprefix`), records the line in the input
history (consecutive duplicates collapse; `%histsize` entries) and executes it
(`Exec` when `%sub` is `off`, else `ExecFull`). `in.InputHistory()` lists the
lines. `/dokey recallb|recallf|recallbeg|recallend|searchb|searchf` navigate it;
failing to move beeps. `PendingInput()/FlushInput()`: a `/dokey newline` issued
inside a macro only queues the line (as tf does); `KeyPressed` runs it when it
returns, a script top level runs it at once, other callers use `FlushInput`.

## Keys

```go
in.KeyPressed(bytes)   // raw terminal bytes, any amount, any split
in.FlushKeys()         // give up on a held-back prefix (e.g. a lone ESC)
in.PendingKeys()       // the held-back prefix
in.Bindings()          // []Binding{Seq, Macro}: what is bound to what
in.FindKey(seq)
in.KeyCode("Up")       // (code, known): keycode()
PrintToAscii("^[[A") / AsciiToPrint(...)   // /bind notation <-> bytes
in.LastKeyTime()       // for idle()
```

`KeyPressed` follows tf's `handle_keyboard_input`:

* The longest bound sequence runs (`/bind`, `/def -b`; `/def -B name` is bound
  to whatever `KeyHost.KeyCode(name)` returns). Notation: `^X`, `^[`, `^?`,
  `\033`, `\0x1b`, `\27`. When one macro is defined twice for a sequence the
  newest wins.
* Bytes that start a longer binding are held until it completes (or fails).
  tf refuses to `/bind` a sequence that is a prefix of another; gtf's `/bind`
  does not yet, so with such an ambiguous pair the longer one wins and the
  shorter one fires only via `FlushKeys`.
* No binding: `\r`/`\n` submit the line, `\b`/DEL erase (honouring `%kbnum`), a
  digit extends `%kbnum` when a prefix is being entered (capped by
  `%max_kbnum`), anything else is typed.
* `/dokey lnext` makes the next byte literal.
* A bound macro sees the numeric prefix as the **local** `%kbnum` and the global
  `%kbnum` is reset, as in tf. The statement budget (`max_instr`) restarts per
  call. A submitted line runs before later bytes in the same call are read.

The three keys tf binds before any library loads (`^W`, `^R`, `^V`) are already
defined by `New`.

### /dokey

`/dokey NAME` (case-insensitive) with every name of the documentation plus the
library's (`DokeyNames()`):

| name(s) | implementation |
|---|---|
| BSPC BWORD DCH DEOL DLINE DWORD HOME END LEFT RIGHT WLEFT WRIGHT UP DOWN | core, on the input buffer; return the new cursor. A user macro `/dokey_<name>` (e.g. from `kbfunc.tf`) takes precedence, as in tf. UP/DOWN move by `%wrapsize` exactly like tf's macros. `%kbnum` counts are honoured. |
| RECALLB RECALLF RECALLBEG RECALLEND SEARCHB SEARCHF | core input history |
| NEWLINE | queues/executes the line (above) |
| LNEXT | core flag + `dokey` event |
| PAUSE | core: pauses the display's `MoreState`, runs the `MORE` hook, `dokey` event; returns 0 if already paused |
| FLUSH | core: clears the more counters/pause, `dokey` event |
| PAGE PAGEBACK HPAGE HPAGEBACK LINE LINEBACK PGUP PGDN | `morescroll()` by `winlines()` (and halves, lines) times the count; PGUP/PGDN are page back/forward as in the library, not half pages as the doc table says |
| SOCKETB SOCKETF | `/fg -c<-n|n>` (needs the world engineer's `/fg`; fails loudly otherwise) |
| REDRAW CLEAR REFRESH SELFLUSH | `Notifier` event `dokey` with the upper-case name; without a `Notifier` the command fails loudly. REFRESH returns the cursor. |

## Status line

Definitions live in core; the client asks for rendered rows.

```go
rows := in.StatusRows(width)   // one StatusRow per %status_height
rows[i].Text                   // the whole row, width columns, ready to print
rows[i].Attrs                  // %status_attr (applies to the whole row)
rows[i].Spans                  // []StatusSpan{Col, Width, Text, ValueStart, ValueLen, Attrs, Field}
in.StatusFields(row)           // []StatusField definitions
```

Redraw when you receive a `status` event (field definitions or a format/attr
variable changed, `@more` counters changed), and every minute for `@clock`.
`ValueStart/ValueLen` say which runes of a span are the value (field attributes
apply to those only; padding gets only `%status_attr`).

Field values: an internal field (`@world @more @read @active @log @mail @clock`)
asks the optional `StatusHost.StatusValue(name)` first; if it declines the
`%status_int_<name>` expression is evaluated, as in tf. A variable field uses
`%status_var_<name>` if set, else the variable's value. Expressions are
evaluated with output disabled; a problem (bad expression, a function the
client lacks) is reported once, not on every redraw (it is reported again after
the status variables change).

Commands and functions (verified against real tf, `testdata/cases/status.tf`):
`/status_add [-A[field]|-B[field]] [-sN] [-rN] [-x] [-c] fields...`,
`/status_edit`, `/status_rm`, `/status_clear -rN`, `status_fields([row])`,
`status_width(field)`. The default layout, `/status_defaults`, `/status_save`,
`/status_restore` come from the library (`tfstatus.tf`) and work unchanged.
Up to 6 rows (`%status_height` > 6 is refused). The deprecated `%status_fields`
variable is kept in sync with row 0, and assigning to it redefines row 0
(`%warn_status` controls the deprecation warning). Attribute strings are
canonicalised exactly as tf prints them (`Br` prints as `rB`, `Bu` as `uB`,
colors by name, e.g. `C1` as `Cred`).

Differences: `status_width()` of a field that does not exist warns and returns
0 (real tf dereferences NULL and crashes).

## Paging ("More")

Core state per world, fed by the client:

```go
in.SetMoreState(world, MoreState{Paused, Back, New, BackFiltered, NewFiltered, Above, Active})
in.MoreState(world)
```

`morepaused([w])`, `moresize([flags[, w]])` (flags `n l a`), `nactive([w])`,
`morescroll(n)` and `/dokey page...` answer from it (`morescroll` moves lines
between `Above`, the window and `Back`, unpauses at the bottom, and emits a
`more` event). `winlines()` is `lines - 1`, or in visual mode
`lines - %isize - %status_height`. A client that owns the output window can take
over completely by implementing `MoreHost` (`MorePaused`, `MoreSize`,
`MoreScroll`) and/or `WindowHost` (`WinLines`); they then override the core
counters. `/set more=off` releases pauses. `in.FitScreen()` applies tf's
setup_screen fix-ups (refuse visual mode on a tiny screen, shrink `%isize`/
`%status_height` to leave 2 output lines); it runs when `visual`, `isize` or
`status_height` change.

`in.TermSettings()` returns the display variables as plain values.

## Prompts and fake_recv

`prompt(text)` sets the current (foreground) world's prompt and runs processes
waiting for a prompt; it returns 0 when there is no current world (tf: no
socket). Read it with `in.PromptText(world)`; a `PromptHost` is told as well, and
a `prompt` event is raised. `fake_recv(line[, world[, "r"]])` injects a line as
if the world had sent it: it needs an open world (`WorldHost.Connected`), runs
`ProcessLine` (triggers, gags) and, if not gagged, shows it through
`LineHost.DisplayLine(world, line)` (falling back to `Host.Echo`). The `r` flag
treats the text as a byte stream: lines end at `\n`; an unterminated remainder
fires the `PROMPT` hook and becomes the prompt. Programs can call
`in.FakeRecv(world, text, raw)` directly.

## beep, localecho, suspend

* `/beep [n|on|off]`: `n` (default 3) beeps become a `beep` event (`Args[0]` =
  count) unless `%beep` is off; `on`/`off` set `%beep`. Negative counts fail.
  Bells from the editing functions use the same path.
* `/localecho [on|off]`: core keeps per-world state (`in.LocalEcho(world)`,
  default on). No argument returns it; `on`/`off` update it and raise
  `localecho` (`world`, `on|off`) so the client can negotiate TELNET ECHO; the
  client reports the outcome with `in.SetLocalEcho(world, bool)`.
* `/suspend`: raises `suspend`.

Without a `Notifier` all three fail loudly.

## Events (Notifier)

| Kind | Args | When |
|---|---|---|
| `beep` | count | `/beep`, editing that cannot move |
| `localecho` | world, on/off | `/localecho on|off` |
| `suspend` | | `/suspend` |
| `dokey` | NAME | REDRAW CLEAR REFRESH SELFLUSH LNEXT PAUSE FLUSH |
| `status` | `fields,row` / var name / `@more` | redraw the status area |
| `more` | `scroll,n` / `off` | paging state changed |
| `prompt` | world, text | prompt changed |
| `var` | name, old, new | a watched variable changed (see `terminal-hooks.md`) |

Optional host interfaces (all in `terminal.go`, besides the existing
`Notifier`, `KeyHost`, `TermHost`, `WorldHost`): `MoreHost`, `WindowHost`,
`LineHost`, `PromptHost`, `StatusHost`.

## Variable-change hook

`in.OnVarChange(func(name, old, new string))`, `in.WatchVar(name)`,
`in.PollVars()`. Without the one-line `setGlobal` patch in
`terminal-hooks.md` changes are detected at the next terminal entry point
(`KeyPressed`, `StatusRows`, the `kb*()` functions, `/dokey`, ...) or `PollVars`.

## Not matched / not done

* `/bind` does not refuse conflicting prefixes (macro.go).
* `idle()`, `sidle()`, `nread()`, `nlog()`, `nmail()` are not implemented here
  (they belong to sockets, `read()`, logging and mail); `LastKeyTime()` is the
  keyboard half of `idle()`. Until they exist the default `@read @log @mail`
  fields warn once that the function is unsupported (or a `StatusHost` supplies
  the values).
* Real tf's `kbcharback()`-style builtin commands called with function syntax
  (`status_add("...")`) depend on the expression engine, not on these files.
* `/clock` (library macro) currently fails in the interpreter with
  `/endif without matching /if or /while`; it is not a status-line problem.
* No real screen exists in core: the `more` counters are what the client
  reports, so `moresize()` etc. report zeros until it does.
