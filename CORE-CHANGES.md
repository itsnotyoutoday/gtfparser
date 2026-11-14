# Core changes

Every change to the engine that a client could notice is logged here, newest
first, in the same commit that makes it. Clients pin a version of this module
and update on purpose (`go get github.com/itsnotyoutoday/gtfparser@vX.Y.Z`),
reading this file for what changed. A change is never required to be taken
mid-task.

Format: `## vX.Y.Z (date) — summary`, then `Added` / `Changed` / `Fixed` /
`Breaking` (and what a client must do).

## v0.0.11 (2026-11-14) — restriction by origin, world database API with revisions and change events

### Part 1: restriction by origin

### core: code from different parties can run under different restriction levels
For hosts that run a remote peer's commands and macros next to the owner's own (gtf with an attached
GUI): `/restrict` is one global level, so locking out the peer locked out the owner. An *origin* is an
opaque string the host chooses; each gets its own level, code run under it is restricted, and what it
defines (macros, triggers, hooks, key bindings, `/repeat` and `/quote` processes, streams) keeps the
tag and runs restricted later, whatever starts it. Strictest of the global level, the running
macro's origin and the callers' origins wins; nothing can loosen it. Full semantics, the audit of
every guarded operation and the decisions left to the host: `docs/engine/origins.md`.
tf has no such thing, so a script that uses no origin behaves exactly as before (same output, same
saved state, same speed within noise).

Added
- `SetOriginRestriction(origin, level) bool`, `OriginRestriction`, `OriginRestrictions`,
  `ExecAs`, `RunAs`, `EvalAs`, `DoAs`, `EffectiveRestriction`, `CurrentOrigin`, `(*Macro).Origin`.
- Script functions `macro_origin(name)` and `current_origin()` (extensions; a script's own macro of the
  same name wins). `/list` and every other output are unchanged.
- State section `origins` (version 1, required, written only when origins are in use): levels and the
  tags of macros, processes and streams survive `SaveState`/`LoadState`. A load never lowers a level.

Changed (only observable when an origin is configured)
- Every guard (`/sh`, `/quote`, `/load`, `/log`, `/save`, `/saveworld`, `/lcd`, `tfopen`, `fwrite`,
  `addworld`, `/connect`, host commands declaring `Needs`) now reads the effective level.
- `/restrict` run by origin code raises that origin's level, never the global one; with no argument it
  reports the effective level.
- A stream handle opened by the host or another origin cannot be used by code whose origin is
  restricted at the level that would have refused opening it (`restricted`).
- Stored status-field expressions are evaluated as if at `RestrictWorld` once any origin has a level.
- A trigger's `-E` condition runs under the macro's origin.
- Redefining a macro from another origin is never a "no change"; `/edit` retags as described in the doc.

Client: wrap peer input in `ExecAs("peer:<id>", ...)` and set `SetOriginRestriction("peer:<id>", RestrictFile)`
once; nothing else is needed for triggers, hooks and timers the peer defines.

### Part 2: world database API with revisions and change events

### core: mutate the world database from Go, and hear about every change
For clients that let something other than a script edit the worlds (a remote peer, a settings
screen) and must tell everyone who looks at the database. No script-visible behaviour changes.

Added
- `World.Rev` (so `WorldInfo.Rev`): the record's revision. It changes on every definition,
  redefinition and rename, is never reused (not even by a later world of the same name) and is
  larger after a restart than before it (the counter starts at the wall clock in ms). Assigned
  lazily for worlds restored by `LoadState`; not part of the saved state. Use it for optimistic
  concurrency: an update based on an older Rev is stale.
- `(*Interp).PutWorld(WorldInfo) (WorldInfo, error)`: create or replace every field (an empty field
  clears the old value, unlike `DefineWorld`); history, macros and connection are kept.
- `(*Interp).DeleteWorld(name) error`, `(*Interp).RenameWorld(old, new) (WorldInfo, error)`.
- Errors (use `errors.Is`): `ErrWorldNotFound`, `ErrWorldExists`, `ErrWorldInUse`, `ErrWorldInvalid`.
  These calls print nothing; validation failures are returned.
- `(*Interp).WorldDefinitions() []string` (the lines `/saveworld` writes) and
  `(*Interp).WorldFile() (string, bool)` (what `/saveworld` writes to by default, the `WORLDFILE`
  macro), so a client can persist the database itself (atomically, with a backup).
