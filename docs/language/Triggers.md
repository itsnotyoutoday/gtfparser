# Triggers

A *trigger* is a macro that runs automatically when a line of text arrives from
the world and matches the macro's pattern. Triggers are how you react to a game:
answer prompts, track your health, gag spam, highlight tells.

```tf
/def -t'You hit *' hitmsg = /echo trigger: %*
/trigger You hit the orc
```
```output
trigger: You hit the orc
```

`/def -t'pattern' name = body` creates one. In a running program the host
feeds incoming lines to the interpreter (`in.ProcessLine(line)` in Go, which
reports whether the line should be gagged). In scripts and in the examples
here, `/trigger text` does the same thing: it behaves as if `text` had arrived
from the world.

## Matching

The pattern's style is `glob` unless you say otherwise (`-mregexp`, `-msimple`,
`-msubstr`, or `%matching`). See [Patterns](Patterns.md) and
[Regular expressions](Regular-Expressions.md). Important points:

* A glob must match the whole line; a regexp matches anywhere.
* Lines are matched after tf strips display codes and expands tabs, so patterns
  see visible text only.
* The pattern is **not substituted**. `-t'%{name} says *'` waits for the
  literal text `%{name} says`. To trigger on a variable, define the trigger from
  a macro (see [Substitution](Substitution.md#why-you-write--inside-nested-definitions)).
* Triggers are tested against complete lines only. A partial line (a prompt) is
  a `PROMPT` [hook](Hooks.md) instead.

## What the macro receives

The line is passed to the macro as if it had been called with `/name line`:

| In the body | Value |
|---|---|
| `%*` | the whole line |
| `%1`, `%2`, ... | the line's **words** (split on spaces), whatever the pattern was |
| `%#` | number of words |
| `%0` | the macro's name |
| `%P0`, `%P1`, ... | for **regexp** triggers: the whole match and the groups |
| `%PL`, `%PR` | for regexp triggers: text left and right of the match |

```tf
/def -t'one two three' w = /echo [%1] [%2] [%3] [%#] [%*] [%{-1}] [%0]
/trigger one two three
/def -mregexp -t'^(\w+) hits you for (\d+)' dmg = /echo %P1 did %P2 (%PL|%PR)
/trigger Orc hits you for 12 damage
```
```output
[one] [two] [three] [3] [one two three] [two three] [w]
Orc did 12 (| damage)
```

A glob's `*` does **not** become a numbered capture. If you want "the thing
between", either pick it from the words (`%3`), or use a regexp group, or
slice with `%{-2}`.

Positional words and regexp groups are available for the length of that macro run.
(Triggers fired by nested `regmatch()` calls inside the body have their own;
see [Regular expressions](Regular-Expressions.md).)

## Priority and fall-through

Several macros may match one line. tf picks which run:

1. Macros are considered from **highest `-p` priority** (default 1) downward.
2. At each priority, matching **fall-through** (`-F`) macros all run.
3. The first *non*-fall-through match at a priority ends the search: among the
   non-fall-through matches at that priority, **one is chosen at random** and
   run. Lower priorities are not considered.

So the normal case is: the highest-priority match runs and nothing else does.
Two plain triggers at the same priority and the same line are a coin flip. If
you want several things to happen, use `-F`.

```tf
/def -t'foo*' -p2 hi5 = /echo high priority
/def -t'foo*' -p1 lo = /echo low priority
/trigger foobar
```
```output
high priority
```

A fall-through macro runs and lets the search go on:

```tf-noverify
/def -p3 -F -t'baz*' fa = /echo fa (fall-through)
/def -p2 -F -t'baz*' fb = /echo fb (fall-through)
/def -p1 -t'baz*' fc = /echo fc (last)
/def -p0 -t'baz*' fd = /echo fd (never reached)
/trigger bazooka
```
```output
fa (fall-through)
fb (fall-through)
fc (last)
```

A negative `-p` is treated as "not given" (priority 1).

The common use of fall-through is a high-priority *observer* that records or
displays something without stopping the usual trigger behind it.

## Shots

`-n count` deletes the macro after it has run `count` times (`-1` is the same as
`-n1`). `-n0` is permanent, the default. Only `/def` can create these.

```tf
/def -n2 -t'shot' s1 = /echo shot
/trigger shot
/trigger shot
/trigger shot
```
```output
shot
shot
```

## Conditions

`-E expr` adds a condition tested each time a line could match: the macro is
only considered when the expression is true. Keep it to cheap variable tests. It
is evaluated for every line, and **`%1`, `%P1` and friends are not available** in it.

```tf
/def -E'flag == 1' -t'cond*' guarded = /echo guard passed
/set flag=0
/trigger cond one
/set flag=1
/trigger cond two
```
```output
guard passed
```

A condition that is false also stops the macro from blocking lower-priority
macros, since it is not a match at all.

## Chance

`-c percent` makes a match run only that percent of the time. `-c0` never runs,
`-c100` always; a negative value means "not given" (100). The macro still counts
as a match for priority purposes even when it does not run.

```tf-noverify
/def -c0 -t'never*' p0 = /echo never
/def -c100 -t'always*' p100 = /echo always
/trigger never x
/trigger always x
```
```output
always
```

## Gag and hilite

Attributes change how the matching line is shown. The most used:

* `-ag` **gags** the line: it is not displayed. `/gag pattern` is a shortcut.
* `-ah` **hilites** it (also `-aB` bold, `-au` underline, `-aC...` colors).
  `/hilite pattern` is a shortcut.
* `-P` **partial hilite**: apply attributes to just a part (`-PL`, `-PR`, `-P0`,
  `-P1`...) of a regexp match, such as `-P1Bgreen`.

A macro's body is optional, so a pure gag needs none:

```tf-noverify
/def -ag -t'*has connected.' gag_connects
/def -ah -t'* tells you: *' hilite_tells
/def -mregexp -P1Cred -t'HP: (\d+)' red_hp
```

Display is the host's job. The interpreter works out which attributes apply and
reports whether the line is gagged; your program decides how to draw them.

## `/trigger`

`/trigger text` runs the matching triggers as if `text` had arrived. It returns
the number of macros that ran. Options: `-n` list the matches instead of
running them, `-l` list them in full, `-g` and `-w world` restrict to global or
a world's macros, and `-h event` fires [hooks](Hooks.md) instead of triggers. It
is the tool for testing your triggers.

## Worlds

`-w world` and `-T type` restrict a trigger to text from one world (or worlds of a
type). This matters to clients with several connections; a single-connection
host does not need them.

## A worked example: health tracking

```tf
/set hp=0
/def -mregexp -t'^HP: (\d+)/(\d+)' hpbar = /set hp=%P1%; /set maxhp=%P2%; /if (hp * 4 < maxhp) /echo LOW HEALTH: %hp of %maxhp%; /endif
/trigger HP: 80/100
/trigger HP: 20/100
/echo hp is now %hp
```
```output
LOW HEALTH: 20 of 100
hp is now 20
```

## Pitfalls

* **Equal priorities.** Two non-fall-through triggers at the same priority and
  matching the same line: one is picked **at random**. Give them distinct
  priorities or use `-F`.
* **Unanchored regexps** firing on text you did not intend.
* **Case.** Globs ignore case, regexps use smart-case.
* **Using `%1` expecting the first `*`.** It is the first *word*.
* **Variables in the pattern.** Not substituted. Define from a macro.
* **Triggers that send text** which comes back as a line that matches the same
  trigger: an infinite loop. Use `-n1`, a guard, or a priority scheme.
* **Doing too much per line.** Every trigger is tested against every line;
  make patterns specific at the start.
* **`/eval`ing trigger text** (`%*`) with substitution on. Use `/eval -s0`.

## See also

[Macros](Macros.md) ·
[Patterns](Patterns.md) ·
[Regular expressions](Regular-Expressions.md) ·
[Hooks](Hooks.md) ·
[Substitution](Substitution.md)
