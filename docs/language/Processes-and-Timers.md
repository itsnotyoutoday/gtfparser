# Processes and timers

tf has no `sleep` and no threads. Delayed and repeated work is done with
*processes*: `/repeat` runs a command repeatedly or after a delay, and `/quote`
feeds it lines from a file or another command. A process is just a scheduled
job. This page covers what the scripts see and how an embedding program makes
time pass.

## `/repeat`

```
/repeat [-S | -P | -time] [-n] count command
```

Runs `command` `count` times. `count` may be `i` for "forever". The command is a
macro body, expanded each time it runs (so use `%%` for anything that must be
substituted at run time rather than when you type the `/repeat`).

| Option | Meaning |
|---|---|
| `-S` | run **synchronously**: all repetitions happen right now, before the next command |
| `-time` | wait `time` between runs: seconds (`-2`, `-0.5`) or `h:mm:ss` (`-0:30`) |
| `-P` | run when a prompt arrives from the world |
| *(none)* | wait `%ptime` seconds (default 1) between runs |
| `-n` | with `-time`, do the first run immediately |

```tf
/repeat -S 3 /echo ping
/def tick = /echo tick %1
/repeat -S 2 /tick now
```
```output
ping
ping
ping
tick now
tick now
```

An asynchronous `/repeat` returns the process id and runs *later*. The first run
comes after the first delay, unless you give `-n`. That makes
`/repeat -5 1 command` the idiom for "do this once, in five seconds". The tf
manual notes there is no sensible way to sleep inside a macro: instead, write the
rest of the work as a macro and schedule it.

```tf
/repeat -30 2 /echo later
/echo started process %?
/ps -s
```
```output
started process 1
1
```

(`/ps -s` lists just the ids; plain `/ps` also shows the time to the next run,
type, count and command.) `/kill id...` ends a process.

```tf
/repeat -60 i /echo forever
/ps -s
/kill 1
/echo killed: %?
/ps -s
```
```output
1
killed: 1
```

Pids are small integers that start at 1 in each interpreter.

> A command scheduled with `/repeat` *without* `-S` does not run before the next
> command in the same macro, even with `-0`. If you need it to run first, use
> `-S`.

## `/quote`

`/quote` generates lines of text and then echoes, sends, or executes each one.
The source is introduced by a character:

| Form | Source of lines |
|---|---|
| `/quote 'file` | each line of a file |
| `/quote !command` | output of a shell command (needs shell access) |
| `` /quote `command `` | echoed output of a tf command |

and `-d` chooses the disposition: `-decho` shows the lines, `-dsend` sends them
to the world (the default when there is no prefix), and `-dexec` runs each as a
command (the default when there *is* a prefix). Text before the source is a
prefix added to every line.

```tf
/test fo := tfopen('/tmp/gtfdoc_q1.txt', 'w')
/test tfwrite(fo, 'north')
/test tfwrite(fo, 'east')
/test tfclose(fo)
/quote -S -decho '/tmp/gtfdoc_q1.txt
/quote -S -decho go '/tmp/gtfdoc_q1.txt
/quote -S -dsend '/tmp/gtfdoc_q1.txt
/quote -S -dexec /echo walking `/echo hello
```
```output
north
east
go north
go east
[send] north
[send] east
walking hello
```

Without `-S`, a `/quote` produces one line per `%ptime` (or per `-time`), which
is how you pace commands to a game. With `-P` each line waits for a prompt, so
the next command is sent only after the game answered the last one.
Text from `/quote` is **not** substituted again; the body of a `/repeat` is.

## How the embedding program drives time

Asynchronous processes do nothing by themselves: the interpreter has no clock
thread. The host calls it. There are two ways.

**A `Runner` does it for you.** `NewRunner(in)` owns the interpreter in one
goroutine, runs its timers, and gives you thread-safe `Do` and `Post`. This is
what most programs want.

```go
run := gtfparser.NewRunner(in)
defer run.Close()
run.Post(func(in *gtfparser.Interp) { in.Run("/repeat -5 1 /echo five seconds later") })
```

**Or run it yourself.** `in.NextDue()` reports when the earliest timed process
wants to run, and `in.RunDue()` runs everything that is due. Call `RunDue` from
your event loop when the time arrives. To handle `-P` processes, call
`in.Prompt()` whenever a prompt arrives from the world. The interpreter's
clock is `in.Now` (set it to a fake clock in tests):

```go
in := gtfparser.New(gtfparser.Console(os.Stdout))
clock := time.Unix(1000, 0)
in.Now = func() time.Time { return clock }
in.Run("/repeat -2 3 /echo tick")
for {
    due, ok := in.NextDue()
    if !ok {
        break
    }
    clock = due // a real program waits until `due`
    in.RunDue()
}
```

That loop prints `tick` three times, two (fake) seconds apart, then finds no
more processes. (This program was run to produce that description.)

## Pitfalls

* **Expecting `/repeat` to block.** Only `-S` does. Everything else returns
  immediately.
* **Forgetting that the delay comes first.** `/repeat -5 1 cmd` runs once, after
  five seconds.
* **Substituting at the wrong time.** The command text is expanded when it
  runs: use `%%x` in a macro body for a variable you want read at run time.
* **A `/repeat i` that nothing stops.** Keep the pid (it is the return value) or
  use `/ps` and `/kill`.
* **No `RunDue`.** If the host never calls it (or has no `Runner`), timed
  processes never fire.

## See also

[Files and streams](Files-and-Streams.md) ·
[Macros](Macros.md) ·
[Evaluation model](Evaluation-Model.md)