- Notifier event `"world"` with args `op, name` where op is `add`, `update` or `remove`, fired for
  every change by any route: `/addworld`-style `addworld()`, `/unworld`, `PutWorld`, `DeleteWorld`,
  `DefineWorld`, `RemoveWorld`. Temporary (`/connect host port`) worlds do not notify. Renames keep
  the existing `"renameworld"` event (`old, new`), now also fired by `RenameWorld`.
  (There is no `/purgeworld` in tf; the commands are `/addworld`, `/unworld`, `/renameworld`.)

Client: implement `Notifier` (gtf already does) and handle `"world"`; to adopt, bump the pinned
version after this is released.

## v0.0.10 (2026-11-14) — state export for restarts, squatter hook events

### core: a session can be carried to a fresh Interp
For clients that re-exec themselves (`/restart`) and keep their sockets open: the new process
builds a fresh `Interp` (same `Host`, commands, functions, match styles, `LibDirs`), calls
`LoadState`, and has everything a running session holds, without running `stdlib.tf` or the user's
`tfrc` again. Full description, section list and the list of what is not carried:
`docs/state-export.md`.

Added
- `(*Interp).SaveState(io.Writer) error` and `(*Interp).LoadState(io.Reader) error`; `StateFormat`;
  the errors `ErrStateTooNew`, `ErrStateCorrupt`, `ErrStateIncompatible` (use `errors.Is`).
- Encoding: one canonical JSON document, `{"magic","format","minReader","sections":{name:{"v","required","data"}}}`.
  A reader loads every older version of every section it knows, skips unknown or too-new optional
  sections, and refuses (`ErrStateTooNew`) a required section or an envelope it cannot read.
  Deterministic: save, load, save returns the same bytes. Sections: interp, globals, locals,
  macros, worlds, history, procs, streams, terminal, limit, watch, mail (all version 1).
- `LoadState` decodes and validates everything before it changes anything: on any error the
  Interp is untouched. On a used Interp it replaces what the state covers; it never lowers
  `/restrict`. Processes (`/repeat`, `/quote`) are re-armed relative to the Interp's clock with
  their remaining times and numbers, so a Runner (which looks at `NextDue` after every call) or a
  client driving `RunDue` just continues.
- Carried: variables with their types and export flags (including every special variable),
  top-level `/let` variables, macros with exact numbers, flags, shots, used counters, patterns
  (recompiled), hooks, key bindings, the order of trigger and hook lists, the next macro number and
  which library macros run their Go ports, the world database with the current world and
  per-world histories, the input/local/global histories and the editor's input history, processes,
  queue and file streams (reopened), logs (reopened for append), `/limit`, status fields,
  prompts, the keyboard line and cursor, `/watchdog` settings, `nmail()` state, `%?`.
