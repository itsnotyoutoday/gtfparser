# Differences from other languages

A short orientation if you know Lua, JavaScript, or a Unix shell. tf resembles a
macro language more than any of them, and most surprises come from that.

## The big differences at a glance

| | Lua / JS | shell | tf |
|---|---|---|---|
| A "program" | statements, executed | lines, expanded then run | **commands** (`/name args`) and **text sent to the game** |
| Functions | first-class values | functions + commands | **macros**: named bodies with positional args; callable as `/name args` or `name(args)` |
| Variables | typed values, lexical scope | strings, global/env | **strings** (with numeric conversion), **dynamic scope** |
| Calling | `f(a, b)` | `f a b` | both: `/f a b` splits on spaces, `f(a, b)` keeps arguments whole |
| Strings | concatenation with `..` / `+` | juxtaposition | `strcat()`, or just substitution: `%{a}%{b}` |
| Quoting | quotes | quotes | none needed in commands; text is text |
| Expression syntax | everywhere | `$(( ))` | only in `$[...]`, `/if (...)`, `/test`, `/return`... |
| Equality | `==` | `[ a = b ]` | `==` numeric, `=~` string |
| Truthiness | `nil`/`false` | exit status 0 | numeric: `0` and non-numeric text are false |

## If you know shell

* tf has **no word splitting except for arguments**, and quotes do not group
  words in a command: `/f "a b"` is two arguments, `"a` and `b"`.
* `%{var}` is `${var}`; `%1` is `$1`; `%*` is `$*`; `%#` is `$#`; `%?` is `$?`;
  `$(cmd)` is the same idea (captured *echoed* output, joined with spaces).
* **Scripts are not substituted line by line as in a shell.** A line in a
  script file is executed as written; only macro bodies expand. The nearest
  shell equivalent of a macro body is a function, expanded each time it is called.
* `|` pipes processes; tf's `%|` connects the `tfout` of one command to the
  `tfin` of the next.
* There is no `$var` in commands: `$` introduces `$[expr]`, `$(cmd)`, `${macro}`.
  `%` is the variable sigil, and `%%`/`$$` are literal.

## If you know JavaScript or Lua

* There are no objects, arrays or tables. Build lists with strings of words and
  slice them with `%{-1}`, `%{L}`, or use a family of variables (`item_1`,
  `item_2`...) and expand names with a nested substitution.
* There are no closures and no lexical scope. A macro reads the variables of its
  *callers*. To capture a value, substitute it into a definition at definition time.
* Functions return one value, as a string or number (`/return`). A macro's printed
  output (`/echo`) is a separate channel, used by `$(...)`.
* There is no `else if` chain as an expression, no `switch`, no `continue`, no
  `try`. `/if ... /elseif ... /else ... /endif`, `/while`, `/for i a b`, and
  `/break [n]` are the whole set.
* **`+` is arithmetic only.** `"a" + "b"` is `0`. Concatenate with `strcat()` or by
  writing the pieces together in text.
* **`==` compares numbers.** `"abc" == "def"` is true. Use `=~` for strings.
* **Integer division truncates**: `7 / 2` is `3`.
* There is no modulo operator; use `mod(a, b)`.
* Comments are lines beginning with `;` in a script file.
* `/def` is both "define a function" and "define an event handler"; the options
  decide which.

```tf
/def js_style = /echo $[strcat('a', 'b')] $[1 + 2] $[7 / 2] $[mod(7, 3)] $["abc" == "xyz"] $["abc" =~ "xyz"]
/js_style
```
```output
ab 3 3 1 1 0
```

## If you know Lua specifically

Lua's `%` is the modulo operator and escape in patterns; tf's `%` is the
substitution character. Lua patterns have no relation to tf [globs](Patterns.md)
or [regexps](Regular-Expressions.md). tf's regexps are PCRE.

## Things that feel familiar, and are

* Short-circuit `&` and `|` (they return the last evaluated operand).
* A ternary `c ? a : b`.
* Recursion through macros called as functions.
* Pattern matching with regexps (PCRE2, so Perl-like syntax) and `regmatch()`.
* File I/O with open/read/write/close functions.

## If you know no other language

Skip this page and read [Overview](Overview.md).

## See also

[Overview](Overview.md) ·
[Evaluation model](Evaluation-Model.md) ·
[Idioms and pitfalls](Idioms-and-Pitfalls.md)
