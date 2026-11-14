# Variables and scope

tf variables are simple: a name and a value. There are global variables (made
with `/set`), local variables (made with `/let`) that live for one macro call,
and a set of special variables that configure the interpreter. The one idea
that surprises people is that scope is **dynamic**.

## Global variables: `/set`

`/set name=value` creates or updates a global variable. There must be no spaces
around the `=`. A second form, `/set name value`, takes the rest of the line as
the value. `/set name` with no value displays the variable. `/unset name` removes it.

```tf
/set colour=red
/set size 10 big
/echo [%colour] [%size]
/unset colour
/echo [%colour] [%{colour-no colour}]
```
```output
[red] [10 big]
[] [no colour]
```

Names are case sensitive and should be letters, digits and underscores, starting
with a letter. `/listvar` lists variables, `/listvar -s name` lists the ones
matching a glob.

Values from `/set` and `/let` are **strings**, even when they look like numbers.
Expressions convert on demand (see [Expressions](Expressions.md#strings-become-numbers-by-their-prefix)),
so `/set n=5` followed by `$[n + 1]` works as you hope.

```tf
/set count=5
/test count := count + 1
/echo [%count] [$[whatis(count)]]
/set count=%count
/echo [$[whatis(count)]]
```
```output
[6] [id:int]
[id:string]
```

`:=` in an expression keeps the type of what it assigns, which matters only for
floats (a float stored through `/set` would be reparsed from text).

## Local variables: `/let`

`/let name=value` creates a variable in the *current macro expansion's scope*;
it disappears when that macro returns. `/let` is meant for use inside macro
bodies. At the top level of a loaded script there is no macro scope and a
`/let` does not persist.

```tf
/def counter = /let n=0%; /while (n < {1}) /let n=$[n + 1]%; /done%; /echo counted to %n
/counter 4
/echo [%n]
```
```output
counted to 4
[]
```

An expression assignment (`:=`, `+=`, `++`) updates whichever variable of that
name is nearest in scope; **if there is none it creates a global**, as if by
`/set`. That is a classic way to leak variables:

```tf
/def loc = /let n=5%; /test n := n + 1%; /echo local n=%n
/set n=100
/loc
/echo global n=%n
/def leak = /test fresh := 7
/leak
/echo the global fresh is %fresh
```
```output
local n=6
global n=100
the global fresh is 7
```

If you want a local, create it with `/let` first.

## Dynamic scope

When a variable is referenced, tf looks in the current macro's scope, then in the
scope of the macro that *called* it, then its caller, and so on out to the
globals. The caller's locals are visible to everything it calls:

```tf
/def inner = /echo inner sees x=%x%; /let x=inner-local
/def outer = /let x=outer-local%; /inner%; /echo outer still has x=%x
/set x=global
/outer
/echo global x=%x
```
```output
inner sees x=outer-local
outer still has x=outer-local
global x=global
```

`inner` read `outer`'s `x`, and `inner`'s own `/let` made a separate local that
was gone when it returned. This is *not* lexical scoping: it is the call chain
at run time, not the shape of the source, that decides what a name means.

The upside is convenient helpers that "just see" the caller's variables. The
downside is accidental capture: a helper that reads a variable it forgot to
define will silently use any variable of that name from further up. Give
helpers' locals distinctive names, or pass values as arguments.

To pin a value at the moment a macro is *defined* rather than looked up when
it runs, substitute it with `%` in the definition (see
[Substitution](Substitution.md)). The tf manual calls this simulating lexical
scope.

## Positional parameters

A macro's arguments are available as `%1`, `%2`, `%*` and `%#` (see
[Substitution](Substitution.md#positional-parameters)). Inside expressions write
them with braces: `{1}`, `{*}`, `{#}`.

There are two ways to call a macro, and they build the parameter list
differently:

```tf
/def show = /echo [%0] count=%# [%*]
/show a b
/test show("a", "b")
/test show("x y", "z")
```
```output
[show] count=2 [a b]
[show] count=2 [a b]
[show] count=2 [x y z]
```

* **Command syntax**, `/show x y z`, splits the argument text on spaces.
* **Function syntax**, `show("x y", "z")`, makes each argument one parameter
  (the last call has two parameters, `x y` and `z`; `%*` joins them with a
  space).

Positional parameters belong to the call and are inherited by `/eval` and by
the bodies of `/if`/`/while`, but **not** by a macro you call, which gets its
own. `/shift` drops leading parameters; see
[Options and getopts](Options-and-getopts.md).

## Special variables

Many variables configure the interpreter. They have fixed types and some check
what you assign: an enumerated variable only accepts its legal values.

```tf
/echo [%matching] [%sigfigs]
/set sigfigs=4
/echo $[1.0 / 3.0]
/set sigfigs=15
/set matching=bogus
/echo [%matching]
/set matching=regexp
/echo [%matching]
```
```output
[glob] [15]
0.3333
[glob]
[regexp]
```

A few you will meet in this guide:

| Variable | Default | Controls |
|---|---|---|
| `matching` | `glob` | default pattern style for `/def -t` / `-h`: `glob`, `regexp`, `simple`, `substr` (see [Patterns](Patterns.md)) |
| `sigfigs` | `15` | significant digits when printing floats |
| `sub` | `off` | substitution level for typed/loaded top-level lines (`off`, `on`, `full`) |
| `max_instr` | `1000000` | instruction budget; a runaway macro is stopped when it is used up (`max_iter` is accepted but ignored) |
| `max_recur` | `100` | limit on macro recursion depth |
| `ptime` | `1.0` | default delay for `/repeat` and `/quote` processes |
| `borg` | `on` | whether triggers fire |
| `hook` | `on` | whether hooks fire |
| `backslash`, `oldslash` | `on` | backslash handling in substitution |
| `lpquote` | `off` | whether processes wait for prompts |
| `mecho` | `off` | whether `/eval` echoes what it runs |

The embedding program can impose ceilings (for example on `max_instr` and
`max_recur`) that scripts cannot raise.

The special parameters `%1`, `%*`, `%#`, `%?`, `%L`, `%R`, `%P1`... are
*substitution-only*: they are not variables and cannot be used as bare names in
an expression.

## The environment

When an interpreter is created, the process's environment variables are imported
as global variables (an embedder can suppress this with the
`WithoutEnvironment` option). Variables defined with `/setenv`, imported from
the environment, or marked with `/export` are *exported*: they are also passed
to subprocesses started with `/sh` and `/quote !`. Plain `/set` variables stay
internal to tf unless a variable of that name was already exported.

```tf-noverify
/setenv EDITOR=vi          ; a global that subprocesses also see
/set mood=cheerful
/export mood               ; now exported too
```

## Pitfalls

* **Spaces around `=`.** `/set x = 5` is the two-argument form: it sets `x`
  to the text `= 5`. Write `/set x=5`.
* **Expecting `/let` to work at the top level of a script.** Use `/set`, or put
  the code in a macro.
* **Expecting `:=` to make a local.** It makes a global if none exists.
* **Variables named like numbers or selectors.** `/set 1=x` and names such as
  `L`, `R`, `P1` collide with the substitution selectors. Avoid them.
* **Loop counters that leak.** A `/while` loop that uses a bare `:=` on a name
  you did not `/let` leaves a global behind that the next call will start from.
* **Relying on a caller's variable by accident.** Dynamic scope makes this
  work, and makes it break when a different caller does not set it.

## See also

[Evaluation model](Evaluation-Model.md) ·
[Substitution](Substitution.md) ·
[Expressions](Expressions.md) ·
[Macros](Macros.md) ·
[Options and getopts](Options-and-getopts.md)