- Not carried (the client's or impossible; see the doc): sockets, the macro stack, pipe streams,
  the random generator, output-window (`SetMoreState`) state, client registrations.
- Anything typed with echo off is never saved: the keyboard line is dropped when the foreground
  world is in no-echo mode, and input-history lines recorded with the `g` attribute are saved blank.
  World passwords are saved in clear text, as `/saveworld` writes them: protect the file (0600)
  and delete it.

Changed
- `/limit` remembers the match style of its pattern (internal; the output is unchanged).

Client: call `LoadState` after `New`, after registering commands, functions and match styles, and
before the first command; then `SetCurrentWorld` and the other things the client owns, and start
the Runner. Do not call `SaveState` while a command other than the one asking for the restart runs.

## v0.0.9 (2026-11-14) — builds on 32-bit platforms

Fixed
- The module did not compile where `int` is 32 bits (linux/386, linux/arm): `cAtoi` saturated at `1<<31` in an `int`.
  It now accumulates in 64 bits and clamps. Behaviour on 64-bit platforms is unchanged.

### Also in v0.0.10

### core: optional hook events SQUAT_CONNECT, SQUAT_DISCONNECT, SQUAT_PAIR
Three optional hook events (like GMCP and MSDP: `/def -h<event>` is refused until the client calls
`EnableHookEvents`) for a client that serves remote peers. Args: `device role`. Added
- `hookTable` entries (alphabetical, after SIGUSR2), documented in `docs/engine/hooks.md`,
- `TestSquatHooksOptional` (refused until enabled, then fire and run the macro).
No behaviour changes for clients that do not enable them; `/list -h` and `/hook` listings do not show optional events.

## v0.0.8 (2026-11-14) — stdlib.tf's library commands move into the engine

### core: 73 library commands are native
tf defines these as invisible macros in `stdlib.tf`; the engine now implements them in Go, so a
client has them with or without `stdlib.tf`. The full lists (native / client-owned / kept in
stdlib.tf) are in `docs/stdlib-split.md`.

Native: `/echo /_echo /true /false /: /prompt /substitute /require /loaded /not /expr /for /toggle
/first /rest /last /nth /escape /replace /split /cat /paste /_paste /time /rand /ver /vercmp /runtime
/ismacro /isvar /redef /more /wrap /isize /ptime /wrapspace /act /reply /background /bamf /borg
/clearfull /cleardone /insert /login /lp /quiet /quitdone /sockmload /sub /nolog /nowrap /nologin
/noquiet /listdef /listhook /listtrig /listgag /listhilite /listfullhilite /listpartial /listbind
/purgedef /purgehilite /purgegag /purgetrig /purgedeft /purgebind /purgehook /untrig /unhook /undeft
/savehilite`, and the variables `pi` and `e`.

- Each is an ordinary command: a user's `/def` of the same name shadows it, as before for `/echo`.
- When `stdlib.tf` defines a macro with exactly the body the engine ports (compared by fingerprint),
  that macro keeps its number, flags and printed body (`/list -i`, `/ismacro`, `/edit`, `/undef` see
  what real tf shows) but runs the Go code. `/undef lp` then really removes `/lp`, and `/@lp` is
  "not a builtin command", as in tf. A different body with the same name stays script.
- Without `stdlib.tf` the commands exist as builtins, and as functions (`first("a", "b")`).
- `NativeLibraryCommands()`, `WithoutNativeLibrary()` (`gtfrun -script`) and `(*Macro).IsNative()` are new.
- Exported helpers for the client-owned library commands: `SetCommand(name, variable)` (`/visual`,
  `/gpri`, `/hpri`, `/lpquote`, `/shpause`), `EchoPrefixCommand(prefix)` (`/kecho`, `/mecho`,
  `/qecho`), `AddWorldCommand(name, type)` (`/addworld`, `/addtiny` ...).
- Differential cases `testdata/cases/native_lib_*.tf`; `TestLibraryMacroTable` accounts for native
  macros by an explicit allowlist; `TestNativeMatchesScript` runs every native against its script body.

Changed
- `/echo` takes tf's options exactly (`-a<attr> -p -o -e -A -r -w<world>`; anything else is
  `echo -x: invalid option`) and, as a macro in tf, never sees trailing blanks in its argument: a hook
  that runs `/echo %*` with a prompt ending in a space prints it without the space. (Before, this
  differed from tf only when `stdlib.tf` was not loaded.) A client implementing `EchoHost` still
  receives `-a`, `-p` and `-w` as before.
