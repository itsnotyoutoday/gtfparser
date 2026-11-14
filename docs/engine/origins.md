# Origins: restriction by who wrote the code

`/restrict` is one level for the whole `Interp`. A host that runs code from
several parties (the user at the keyboard, a remote peer attached to the
client, a downloaded script) needs one level per party: raising the global
level for a peer would lock the owner out too. An **origin** is an opaque
string chosen by the host (`"peer:laptop"`) that stands for one party. Each
origin gets its own level; code run under it, and everything that code leaves
behind, stays under that level.

An origin with no configured level is unrestricted. With no origin configured
or running, nothing in this page applies and nothing about the interpreter's
behaviour, output or saved state changes.

Files: `origin.go` (API, state section), the hooks in `callMacro` (interp.go),
`runBody` (triggers.go), `stepProc` (proc.go), `completeMacro` (macro.go),
`findStream` (streams.go), `allow` (policy.go). Tests: `origin_test.go`.

## Go API

| Call | Meaning |
|---|---|
| `in.SetOriginRestriction(origin, level) bool` | give `origin` a level (`RestrictShell`, `RestrictFile`, `RestrictWorld`); `RestrictNone` removes it. Unlike `/restrict` this may lower a level: it is the host's policy. Refused (false) for an empty origin or a bad level, and refused when it would *lower or remove* a level while any origin's code is running |
| `in.OriginRestriction(origin) int` | the configured level (0 if none) |
| `in.OriginRestrictions() map[string]int` | copy of all of them |
| `in.ExecAs(origin, line)` | `Exec` (a typed or loaded line) under `origin` |
| `in.RunAs(origin, script)` | `Run` (full substitution, like `/eval`) under `origin` |
| `in.EvalAs(origin, expr)` | `Eval` under `origin` |
| `in.DoAs(origin, func())` | any Go code under `origin` (for `LoadFile`, `Process`, `FireHook`, ...) |
| `in.EffectiveRestriction() int` | the level in force now (see below) |
| `in.CurrentOrigin() string` | the origin new definitions are tagged with now; `""` for the host |
| `(*Macro).Origin() string` | who defined the macro |
| `in.Restriction()` / `SetRestriction` | unchanged: the **global** `/restrict` level |

```go
in := gtfparser.New(host)
in.SetOriginRestriction("peer:alice", gtfparser.RestrictFile) // no shell, no files

// a line the peer typed, or sent as tf.exec:
in.ExecAs("peer:alice", line)

// the peer defined a trigger earlier: it runs under peer:alice whenever
// server text fires it, because the macro carries the tag. Nothing to do.
in.ProcessLine(serverLine)

// the owner trusts alice after all (from the host's own code, not from a script):
in.SetOriginRestriction("peer:alice", gtfparser.RestrictNone)
```

With a `Runner`, call these inside `r.Do(func(in *gtfparser.Interp) { ... })`
like every other `Interp` call.

## Semantics

**Effective level.** Every guarded operation checks
`max(global /restrict level, level of every origin whose code is running)`.
*Strictest wins*; nothing can loosen it. The origins "whose code is running"
are those of: each `ExecAs`/`RunAs`/`DoAs` on the call chain, each running
macro that is tagged (called by name, as a function, from a key binding,
fired as a trigger or hook, called by another macro), and the process
(`/repeat`, `/quote`) being stepped. They stack: a restricted macro calling a
host macro keeps running restricted, and a stricter macro called from a looser
one makes the rest of that call stricter.

**Tags (provenance).** A macro created while an origin's code runs is tagged
with an origin: `/def`, `/trigger`-style shorthands (`/trig`, `/hilite`,
`/gag`), `/bind`, hooks, and `/edit` all go through one installer. A
`/repeat` or `/quote` process is tagged when it starts and keeps the tag after
the macro that started it has ended. An open stream is tagged by its opener.
When several origins are running at once the tag is the one with the *highest
configured level* (ties: innermost), so a strict origin cannot launder code
through a looser one's macro.

* Redefining a macro (`/def` of an existing name) gives the new macro the tag
  of whoever redefined it; an "identical" redefinition from another origin is
  not a no-op (the tag differs).
* `/edit`: changing the body (or `-E`) makes the macro the editor's code. An
  edit that leaves the body alone keeps the tag, except that a host macro
  edited from inside an origin becomes that origin's.
