# Hook events (for client authors)

A hook is a macro that runs when something happens inside the client
(`/def -hCONNECT greet = ...`). The interpreter owns the hook database,
priority/fall-through/shots/pattern matching and the default messages; **the
client owns the events**: it calls `Interp.FireHook(event, args)` (or
`FireHookArgs(event, args...)`) when the thing happens.

```go
ran := in.FireHook("CONNECT", "mymud")             // after the socket is up: []*Macro that ran
res := in.FireHookArgs("SEND", "look")             // the full *HookResult
if res.Count > 0 { ... }                           // meaningful for SEND, PROMPT
```

`FireHook(event, args string) []*Macro` is the simple form (the arguments are
one string and are also the text hook patterns match); it returns the hook
macros that were selected, so a client can look at their attributes, e.g. a
gag attribute on a `SEND` hook, which in tf means the text is not sent.
`FireHookArgs` takes the arguments separately (the default messages use them)
and returns the `*HookResult`.

`FireHook` does what tf's `do_hook()` does:

1. builds the hook argument text (the arguments joined with single spaces),
2. selects the macros by priority (see *triggers-hooks.md*), applying their
   attributes to the event's default message (so `/def -ag -hCONNECT` hides
   "% Connected to ...");
3. displays the default message, if the event has one — **before** any hook
   macro body runs (tf 5.0 order);
4. runs the selected macros and returns a `*HookResult` (`Count` = number of
   non-quiet macros that ran, `Ran` = the macros selected).

`FireHookEvent(HookEvent{...})` is the same with control over the argument
text, the message (`Message` / `NoMessage`) and the world the event belongs to
(for `/def -w` and `-T` restrictions).

## Where the message goes

Implement `HookDisplayHost` to place messages yourself:

| class         | tf stream            | meaning                                                         |
|---------------|----------------------|-----------------------------------------------------------------|
| `HookScreen`  | tferr (the screen)   | print it in the output window                                   |
| `HookAlert`   | alert (status line)  | show it as a status-line alert                                  |
| `HookWorld`   | the event's world    | print in that world's output; if it is not the foreground world, also alert |

`HookMessage.XSock` means the alert should be cleared when that world is
brought to the foreground. Without `HookDisplayHost`, messages go to
`Host.Echo` (through `AttrEchoHost.EchoAttr` if implemented; error-style
messages through `Interp.Warn`).

## Event table

The arguments are the strings listed; they become the text hook
patterns match. Times marked *(client)* are where only the client can know.

| Event | Arguments | Class | Default message / action |
|-------|-----------|-------|--------------------------|
| ACTIVITY | world | alert, xsock | `% Activity in world <world>`. Fire the first time a background world gets visible text (after PREACTIVITY and the line display). |
| BAMF | world | world, xsock | `% Bamfing to <world>`. A portal moved the connection. |
| BGTEXT | world | | Fire after text was shown in a background world. |
| BGTRIG | world, text | alert, xsock | `% Trigger in world <world>`. Fire when `ProcessText` for a *background* world reported `Count > 0`. |
| CONFAIL | world, reason... | world, xsock | `% Connection to <world> failed: <reason>` |
| CONFLICT | macro | | `warning: macro "<m>" conflicts with the builtin command.` Fired by /def itself. |
| CONNECT | world [, cipher] | world, xsock | `% Connected to <world>[ using cipher <c>].` |
| DISCONNECT | world [, reason...] | world, xsock | `% Connection to <world> closed: <reason>.` / `closed by foreign host.` Not for the client's own /dc. |
| ICONFAIL | world, reason... | world, xsock | `% Intermediate connection to <world> failed: <reason>` |
| KILL | pid | | A process ended. |
| LOAD | file | | `% Loading commands from <file>.` (not for `/load -q`). |
| LOADFAIL | file, reason... | | `<file>: <reason>` (error style). |
| LOG | file | | `% Logging to file <file>`. |
| LOGIN | world, character, password | | Automatic login: fire after CONNECT when `%login` is on and the world has a character and password. The login line itself is the client's job (a hook macro normally sends it). |
| MAIL | file | alert | `% You have new mail in <file>`. |
| MORE | | | The `--More--` prompt was shown or cleared. |
| NOMACRO | name | | `<name>: no such command or macro` (error style). Fired by the interpreter when a command is not found. |
| PENDING | world [, address] | world, xsock | `% Hostname lookup for <world> in progress.` / `% Trying to connect to <world>: <address>.` |
| PREACTIVITY | world | | Immediately before a line is displayed in a background world. |
| PROCESS | pid | | A process started. |
| PROMPT | text | | An unterminated line (prompt) arrived. If `Ran > 0` the client must treat the prompt as consumed. |
| PROXY | world | | A proxy connection completed. |
| REDEF | type, name | | `Redefined <type> <name>` (error style). Fired by the interpreter for macros and bindings; a client fires it for worlds. |
| RESIZE | columns, lines | | The window was resized. |
| SEND | text | | Text is about to be sent. **If `Ran > 0` do not send it** (the hook handled it; a `-q` hook does not count). The interpreter fires it for plain-text lines in macro bodies. |
| SHADOW | var_name | | `Warning: Local variable "<v>" overshadows global variable of same name.` |
| SHELL | type, command | | `% Executing <type>: <command>`. |
| SIGHUP, SIGTERM, SIGUSR1, SIGUSR2 | | | The signal was caught. |
| WORLD | world | world | The foreground world changed. Supply a message with `FireHookEvent` if you want one. |

Optional events — ATCP, GMCP, GMCP_LOGIN, MSDP, OPTION102 (args: the
subnegotiation text, or the world for GMCP_LOGIN) — exist only in tf builds
with those protocols. Call `in.EnableHookEvents("GMCP", "GMCP_LOGIN")` when
your client implements them; until then `/def -hGMCP` fails exactly as in the
reference build.

Three more optional events serve clients that accept remote peers (a "squatter"
daemon built into the client): `SQUAT_CONNECT`, `SQUAT_DISCONNECT` and
`SQUAT_PAIR` (args: device name, role). They are off until the client calls
`in.EnableHookEvents("SQUAT_CONNECT", "SQUAT_DISCONNECT", "SQUAT_PAIR")`.

`HookEvents()` returns the table (with `Doc` and `Args`) programmatically.
`/def -hBACKGROUND` is accepted as a name for BGTRIG.

## Firing order when connecting

1. PROXY (proxy connections only), 2. load the world's macro file (LOAD),
3. CONNECT, 4. LOGIN (if `%login` is on and the world has a login).

## Variables

`%hook` (on/off) disables running hook macros (messages still show);
`%max_hook` (default 1000) limits hooks run in less than ten seconds, after
which `%hook` is turned off with a warning (a `rate` notification is raised
through `Notifier` too).
