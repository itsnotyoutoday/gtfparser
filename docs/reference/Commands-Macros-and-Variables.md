# Commands: Macros, Triggers, Hooks and Variables

Everything a script author uses to *define behaviour* and to *keep state*:
`/def` and its family (triggers, hooks, gags, hilites, key bindings are all just
macros with different options), the commands that list and remove macros, and
the variable commands (`/set`, `/let`, `/setenv`...). Entries marked
**Library** are macros in tf's `stdlib.tf`, not C builtins; most are thin
wrappers around `/def`, `/list` and `/purge`.

How to read the examples. They run with the standard library loaded and with
full substitution of each line, as if `/set sub=full`; in real tf a typed line is
only expanded in that mode, so inside a real macro or `/eval` the examples
behave the same. A "trigger" is fed a line of game text with `/trigger`, which
is how scripts are tested without a connection.

Words in a trigger's text become positional parameters: for the line
`Bob tells you: hi` the macro sees `%1`=`Bob`, `%2`=`tells`, `%{-2}`=`you: hi`
(everything from word 3 on), `%*`=the whole line, `%#`=4. Regexp triggers also
set `%P0`, `%P1`... (whole match, groups), `%PL` and `%PR` (text left and right of the match).

## /def [options] [name] [= body]

**What it is for:** creating (or replacing) a macro: a named command, a trigger
that reacts to a line from the game, a hook that reacts to a client event, a
key binding, or any combination.

**Syntax:** `/def [options] [name] [= body]`

**Options**

| Option | Meaning |
|---|---|
| `-t'pattern'` | trigger: run when a line from a world matches |
| `-mstyle` | pattern style for `-t`, `-h`, `-T`: `simple`, `glob` (default, `%{matching}`) or `regexp` |
| `-h'EVENT [pattern]'` | hook: run when client event EVENT (optionally with matching arguments) happens; `A\|B` lists events |
| `-b'keys'` | key binding (`^X` forms; see `/bind`); `-Bname` binds a termcap key name (deprecated) |
| `-ppri` | priority of the trigger/hook (default 1; a number or an expression, evaluated once) |
| `-F` | fall-thru: lower-priority matches may also run |
| `-nN` | shots: delete the macro after it has fired N times as a trigger/hook (0 = forever) |
| `-1` | same as `-n1` |
| `-cN` | percent chance (default 100) that a match runs its body |
| `-E'expr'` | only consider this macro when the expression is true (evaluated for every line, keep it cheap) |
| `-wworld`, `-Ttype` | limit to a world, or to worlds of a type |
| `-a[ngGLAurBbhC]` | display attributes for the matched line: `g` gag, `G` no-history, `L` no-log, `A` no-activity, `u` underline, `r` reverse, `B` bold, `b` bell, `h` hilite, `n` normal, `C`+colour (`Cred`) |
| `-P[part]attr;...` | partial hilite of a part (`0` whole match, `1`.. group, `L`, `R`) in a regexp trigger |
| `-i` | invisible: skipped by `/list`, `/save`, `/purge` unless forced |
| `-q` | quiet: does not count as "handled" for `BACKGROUND`, the return value of `/trigger`, or `SEND`/`PROMPT` hooks |

**Arguments.** `name` is optional for triggers/hooks. The body after `=` runs
when the macro is called, with the arguments as `%1`, `%{-1}`, `%*`...; a
definition on one line may use `%;` between commands.

**Return value:** the number of the new macro, or 0 on failure.

```tf
/def -t'You hit *' hit = /echo Hit! (%*)
/trigger You hit the orc.
/def -mregexp -t'^(\w+) (arrives|leaves)\.$' watch = /echo %P1 %P2 (%{#} words)
/trigger Zed arrives.
/trigger Zed leaves.
/trigger Zed dies.
```
```output
Hit! (You hit the orc.)
Zed arrives (2 words)
Zed leaves (2 words)
```

Priorities and fall-through: of all matching triggers only the highest-priority
one runs unless it is marked `-F`.

```tf
/def -t'bar' -p3 low = /echo low
/def -t'bar' -p5 high = /echo high
/trigger bar
/def -t'tock' -F -p1 fa = /echo first
/def -t'tock' -p1 fb = /echo second
/trigger tock
```
```output
high
first
second
```

Shots, conditions and one-line macros:

