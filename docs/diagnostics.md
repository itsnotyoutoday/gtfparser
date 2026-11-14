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

gtfparser builds the same line in `Interp.warn` (location of the `/load`ed file and line, the
innermost command, macro or function, `Warning: ` for warnings), so `/test nofunc(1)` in a macro `f`
called from line 3 of a file prints `% file.tf, line 3: f: nofunc: no such function`, exactly as tf
does. `scripts/warncheck.py` reports "context" mismatches separately from "body" mismatches.

Rules the engine follows (each is pinned by a case in `testdata/warnings` and by `diag_test.go`):

* The loader (`LoadFile`) names no command for the lines it runs; its own diagnostics
  (`Invalid command. Aborting.`, `whitespace following final '\'`) name the command that started the
  load (`LOAD` for `/load`, nothing for the startup file). A line of a loaded file that does not
  start with `/` aborts the load.
* A macro body, typed line or `/eval` text is checked as a whole before anything in it runs, as
  tf's compiler does: an unbalanced `/if` `/while`, a stray `/endif` `/done`, a `%|` into text and
  similar produce one `macro syntax error` / `unexpected /X in ...` line and the body does not run
  (so a loop that cannot be compiled can neither spin nor repeat its message). The check is
  `compileStmts` in `compile.go`; its result is cached per text and replayed on every run.
* Compile-time diagnostics of an `/eval` (`unmatched $[`, `/@name: not a builtin command`) name
  `EVAL`; errors raised while the evaluated text runs do not.
* Messages of functions carry the function name as context (`mod: `), never repeated in the text.
* Text sent with no connection (`Host.Send` returning false) prints `Not connected.`.
* `/load` fires the `LOAD` and `LOADFAIL` hooks (`-q` silences only the "Loading commands from"
  line, also for nested loads); that is how `stdlib.tf` loads its optional `local.tf` silently.

Messages of commands and functions that a client registers through the public API keep their own
form (for example `/version`, `/help`, `/connect`, the squat bridge commands).

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

* gtfparser's `% ` lines are collected from both stdout and stderr (informational output such as
  `% Logging to file ...` is echoed; errors go to the `Warn` hook).
* `/set name` followed only by blanks is taken as "set to the empty value" (tf decides from the
  unexpanded text whether `/set x %*` has a value part; the engine sees the expanded text). A typed
  `/set x ` therefore sets x instead of listing it.
* The harness compares what tf prints on a pty at width 4000; messages that include the screen
  width (`total status width (N) is wider than screen (M)`) depend on that.
* Some real-tf behaviours are bugs, not targets: macro syntax errors are followed by an `Internal
  error ... bad op 0x0000` block, and a division by zero inside a macro crashes this build with
  signal 11 after printing its message (reported as `tf crashed`, the message is still compared).
* tf stops at the first `Invalid command. Aborting.`; such cases load a helper file so the case
  itself can still `/quit`.
* The reference tf here includes the squat bridge commands (`/squat`, `/control`); those are
  catalogued from `src/squat.c`, and most need a bridge process, so they are mostly untestable.
