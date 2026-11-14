# Core changes

Every change to the engine that a client could notice is logged here, newest
first, in the same commit that makes it. Clients pin a version of this module
and update on purpose (`go get github.com/itsnotyoutoday/gtfparser@vX.Y.Z`),
reading this file for what changed. A change is never required to be taken
mid-task.

Format: `## vX.Y.Z (date) — summary`, then `Added` / `Changed` / `Fixed` /
`Breaking` (and what a client must do).

## Unreleased

### core: tflib
Added package `tflib`: the real tf library (unmodified `lib/tf`, GPL, see `tflib/NOTICE.md`)
embedded with go:embed so every client ships the same one. `tflib.FS`, `Extract(dir)`,
`CacheDir()`, `Setup(in, dir)` (extracts and prepends to `LibDirs`), `Version()`. The engine
loads library files from disk, so clients extract to a cache directory. No change to existing packages.

### core: LimitFilter
Added `Interp.LimitFilter(world) func(line, attrs string) bool`: a goroutine-safe
snapshot of a world's /limit filter (nil when none), so a client can filter its
scrollback without calling into the interpreter from another goroutine.

## v0.0.5 (2026-11-14) — faithful triggers, hooks, macros and attributes

### core: trigger, hook, macro and attribute machinery rewritten after tf's macro.c
The macro database, trigger matcher, hook table and display attributes were
rewritten to follow tf 5 exactly (priority/fall-through selection, `-c`
probability, `%max_trig`/`%max_hook`, `-P` partial hilites, `/substitute`,
`-n` shots, `/list -s -u -S`, `/save`, `/def` option parsing). Docs:
`docs/engine/triggers-hooks.md`, `docs/engine/hooks.md`. Corpora: `attrs
defopts hooks hooks2 save shorthand triggers2 triggers3 used`.

Added
- `LineResult` (what `Process` returns) now also has `Line AttrString` (the
  line to display: text after `/substitute`, line and per-character attributes),
  `Original`, `NoLog`, `NoHistory`, `NoActivity`, `Bell`, `Substituted`,
  `Matched []*Macro`, `Ran []MacroRun`, `Count`, `ShotsUsed`, `Suppressed`;
  `Matches []Match` and `Gag` keep their meaning. `Match{Macro, Span, Groups}`
  is unchanged.
- `Interp.ProcessText(LineInput) *LineResult`: the full interface (incoming
  attributes, per-line world, `WorldOnly`/`GlobalOnly`, `Dry`). `Process(line,
  live)` is `ProcessText` for one plain line; `live` false is `Dry`: nothing
  runs and no shot is consumed, but the selected macros' attributes are applied.
- `Attr`/`AttrCtx`/`AttrString` (attrs.go): one attribute implementation.
  `Interp.ParseAttrs`, `ColorName`, `ColorNumber`, `Attr.String()`. Status
  lines, history/`/recall`/`/recordline`, `/log` and `/def -a` all use it.
  `Macro.Attr` is the parsed `-a` set; `Macro.Attrs` (text) and `Macro.Gag`
  remain as read-only mirrors.
- Match styles are a registry: `RegisterMatchStyle(name, MatchStyle)`, `CompilePattern`;
  `Macro.Trig`, `HArgs`, `WType` are `*Pattern`.
- Hook table: `HookEvents()`, `EnableHookEvents`, `FireHookArgs(event, args...)
  *HookResult`, `FireHookEvent(HookEvent)`, `AddHookFilter`, `HookDisplayHost`
  (clients place hook messages), default messages for each event.
  `HookResult{Count, Ran []*Macro, Message, Gag, Class, ...}`. `FireHook(event,
  args string) []*Macro` is unchanged for callers: it returns the hook macros
  selected (the ones that ran).
- `TriggerObserver` (`AddObserver`), `AddPreLineFilter`, `AddPostLineFilter`
  (veto or adjust a line), `SetRand`, `WorldInfoHost` (world type / existence
  for `-T` and `-w`), `AttrEchoHost` (attributed `/echo`, `echo()`, hook
  messages), `Interp.Macros()`, `FindMacro`, `FindBinding`, `ParseMacroSpec`,
  `RemoveWorldMacros`, `Macro.Used` (`/list -u`).
- `Define(*Macro)` returns the macro number (0 on failure) and accepts macros
  described by the text fields (`Trigger`, `Hook`, `Attrs`, `Gag`,
  `WorldType`) or by the parsed ones.
