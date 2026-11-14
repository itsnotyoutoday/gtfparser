# Macros

A macro is a named list of commands. It is tf's only way to define your own
commands, functions, triggers and hooks: all of them are macros, differing in
which options they were defined with. `/def` is the command that makes them.

```tf
/def hello = /echo hello %{1-world}
/hello
/hello Bob
```
```output
hello world
hello Bob
```

## `/def`

```
/def [options] [name] [= body]
```

`name` and `body` are both optional. A macro with a trigger or hook usually has
a name too, so you can refer to it later, but it does not have to.

* **Names** should start with a letter and contain letters, digits and `_`.
  Other characters (especially `$`, `/` and `%`) get interpreted when the name is
  used. Names are **case-insensitive**: `/def Foo` is callable as `/foo`.
* **Redefinition.** Defining a name that exists replaces the old macro. (A
  `REDEF` hook fires, and tf warns on the screen unless the new body is identical.)
* **Numbers.** Every macro gets a sequential number. `/def` returns it, `/list`
  shows it, and `/#number` calls the macro with that number (see
  [Evaluation model](Evaluation-Model.md)).
* **Body.** Everything after the `=`, up to the end of the line (or the
  continued lines). It is substituted every time the macro runs. In a script
  file a `\` at the end of a line continues it.
* A macro defined **without a body** is useful for options-only effects, such
  as a gag trigger: `/def -ag -t'*spam*'`.

```tf
/def Foo = /echo FooBody
/foo
/FOO
/def keep = /echo keep v1
/def keep = /echo keep v2
/keep
```
```output
FooBody
FooBody
keep v2
```

If a macro has the same name as a builtin command, **the macro wins**. Reach the
builtin with `/@name`. (That makes it easy, and dangerous, to shadow builtins
like `/echo` or `/set`.)

### The options

Most options turn a macro into a trigger or hook. Numeric options (`-p`, `-n`,
`-c`) accept an expression, evaluated once at definition. A negative value
means "not given", so you get the default.

| Option | Meaning | Details |
|---|---|---|
| `-t'pattern'` | **trigger**: run when a line from the world matches | [Triggers](Triggers.md) |
| `-h'EVENT [pattern]'` | **hook**: run when an event occurs | [Hooks](Hooks.md) |
| `-m style` | how patterns match: `simple`, `glob`, `regexp`, `substr` (default: `%matching`, which is `glob`; `regexp` with `-P`) | [Patterns](Patterns.md) |
| `-p n` | priority of the trigger or hook (default 1) | [Triggers](Triggers.md#priority-and-fall-through) |
| `-F` | fall-through: also let lower-priority macros run | |
| `-n n` | run `n` times then delete the macro (0 = permanent, the default) | [shots](Triggers.md#shots) |
| `-1` | same as `-n1` | |
| `-c n` | percent chance (0 to 100) of actually running when matched (default 100) | |
| `-E expr` | only matches while the expression is true | [Triggers](Triggers.md#conditions) |
| `-w world` | only for text or events from this world | |
| `-T type` | only for worlds of this type | |
| `-a[attrs]` | display attributes for the trigger's text (`g` gag, `h` hilite, `u` underline, `B` bold, `C`*color*...) | [Triggers](Triggers.md#gag-and-hilite) |
| `-P[part]attrs` | partial hilite: apply attributes to part of a regexp match (implies `-mregexp`) | |
| `-b'keys'` | key binding | |
| `-i` | invisible: skipped by `/list`, `/save`, `/purge` | |
| `-q` | quiet: does not count as a match for background-activity purposes | |

Options come before the name; `-t'...'`, `-t"..."` and `-t` followed by a word
without spaces are all fine. If the pattern contains the quote character or a
backslash, escape it with a backslash.

## Managing macros

| Command | Does |
|---|---|
| `/undef name...` | remove macros by name |
| `/undefn number...` | remove by number |
| `/undeft pattern` | remove the trigger macros whose trigger is `pattern` |
| `/list [options] [name]` | show macros (`-s` short form, `-t pattern`, `-h`...) |
| `/purge [options] [name]` | remove all macros matching the same filters |

```tf
/def pa1 = /echo pa1
/def pa2 = /echo pa2
/def keep = /echo keep
/purge pa*
/keep
/def -i hid = /echo hid-body
/purge
/hid
```
```output
keep
hid-body
```

`/list` and `/purge` have the same selection options; with none, `/purge`
removes **every** visible macro, so scope it. Macros defined with `-i` (all of
the standard library's) are left alone unless you add `-i`.

## Macros as functions

Any macro can be called like a function with `name(args)` in an expression. The
value is what the macro returns (`/return`, or `/result`). Arguments become
positional parameters. See [Evaluation model](Evaluation-Model.md#the-three-kinds-of-result).

```tf
/def dbl = /return {1} * 2
/echo $[dbl(21)] $[dbl(dbl(2))]
/def noret = /echo noret
/echo [$[noret()]]
```
```output
42 8
noret
[1]
```

`dbl(dbl(2))` is `8`. `noret()` runs and echoes `noret`, then returns the value
of its last command (here `1`), so the surrounding `$[...]` is `1`.
Recursion works (`/return {1} <= 1 ? 1 : {1} * fact({1} - 1)`) up to `%max_recur`.

## Defining macros from macros

A macro body that contains a `/def` is expanded when it **runs**, so you
double the `%`, `$` and `\` for anything the inner macro should expand later
(`%%{1}`, `$$[x]`). This is the most common source of macro bugs; read
[Substitution](Substitution.md#why-you-write--inside-nested-definitions).

```tf
/def outer = /def inner = /echo inner %%{1}%; /echo outer ran
/outer
/inner hello
/def outer2 = /def inner2 = /echo inner2 %%{1}%%; /echo and more
/outer2
/inner2 hello
```
```output
outer ran
inner hello
inner2 hello
and more
```

In `outer`, the plain `%;` ends *outer's* `/def` statement, so `/echo outer ran`
is the next command of `outer`, and `inner` only echoes. In `outer2` the `%%;`
survives as `%;` inside `inner2`'s body, which makes `/echo and more` part of
`inner2`.

## Pitfalls

* **Spaces around `=`** are fine in `/def` (unlike `/set`), but a space in a
  trigger pattern needs quotes: `/def -t'You hit *' name = ...`.
* **Shadowing builtins by accident** (a macro called `echo`, `set`, `do`...).
* **`/purge` with no arguments** removes everything visible.
* **Numbers change.** A macro's number is assigned at definition and is not stable
  across sessions; use names.
* **Case.** Names are case-insensitive but variable names are not.
* **`-t` patterns are not substituted.** See [Triggers](Triggers.md).

## See also

[Triggers](Triggers.md) ·
[Hooks](Hooks.md) ·
[Patterns](Patterns.md) ·
[Substitution](Substitution.md) ·
[Options and getopts](Options-and-getopts.md) ·
[Evaluation model](Evaluation-Model.md)
