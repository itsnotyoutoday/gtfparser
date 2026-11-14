# Core changes

Every change to the engine that a client could notice is logged here, newest
first, in the same commit that makes it. Clients pin a version of this module
and update on purpose (`go get github.com/itsnotyoutoday/gtfparser@vX.Y.Z`),
reading this file for what changed. A change is never required to be taken
mid-task.

Format: `## vX.Y.Z (date) — summary`, then `Added` / `Changed` / `Fixed` /
`Breaking` (and what a client must do).

## Unreleased

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
