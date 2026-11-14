# Options and getopts

Builtin commands take options like `-n5` or `-s'text'`. You can write macros
that accept the same syntax with `getopts()`, and `/shift` to drop what you
have consumed.

## `getopts(spec [, init])`

`spec` is a string of the option letters your command accepts:

| In the spec | Meaning |
|---|---|
| `a` | a flag; `-a` sets `opt_a` to `1` |
| `s:` | takes a string argument: `-sfoo` or `-s'two words'` |
| `n#` | takes an integer: `-n5` |
| `t@` | takes a time: `-t1:30` |

For each option found, `getopts` creates a **local** variable `opt_X` (where `X`
is the letter). It then removes the parsed options from the macro's
parameters, so afterwards `%{*}` holds only the remaining arguments. It
returns nonzero on success, and prints an error and returns 0 if it hits a bad
option.

`init` pre-sets every `opt_X` before parsing, so unused options have a defined
value; with no `init` they are left alone (they may be unset or hold whatever
the caller had).

```tf
/def foo = /if (!getopts("abn#s:", "")) /return 0%; /endif%; /echo a=[%{opt_a}] b=[%{opt_b}] n=[%{opt_n}] s=[%{opt_s}]%; /echo rest: %{*}
/foo -a -n5 -s"can't stop" whiz bang
/foo -ba -- -n whiz
/foo plain words
```
```output
a=[1] b=[] n=[5] s=[can't stop]
rest: whiz bang
a=[1] b=[1] n=[] s=[]
rest: -n whiz
a=[] b=[] n=[] s=[]
rest: plain words
```

Things to notice:

* Flags can be bundled (`-ba`), and an argument may follow its letter directly
  (`-n5`, `-sfoo`).
* `--` ends option parsing, so `-n` in `-ba -- -n whiz` was left alone as an
  ordinary argument.
* Parsing stops at the first word that is not an option.
* Numeric (`#`) arguments should be attached to their letter: write `-n5`. The
  reference client rejects `-n 5`.

A common pattern is to set defaults first and omit `init`:

```tf
/def bar = /test getopts("t:", "")%; /echo t=[%{opt_t-none}] rest=[%*]
/bar -t90 text here
/bar text only
```
```output
t=[90] rest=[text here]
t=[none] rest=[text only]
```

And one that reports errors with `/return 0`, as builtins do:

```tf
/def baz = /if (!getopts("v", 0)) /return 0%; /endif%; /echo verbose=%{opt_v} args=[%*] count=%#
/baz -v one two
/baz one two
/baz -x one
/echo returned %?
```
```output
verbose=1 args=[one two] count=2
verbose=0 args=[one two] count=2
returned 0
```

(The error text itself, `invalid option 'x'`, goes to the warning stream.)

## `/shift`

`/shift [n]` drops the first `n` (default 1) positional parameters, so `%1`
becomes what was `%2`, and `%*` and `%#` shrink.

```tf
/def sh1 = /echo before: %*%; /shift%; /echo after one shift: %*%; /shift 2%; /echo after two more: [%*]
/sh1 a b c d
```
```output
before: a b c d
after one shift: b c d
after two more: [d]
```

A loop that consumes arguments one at a time:

```tf
/def each = /while ({#}) /echo item: %1%; /shift%; /done
/each x y z
```
```output
item: x
item: y
item: z
```

## Related commands

`/split` (breaks `name = body` text into `%P1`/`%P2`) and `/escape` (backslash
escapes characters) help when writing commands like `/def`. In this engine
`/split` does not set `%P1`/`%P2` yet; see [FINDINGS](FINDINGS.md).

## Pitfalls

* **Reading `opt_x` when the option is absent** without `init`: you get empty (or a
  stale value from a caller, thanks to
  [dynamic scope](Variables-and-Scope.md#dynamic-scope)). Pass `""` or `0` as
  `init`.
* **Forgetting to `/return 0`** on `getopts` failure, so the command carries on
  with bad options.
* **Options after arguments.** Everything after the first non-option is
  an argument, so `/foo word -a` does not set `a`.
* **Using `%1` after getopts expecting the original.** The option words are
  gone from the parameters.

## See also

[Macros](Macros.md) ·
[Variables and scope](Variables-and-Scope.md) ·
[Substitution](Substitution.md#positional-parameters)
