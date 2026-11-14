# Known differences: language

gtfparser reproduces TinyFugue's results, formatting, warnings and errors exactly. It deviates only where TinyFugue's output is a defect of its implementation; those are deliberate differences, registered and explained (see [Deliberate differences](Deliberate-Differences.md)). Everything else on this page is a bug we intend to fix.

The reference is TinyFugue 5.0 beta 8 (UTF-8 build); every entry was re-run against gtfparser v0.0.6. Scripts are loaded from a file, so a macro's body is substituted when it runs and top-level lines are not (see [Evaluation model](Evaluation-Model.md)). Warnings go to the error stream and are shown here as `% ...` lines; `FILE` stands for the script's name.

## A detached numeric option argument

`getopts()` takes a `#` (integer) argument only when it is attached to its letter. tf stops with an expression error when it is detached; gtfparser accepts it and leaves the option unset.

| Script | TinyFugue prints | gtfparser prints |
|---|---|---|
| `/def nm = /if (!getopts("x#", "")) /return 0%; /endif%; /echo x+1=$[opt_x + 1]` then `/nm -x 41` | nothing on the screen; warning `% FILE, line 5: nm -x: expression syntax error: expected operand, found end of body.` | `x+1=1` |
| the same, `/nm -x41` | `x+1=42` | `x+1=42` |

## A lone `$` followed by a space

The manual says a lone `$` is left alone; the tf binary drops it. gtfparser follows the manual.

| Script | TinyFugue prints | gtfparser prints |
|---|---|---|
| `/def m3 = /echo [a $ b]` then `/m3` | `[a  b]` | `[a $ b]` |

Write `$$` when you mean a dollar sign, which works the same in both.

## A malformed substitution abandons the whole macro

tf expands a macro's whole body before running any of it, so a malformed `%{...}` stops the macro before its first command. gtfparser runs the commands before the bad one.

| Script | TinyFugue prints | gtfparser prints |
|---|---|---|
| `/def b1 = /echo before%; /echo [%{ 1}]` then `/b1` | nothing; warning `% FILE, line 2: b1: unmatched { or illegal character ' '` | `before`; the same warning |

## `%{n-}` with an empty default

tf discards the text in front of the substitution (in the same statement) when the default is empty; gtfparser treats it as an ordinary empty default.

| Script | TinyFugue prints | gtfparser prints |
|---|---|---|
| `/def f = /echo [%{1-}] after` then `/f x` | `] after` | `[x] after` |
| `/def g = /echo pre %{1-} post` then `/g x` and `/g` | `post` and `post` | `pre x post` and `pre  post` |

## Regexps: recursion, subroutine calls and backtracking verbs

gtfparser's regexp engine does not implement them; the pattern is rejected and `regmatch()` returns 0.

| Script | TinyFugue prints | gtfparser prints |
|---|---|---|
| `/echo $[regmatch("(a(?1)?b)", "aabb")]` (in a macro) | `2` | `0`; warning `% FILE, line 2: regmatch: regmatch: regexp error at character 4: recursion and subroutine calls are not supported` |
| `/echo $[regmatch("(*SKIP)a", "a")]` (in a macro) | `1` | `0`; warning `% FILE, line 4: regmatch: regmatch: regexp error at character 6: (*VERB) not recognized or malformed` |

## Runtime errors in a loaded macro under a pty, and `%?` in a function call

These two are tf hangs that gtfparser does not reproduce.

| Script | TinyFugue prints | gtfparser prints |
|---|---|---|
| `1/0` evaluated in a macro run from a loaded file (pty) | hangs | the error `division by zero`, then continues |
| `%?` expanded inside a macro called as a function | hangs in the reference run | the value, no hang |

## Not covered here

Display attributes (colours, hilites) are the host's business and `/list` output carries macro numbers that depend on what was defined before, so neither is compared on this page.
