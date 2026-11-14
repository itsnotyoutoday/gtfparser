# Core changes

Every change to the engine that a client could notice is logged here, newest
first, in the same commit that makes it. Clients pin a version of this module
and update on purpose (`go get github.com/itsnotyoutoday/gtfparser@vX.Y.Z`),
reading this file for what changed. A change is never required to be taken
mid-task.

Format: `## vX.Y.Z (date) — summary`, then `Added` / `Changed` / `Fixed` /
`Breaking` (and what a client must do).

## Unreleased

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

## v0.0.1 (2026-11-14) — first extraction

The engine split out of the gtf client repo: interpreter, PCRE2-compatible
regexp engine, per-interpreter command/function registries, host capability
interfaces (WorldHost, Notifier, KeyHost, TermHost, HelpHost), Policy and
/restrict, Limits and Interrupt, the Runner actor.
