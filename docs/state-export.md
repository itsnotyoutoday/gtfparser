# State export: SaveState and LoadState

A client that re-execs itself (`/restart`) keeps its sockets and hands the
scripting session to the new process with

```go
func (in *Interp) SaveState(w io.Writer) error
func (in *Interp) LoadState(r io.Reader) error
```

The new process runs `New`, installs its commands, functions, match styles and
host capabilities as always, and calls `LoadState` instead of `stdlib.tf` and
the user's `tfrc`. Nothing is re-run: the macros, variables, worlds, history
and running processes arrive as they were.

## Call order and rules

1. Old process, between commands (the command that triggered the restart may
   still be on the stack; the stack is ignored): `SaveState(w)`. Write the file
   with mode 0600 and delete it after the load: it holds world passwords in
   clear text, exactly as `/saveworld` does.
2. New process: `New(host, opts...)` with the same `LibDirs`, `Policy` and
   `Limits`, register commands, functions and match styles (a macro that uses a
   custom style cannot be restored before the style exists), then
   `SetFeature`/`EnableHookEvents` as usual, then `LoadState(r)`.
3. Tell the engine what is not carried (below): `SetCurrentWorld`,
   `SetMoreState`, telnet state, and so on.
4. Start the `Runner` (or call `RunDue` as usual). Running processes are
   re-armed by `LoadState`, relative to the Interp's own clock (`Now`): the
   remaining time of every `/repeat` and `/quote` is what it was at save time,
   so `/ps` shows the same figures, and process numbers are kept. A Runner that
   already runs may call `LoadState` through `Do`: it looks at `NextDue` after
   every call.

`LoadState` on an Interp that has been used **replaces** what the state covers
(it is not a merge); what the state does not mention (client registrations,
`Host`, `Policy`, `Limits`, `Now`, `LibDirs`, `Warn`) stays. A restriction level
is never lowered by a load.

`LoadState` first decodes and validates the whole input, building every object
(recompiling the patterns, checking the cross references), and only then
installs it. On any error the Interp is untouched:

| error | meaning |
|---|---|
| `ErrStateTooNew` | written by a newer engine: the envelope, or a section marked required, cannot be read |
| `ErrStateCorrupt` | not a state document, truncated, or inconsistent |
| `ErrStateIncompatible` | well formed, but does not fit this Interp (a match style or hook event that does not exist here) |

(Use `errors.Is`.) A read error from the reader is returned as is. A panic while
decoding is turned into `ErrStateCorrupt`.

## Format

One JSON document, canonical (fixed key order, sorted maps, no clock or
random content: the same state always produces the same bytes, and
save, load, save gives the same bytes back):

```json
{"magic":"gtfparser-state","format":1,"minReader":1,
 "sections":{"globals":{"v":1,"required":true,"data":{...}}, "...": {}}}
```

* `format` is the envelope version of the writer, `minReader` the oldest
  reader that can parse the envelope; a reader whose `StateFormat` is smaller
  than `minReader` refuses (`ErrStateTooNew`).
* Every part is a **section** with its own version `v` and a `required` flag.
  A reader accepts every older version of every section it knows (migrating
  when versions differ), **skips** an unknown or too-new section that is not
  required (that part of the Interp then keeps what it had), and refuses a
  required one it cannot read. A section that is absent leaves that part alone.
* Rule for changing the format: add a field that old readers may ignore
  without harm to the same version when the field's absence means the old
  behavior; otherwise raise that section's `v`, keep the decoder for the old
  `v`, and mark new sections `required` only when ignoring them would change
  behavior. `minReader` only moves when the envelope itself changes.
* Strings are UTF-8 JSON strings; a string that is not valid UTF-8 (server text
  in an old encoding) is `{"b64":"..."}`. Values are `{"k":"i"|"r"|"s"|"d",...}`
  (integer; float as its bit pattern in hex, so NaN and infinities survive;
  string; fixed-point decimal/time with its type) plus the enumeration index of
  an enumerated variable.
* Times are RFC 3339 with nanoseconds; process times are saved as the time that
  remains until they run.

`testdata/state/v1-busy.json` is the frozen version-1 document: every later
reader must load it (`TestStateFixtureV1`).

### Sections (all `v` 1)

