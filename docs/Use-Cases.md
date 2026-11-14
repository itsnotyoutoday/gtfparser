# Use cases

tf was written to automate MUD clients, so it is excellent at one thing:
**reacting to lines of text**, with patterns, state, timers and a compact
syntax non-programmers can pick up. That makes it useful far beyond its
original home. Here are the shapes it fits, with a sketch of each.

## 1. A MUD / MU* client (what it was made for)

Users write triggers, aliases and macros against the game's output. Your client
feeds lines in and renders what scripts echo and send.

```tf
/def -t'You are hungry.' eat = /send eat bread
/def -mregexp -t'^(\w+) tells you: (.*)$' tell = /echo [tell] %{P1}: %{P2}
```

Your client adds layout and display commands through
[tables](Extending-the-Engine.md), and the existing library of tf scripts
people have written for decades just runs.

## 2. User-scriptable game servers

Let players or builders write automation that runs **on your server**: item
reactions, NPC behavior, quest triggers. Use [`/restrict` and a
policy](Safety-and-Sandboxing.md) so scripts can touch only what you allow,
[limits](Safety-and-Sandboxing.md#limits) so a loop cannot hang the server, and a
[Runner](Concurrency-and-Performance.md) per script so each is isolated.

```go
npc := gtfparser.New(npcHost,
    gtfparser.WithoutEnvironment(),
    gtfparser.WithRestriction(gtfparser.RestrictWorld),   // no shell, files or new connections
    gtfparser.WithLimits(gtfparser.Limits{MaxInstr: 50_000, MaxRecur: 20}))
npc.AddFunction(gtfparser.Func("hp", world.HP))
npc.LoadFile("npcs/guard.tf")
// each event in the room: npc.ProcessLine("Bob waves at the guard.")
```

## 3. Chat bots

Bridge IRC, Discord, Slack, Matrix: every incoming message is a line;
`/send` goes back to the channel. Commands people already know (`/def`) let
operators customize the bot without recompiling.

```tf
/def -mregexp -t'^!roll (\d+)$' roll = /send you rolled $[rand({P1}) + 1]
/def -t'*good morning*' gm = /send Good morning, %1!
```

## 4. Log and stream processing

Tail a log, a serial line, a websocket feed, or `journalctl`; patterns
select the interesting lines, macros keep running state, timers summarize.

```tf
/def -mregexp -t'ERROR (\w+): (.*)' err = /test errors := errors + 1
/def -mregexp -t'WARN' warn = /test warns := warns + 1
/repeat -60 i /echo last minute: %{errors} errors, %{warns} warnings
```

## 5. Alerting and monitoring rules

Operators edit the rules, you don't redeploy. The engine runs each rule with
limits, and [`/restrict`](Safety-and-Sandboxing.md) keeps rules from reaching
the shell.

```tf
/def -mregexp -t'disk (\d+)% full' disk = /if ({P1} > 90) /send page-oncall disk %{P1}%%%; /endif
```

## 6. Expect-style automation: telnet, SSH, BBS, serial consoles

Script a conversation with something that speaks text: wait for a prompt, answer
it, branch on the reply. One-shot triggers (`-n1`) are the idiom.

```tf
/def -n1 -t'login:*' u = /send admin
/def -n1 -t'Password:*' p = /send %{secret}
/def -n1 -t'*#' done = /echo logged in%; /send show version
```

## 7. Bots and load tests for game servers

Many `Interp`s, one per simulated player, each with its own script and a
connection as its host. Timers (`/repeat`) drive behavior; counters
(`/test`) record results. [Limits](Safety-and-Sandboxing.md#limits) keep
a buggy script from stalling the run.

## 8. A user-facing rules / macro language for any application

An editor, a build tool, a home-automation hub, a spreadsheet-like app: anywhere
users want "when X, do Y" without learning a general-purpose language. The
host exposes your domain as commands and functions
([tables](Extending-the-Engine.md)); users compose them.

```go
in.Load(gtfparser.Extension{Commands: homeCommands, Functions: homeFunctions})
```

```tf-noverify
/def -t'motion detected *' porch = /if (ftime('%H') >= 22) /light porch on%; /endif
/repeat -0:30:00 i /thermostat adjust
```

## 9. A scriptable shell or admin console

A REPL or admin prompt whose commands are a table of
[`CommandSpec`s](Extending-the-Engine.md) with options, usage and help generated
for you, plus macros, variables, expressions and loops for free: your own
console with a real scripting layer.

## 10. Relay, proxy and bridge daemons

A middleman that sits between users and servers can run scripts on the traffic
in transit: filtering, rewriting, logging, auto-replies. One interpreter per
session, a [`Policy`](Safety-and-Sandboxing.md) per tenant.

## 11. Running legacy tf scripts in new places

Decades of tf scripts exist. Because tf's own library loads and runs
unmodified and behavior is
[checked against the real tf](Testing-and-Compatibility.md), you can bring them to
a web app, a mobile client, a daemon, with no rewrite.

## 12. Teaching and prototyping

A small language with real semantics (dynamic scope, substitution, triggers)
that runs anywhere Go does: handy for teaching text-processing ideas, for
prototyping automation rules before hard-coding them, and for config files that
need a little logic.

---

### When it is *not* the right tool

- You need a general-purpose language with data structures, modules and a
  package ecosystem; use Lua, JavaScript or Starlark.
- You need hard isolation of hostile code; use [`/restrict`, policy and
  limits](Safety-and-Sandboxing.md) for guardrails, and operating-system or
  container sandboxing for real isolation.
- You need high-throughput numeric work; tf is for orchestrating text and
  events.