```tf
/def -n2 -t'tick' tk = /echo tick
/trigger tick
/trigger tick
/trigger tick
/set alert=0
/def -E'alert' -t'danger*' dg = /echo RUN!
/trigger danger ahead
/set alert=1
/trigger danger ahead
/def double = /echo $[2 * {1}]
/double 21
```
```output
tick
tick
RUN!
42
```

**Gotchas**

- `/def` body text is substituted **when the macro runs**, not when defined, but
  the *line* containing `/def` has already been through one round of substitution,
  so in macros that define macros double the metacharacters (`%%`, `$$`, `\\`).
- A name that shadows a builtin gets a warning (`macro "sh" conflicts with the
  builtin command`) and is still created; call the builtin as `/@sh`.
- A macro called as a *command* ignores `-n`: shots only count trigger/hook firings.
- `-p` accepts a variable (`-pmaxpri`): the value is read once, at definition.
- A glob pattern with `{a|b}` alternation and `[a-c]` classes is available in `-t`.
- Real tf accepts `-1` as a one-shot; the engine does not yet (FINDINGS C3).
  Hooks fired by `Interp.Hook(event, args)` from Go; `/trigger -h` does not (C5).
  Fall-through to a *lower* priority than the `-F` macro does not run it yet (C15); equal priorities work.

**Status:** see [Status](Status.md)

## /undef name...

**What it is for:** deleting macros by name.

**Syntax:** `/undef name [name ...]`

**Return value:** the number of macros removed. Unknown names warn but do not
stop the rest.

```tf
/def tmp1 = /echo one
/def tmp2 = /echo two
/undef tmp1
/echo $[ismacro("tmp1") + 0]
/echo gone, but tmp2 stays: $[ismacro("tmp2") > 0]
```
```output
0
gone, but tmp2 stays: 1
```

**Gotchas.** Multi-name removal returns the count in real tf (2 for two names);
the engine returns 1 (FINDINGS C8). To remove by *trigger text* use `/untrig`; by
number, `/undefn`.

**Status:** see [Status](Status.md)

## /undefn number...

**What it is for:** deleting a macro by the number `/list` and `/def` report,
useful for macros with no name (a bare trigger).

**Syntax:** `/undefn number [number ...]`

```tf-noverify
/def -t'spam*' -ag
/list -s -t'spam*'
% 751: (trig) 'spam*'
/undefn 751
```

**Status:** see [Status](Status.md)

## /undeft pattern

**Library.** Removes triggers whose *text* equals `pattern` (attributes ignored).
Same idea as `/untrig`; `/undeft` also strips gag and hilite attributes.

**Status:** see [Status](Status.md)

## /purge [options] [name] [= body]

**What it is for:** removing every macro that matches a description. Takes the
same options as `/list`, so `/list` first, then `/purge` with the same words,
is the safe way to clean out a family of macros.

**Syntax:** `/purge [-mstyle] [-t pat] [-h ev] [-ai...] [name-pattern] [= body-pattern]`

Invisible macros (`-i`) are skipped unless `-i` is given.

```tf
/def -mglob -t'ant*' a1 = /echo a1
/def -mglob -t'bee*' b1 = /echo b1
/def keep = /echo keep
/purge {a1|b1}
/echo $[ismacro("a1")]/$[ismacro("b1")]/$[ismacro("keep") > 0]
```
```output
0/0/1
```

**Gotchas.** `/purge` with no arguments removes every visible macro, including
your own definitions: not `/purge` alone. The name is ONE pattern, using `%{matching}` (glob by default):
`/purge g*` removes every macro beginning with `g`, `/purge {a|b}` removes `a` and `b`.

**Status:** see [Status](Status.md)

## /purgedef, /purgetrig, /purgegag, /purgehilite, /purgebind, /purgehook, /purgedeft pattern

**Library.** One-argument wrappers around `/purge` for each kind of macro
(always glob matching): plain macros (by name; default `?*`), triggers (by trigger text), gags,
hilites, key bindings, hooks, and "trigger-only" macros respectively.
`/purgeworld` (a synonym for `/unworld`) is covered under worlds.

```tf-noverify
/gag *spam*
/purgegag *spam*
```

**Status:** see [Status](Status.md)

## /list [-s] [-S] [options] [name] [= body]

