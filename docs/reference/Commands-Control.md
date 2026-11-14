# Commands: Control Flow and Evaluation

These commands turn a macro body from a list of lines into a program:
conditionals, loops, early exits, return values, and re-evaluation of text.
Most of them are *keywords* handled by the interpreter itself in real tf, and a
few (`/for`, `/not`, `/expr`, `/toggle`) are library macros defined in
`stdlib.tf`; this page covers both, and marks the library ones as **Library**.

Examples run with full substitution of each line, as if `/set sub=full` (in real tf a typed line is only expanded in that mode; inside macros and `/eval` it always is).

Two rules apply to everything below. Commands in one body are separated by
`%;`. Every `/if` needs an `/endif` and every `/while` needs a `/done`.
Examples run as typed input, with full substitution, with the standard library
loaded.

## /if (expr) list [/elseif (expr) list]... [/else list] /endif

**What it is for:** running commands only when a condition holds.

**Syntax**

    /if (expression) commands %; [/elseif (expression) commands %;]... [/else commands %;] /endif
    /if command %; /then commands %; [/elseif command %; /then commands %;]... [/else commands %;] /endif

**Arguments.** The first form takes a parenthesised
expression; a non-zero result selects the following list.
The second form uses a *command* as the condition (its return value is the test);
`/then` starts the list that runs when it succeeds. Each `/elseif` is tried in
turn, and `/else` runs when nothing matched. Every list ends with `%;`.

```tf
/def hpcheck = /if ({1} < 50) /echo %1: low%; /elseif ({1} < 80) /echo %1: ok%; /else /echo %1: full%; /endif
/hpcheck 20
/hpcheck 60
/hpcheck 99
```
```output
20: low
60: ok
99: full
```

**Gotchas**

- The condition expression sees macro parameters as `{1}`, `{*}`, `{#}`, not
  `%1`: the expression is not put through body substitution first.
- If the condition is plain text (a MUD command) rather than a tf command, the
  test is only "was it sent", which is almost never what you want. React to the
  game's reply with a trigger instead.
- The return value of the whole `/if` statement is undefined; use `/return`
  inside the branch you care about.

**Status:** see [Status](Status.md)

## /while (expr) list /done

**What it is for:** repeating commands while a condition stays true.

**Syntax**

    /while (expression) commands %; /done
    /while command %; /do commands %; /done

**Arguments.** The condition is evaluated before every pass. The second form
tests the return value of a command and uses `/do` to start the loop body.
`/break` leaves the loop early.

```tf
/def drain = /let n=%{1}%; /while (n > 0) /echo %{n} potions left%; /test --n%; /done
/drain 3
```
```output
3 potions left
2 potions left
1 potions left
```

**Gotchas**

- `/done` closes the loop; a stray `/endwhile` does not exist.
- Inside a loop that is part of a *typed* line, `%;` separates commands even when
  substitution is otherwise off.
- `/let` variables created inside the body are local to the macro, so they
  survive between passes.

A runaway loop is cut off by the interpreter: real tf aborts a macro that runs
more than `max_instr` instructions (default 1,000,000) with an
"instruction count exceeded" error. The documented `max_iter` variable is not
honoured by the reference build (this engine does honour it: FINDINGS C2, still open at v0.0.5).

**Status:** see [Status](Status.md)

## /do

**What it is for:** the keyword that starts the body of the command-condition form
of `/while` (`/while command %; /do body %; /done`). It is not a standalone
command.

**Syntax**

    /while command %; /do commands %; /done

```tf
/set cnt=0
/def bump = /test ++cnt%; /test cnt < 3
/def runbump = /while /bump%; /do /echo cnt=%cnt%; /done
/runbump
```
```output
cnt=1
cnt=2
```

**Gotchas.** Only meaningful between `/while` and `/done`; `/done` ends it.

**Status:** see [Status](Status.md)

## /done

