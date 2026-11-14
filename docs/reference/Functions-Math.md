# Functions: Math

tf has two number types, integers and reals, and the math functions work on
either. A real that happens to be whole prints with a trailing dot (`4.`), and
the same holds in this engine. Angles are in radians. Everything here is also
usable in `/test`, `/if`, `$[...]` and any other expression; see the
expression chapter of the tf manual for the operators (`+ - * / % ** ...`).

## abs(n)

**What it is for:** the distance of a number from zero: damage amounts, deltas, absolute differences.

**Syntax:** `abs(n)`.

**Returns:** the absolute value of `n`, with the same numeric type as `n` (integer in, integer out).

```tf
/echo $[abs(-5)]
/echo $[abs(7)]
/echo $[abs(-2.5)]
/set old=120
/set new=95
/echo HP changed by $[abs(new - old)]
```
```output
5
7
2.5
HP changed by 25
```

**Gotchas:** text that is not a number counts as 0, so `abs("x")` is 0.

**Status:** see [Status](Status.md).

## sqrt(n)

**What it is for:** square roots, for distances and area calculations.

**Syntax:** `sqrt(n)` returns a real (same as `pow(n, 0.5)`).

```tf
/echo $[sqrt(16)]
/echo $[sqrt(2)]
/echo $[trunc(sqrt(50))]
/echo $[sqrt(3*3 + 4*4)]
```
```output
4.
1.4142135623731
7
5.
```

**Gotchas:** the result is always real, so a perfect square prints with a dot; wrap it in `trunc()` for an integer. Reals print with up to 15 significant digits. The square root of a negative number is not a number (`nan`).

**Status:** see [Status](Status.md).

## pow(n1, n2)

**What it is for:** raising to a power, e.g. experience curves.

**Syntax:** `pow(n1, n2)` returns a real. If `n1` is negative, `n2` must be a whole number.

```tf
/echo $[pow(2, 10)]
/echo $[pow(10, 3) + 5]
/echo $[trunc(pow(1.5, 4) * 100)]
```
```output
1024.
1005.
506
```

**Gotchas:** the `**` operator does the same job and is often shorter. The result is a real, so large powers print in scientific notation. The engine prints a negative base with an odd integer exponent (e.g. `pow(-8, 3)`) as a real, where real tf prints `-512` without the dot (FINDINGS.md).

**Status:** see [Status](Status.md).

## exp(r)

**What it is for:** the exponential function e^r, for growth and decay curves.

**Syntax:** `exp(r)` returns a real.

```tf
/echo $[exp(0)]
/echo $[exp(1)]
```
```output
1.
2.71828182845905
```

**Gotchas:** `exp` is the inverse of `ln`: `ln(exp(x))` is `x` up to rounding.

**Status:** see [Status](Status.md).

## ln(n) and log10(n)

**What they are for:** natural and base-10 logarithms, e.g. to count digits or scale a bar graph.

**Syntax:** `ln(n)`, `log10(n)` return reals; `n` must be positive.

```tf
/echo $[ln(1)]
/echo $[ln(10)]
/echo $[log10(1000)]
/echo digits in 98765: $[trunc(log10(98765)) + 1]
/echo log2 of 64 is $[trunc(ln(64) / ln(2) + 0.5)]
```
```output
0.
2.30258509299405
3.
digits in 98765: 5
log2 of 64 is 6
```

**Gotchas:** any other base `B` is `ln(N) / ln(B)`. Zero gives `-inf` and a negative number gives `nan` (both printed in lower case by real tf; the engine prints `-Inf` and `NaN.`, see FINDINGS.md).

**Status:** see [Status](Status.md).

## sin(r), cos(r), tan(r)

**What they are for:** trigonometry on radians, for headings and wave animation.

**Syntax:** `sin(r)`, `cos(r)`, `tan(r)`; each returns a real.

```tf
/echo $[sin(0)]
/echo $[cos(0)]
/echo $[tan(0)]
/echo $[trunc(sin(3.14159265 / 2) * 1000)]
```
```output
0.
1.
0.
1000
```

**Gotchas:** degrees must be converted: `radians = degrees * 3.14159265 / 180`.

**Status:** see [Status](Status.md).

## asin(r), acos(r), atan(r)

**What they are for:** the inverse trigonometric functions.

**Syntax:** `asin(r)` and `acos(r)` need `r` in [-1, 1]; `atan(r)` accepts any number. Results are radians (`asin`/`atan` in [-pi/2, pi/2], `acos` in [0, pi]).

```tf
/echo $[asin(1)]
/echo $[acos(1)]
/echo $[atan(1) * 4]
```
```output
1.5707963267949
0.
3.14159265358979
```

**Gotchas:** `atan(1) * 4` is a handy way to get pi. Out-of-domain arguments give `nan`.

**Status:** see [Status](Status.md).

## mod(i1, i2)

**What it is for:** the remainder after division: wrapping a counter, testing for odd/even, every n-th line.

**Syntax:** `mod(i1, i2)` returns an integer.

```tf
/echo $[mod(10, 3)]
/echo $[mod(17, 5)]
/echo [$[mod(-10, 3)]]
/echo $[mod(8, 2) == 0 ? "even" : "odd"]
```
```output
1
2
[-1]
even
```

**Gotchas:** the sign follows the first argument (C semantics). Real tf reports `mod: division by zero` and aborts the expression for a zero divisor; the engine silently returns 0 (FINDINGS.md). The `%` operator is the same operation.

**Status:** see [Status](Status.md).

## trunc(r)

**What it is for:** turning a real into an integer by dropping the fraction, as in percentages and ratios.

**Syntax:** `trunc(r)` returns an integer.

```tf
/echo $[trunc(3.9)]
/echo [$[trunc(-3.9)]]
/echo $[trunc(5)]
/echo $[trunc(30 / 120.0 * 100)]%
```
```output
3
[-3]
5
25%
```

**Gotchas:** it truncates toward zero; it does not round. To round add 0.5 first for positive numbers.

**Status:** see [Status](Status.md).

## rand([i1 [, i2]])

**What it is for:** dice rolls, random delays and random choices.

**Syntax:** `rand()` is a random integer from 0 up to a large system maximum; `rand(i)` is in `[0, i-1]`; `rand(i1, i2)` is in `[i1, i2]` (both ends included).

```tf
/echo $[rand(6) < 6]
/echo $[rand(3, 6) >= 3 & rand(3, 6) <= 6]
/echo $[rand(5, 5)]
```
```output
1
1
5
```

An in-game dice macro (output varies, so it is not run by the checker):

```tf-noverify
/def roll = /echo You roll a $[rand(1, 6)].
/roll
```

**Gotchas:** the checked lines test the bounds because the values themselves change on every run. There is no `srand()` in tf: the generator is seeded at startup. `rand(0)` and an upside-down range return a fixed value rather than failing.

**Status:** see [Status](Status.md).

### /rand (Library)

**What it is for:** the command form of `rand()`, for typing at the keyboard.

**Syntax:** `/rand`, `/rand n`, `/rand lo hi`; it echoes the number. More than two arguments is an error.

```tf
/rand 5 5
```
```output
5
```

**Status:** library macro from `stdlib.tf`; see [Status](Status.md).
