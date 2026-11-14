# FAQ

**Is this a port of tf or a new implementation?**
A new implementation in Go, written to match the behavior of tf and checked
against the real binary. It runs tf's own standard library unmodified.

**Which tf version does it match?**
tf 5.0 beta 8 (the maintained 5.x line). Behavior is verified against a build of
it; see [Testing and compatibility](Testing-and-Compatibility.md).

**Do I have to be building a MUD client?**
No. Anything that reacts to lines of text can use it; see [Use cases](Use-Cases.md).

**Why tf and not Lua or JavaScript?**
Lua and JS are general-purpose and you will have to build the "react to a line,
keep state, run timers" layer yourself. tf is that layer, with a compact syntax
that non-programmers learn quickly, and a huge existing body of scripts. If you
need general programming, use a general language; if you need a small,
event-driven rules language, this fits.

**Is it safe to run untrusted scripts?**
It gives you real guardrails (restriction levels, a policy hook, resource limits,
interrupts) but is not an OS-level sandbox. See [Safety and sandboxing](Safety-and-Sandboxing.md).

**Can I use it from several goroutines?**
One interpreter belongs to one goroutine; use a `Runner` to share it. See
[Concurrency](Concurrency-and-Performance.md).

**What does "not supported by this client" mean?**
Real tf has a command or function the engine does not implement for your host
yet (usually because it needs a capability like connections or a terminal). It
fails loudly instead of pretending. The [status table](reference/Status.md) lists
which, and the [embedding guide](Embedding-Guide.md#optional-capabilities) shows
which interface supplies what.

**Why do `Run` and `Exec` behave differently?**
Because tf lines read from a script file are not substituted until a macro runs,
while macro bodies are. It is the one tf rule worth learning first:
[Evaluation model](language/Evaluation-Model.md).

**Can I make my own scripting dialect on top?**
Yes, with [extension tables](Extending-the-Engine.md): add commands and
functions, disable built-ins, preload macros, restrict capabilities.

**How do I report a bug?**
Give a few lines of tf, what real tf prints and what the engine prints. See
[Testing and compatibility](Testing-and-Compatibility.md#reporting-a-difference).
