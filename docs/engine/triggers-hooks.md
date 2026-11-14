# Triggers, hooks and display attributes (client API)

This describes how a client drives the macro database in `gtfparser`: feeding
it received lines, getting back what to display, firing hook events, and the
extension points a client can plug into. Behaviour follows real TinyFugue 5
(`macro.c`); where gtf cannot match, see *Known differences* at the end.
Hook events: see `hooks.md`.

## Receiving text from a world

```go
res := in.ProcessText(gtfparser.LineInput{Text: line, World: "mymud"})
// or in.Process(line, live) / in.ProcessLine(line) for the current world
```

`ProcessText` runs the line through every trigger (`/def -t`), exactly like tf
does for a line from a socket, and returns a `*LineResult`:

| field | meaning |
|-------|---------|
| `Line` | the line to display: text (after any `/substitute`) with the line attributes and per-character (`-P` partial hilite) attributes triggers applied. `Line.Spans(ctx)` / `res.Spans(in)` give runs of text with identical effective attributes. |
| `Gag` | do not display the line (tf still lets `/recall` find it) |
| `NoLog`, `NoHistory`, `NoActivity`, `Bell` | log / record in history / count as activity / ring the bell? |
| `Substituted` | a trigger replaced the text with `substitute()` / `/substitute` |
| `Matched`, `Matches` | macros selected, in order; `Matches` adds where each matched (`Match{Macro, Span, Groups}`: byte range of the whole match and of each regexp group) |
| `Ran` | every selected macro: `Macro`, `Name`, `Args`, `Ran`, `Counted` (not `-q`), `Skipped` (`"prob"`, `"rate"`, `"recursion"`) |
| `Count` | number of non-quiet macros that ran. For a background world, fire `BGTRIG` when `Count > 0` |
| `ShotsUsed` | macros whose last `-n` shot was consumed (already undefined) |
| `Suppressed` | a pre-line filter vetoed processing |

The client then: skips display if `Gag`; otherwise draws `Spans`; honours
`NoLog`/`NoHistory`/`NoActivity`. (Attributes are `gtfparser.Attr` bit sets;
`Attr.String()` renders them as tf's attribute string, e.g. `uBCred,Cbgblue`.)

`in.Process(line, live) LineResult` is the same for one plain line, by value.
With `live` false it only matches (a dry run): no macro body runs and no shot is
consumed (the attributes of the selected macros are still applied), so a client
can recompute the display effects of replayed history; `LineInput{Dry: true}` is
the same for `ProcessText`. `in.ProcessLine(line) bool` runs live and returns
`Gag`.

`LineInput` also has `Attr`/`Line` (incoming attributes, e.g. from
`AttrCtx.DecodeAnsi`), `WorldOnly` and `GlobalOnly` (`/trigger -w` / `-g`).

### Selection rules (priority)

Exactly tf's: macros are visited in decreasing priority, fall-thrus (`-F`)
before non-fall-thrus of the same priority (newest definition first). Each
matching fall-thru is applied/run at once. The first matching non-fall-thru
fixes the lowest priority considered; **one** of the non-fall-thrus of that
priority is chosen at random and applied. (So "equal priority" macros do *not*
all run, unless they are fall-thrus.) Priorities below 1 that are negative
do not exist: tf treats a negative `-p`/`-c`/`-n` as "not given" and uses the
default (priority 1, probability 100, permanent). A macro that is only an attribute match (empty body) still counts for
priority. `-E` is evaluated per line; a macro whose `-E` is false is invisible
to the search. `-c` probability and the random choice use
`in.SetRand(func(n int) int)` (deterministic in tests).

Conditions on a macro: `-w world` (text from that world only; `-w` alone =
the current world when defined), `-T type` (the world type, a pattern in the
macro's match style), `-E expr`, `-A attrs` (the line's attributes must equal
`attrs`), the `%borg` flag (bodies run only when on; attribute macros need
`%hilite`/`%gag`), `%max_trig` (rate limit: exceeded ⇒ `%borg` turned off).

## World context

Implement `WorldInfoHost` (optional) so `-w` and `-T` can work:

```go
type WorldInfoHost interface {
	CurrentWorld() string          // xworld: where the text/event came from
	WorldType(world string) string // for /def -T
	WorldExists(world string) bool // validates /def -w (error "No world x")
}
```

Without it, `-w` is accepted unvalidated, `-T` never matches (no world type),
and the "current world" falls back to `WorldHost.ForegroundWorld()`.
When a world is deleted call `in.RemoveWorldMacros(name)`.

## Match styles

`/def -msimple|glob|regexp|substr` as in tf. A client can add more:

```go
in.RegisterMatchStyle("prefix", gtfparser.MatchStyleFunc(func(p string) (gtfparser.Matcher, error) {
	return gtfparser.MatcherFunc(func(s string) (bool, []int) { return strings.HasPrefix(s, p), nil }), nil
}))
// /def -mprefix -t'Hello' greet = ...
```

A style compiles once at `/def`; `Matcher.Match` may return capture offsets
(`{start0,end0,start1,end1,...}`, -1 = unset) to supply `%P0..`, `%PL`, `%PR`.
Custom styles are selected with `-m` (the `%matching` variable only knows the
four built-in names). `in.CompilePattern(style, pat)` exposes the same
compiler. Regexps are PCRE2-compatible with tf's options (smart case, dot
matches newline, `$` end-only).

## Extension points

