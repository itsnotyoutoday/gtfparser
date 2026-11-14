# Known differences: built-ins and library

gtfparser reproduces TinyFugue's results, formatting, warnings and errors exactly. It deviates only where TinyFugue's output is a defect of its implementation; those are deliberate differences, registered and explained. Everything else on this page is a bug we intend to fix.

Checked against tf 5.0 beta 8 (UTF-8 build) and gtfparser v0.0.6. The deliberate differences (64-bit integers, Unicode case mapping, `char()` above 255, the string-size ceiling) are in [../deliberate-differences.md](../deliberate-differences.md); one more found while writing this reference is listed at the end. Diagnostics wording is covered in [../diagnostics.md](../diagnostics.md).

## Open bugs

### Script values carry no attributes

Attributes exist inside the engine (triggers, `/echo -p`, `/substitute`) but a value passed between functions in a script is plain text, so attribute round trips lose them.

| Script | TinyFugue prints | gtfparser prints |
|---|---|---|
| `/eval /echo [$[encode_attr(decode_attr("@{BCred}Danger@{n} ahead"))]]` | `[@{BCred}Danger@{n} ahead]` | `[Danger ahead]` |
| `/eval /echo [$[encode_attr(decode_attr("hello", "Cgreen"))]]` | `[@{Cgreen}hello@{n}]` | `[hello]` |
| `/eval /echo [$[strcmpattr(decode_attr("@{B}abc"), "abc")]]` | `[4]` | `[0]` |
| `/eval /echo [$[encode_attr(decode_attr("a@{u}b@{n}"))]]` | `[a@{u}b@{n}]` | `[ab]` |

### Type names of durations

| Script | TinyFugue prints | gtfparser prints |
|---|---|---|
| `/eval /echo [$[whatis(sidle())]]` | `[dtime]` | `[decimal]` |
| `/eval /echo [$[whatis(idle())]]` (no host supplying keyboard idle time) | `[dtime]` | warning `idle: idle(): not supported by this client`, then `[int]` |

### /ps -r and /ps -q do not filter

| Script | TinyFugue prints | gtfparser prints |
|---|---|---|
| `/repeat -300 i /send look`, `/quote -300 :x \`/echo q`, then `/eval /echo D[$(/ps -r -s)]` | `D[1]` | `D[1 2]` |
| same two jobs, then `/eval /echo E[$(/ps -q -s)]` | `E[2]` | `E[1 2]` |

### /ps table format

| Script | TinyFugue prints | gtfparser prints |
|---|---|---|
| `/repeat -300 i /send look` then `/eval /echo [$(/ps)]` | `[  PID     NEXT T D WORLD       PTIME COUNT COMMAND     1  0:04:59 r             0:05:00     i /send look]` (a header line, then one line per process) | `[% 1 r 299.999676292 /send look]` |
| `/eval /echo [$(/ps -q)]` with only a `/repeat` running | `[  PID     NEXT T D WORLD       PTIME COUNT COMMAND]` (header only) | `[% 1 r 299.999412917 /send look]` (`-q` ignored) |

### ps() with no process running

| Script | TinyFugue prints | gtfparser prints |
|---|---|---|
| `/eval /echo A[$[ps("-s")]]` (nothing running) | `A[1]` | `A[0]` |

### /sh -q

| Script | TinyFugue prints | gtfparser prints |
|---|---|---|
| `/sh -q echo quiet > /tmp/q.txt`, then read the file | the file contains `quiet` (`-q` is an option of `/sh`, per its manual page) | the file is empty; stderr `sh: -q: invalid option` followed by the shell's usage text, because `-q` is passed on to `/bin/sh` |

### /version and /ver report the engine's identity

| Script | TinyFugue prints | gtfparser prints |
|---|---|---|
| `/version` | `% TinyFugue version development.` then the copyright line, then `% Ingwar Swenson (ingwar@ingwar.eu.org) +ansi_log +log_timestamp +lua +gmcp +ATCP +GMCP +option102`, then `% Built for Darwin Darwin Kernel Version 25.5.0: ...` (four lines) | `% TinyFugue (gtf) version 5.0 beta 8.` then the copyright line (two lines) |
| `/echo [$(/ver)]` | `[development]` | `[5.0 beta 8]` |

## Deliberate differences found while writing this reference

### status_width() of an unknown field

| Script | TinyFugue prints | gtfparser prints |
|---|---|---|
| `/eval /echo w=$[status_width("nosuchfield")]` | tf crashes (`Abnormal termination - signal 11`) and the script ends | warning `% file, line 2: status_width: nosuchfield: no such field`, then `w=0` and the script continues (a crash is an implementation defect, so this is a deliberate difference) |
