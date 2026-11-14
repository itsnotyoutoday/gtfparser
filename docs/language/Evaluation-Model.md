# Evaluation model

How a line of text becomes actions: which lines are commands, which are sent to
the world, what gets substituted when, and how results travel back.

This page contains the one subtlety that trips up almost every newcomer, so read
[top-level lines versus macro bodies](#top-level-lines-versus-macro-bodies)
even if you skim the rest.

## Commands and plain text

Every line the interpreter executes is one of:

| First characters | Kind | What happens |
|---|---|---|
| `/name ...` | command | runs the macro `name` if one exists, otherwise the builtin `name` |
| `/@name ...` | builtin command | always the builtin, even if a macro has the same name |
| `/#123 ...` | numbered macro | runs macro number 123 (see [Macros](Macros.md)) |
| `/!name ...` | negated command | runs `/name` and logical-NOTs its return value |
| `//text` | plain text | sends `/text` to the world (one `/` is stripped) |
| anything else | plain text | sent to the current world as-is |

If `/name` is neither a macro nor a builtin, the `NOMACRO` hook fires and a
warning is printed.

```tf
/echo this is a command
say this is text for the world
//echo this is text too, with one slash removed
/@echo builtin forced
```
```output
this is a command
[send] say this is text for the world
[send] /echo this is text too, with one slash removed
builtin forced
```

The `[send]` prefix above comes from the sample runner used to check this
documentation: it prints every line that would go to the world. In your own
program, text sent to the world goes to your `Host`'s `Send` method.

A **list** is a sequence of commands separated by `%;`. A **compound command**
is an `/if ... /endif` or `/while ... /done` block. The return value of a list
is the return value of the last command run in it, and an empty list returns 1.

## Top-level lines versus macro bodies

There are two places tf runs commands from, and they treat text differently.

**A macro body** is what follows the `=` in `/def name = body` (and what
`/eval`, `/repeat`, and `$(...)` run). It gets full
[substitution](Substitution.md) each time it runs: `%{x}`, `%1`, `$[expr]`,
`$(cmd)`, `%%`, `\n`, and `%;` splitting into commands.

**A top-level line** is a line typed at the prompt or read from a script file
by `/load`. By default (`%sub` is `off`) it gets **no substitution**: `%x` and
`$[1+1]` pass through untouched, and `%;` does **not** split it. The only
exception is a line that starts with `/if` or `/while`, whose condition and body
are lists and are split at `%;`.

That is why scripts are written as a flat sequence of `/set`, `/def` and
`/load` commands, and anything that needs variables or arithmetic lives inside a
macro. Here is a script file written and loaded to show the difference.
(`%%` and `$$` in the lines that write the file are ordinary
[substitution escapes](Substitution.md); the file ends up containing `%x`
and `$[x+1]`.)

```tf
/set x=5
/test fd := tfopen('/tmp/gtfdoc_eval.tf', 'w')
/test tfwrite(fd, '/echo top-level: x=%%x sum=$$[x+1]')
/test tfwrite(fd, '/def body = /echo in a macro: x=%%x sum=$$[x+1]')
/test tfwrite(fd, '/body')
/test tfclose(fd)
/load /tmp/gtfdoc_eval.tf
```
```output
top-level: x=%x sum=$[x+1]
in a macro: x=5 sum=6
```

The first line of the file is a top-level line: nothing was substituted. The
`/body` macro is the same text, but as a body it is expanded when it runs.

If you do want substitution on a one-off line, use `/eval`, which runs its
argument as a macro body:

```tf
/set x=5
/eval /echo x is %x, x+1 is $[x+1]
```
```output
x is 5, x+1 is 6
```

### When you want the line to be literal

A body that consists only of `%{*}` expands to the argument text, and a line of
plain text is sent to the world even if it starts with `/`:

```tf
/set who=world
/def fwd = %{*}
/fwd /echo hi %who
/def fwd2 = /eval -s0 %{*}
/fwd2 /echo hi %%who
```
```output
[send] /echo hi world
hi %who
```

`/fwd` sent the expanded text `/echo hi world` to the world instead of running
it (a body's expansion is not re-parsed as a command). `/fwd2` hands the text to
`/eval -s0`, which runs it as a command with no further substitution. The
`%%who` was reduced to `%who` when you typed the line, and `-s0` then leaves it
alone. See [`/eval`](#eval) below.

> **For embedders.** The Go API mirrors this split. `Interp.Exec(line)` runs a
> line the way a script file does (no substitution, no `%;` splitting).
> `Interp.Run(text)` runs text the way a macro body or `/eval` does (full
> substitution). Use `Exec` for configuration lines you read from a file and
> `Run` for commands your program composes itself.
> Examples in this guide are written as `Run` would run them, except `/def`,
> `/load` and `/require` lines, which load as from a script file.

## Return values and `%?`

Every command has a return value, which `%?` holds until the next command runs.
Builtins usually return nonzero on success and 0 on failure; a macro returns the
value of the last command in its body, or whatever `/return` gives it.

```tf
/def ok = /return 1
/def fail = /return 0
/ok
/echo ok gave %?
/fail
/echo fail gave %?
/!fail
/echo /!fail gave %?
```
```output
ok gave 1
fail gave 0
/!fail gave 1
```

`/return` ends the macro immediately. Called from another macro, the value
propagates through `%?` as usual:

```tf
/def a = /echo one%; /return 5%; /echo never printed
/a
/echo a returned %?
/def b = /a%; /echo b sees %?
/b
```
```output
one
a returned 5
one
b sees 5
```

A plain-text command returns 1 if the text was sent and 0 if there is no world
to send it to.

### The three kinds of "result"

A command can hand something back in three different ways. They are
independent, and mixing them up is a common source of confusion.

| Channel | Produced by | Read with |
|---|---|---|
| **Return value** | `/return expr`, `/result expr`, or the last command run | `%?` after a command; the value of `name(...)` in an expression |
| **Echoed output** (tfout) | `/echo`, `/result` when called as a command, most builtins | `$(...)`, `%\|` pipes, `/quote` |
| **Text sent to the world** | plain-text commands, `/send` | your host's `Send`; not capturable from inside tf |

`/return` produces only a return value. `/result` produces a return value *and*,
when called as a command, echoes the value. That makes it useful for macros you
want to call both ways:

```tf
/def sq = /result {1} * {1}
/sq 7
/echo $[sq(7) + 1]
/echo [$(/sq 5)]
```
```output
49
50
[25]
```

`/sq 7` as a command echoed 49. `sq(7)` in an expression used the return value (and
did not echo). `$(/sq 5)` captured the echoed output.

A macro that never says `/return` returns whatever its last command returned. A
macro called as a function with no usable result returns the empty string:

```tf
/def noreturn = /echo working
/noreturn
/echo noreturn returned [%?]
/def r0 = /return
/r0
/echo r0 returned [%?]
```
```output
working
noreturn returned [1]
r0 returned []
```

(The reference client returns `0` for a bare `/return`; see
[FINDINGS](FINDINGS.md).)

## `/eval`

`/eval text` runs `text` as a macro body: full substitution, in a new scope,
with the caller's positional parameters (`%1`, `%*`...) inherited. Use it when a
command is built dynamically.

```tf
/def showvar = /eval /echo %{1} is %%{%{1}}
/set colour=teal
/showvar colour
```
```output
colour is teal
```

Here `%{1}` is expanded when `showvar` runs, giving `/eval /echo colour is %{colour}`;
`/eval` then expands the `%{colour}`.

`/eval -s0 text` runs `text` with **no** substitution (as a top-level line
would be), which is the safe way to execute text that came from elsewhere, such
as a trigger's captured output. `-s1` allows only `%%`, `%;` and `\n`, and
`-sfull` is the default. **Never `/eval` text from the world with
substitution enabled**: a hostile game could make your script run commands.

## Leaving early: `/return`, `/break`, `/exit`

* `/return [expr]` leaves the current macro body, returning `expr`.
* `/break [n]` leaves the innermost `n` `/while` or `/for` loops
  (see [Control flow](Control-Flow.md)).
* `/exit [n]` is for scripts: it stops the macro bodies in progress *and*
  abandons the current `/load`ed file (or `n` nested files). Outside a `/load`
  it does nothing. It is how a script file guards against being loaded twice:

```tf-noverify
/if (loaded_myscript) /exit%; /endif
/set loaded_myscript=1
```

See [Files and streams](Files-and-Streams.md#loading-scripts) for `/load`,
`/require` and `/loaded`.

## Scopes

Each macro expansion creates a new scope, and `/let` variables live in it. Scope
lookup is *dynamic*; see [Variables and scope](Variables-and-Scope.md).

## Pipes

`cmd1 %| cmd2` runs `cmd1` to completion, then connects its echoed output to
`cmd2`'s input stream, which `cmd2` reads with `tfread()` (see
[Files and streams](Files-and-Streams.md#pipes)). Plain-text commands and
compound commands cannot be the writing end of a pipe directly.

## Pitfalls

* Expecting `%x` or `$[...]` to work in a script file's top-level lines. They
  do not; put the logic in a macro or use `/eval`.
* Expecting `/set a=1%; /set b=2` on one top-level line to set both. It sets
  `a` to the string `1%; /set b=2`. Write one command per line, or put both in a
  macro body.
* Writing `/echo %1` at the prompt to test a macro's body. At the prompt `%1`
  is empty; test it inside a macro.
* Treating a macro's *echoed output* as its return value. Use `/result` when
  you want both, or `$(...)` to capture echoed output.

## See also

[Substitution](Substitution.md) ·
[Expressions](Expressions.md) ·
[Control flow](Control-Flow.md) ·
[Variables and scope](Variables-and-Scope.md) ·
[Macros](Macros.md)
