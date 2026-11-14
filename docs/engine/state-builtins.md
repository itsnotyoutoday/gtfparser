# Data and state builtins: what a client has to know

These builtins keep their state in the core, per `Interp` (nothing is
package-level; two Interps in one process never share anything). A client
feeds data in through exported methods, reads state out through exported
methods, and reacts to `Event`s; where the builtin needs something only a
client has (a socket list, a screen), it asks through an optional interface
declared in `host.go`. A host that lacks the interface gets
`% /name: not supported by this client`, except where the truthful answer is
"there is nothing" (see "No WorldHost" below).

Files: `history.go` `logging.go` `limit.go` `worlds.go` `saveload.go`
`misc_state.go` (state plumbing, options, features, watchdog, /help /core
/reboot). Tests: `state_test.go` and the differential corpora
`testdata/cases/{history,history2,log,worlds,addworld_cmd,saveworld,edit,state}.tf (state.tf covers /limit, /features, /watch*, /liststreams, /save)`.

## Feeding and reading

| Method | Meaning |
|---|---|
| `in.RecordInput(line)` / `RecordInputAttr(line, attrs)` | a line the user typed (input history + `/log -i`); duplicates of the previous line are dropped; `attrs "g"` for a password |
| `in.RecordLocalLine(line)` / `RecordLocalLineAttr` | TF's own output: local + global history and their logs |
| `in.RecordWorldLine(world, line)` / `RecordWorldLineAttr` | text from a world: that world's history and the global one, and the logs attached to them |
| `in.LogLine(world, line)` | append to a log only (world "" = global log); false if none open |
| `in.CheckWatch(world, line) bool` | call per received line **before** recording it: `%watchdog`/`%watchname`; true = gag it |
| `in.History(kind, world)` / `HistoryLines` | copy of a history (`HistInput` `HistLocal` `HistGlobal` `HistWorld`), oldest first, plus the number of the first line (numbers never restart) |
| `in.RecallInput(n, mode, cur)` | the line editor's history keys (TF's recall_input) |
| `in.Limited(line)` / `LimitedFor(world, line, attrs)` / `LimitActive()` | true if the foreground screen's `/limit` hides the line |
| `in.SetCurrentWorld(name)` / `CurrentWorld()` | the world the interpreter acts for (TF's xworld). Drives `%{world_name}` etc., `/recall`'s default history, `is_open()` with no argument |
| `in.Worlds()` `World(name)` `DefineWorld(WorldInfo)` `RemoveWorld(name)` `WorldClosed(name)` | the world database (see below) |
| `in.SetFeature(name, bool)` / `Feature` / `FeatureString` | `/features`; the core provides float ftime history locale process subsecond TZ; the client declares 256colors IPv6 MCCPv1 MCCPv2 ssl SOCKS core |
| `in.LogCount()` | `nlog()` |
| `in.WorldMacroBody(name)` | for `${world_name}`, `${world_host}`... (see "Merge notes") |

History capacities follow TF: input 100, local 100, global 1000, a world's
history `%histsize` when its first line arrives (or `/histsize -w`). The input
history always ends with a blank slot (the line being typed), exactly as TF's.
Attribute strings are TF's letters (`g` gag, `G` not recorded, `L` not logged,
`B u r ...`, `Ccolor`); only `g`, `G`, `L` (and "has attributes" for
`/limit -a`) mean anything to the core. `/recall`'s header and footer lines
appear only when tfout goes to the screen (not in a pipe or `$(...)`), as in TF.

## Optional host interfaces (host.go)

* `WorldHost` (existing): `Connect Disconnect Foreground ForegroundWorld Connected`.
* `ConnectHost.ConnectWith(world, ConnectOptions)`: `/connect -l -q -x -f -b`. The world is already in the database (a `(unnamedN)` temporary one for `/connect host port`); read it with `in.World`. Without it, options make `/connect` unsupported.
* `FgHost.ForegroundQuiet(world, quiet)`: `/fg -q`. `Foreground("")` means "no world in the foreground" (`/fg -n`).
* `SocketHost.Sockets() []SocketInfo`: socket list in TF's order (first = what `/fg` picks). Backs `/listsockets`, `/dc -all`, `/fg -< -> -c`, `is_open()`, `is_connected()`, `idle(world)`, `sidle`, `nactive`, and `world_info("host")` of the current world.
* `IdleHost.LastKeyboard()`: `idle()` with no argument.
* `LimitHost.Redraw(world) int`: re-render the screen under the new filter, return how many lines are visible. TF says "No lines matched criteria." and reverts when that is 0. A host with no screen shows nothing, so every `/limit` finds no lines (as tf in batch mode does).
* `RebootHost.Reboot(force)`: `/reboot [-y]`.
* `HelpHost.Help(topic)` (existing): `/help [topic]` (default topic `summary`).
* `Notifier` (existing): events raised: `limit world on|off`, `log on file|off`, `renameworld old new`, `watchdog|watchname on match lines`, `reboot 0|1`, `core text`. `/core` without a Notifier is unsupported.