- `/trigger`, `/hilite`, `/gag`, `/trigpc`, `/hook`, `/purge`, `/list`,
  `/undefn` are real implementations; `/def -m` accepts every registered style.
  Unknown commands and plain-text sends fire the `NOMACRO` and `SEND` hooks.

Changed (now as in tf)
- Equal-priority non-fall-through macros that match a line: exactly one is
  chosen at random (`SetRand` makes it deterministic); fall-throughs all run.
- A negative `-p`, `-c` or `-n` means "not given" (the default applies).
- Macro names are case-insensitive; `#<number>` names a macro.
- Redefining a macro identically is a no-op returning the old number; `%redef`
  off refuses; the REDEF hook fires; a name that is a tf builtin fires CONFLICT.
- `/edit` reinstalls an edited copy of the macro under the same number and
  position (a `*Macro` held across an `/edit` is stale).
- `/echo -a/-p` without an `EchoHost` now renders through `AttrEchoHost` (or
  prints the text with `@{...}` decoded); an `EchoHost` still gets the raw
  options. `echo(text, attrs, inline, dest)` handles attributes.
- Display attributes: `f` and `d` (flash, dim) are accepted but not kept;
  `h` expands to `%hiliteattr`; `E`/`W`/`I` to `%error_attr`/`%warning_attr`/
  `%info_attr`; `/recall -ag` and history gag tests no longer misread the
  letters inside a colour name; the "invalid colour" error lists tf's valid
  values.
