# Built-in reference

Every builtin function and command of TinyFugue, plus the macros that tf's
standard library (`lib/tf/stdlib.tf`) defines, grouped by what you use them for.
Each entry says what the item is for, its syntax, a verified example, gotchas,
and points at [Status](Status.md) for what this build of the engine supports.

Status is **generated from the code** (`go run ./cmd/gtfdoc > docs/reference/Status.md`),
so it stays correct as features land; the per-entry prose describes what the
command does in tf and does not claim support either way.

## Functions

| Page | Contents |
|---|---|
| [Functions-Strings](Functions-Strings.md) | strlen, substr, strcat, strstr, strchr, strrchr, strcmp, strncmp, strrep, replace, pad, tolower, toupper, ascii, char |
| [Functions-Math](Functions-Math.md) | abs, sqrt, pow, exp, ln, log10, trig, mod, trunc, rand |
| [Functions-Time](Functions-Time.md) | time, ftime, mktime, cputime, idle, sidle |
| [Functions-Regex-and-Eval](Functions-Regex-and-Eval.md) | regmatch, eval, test, getopts, substitute |
| [Functions-Files-and-Streams](Functions-Files-and-Streams.md) | tfopen, tfclose, tfread, tfreadable, tfwrite, tfflush, fwrite, filename |
| [Functions-World-and-Connection](Functions-World-and-Connection.md) | fg_world, is_connected, is_open, world_info, addworld, nactive |
| [Functions-Terminal-and-Input](Functions-Terminal-and-Input.md) | kb* functions, keycode, columns, lines, status_fields, status_width, moresize |
| [Functions-Attributes](Functions-Attributes.md) | decode_attr, encode_attr, decode_ansi, encode_ansi, strip_attr, strcmpattr |
| [Functions-Protocol](Functions-Protocol.md) | gmcp, msdp, atcp, option102 |
| [Functions-Misc](Functions-Misc.md) | everything else |

## Commands

| Page | Contents |
|---|---|
| [Commands-Macros-and-Variables](Commands-Macros-and-Variables.md) | /def, /undef, /list, /set, /let, /export, /trigger, /hook, /bind ... |
| [Commands-Control](Commands-Control.md) | /if, /while, /for, /eval, /return, /result, /break, /exit, /test, /shift ... |
| [Commands-Output-and-Input](Commands-Output-and-Input.md) | /echo, /send, /input, /beep, /localecho ... |
| [Commands-Files](Commands-Files.md) | /load, /require, /save, /log, /lcd, /sh ... |
| [Commands-Worlds-and-Connections](Commands-Worlds-and-Connections.md) | /connect, /dc, /fg, /listworlds, /addworld ... |
| [Commands-History-and-Display](Commands-History-and-Display.md) | /recall, /recordline, /histsize, /limit, /more, /status_add ... |
| [Commands-Processes](Commands-Processes.md) | /repeat, /quote, /ps, /kill, /at |
| [Commands-System](Commands-System.md) | /restrict, /suspend, /quit, /watchdog, /features, /help, /version ... |

## Other pages

- [Status](Status.md): generated table of every builtin and what this build supports.
- [FINDINGS](FINDINGS.md): places where the engine differs from real tf.

## Entry conventions

- `s`, `i`, `r`, `n`, `f` in a signature mean string, integer, real, any number, flag (0/"off" or 1/"on").
- **Library** marks a macro defined by tf's own standard library rather than a C builtin;
  it is available when the library is loaded.
- Examples are a `tf` block followed by an `output` block holding exactly what the lines echo.
  `scripts/doccheck.py` runs them: `go build -o /tmp/gtfrun ./cmd/gtfrun && python3 scripts/doccheck.py --lib $TF_LIB docs/reference`.
  Examples tagged `tf-noverify` are not run (for builtins that are not yet supported).
- Where the engine deliberately matches an oddity of real tf, the entry says so under **Gotchas**.
