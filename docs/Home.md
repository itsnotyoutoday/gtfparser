# gtfparser wiki

**gtfparser** is a scripting engine for Go. It implements the TinyFugue (tf)
language, a compact, line-oriented language built for reacting to streams of
text, and gives you a runtime, a regexp engine and a set of clean extension
points so you can tie it into your own program.

If you have ever wanted users to be able to write *"when this line arrives, do
that"*, or to give your app a small, safe, battle-tested macro language, this
is that, and it already speaks a language with thirty years of scripts behind
it.

## Start here

| I want to... | Read |
|---|---|
| see it work in five minutes | [Getting started](Getting-Started.md) |
| embed it in my Go program | [Embedding guide](Embedding-Guide.md) |
| add my own commands and functions | [Extending the engine](Extending-the-Engine.md) |
| know what I could build with it | [Use cases](Use-Cases.md) |
| learn the scripting language | [Language guide](language/README.md) |
| look up a function or command | [Built-in reference](reference/README.md) |
| run other people's scripts safely | [Safety and sandboxing](Safety-and-Sandboxing.md) |
| drive it from many goroutines | [Concurrency and performance](Concurrency-and-Performance.md) |
| understand how compatible it is | [Testing and compatibility](Testing-and-Compatibility.md) |
| understand how it is built | [Architecture](Architecture.md) |

## What you get

- **The tf language**: macros, triggers, hooks, expressions with real numeric
  types, `%`-substitution, regexps (PCRE2 flavour), timers, pipes, file streams.
- **Compatibility you can check**: every behavior is compared against the real
  `tf` binary, and tf's own standard library loads and runs unmodified. See
  [Testing and compatibility](Testing-and-Compatibility.md).
- **An embedding API that stays out of your way**: a two-method `Host`, a
  declarative table loader, ordinary Go functions as script functions,
  optional capability interfaces for everything else.
- **Safety dials**: tf's own `/restrict` levels, your own `Policy`, resource
  limits, and an interrupt that works from any goroutine.
- **Speed**: compiled expressions and templates; on loops, calls and
  expressions it keeps pace with the C implementation.

## Install

    go get github.com/itsnotyoutoday/gtfparser

Go 1.24 or newer. No dependencies outside the standard library.

## A taste

```tf
/def -t'* tells you: *' reply = /echo (to %{1}) I heard: %{-3}
/trigger Bob tells you: nice weather
```
```output
(to Bob) I heard: nice weather
```

The first line is ordinary TinyFugue: a macro that fires whenever a matching
line of text arrives. The second pretends such a line arrived. In a real
program the line comes from a socket, a log, a chat service, a file.

## Status

The engine is young (v0.0.x) and the API is additive but not yet frozen. The
[built-in reference](reference/README.md) lists, per command and function,
what this build implements, what needs a capability from your program, and
what is not supported yet (those fail loudly rather than pretend).