- `getopts()` treats a lone `-` as the end of the options; glob literals fold
  case for ASCII letters only (as tf's tolower).
- Hook events are validated: `FireHook` with a name that is not in `HookEvents()`
  warns. `LOG` and `REDEF world` messages come from the hook table.

Breaking (adapting a client)
- `Macro.Gag`, `Macro.Attrs`, `Macro.Trigger`, `Macro.Hook`, `Macro.WorldType`
  are mirrors filled in when the macro is installed. Set the real fields
  (`Attr` with `AttrGag`, `Trig`, `Hooks`/`HArgs`, `WType`) or define the macro
  with `Define` before reading; changing a mirror after `Define` has no effect.
  `Macro.OnlyInvis` is gone (it was an internal `/list` filter).
- `HookResult` (new, from `FireHookArgs`) has `Count` where the standalone
  hook engine had `Ran int`; `Ran` is the list of macros.

## v0.0.4 (2026-11-14) — what SquatClient needed

### core: line results and world context
Added `Interp.Process(line, live) LineResult` (matching macros with the byte
range of each match and of its regexp groups; `live=false` runs no body and
consumes no shot, for recomputing display effects of replayed history),
`Interp.SetWorld` (the world the interpreter is acting for, read back with `CurrentWorld()`; `SetWorld` is `SetCurrentWorld`; macros and
hooks defined with `-w` fire only for it; "" = any). `ProcessLine` is now
`Process(line, true).Gag`. The engine still does not interpret `-a`
attributes: `Match.Macro.Attrs` is handed to the client.

### core: /echo options reach the host
Added optional `EchoHost` (`EchoTo(EchoOpts{World, Attrs, Markup}, text)`).
`/echo -w<world> -a<attrs> -p` call it when the host implements it; every
other host (and captured/piped output) just gets `Echo(text)` as before.

### core: UndefID
Added `Interp.UndefID(id)`: remove a macro by its /list number (reaches
unnamed macros, which `Undef(name)` cannot). Lets a client implement `/undefn`.

### core: hook bodies expand when the hook fires inside a typed line
Fixed: `Hook` ran bodies with substitution off when called while a typed line
(which `Exec` leaves unexpanded) was executing, so a SEND hook saw a literal
`%*`. Hooks now always expand their body, like triggers.

### core: FireHook returns the macros that ran
Added `Interp.FireHook(event, args) []*Macro` (`Hook` is now a wrapper). A
client uses it to honour a gag attribute on a SEND hook (real tf then does not
send the text; lib/tf/alias.tf relies on it).

## v0.0.3 (2026-11-14) — history, worlds, logging and other state builtins

State that needs no terminal now lives in core (docs in `docs/engine/state-builtins.md`)
- History: `/recordline`, `/recall` (all range forms and options), `/histsize`; input,
  local, global and per-world histories fed by `RecordInput`, `RecordLocalLine`,
  `RecordWorldLine`; read back with `History`, `HistoryLines`, `RecallInput`.
- `/limit`, `/relimit`, `/unlimit` (`Limited`, `LimitActive`, a `limit` event).
- `/log` with `log_prefix`, `wraplog`, `ansi_log`; `LogLine`, `nlog()`; policy-checked.
- World database: `addworld()`, `/unworld`, `/renameworld`, `/listworlds`,
  `/saveworld`, `world_info()`, temporary worlds, `world_*` variables and
  `${world_name}`; `SetCurrentWorld`, `Worlds`, `DefineWorld`.
- Connections: `/connect`, `/dc`, `/fg`, `/listsockets`, `is_connected()`,
  `is_open()`, `fg_world()`, `idle()`, `sidle()`, `nactive()` over optional host
  interfaces (`SocketHost`, `ConnectHost`, `FgHost`, `IdleHost`, `RebootHost`,
  `LimitHost`); a host without `WorldHost` behaves as having no sockets.
- `/save`, `/edit`, `/undefn`, `/liststreams`, `/watchdog`, `/watchname`,
  `/features`, `/help`, `/core`, `/reboot`.
- `nactive()` uses sockets when the host has a `SocketHost`, else the paging state.

Known gaps: `/edit $text`; `%wraplog` breaks at blanks (tf uses ICU); `/features`
reports optional features off unless the client calls `SetFeature`.

## v0.0.2 (2026-11-14) — extension tables, parsed handlers, terminal builtins

Added
- `Extension`, `CommandSpec`, `FuncSpec` and `Interp.Load`: declare commands,
  functions, variables and disabled builtins as tables and load them in one
  validated call (argument-count checks, privilege checks via `Needs`, usage/help
  kept for generated help). `Interp.CommandSpecs` / `FuncSpecs` list them.
- `Interp.Run` and `Interp.Eval` (clear names for running a script / evaluating an
  expression), `HostFuncs` and `Console` hosts for small programs and tests.
- Command handlers as `Do func(*Call)`: options declared with `Options` (tf getopts
  syntax) are parsed before the handler runs; `Call` exposes `Has/Opt/Int/Arg/Rest/Args/Fail`
  and embeds the interpreter. `CommandSpec` also gains `Aliases` and `Category`.
  `gtfparser.Func(name, goFunc)` exposes any Go function as a script function with
  automatic argument conversion. `Interp.AddCommand` / `AddFunction` load one entry.
- Exported `Interp.Echo`, `Warnf`, `Send`, `Allow`, so command and function
  implementations outside the package can print, warn, send and check
  privileges.

Terminal, input and status (from the terminal builtins work; docs in
`docs/engine/terminal-builtins.md` and `terminal-hooks.md`)
- Input buffer (`in.Input()`, `TypeText`, `SubmitInput`, input history with
  `^old^new` substitution) and the whole `kb*()` function family and `/input`.
- `/dokey` with every documented name; editing operations run in core, display
  operations raise a `dokey` Notifier event. A `/dokey_<name>` macro overrides.
- Key dispatch for clients: `KeyPressed(bytes)`, `FlushKeys`, `PendingKeys`,
  `Bindings`, `FindKey`, `PrintToAscii`/`AsciiToPrint`; `keycode()` via `KeyHost`.
- Status line: `/status_add|edit|rm|clear`, `status_fields()`, `status_width()`,
  `in.StatusRows(width)` rendering rows with attribute spans; internal field
  values supplied by a `StatusHost` or the `%status_int_*` variables.
- `/beep`, `/localecho`, `/suspend`, `prompt()`, `fake_recv()`, paging state and
  functions (`morepaused`, `moresize`, `morescroll`, `winlines`, `nactive`).
- New optional host interfaces (in `terminal.go`): `MoreHost`, `WindowHost`,
  `LineHost`, `PromptHost`, `StatusHost`. New Notifier event kinds: `beep`,
  `localecho`, `suspend`, `dokey`, `status`, `more`, `prompt`, `var`.
- Variable-change hooks: `OnVarChange`, `WatchVar`, `PollVars`.

Known gaps carried with this change: `/bind` does not reject a binding that is
a prefix of another; `idle()`, `sidle()`, `nread()`, `nlog()`, `nmail()` await
sockets/log/mail owners.

## v0.0.1 (2026-11-14) — first extraction

The engine split out of the gtf client repo: interpreter, PCRE2-compatible
regexp engine, per-interpreter command/function registries, host capability
interfaces (WorldHost, Notifier, KeyHost, TermHost, HelpHost), Policy and
/restrict, Limits and Interrupt, the Runner actor.