| section | required | carries |
|---|---|---|
| `interp` | yes | next macro number, next process number, next stream handle; `%?` and the user result; one-time warning flags; `/restrict` level; names of library commands bound to their Go ports; `/loaded` list; exported-variable names |
| `globals` | yes | every global variable with its type (int, float, string, decimal/time, enumerated); `TFVERSION` and `version` are not restored (this build's own are kept). The special variables (`%wrap %more %visual %isize %login %hook %maildelay %matching %status_* ...`) are ordinary variables here, so all are carried. `%sigfigs` is re-applied. |
| `locals` | yes | the variables of the global frame (top-level `/let`) |
| `macros` | yes | every macro with its number, name, body, flags, priority, probability, remaining shots, `-t`/`-h`/`-T` patterns (text and style; recompiled on load), hook events, attributes (`-a -f -A -P`), `-E` condition, `-w`, key binding or key name, used counters, match style, invisible flag, whether it runs a Go port; the order of every trigger and hook list; enabled optional hook events |
| `worlds` | yes | every world with all fields (including password), the `default` world, the current world, the `(unnamedN)` counter, and each world's history |
| `history` | yes | input, local and global histories (lines, times, attributes, counters, the editor position); log file names |
| `procs` | yes | `/repeat` and `/quote`: number, delay, remaining time, count, command, world, `-P`, remaining quoted lines and their prefix/suffix |
| `streams` | no | `tfopen` handles |
| `terminal` | no | the keyboard line and cursor, the editor's input history, `lnext`, held-back key prefix, pending newline, prompts, no-echo worlds, status-line fields |
| `limit` | no | `/limit` filters per screen |
| `watch` | no | `/watchdog` and `/watchname` settings |
| `mail` | no | per-file state of `nmail()` |

### What the load does with files

* **Logs** (`/log`): reopened for appending at the saved name (created if
  missing). One that cannot be opened is reported through `Warn`
  (`state: cannot reopen log ...`) and dropped.
* **Streams**: queues (`tfopen("", "q")`) come back with their pending lines.
  Files come back reopened by name: `r` at the read position the script had
  (read-ahead excluded), `w` without truncating, at the write position, `a`
  appending. One whose file cannot be reopened is closed (its handle is then a
  "bad handle"). **Pipes** (`p`, `P`) are child processes and cannot move: they
  are closed. Handle numbers are never reused: the next `tfopen` returns the
  number it would have returned.
* Open `/quote !cmd` is not a stream: its output was read when the command
  started, so the remaining lines travel with the process.

### Input typed with echo off

If the foreground world (`LocalEcho("")`) or the current world is in no-echo
mode when `SaveState` runs, the keyboard line, and the line-being-typed slot of
the editor history, are not saved. Lines recorded in the input history with the
`g` attribute (`RecordInputAttr(line, "g")`, how a client records a password) are
saved with blank text, whatever the mode. The engine cannot tell by itself
that a line given to `SubmitInput` was typed with echo off: a client should not
submit those through the editor history.

## What is not carried, and why

* **Sockets** and everything about the connection (telnet/MCCP/SSL state,
  `SockState`, what `SocketHost` reports): the client's.
* **The macro stack**: state is saved between commands. The root frame's
  variables are carried; positional parameters, `%P` registers, `/let`
  variables of running macros, pipes' `tfin`, the statement counters and the
  running `/exit` and `/break` flags are not.
* **Pipe streams** (see above) and anything held by a child process.
* **The random generator**: `rand()` uses Go's process-wide generator, which
  the engine does not seed; a sequence cannot be continued (and a script cannot
  rely on one).
* **Output-window state** (`SetMoreState`, `MoreHost`), `LimitHost`, the
  `OnVarChange` snapshot and `WatchVar` registrations: the client re-feeds them
  from its own screen. `SetFeature`: the client declares it again.
* **Registrations**: commands, functions, `RegisterMatchStyle`, filters,
  observers, `Policy`, `Limits`, `Warn`, `Now`, `LibDirs`, `Host`.
  (`DisableCommand` marks and `AddCommand` specs belong to the client too.)
* Compiled caches (templates, expression and statement caches).
* The current world's *connection*: `CurrentWorld` is carried (and the
  `world_*` variables recomputed), but the client's idea of the foreground
  world is its own; call `SetCurrentWorld` if it differs.

## Tests

`state_export_test.go` builds a busy session (variables of every type; triggers,
hooks, hilites, gags, shots, priorities, `-w`, bindings; worlds; histories;
processes; streams; logs; limits; status fields; typed input), saves it,
loads it into a fresh Interp, and runs the same script of listing, firing and
clock-advancing commands in both, comparing the output byte for byte, with and
without the real library (`GTF_TF_LIB`) and with the library running as script.
`state_envelope_test.go` covers the envelope: the frozen fixture, version
rules, truncated, corrupt and mutated input, atomicity, no-echo input, files,
and a Runner picking up the processes.
