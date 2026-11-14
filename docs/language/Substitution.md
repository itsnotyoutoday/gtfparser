# Substitution

Substitution is how a macro body turns into the actual commands that run:
special sequences such as `%{name}`, `%1` and `$[1+1]` are replaced with text
before a command executes. This page lists every form, says when substitution
happens, and explains the `%%` rule that governs nested definitions.

## When substitution happens

Substitution is applied to the text of:

* a **macro body**, each time the macro runs;
* the argument of **`/eval`**, **`/repeat`** (when the process runs) and
  **`$(...)`**;
* a typed or loaded line only if `%sub` is `full` (it is `off` by default; see
  [Evaluation model](Evaluation-Model.md#top-level-lines-versus-macro-bodies)).

It is a single left-to-right pass over the text. The result of one substitution
is **not** scanned again, so a variable whose value contains `%x` does not
cause a second expansion. Expansion then happens again only if the result is
itself executed by something that substitutes, such as `/eval`.

It is *not* applied to trigger patterns, to the pattern part of `/def -t`, or to
text received from the world.

## Quick reference

| Form | Meaning |
|---|---|
| `%name`, `%{name}` | value of variable `name` |
| `%{name-default}` | value, or `default` if empty or unset |
| `%1` ... `%9`, `%{12}` | positional parameter *n* (a macro's arguments) |
| `%0` | the name of the macro that is running |
| `%*`, `%{*}` | all positional parameters |
| `%#`, `%{#}` | number of positional parameters |
| `%-n`, `%{-n}` | all parameters *except the first n* |
| `%L`, `%Ln` | the last parameter, the *n*th from last |
| `%-L`, `%-Ln` | all parameters except the last (n) |
| `%R` | a randomly chosen parameter |
| `%P0`, `%P1`, ... | text matched by a regexp, see [Regular expressions](Regular-Expressions.md) |
| `%PL`, `%PR` | text left and right of the regexp match |
| `%?` | return value of the last command |
| `%%` | a literal `%` |
| `%;` | command separator |
| `%\|` | pipe separator |
| `$[expr]` | value of an [expression](Expressions.md) |
| `$(cmd)` | echoed output of a command |
| `${name}` | the body of macro `name` |
| `$$` | a literal `$` |
| `\n`, `\c` | backslash escapes |

## Variables and parameters

`%name` and `%{name}` substitute a variable's value. The braces are needed when
the next character could be part of the name. A name is never empty: an unset
variable substitutes as the empty string.

```tf
/set v=hello
/def a = /echo [%v] [%{v}] [%{v}world] [%vworld]
/a
```
```output
[hello] [hello] [helloworld] []
```

`%vworld` looks for a variable named `vworld`, which does not exist, so it
substitutes nothing.

### Positional parameters

When a macro is called, the text after its name is split into words. `%1` is the
first, `%2` the second, and so on; `%*` is the whole argument text and `%#` the
count. `%0` is the macro's name.

```tf
/def args = /echo name=%0 count=%# first=%1 second=%2 all=[%*]
/args one two three
/args
```
```output
name=args count=3 first=one second=two all=[one two three]
name=args count=0 first= second= all=[]
```

The remaining selectors slice the list from either end:

```tf
/def slice = /echo [%{-1}] [%{-2}] [%{L}] [%{L2}] [%{-L}] [%{-L2}]
/slice a b c d
```
```output
[b c d] [c d] [d] [c] [a b c] [a b]
```

`%{-1}` is everything after the first word; `%{L}` is the last word and `%{L2}`
the second to last; `%{-L}` is everything *before* the last word. There is no
maximum parameter number: `%{9}` is simply empty when fewer words were given.
(`L`, `R` and `P` are case sensitive.)

How the words are formed depends on how the macro was called:

* `/name a b c` splits the text after the name at spaces.
* `name(a, b, c)` (function syntax) makes each argument one parameter.
* A **trigger** gets the words of the matching line; see [Triggers](Triggers.md).
* A **hook** gets the hook's arguments; see [Hooks](Hooks.md).
* `/eval` inherits the caller's parameters.

`{1}`, `{*}` and so on are also accepted inside expressions, without the `%`;
see [Expressions](Expressions.md).

### Defaults

`%{sel-default}` substitutes `default` when the selector would be empty. The
default may itself contain substitutions.

```tf
/set n=3
/def greet = /echo hello, %{1-stranger}! pick %{2-$[n*2]}
/greet
/greet Ana
/greet Ana 9
```
```output
hello, stranger! pick 6
hello, Ana! pick 6
hello, Ana! pick 9
```

Only the first `-` separates the selector from the default, so a default like
`%{1-a-b}` is `a-b`. (A selector that itself starts with `-`, such as `%{-2}`,
is the "all except" form above, not a default.)

## `%;`, `%|`: splitting commands

Inside a macro body, `%;` ends one command and starts the next:

```tf
/def two = /echo first%;/echo second
/two
```
```output
first
second
```

`%|` also separates commands but connects the first one's output to the next
one's input; see [Files and streams](Files-and-Streams.md#pipes).

## `$[expr]`: expressions

The expression is evaluated and its string value substituted. Variables are
named without a `%` inside an expression.

```tf
/set hp=42
/set maxhp=60
/def status = /echo HP $[hp]/$[maxhp] ($[hp * 100 / maxhp]%%)
/status
```
```output
HP 42/60 (70%)
```

## `$(cmd)`: command substitution

`$(...)` runs `cmd` as a macro body in a new scope and substitutes what it
*echoed*, like backticks in a shell. If it echoed several lines they are joined
with single spaces. A literal `)` inside must be written `\)`.

```tf
/def pair = /echo %{1}%;/echo %{2}
/echo got: $(/pair left right)
```
```output
got: left right
```

## `${name}`: macro bodies

`${name}` substitutes the **unexpanded body** of the macro `name` (not its
result). It is occasionally used to include a shared fragment of text.

```tf
/def fragment = swing the sword
/def attack = /echo you ${fragment}
/attack
```
```output
you swing the sword
```

The older form `$name$` still exists but is discouraged.

## Backslash escapes

`\n`, `\c`: a backslash followed by digits is the character with that code
(decimal, `0` prefix for octal, `0x` for hex); a backslash followed by anything
else is that character, which removes any special meaning. `\\` is a single
backslash.

```tf
/def esc = /echo [\101] [\0x41] [\q] [\\] [\%x] [100\%]
/esc
```
```output
[e] [A] [q] [\] [%x] [100%]
```

## `%%` and `$$`: literal characters

`%%` becomes `%` and `$$` becomes `$`. Longer runs lose one character each
(`%%%%` becomes `%%%`). A lone `%` or `$` that does not begin one of the forms
above is left as is.

```tf
/def lit = /echo 100%% of $$5 [%%%%] [$$$$]
/lit
```
```output
100% of $5 [%%%] [$$$]
```

> A lone `$` followed by a space is the one place this engine and the reference
> `tf` disagree; use `$$` whenever you mean a dollar sign. See
> [FINDINGS](FINDINGS.md).

## Why you write `%%` inside nested definitions

A macro body is expanded when the macro **runs**. If the body contains a `/def`,
the text of the inner `/def` is expanded too, *before* the inner definition
exists. Anything you want to survive until the *inner* macro runs must be
escaped once for each level of nesting.

Suppose you want `/follow Joe` to create a trigger that sends
`go <direction>` when the world says `Joe goes <direction>.`.
Work from the inside out: first decide the inner macro,

```tf-noverify
/def -t'Joe goes *' do_follow = go %3
```

then escape it for the outer one. Anything you want substituted *now* (the
name) keeps one `%`; anything for *later* (the word `%3` the trigger will
receive) gets doubled:

```tf
/def follow = /def -t"%{1} goes *" do_follow = go %%3
/follow Joe
/list do_follow
```
```output
% 5: /def -p1 -mglob -t'Joe goes *' do_follow = go %3
```

When `/follow Joe` runs, `%{1}` is replaced by `Joe` and `%%3` by `%3`, so the
macro that actually gets created is
`/def -t"Joe goes *" do_follow = go %3`. When *that* macro later fires, `%3`
is the third word of the line from the world.

The same applies to `$[...]` and `$(...)` (use `$$[...]`) and to `\`:
backslashes are halved at each level.

The `/for` command is a frequent source of this problem because its commands are
expanded again when the loop runs:

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

`%{1}` is substituted when `/countdown` runs; `$$[...]` survives as `$[...]`
and is evaluated on each pass of the loop, when `i` has its loop value. With a
single `$` the expression would be evaluated once, up front, and every line
would print the same number.

## Pitfalls

* **Forgetting that triggers' parameters are words.** In a trigger,
  `%1` is the first *word* of the line, not the first `*` in the pattern. See
  [Triggers](Triggers.md#what-the-macro-receives).
* **Expecting variables to be substituted in a trigger pattern.** They are not;
  the pattern is fixed when you define the macro. Redefine the trigger from a
  macro instead (see the nested-definition example above).
* **Putting `%x` at the top level of a script.** It is not substituted there;
  see the [evaluation model](Evaluation-Model.md).
* **Text from the world.** `%`, `$` and `\` in text you later pass through
  `/eval` are live. Use `/eval -s0` for untrusted text.

## See also

[Evaluation model](Evaluation-Model.md) ·
[Variables and scope](Variables-and-Scope.md) ·
[Expressions](Expressions.md) ·
[Macros](Macros.md)
