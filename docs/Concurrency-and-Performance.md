# Concurrency and performance

## The rule

An `Interp` is **single-owner**: call its methods from one goroutine at a time.
Two interpreters in the same process are completely independent (shared
caches are internally locked), so one interpreter per user, session or script
scales out naturally.

## `Runner`: the actor

When several goroutines (a socket reader, a UI loop, timers, an HTTP handler)
all need the same interpreter, give it to a `Runner`. The Runner owns it in one
goroutine and everything else sends it work:

```go
run := gtfparser.NewRunner(in)
defer run.Close()

run.Do(func(in *gtfparser.Interp) { in.Run("/set hp=30") })          // wait for it
run.Post(func(in *gtfparser.Interp) { in.ProcessLine(line) })        // fire and forget
run.Interrupt()                                                      // from anywhere
```

- `Do` runs the function on the interpreter's goroutine and waits.
- `Post` queues it and returns immediately.
- `Interrupt` is lock-free and safe from any goroutine; use it to stop a
  runaway script.
- The Runner also runs `/repeat` and `/quote` timers: it sleeps until the next
  one is due and runs it on the same goroutine, so scripts never race with
  your code.
- After `Do`/`Post` begin, don't touch the `Interp` directly except `Interrupt`.

Your **host** callbacks (`Echo`, `Send`, `Notify`, your command handlers) run on
the Runner's goroutine. They must not block for long and must not call back into
`Do` on the same Runner (that would wait on itself). Hand work to another
goroutine with a channel instead.

## Without a Runner

Single-goroutine programs (a script runner, a REPL, a test) can drive the
interpreter directly. Drive timers yourself:

```go
for {
    next, ok := in.NextDue()      // when the earliest /repeat or /quote wants to run
    // ...wait until next (or until input arrives)...
    in.RunDue()                   // run everything that is due
}
```

## Performance

Speed comes from compiling the three things that repeat:

- **expressions** (`$[...]`, `/if (...)`, `/test`) are parsed once into a closure
  tree, cached per interpreter, then evaluated;
- **macro bodies** are split into statements once;
- **substitution text** (`%{x}`, `$[..]`, ...) is compiled once into a template of
  literal and substitution pieces.

The effect, measured against the C tf on the same scripts (startup time
subtracted; ratio < 1 means this engine is faster):

| Workload | gtf / tf |
|---|---|
| `/while` loop with `/let i=$[i+1]` | 0.6× |
| expression-heavy loop (`x := x + i*2 - i/3`) | 0.8× |
| macro calls as functions | 1.0× |
| string functions in a loop | 1.2× |

(2026-11, against tf 5.0 beta 8 on an Apple-silicon laptop; reproduce with
`SCALE=10 python3 scripts/bench.py`. Your numbers will differ; the ratios are
what matter.)

**Honest caveat:** dispatching received lines to triggers is the one place the
C implementation is currently faster; a faster trigger index is being worked on.
For a client that sees hundreds of lines a second this is irrelevant; for a
bulk log processor with thousands of triggers, measure with
`testdata/bench/trig.tf`.

### Tips

- Define macros once and call them; do not build `/def` text and `Run` it in a
  loop.
- Prefer `$[...]` expressions to chains of commands for arithmetic.
- Give triggers specific patterns; a trigger that matches everything runs on
  every line.
- Use `Limits` to bound worst cases (see [Safety](Safety-and-Sandboxing.md)).

### Memory

Caches hold compiled forms of recently used text (a few thousand entries per
interpreter, reset when full). A script that generates unbounded unique text
will churn the cache but not grow it without limit.