* `/undef` and redefine: a fresh macro, tagged by the new definer.
* The tag is visible to scripts only through `macro_origin(name)` (returns the
  origin, `""` for the host's own, `0` if there is no such macro) and
  `current_origin()`. Both are extensions (tf has neither); a script's own
  macro of that name takes precedence. `/list`, `/save` and every other output
  are unchanged.
* The `-E` condition of a trigger is code: it is evaluated under the macro's
  origin.

**/restrict inside an origin.** `/restrict` run by code under an origin can
raise the level of *that origin* (and every other origin on the chain); it
never raises the global level (a peer must not lock the owner out), and it can
never lower anything (`/restrict: restriction level cannot be lowered`).
`/restrict` with no argument reports the effective level. A script cannot
reach `SetOriginRestriction`; a host command that does is refused when it
would lower a level while origin code runs, so a peer cannot lift its own
restriction through a host command either.

**Lifting.** `SetOriginRestriction(origin, RestrictNone)` takes effect
immediately for every macro, process and stream already carrying that tag
(the tag is a name, the level is looked up when the code runs).

**Streams.** A stream handle is a capability. Code running under an origin
whose level would refuse opening a file (`FILE`) or a pipe (`SHELL`) may not
use such a stream opened by anyone else (the host or another origin); it gets
`restricted`. It may use queue streams, and streams its own origin opened.
Without an origin level every stream is usable, as in tf.

**Stored expressions.** Status-field expressions (`status_int_*`,
`status_var_*` variables) are stored by whoever set the variable and run by the
host on every redraw, so they cannot be attributed. When at least one origin
has a level, they are evaluated as if at `RestrictWorld` (no shell, no files,
no connections). With no origins nothing changes.

**State.** `SaveState` writes the levels and the tags of macros, processes and
streams in an `origins` section (version 1, *required*: an older engine that
does not know it refuses the state instead of silently turning a peer's macros
into the owner's). The section is omitted when empty, so states of sessions
that never used origins are byte-identical to before. `LoadState` merges levels
by maximum (a load never lowers a level, like `/restrict`).

## What is guarded (the audit)

Everything that reaches the shell, the file system or the network consults the
effective level. `origin_test.go` (`TestOriginAudit`) runs each row under a
restricted origin at the refusing level (it must print `restricted` and have no
effect), one level below, and with no level (it must work). Wording is tf's
(`restricted`), the command name is upper-cased where the existing code does so.

| Operation | Refused at | Message |
|---|---|---|
| `/sh` | SHELL | `SH: restricted` |
| `/quote !cmd` (async and `-S`) | SHELL | `QUOTE: restricted` |
| `tfopen(cmd, "p")`, `tfopen(cmd, "P")` (pipes) | SHELL | `tfopen: restricted` |
| anything that goes through those: `/eval /sh`, `/repeat -S /sh`, a macro or alias that runs `/sh`, `/sys` (the stdlib macro), key bindings, triggers, hooks, timers | SHELL | as above |
| `/load`, `/require` | FILE | `LOAD: restricted` |
| `/log` | FILE | `LOG: restricted` |
| `/save` | FILE | `SAVE: restricted` |
| `/saveworld` | FILE | `SAVEWORLD: restricted` |
| `/lcd` | FILE | `LCD: restricted` |
| `/quote 'file` (async and `-S`) | FILE | `QUOTE: files restricted` |
| `tfopen(file, "r"/"w"/"a")` | FILE | `tfopen: restricted` |
| `fwrite()` | FILE | `fwrite: restricted` |
| use of a file/pipe stream someone else opened (`tfread`, `tfwrite`, `tfflush`, `tfclose`, ...) | FILE / SHELL | `restricted` (new, see below) |
| `addworld()` / `/addworld` | WORLD | `addworld: restricted` |
| `/connect host port` | WORLD | `CONNECT: restricted` then `CONNECT: arbitrary connections restricted` (as before) |
| host commands and functions registered with `Needs` | by op | `<name>: restricted` |
| `Policy.Allow` | n/a | still consulted after the level check; `in.CurrentOrigin()` tells a `Policy` who is asking |

Operations that already went through `in.allow` (everything above except the
stream rule) did not change: `allow` and the two inline checks
(`/quote '` and `tfopen`) now read the effective level instead of the global
one, which is the same number when no origin is involved.

Not guarded, deliberately (tf does not guard them either, and none of them can
reach outside the interpreter on a script's behalf):

* The process environment is imported into script variables when the `Interp`
  is created, so `%{HOME}` and friends are ordinary variables; an origin
  cannot be kept from reading them. Create the `Interp` with
  `WithoutEnvironment()` if the environment is sensitive. (`/sh` children get
  the environment only through the shell guard.)
* `nmail()` and the mail check `stat` the files named by `MAIL`/`TFMAILPATH`:
  existence, size and time only, run by the host. A peer that can set `MAIL`
  can probe a path this way.
* `/connect worldname` and `/fg` of worlds that already exist; `/unworld`,
  `/renameworld`, `/listworlds`: they work on the world database, as in tf.
* `/def` of a name the owner uses replaces the owner's macro (tf's `/redef`
  rules); the replacement carries the peer's tag, so it only runs restricted.
  Hosts that want protected macros set `/set redef=off`.
* `/kill` of the owner's processes, `/undef` of the owner's macros.

Found unguarded before and guarded now: using an already-open file or pipe
stream by handle (`tfwrite(fd, ...)` to a host's pipe is a shell by proxy), and
the `-E` condition and status-field expressions that the host ran on behalf of
whoever stored them. Neither can matter without origins (they act with the
global level only, as before).

## Decisions a host makes

* Which level a peer gets. `RestrictFile` is the useful default for a remote
  peer: triggers, hooks, timers and macros work, shell and files do not.
  `RestrictWorld` also stops it opening connections.
* Whether peers may redefine the owner's macros (`/set redef=off`, or a
  `Policy`/`Needs` of your own).
* Lifting: only from Go, only from outside origin code.
