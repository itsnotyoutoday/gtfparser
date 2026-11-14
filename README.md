# gtfparser

**A TinyFugue scripting engine for Go.** The language, its runtime, and a
PCRE2-compatible regexp engine, as a library you can drop into any program:
a terminal MU* client, a desktop app, a relay daemon, a test rig.

It runs TinyFugue's own library (`stdlib.tf` and friends) unmodified, and is
checked line by line against the real `tf` binary.

    go get github.com/itsnotyoutoday/gtfparser

## Thirty seconds

```go
in := gtfparser.New(gtfparser.Console(os.Stdout))

in.Run("/echo hello from tf")           // run commands
v, _ := in.Eval("strlen('abc') * 2")    // evaluate expressions
fmt.Println(v)                          // 6

in.Run("/set score=40")                 // scripts and Go share variables
in.SetVar("bonus", "2")
in.Run("/echo score is $[score + bonus]")   // score is 42
```

`Run` executes text the way a macro body runs: commands, `%{vars}`, `$[exprs]`,
`%;` separators, plain text sent to the current world. `Eval` evaluates an
expression. That is most of the API you need to start.

## Macros, triggers and the game's side of the conversation

Definitions are ordinary TinyFugue. Anything that works in tf works here.

```go
in.Exec("/def square = /result {1} * {1}")
in.Exec("/def -t'* tells you: *' reply = say %{-3}")      // a trigger

in.Run("/echo 7 squared is $[square(7)]")                 // 7 squared is 49
in.ProcessLine("Bob tells you: nice to see you")          // a line arrives from the game
//  -> say nice to see you
```

`Exec` takes a line the way a script file does (no substitution until the macro
runs); `Run` takes one the way a macro body does.

## Your program is the host

An interpreter talks to its surroundings through a `Host`: somewhere to show
text, somewhere to send lines. For a quick start use `Console`, or adapt two
functions:

```go
in := gtfparser.New(gtfparser.HostFuncs{
    OnEcho: func(text string) { ui.Append(text) },
    OnSend: func(world, text string) bool { return conn.Write(world, text) },
})
```

Everything else a client may or may not provide is an **optional interface**
the host implements when it can: `WorldHost` (connections), `Notifier`
(beeps, status, limits...), `KeyHost` and `TermHost` (terminal facts),
`HelpHost`. A feature whose capability is missing says so
(`% /connect: not supported by this client`) instead of pretending to work.

## Extend it with a table

Commands, functions, variables and disabled builtins are declared as **data**
and loaded in one call. The whole table is validated first (nothing is
installed if any entry is bad), argument counts and privileges are checked
before your function runs, and usage/help text is kept so `/help` can be
generated from it.

```go
var toolkit = gtfparser.Extension{
    Name: "toolkit",
    Commands: []gtfparser.CommandSpec{
        {Name: "greet", Fn: cmdGreet, Usage: "<name>", Help: "say hello", MinArgs: 1},
        {Name: "roll",  Fn: cmdRoll,  Usage: "[sides]", Help: "roll a die", MaxArgs: 1},
        {Name: "pwd",   Fn: cmdPwd,   Help: "show the directory",
            Needs: []gtfparser.Op{gtfparser.OpFileRead}},      // refused under /restrict file
    },
    Functions: []gtfparser.FuncSpec{
        {Name: "clamp", Fn: fnClamp, Usage: "value, low, high", MinArgs: 3, MaxArgs: 3},
    },
    Vars:            map[string]string{"greeting": "Hello"},
    DisableCommands: []string{"sh"},                           // keep scripts off the shell
}

func cmdGreet(in *gtfparser.Interp, args string) gtfparser.Value {
    in.Echo(in.Var("greeting") + ", " + strings.TrimSpace(args) + "!")
    return gtfparser.Int(1)
}

in.Load(toolkit)
```

```
/greet world                      Hello, world!
/echo $[clamp(15, 0, 10)]         10
/greet                            % usage: /greet <name>
```

Tables layer: load several in order and a later entry overrides an earlier one,
built-ins included (fetch the original first with `in.Command("echo")` to wrap
it). `in.CommandSpecs()` returns what was declared, for building help. Handlers
use `in.Echo`, `in.Warnf`, `in.Send` and `in.Allow`. A small one-off needs no
table: `in.RegisterCommand("name", fn)`.

## Use it from many goroutines

An `Interp` belongs to one goroutine. When sockets, a UI and timers all want
it, give it to a `Runner`, which owns it in one goroutine (and runs `/repeat`
and `/quote` timers for you):

```go
run := gtfparser.NewRunner(in)
defer run.Close()

run.Do(func(in *gtfparser.Interp) { in.Run("/set hp=30") })    // from any goroutine
run.Post(func(in *gtfparser.Interp) { in.ProcessLine(line) })  // fire and forget
run.Interrupt()                                                // stop a runaway script
```

## Make it safe for other people's scripts

```go
in := gtfparser.New(host,
    gtfparser.WithPolicy(myRules),                              // veto shell/file/world access
    gtfparser.WithRestriction(gtfparser.RestrictShell),         // tf's own /restrict, from the start
    gtfparser.WithLimits(gtfparser.Limits{MaxInstr: 1_000_000, MaxRecur: 100}))

in.Run("/sh rm -rf /")      // % /sh: restricted
in.Run("/spin")             // % max_instr (1000000) exceeded; execution aborted
```

Limits are ceilings: a script may lower `max_instr` but never raise it past
yours.

## Compatible, and proven so

The test suite runs scripts through the real `tf` and through this engine and
compares the results: expressions, substitution, control flow, scoping, globs,
triggers, pipes, special variables, processes, and a golden check that loading
TinyFugue's whole library defines the exact same macro table. The regexp
engine (`pcre`) is checked against real PCRE2.

```sh
export TFREF_DIR=/path/to/built/tinyfugue    # a UTF-8 build: src/tf, lib/tf
. scripts/env.sh
go build -o /tmp/gtfrun ./cmd/gtfrun
python3 scripts/difftest.py testdata/cases/*.tf
GTF_TF_LIB=$TF_LIB go test .
```

## Layout

| Path | |
|---|---|
| `.` (package `gtfparser`) | the interpreter and runtime |
| `pcre/` | PCRE2-compatible regexp engine |
| `cmd/gtfrun` | batch runner used by the differential tests |
| `scripts/`, `testdata/` | differential harness, corpus, benchmarks |

Every example above runs in the test suite (`easy_test.go`,
`example_test.go`). Changes that clients can see are logged in
[CORE-CHANGES.md](CORE-CHANGES.md); see [CONTRIBUTING.md](CONTRIBUTING.md) before
changing the engine.

GPL-3.0-or-later (TinyFugue is GPL-2.0-or-later).