**What it is for:** showing macros: all of them, or the ones matching the options
you give. It is how you inspect what `/def`, `/gag` or `/hilite` actually created.

**Syntax:** `/list [-s] [-S] [-mstyle] [-t pat] [-b pat] [-h ev] [-a attrs] [-P] [-i|-I] [name] [= body]`

**Options.** `-s` short (one line each, kind then name); `-S` sort by name; the
other options select macros: `-t`, `-b`, `-E`, `-T`, `-h`, `-a` (attributes), `-P`
(partial hilites), `-i` (include invisible), `-I` (only invisible), name glob,
`= body` glob; a name starting with `#` selects by number. An option without
an argument matches any macro that has it; `{}` (or `-h0`) matches macros that
do not.

**Return value:** the number of the last macro listed, or 0.

```tf-noverify
/def -t'You hit *' hit = /echo Hit!
/list hit
% 751: /def -p1 -mglob -t'You hit *' hit = /echo Hit!
/list -s -t
% 751: (trig) 'You hit *' hit
```

**Gotchas.** Macro numbers depend on how many library macros are loaded, so do not
hard-code them. Listing goes to the screen, not through `/echo`.

**Status:** see [Status](Status.md)

## /listdef, /listtrig, /listgag, /listhilite, /listfullhilite, /listpartial, /listbind, /listhook

**Library.** `/list` pre-set to one kind of macro: plain definitions, triggers
(no attributes), gags, hilites (`/listhilite` = full-line + partial), partial
hilites, key bindings, hooks. The argument is a glob on the trigger text (the key
for `/listbind`, the event for `/listhook`).

```tf-noverify
/gag *spam*
/listgag
% 752: /def -p0 -ag -mglob -t'*spam*'
```

**Status:** see [Status](Status.md)

## /edit [options] name = body

**What it is for:** changing an existing macro in place, keeping what you do not
mention: `/edit -t'new pattern' hit` alters the trigger of `hit` only.

**Syntax:** `/edit [options] name [= body]`

```tf-noverify
/def hit = /echo ouch
/edit -t'You are hit*' hit
```

**Gotchas.** The macro is found by name (or `#number`); options replace the old
ones, the body only changes when `= body` is given. Reads from or writes to the
input line in some versions (`/edit` after `/grab` is the editing workflow).

**Status:** see [Status](Status.md). Needs the host's input line for the interactive form.

## /redef, /ismacro, /isvar

**/ismacro name** (Library): returns the number of the macro (non-zero) if it
exists, else 0; **/isvar name** (Library): returns 1 if a variable of that
name exists, else 0. Both set `%?`, and print nothing. As functions: `ismacro()`,
`isvar()`.

```tf
/def probe = /echo probing
/ismacro probe
/echo macro found: $[%? > 0]
/ismacro nope
/echo macro missing: %?
/set sw=1
/isvar sw
/echo isvar: %?
/isvar nosuchvar
/echo isvar: %?
```
```output
macro found: 1
macro missing: 0
isvar: 1
isvar: 0
```

`/redef` is a one-word option macro: `/redef on|off` sets the variable `redef`
(see the option macros at the end of this page).

**Status:** see [Status](Status.md)

## /trigger [-ln] [-g] [-w[world]] [-hevent] text

**What it is for:** running the macros whose triggers (or hooks) match `text`,
as if `text` had arrived from the game. The standard way to test triggers
without a connection, and to make one script feed another.

**Syntax:** `/trigger [-ln] [-g] [-w[world]] [-hevent] text`

**Options.** `-hevent` matches **hooks** on that event whose argument pattern
matches `text` (without it, triggers are matched); `-n` only *lists* what would
run; `-l` lists it in full; `-g` / `-w[world]` restrict to global / world macros.

**Return value:** the number of (non-quiet) macros that ran.

```tf
/def -t'* tells you: *' -mglob reply = /echo reply to %1
/trigger Bob tells you: hello
/trigger nothing here
```
```output
reply to Bob
```

**Gotchas.** Echoed text (from `/echo`) is never matched against triggers. In this
engine `/trigger` currently ignores its options (FINDINGS C5); to fire a hook
from Go use `Interp.Hook(event, args)`.

**Status:** see [Status](Status.md)

## /trig, /trigc, /trigp, /trigpc, /act pattern = body