- `/for`, `/require` and `/loaded` are exact ports of the library macros (`/loaded` uses
  `%_loaded_libs` as tf does; `/for` builds and evaluates tf's `/eval` text) instead of
  approximations.
- `echo()`-visible effects of the macros are unchanged when the library is loaded; the A/B test
  (native vs script) pins that.

Fixed (engine behaviour the natives rely on; see also the `/eval` entry below)
- `/return` and `/break` inside `/eval` stay in the eval (`/def m = /eval /return 7%; /echo after`
  prints `after`), as in tf. A pipe's input is dropped when its right side finishes.
- `getopts()` failures read `echo -x: invalid option` and `echo: options: -a<string> -poerA
  -w<string>` (the specifier's order, consecutive flags together) instead of `getopts: echo: invalid
  option 'x'`. A client's `AddCommand` usage line follows the same grouping.

## v0.0.7 (2026-11-14) — diagnostics match tf

### core: diagnostics identical to real tf's
Warning and error text, and the `<file>, line N: <command or macro>: ` prefix, now match real tf
for 235 of the 257 script-triggerable diagnostics in `testdata/warnings` (it was 3 before the v0.0.6
context work and 104 after it; the rest are client-owned features, see
`testdata/warnings/known.tsv`). A client that parsed warning text must read this list.

Changed (message text)
- `LOAD: <path>: No such file or directory` (OS error text, upper-case first letter) for `/load`,
  `/save`, `/saveworld`, `/log`, `/quote`, `fwrite()` and `tfopen()`; `/load -q` still reports a
  missing file. `/load` of a directory: `Is a directory`. `/load` with no file: `missing filename`.
- Function arity (`strlen: found 0 arguments, expected 1`), `<name>: no such function`,
  `function name must be an identifier.`, `EXIT: not a function`, `VERSION: command called as
  function must have 0 or 1 argument`, `condition syntax error: expected operator or ')', found
  '2'.`, `expression syntax error: expected operator or ']', ...` inside `$[...]`, `* operator:
  arithmetic overflow` / `MULA operator: ...`, `illegal object of assignment`, `numeric value too
  large`, `expression stack overflow`, regexp errors as `regexp error: character N: message`.
- Macro structure: `unexpected /DONE in IF body`, `unexpected /ENDIF in WHILE body`, `... in outer
  block`, `macro syntax error: expected /endif|/done|/then|/do, found end of body.`, `/BREAK syntax
  error ...`, `Piping input|output ... is not allowed.`, `'%|' must be followed by another command.`
  and tf's two blank-line warnings. `recursion count exceeded %max_recur (N)` replaces `too many
  nested calls`.
- Substitution errors are named `EVAL: ` while `/eval` compiles its text (`EVAL: unmatched $[`).
- Variables: `illegal variable name: a-b`, `'=' following space is part of value.`, `<name> not
  set globally`, `<name> not defined.` (`/export`), `<name> must be a positive integer, so can not
  be unset.`, the local-overshadows-global warning (through the `SHADOW` hook), warnings for names
  like `L` or `PR` and for obsolete variables (`prompt_sec`).
- Glob errors (`glob error: unmatched '['` ...) in `=/` and `!/`; the `{` warning in `regmatch()`.
- `/repeat`, `/kill`, `/quote`, `/recall` argument errors with tf's wording (`REPEAT: invalid
  repeat count (0)`, `KILL: no process 999999`, `integer value too large`); `/def -B` key checks,
  `-P-1`, `-?` (only the option summary), `/undef #N`, `/unbind` of an unbound key, a key binding
  that extends or is a prefix of another; `/listsockets -S` style option letters in the context.
- File loading: tf's loader messages (`Invalid command. Aborting.`, `possibly missing trailing \`,
  `whitespace following final '\'`, `last command is incomplete because of trailing \`).

Changed (behavior)
- A macro body, typed line or `/eval` whose `/if` `/while` structure is not well formed is not run
  at all (tf compiles before running). `/while (1) /endif%; /done` no longer loops and repeats its
  message a million times; an unterminated `/while` no longer runs until `%max_instr`.
- A line of a loaded file that does not start with `/` (after the leading blanks) aborts the load
  with `Invalid command. Aborting.`; before, it was sent as text. A file ending in a trailing
  backslash no longer runs the incomplete command.
- `/load` fires the `LOAD` and `LOADFAIL` hooks. The stdlib's `/def -hloadfail -ag` around the
  optional `local.tf` now silences its absence, so a client no longer needs to create an empty
  `local.tf`. `/load -q` (and `/require`) are quiet for nested loads too.
- Text sent while `Host.Send` returns false prints `Not connected.` (context: the running macro);
  `gtfrun`'s host now never connects. `send()` rejects unknown flags (`invalid flag Z`) and honours
  `h`. `echo(..., "wNAME")` of an unknown world: `No world NAME`.
- Processes (`/repeat`, `/quote`) run under no command name, like tf's main loop.
- `read()` and `tfread("i", v)` report that the keyboard can only be read from a command line
  command (the engine has no keyboard).
- `/set x %*` with empty arguments sets x to the empty string while a typed `/set x ` lists it,
  as in tf: the engine decides from the unexpanded line (`Interp.rawSetValue`).

Fixed
- `${world_*}` expands to nothing (no "macro not defined") when there is no current world; `/limit -m` error has tf's two spaces.

Added
- Native `/_echo`, `/true`, `/false` and `/:` (tf defines them as stdlib macros); `/echo` was already native.
- `diag_test.go`: pins the most common messages without needing real tf.
- `scripts/warncheck.py` reads gtfparser's stdout `% ` lines too; `warn_baseline.py` and
  `BASELINE.md` regenerated; `known.tsv` lists only genuine remaining differences.

## v0.0.6 (2026-11-14) — tf compatibility pass, license, diagnostics

### core: tf compatibility pass (differential fuzzing findings)
The engine now matches real tf where it previously did not. Reference configuration: tf 5.0 beta 8,
UTF-8 build, 64-bit macOS, `en_US.UTF-8`. Corpus: `testdata/cases/compat_*.tf`; the few intentional
differences are listed in `docs/deliberate-differences.md` and pinned by `testdata/deviations` and
`compat_test.go`. A client that parsed warning text or relied on the old behavior should read this list.

Fixed (crashes)
- `/shift -1` and `strncmp()` with a negative count no longer panic (`/shift -1` returns 0).

Fixed (macro language)
- A nested `/if` that starts on the same line as the outer condition no longer closes the outer
  `/if` (`/if (a) /if (b) ...%; /endif%; /else ...%; /endif`); same for `/while`.
- Builtin commands can be called as functions with 0 or 1 argument (`shift()`, `def()`, `load()`,
  `set()`, `limit()`, `features()`, `version()` ...); `shift` returns 0 when it cannot shift that many.
  This fixes lisp.tf (`/reverse /mapcar /remove /unique`), stdlib `/nth` and the startup warnings.
- `regmatch()` captures (`%P1`...) set in a called macro are visible to its caller (`/split`).
- Substitution grammar follows tf: `%1-queue` / `%name-default` (default to the next blank, taken
  only when the value is empty), `%{...}` malformed forms (`%{-0}`, `%{#-x}`, `%-`, `%{1:}`, `%{ 1}`)
  are errors that abort the macro, `%.` and similar drop the `%`, `${%{x}}` expands the name,
  `$(...)` may contain `%;`, unmatched `$[` `$(` `${` are errors. A failed `$[...]` aborts the
  statement and the macro (it used to continue with an empty value).
- A macro's arguments no longer end in blanks; a macro whose last command is `/set` or `/let`
  returns "" when called as a function; bare `/return` returns 0; `/quote` source lines run
  unsubstituted (`/qdef` keeps `%1`); a synchronous `/quote !cmd` (and so `/sys`) returns the exit
  status; `/exit` in a loaded file ends that `/load`; `/load` treats `;` and `#` in the first column
  as comments and a trailing backslash as a continuation unless a `%` precedes it; `%max_iter` is
  ignored (as in tf: runaway loops end at `max_instr`).
- `/load`, `/lcd`, `/version` (now also prints the Copyright line so `/ver` works) texts and option
  usage lines (`-lgi -w<string>`) match tf; `%isize`/`%tabsize` ... reject values below 1 (`pos`) and
  integer variables values below 0 with tf's message; time variables print `1.0`; visual mode is
  refused silently when the host says it cannot (new optional `VisualHost`).

Fixed (expressions)
- `=~ !~ =/ !/` and `< <= > >=` are one left-associative level; `1 ? 2:3` is a syntax error (`2:3`
  is a time); arguments of builtin functions are counted when the expression is compiled
  (`strlen("a","b")`, `pow(1)`), blanks before `(` are allowed (`pow (2,3)`).
- Identifiers are read when their operator runs (`y - --y` is 0, `y:=5, y + ++y` is 12), function
  arguments that are bare variables likewise; `++f` on a float truncates to an integer.
- Float printing follows tf: `%.15g`, a `.` only when the text is all digits, so `-2.0*1` prints
  `-2`, `-0`, `nan`, `inf`, `-inf`; `0/0.0` prints `nan` while an infinite result of `+ - * /` is
  `/ operator: arithmetic overflow`; math functions do not check their results.
- `time()`, `mktime()`, `idle()` have time types (`whatis` says `atime`/`dtime`; durations of a
  minute or more print as `h:mm:ss`); `ftime()` is a full strftime (`%s %e %j %k %l %U %G` ...,
  `%.`/`%@`) in the `TZ` the script sees; `filename("~")` expands; `test()`, `fwrite()`, `mktime()`,
  `read()` exist; `isatty()` (new optional `TTYHost`), `nread()` (`SetReadDepth`) and `nmail()`
  (the MAIL/TFMAILPATH rules of util.c) are real.
- String functions work in characters (`strlen`, `substr`, `strstr`, `strchr`, `strrchr`, `pad`);
  negative `substr()` start/length, the start argument of `strstr`/`strchr`/`strrchr`, the count
  argument of `tolower`/`toupper`, `strcmp`/`strncmp` byte differences, `replace()` with an empty
  pattern, `mod(x,0)` error text, `regmatch()` counting only groups up to the last that took part
  and 0 for an invalid regexp.
- tfopen() modes `q` (queue) and `p`/`P` (pipe), a single argument is the mode, bad handles
  and unusable streams give tf's messages and return values for tfclose/tfflush/tfreadable/tfwrite.

Changed (diagnostics)
- Warnings now carry tf's context: `file, line N: ` while loading (`lines S-E:` for a continued
  line), then the running macro, builtin command or function (`f5: nosuchcmd2: no such command or
  macro`, `mod: division by zero`); builtin commands appear in upper case (`LOAD:`, `SH:`) as tf
  prints them. Expression syntax errors use tf's wording (`expression syntax error: expected
  operand, found end of body.`); `max_instr` says `instruction count exceeded %max_instr (N).`
  Messages of client-registered commands and functions keep their old form.

Deliberate differences (see `docs/deliberate-differences.md`)
- Integers are 64-bit: `1+4294967296` is `4294967297` (tf narrows to 32 bits and prints `1`);
  results become reals only on int64 overflow; `1/4294967296` is `0`; `++`/`--` at the int64 limit
  raise `integer overflow in ++x`.
- `tolower`/`toupper` map Unicode characters one to one (tf maps bytes and can emit invalid UTF-8);
  `char(n)` for n >= 256 is the Unicode character; `trunc()` of an out-of-range real clamps.
- `strrep`, `pad` and `replace` refuse results above the new `Limits.MaxString` (default 16 MiB)
  with an error instead of hanging or exhausting memory.

Added
- Optional host interfaces `TTYHost` (`IsTTY() bool`), `VisualHost` (`CanVisual() bool`),
  `Interp.SetReadDepth`, `Limits.MaxString`, `DefaultMaxString`.
- `scripts/difftest.py` understands `testdata/deviations/*.tf` + `.out` (registered deliberate
  differences: gtfparser must print the documented output and real tf must still differ).

### core: tflib
Added package `tflib`: the real tf library (unmodified `lib/tf`, GPL, see `tflib/NOTICE.md`)
embedded with go:embed so every client ships the same one. `tflib.FS`, `Extract(dir)`,
`CacheDir()`, `Setup(in, dir)` (extracts and prepends to `LibDirs`), `Version()`. The engine
loads library files from disk, so clients extract to a cache directory. No change to existing packages.

### core: LimitFilter
Added `Interp.LimitFilter(world) func(line, attrs string) bool`: a goroutine-safe
snapshot of a world's /limit filter (nil when none), so a client can filter its
scrollback without calling into the interpreter from another goroutine.

### core: Placeholders
Added
- `Interp.Placeholders()` lists the builtins real tf has that the engine only stubs
  ("/name" / "name()"), so documentation and status tooling can be generated from the code;
  `TFCommands()` / `TFFunctions()` list every builtin of real tf.

## v0.0.5 (2026-11-14) — faithful triggers, hooks, macros and attributes

### core: trigger, hook, macro and attribute machinery rewritten after tf's macro.c
The macro database, trigger matcher, hook table and display attributes were
rewritten to follow tf 5 exactly (priority/fall-through selection, `-c`
probability, `%max_trig`/`%max_hook`, `-P` partial hilites, `/substitute`,
`-n` shots, `/list -s -u -S`, `/save`, `/def` option parsing). Docs:
`docs/engine/triggers-hooks.md`, `docs/engine/hooks.md`. Corpora: `attrs
defopts hooks hooks2 save shorthand triggers2 triggers3 used`.

Added
- `LineResult` (what `Process` returns) now also has `Line AttrString` (the
  line to display: text after `/substitute`, line and per-character attributes),
  `Original`, `NoLog`, `NoHistory`, `NoActivity`, `Bell`, `Substituted`,
  `Matched []*Macro`, `Ran []MacroRun`, `Count`, `ShotsUsed`, `Suppressed`;
  `Matches []Match` and `Gag` keep their meaning. `Match{Macro, Span, Groups}`
  is unchanged.
- `Interp.ProcessText(LineInput) *LineResult`: the full interface (incoming
  attributes, per-line world, `WorldOnly`/`GlobalOnly`, `Dry`). `Process(line,
  live)` is `ProcessText` for one plain line; `live` false is `Dry`: nothing
  runs and no shot is consumed, but the selected macros' attributes are applied.
- `Attr`/`AttrCtx`/`AttrString` (attrs.go): one attribute implementation.
  `Interp.ParseAttrs`, `ColorName`, `ColorNumber`, `Attr.String()`. Status
  lines, history/`/recall`/`/recordline`, `/log` and `/def -a` all use it.
  `Macro.Attr` is the parsed `-a` set; `Macro.Attrs` (text) and `Macro.Gag`
  remain as read-only mirrors.
- Match styles are a registry: `RegisterMatchStyle(name, MatchStyle)`, `CompilePattern`;
  `Macro.Trig`, `HArgs`, `WType` are `*Pattern`.
- Hook table: `HookEvents()`, `EnableHookEvents`, `FireHookArgs(event, args...)
  *HookResult`, `FireHookEvent(HookEvent)`, `AddHookFilter`, `HookDisplayHost`
  (clients place hook messages), default messages for each event.
  `HookResult{Count, Ran []*Macro, Message, Gag, Class, ...}`. `FireHook(event,
  args string) []*Macro` is unchanged for callers: it returns the hook macros
  selected (the ones that ran).
- `TriggerObserver` (`AddObserver`), `AddPreLineFilter`, `AddPostLineFilter`
  (veto or adjust a line), `SetRand`, `WorldInfoHost` (world type / existence
  for `-T` and `-w`), `AttrEchoHost` (attributed `/echo`, `echo()`, hook
  messages), `Interp.Macros()`, `FindMacro`, `FindBinding`, `ParseMacroSpec`,
  `RemoveWorldMacros`, `Macro.Used` (`/list -u`).
- `Define(*Macro)` returns the macro number (0 on failure) and accepts macros
  described by the text fields (`Trigger`, `Hook`, `Attrs`, `Gag`,
  `WorldType`) or by the parsed ones.
- `/trigger`, `/hilite`, `/gag`, `/trigpc`, `/hook`, `/purge`, `/list`,
  `/undefn` are real implementations; `/def -m` accepts every registered style.
  Unknown commands and plain-text sends fire the `NOMACRO` and `SEND` hooks.

Changed (now as in tf)
- Equal-priority non-fall-through macros that match a line: exactly one is
  chosen at random (`SetRand` makes it deterministic); fall-throughs all run.
- A negative `-p`, `-c` or `-n` means "not given" (the default applies).
- Macro names are case-insensitive; `#<number>` names a macro.
- Redefining a macro identically is a no-op returning the old number; `%redef`
  off refuses; the REDEF hook fires; a name that is a tf builtin fires CONFLICT.
- `/edit` reinstalls an edited copy of the macro under the same number and
  position (a `*Macro` held across an `/edit` is stale).
- `/echo -a/-p` without an `EchoHost` now renders through `AttrEchoHost` (or
  prints the text with `@{...}` decoded); an `EchoHost` still gets the raw
  options. `echo(text, attrs, inline, dest)` handles attributes.
- Display attributes: `f` and `d` (flash, dim) are accepted but not kept;
  `h` expands to `%hiliteattr`; `E`/`W`/`I` to `%error_attr`/`%warning_attr`/
  `%info_attr`; `/recall -ag` and history gag tests no longer misread the
  letters inside a colour name; the "invalid colour" error lists tf's valid
  values.
- `getopts()` treats a lone `-` as the end of the options; glob literals fold
  case for ASCII letters only (as tf's tolower).
- Hook events are validated: `FireHook` with a name that is not in `HookEvents()`
  warns. `LOG` and `REDEF world` messages come from the hook table.

Breaking (adapting a client)
- `Macro.Gag`, `Macro.Attrs`, `Macro.Trigger`, `Macro.Hook`, `Macro.WorldType`
  are mirrors filled in when the macro is installed. Set the real fields
  (`Attr` with `AttrGag`, `Trig`, `Hooks`/`HArgs`, `WType`) or define the macro
  with `Define` before reading; changing a mirror after `Define` has no effect.
  `Macro.OnlyInvis` is gone (it was an internal `/list` filter).
- `HookResult` (new, from `FireHookArgs`) has `Count` where the standalone
  hook engine had `Ran int`; `Ran` is the list of macros.

## v0.0.4 (2026-11-14) — what SquatClient needed

### core: line results and world context
Added `Interp.Process(line, live) LineResult` (matching macros with the byte
range of each match and of its regexp groups; `live=false` runs no body and
consumes no shot, for recomputing display effects of replayed history),
`Interp.SetWorld` (the world the interpreter is acting for, read back with `CurrentWorld()`; `SetWorld` is `SetCurrentWorld`; macros and
hooks defined with `-w` fire only for it; "" = any). `ProcessLine` is now
`Process(line, true).Gag`. The engine still does not interpret `-a`
attributes: `Match.Macro.Attrs` is handed to the client.

### core: /echo options reach the host
Added optional `EchoHost` (`EchoTo(EchoOpts{World, Attrs, Markup}, text)`).
`/echo -w<world> -a<attrs> -p` call it when the host implements it; every
other host (and captured/piped output) just gets `Echo(text)` as before.

### core: UndefID
Added `Interp.UndefID(id)`: remove a macro by its /list number (reaches
unnamed macros, which `Undef(name)` cannot). Lets a client implement `/undefn`.

### core: hook bodies expand when the hook fires inside a typed line
Fixed: `Hook` ran bodies with substitution off when called while a typed line
(which `Exec` leaves unexpanded) was executing, so a SEND hook saw a literal
`%*`. Hooks now always expand their body, like triggers.

### core: FireHook returns the macros that ran
Added `Interp.FireHook(event, args) []*Macro` (`Hook` is now a wrapper). A
client uses it to honour a gag attribute on a SEND hook (real tf then does not
send the text; lib/tf/alias.tf relies on it).

## v0.0.3 (2026-11-14) — history, worlds, logging and other state builtins

State that needs no terminal now lives in core (docs in `docs/engine/state-builtins.md`)
- History: `/recordline`, `/recall` (all range forms and options), `/histsize`; input,
  local, global and per-world histories fed by `RecordInput`, `RecordLocalLine`,
  `RecordWorldLine`; read back with `History`, `HistoryLines`, `RecallInput`.
- `/limit`, `/relimit`, `/unlimit` (`Limited`, `LimitActive`, a `limit` event).
- `/log` with `log_prefix`, `wraplog`, `ansi_log`; `LogLine`, `nlog()`; policy-checked.
- World database: `addworld()`, `/unworld`, `/renameworld`, `/listworlds`,
  `/saveworld`, `world_info()`, temporary worlds, `world_*` variables and
  `${world_name}`; `SetCurrentWorld`, `Worlds`, `DefineWorld`.
- Connections: `/connect`, `/dc`, `/fg`, `/listsockets`, `is_connected()`,
  `is_open()`, `fg_world()`, `idle()`, `sidle()`, `nactive()` over optional host
  interfaces (`SocketHost`, `ConnectHost`, `FgHost`, `IdleHost`, `RebootHost`,
  `LimitHost`); a host without `WorldHost` behaves as having no sockets.
- `/save`, `/edit`, `/undefn`, `/liststreams`, `/watchdog`, `/watchname`,
  `/features`, `/help`, `/core`, `/reboot`.
- `nactive()` uses sockets when the host has a `SocketHost`, else the paging state.

Known gaps: `/edit $text`; `%wraplog` breaks at blanks (tf uses ICU); `/features`
reports optional features off unless the client calls `SetFeature`.

## v0.0.2 (2026-11-14) — extension tables, parsed handlers, terminal builtins

Added
- `Extension`, `CommandSpec`, `FuncSpec` and `Interp.Load`: declare commands,
  functions, variables and disabled builtins as tables and load them in one
  validated call (argument-count checks, privilege checks via `Needs`, usage/help
  kept for generated help). `Interp.CommandSpecs` / `FuncSpecs` list them.
- `Interp.Run` and `Interp.Eval` (clear names for running a script / evaluating an
  expression), `HostFuncs` and `Console` hosts for small programs and tests.
- Command handlers as `Do func(*Call)`: options declared with `Options` (tf getopts
  syntax) are parsed before the handler runs; `Call` exposes `Has/Opt/Int/Arg/Rest/Args/Fail`
  and embeds the interpreter. `CommandSpec` also gains `Aliases` and `Category`.
  `gtfparser.Func(name, goFunc)` exposes any Go function as a script function with
  automatic argument conversion. `Interp.AddCommand` / `AddFunction` load one entry.
- Exported `Interp.Echo`, `Warnf`, `Send`, `Allow`, so command and function
  implementations outside the package can print, warn, send and check
  privileges.

Terminal, input and status (from the terminal builtins work; docs in
`docs/engine/terminal-builtins.md` and `terminal-hooks.md`)
- Input buffer (`in.Input()`, `TypeText`, `SubmitInput`, input history with
  `^old^new` substitution) and the whole `kb*()` function family and `/input`.
- `/dokey` with every documented name; editing operations run in core, display
  operations raise a `dokey` Notifier event. A `/dokey_<name>` macro overrides.
- Key dispatch for clients: `KeyPressed(bytes)`, `FlushKeys`, `PendingKeys`,
  `Bindings`, `FindKey`, `PrintToAscii`/`AsciiToPrint`; `keycode()` via `KeyHost`.
- Status line: `/status_add|edit|rm|clear`, `status_fields()`, `status_width()`,
  `in.StatusRows(width)` rendering rows with attribute spans; internal field
  values supplied by a `StatusHost` or the `%status_int_*` variables.
- `/beep`, `/localecho`, `/suspend`, `prompt()`, `fake_recv()`, paging state and
  functions (`morepaused`, `moresize`, `morescroll`, `winlines`, `nactive`).
- New optional host interfaces (in `terminal.go`): `MoreHost`, `WindowHost`,
  `LineHost`, `PromptHost`, `StatusHost`. New Notifier event kinds: `beep`,
  `localecho`, `suspend`, `dokey`, `status`, `more`, `prompt`, `var`.
- Variable-change hooks: `OnVarChange`, `WatchVar`, `PollVars`.

Known gaps carried with this change: `/bind` does not reject a binding that is
a prefix of another; `idle()`, `sidle()`, `nread()`, `nlog()`, `nmail()` await
sockets/log/mail owners.

## v0.0.1 (2026-11-14) — first extraction

The engine split out of the gtf client repo: interpreter, PCRE2-compatible
regexp engine, per-interpreter command/function registries, host capability
interfaces (WorldHost, Notifier, KeyHost, TermHost, HelpHost), Policy and
/restrict, Limits and Interrupt, the Runner actor.
