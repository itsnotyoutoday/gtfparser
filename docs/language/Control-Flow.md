# Control flow

tf has `/if`, `/while` and `/for`, with `/break` and `/return` to leave early.
Everything here is a *command* with command rules: each part of a block is a
list of commands separated by `%;`, and a block ends with `/endif` or `/done`.

The examples are written as macros because that is where control flow normally
lives (see [Evaluation model](Evaluation-Model.md) for why).

## `/if`

There are two forms. The common one tests an expression in parentheses:

```
/if (expr) list [/elseif (expr) list]... [/else list] /endif
```

```tf
/def rate = /if ({1} >= 90) /echo excellent%; /elseif ({1} >= 50) /echo fair%; /else /echo poor%; /endif
/rate 95
/rate 60
/rate 10
```
```output
excellent
fair
poor
```

Every list must end with `%;` before the next keyword, including the one before
`/endif`.

The expression is a normal [expression](Expressions.md): remember that strings
are false unless they start with a nonzero number.

The second form tests the **return value of a command list**. The condition list
is closed by `/then`:

```
/if list /then list [/elseif list /then list]... [/else list] /endif
```

```tf
/def listif = /if /test {1} > 5%; /then /echo big%; /else /echo small%; /endif
/listif 9
/listif 2
```
```output
big
small
```

This is useful when the thing you want to test is a command, such as a
function call whose result you want to keep, or a command that succeeds or
fails (`/if /test regmatch(...)%; /then ...`).

Do not use a **plain-text command** as the condition (`/if say hi%; /then`):
what it returns is whether the line was *sent*, not whether the world liked it.

The return value of the `/if` statement itself is not defined; do not use it.

### Nesting

`/if` blocks nest. Mind the `%;` after each part:

```tf
/def n3 = /if ({1}) /echo a%; /else /echo b%; /if ({2}) /echo c%; /else /echo d%; /endif%; /endif
/n3 0 0
/n3 0 1
```
```output
b
d
b
c
```

## `/while`

```
/while (expr) list /done
/while list /do list /done
```

The condition is tested before each pass and the body repeats while it is
true.

```tf
/def count = \
    /let i=1%; \
    /while (i <= {1}) \
        /echo line %i%; \
        /let i=$[i + 1]%; \
    /done
/count 3
```
```output
line 1
line 2
line 3
```

(A trailing `\` continues a line in a script file; the pieces are joined.)

There is a list-condition form with `/do`, the counterpart of `/if ... /then`:

```tf
/def lw = /let n=%{1}%; /while /test n > 0%; /do /echo n=%n%; /let n=$[n-1]%; /done
/lw 2
```
```output
n=2
n=1
```

As in tf, `%max_iter` is ignored: a runaway loop ends when the instruction budget
(`%max_instr`, and the embedding program's limits; see the README) runs out, with
a warning, and the script carries on:

```tf
/def forever = /let i=0%; /while (1) /let i=$[i+1]%; /done
/forever
/echo still alive
```
```output
still alive
```

### `/while` on a script's top level

A top-level `/while` line is split at `%;` (it is one of the two exceptions to
"top-level lines are not split"), but its **body is not substituted**, so
`/set i=$[i+1]` does not do what it says. The loop below would never end because
`i` never changes. Put loops inside macros.

```tf-noverify
/set i=0
/while (i < 3) /echo %i%; /set i=$[i+1]%; /done      ; infinite at top level!
```

## `/for`

```
/for variable start end command...
```

`/for` runs the command for each integer from `start` to `end` inclusive, with
`variable` set to the current value. If `end` is smaller than `start`, nothing
runs.

```tf
/def f4 = /for i 1 3 /echo i=%%i
/f4
/def f5 = /for i 3 1 /echo never
/f5
/echo done
```
```output
i=1
i=2
i=3
done
```

Note the **`%%i`**. The commands of a `/for` are expanded again each time round
the loop, so anything that must see the loop variable has to survive the
macro's own expansion, exactly like a nested `/def` (see
[Substitution](Substitution.md#why-you-write--inside-nested-definitions)).
With `%i` the macro would substitute an empty `i` up front:

```tf
/def f1 = /for i 1 3 /echo i=%i
/f1
```
```output
i=
i=
i=
```

Likewise, mix macro arguments (expanded now) and loop values (expanded per
pass) deliberately:

```tf
/def times = /for i 1 3 /echo $$[i * %{1}]
/times 10
```
```output
10
20
30
```

`%{1}` is replaced when `times` runs; `$$[i * 10]` becomes `$[i * 10]` and is
evaluated each pass.

The body is the rest of the *statement*: a `%;` in the macro ends the loop, and
whatever follows runs once, afterwards. To put several commands inside the loop,
double the separator (`%%;`), as you would any other text that must survive
until the loop runs:

```tf
/def lv = /for i 1 2 /echo in %%i%%; /echo again %%i%; /echo after the loop
/lv
```
```output
in 1
again 1
in 2
again 2
after the loop
```

The loop variable is local to the loop (a global of the same name is left
alone). For anything more elaborate (a step
other than 1, early exit, loops over words) write a `/while`.

## `/break`

`/break` leaves the innermost `/while` or `/for`. `/break n` leaves `n` nested
loops.

```tf
/def brk = /let i=0%; /while (1) /let i=$[i + 1]%; /if (i == 2) /echo skipping%; /endif%; /if (i > 3) /break%; /endif%; /echo i=%i%; /done
/brk
/def brk2 = /let a=0%; /while (a < 3) /let a=$[a + 1]%; /let b=0%; /while (1) /let b=$[b + 1]%; /if (b > 1) /break 2%; /endif%; /echo a=%a b=%b%; /done%; /done%; /echo out a=%a b=%b
/brk2
```
```output
i=1
skipping
i=2
i=3
a=1 b=1
out a=1 b=2
```

There is no `/continue`. Structure the body with `/if` so the rest is skipped.

## `/return`

`/return [expr]` leaves the whole macro, not just the loop, and gives the macro
its return value. Because a loop is inside a macro, it is the way to
return a result out of a search:

```tf
/def first_over = /let i=0%; /while (i < 100) /let i=$[i + 1]%; /if (i * i > {1}) /return i%; /endif%; /done
/echo $[first_over(50)]
```
```output
8
```

## Pitfalls

* **Forgetting `%;`** before `/else`, `/elseif`, `/endif` or `/done`: the keyword
  ends up as an argument of the previous command.
* **Missing `/endif`.** tf reports `macro syntax error: expected /endif`.
* **`%i` in a `/for` body** inside a macro (use `%%i`, or `{i}` in an
  expression written `$$[i]`).
* **Naming a macro `do`, `then`, `else`, `done`, `if`...** These are keywords
  or builtins; the keyword wins.
* **Testing `/if (name)`** for non-empty strings; see
  [Expressions](Expressions.md#truth).
* **Several commands in a `/for` body.** The body is the rest of one statement,
  so a plain `%;` ends the loop and starts the next command of the macro. Use
  `%%;` to keep commands inside the loop.

## See also

[Evaluation model](Evaluation-Model.md) ·
[Expressions](Expressions.md) ·
[Macros](Macros.md) ·
[Idioms and pitfalls](Idioms-and-Pitfalls.md)
