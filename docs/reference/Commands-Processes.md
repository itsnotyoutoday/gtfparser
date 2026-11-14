# Commands: Processes

tf can run things in the background of your own session: `/repeat` runs a command again and again, `/quote` feeds a stream of lines to the screen, a world or the command interpreter. Each such job is a *process* with a number (pid, unrelated to the operating system's). `/ps` lists them and `/kill` stops them. The library adds `/at`, which schedules a command for a clock time.

*Synchronous* processes (option `-S`) run to completion immediately, before the next command. *Asynchronous* ones are only *scheduled*: they run later, on the client's event loop, either every `%ptime` seconds (or the `-time` you give) or, with `-P`, once per received prompt. Anything typed straight after an asynchronous `/repeat` or `/quote` runs first, even with a zero delay.

## /repeat [-w[world]] [-n] {-time | -S | -P} count command

**What it is for:** running a command a number of times, on a timer: reminders, keep-alives, polling, or delaying a single command (`/repeat -30 1 command` runs it once after 30 seconds).

**Syntax:** `/repeat [-w[world]] [-n] [-time|-S|-P] count command`

**Arguments/Returns:** `count` is a number of runs, or `i` for forever. `command` is any macro body and is expanded each time it runs. `-time` is the delay between runs: seconds (fractions allowed), `minutes:seconds` or `hours:minutes:seconds`; with none of `-time`, `-S`, `-P` the delay is `%ptime`. `-n` makes the first run happen at once instead of after the first delay. `-P` runs on each prompt from the server. `-w[world]` makes `world` current when the command runs (bare `-w` means the world current when you typed `/repeat`; without it, whichever world is in front at that moment). An asynchronous repeat returns its pid (0 on error); a synchronous one returns the result of its last command.

```tf
/repeat -S 3 /echo knock
/repeat -S -n 2 /echo once more
```
```output
knock
knock
knock
once more
once more
```

```tf-noverify
/repeat -0:30 1 /echo -ab Dinner is ready
/repeat -w -60 i /send look
```

**Gotchas:** There is no sleep command: a `/repeat -5 1 /next_step` that re-arms itself is the idiom for "do this, wait, do that". Locals do not survive between runs, so share state in global variables. To wait for a server's reply, use a trigger, not a delay (a `/repeat` makes a handy time-out for the no-reply case). A count of `0` is an error (`REPEAT: invalid repeat count (0)`).

**Status:** see [Status](Status.md)

## /quote  (see also [Commands-Files](Commands-Files.md#quote))

**What it is for:** the other process-making command; its syntax, sources (`'file`, `!shell`, `` `tfcommand ``, `#recall`) and the `-d`, `-w`, `-S`, `-P`, `-time` options are described under [/quote](Commands-Files.md#quote-options-prefix-file--command--tfcommand--recall--suffix). An asynchronous `/quote` is a process: it returns its pid and shows in `/ps`.

```tf-noverify
/quote -0.5 -dsend 'spell_list.txt
```

**Status:** see [Status](Status.md)

## /ps [-srq] [-wworld] [pid]

**What it is for:** seeing what background jobs are running and when they fire next.

**Syntax:** `/ps [-srq] [-wworld] [pid]`

**Arguments/Returns:** lists all `/repeat` and `/quote` processes, or just `pid`. `-s` prints bare pids, one per line, for use in macros (the engine matches real tf); `-r` only repeats, `-q` only quotes, `-w` only those for a world (this build does not yet apply the `-r`/`-q` filters). Called as a function, `ps("-s")` takes a command line like `/ps`; real tf returns 1 (see [Known differences](FINDINGS.md)). The full table has the columns PID, NEXT (time to the next run, or `pending` when waiting for a shell command), T (`r` repeat, `q` quote), D (quote disposition `e`cho, `s`end, e`x`ec), WORLD, PTIME (the delay), COUNT (runs left) and COMMAND.

```tf
/repeat -3600 i /echo hourly report
/quote -3600 :says `/echo hello
/ps -s
/kill 1
/ps -s
```
```output
1
2
2
```

**Gotchas:** Process numbers are per tf session and not reused while it lives. The first of the two jobs above is killed by `/kill 1`, so the second `/ps -s` lists only pid 2.

**Status:** see [Status](Status.md)

## /kill pid...

**What it is for:** stopping runaway or no-longer-wanted processes.

**Syntax:** `/kill pid...` terminates each listed process. Get pids from the return value of `/repeat` and `/quote`, from `/ps`, or from a `PROCESS` hook. A non-numeric or unknown pid prints an error and is skipped.

```tf
/repeat -60 i /echo heartbeat
/repeat -90 i /echo second
/eval /echo A[$(/ps -s)]
/kill 1
/eval /echo B[$(/ps -s)]
/kill 2
/eval /echo C[$(/ps -s)]
```
```output
A[1 2]
B[2]
C[]
```
`/ps` without `-s` prints the table; real tf shows, for one `/repeat -300 i /send look`:
```tf-noverify
  PID     NEXT T D WORLD       PTIME COUNT COMMAND
    3  0:04:59 r             0:05:00     i /send look
```

**Gotchas:** Killing a `/quote !command` that is waiting on a slow shell command blocks until the shell command exits (a documented bug in tf); interrupt with ^C.

**Status:** see [Status](Status.md)

## /at [-v] [date] time command  *(Library, at.tf)*

**What it is for:** running a command at a clock time, such as a daily reminder or a timed login.

**Syntax:** `/at [-v] [[yyyy-]mm-dd] hh:mm[:ss[.uuuuuu]] command`, after `/require at.tf`.

**Arguments/Returns:** `date` is `year-month-day` (2 or 4 digit year) or `month-day`; `time` is `hours:minutes` or `hours:minutes:seconds` (seconds may carry microseconds). A missing part defaults to the nearest future moment: at 16:00, `15:00` means tomorrow 15:00 and `17:00` means today. `-v` prints the full date and time chosen. Implemented with `mktime()` and a `/repeat`, and the command is a macro body, so a time in a past date is rejected (`date and time must be in the future`).

```tf-noverify
/require at.tf
/at 04-01 00:00:00 /echo Happy April Fools Day!
/def lunch_reminder = /at 12:00 /echo Lunchtime!%%;/lunch_reminder
```

**Gotchas:** To repeat daily, a macro re-schedules itself (as above; note the doubled `%%` so the `%;` separator belongs to the at-command, not to `/def`).

**Status:** see [Status](Status.md) (uses `mktime()`)

## /qdef, /qmac, /qworld, /qfile, /qtf, /qsh, /qmud  *(Library, quoter.tf)*

**What they are for:** shortcuts that send the text of a macro, a macro file entry, a world definition, a file, a tf command's output or a shell command's output to the game, one `/quote -S` line at a time, each prefixed with `:|` (override with `%qdef_prefix` etc.), for sharing definitions with other players.

**Syntax:** after `/require quoter.tf`: `/qdef [prefix] name`, `/qmac [prefix] name`, `/qworld [prefix] name`, `/qfile [prefix] name`, `/qtf command`, `/qsh command`, `/qmud [-wworld] command` (the last needs `OUTPUTPREFIX`/`OUTPUTSUFFIX` on the MUD).

```tf-noverify
/require quoter.tf
/def greet = /echo Hello, %1!
/qdef greet
```
Each line of the definition is sent to the game, so a connected world receives
`:| /def greet = /echo Hello, %1!` (the `%1` stays literal in real tf).

**Status:** see [Status](Status.md)

## /signal [signal]  *(Library)*

**What it is for:** sending an operating-system signal to tf itself, for testing signal hooks or breaking out of a stuck state; with no argument it lists signal names.

**Syntax:** `/signal [name-or-number]`. It runs `!kill -sig <tf pid>` through `/quote`; for example `/signal HUP`.

**Gotchas:** `/signal` without an argument runs `kill -l` and does not signal anything.

**Status:** see [Status](Status.md)

## /tick family  *(Library, tick.tf)*

**What it is for:** a Diku-style tick timer on the status line.

**Syntax:** after `/load tick.tf`: `/tick` shows the time to the next tick, `/tickon` and `/tickset` start (reset) the counter, `/tickoff` stops it, `/ticksize n` sets the tick length (default 75 seconds). Messages go through `/echo` using the `tick_warn` and `tick_action` macros, which you may redefine.

**Status:** see [Status](Status.md)
