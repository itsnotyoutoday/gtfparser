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

The smallest host just prints what scripts echo and accepts what they send:

    type host struct{}
    func (host) Echo(s string)         { fmt.Println(s) }
    func (host) Send(w, s string) bool { return true }

    in := gtfparser.New(host{})
    in.LoadFile("stdlib.tf")       // TinyFugue's own library runs unmodified
    in.ExecFull("/echo $[1 + 2]")  // 3

An `Interp` belongs to one goroutine. When sockets, a UI and timers all need
it, wrap it in a `Runner` (an actor): `run.Do(func(in *gtfparser.Interp) { ... })`.

## Extending the engine: load a table

A client adds its own commands, functions and variables by declaring them as
**data** and loading the table in one call. The engine validates the whole
table first (nothing is installed if any entry is bad), checks argument counts
and privileges before your function runs, and keeps the usage and help text so
help listings can be generated from the table.

    var toolkit = gtfparser.Extension{
        Name: "toolkit",
        Commands: []gtfparser.CommandSpec{
            {Name: "greet", Fn: cmdGreet, Usage: "<name>", Help: "say hello", MinArgs: 1},
            {Name: "roll",  Fn: cmdRoll,  Usage: "[sides]", Help: "roll a die", MaxArgs: 1},
            {Name: "pwd",   Fn: cmdPwd,   Help: "show the directory",
                Needs: []gtfparser.Op{gtfparser.OpFileRead}},   // refused under /restrict file
        },
        Functions: []gtfparser.FuncSpec{
            {Name: "clamp", Fn: fnClamp, Usage: "value, low, high", MinArgs: 3, MaxArgs: 3},
        },
        Vars:            map[string]string{"greeting": "Hello"},
        DisableCommands: []string{"sh"},   // keep scripts away from the shell
    }

    func cmdGreet(in *gtfparser.Interp, args string) gtfparser.Value {
        in.Echo(in.Var("greeting") + ", " + strings.TrimSpace(args) + "!")
        return gtfparser.Int(1)
    }

    if err := in.Load(toolkit); err != nil { log.Fatal(err) }

Then, from a script: `/greet world` prints `Hello, world!`, `$[clamp(15, 0, 10)]`
is `10`, and `/greet` alone prints `% usage: /greet <name>`.

Tables layer: load several in order and a later entry overrides an earlier one,
including a built-in (to wrap the original, fetch it first with
`in.Command("echo")`). `in.CommandSpecs()` and `in.FuncSpecs()` return what the
tables declared, sorted, for building `/help` output. A runnable version of the
example above is in `example_test.go`.

Handlers talk to the user and the world with `in.Echo`, `in.Warnf`,
`in.Send` and `in.Allow` (the privilege check, for handlers that do something
privileged and don't declare it in `Needs`).

### Other extension points

| Need | Use |
|---|---|
| One-off command or function | `RegisterCommand` / `RegisterFunction` |
| Turn a builtin off for scripts | `DisableCommand` / `DisableFunction`, or `DisableCommands` in a table |
| Connections, status, keys, help, terminal size | optional host interfaces: `WorldHost`, `Notifier`, `KeyHost`, `HelpHost`, `TermHost` |
| Veto shell, file or world access | `WithPolicy(...)`, `WithRestriction(...)` (and the script-level `/restrict`) |
| Resource ceilings, stopping a runaway script | `WithLimits(...)`, `in.Interrupt()` |

A builtin that real tf has and that is not implemented fails loudly ("not
supported by this client") instead of silently succeeding.

## Testing against real tf

    export TFREF_DIR=/path/to/built/tinyfugue   # a UTF-8 build; contains src/tf and lib/tf
    . scripts/env.sh
    go build -o /tmp/gtfrun ./cmd/gtfrun
    python3 scripts/difftest.py testdata/cases/*.tf
    GTF_TF_LIB=$TF_LIB go test .           # whole-library macro table vs real tf

License: GPL-3.0-or-later (TinyFugue is GPL-2.0-or-later).
