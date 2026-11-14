# Diagnostics: matching real tf's warnings and errors

The goal is that every error and warning gtfparser prints is *identical* to what real
TinyFugue prints (wording, punctuation and the location prefix) wherever real tf behaves
legitimately. This page describes what tf prints, how the catalog of messages is organised,
and how to measure the gap. Nothing here changes engine behaviour; it is test infrastructure.

## What real tf prints

Diagnostics go to tf's error stream (the screen) as one line starting with `% `. The functions
are in `src/tfio.c` (`eprintf`, `tf_wprintf`, `oprintf`, `eprefix`):

| tf call | line printed |
|---|---|
| `eprintf(fmt, ...)` | `% <context><message>` |
| `tf_wprintf(fmt, ...)` | `% <context>Warning: <message>` |
| `operror(name)` | `% <context><name>: <strerror(errno)>` (capitalised C-library text, e.g. `No such file or directory`) |
| `do_hook(H_x, "!fmt", ...)` | same as `eprintf` (the `!` means "put the error prefix here"); also runs `/def -hx` macros |
| `oprintf("%% ...")`, `do_hook(H_x, "%% ...")` | `% ...` with no context (informational: `Logging to file ...`, `Writing macros to ...`) |
| `tfprintf(tferr, "%% file: line N: ...")` | custom text built at the call site (`/load` backslash problems) |
| `internal_error()` | `% <context>Internal error at file.c:NNN, <version>.  Please report ...`, then `cmd: "..."` |

### The context rule (`eprefix()` in `tfio.c`)

```
% [<file>, line N: | <file>, lines N-M: ][<command>[ -<opt>]: ][Warning: ]<message>
```

* The **location** is present while a file is being `/load`ed (including the `-f` startup file),
  naming the innermost *loading* file and the line, or `lines N-M` for a command continued with a
  trailing backslash. It is the line of the *call*, not of the `/def` that holds the code. Typed at
  the prompt there is no location.
* The **command** is `current_command`, i.e. the innermost thing running:
  * a builtin command: its upper-case name (`DEF`, `EVAL`, `UNDEF`, `LOAD`, `RECALL`); while an
    option is being parsed the option letter follows (`DEF -h: `, `DEF -Z: `, `LOG -w: `);
  * a user macro: its name (`f: `, `trg: `), only the innermost one, never a chain;
    an unnamed macro is `#N`;
  * a function call: the function name (`mod: `, `tfread: `);
  * nothing for `/test` and bare expressions: `/test nofunc(1)` gives `... line 3: nofunc: no such function`.
* `Warning: ` follows the command for `tf_wprintf` messages.
* Several messages are unaffected by the context (`oprintf`), see the `context` column below.

gtfparser today prints `% /test: nofunc: no such function` (slash plus lower-case command, no
location, no macro name). `docs/diagnostics.md` does not prescribe the fix; `scripts/warncheck.py`
reports "context" mismatches separately from "body" mismatches so each can be fixed on its own.

## The catalog: `testdata/warnings/catalog.tsv`

One row per message-emitting call in tf's C source (`src/*.c`), extracted with
`scripts/warn_extract.py` and curated by `scripts/warn_catalog.py` (which also reads the case
files). Tab separated, `\t` and `\n` escaped inside fields, one header row.

| column | meaning |
|---|---|
| `key` | `<source-stem>.<line>` (`expr.813`), suffix `.b` when a line has several calls, or `ctx.<name>` for context-rule probes |
| `kind` | `E` error, `W` warning (`Warning: ` is added), `O` oprintf info, `T` tfprintf to tferr, `H` hook default text, `I` internal error |
| `file`, `line`, `func` | where tf raises it |
| `template` | the printf template from the C source; `%s %S %d %c` are arguments, `%%` a literal `%` |
| `context` | `eprefix` (the rule above), `none` (no prefix), `custom` (call-site text) |
| `trigger` | one line describing how a script provokes it (from the case header) |
| `status` | `testable` or `untestable` |
| `case` | the case key (equal to `key`) when testable |
| `note` | why it is untestable, or remarks |