**What it is for:** closes a `/while` loop (and the `/do` section of the command form).
See [/while](#while-expr-list-done).

**Status:** see [Status](Status.md)

## /elseif, /else, /endif, /then

**What it is for:** the pieces of an `/if` statement; see [/if](#if-expr-list-elseif-expr-list-else-list-endif).
They are only valid inside an `/if ... /endif`.

**Status:** see [Status](Status.md)

## /for variable start end commands

**Library.** Counts a variable from `start` to `end` inclusive and runs the
commands once per value.

**Syntax**

    /for variable start end commands

**Arguments.** `variable` is a name (not `%name`), `start` and `end` are
numbers or expressions, and `commands` is the rest of the line. If `end < start`
the commands never run.

```tf
/for n 1 3 /echo room %%{n}
```
```output
room 1
room 2
room 3
```

**Gotchas**

- The commands are expanded **twice** (once when the line is read, once by the
  loop), so the loop variable and any `%;` must be protected with `%%`, and `$`
  with `$$`, when `/for` is used inside a macro definition:

```tf
/def countdown = /for i 0 %{1} /echo $$[%{1} - i]
/countdown 3
```
```output
3
2
1
0
```

- `/for` is implemented with a local variable, so a macro called from the body
  does not see it unless passed in.

**Status:** see [Status](Status.md)

## /break [n]

**What it is for:** leaving a loop early.

**Syntax:** `/break [n]`

**Arguments.** `n` (default 1) is how many enclosing `/while` loops to leave.
Outside any loop, `/break` ends the macro.

```tf
/def hunt = /let i=0%; /while (1) /test ++i%; /if (i == 3) /break%; /endif%; /echo swing %i%; /done%; /echo stopped at %i
/hunt
```
```output
swing 1
swing 2
stopped at 3
```

**Gotchas.** `/break 2` leaves two nested loops; it does not leave the macro.

**Status:** see [Status](Status.md)

## /return [expression]  and  /result [expression]

**What it is for:** giving a macro a value, so it can be used as a function in an
expression (`$[double(4)]`).

**Syntax**

    /return [expression]
    /result [expression]

**Arguments.** Both stop the macro and hand back the **string** value of the
expression (empty if omitted). `/result` also *echoes* the value when the macro
was called as a command rather than a function, so one definition serves both
uses; `/return` never prints.

```tf
/def double = /result {1} * 2
/double 21
/echo $[double(4)]
/def quiet_double = /return {1} * 2
/quiet_double 21
/echo $[quiet_double(8)]
```
```output
42
8
16
```

**Gotchas**

- The value is converted to a string. An enumerated variable such as `borg`
  returns "on"; write `/return +borg` to return the number.
- `/return` is also how a recursive macro reports its answer:

```tf
/def strrev = /let len=$[strlen({*})]%; /return (len <= 1) ? {*} : strcat(strrev(substr({*},len/2)), strrev(substr({*},0,len/2)))
/echo $[strrev("orc")]
```
```output
cro
```

**Status:** see [Status](Status.md)

## /exit [n]

**What it is for:** abandoning a script file part-way, for example to avoid loading
it twice.

**Syntax:** `/exit [n]`

**Arguments.** During a `/load`, `/exit` stops all enclosing macro bodies and
abandons `n` (default 1) enclosing loads. Outside a load it does nothing; it
does **not** quit tf (that is `/quit`).

```tf-noverify
; top of a script file
/if (loaded_mymud) /exit%; /endif
/set loaded_mymud=1
```

**Gotchas.** Known difference: this engine keeps reading the file after an `/exit`
inside a macro called from it (FINDINGS C1, still open at v0.0.5).

**Status:** see [Status](Status.md)

## /test expression

**What it is for:** evaluating an expression for its side
effects or its value, from a command line or a macro.

**Syntax:** `/test expression`

**Arguments.** The expression's value becomes the return value of the command
(`%?`). Assignments (`:=`), increments (`++i`) and function calls such as
`regmatch()` are how scripts do real work with it. `/test` does **not** create
a new variable scope.

```tf
/test regmatch('^(\\w+) tells you: (.*)$', 'Bob tells you: hello')
/echo %P1 said "%P2"
/set gold=10
/test gold := gold * 3
/echo gold=%gold
/test 4+4
/echo result %?
```
```output
Bob said "hello"
gold=30
result 8
```

**Gotchas**

- `/test` takes the whole rest of the line as the expression; the line is first
  put through normal substitution, so write `$$` when you mean a literal `$`
  inside a macro.
- Before version 3.5 `/test` was the condition of `/if` and `/while`; today
  `/if (expr)` does it directly.
- The text is also reachable as the function `test()`.

**Status:** see [Status](Status.md)

## /@test expression

**What it is for:** like `/test`, but quiet and not logged.

The `@` prefix on any command name means "run the builtin, even if a macro of
that name exists". `/@test` is how library macros call the real `/test`
without being fooled by a user redefinition.

**Status:** see [Status](Status.md)

## /! command

**What it is for:** run a command and negate its result: `/!purge ...` succeeds
when the purge removed nothing.

```tf
/!false
/echo negated false = %?
/!true
/echo negated true = %?
```
```output
negated false = 1
negated true = 0
```

**Status:** see [Status](Status.md)

## /eval [-slevel] text

**What it is for:** evaluating text *again* as a macro body, after one
more round of substitution. Needed when a macro builds a command string.

**Syntax:** `/eval [-s<level>] text`

**Arguments.** `-s` sets the substitution level used for the text: `0` (none),
`1`, or `full`. Without it the text is expanded fully. Positional parameters of
the caller are visible inside. The return value is that of the last command run.

```tf
/def showvar = /eval /echo %{1} is %%{%{1}}.
/set borg=1
/showvar borg
/def runit = /eval -s0 %{*}
/runit /echo test
```
```output
borg is on.
test
```

**Gotchas**

- This is the classic injection hazard: never `/eval` text that came from a
  trigger match unless you have checked it, since the text may contain
  commands. Prefer `/eval -s0` when you only want to run a string as a command.
- `/eval` is also callable as the function `eval(text [, level])`.

**Status:** see [Status](Status.md)

## /expr expression

**Library.** Evaluates an expression and prints its value.

**Syntax:** `/expr expression`

```tf
/set base=4
/expr base * 2
```
```output
8
```

**Gotchas.** Positional parameters (`{1}`) and `{#}` are not defined inside the
expression; use `/result` or `echo()` when you need them.

**Status:** see [Status](Status.md)

## /not text

**Library.** Evaluates `text` as a command line like `/eval`, then returns the
logical negation of its result. Normally prefer `/!command`.

```tf
/not /false
/echo [%?]
```
```output
[1]
```

Note that, like `/eval`, `/not` evaluates its argument and a line of plain text
is *sent to the current world*, so `/not 5` sends "5".

**Status:** see [Status](Status.md)

## /toggle variable

**Library.** Flips a variable between 0 and 1: a zero value becomes 1, any other
value becomes 0. Handy for `/def -b'^Tx' flip = /toggle brief`.

```tf
/toggle brief
/echo brief=%brief
/toggle brief
/echo brief=%brief
```
```output
brief=1
brief=0
```

**Status:** see [Status](Status.md)

## /shift [n]

**What it is for:** consuming a macro's positional parameters one at a time, the
usual way to walk an argument list.

**Syntax:** `/shift [n]`

**Arguments.** Drops the first `n` (default 1) parameters: `%2` becomes `%1`,
and `%#` shrinks. Only useful inside a macro.

```tf
/def worlds = /while ({#}) /echo world %1%; /shift%; /done
/worlds foo bar baz
```
```output
world foo
world bar
world baz
```

**Gotchas.** `%*` shrinks too, and `%{-n}` style references always count from
the current first parameter.

**Status:** see [Status](Status.md)

## /true, /false and /:

**Library.** Do-nothing commands that return 1 (`/true`, `/:`) or 0 (`/false`),
ignoring their arguments. Useful as the condition of a command-style loop
or as a comment-like no-op that still expands its text.

```tf
/false
/echo false -> %?
/: this text is ignored
/echo colon -> %?
```
```output
false -> 0
colon -> 1
```

**Status:** see [Status](Status.md)

## /result vs /return, /test vs $[...]

A short comparison, since people mix these up. `/test expr` runs an expression
for effect (and sets `%?`); `$[expr]` substitutes an expression's text into a
line; `/return` and `/result` hand a value back from a macro; `%?` holds the
return value of the last command. Putting them together:

```tf
/def checkarmor = /test regmatch("^(\\d+)", {1})%; /return {P1} >= 30
/echo ok=$[checkarmor("45 plate")] bad=$[checkarmor("10 cloth")]
```
```output
ok=1 bad=0
```

**Status:** see [Status](Status.md)
