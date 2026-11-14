# Testing and compatibility

The promise of this engine is that **tf scripts behave the way they do in real
tf**. That is a claim you can check, and the repository checks it constantly.

## Real tf is the specification

Where the documentation is vague or silent, the C client's observable behavior
decides. The test suite runs the same script through the real `tf` binary and
through this engine, and compares what they produce:

    scripts/difftest.py testdata/cases/*.tf

Each case is a small tf script (`testdata/cases/`) covering one area:
expressions and types, strings, substitution, control flow, scoping, getopts,
globs, regexps, triggers, pipes, special variables, macro management,
processes, and more. The harness drives real tf in a pseudo-terminal and
captures its output; any difference is a failing test.

## What the suite proves

| Check | What it establishes |
|---|---|
| **Differential corpus** (`testdata/cases`) | behavior of every language feature matches real tf, output line for line |
| **Golden library table** (`GTF_TF_LIB=... go test .`) | loading tf's own `stdlib.tf` and everything it pulls in defines the *exact same* 747 macros (names, options, bodies, numbering) as real tf |
| **Regexp engine** (`pcre/`) | 1,200+ pattern/subject pairs agree with real tf; direct comparison with PCRE2 10.47 on start offsets, lookbehind, invalid UTF-8; hundreds of thousands of fuzzed cases compared against PCRE2 |
| **Documented examples** (`scripts/doccheck.py`) | every example in these docs runs and prints what the docs say |
| **Unit and race tests** | `go test -race ./...` covers the Go API, limits, policy, concurrency |
| **Benchmarks** | `scripts/bench.py` compares speed with the C client |

Run it yourself:

```sh
export TFREF_DIR=/path/to/a/built/tinyfugue     # a UTF-8 build: src/tf and lib/tf
. scripts/env.sh
go build -o /tmp/gtfrun ./cmd/gtfrun
python3 scripts/difftest.py testdata/cases/*.tf
GTF_TF_LIB=$TF_LIB go test -race .
python3 scripts/doccheck.py docs README.md
```

## Subtleties the suite pins down

Things that surprise people, all matching real tf, each locked in by a test:

- Lines typed at the prompt or read from a script file get **no substitution** and
  `%;` does not split them (except after `/if` and `/while`); only macro bodies
  do.
- Numbers have **four** kinds: integer, **decimal** (fixed-point, six places:
  `1.5`, `7.0`, `0:01:02`), float, and string. `2 * 3.5` is a float (`7.`);
  `1.5 + 1.5` is a decimal (`3.0`).
- Integers are 64-bit; arithmetic that overflows int64 becomes a float (tf narrows to 32 bits
  by mistake; see [Deliberate differences](deliberate-differences.md)).
- Floats print as C `%.15g` plus a trailing `.` when the whole text is digits, so `2.0*1` prints
  `2.`, `-2.0*1` prints `-2` and `0/0.0` prints `nan`.
- Built-in *functions* win over same-named macros, which is how the standard
  library can wrap `echo()` in a `/echo` macro.
- Globs match case-insensitively; `{a|b}` matches whole words.
- Regexps are case-insensitive unless the pattern contains an uppercase letter.

## Known differences

Honest list of where this engine intentionally or currently differs:

| Area | Difference |
|---|---|
| Instruction limit | `max_instr` counts statements; real tf counts finer internal instructions, so runaway loops stop at a somewhat different point |
| Regexps | no recursion or subroutine calls (`(?R)`, `(?1)`), backtracking verbs (other than `(*FAIL)`), callouts, `(?[...])` |
| Real-tf crashes | not reproduced (for example an internal error on some nested `/if`... `/else`, and a NULL dereference in `status_width()` of a missing field) |
| Messages | the differential harness compares output only; the wording and context (`file, line N: macro: message`) of common warnings follow tf and are pinned by Go tests (`compat_test.go`) |
| Deliberate differences | a short, registered list (64-bit integers, Unicode case mapping, resource ceilings): [deliberate-differences.md](deliberate-differences.md), checked by `scripts/difftest.py testdata/deviations/*.tf` |
| Large non-integer numbers | printing follows C `%.15g` (verified), but extreme values are tested lightly |
| Unsupported builtins | commands and functions that need client features not yet implemented fail loudly; see the [status table](reference/Status.md) |

## Writing a test for your extension

Your own commands and functions deserve tests that do not need a terminal:

```go
func TestRoll(t *testing.T) {
    var out []string
    in := gtfparser.New(gtfparser.HostFuncs{OnEcho: func(s string) { out = append(out, s) }})
    in.Load(myExtension)
    in.Run("/roll -n2 6")
    if got := strings.Join(out, "|"); got != "rolling 2d6" {
        t.Fatalf("got %q", got)
    }
}
```

`HostFuncs` needs no new type, and nothing touches the terminal, files or
network unless your test asks for it.

## Reporting a difference

If you find a tf script that behaves differently here than in real tf, that is
a bug. A great report is a few lines of tf, what real tf prints, and what this
engine prints. Add it as a case in `testdata/cases/` and it stays fixed.