Untestable entries need a socket or OS failure, an interrupt, an environment variable the harness
fixes, compiled-out or dead code, a squat bridge, or an internal consistency check. Every one has a
reason in `note`. Entries are not exhaustive for things tf prints that are not diagnostics
(listings such as `/list`, `/ps`, help text, anything from `lib/tf/*.tf`).

## Cases: `testdata/warnings/*.tf`

A case starts with a header line and runs until the next header:

```
;;; expr.813 # division by zero
/test 1/0
```

`;;; <catalog-key> [typed] # description`. The body is a TF script run as its own file, so line
numbers in the expected context are relative to the case. `@DIR@` is replaced by a scratch
directory. With the `typed` option no file is loaded; the lines are typed at tf's prompt (gtfrun
`-i` on the gtfparser side), which gives the no-location context. A case is *valid* when real tf
prints a line matching the catalog template; otherwise it is reported `INVALID` and must be fixed.
Keep each case to the one trigger it is named for, plus whatever setup it needs.

## Running the harness

```sh
. scripts/env.sh                       # TF_REF / TF_LIB: a built real tf (see scripts/pcre_build_tfwide.sh)
GOWORK=off go build -o /tmp/warn_gtfrun ./cmd/gtfrun
python3 scripts/warncheck.py                       # all cases, table + summary, exit 1 on new mismatches
python3 scripts/warncheck.py --ignore-context      # compare message bodies only
python3 scripts/warncheck.py expr. macro.1         # only keys with these prefixes
python3 scripts/warncheck.py --show-ok --json out.json --tsv out.tsv
python3 scripts/warncheck.py --probe scratch.tf    # run cases that are not in the catalog (authoring aid)
```

Per case it prints one of `OK`, `body`, `context`, `body+context`, `missing` (real tf warned,
gtfparser did not), `extra` (gtfparser warned, tf did not), `INVALID`, with the real and
gtfparser lines. Real tf's screen is read from a 4000-column pty, ANSI removed, wrapped lines
joined, tf's repaint-on-exit copies dropped and its startup noise (`LC_CTYPE category set`, mail
file, `Loading commands from <stdlib|case file>`, `Visual mode`, and the three-line `Internal error
at expr.c:702 ... bad op 0x0000` that real tf adds to some macro syntax errors) removed. Paths are
normalised to `<FILE>`/`<DIR>`/`<LIB>`.

Each line is split into context and body using the catalog template (the longest context whose
remainder still matches the template wins), and the two are compared separately.

### Gating: `testdata/warnings/known.tsv`

Rows `key<TAB>class<TAB>reason`; `*` matches any key or any class. The run fails only for a
mismatch that is not listed, so the suite can gate regressions while fixes land. When a fix lands,
delete its rows (or regenerate with `--update-known`, which writes every current mismatch with the
reason from `testdata/warnings/causes.tsv`). `internal/warncatalog` has a Go test (no tf needed)
that checks the catalog and the allowlist parse and are consistent with the case files.

`scripts/warn_baseline.py RESULT.json` renders `testdata/warnings/BASELINE.md`, the current gap
grouped by root cause and priority.

## Caveats

* The harness compares what tf prints on a pty at width 4000; messages that include the screen
  width (`total status width (N) is wider than screen (M)`) depend on that.
* Some real-tf behaviours are bugs, not targets: macro syntax errors are followed by an `Internal
  error ... bad op 0x0000` block, and a division by zero inside a macro crashes this build with
  signal 11 after printing its message (reported as `tf crashed`, the message is still compared).
* tf stops at the first `Invalid command. Aborting.`; such cases load a helper file so the case
  itself can still `/quit`.
* The reference tf here includes the squat bridge commands (`/squat`, `/control`); those are
  catalogued from `src/squat.c`, and most need a bridge process, so they are mostly untestable.
