# gtfparser

A TinyFugue scripting engine for Go: the language, its runtime and a
PCRE2-compatible regex engine, as an embeddable library. It is the shared core
of two programs:

- **gtf**, a terminal MU* client (separate repository)
- **SquatClient / squatterd** (separate repositories)

Neither client owns it, and neither may fork it. A client extends the engine
through its public extension points; anything a client needs that the engine
lacks is added here, generically.

## Packages

| Path | What |
|---|---|
| `gtfparser` (repo root) | the interpreter: expressions, `%`/`$` substitution, macros, triggers, hooks, processes, `/load`, builtin commands and functions |
| `pcre` | PCRE2-compatible backtracking regexp engine, verified against real tf and PCRE2 10.47 |
| `cmd/gtfrun` | batch runner used by the differential tests (not a client) |
| `scripts/` | differential harness (`difftest.py`), benchmarks, PCRE tooling |
| `testdata/` | differential corpus, golden macro table, benchmarks |

## Embedding

    in := gtfparser.New(host,                       // host implements Echo/Send
        gtfparser.WithLimits(gtfparser.Limits{MaxInstr: 1_000_000}),
        gtfparser.WithPolicy(myPolicy),             // veto shell/file/world access
        gtfparser.WithRestriction(gtfparser.RestrictShell))
    in.RegisterCommand("mycmd", func(in *gtfparser.Interp, args string) gtfparser.Value { ... })
    run := gtfparser.NewRunner(in)                  // an Interp is single-owner: use the Runner
    run.Do(func(in *gtfparser.Interp) { in.LoadFile("stdlib.tf") })

Extension points: `RegisterCommand` / `RegisterFunction` / `DisableCommand`,
optional host interfaces (`WorldHost`, `Notifier`, `KeyHost`, `TermHost`,
`HelpHost`), `Policy`, `Limits`, `Interrupt()`. A builtin that real tf has and
that is not implemented fails loudly ("not supported by this client").

## Testing against real tf

    . scripts/env.sh                       # points at a reference tf build
    go build -o /tmp/gtfrun ./cmd/gtfrun
    python3 scripts/difftest.py testdata/cases/*.tf
    GTF_TF_LIB=$TF_LIB go test .           # whole-library macro table vs real tf

License: GPL-3.0-or-later (TinyFugue is GPL-2.0-or-later).
