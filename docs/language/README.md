# The tf language guide

A guide to the TinyFugue (tf) scripting language as implemented by gtfparser,
for two readers:

* **developers new to tf** who need to learn the language to script an application
  that embeds the engine, and
* **tf veterans** who want to know exactly how this engine behaves.

Every example is a `tf` block followed by an `output` block, and
`python3 scripts/doccheck.py docs/language` checks that the engine really prints
that output. Where this engine differs from TinyFugue, the pages say so;
[Known differences](Known-Differences.md) and
[Deliberate differences](Deliberate-Differences.md) list every case with the
exact output of each.

## Start here

| Page | What it covers |
|---|---|
| [Overview](Overview.md) | what tf is, the mental model, a first script |
| [Evaluation model](Evaluation-Model.md) | how a line becomes actions; commands vs text; **top-level lines vs macro bodies**; return values, `/eval`, `/exit` |
| [Substitution](Substitution.md) | every `%` and `$` form; when expansion happens; why nested `/def`s need `%%` |

## The language

| Page | What it covers |
|---|---|
| [Expressions](Expressions.md) | types (int, decimal, float, string), operators and precedence, comparisons, truth, functions |
| [Variables and scope](Variables-and-Scope.md) | `/set` vs `/let`, dynamic scope, parameters, special variables, environment |
| [Control flow](Control-Flow.md) | `/if`, `/while`, `/for`, `/break`, `/return` |
| [Macros](Macros.md) | `/def` and its options, naming, redefinition, `/undef`, `/list`, `/purge`, macros as functions |
| [Options and getopts](Options-and-getopts.md) | writing commands that take `-options` like builtins; `/shift` |

## Reacting to the world

| Page | What it covers |
|---|---|
| [Triggers](Triggers.md) | matching, priority and fall-through, shots, conditions, gag and hilite |
| [Hooks](Hooks.md) | events, patterns, how a program fires them |
| [Patterns](Patterns.md) | simple, glob (including `{a\|b}`), regexp, substr; `-m` and `%matching` |
| [Regular expressions](Regular-Expressions.md) | PCRE2 specifics: smart case, DOTALL, captures, what is not supported |

## Running things and talking to the outside

| Page | What it covers |
|---|---|
| [Processes and timers](Processes-and-Timers.md) | `/repeat`, `/quote`, `/ps`, `/kill`, how the host drives timers |
| [Files and streams](Files-and-Streams.md) | `tfopen` and friends, handles `i`/`o`/`e`, pipes, `/load`, `/require`, `/exit` |

## Reference and orientation

| Page | What it covers |
|---|---|
| [Idioms and pitfalls](Idioms-and-Pitfalls.md) | the mistakes everyone makes, and idioms worth copying |
| [Differences from other languages](Differences-from-Other-Languages.md) | orientation for shell, JavaScript and Lua users |
| [Known differences](Known-Differences.md) | bugs we intend to fix: exact outputs of TinyFugue and gtfparser side by side |
| [Deliberate differences](Deliberate-Differences.md) | where gtfparser deliberately departs from TinyFugue, and why |

## Reading order

New to tf: Overview, Evaluation model, Substitution, Expressions, Variables and
scope, Control flow, Macros, Triggers, Patterns. Then the rest as you need it.

## About the examples

Examples run as if each line were typed with full substitution (like a macro
body), except `/def`, `/load` and `/require` lines, which load as from a script
file. Lines sent to the world appear as `[send] text`. Examples that need a
real client (hooks fired by the host, display attributes) are shown with the
output the reference `tf` produces.
