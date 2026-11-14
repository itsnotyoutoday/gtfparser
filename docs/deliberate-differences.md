# Deliberate differences from TinyFugue

gtfparser reproduces tf's results, formatting, warnings and errors, including quirks that follow
from tf's own rules. It deviates only where tf's output is a defect of the *implementation* (integer
narrowing, undefined or platform-dependent behavior, crashes, hangs, unbounded resource use). Each
deviation is pinned by a unit test (`compat_test.go`, `TestDeliberateDeviations`) and by the scripts
in `testdata/deviations`, which `scripts/difftest.py` checks against real tf (it must still differ).

Reference configuration: tf 5.0 beta 8, UTF-8 (WIDECHAR) build, 64-bit arm64 macOS, locale
`en_US.UTF-8`.

## Implementation defect: integer narrowing

tf's `reduce_arithmetic()` stores operands and results of `+ - * /` in a 32-bit `int` on a 64-bit
platform (a porting defect; its own help says integers become reals only on overflow). gtfparser keeps
64-bit integers: results become reals only when the exact result overflows int64.

| Script | TinyFugue: | gtfparser: |
|---|---|---|
| `/test E(1 + 4294967296)` | `1` | `4294967297` |
| `/test E(2147483647 + 1)` | `2147483648.` | `2147483648` |
| `/test E(+2147483648)` | `-2147483648` | `2147483648` |
| `/test E(100000 * 100000)` | `10000000000.` | `10000000000` |
| `/test E(1 / 4294967296)` | error (division by zero) | `0` |
| `/set x=9223372036854775807` then `/test E(++x)` | `-9223372036854775808` (signed overflow, undefined in C) | error `integer overflow in ++x` (tf's own message, which its guard never reaches) |
| `/test E(9223372036854775807 + 1)` | `0` | `9.22337203685478e+18` |

Why: the narrowing contradicts tf's documentation and yields mathematically wrong values.
Tests: `testdata/deviations/int64`, `TestDeliberateDeviations`.

## Implementation defect: byte-wise case mapping and character codes

| Script | TinyFugue: | gtfparser: |
|---|---|---|
| `/test E(tolower("ÀÉÎ"))` | invalid UTF-8 (first bytes remapped as Latin-1) | `àéî` |
| `/test E(toupper("àéîõü", 2))` | bytes counted: `Ã` corrupted | `ÀÉîõü` (count is in characters) |
| `/test E(char(8364))` | one byte (the code modulo 256) | `€` |
| `/test E(trunc(1e30))` | platform dependent (`LONG_MAX` on arm, `LONG_MIN` on x86) | `9223372036854775807` (clamped) |

Why: tf maps single bytes through the C library, which corrupts multibyte text; the real-to-integer
conversion of an out-of-range value is undefined behavior in C. `char(n)` for n below 256 still
returns the single byte n, as tf does (a lone byte above 127 is not valid UTF-8; judgement call).
Tests: `testdata/deviations/strings`.

## Resource ceiling: `strrep`, `pad`, `replace`

| Script | TinyFugue: | gtfparser: |
|---|---|---|
| `/test E(strrep("x", 99999999999999999999))` | hangs | warning `strrep: result would exceed the string size limit (16777216 bytes)`, no value |
| `/test E(pad("a", 99999999999))` | crashes or hangs | the same warning for `pad` |

The ceiling is `Limits.MaxString` (default 16 MiB), owned by the embedding program.
Tests: `TestDeviationWarnings`.

## Implementation defect: internal-error block after a macro syntax error

Reference tf follows some macro syntax errors with `Internal error at expr.c:702 ... bad op
0x0000` and a `cmd:` dump (the compiler leaves a dangling opcode behind). gtfparser prints only the
syntax error; the macro does not run in either. Test: `TestDiagnosticsMatchTf` (each syntax-error
case expects exactly one line) and `TestBrokenBodyRunsNothing`.

## Matched quirks that look like bugs (not deviations)

These follow tf's own rules and are reproduced: float printing without a trailing `.` for negative
numbers (`-2.0*1` prints `-2`), `nan`/`inf`, evaluation order of identifiers (`y - --y` is 0),
`++f` truncating a float, `strcmp` returning the byte difference, `tan("1:30")` using `strtod`
(1) while arithmetic reads `"1:30"` as a time, `tfopen(x)` taking a single argument as the mode,
`1 ? 2:3` being a syntax error.

After `Invalid command. Aborting.` tf also prints `last command is incomplete because of trailing \`
(its loop breaks without clearing the command buffer). The message is misleading but harmless, so it
is reproduced (`TestDiagnosticsMatchTf`, "non-command line aborts load").

## Known differences not replicated

- `%{n-}` with an empty default discards the text before it in the statement in tf; gtfparser treats
  it as an ordinary default.
- tf hangs when a runtime error (`1/0`) happens in a macro run from a loaded file under a pty;
  gtfparser reports the error and continues.
- `%?` expansions inside a macro called as a function hang tf in the reference run; gtfparser does
  not hang.
