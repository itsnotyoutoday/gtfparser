# Overview: what tf is

TinyFugue ("tf") is a scripting language that grew up inside a MUD client. Its job
is to sit between you and a text game: it sees every line the game sends,
every line you type, and lets scripts react to both. This engine implements the
language and its runtime as a Go library, so a program of yours can host tf
scripts: a game client, a relay, a bot, a test rig.

This guide teaches the language. If you are embedding the engine, start with the
repository README for the Go API, and come back here to write the scripts your
program will run.

## The mental model

Think of three kinds of thing:

1. **Lines.** Text moving through the system: lines from the world
   ("You hit the orc."), and lines you give to the interpreter ("/echo hi", "kill orc").
2. **Commands.** A line that starts with `/` is a *command*; anything else
   is *text for the world*, sent as is.
3. **Macros.** Named bodies of commands. A macro can be called by name
   (`/name`), by a **trigger** (a line from the world matched a pattern), by a
   **hook** (something happened), or from an expression (`name(args)`).

Everything else (variables, expressions, loops) is machinery for writing
macro bodies.

```
 world ──line──▶ triggers ──▶ macro body ──▶ commands ──▶ /echo, /set, ...
                    ▲                  │
 you/host ─ /cmd ───┘                  └──▶ text ──▶ sent to the world
```

## A first script

```tf
/set target=orc
/def kill_it = /echo attacking the %{target}!%; kill %{target}
/kill_it
/def -t'* attacks you!' counter = /echo counter-attack against %1%; kill %1
/trigger Goblin attacks you!
```
```output
attacking the orc!
[send] kill orc
counter-attack against Goblin
[send] kill Goblin
```

In order:

* `/set target=orc` stores text in a global variable.
* `/def kill_it = ...` defines a macro. Its body has two commands separated by
  `%;`: `/echo` (a command, shows text to you) and `kill orc` (not a command:
  plain text, sent to the game). `%{target}` is replaced when the macro **runs**.
* `/kill_it` calls it.
* `/def -t'...' counter = ...` defines a macro with a **trigger**: a glob
  pattern. When a line matching it arrives, the macro runs with the line's
  words as arguments. (`/trigger text` pretends a line arrived; in a real
  program the host passes incoming lines to the interpreter.)

## The ingredients

| You want to... | Use | Page |
|---|---|---|
| send text, run commands, understand what runs when | commands and the evaluation rules | [Evaluation model](Evaluation-Model.md) |
| insert variables, arguments, results into text | `%x`, `%1`, `$[expr]`, `$(cmd)` | [Substitution](Substitution.md) |
| calculate, compare, test | expressions | [Expressions](Expressions.md) |
| remember things | `/set`, `/let` | [Variables and scope](Variables-and-Scope.md) |
| branch and loop | `/if`, `/while`, `/for` | [Control flow](Control-Flow.md) |
| define your own commands/functions | `/def` | [Macros](Macros.md) |
| react to the game | `/def -t` | [Triggers](Triggers.md), [Patterns](Patterns.md), [Regular expressions](Regular-Expressions.md) |
| react to client events | `/def -h` | [Hooks](Hooks.md) |
| do things later or repeatedly | `/repeat`, `/quote` | [Processes and timers](Processes-and-Timers.md) |
| read and write files, load scripts | `tfopen`, `/load` | [Files and streams](Files-and-Streams.md) |
| make commands with `-options` | `getopts` | [Options and getopts](Options-and-getopts.md) |

## What makes tf different

Four things that are worth knowing before you write much:

1. **Everything is text.** Variables hold text, and arithmetic converts text to
   numbers as needed (`"12abc" + 1` is 13). Typing is loose; the
   [Expressions](Expressions.md) page tells you exactly how.
2. **Substitution is a separate step** from evaluation. A macro body is first
   expanded (`%x` and `$[...]` replaced), then executed. Writing a macro that
   writes another macro is mostly a matter of counting percent signs. See
   [Substitution](Substitution.md).
3. **Scripts read top to bottom with no substitution.** Lines of a script file are
   executed as written; only macro bodies are expanded. The consequences
   are explained in [Evaluation model](Evaluation-Model.md#top-level-lines-versus-macro-bodies),
   and are the thing newcomers most often trip over.
4. **Scope is dynamic.** A macro sees the local variables of whoever called it.
   See [Variables and scope](Variables-and-Scope.md).

## How the examples in this guide work

Each example is a `tf` block followed by an `output` block with exactly what it
echoes. They are run by `scripts/doccheck.py`, which treats each line as if you had
typed it with full substitution (as in a macro body), except that `/def`,
`/load` and `/require` lines load as from a script file. Lines the interpreter
would send to the world are printed as `[send] text`.

## Where to go next

New to tf: read [Evaluation model](Evaluation-Model.md), [Substitution](Substitution.md),
[Expressions](Expressions.md), then [Macros](Macros.md) and [Triggers](Triggers.md).
Experienced with tf: skim [Idioms and pitfalls](Idioms-and-Pitfalls.md) and
[Known differences](Known-Differences.md) and
[Deliberate differences](Deliberate-Differences.md) for where this engine and
TinyFugue differ.
Coming from another language: [Differences from other languages](Differences-from-Other-Languages.md).
