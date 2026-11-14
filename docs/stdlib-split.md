# What stdlib.tf still has to do

tf's `stdlib.tf` defines the commands that are plain language and state operations (`/first`,
`/listgag`, `/lp`, `/paste` ...) as invisible (`-i`) macros. gtfparser implements those itself, so
every client has them whether or not it loads `stdlib.tf`, and a script no longer has to be parsed to
run them. The rest of `stdlib.tf` stays script (it is tf's own wiring of worlds, logins and trigger
shorthands) or is implemented by each client (it needs the client's connections, terminal or files).

The library is still loaded unchanged: there is no trimmed copy of it to maintain. When `stdlib.tf`
defines a macro whose body is exactly the one the engine ports, the macro keeps its number, name,
flags and printed body, but running it runs Go code. Anything else with the same name (a modified
library, a user's own definition) is an ordinary script macro.

## Tier 1: native in the engine (73 commands)

Each is an ordinary command (`RegisterCommand` semantics: a user's `/def` of the same name shadows
it), and also the implementation behind the stdlib macro of that name.

| Group | Commands |
|---|---|
| Output | `/echo` `/_echo` `/prompt` `/substitute` |
| Null commands | `/true` `/false` `/:` |
| Library loading | `/require` `/loaded` |
| Control and expressions | `/not` `/expr` `/for` `/toggle` |
| Word and string helpers | `/first` `/rest` `/last` `/nth` `/escape` `/replace` `/split` |
| Input capture | `/cat` `/paste` `/_paste` |
| Time, numbers, version | `/time` `/rand` `/ver` `/vercmp` `/runtime` |
| Macro and variable tests | `/ismacro` `/isvar` |
| List family | `/listdef` `/listhook` `/listtrig` `/listgag` `/listhilite` `/listfullhilite` `/listpartial` `/listbind` |
| Purge family | `/purgedef` `/purgehilite` `/purgegag` `/purgetrig` `/purgedeft` `/purgebind` `/purgehook` `/untrig` `/unhook` `/undeft` |
| Save | `/savehilite` |
| `/set` wrappers | `/redef` `/more` `/wrap` `/isize` `/ptime` `/wrapspace` `/act` `/reply` `/background` `/bamf` `/borg` `/clearfull` `/cleardone` `/insert` `/login` `/lp` `/quiet` `/quitdone` `/sockmload` `/sub` `/nolog` `/nowrap` `/nologin` `/noquiet` |
| Constants | the variables `pi` and `e` exist from the start |

`/act` runs `/trig` (which stays script, tier 3), so a user's own `/trig` is honoured.
`NativeLibraryCommands()` returns the names; `WithoutNativeLibrary()` (an `Option`; `gtfrun -script`)
keeps every library macro as script, for comparing the two.

### Behaves as in real tf

These were compared with real tf (running its own `stdlib.tf`) in `testdata/cases/native_lib_*.tf`
and, for warnings, with `scripts/warncheck.py --probe`; every native is also checked against its
own script body (`TestNativeMatchesScript`).

- `/list -i`, `/list` numbers, `/ismacro`, `/edit`, `/undef`: the library macros are invisible,
  keep their numbers, and `/undef` removes them. After `/undef lp`, `/lp` is "no such command or
  macro", and `/@lp` is "not a builtin command" while `stdlib.tf` has defined it, exactly as in tf
  (where these are macros, not builtins).
- Typed `/set lp` lists the variable but `/lp` from a macro with an empty argument sets it empty
  (the empty `%*` still counts as a value).
- Arguments never end in blanks (a macro's argument vector is cut at the last word); this now also
  applies to `/echo` when no stdlib is loaded.
- Warnings carry the macro's name (`lp: Invalid lp value "".  Valid values are: off (0), on (1)`)
  and getopts failures read `echo -x: invalid option` / `echo: options: ...`.

### Without stdlib.tf

The commands exist as builtins (and as functions: `first("a b", "c")` calls the library macro, as in
tf). A client that does not load `stdlib.tf` therefore still has the whole tier. Real tf has no such
mode; there `/@first` and `/ismacro first` have no reference answer, so nothing is registered as a
deviation: with `stdlib.tf` loaded the behaviour is tf's.

## Tier 2: implemented by each client

These need the client's connections, terminal, process or files. The engine does not define them;
stdlib.tf's macro runs unless the client registers its own command of that name.

| Group | Commands |
|---|---|
| Files and shell | `/sys` `/cd` `/pwd` `/man` `/signal` `/loadhist` `/loaddef` `/loadhilite` `/loadgag` `/loadtrig` `/loadbind` `/loadhook` `/loadworld` `/savedef` `/savegag` `/savetrig` `/savebind` `/savehook` |
| Worlds and connections | `/addworld` `/addtiny` `/addlp` `/addlpp` `/adddiku` `/addtelnet` `/purgeworld` `/world` `/telnet` `/bg` `/retry` `/retry_off` `/proxy_hook` `/proxy_command` `/proxy_connect` |
| Terminal and key handling | `/visual` `/gpri` `/hpri` `/lpquote` `/shpause` `/kecho` `/mecho` `/qecho` (`~do_prefix`) |

Where the logic is generic the engine exports it, so a client does not write it twice. Each takes
no client state and behaves like the macro it ports (`TestHelperCommandsMatchScript`):

| Helper | For |
|---|---|
| `SetCommand(name, variable)` | `/visual`, `/gpri`, `/hpri`, `/lpquote`, `/shpause`: `/name x` sets the variable (an empty argument sets it empty) |
| `EchoPrefixCommand(prefix)` | `/kecho`, `/mecho`, `/qecho` (`"k"`, `"m"`, `"q"`) |
| `AddWorldCommand(name, type)` | `/addworld` (type `""`) and `/addtiny` `/addlp` `/addlpp` `/adddiku` `/addtelnet`; calls the engine's `addworld()` function |

```go
in.RegisterCommand("kecho", gtfparser.EchoPrefixCommand("k"))
in.RegisterCommand("addtiny", gtfparser.AddWorldCommand("addtiny", "tiny"))
```

`/world`, `/purgeworld`, `/telnet`, `/retry` and `/proxy_*` are orchestration of the client's own
`/connect`, `/fg`, `/listworlds` and hooks and need no helper: the stdlib.tf macros work as they are.

## Tier 3: stays in stdlib.tf

| Group | Macros |
|---|---|
| Trigger shorthands | `/trig` `/trigc` `/trigp` `/partial` `/nogag` `/nohilite` `/hilite_whisper` `/hilite_page` `/nohilite_whisper` `/nohilite_page` |
| Sending | `/send` `~send` |
| Login and world wiring | `~detect_worldtype_hook` `~default_login_hook` `~world_hook_tiny` `~login_hook_tiny` `~world_hook_prompt` `~world_hook_lp` `~login_hook_lp` `~world_hook_lpp` `~login_hook_lpp` `~con_hook_telnet` `~world_hook_telnet` `~login_hook_telnet` `~telnet_passwd` |
| Help | `send_help` (the `SEND` hook for newbies) |
| File name macros | `MACROFILE` `HILITEFILE` `GAGFILE` `TRIGFILE` `BINDFILE` `HOOKFILE` `WORLDFILE` `LOGFILE` `~def_file_command`, `COMPRESS_SUFFIX` `COMPRESS_READ` |
| Other | `/grab` (input-line editing), `~gagloadfail` (silences the optional `local.tf`), the `MAILPATH` to `TFMAILPATH` copy, `%maxpri`, `%TINYPREFIX` and the loading of `tfstatus.tf`, `kbbind.tf`, `world-q.tf`, `color.tf`, `changes.tf`, `at.tf` |

These are tf's configuration, not language operations: hooks that react to a world's type, shorthand
for `/def` option strings, default file names. They stay script so they remain editable and stay
exactly tf's.

## The golden table

`TestLibraryMacroTable` (`GTF_TF_LIB` set) loads the real `stdlib.tf` and compares every macro
with the table real tf printed (`testdata/golden/libtable.txt`). The natively provided macros are
accounted for by an explicit allowlist (`nativeAllowlist` in `nativelib_test.go`): each must still be
in the table with tf's number and body, and must be bound to its Go implementation, and no other
macro may be. `TestNativeAllowlist` checks the allowlist against the registry and that every
allowlisted name works without any script (`TestNativeRunsWithoutScript`).
`TestNativeFingerprints` pins each Go implementation to the exact `stdlib.tf` definition it ports
(regenerate `nativelib_fp.go` with `GTF_GEN_FP=1` when the library changes).
