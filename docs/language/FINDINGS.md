# Findings: engine versus real tf, found while writing the language guide

Each entry gives a snippet, what the reference `tf` (5.0, UTF-8 build, driven the
way `scripts/difftest.py` does) does, and what this engine does. Every entry below was re-checked against engine v0.0.5 and is **still open at v0.0.5**. Entries the triggers/hooks rewrite fixed (fall-through to lower priorities, `/purge` and invisible macros, case-insensitive names, `/trigger` options and hook firing, `1 ? 2:3`, `%?` at macro start) were removed.

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

### 2. `tfopen` modes `p` and `q`

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

### 3. `/require`, `/loaded` and `#` comments

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

### 4. `/split` does not set `%P1`/`%P2`; `getopts` accepts a detached numeric argument

```tf-noverify
/def splitter = /split %{*}%; /echo name=[%{P1}] body=[%{P2}]
/splitter greet = say hello there
/def nm = /if (!getopts("x#", "")) /return 0%; /endif%; /echo x+1=$[opt_x + 1]
/nm -x 41
```

* real tf: `name=[greet] body=[say hello there]`; `/nm -x 41` fails option
  parsing (the `#` argument must be attached: `-x41`).
* engine: `%P1`/`%P2` are empty; `/nm -x 41` runs with `opt_x` empty.

### 5. `ismacro()` and `isvar()` as functions

```tf-noverify
/def a2 = /echo two
/set zzz=1
/echo [$[ismacro('a2')]] [$[isvar('zzz')]]
```

* real tf: the macro's number (a positive integer) and `1`.
* engine: `% ismacro: no such function` (the commands exist; the function forms do not).

### 6. `/return` with no expression

```tf-noverify
/def r0 = /return
/echo [$[r0()]] [$[whatis(r0())]]
```

* real tf: `[0] [int]`.
* engine: `[] [string]`, which is what the tf manual says ("the empty string");
  the reference binary disagrees with its own manual.

## Number formatting and parsing differences

### 7. Whole negative floats print without the trailing `.`

```tf-noverify
/echo $[-1.5 * 2] $[1.5 * 2]
```

* real tf: `-3 3.`
* engine: `-3. 3.`

### 8. 32-bit boundary cases

```tf-noverify
/echo $[123456789012 + 1] $[-2147483648 + 0]
```

* real tf: `-1097262571` (a 64-bit literal is truncated to 32 bits before the
  addition) and a float `-2147483648`.
* engine: `123456789013.` (a float; the literal is kept whole). `-2147483648 + 0` now
  matches (`-2147483648`).

Results that overflow 32 bits become floats in both, which is the rule the guide
teaches.

### 9. A lone `$` before a space inside a macro body

```tf-noverify
/def m3 = /echo [a $ b]
/m3
```

* real tf: `[a  b]` (the `$` is dropped; the manual says a lone `$` is left alone).
* engine: `[a $ b]`, as the manual says. Use `$$` for a literal dollar sign.

### 10. Recursion, subroutine calls and backtracking verbs (by design)

```tf-noverify
/echo $[regmatch('(a(?1)?b)', 'aabb')] $[regmatch('(*SKIP)a', 'a')]
```

* real tf (PCRE2): both match.
* engine: both are rejected (`recursion and subroutine calls are not supported`)
  and the match is 0. Documented in Regular-Expressions.md.

### 11. `regmatch()` return value with an unmatched optional group

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

