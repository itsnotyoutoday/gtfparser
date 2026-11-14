# Commands: History and Display

These commands look after the text you have already seen (history, scrollback,
filtering) and the way it is presented (the More pause, the status bar, line
wrapping, visual mode). Most of them mean something only to a front end that
owns a screen, so the engine leaves the on-screen part to the embedding
program and keeps the state and the script-visible results. "Library" entries
are macros defined by tf's own library (`stdlib.tf`, `tfstatus.tf`,
`savehist.tf`), not C builtins; they work wherever the library is loaded.

Examples are checked with `python3 scripts/doccheck.py --lib $TF_LIB`. Messages
that tf prints with a leading `%` go to the message stream, not to the echo
output shown in the `output` blocks. For what this build of the engine
supports, see [Status](Status.md).

Contents: [/recall](#recall-options-range-pattern) -
[/recordline](#recordline-options-text) - [/histsize](#histsize-options-size) -
[/limit](#limit-options-pattern) - [/relimit](#relimit) - [/unlimit](#unlimit) -
[/status_add](#status_add-options-fields) - [/status_rm](#status_rm--rn-name) -
[/status_edit](#status_edit--rn-field) - [/status_clear](#status_clear--rn) -
[Library: /more](#more-offon) - [/visual](#visual-offon) - [/wrap](#wrap-offonwidth) -
[option shortcuts](#option-shortcut-macros) - [status library](#status-bar-library-tfstatustf) -
[history library](#history-library-loadhist-and-savehisttf)

---

## /recall [options] [#]range [pattern]

**What it is for:** printing lines you have already received (or typed) from a
history buffer, optionally filtered by pattern, time or position, e.g. "what did
Kite whisper in the last hour?".

**Syntax:** `/recall [-w[world] | -l | -g | -i] [-t[format]] [-v] [-q] [-a<attrs>] [-m<style>] [-A<n>] [-B<n>] [-C<n>] [#]range [pattern]`

**Options.** Only one history may be chosen. `-w` (the default) is the current
world's history; `-wNAME` is that world's; `-l` is *local* history (everything tf
itself printed); `-g` is the global history (all worlds plus local); `-i` is
your typed-input history. `-t` prefixes each line with a timestamp (the format
is an `ftime()` format; bare `-t` uses `%time_format` in brackets). `-v`
inverts the pattern, `-q` drops the header and footer lines, `-a<attrs>`
suppresses lines carrying given attributes (`-ag` shows gagged lines too), `-m`
picks `simple`, `glob` or `regexp` matching (default `%matching`). `-A`, `-B`,
`-C` add lines of context after, before or around each match; separate groups
are divided by `--`. A `#` just before the range numbers the lines.

**Range** (`x` and `y` are line numbers or counts, or times such as `1:00`,
`11:00-13:00`, or absolute epoch seconds):

| range | meaning |
|---|---|
| `x` | the last `x` lines, or lines within the last time period `x` |
| `/x` | the last `x` lines that *match* the pattern |
| `x-y` | line `x` through line `y` |
| `x-` | everything after `x` |
| `-y` | the `y`th previous line (write `- -4`, so `-4` is not read as an option) |

**Return value:** the number of lines recalled.

```tf
/recordline -l first line
/recordline -l second line with spam
/recordline -l third
/recordline -l fourth spam
/recall -l 3
/recall -l -q /2 *spam*
```
```output
================ Recall start ================
second line with spam
third
fourth spam
================= Recall end =================
second line with spam
fourth spam
```

**Gotchas.**
- The header and footer appear only when output goes to the screen; inside
  `$(/recall ...)` or a `/quote` pipe they are omitted.
- Local history also holds tf's own startup text and the output of earlier
  `/recall`s, so line numbers in `-l` start higher than you might expect and
  the history grows by what you recall.
- Timestamps of lines that arrived while tf was suspended or in `/sh` are the
  time tf resumed, not the arrival time.
- The `#` (line numbers) must come last, after the other options: `/recall -l -q # 1-2`.

**Status:** see [Status](Status.md). No host capability needed: the core keeps the history buffers (a front end feeds lines in with `RecordInput` / `RecordLocalLine` / `RecordWorldLine`).

---

## /recordline [options] text

**What it is for:** inserting a line into a history buffer without printing it
or logging it, e.g. to replay a saved log into scrollback or input history.

**Syntax:** `/recordline [-l | -i | -g] [-w[world]] [-t<time>] [-a<attrs>] [-p] text`

**Options.** `-g` (default) records to global history; `-l` local; `-i` input
history; `-w`/`-wNAME` the current or named world's history. `-ttime` stamps
the line with a system time instead of now (`/recall -t` shows it). `-aattrs`
gives the line attributes; `-p` interprets `@{attr}` codes in `text` (`@@` is a
literal `@`).

**Gotchas.** The text is neither echoed nor logged. It always appends to the
*end* of a history, so a `-t` time earlier than the last line produces an
out-of-order buffer and odd `/recall` results. Combined with `/quote` it reads a
file back: `/quote -S -dexec /recordline -i - 'input.log` restores a
previous session's input history (see [`/loadhist`](#history-library-loadhist-and-savehisttf)).

```tf
/recordline -i /connect Cave
/recall -i -q 2
```
```output
/connect Cave
```
(The input history always ends with a blank slot for the line being typed,
so `/recall -i -q 1` alone prints that blank line.)

**Status:** see [Status](Status.md). No host capability needed.

---

## /histsize [options] [size]

**What it is for:** asking for, or changing, how many lines a history keeps.

**Syntax:** `/histsize [-l | -i | -g] [-w[world]] [size]`

**Options.** `-g` global (the default), `-l` local, `-i` input, `-wNAME` a
world. Without `size` it reports the capacity; with `size` it sets it,
discarding the oldest lines at once if the history shrinks (growing discards
nothing). The return value is 0 on failure and otherwise the (new) size.

Defaults: 1000 lines per world and for global, 100 for local and input. The
variable `%histsize` sets the default for world histories created later.

```tf
/histsize -i 3
/recordline -i one
/recordline -i two
/recordline -i three
/recordline -i four
/eval /echo [$(/recall -i -q 9)]
```
```output
% input history capacity changed to 3 lines.
[three four ]
```
Only the newest lines survive (the capacity counts the blank slot for the line
being typed). `/histsize -l` alone prints `% local history capacity is 100
lines.`; with a number it prints `% local history capacity changed to 50 lines.`.

**Gotchas.** Real tf rejects `-q` as an invalid option here (options are
`-lgi -w<string>`).

**Status:** see [Status](Status.md). No host capability needed.

---

## /limit [options] [pattern]

**What it is for:** hiding everything in the scrollback window except lines that
match a pattern, so you can scroll through just the interesting lines; the
limit affects the screen only, never the history.

**Syntax:** `/limit [-v] [-a] [-m<style>] [pattern]`

**Options.** `-v` shows only lines that do *not* match; `-a` shows only lines
that have attributes; `-m` picks the matching style (default `%matching`).
With no options and no pattern, `/limit` just returns 1 if a limit is in
effect and 0 if not.

**Behavior.** While a limit is on, scrolling to any point (the bottom
included) gives a More prompt counting the lines below; `/unlimit` from a
scrolled position leaves the bottom visible line where it is and redraws the
unlimited lines above it, so you can find a line and then see its context.
Scrolling past the bottom changes the prompt to `LIMIT ON`. By default the
`@more` status field ignores lines hidden by the limit. The default keybinding
`^[L` toggles the last limit.

```tf
/limit *spam*
/echo limit in effect: $[limit()]
/unlimit
```
```output
limit in effect: 0
```

**Gotchas.** `/limit -q` is an invalid option in real tf (options are `-av`
and `-m`).

**Status:** see [Status](Status.md). Capability: LimitHost (the host re-renders
its screen under the filter; without one every `/limit` finds no lines, as tf
in batch mode does and prints `% No lines matched criteria.`). A builtin command
may be called as a function in real tf, so `limit()` (as in the default `@more`
status definition) is the same test.

---

## /relimit

**What it is for:** re-applying the most recent `/limit` (after `/unlimit`, or
from a key binding).

**Syntax:** `/relimit`

With nothing to repeat, real tf prints `% No previous limit.`

**Status:** see [Status](Status.md). Capability: LimitHost.

---

## /unlimit

**What it is for:** turning the current `/limit` off so every line is shown again.

**Syntax:** `/unlimit`

See `/limit` for how the window is repositioned.

**Status:** see [Status](Status.md). Capability: LimitHost.

---

## /status_add [options] name[:width[:attrs]] ...

**What it is for:** adding fields to the status bar (the line between output and
input in visual mode), for example an `hp` field showing a variable.

**Syntax:** `/status_add [-r<n>] [-A[field] | -B[field]] [-s<n>] [-x] [-c] name[:width[:attrs]] ...`

**Options.** `-rN` row (default 0). `-A` adds after all fields, `-Afield` after
`field`; `-B`/`-Bfield` before; `-A` is assumed. `-sN` puts N spaces of padding
between the new field and its neighbor (default 1). `-x` skips a field that
already exists; `-c` clears the row first.

**Fields.** A name starting with `@` is an internal state (`@more`, `@world`,
`@read`, `@active`, `@log`, `@mail`, `@clock`); a plain identifier shows the
variable of that name and updates when it changes; a quoted name is literal
text; an unnamed field (`:1`) is padding. A width truncates; a negative width
right-justifies; one field may have width 0 to take up the slack. Attributes
combine with `%status_attr_int_<name>` / `%status_attr_var_<name>`; padding is
drawn with `%status_pad`.

```tf
/set status_height=2
/status_add -r1 foo:5 bar:3
/status_add -r1 -Afoo hp:4
/echo $[status_fields(1)]
/status_edit -r1 hp:6
/echo $[status_fields(1)]
/status_rm hp
/echo $[status_fields(1)]
/status_clear -r1
/echo [$[status_fields(1)]]
```
```output
foo:5 :1 hp:4 bar:3
foo:5 :1 hp:6 bar:3
foo:5 :1 bar:3
[]
```
Several fields can be given at once, but padding is not inserted for you
(`/status_add -Aclock foo:4 :1 bar:4`); adding a field next to others inserts a
one-column `:1` spacer, and `/status_rm` leaves it behind. In the default
layout (library `tfstatus.tf`) use `-A@world` and friends to place a field
after a stock one. Only one variable-width field (no `:n`) may exist per row.

**Gotchas.** Row 0's list is also reachable through the deprecated variable
`%status_fields`; prefer these commands.

**Status:** see [Status](Status.md). No host capability needed to keep the fields; a front end draws the bar from `StatusRows` and redraws on `status` Notifier events (optionally `StatusHost` supplies field values).

---

## /status_rm [-r<n>] name

**What it is for:** removing the first status field called `name` (searching
all rows unless `-r` limits it), together with the smaller neighboring pad
field if padding sits on both sides.

**Syntax:** `/status_rm [-rN] name` (e.g. `/status_rm @mail`).

Real tf says `STATUS_RM: hp: no such field` when it is absent.

**Status:** see [Status](Status.md). No host capability needed to keep the fields; a front end draws the bar from `StatusRows` and redraws on `status` Notifier events (optionally `StatusHost` supplies field values).

---

## /status_edit [-r<n>] field

**What it is for:** replacing an existing field's width or attributes in place
(neighboring padding is untouched), e.g. `/status_edit @log:1`.

**Syntax:** `/status_edit [-rN] name[:width[:attrs]]`. Only the first matching
field is edited; `-r` restricts the search to one row.

**Status:** see [Status](Status.md). No host capability needed to keep the fields; a front end draws the bar from `StatusRows` and redraws on `status` Notifier events (optionally `StatusHost` supplies field values).

---

## /status_clear -r<n>

**What it is for:** emptying one status row entirely.

**Syntax:** `/status_clear -rN`. Real tf requires `-r`; without it the command
returns 0 and does nothing. Returns 1 on success.

**Status:** see [Status](Status.md). No host capability needed to keep the fields; a front end draws the bar from `StatusRows` and redraws on `status` Notifier events (optionally `StatusHost` supplies field values).

---

## Library: /more [off|on]

**What it is for:** turning More paging on or off: with `more` on, output stops
at a "More" prompt whenever the window fills, until you page on.

**Syntax:** `/more [off|on|0|1]` (it is `/set more`). Independently of the setting, the
`/dokey pause` key (^S) pauses immediately whatever the setting.

```tf
/set more=off
/more on
/echo more is %more
/more off
/echo more is %more
```
```output
% "More" paging enabled.  Use TAB to scroll.
more is on
more is off
```

**Gotchas.** The first time you turn paging on while TAB is still bound to the
default page macro, the library also echoes the reminder line shown above (as an error-class `/echo -e` message, so a host may route it to its message area).
A bare `/more` with no argument is an error in real tf (`Invalid more value ""`),
because it expands to `/set more` with an empty value; use `/set more` to look
at the value. Each socket has its own window and More state.

**Status:** library macro; see [Status](Status.md) for the underlying display
support.

---

## Library: /visual [off|on]

**What it is for:** switching visual mode (separate output window, status bar and
input line) on or off.

**Syntax:** `/visual [on|off]` (it is `/set visual`).

```tf
/set visual=off
/visual on
/echo visual is %visual
```
```output
visual is off
```

**Gotchas.** Like real tf, `/visual on` is refused when the terminal cannot do
visual mode (for example when not on a tty): `%visual` stays off, as the example
shows. A host with a capable terminal enables it by declaring the capability.

**Status:** library macro. Capability: TermHost for any visible effect.

---

## Library: /wrap [off|on|width]

**What it is for:** word-wrapping long lines at a width; a number sets
`%wrapsize` and turns wrapping on.

**Syntax:** `/wrap [on|off|<width>]`. `/nowrap` is `/set wrap off`.

```tf
/wrap 40
/echo wrap=%wrap size=%wrapsize
/nowrap
/echo wrap=%wrap
```
```output
wrap=on size=40
wrap=off
```

**Gotchas.** Like `/more`, a bare `/wrap` is an error in real tf.

**Status:** library macro.

---

## Option shortcut macros

The library defines a family of one-word macros that are just `/set` with the
option name pre-filled; each takes the same argument as the variable:
`/insert`, `/isize`, `/wrapspace`, `/ptime`, `/gpri`, `/hpri`, `/background`,
`/bamf`, `/borg`, `/clearfull`, `/cleardone`, `/login`, `/lp`, `/lpquote`,
`/quiet`, `/quitdone`, `/redef`, `/shpause`, `/sockmload`, `/sub`, `/reply`
(sets `borg`), `/nologin`, `/noquiet`, `/nowrap`. Display ones among them:
`/insert off` makes typing overstrike (the `(Over)` status field),
`/isize` sets the input window height, `/wrapspace` the continuation indent.

```tf
/insert off
/isize 3
/wrapspace 4
/quiet on
/echo insert=%insert isize=%isize wrapspace=%wrapspace quiet=%quiet
```
```output
insert=off isize=3 wrapspace=4 quiet=on
```

**Status:** library macros.

---

## Status bar library (tfstatus.tf)

`tfstatus.tf` (loaded by the standard `tfrc`, and by `/require tfstatus.tf`)
adds these macros on top of `/status_add` and friends. Each needs the status
builtins above to do anything visible.

- `/status_defaults` - restore the default fields for every row and the
  `%status_int_*` / `%status_var_*` formats.
- `/status_save name` / `/status_restore name` - remember and bring back row 0's
  field list under a variable-safe label (kept until tf exits).
- `/clock [off|on|format]` - add, remove or reformat the `@clock` field; the
  default format is `%H:%M`, widths are computed from the format.
- `status_int_more()` - the macro behind the `@more` field text (`LIMIT ON`,
  `More 12`, `L 15+  3`).

```tf
/require tfstatus.tf
/status_save mine
/clock %I:%M
/echo clock=$[status_fields(0)]
/status_restore mine
/echo restored=$[status_fields(0)]
```
```output
clock=@more:8:rB :1 @world :1 @read:6 :1 @active:11 :1 @log:5 :1 @mail:6 :1 insert:6 :1 kbnum:4 :1 @clock:1
restored=@more:8:rB :1 @world :1 @read:6 :1 @active:11 :1 @log:5 :1 @mail:6 :1 insert:6 :1 kbnum:4 :1 @clock:5
```

**Status:** library macros; see [Status](Status.md) for `/status_*`.

---

## History library: /loadhist and savehist.tf

- `/loadhist [-lig] [-w<world>] file` (stdlib) reads a log file back into the
  chosen history: it runs `/recordline` on each line.
- `savehist.tf` (`/require savehist.tf`) persists histories between sessions
  under `%savehist_dir` (default `~/tf-hist`): `/save_history {-l|-i|-g|-w<world>}`,
  `/load_history` with the same arguments, and `/save_histories` /
  `/load_histories` for all of them (call them from `tfrc` and on `/quit`).
  It depends on `/recall`, `/histsize`, `/quote`, `tfopen()` and `/sh`-style
  shell access (`/restrict` below `file` is required to leave them working).

```tf-noverify
/require savehist.tf
/load_histories
/def -hQUIT save_on_quit = /save_histories
```

**Status:** library macros; they inherit the status of the builtins they call.