**Library.** Shorthands for a trigger that takes just a pattern and a body, with
optional probability (`trigc`), priority (`trigp`) or both (`trigpc`).

**Syntax**

    /trig pattern = body            same as  /def -t'pattern' = body
    /trigp priority pattern = body
    /trigc chance pattern = body
    /trigpc priority chance pattern = body      (the builtin the others call)
    /act pattern = body             old name for /trig

```tf-noverify
/trigp 5 You are hungry = eat bread
/trigc 25 *smiles* = smile
```

**Gotchas.** The pattern follows `%{matching}`; give a body whose first word is a
command or the text is sent to the game.

**Status:** see [Status](Status.md)

## /hook event[ pattern] [= body]  and  /hook [on|off]

**What it is for:** running a macro when the *client* does something (a world
connects, a line is about to be sent, a key is bound...) rather than when the
game sends text. `/hook` is the short form of `/def -h`.

**Syntax**

    /hook event[ pattern] [= body]
    /hook                    show whether hooks are enabled
    /hook on|off             enable/disable all hooks (variable %hook)

`event` may be a list `A|B`; common ones are `CONNECT`, `DISCONNECT`, `LOGIN`,
`SEND`, `PROMPT`, `ACTIVITY`, `RESIZE`, `WORLD`, `LOAD`, `ICONFAIL`, `BACKGROUND`.
Each `/hook` adds another macro on the event; they run by priority.

```tf-noverify
/hook CONNECT = /echo Connected to ${world_name}
/def -h'SEND {k|K}*' -F -q log_k = /echo sent a k-line
```

**Return value:** the new macro's number, or 0. **Gotchas.** The pattern's match
style (`%{matching}`) is read when the hook is *defined*.

**Status:** see [Status](Status.md)

## /unhook event[ pattern]

**Library.** Removes hooks whose event text equals the argument. Warns
"No hook on ..." if none.

**Status:** see [Status](Status.md)

## /untrig pattern

**Library.** Removes triggers (any macro with a `-t` pattern equal, literally, to
`pattern`), including its gag/hilite attributes; warns "No trigger on ..." if none.

```tf-noverify
/def -t'You are hungry' hungry = eat bread
/untrig You are hungry
```

**Gotchas.** The match is on the pattern *text*, so quoting is: `/untrig You are
hungry`, not a glob of it. With glob characters in the pattern the engine
currently fails to match (FINDINGS C11).

**Status:** see [Status](Status.md)

## /gag pattern

