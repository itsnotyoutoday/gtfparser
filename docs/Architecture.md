# Architecture

A tour of how the engine is put together, for contributors and for the curious.

## Packages

| Path | Role |
|---|---|
| `gtfparser` (repo root) | the interpreter and runtime: values, expressions, substitution, macros, triggers, builtins, host interfaces, policy, limits, runner |
| `pcre` | a backtracking regexp engine with PCRE2 semantics (UTF-8, Unicode properties, lookaround, backreferences, atomic groups) |
| `cmd/gtfrun` | batch script runner for the differential tests and for documentation checks |
| `scripts/`, `testdata/` | the differential harness, corpus and benchmarks |

The engine has **no dependencies** beyond the Go standard library, knows nothing
about terminals or networks, and has no process-global mutable state (the only
shared state is small, internally locked caches), so any number of interpreters
can live in one process.

## From text to action

```
 text ──► Exec (script-file line)  ──┐
          Run  (macro-body text)  ───┼─► statement list ──► for each statement:
          ProcessLine (from world) ──┘     (cached split)       │
                                                                ├─ /if /while  → control flow (nests)
                                                                ├─ substitution (cached template)
                                                                │     %{x} %n  $[expr] $(cmd) ${m}
                                                                ├─ /command → macro? builtin? (registry)
                                                                └─ plain text → Host.Send
```

- **Two entry semantics.** `Exec` models a script-file line (no substitution
  until a macro runs); `Run` models a macro body (full substitution). This split
  mirrors tf exactly and is the root of most tf behavior.
- **Statements.** A body is split at `%;`/`%|` once and cached. `/if`/`/while`
  collect their bodies from the following statements.
- **Substitution.** Text is compiled once into a template of literals and
  substitution segments; running the template reads variables and evaluates
  embedded expressions.
- **Expressions.** Parsed once into a closure tree and cached; evaluation walks
  the closures. Values are a small tagged union: int, decimal (fixed point),
  float, string (plus enumerated variables).
- **Scope.** Each macro call pushes a *frame* holding local variables and
  positional parameters; lookups walk outward (dynamic scope) and fall back to
  globals. Frames also carry regexp captures (`%P1`) and tfout state.
- **Statement budget.** Every statement ticks a counter checked against
  `max_instr`; an interrupt flag is checked at the same point.

## Registries

Each interpreter owns its tables of builtin commands and functions, copied from
package defaults at creation, so one interpreter can add, override or disable
things without affecting another. Anything real tf has that is not implemented
is registered as a *placeholder* that fails loudly, so nothing silently
no-ops.

`Extension` tables and `Func` binders are layers over the same registries: they
validate, wrap handlers with argument and privilege checks, and call
`RegisterCommand` / `RegisterFunction`.

## The host boundary

```
 your program ◄────────────────────────► engine
   Host{Echo, Send}        output/side effects
   optional: WorldHost, Notifier, KeyHost, TermHost, HelpHost, ...
   Policy, Limits          your rules about what scripts may do
   Runner                  one goroutine owns the Interp
```

The engine never writes to a terminal or opens a socket. It *asks* the host,
through interfaces it checks for at the moment of use. That is why one engine
serves a terminal client, a desktop app and a daemon.

## Triggers, hooks and timers

- **Triggers** are macros with a pattern (`-t`); `ProcessLine` finds the
  matching ones by priority and runs them.
- **Hooks** are macros attached to named events (`-h`); the host fires events.
- **Processes** (`/repeat`, `/quote`) live on the interpreter's own schedule;
  `NextDue`/`RunDue` (or a `Runner`) drive them, so timers need no extra
  goroutines of their own.

## Regexps

`pcre` compiles a pattern into a node program and matches by backtracking with
a step limit. `CompileTF` applies tf's option set (dot matches newline, `$`
only at the very end, UTF-8 + Unicode properties, smart case). The interpreter
reaches it through a single seam, so the engine can be swapped without touching
the language code.

## How correctness is kept

See [Testing and compatibility](Testing-and-Compatibility.md): a differential
corpus against the real tf binary, a golden check of the whole standard library,
a regexp conformance suite, executable documentation, and benchmarks.

## Contributing

Read [CONTRIBUTING.md](../CONTRIBUTING.md). In short: real tf is the spec; add a
differential case with every behavior change; keep API changes additive and log
them in [CORE-CHANGES.md](../CORE-CHANGES.md).
