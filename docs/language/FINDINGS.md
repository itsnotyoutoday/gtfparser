# Findings: engine versus real tf, found while writing the language guide

Each entry gives a snippet, what the reference `tf` (5.0, UTF-8 build, driven the
way `scripts/difftest.py` does) does, and what this engine does. Items marked
**(rewrite)** concern code the triggers/hooks/macros rewrite is expected to
replace; re-check them after it merges.

## Bugs

### 1. Nested `/if` right after the outer condition closes the outer block

```tf-noverify
/def n = /if ({1}) /if ({2}) /echo both%; /endif%; /else /echo outer false%; /endif
/n 0 1
```

* real tf: `outer false`
* engine: prints nothing (the inner `/endif` ends the outer block, so the outer
  `/else` is never reached).

The nested `/if` is on the same statement as the outer condition. Starting the
outer body with another command, or `%;`, avoids it: `/if ({1}) %; /if ...`.
Cause: `execIf` (interp.go) counts nesting only for statements that begin with
`/if`, and here the inner `/if` is inside the first statement.

### 2. Fall-through does not reach lower priorities (rewrite)

```tf-noverify
/def -p3 -F -t'baz*' fa = /echo fa
/def -p2 -F -t'baz*' fb = /echo fb
/def -p1 -t'baz*' fc = /echo fc
/trigger bazooka
```

* real tf: `fa`, `fb`, `fc`
* engine (before the rewrite): `fa` only. `ProcessLine` stops considering
  macros below the first match's priority even when that match is `-F`.

### 3. `/purge` removes invisible macros; macro names are case-sensitive (rewrite)

```tf-noverify
/def -i hid = /echo hid-body
/purge
/hid
/def Foo = /echo FooBody
/foo
```

* real tf: `hid-body` (invisible macros survive a bare `/purge`) and `FooBody`.
* engine: `/hid` no longer exists and `/foo` is not found.

### 4. `/trigger` ignores its options; hooks cannot be fired from scripts (rewrite)

```tf-noverify
/def -hCONNECT hc = /echo connected: %1
/trigger -hCONNECT mymud
```

* real tf: `connected: mymud`.
* engine (before the rewrite): `/trigger` skips all `-` options and runs line
  triggers on the rest, so no hook runs. `Interp.Hook(event, args)` fires hooks
  from Go, but ignores `-p`, `-F`, `-n`, `-E`, `-m`, and did not give the macro `%1`.

Also ignored by `ProcessLine`/`Hook` before the rewrite: `-c` (chance), `-w` and
`-T` (a world-restricted macro matched text from any world), and display
attributes other than `-ag` (stored and listed, not reported to the host).

The examples in Triggers.md and Hooks.md that depend on these are tagged
`tf-noverify` until the rewrite lands; their output blocks are what real tf prints.

### 5. `tfopen` modes `p` and `q`

```tf-noverify
/test q := tfopen()
/test tfwrite(q, 'alpha')
/test tfread(q, a)
/echo %a
```

* real tf: `alpha` (`tfopen()` opens an unnamed queue; mode `'p'` runs a shell
  command and reads its output).
* engine: `tfopen()` returns `-1`; mode `'p'` tries to open the command as a file
  name. Only `r`, `w`, `a` work.

### 6. `/require`, `/loaded` and `#` comments

```tf-noverify
/loaded gtfdoc_lib2.tf          ; first line of /tmp/gtfdoc_lib2.tf
/require /tmp/gtfdoc_lib2.tf
/require /tmp/gtfdoc_lib2.tf
```

* real tf: the file's own `/loaded` call stops a second read; it loads once.
* engine: `/require` skips a file only when the name it was given equals a token
  already registered, so a token that differs from the argument never matches and
  the file loads twice. (The guide recommends the full path as the token, which
  works in both.)

Also: lines beginning with `#` are ignored by real tf's `/load`; the engine sends
them to the world as text (only `;` comments are skipped).

### 7. `/split` does not set `%P1`/`%P2`; `getopts` accepts a detached numeric argument

```tf-noverify
/def splitter = /split %{*}%; /echo name=[%{P1}] body=[%{P2}]
/splitter greet = say hello there
/def nm = /if (!getopts("x#", "")) /return 0%; /endif%; /echo x+1=$[opt_x + 1]
/nm -x 41
```

* real tf: `name=[greet] body=[say hello there]`; `/nm -x 41` fails option
  parsing (the `#` argument must be attached: `-x41`).
* engine: `%P1`/`%P2` are empty; `/nm -x 41` runs with `opt_x` empty.

### 8. `ismacro()` and `isvar()` as functions

```tf-noverify
/def a2 = /echo two
/set zzz=1
/echo [$[ismacro('a2')]] [$[isvar('zzz')]]
```

* real tf: the macro's number (a positive integer) and `1`.
* engine: `0` and `0`.

### 9. `/return` with no expression

```tf-noverify
/def r0 = /return
/echo [$[r0()]] [$[whatis(r0())]]
```

* real tf: `[0] [int]`.
* engine: `[] [string]`, which is what the tf manual says ("the empty string");
  the reference binary disagrees with its own manual.

## Number formatting and parsing differences

### 10. Whole negative floats print without the trailing `.`

```tf-noverify
/echo $[-1.5 * 2] $[1.5 * 2]
```

* real tf: `-3 3.`
* engine: `-3. 3.`

### 11. 32-bit boundary cases

```tf-noverify
/echo $[123456789012 + 1] $[-2147483648 + 0]
```

* real tf: `-1097262571` (a 64-bit literal is truncated to 32 bits before the
  addition) and a float `-2147483648`.
* engine: `123456789013.` (a float; the literal is kept whole) and an integer
  `-2147483648`.

Results that overflow 32 bits become floats in both, which is the rule the guide
teaches.

### 12. `1 ? 2:3`

```tf-noverify
/echo $[1 ? 2:3]
```

* real tf: syntax error (`2:3` is read as a time literal).
* engine: prints `0` without complaint.

### 13. A lone `$` before a space inside a macro body

```tf-noverify
/def m3 = /echo [a $ b]
/m3
```

* real tf: `[a  b]` (the `$` is dropped; the manual says a lone `$` is left alone).
* engine: `[a $ b]`, as the manual says. Use `$$` for a literal dollar sign.

### 14. `%?` at the start of a macro

```tf-noverify
/def h = /echo [%?]
/h
```

* real tf: `%?` still holds the return value of the previous command (here the
  number `/def` returned).
* engine: `1`.

## Regexps

### 15. Recursion, subroutine calls and backtracking verbs (by design)

```tf-noverify
/echo $[regmatch('(a(?1)?b)', 'aabb')] $[regmatch('(*SKIP)a', 'a')]
```

* real tf (PCRE2): both match.
* engine: both are rejected (`recursion and subroutine calls are not supported`)
  and the match is 0. Documented in Regular-Expressions.md.

### 16. `regmatch()` return value with an unmatched optional group

```tf-noverify
/echo $[regmatch('x(y)?z', 'xz')]
```

* real tf: `1`.
* engine: `2`.

The guide only relies on "nonzero means matched".

## Not verifiable here

* Display attributes (hilites, colours) and the exact text `/list` prints (macro
  numbers vary) are host-side or build-dependent, so the guide describes them
  without runnable examples.
* Examples that write to the world (`[send]` lines) cannot be compared against the
  reference client through the harness (it has no world); those blocks were
  checked against this engine and against the documented behavior.