**What it is for:** hiding lines from the game that match a pattern (spam,
repeated combat messages, other players' pages).

**Syntax:** `/gag [pattern]`

**Arguments.** A glob (or `%{matching}` style) pattern. Equivalent to
`/def -ag -t'pattern'`. With no argument it only reports/ensures the `%gag`
flag (gagging enabled). `%gag` off disables all gags at once (`/nogag` with no
argument).

```tf-noverify
/gag *'s rowdy friends*
/gag You hear a distant howl
```

**Gotchas.** A gagged line is still *matched* by other triggers of lower priority
unless the gag trigger has higher priority and is not `-F`. A gag defined with
`/gag` uses priority `%gpri` (0 by default; see `/gpri`).

**Status:** see [Status](Status.md)

## /nogag [pattern]

**Library.** `/nogag pattern` removes the gag with that pattern (via `/untrig -ag`);
with no argument it turns gagging off globally by setting `gag=0`.

**Status:** see [Status](Status.md)

## /hilite pattern

**What it is for:** making matching lines stand out (the default is bold-ish
"hilite") in the display.

**Syntax:** `/hilite [pattern]`

Equivalent to `/def -ah -t'pattern'`. For other looks use `/def -aBCred -t'...'`
(bold red) or `-au`, `-aCgreen`. No argument reports the `%hilite` flag.

```tf-noverify
/hilite {*} tells you: *
/def -aBCred -t'*** WARNING*' warn
```

**Status:** see [Status](Status.md)

## /nohilite [pattern]

**Library.** Counterpart of `/hilite`: removes hilites for the pattern, or turns
hiliting off (`hilite=0`) with no argument.

**Status:** see [Status](Status.md)

## /partial regexp

**Library.** Hilites only the part of the line the regexp matches
(`/def -F -p%{hpri} -Ph -t"regexp"`), leaving the rest alone. Needs a regexp.

```tf-noverify
/partial \bdragon\b
```

**Status:** see [Status](Status.md)

## /hilite_page, /hilite_whisper, /nohilite_page, /nohilite_whisper

**Library.** One-word presets that hilite the usual MUD page/whisper formats
(several `-ah` triggers named `~hilite_page1..5`, `~hilite_whisper1`).
The `nohilite_` forms purge them.

**Status:** see [Status](Status.md)

## /bind sequence = command  and  /unbind sequence

**What it is for:** attaching a command to a key sequence. The sequence uses
`^X` for control-X, `^[` for Escape and `^^` for a literal caret.

**Syntax:** `/bind sequence = command`  `/unbind sequence`

`/bind` is `/def -b'sequence' = command`. In real tf a sequence that is a prefix
of an existing binding is refused.

```tf-noverify
/bind ^[oj = /input OliverJones
/unbind ^[oj
```

**Gotchas.** Whether the key is delivered at all depends on the front end's
terminal handling; `/dokey` (below) gives a script the editing actions
by name.

**Status:** see [Status](Status.md). Capability: `KeyHost` for key codes.

## /dokey name

**What it is for:** doing an input-editing action from a macro, the same as
pressing the key bound to it: `/dokey dline` (delete line), `/dokey newline`
(send the line), `/dokey pause` (pause output), `/dokey recallb` (history).
The names are those used by the default key bindings (`bind.c` list).

```tf-noverify
/def -h'ACTIVITY' wakeup = /dokey dline
```

**Status:** see [Status](Status.md)

## /set name=value  and  /set [name [value]]

**What it is for:** creating or changing a global variable, or looking at it.

**Syntax**

    /set name=value        no spaces around =
    /set name value        two-argument form
    /set name              show one variable
    /set                   show all

**Return value:** 1 on success. Showing returns the number listed.

Setting a built-in special variable (`wrap`, `borg`, `quiet`, `more`...) changes
the client's behaviour. Values are strings; arithmetic needs `$[...]` or `/test`.

```tf
/set target=orc
/set mood happy as a clam
/echo hunting the %target, feeling %{mood}
/set target
/set empty=
/echo [%empty]
/set hp=40
/set hp=$[hp - 7]
/echo hp is %hp
```
```output
hunting the orc, feeling happy as a clam
% target=orc
[]
hp is 33
```

**Gotchas**

- `/set` is for globals; inside a macro prefer `/let` so you do not clobber
  state. `%{name}` and `%name` both read a variable; `${name}` is the form for
  `/def` bodies that may be expanded twice.
- `L` and `R` are reserved names.

**Status:** see [Status](Status.md)

## /let name=value  and  /let name value

**What it is for:** a variable that exists only inside the running macro (or
`/eval`) and vanishes with it.

**Syntax:** `/let name=value`  `/let name value`

Usable only during macro expansion. If a local of that name exists in a
calling macro it is not shared: the callee gets its own.

```tf
/def lt = /let a=1%; /let b 2%; /echo %a %b%; /set seen=yes
/lt
/echo [%a] [%seen]
```
```output
1 2
[] [yes]
```

**Status:** see [Status](Status.md)

## /setenv name=value  and  /setenv [name [value]]

**What it is for:** setting a variable in the process environment, which programs
started by `/sh` and `/quote !` inherit; a variable can be either an internal tf
variable or an environment one, not both.

**Syntax:** like `/set`, except in the `name=value` form spaces after the `=` are
kept. **Return value:** 1/0, or the count listed.

```tf
/setenv GREETING=hi there
/echo %{GREETING}
```
```output
hi there
```

**Status:** see [Status](Status.md)

## /export variable

**What it is for:** turning an existing global variable into an environment variable.

```tf
/set mudname=Arcadia
/export mudname
/echo %mudname
```
```output
Arcadia
```

**Gotchas.** Locals cannot be exported. Afterwards `/listvar -x` shows it as
`/setenv name=value` in real tf (the engine prints `/set`; FINDINGS C6).

**Status:** see [Status](Status.md)

## /unset name

**What it is for:** removing a variable. Returns 0 on error, non-zero otherwise.

```tf
/set temp=1
/unset temp
/isvar temp
/echo exists: %?
```
```output
exists: 0
```

**Status:** see [Status](Status.md)

## /listvar [-mstyle] [-gxsv] [name [value]]

**What it is for:** listing variables whose name (and optionally value) match,
sorted by name.

**Options:** `-g` only global, un-exported; `-x` only exported; `-s` names only;
`-v` values only (documented; both real tf and the engine print the full `/set` form in current builds); `-m` match style for the patterns.

**Return value:** how many were listed.

```tf
/set myhp=40
/set mymp=12
/listvar -s my*
/listvar my*
```
```output
myhp
mymp
/set myhp=40
/set mymp=12
```

**Gotchas.** `/listvar` with no arguments lists every variable including all the
client's special ones.

**Status:** see [Status](Status.md)

## Library: key/value and word helpers for macro bodies

**/first words**, **/rest words**, **/last words**, **/nth n words**: print the
first word, all but the first, the last, or the nth (1-based, from the argument
list). **/split text** splits `key = value`-style text into `%P1` and `%P2`.
**/escape chars text** prints `text` with each character in `chars` (and the backslash) preceded by a
backslash; it is how library macros quote a literal into a pattern.

```tf
/first alpha beta gamma
/rest alpha beta gamma
/last alpha beta gamma
/echo $[escape("*?", "a*b?c")]
```
```output
alpha
beta gamma
gamma
a\*b\?c
```

**Gotchas.** `/nth` and `/split` do not work in the engine yet (FINDINGS C9, C10).

**Status:** see [Status](Status.md)

## Library: saving and loading macro collections

**/savedef [file] [pattern]**, **/savegag**, **/savetrig**, **/savebind**,
**/savehook**, **/savehilite** write the matching kind of macro to a file
(default file from the variable `MACROFILE`, `GAGFILE`, `TRIGFILE`, `BINDFILE`,
`HOOKFILE`, `HILITEFILE`); **/loaddef**, **/loadgag**, **/loadtrig**,
**/loadbind**, **/loadhook**, **/loadhilite** (and **/loadworld**) load them
back. They call `/save` and `/load`, which are described with the file commands.
**/loadhist file** loads a history file with `/recordline`.

```tf-noverify
/gag *spam*
/savegag ~/.tf-gags
; later, in another session:
/loadgag ~/.tf-gags
```

**Status:** see [Status](Status.md). Needs `/save`.

## Library: option macros (`/name value` sets variable `name`)

Each of these is one line in `stdlib.tf`: `/name args` runs `/set name args`.
They exist so that old habits like `/borg on` keep working.

| Macro | Variable | Meaning |
|---|---|---|
| `/quiet`, `/noquiet` | `quiet` | suppress most of tf's informational `%` messages |
| `/borg`, `/reply` | `borg` | `borg`: whether triggers with a body of plain text run it automatically |
| `/background` | `background` | allow output from non-foreground worlds |
| `/bamf` | `bamf` | the "bamf" teleport handling (old) |
| `/clearfull`, `/cleardone` | `clearfull`, `cleardone` | screen clearing behaviour |
| `/insert` | `insert` | input editing insert vs overwrite |
| `/login`, `/nologin` | `login` | automatic login on connect |
| `/lp`, `/lpquote` | `lp`, `lpquote` | LP-MUD style handling |
| `/quitdone`, `/redef`, `/shpause`, `/sockmload` | same | misc. flags |
| `/sub` | `sub` | substitution level for typed lines: `off`, `on`, `full` |
| `/visual` | `visual` | split-screen mode |
| `/gpri`, `/hpri` | `gpri`, `hpri` | priority for `/gag` and `/hilite` triggers |
| `/isize`, `/ptime`, `/wrapspace` | same | input size, pause time, wrap indent |
| `/wrap`, `/nowrap` | `wrap`, `wrapsize` | see the output page |
| `/kecho`, `/mecho`, `/qecho` | `*echo`, `*prefix` | see the output page |

```tf
/quiet on
/echo quiet=%quiet
/borg off
/echo borg=%borg
/sub full
/echo sub=%sub
/gpri 3
/echo gpri=%gpri
```
```output
quiet=on
borg=off
sub=full
gpri=3
```

**Gotchas.** `/nolog` is `/log off`, `/nowrap` is `/set wrap off`, `/nologin` and
`/noquiet` set the flag off. Values are the variable's own: on/off or 0/1.

**Status:** see [Status](Status.md)
