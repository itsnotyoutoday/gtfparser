# Expressions

Expressions are tf's calculator: arithmetic, comparisons, string matching,
assignments and function calls. They appear in `$[...]` substitutions, in the
condition of `/if` and `/while`, in `/test`, `/return`, `/result`, in numeric
options such as `/def -p`, and in `/def -E`.

```tf
/set hp=42
/echo hp is $[hp], double that is $[hp * 2]
/test hp := hp - 10
/echo now $[hp]
```
```output
hp is 42, double that is 84
now 32
```

## Operands

| Written as | Example | Notes |
|---|---|---|
| integer | `42`, `-7` | whole numbers |
| decimal | `1.5`, `0.25` | up to 6 decimal places, exact (fixed point) |
| float | `1e3`, `1.5e-2`, `1.1234567` | exponent form, or more than 6 decimal places |
| time duration | `0:01:02`, `1:30`, `0:00:01.5` | hours:minutes[:seconds]; evaluates to seconds as a decimal |
| string | `"hello"`, `'hello'`, `` `hello` `` | any of the three quotes, the same at both ends |
| variable | `hp`, `name` | just the name, no `%` |
| substitution | `{1}`, `{*}`, `{P1}`, `{var-default}` | braces required, no `%` |
| macro body | `${name}` | rarely used |
| command output | `$(/cmd)` | |
| function call | `strlen(name)` | |

