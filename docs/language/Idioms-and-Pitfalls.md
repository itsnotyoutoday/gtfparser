# Idioms and pitfalls

A field guide to the mistakes tf scripts are most often written with, each with the
fix, plus a few idioms worth copying. Every snippet here runs.

## 1. Percent signs: `%x` is expanded when you think it isn't (and vice versa)

A macro body is expanded when the macro **runs**; the body of a macro defined
*inside* a macro is expanded **twice** (once by each level), so anything that
must survive the first pass is escaped.

```tf
/set colour=red
/def paint_now = /def show_colour = /echo colour is %{colour}
/def paint_later = /def show_colour2 = /echo colour is %%{colour}
/paint_now
/paint_later
/set colour=blue
/show_colour
/show_colour2
```
```output
colour is red
colour is blue
```

`show_colour` froze `red` into its body when `paint_now` ran; `show_colour2`
reads `colour` each time it runs. Decide which you want, deliberately. See
[Substitution](Substitution.md#why-you-write--inside-nested-definitions).

## 2. Top-level lines are not substituted

```tf
/def demo = /set total=0%; /set total=$[total + 5]%; /echo total=%total
/demo
```
```output
total=5
```

Works because it is a macro body. In a script *file*, the same `/set total=$[total + 5]` on its own line
would store the literal text `$[total + 5]`. Put the logic in a macro, or use
`/eval`. See [Evaluation model](Evaluation-Model.md).

## 3. `%;` on a top-level line

Another top-level rule: `%;` does not split a file line (except after `/if` and
`/while`). `/echo a%; /echo b` on one line of a script file echoes the single
line `a%; /echo b`. Put each command on its own line, or in a macro.

## 4. `/for` and `%i`

```tf
/def bad = /for i 1 3 /echo i=%i
/def good = /for i 1 3 /echo i=%%i
/bad
/good
```
```output
i=
i=
i=
i=1
i=2
i=3
```

See [Control flow](Control-Flow.md#for).

## 5. Integer division

```tf
/def pct = /echo wrong: $[{1} / {2} * 100]%%  right: $[{1} * 100 / {2}]%%  float: $[{1} * 100.0 / {2}]%%
/pct 1 3
```
```output
wrong: 0%  right: 33%  float: 33.3333333333333%
```

`1 / 3` is `0` in integers. Multiply first, or make one side decimal (the result
is then a float with a trailing dot when whole).

## 6. Strings that are numbers, and numbers that are strings

```tf
/def cmp = /echo [$[{1} == {2}]] [$[{1} =~ {2}]]
/cmp abc xyz
/cmp 5 +5
/cmp 1.0 1
```
```output
[1] [0]
[1] [0]
[1] [0]
```

`==` converts both sides to numbers, so any two non-numeric strings are equal
(both `0`). `=~` compares text. Use `=~` for strings and `==` for numbers.
A related trap: a string with no numeric prefix is *false*, so `/if (name)`
is not "name is not empty". Use `strlen(name)`.

## 7. Comparing with `=` is not assignment (and `:=` is)

`=` is an alias for `==`. Assign with `:=` or `/set`.

## 8. Spaces in `/set`

```tf
/set a=5
/set b = 5
/echo [%a] [%b]
```
```output
[5] [= 5]
```

## 9. A macro that eats its own output

Echoing is not returning. If you want a value, `/return` it (or `/result` to do both):

```tf
/def twice_bad = /echo $[{1} * 2]
/def twice_good = /return {1} * 2
/echo [$[twice_bad(4)]] [$[twice_good(4)]]
```
```output
8
[1] [8]
```

`twice_bad(4)` printed its echo and returned `1` (the success of `/echo`). The
`8` is only from `twice_good`.

## 10. Variables leaking and shadowing

An assignment in an expression to a name that is not already a local creates a
**global**. Create locals with `/let` first, and give loop variables distinctive names.
See [Variables and scope](Variables-and-Scope.md).

## 11. Forgetting that text is sent to the world

A line that does not start with `/` goes to the game. A typo'd command
(`/ecoh hi`) reports "no such command"; a plain-text slip (`echo hi`) is sent
as a game command. And a body that expands to something starting with `/` but
is *text* (like `%{*}` holding `/echo x`) is **sent**, not run; use
`/eval -s0 %{*}` to run it.

## 12. Using `%1` for the first `*` of a glob

A glob's wildcards do not capture. `%1` is the first **word** of the line. Use
a regexp group (`%P1`) when you need the matched piece.

## Idioms worth keeping

**Default arguments.** `%{1-default}`.

```tf
/def greet = /echo hello %{1-adventurer}, you have %{2-no} messages
/greet
/greet Ana 3
```
```output
hello adventurer, you have no messages
hello Ana, you have 3 messages
```

**Swap-in/out with a flag.** Gate behaviour on a variable with `-E` instead of
defining/undefining macros:

```tf
/set autoloot=0
/def -E'autoloot' -t'* dies.' loot_corpse = /echo looting %1
/trigger Orc dies.
/set autoloot=1
/trigger Orc dies.
```
```output
looting Orc
```

**Quiet helpers.** Silence a helper macro's output with `/test tfclose('o')` at
its top ([Files and streams](Files-and-Streams.md)).

**Loop over words.**

```tf
/def each_word = /while ({#}) /echo word: %1%; /shift%; /done
/each_word red green blue
```
```output
word: red
word: green
word: blue
```

**Return a value and show it.** `/result` works as a command *and* a function:

```tf
/def area = /result {1} * {2}
/area 3 4
/echo $[area(3, 4) + 1]
```
```output
12
13
```

**Generate a trigger from a variable.** Triggers don't substitute their
patterns, so redefine from a macro:

```tf
/def watch = /def -t"%{1} has arrived." greet_%{1} = /echo welcome, %{1}
/watch Bob
/trigger Bob has arrived.
/trigger Carol has arrived.
```
```output
welcome, Bob
```

## See also

[Evaluation model](Evaluation-Model.md) ·
[Substitution](Substitution.md) ·
[Expressions](Expressions.md) ·
[Differences from other languages](Differences-from-Other-Languages.md)
