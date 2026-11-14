# Deliberate differences

gtfparser reproduces TinyFugue's results, formatting, warnings and errors exactly. It deviates only where TinyFugue's output is a defect of its implementation (integer narrowing, undefined or platform-dependent behaviour, crashes, hangs, unbounded resource use). Each deviation is pinned by a unit test and by a script that must still differ from real tf. The full registry, with tests, is `docs/deliberate-differences.md` in the repository; this page lists the language-visible ones.

Reference: TinyFugue 5.0 beta 8, UTF-8 build, 64-bit arm64 macOS, locale `en_US.UTF-8`.

## Integers are 64-bit

tf stores operands and results of `+ - * /` in a 32-bit `int` on a 64-bit machine, although its own manual says integers become reals only on overflow. gtfparser keeps 64-bit integers; a result becomes a real only when the exact result does not fit in 64 bits.

| Script | TinyFugue prints | gtfparser prints | Why |
|---|---|---|---|
| `/test E(1 + 4294967296)` | `1` | `4294967297` | 32-bit narrowing gives a mathematically wrong value |
| `/test E(2147483647 + 1)` | `2147483648.` | `2147483648` | the sum fits in 64 bits, so it stays an integer |
| `/test E(+2147483648)` | `-2147483648` | `2147483648` | narrowing |
| `/test E(100000 * 100000)` | `10000000000.` | `10000000000` | stays an integer |
| `/test E(1 / 4294967296)` | error `division by zero` | `0` | the divisor is not zero |
| `/set x=9223372036854775807` then `/test E(++x)` | `-9223372036854775808` | error `integer overflow in ++x` | signed overflow is undefined in C; tf's own message exists but its guard never reaches it |
| `/test E(9223372036854775807 + 1)` | `0` | `9.22337203685478e+18` | the overflow becomes a real, as the manual says |

(`E` is a helper that echoes its argument.)

## Case mapping and character codes work on characters

tf maps single bytes through the C library, which corrupts multi-byte text.

| Script | TinyFugue prints | gtfparser prints | Why |
|---|---|---|---|
| `/test E(tolower("ÀÉÎ"))` | invalid UTF-8 (first bytes remapped as Latin-1) | `àéî` | byte-wise mapping |
| `/test E(toupper("àéîõü", 2))` | bytes counted; `Ã` corrupted | `ÀÉîõü` | the count is in characters |
| `/test E(char(8364))` | one byte (the code modulo 256) | `€` | the code is a character |
| `/test E(trunc(1e30))` | platform dependent (`LONG_MAX` on arm, `LONG_MIN` on x86) | `9223372036854775807` | converting an out-of-range real to an integer is undefined in C, so gtfparser clamps |

`char(n)` for `n` below 256 still returns the single byte `n`, as in tf.

## Resource ceilings

| Script | TinyFugue prints | gtfparser prints | Why |
|---|---|---|---|
| `/test E(strrep("x", 99999999999999999999))` | hangs | warning `strrep: result would exceed the string size limit (16777216 bytes)`, no value | unbounded resource use |
| `/test E(pad("a", 99999999999))` | crashes or hangs | the same warning for `pad` | unbounded resource use |

The ceiling is `Limits.MaxString` (16 MiB by default), chosen by the embedding program.

## Quirks that look like bugs but are reproduced

These follow tf's own rules and are *not* deviations: negative whole floats print without a trailing dot (`-2.0*1` prints `-2`), `nan` and `inf` print as such, `y - --y` is `0` (identifiers are read when their operator runs), `++f` truncates a float, `strcmp` returns the byte difference, `tfopen(x)` with one argument takes it as the mode, and `1 ? 2:3` is a syntax error (`2:3` is a time).

## See also

[Known differences](Known-Differences.md) ·
[Expressions](Expressions.md) ·
[Evaluation model](Evaluation-Model.md)