Inside a quoted string, write `\"` (or `\'`) for a quote of the same kind. Other
backslashes are kept as typed: `"a\nb"` is four characters, not a newline. (Use
`\n` and friends in macro body text, where [substitution](Substitution.md#backslash-escapes)
handles them, or `char(10)`.)

Inside an expression, the special parameter selectors (`{1}`, `{*}`, `{#}`,
`{L}`, `{P1}`, `{?}`) need braces; ordinary variables are written bare:

```tf
/def total = /echo $[{1} + {2}] items, tax $[{1} * rate / 100]
/set rate=8
/total 50 25
```
```output
75 items, tax 4
```

## Types

Every value is one of four types. You rarely need to think about it, but when
arithmetic surprises you the type is almost always why. `whatis(x)` reports a
value's type.

| Type | Where it comes from | Prints as |
|---|---|---|
| `int` | integer literals; `+ - *` of two ints; `/` of two ints (truncated); results of `strlen()` etc. | `42` |
| `decimal` | literals with up to six decimal digits; time literals; `+` or `-` involving a decimal | at least one decimal digit: `1.5`, `7.0`, `62.0` |
| `float` | exponent literals; seven or more decimal digits; `*` or `/` involving a decimal; most math functions (`sqrt`, `pow`...); int arithmetic that overflows 64 bits | `%sigfigs` significant digits, plus a trailing `.` when the text is all digits: `2.5`, `3.`, `-3`, `1e+20`, `nan`, `inf` |
| `string` | quoted text; variables set with `/set`, `/let` | as written |

```tf
/echo [$[whatis(7)]] [$[whatis(1.5)]] [$[whatis(1e3)]] [$[whatis("hi")]] [$[whatis(0:01:02)]]
/echo $[7 / 2] $[7 / 2.0] $[1.5 + 1.5] $[1.5 * 2] $[sqrt(16)]
```
```output
[int] [decimal] [float] [string] [decimal]
3 3.5 3.0 3. 4.
```

Things to take from that:

* **Integer division truncates toward zero**: `7 / 2` is `3`, and `-5 / 2` is
  `-2`. Make one side a decimal to get a fractional result, and note that the
  fractional result is then a *float*.
* **Decimals are exact.** `0.1 + 0.2 == 0.3` is true, because decimal addition
  and subtraction is done in fixed-point with six places. As soon as you
  multiply or divide, you are in floating point.
* **Integers are 64-bit.** `+`, `-` and `*` stay integers (so `1 + 4294967296` is
  `4294967297`) and become floats only when the exact result does not fit in 64
  bits. Floats that are whole print with a trailing dot, which is how you can tell
  (`9223372036854775807 + 1` is `9.22337203685478e+18`). The reference `tf`
  narrows to 32 bits here; that is a
  [deliberate difference](Deliberate-Differences.md).

```tf
/echo $[46341 * 46341] $[2147483647 + 1] $[1 + 4294967296] $[9223372036854775807 + 1]
/echo $[1.5 * 2] $[-1.5 * 2] $[0 / 0.0]
```
```output
2147488281 2147483648 4294967297 9.22337203685478e+18
3. -3 nan
```

* Floats print with `%sigfigs` significant digits (15 by default), in plain
  notation when the exponent is small enough and in exponent form otherwise. A
  `.` is added only when the text is all digits, so a negative whole float prints
  as `-3` (tf does the same). Dividing by zero is an error for integers
  (`division by zero`), and an infinite result of `+ - * /` on floats is
  `/ operator: arithmetic overflow`.

### Strings become numbers by their prefix

When an operator wants a number and gets a string, the leading numeric part is
used and the rest ignored; a string with no numeric prefix is `0`. This is
extremely permissive and is the source of most expression bugs.

```tf
/echo $["12abc" + 1] $["abc" + 1] $["1.5" + 1] $[+"7"]
```
```output
13 1 2.5 7
```

The unary `+` is the idiom for "treat this string as a number".

### Truth

A value is false when it is numerically zero and true otherwise. **A string
with no numeric prefix is therefore false**, as is the empty string, and a string
that starts with `0`:

```tf
/def truth = /echo "%1" is $[{1} ? 'true' : 'false']
/truth 1
/truth 0
/truth yes
/truth 5apples
```
```output
"1" is true
"0" is false
"yes" is false
"5apples" is true
```

So `/if (name)` is not a test for "name is non-empty"; use `strlen(name)` or
`name !~ ""`.

## Operators

Highest precedence first. All binary operators group left to right except
assignment, which groups right to left.

| Level | Operators | Meaning |
|---|---|---|
| 1 | `( )`, `f(args)` | grouping, function call |
| 2 | `!` `+` `-` `++` `--` | not, unary plus (string to number), negate, pre-increment/decrement |
| 3 | `*` `/` | multiply, divide |
| 4 | `+` `-` | add, subtract |
| 5 | `==` `!=` `=` | **numeric** equal / not equal (`=` is the same as `==`) |
| 5 | `=~` `!~` | **string** equal / not equal (case sensitive) |
| 5 | `=/` `!/` | string matches / does not match a **glob** |
| 5 | `<` `<=` `>` `>=` | **numeric** comparisons |
| 6 | `&` | logical and (short-circuit) |
| 7 | `\|` | logical or (short-circuit) |
| 8 | `c ? a : b`, `c ?: b` | conditional |
| 9 | `:=` `+=` `-=` `*=` `/=` | assignment |
| 10 | `,` | evaluate both, result is the right one |

All the comparison operators in level 5 share one level and group left to right.

Because the comparisons share one level, `0 == 1 < 2` means `(0 == 1) < 2` and
`1 < 2 == 1` means `(1 < 2) == 1`; chained comparisons do not mean what they do
in mathematics. Use parentheses.

There is **no modulo operator** (`%` means something else in tf) and no `&&`,
`||`, `^` or `<<`. Use the functions `mod(a, b)` and `abs()`, and `&` and `|`
for logic.

### Numeric versus string comparison

This is the most important rule in the table. `==`, `<`, `>` and friends compare
*numbers*, so strings are converted first, usually to `0`:

```tf
/echo $["abc" == "abd"] $["abc" =~ "abd"] $[5 == "+5"] $[5 =~ "+5"]
/echo $["10" < "9"] $[strcmp("10", "9") < 0]
```
```output
1 0 1 0
0 1
```

Use `=~` / `!~` to compare strings for equality, `strcmp()` to order them, and
`==` only when you mean numbers.

`=/` compares against a glob pattern (see [Patterns](Patterns.md)), which makes
quick case-insensitive tests easy, since globs ignore case:

```tf
/def classify = /echo $[{1} =/ 'orc*' ? 'an orc' : 'something else']
/classify Orcish
/classify Elf
```
```output
an orc
something else
```

`&` and `|` return the value of the last operand evaluated, and stop early:

```tf
/echo $[0 | 7] $[3 & 4] $[0 & (x := 99)] $[x]
```
```output
7 4 0 
```

### Conditional, comma, assignment

`cond ? a : b` picks one side. A missing middle (`cond ?: b`) returns `cond`
itself when it is true. **A digit directly before a colon is read as a time**
(`1:30`), so in `c ? 1 : 2` keep spaces around the `:`.

`:=` assigns and yields the assigned value; `+=`, `-=`, `*=` and `/=` update;
`++x` and `--x` increment first. The comma evaluates left to right and keeps the
last result, which lets you assign inside a condition:

```tf
/test n := 5
/echo $[n += 2] $[++n] $[n * 2]
/echo $[(a := 3, a * a)]
```
```output
7 8 16
9
```

Assignment with `:=` keeps the value's type. That is different from `/set` and
`/let`, which always store text; you notice when you store a float and read it
back:

```tf
/set s=1.5
/test d := 1.5 * 2
/echo [$[whatis(s)]] [$[whatis(d)]]
```
```output
[id:string] [id:float]
```

(`whatis` of a variable name reports `id:` plus the type of the value it holds.)

An assignment to a name that is not a local variable creates a **global**, as if
by `/set`. See [Variables and scope](Variables-and-Scope.md).

## Functions

`name(arg, ...)` calls a builtin function or a user macro. Macros are called
with the arguments as their positional parameters, and the call evaluates to
the macro's return value (so macros that finish with `/return` or `/result`
work as functions).