| hook | purpose |
|------|---------|
| `RegisterMatchStyle` | custom pattern dialects |
| `AddPreLineFilter(func(*Interp, *LineInput) bool)` | edit a line before triggers; return false to skip triggers |
| `AddPostLineFilter(func(*Interp, *LineResult))` | adjust the result (force a gag, add attributes) |
| `AddHookFilter(func(*Interp, *HookEvent) bool)` | edit/drop hook events |
| `AddObserver(TriggerObserver)` | debugging: `EvSelected`, `EvRun`, `EvSkipProb`, `EvSkipRate`, `EvShotsExhausted`, `EvSubstitute`, `EvCalled` (any macro called by name - run accounting). Hook activity has `Hook` set. |
| `SetRand` | injectable random source |
| `WorldInfoHost` | world / world type resolution |
| `HookDisplayHost` | where hook messages go (screen/alert/world) |
| `AttrEchoHost` | render attributed text (`/echo -a/-p`, `echo()`, hook messages) |
| `Notifier` | receives `Event{Kind:"rate"}` when `%max_trig`/`%max_hook` trip |
| `EnableHookEvents` | turn on ATCP/GMCP/MSDP/OPTION102 events |
| `Macro.Used`, `Macros()`, `FindMacro()`, `FindBinding()` | inspect the database |
| `ParseMacroSpec`, `Define` | build macros from Go (`/edit`, client startup bindings) |

Observers run synchronously on the interpreter goroutine and must not call
back into the interpreter.

## Hooks

`FireHook(event, args) []*Macro`, `FireHookArgs(event, args...)` /
`FireHookEvent` - see `hooks.md`. The latter two return
`*HookResult{Count, Ran, Message, HasMessage, Gag, Class}`. `SEND` and `PROMPT`
use `Count > 0` as "handled".

## Attributes

`Attr` is tf's attribute set (256-colour layout: `AttrBold`, `AttrUnderline`,
`AttrReverse`, `AttrItalic`, `AttrGag`, `AttrNoLog`, ..., foreground/background
colour numbers via `FgColor()`/`BgColor()`, `WithFg/WithBg`). `AttrCtx`
(from `in.attrCtx()` - exported through `ParseAttrs`) carries `%hiliteattr`,
`%error_attr`, `%warning_attr`, `%info_attr` and provides
`Adj` (combine, `x` = exclusive), `Parse` (attribute lists such as
`BuCred,Cbgblue`), `DecodeAttr`/`EncodeAttr` (the `@{...}` inline syntax),
`DecodeAnsi`/`EncodeAnsi` (ANSI SGR <-> attributes; `emulation` levels
`EmulRaw..EmulDebug`; returns the final state so a client can carry it across
lines). `AttrString{Text, Attr, Chars}` is attributed text (`Chars` is
per-byte, nil if the whole line is uniform). Script functions:
`decode_attr`, `encode_attr`, `decode_ansi`, `encode_ansi`, `strip_attr`,
`strcmpattr`, `substitute`, `echo(text, attrs, inline, dest)`.

`@{...}` in `/echo -p` and `echo(.., 1)`: codes are attribute lists;
`@{~u}` turns attributes off, `@{n}` turns everything off, `@@` is a literal
`@`.

## The macro database

`/def /undef /undefn /purge /list /save /hook /gag /hilite /trigpc /trigger`
are builtins (`/trig /trigp /trigc /partial /undeft /nogag /nohilite /untrig
/unhook` are stdlib macros over them). Macros keep tf's numbers, case-insensitive
names, redefinition rules (`%redef`, REDEF hook, identical redefinition is a
no-op returning the old number), invisibility (`-i`/`-I`), `/list` formats
(including `-s`, `-S`, `-u` usage counts and the `-t -h -b -E -T -a -P -p -n
-c -F -w -m` selectors). Macro lookup is `Interp.FindMacro`. `/edit` keeps the
macro's number and place in the list (it reinstalls an edited copy under the
same number, so a `*Macro` held across an `/edit` is stale).

The text fields `Macro.Trigger`, `Hook`, `Attrs`, `Gag` and `WorldType` mirror
the parsed fields (`Trig`, `Hooks`/`HArgs`, `Attr`, `WType`) for clients that
read a macro as text; they are filled in when the macro is installed and are
read-only. `Define` accepts a Go-built macro described either way.

World scoping: a macro defined with `-w` fires only while the interpreter acts
for that world (`SetWorld`, compared case-insensitively); with no current world
set every macro is eligible.

## Known differences from real tf

* **Strings carry no attributes.** Script values are plain strings:
  `decode_attr()` returns the visible text, `encode_attr(s)` doubles `@`,
  `strip_attr`/`strcmpattr` operate on text. Attributed output is available
  to Go through `AttrString` (`/echo -p -a`, `substitute(..., inline)` and
  triggers all keep attributes end to end); only the round trip through a
  script *variable* loses them.
* `%Pn` of a regexp trigger is a snapshot taken when the macro is selected;
  tf reads the live state of the regexp, so a nested `/trigger` that re-tests
  the same regexp can blank the outer macro's `%Pn` there.
* `/def -P` with `L`/`R` parts is rejected with tf's own (quirky) message
  `-P-1: trigger has only N subexpressions`, as the reference build does.
* `mecho` shows `+TRIGGER: /name text` with a nesting prefix computed from
  the interpreter's call depth, which can differ by a level from tf's.