### No WorldHost
A host without `WorldHost` is treated as one with no sockets: `is_connected()`
etc. return 0, `/dc` says "no current socket", `/fg` says "no socket named",
`/listsockets` says "Not connected to any sockets." Only `/connect` is refused
as unsupported. `idle()` with no argument and `/reboot` need their interface.

## World database

`addworld()` (and the stdlib `/addworld` macro that wraps it), `/unworld`,
`/renameworld`, `/listworlds` (table, `-c`, `-s`, `-u`, `-T`, `-S`, `-m`; table
width is `%wrapsize`, else the terminal width), `/saveworld [-a] [file]`
(default file from the `WORLDFILE` macro; the output reloads with `/load`),
`world_info([world,] field)`, `%{world_*}` variables. Names are
case-insensitive, `default` is the default world (no history). Redefining a
world fires the `REDEF` hook and message. Passwords are stored in clear (as TF
does); `/saveworld` writes them with mode 0600.

Temporary worlds: `/connect host port` defines `(unnamedN)` with `Temp` set.
Call `in.WorldClosed(name)` when its connection ends; it is forgotten unless
the user `/renameworld`ed it.

Privileged operations go through `in.allow`: `/log`, `/saveworld`, `/save` are
file writes; `addworld()` and `/connect host port` are OpNewWorld.

## Differences from real tf (and why)

* **`/edit` on a plain macro**: tf marks it as a hook macro with an empty hook (`-h` in `/list`); that is a bug and is not reproduced. `/edit $text` (select by trigger text) is refused as unsupported.
* **Time-of-day recall ranges** (`0:01`, `11:00-13:00`) work like TF's but are computed in the process's time zone with `in.Now`.
* **`/log` wrapping** (`%wraplog`) breaks lines at blanks within `%wrapsize`/terminal width; TF uses ICU line breaking. `%ansi_log` encodes the line's attribute letters (bold, underline, flash, dim, italic, reverse, 16 colours) as SGR sequences.
* **`/liststreams`** shows the mode letter by reading the descriptor's flags (unix); autoflush is always `on` for writable streams since Go writes are unbuffered. Queue (`q`) and pipe (`p`/`P`) streams are not supported by `tfopen` in this package.
* **`/limit` return values** depend on the client's screen (see `LimitHost`).
* **`features()`** is not a function in tf and is not defined; use `/features`.
* The warning "file contains passwords and is readable by others" on `/load` is not issued.
* `/recall -i` does not include the text currently being typed (the hold_input step of TF); pass it to `RecallInput` instead.
* The reference tf terminated on `/recall -i /2 pattern` while the corpus was being written; we do not reproduce that.

## Merge notes

* `ds()` (misc_state.go) finds per-Interp state through a weak side table because `interp.go` was off limits. If `Interp` gains a field `data *dataState`, replace the body of `ds()` with a lazy field access.
* `outIsScreen()` (history.go) detects piping with `in.Host.(*captureHost)`; keep that name or adapt.
* `cmdEdit` mutates `*Macro` in place (fields `ID Trigger Hook Bind Matching ... re set hasBody`) and `saveload.go`/`worlds.go` use `in.macros`, `in.removeFromList`, `in.parseDef`, `in.listMatch`, `Macro.printDef`.
* `${world_name}`: `interp.go`'s `${...}` lookup should try `in.WorldMacroBody(name)` before `in.macros[name]` (tf's `macro_body`). `%{world_name}` already works (the variables are maintained).
* `in.Hook(event, args)` is used for `LOG` and `REDEF`.