```tf
/def hypot = /return sqrt({1} * {1} + {2} * {2})
/echo $[hypot(3, 4)]
/echo $[strlen("dragon")] $[toupper("hi")] $[substr("treasure", 2, 3)]
```
```output
5.
6 HI eas
```

A few of the most useful builtins; the full list is in the tf manual under
"functions":

| Function | Result |
|---|---|
| `strlen(s)`, `substr(s, start, len)`, `strcat(...)`, `strstr(s, t)` | string basics |
| `toupper(s)`, `tolower(s)`, `strcmp(a, b)` | case and ordering |
| `regmatch(re, s)`, `replace(s, old, new)` | see [Regular expressions](Regular-Expressions.md) |
| `abs(x)`, `mod(a, b)`, `trunc(x)`, `sqrt(x)`, `pow(x, y)`, `rand(n)` | math |
| `whatis(x)` | the type of a value |
| `getopts(spec)` | [option parsing](Options-and-getopts.md) |
| `tfopen`, `tfread`, `tfwrite`, `tfclose` | [streams](Files-and-Streams.md) |
| `eval(text)` | runs text as a macro body, like `/eval` |

## `/test`, and what `/if` evaluates

`/test expr` evaluates an expression for its effect (assignments, function
calls) and its return value; `/if (expr)` and `/while (expr)` test the value
for truth. In a macro body `/test` is an ordinary command, so `%x` and `$[...]` inside its
argument are substituted *before* the expression is parsed. Inside the expression
itself, use bare names.

```tf
/def bump = /test counter := counter + 1
/set counter=0
/bump
/bump
/echo counter is $[counter]
```
```output
counter is 2
```

## Pitfalls

* **`"abc" == "abc"` is true, and so is `"abc" == "xyz"`.** Both are `0`. Use `=~`.
* **`/if (name)`** is false for any name that does not start with a nonzero
  digit. Use `/if (strlen(name))`.
* **Integer division**: `percent = hp / maxhp * 100` is `0` unless `hp ==
  maxhp`. Multiply first: `hp * 100 / maxhp`.
* **Mixing `%x` into expressions.** `$[%x + 1]` works, but only because `%x`
  was substituted *before* the expression ran; if `x` is empty you get `$[ +
  1]`. Prefer the bare name.
* **`1 ? 2:3`**: a digit directly before the colon makes `2:3` a time literal,
  and tf reports a syntax error. Write `1 ? 2 : 3`.

## See also

[Substitution](Substitution.md#expr-expressions) ·
[Variables and scope](Variables-and-Scope.md) ·
[Control flow](Control-Flow.md) ·
[Patterns](Patterns.md) ·
[Idioms and pitfalls](Idioms-and-Pitfalls.md)
