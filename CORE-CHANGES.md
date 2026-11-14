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
- Exported `Interp.Echo`, `Warnf`, `Send`, `Allow`, so command and function
  implementations outside the package can print, warn, send and check
  privileges.

### core: line results and world context
Added `Interp.Process(line, live) LineResult` (matching macros with the byte
range of each match and of its regexp groups; `live=false` runs no body and
consumes no shot, for recomputing display effects of replayed history),
`Interp.SetWorld/World` (the world the interpreter is acting for; macros and
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

## v0.0.1 (2026-11-14) — first extraction

The engine split out of the gtf client repo: interpreter, PCRE2-compatible
regexp engine, per-interpreter command/function registries, host capability
interfaces (WorldHost, Notifier, KeyHost, TermHost, HelpHost), Policy and
/restrict, Limits and Interrupt, the Runner actor.
