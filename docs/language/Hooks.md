# Hooks

A *hook* is a macro that runs when something happens inside the client rather
than when a line arrives: a connection opens, a file is loaded, a prompt
appears, an unknown command is typed. Hooks are to events what
[triggers](Triggers.md) are to text.

```tf
/def -hCONNECT greet = /echo connected to %1
/trigger -hCONNECT mymud
```
```output
connected to mymud
```

`/def -h'EVENT [pattern]' name = body` defines one. The event name is
case-insensitive. `/hook EVENT pattern = body` is a shortcut. As with
triggers, `/trigger -hEVENT args` fires an event by hand, which is the way to
test a hook from a script.

## Events and arguments

When an event fires it supplies **arguments**: the words your macro sees as
`%1`, `%2`, `%*`, `%#`. The optional `pattern` in the definition is matched
against the argument text (glob by default; `-m` and `%matching` apply as for
triggers).

```tf
/def -hCONNECT hc = /echo CONNECT hook: world=%1 args=[%*] n=%#
/trigger -hCONNECT mymud
/trigger -hCONNECT mymud extra words
/def -h'LOAD *.tf' hl = /echo load hook: %1
/trigger -hLOAD foo.tf
/trigger -hLOAD foo.txt
```
```output
CONNECT hook: world=mymud args=[mymud] n=1
CONNECT hook: world=mymud args=[mymud extra words] n=3
load hook: foo.tf
```

The second `LOAD` shows a pattern at work: `foo.txt` does not match `*.tf`, so
nothing ran.

A hook can listen to several events at once by separating them with `|`:

```tf
/def -h'ACTIVITY|BGTEXT' hb = /echo multi: %*
/trigger -hACTIVITY w1
/trigger -hBGTEXT w2
```
```output
multi: w1
multi: w2
```

## The events

The core set. "Args" is what `%1`, `%2`... will hold.

| Event | Args | When |
|---|---|---|
| `CONNECT` | world | a connection is established |
| `DISCONNECT` | world, reason | a connection closed |
| `CONFAIL` | world, reason | a connection attempt failed |
| `PENDING` | world[, address] | a connection attempt is in progress |
| `LOGIN` | world | time to send the automatic login |
| `WORLD` | world | the foreground world changed |
| `ACTIVITY`, `PREACTIVITY` | world | first new text arrived in a background world |
| `BGTEXT` | world | text was shown in a background world |
| `BGTRIG` | world | a trigger fired in a background world |
| `PROMPT` | text | a partial line (a prompt) arrived; see below |
| `SEND` | text | text is about to be sent to the world |
| `NOMACRO` | name | `/name` is neither a macro nor a builtin |
| `REDEF` | type, name | a macro was redefined |
| `CONFLICT` | macro | a macro's name collides with a builtin |
| `LOAD` | file | `/load` started |
| `LOADFAIL` | file, reason | `/load` could not read the file |
| `LOG` | file | logging started |
| `PROCESS`, `KILL` | pid | a `/repeat` or `/quote` process started or ended |
| `RESIZE` | columns, lines | the display was resized |
| `SIGHUP`, `SIGTERM`, `SIGUSR1`, `SIGUSR2` | | the signal arrived |

(There are a few more for mail, `--More--`, proxies and the like; the
engine reference (`docs/engine/hooks.md` and `docs/engine/triggers-hooks.md`) has the complete table with default messages.)

Several events have a **default message** the client would print, such as
`% Connected to mymud.`. A hook with the gag attribute (`-ag`) suppresses
it, and other attributes style it. The message is shown *before* the hook
bodies run.

```tf
/def -hNOMACRO nm = /echo no such macro: %1
/trigger -hNOMACRO frobnicate
```
```output
no such macro: frobnicate
```

### `SEND`

`SEND` fires when text would be sent to the world. If a `SEND` hook matches,
the original text is **not** sent unless the hook was defined with `-q`; the
hook runs instead. It is a way to rewrite everything you type: the hook
decides what, if anything, is sent. Be careful: a `SEND` hook that sends text
can trigger itself.

```tf
/def -hSEND sendh = /echo SEND hook saw: %*
/trigger -hSEND say hi there
```
```output
SEND hook saw: say hi there
```

### `PROMPT`

Prompts (lines without a newline, such as `login:`) never match triggers. They
reach macros through the `PROMPT` hook with the prompt text as the argument. A
`PROMPT` hook defined with `-q` does not consume the text.

```tf
/def -n1 -hPROMPT hp = /echo prompt hook once: %*
/trigger -hPROMPT login:
/trigger -hPROMPT login:
```
```output
prompt hook once: login:
```

## Priority, fall-through and shots

Hooks are selected exactly like triggers: highest `-p` first; `-F` macros all run;
among the non-fall-through macros at the first priority that has one, a single
one is chosen at random; `-n` limits the number of runs. See
[Triggers](Triggers.md#priority-and-fall-through).

```tf
/def -hRESIZE -p2 r2 = /echo high
/def -hRESIZE -p1 r1 = /echo low
/trigger -hRESIZE 80 24
/def -hWORLD -p2 -F w2f = /echo first
/def -hWORLD -p1 w1f = /echo second
/trigger -hWORLD foo
```
```output
high
first
second
```

Hooks can use a regexp pattern too:

```tf
/def -mregexp -h'DISCONNECT ^(\w+) (.*)$' dc = /echo dc: [%P1] [%P2]
/trigger -hDISCONNECT mymud closed
```
```output
dc: [mymud] [closed]
```

The `%hook` variable turns all hooks off when set to `off`.

## How a program fires hooks

Your program decides when events happen. It tells the interpreter with
`FireHook(event, args)` (or `FireHookArgs(event, args...)`, which returns the full
`*HookResult`; see `docs/engine/hooks.md`):

```go
ran := in.FireHook("CONNECT", "mymud")       // %1 is "mymud"; ran is the []*Macro that ran
in.FireHook("DISCONNECT", "mymud connection lost")
```

The default message (here `% Connected to mymud.`) is echoed first, unless a
hook gags it. The arguments are one string, split into words for `%1`, `%2`... and matched
against the macro's pattern. Fire an event even when you expect no scripts to
care; a macro defined later may. The interpreter itself raises the events that come from running scripts
(`LOAD`, `REDEF`, `NOMACRO`, `PROCESS`, `KILL`...); see the engine reference for
exactly which.

## Pitfalls

* **Expecting hooks to fire for events the host never reports.** `CONNECT` only
  runs if the program calls `FireHook("CONNECT", ...)` (or the client's own
  `/connect` support does).
* **Name confusion.** `ACTIVITY` is the first text in a background world, not
  "any text".
* **`SEND` loops.** A `SEND` hook that sends text.
* **Same-priority hooks.** One is chosen at random; use `-F` or priorities.
* **Hook patterns match the argument text**, not a line from the world.
* **Quoting.** `-h'EVENT pattern'` needs the quotes as soon as there is a
  space.

## See also

[Triggers](Triggers.md) ·
[Macros](Macros.md) ·
[Patterns](Patterns.md)
