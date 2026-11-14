# Functions: Time

tf keeps absolute times as seconds since 1970-01-01 UTC with microsecond
precision, and elapsed times in seconds. These functions read the clock, turn
times into text, build a time from a calendar date, and measure idleness. The
text formats follow the C `strftime` conventions and the local time zone (the
`TZ` environment variable).

Time-dependent output cannot be pinned down in a doc, so the examples below
use fixed timestamps (`1710504000` is 2024-03-15 12:00 UTC, which falls on
March 15 in any time zone) or test the *relationship* between values.

## time()

**What it is for:** reading the clock, to timestamp events or measure how long something took.

**Syntax:** `time()` takes no arguments.

**Returns:** the current absolute time in seconds, with a fractional microsecond part.

```tf
/echo $[time() > 1700000000]
/echo $[time() - time() < 1]
```
```output
1
1
```

**Gotchas:** the value has a fraction, so print it through `ftime()` or `trunc()` for display. The library also defines a `/time` command (below) that prints a formatted clock.

**Status:** see [Status](Status.md).

## ftime([format [, time]])

**What it is for:** formatting a time as text for logs, prompts and status lines.

**Syntax:** `ftime()`, `ftime(s)` or `ftime(s, n)`: format `s` (default: the variable `time_format`, normally `%c`-style) applied to time `n` (default: now).

**Returns:** the formatted string, or an empty string with an error message if `n` is out of range. Format codes are the usual ones: `%Y` year, `%m` month, `%d` day, `%H` `%M` `%S` time, `%A` `%a` weekday, `%B` `%b` month name, `%j` day of year, `%%` a literal percent.

```tf
/echo $[ftime("%Y-%m-%d", 1710504000)]
/echo $[ftime("%A %B %d", 1710504000)]
/echo day $[ftime("%j", 1710504000)] of the year
/echo $[ftime("%a %b", 1710504000)] ($[ftime("100%%", 0)])
```
```output
2024-03-15
Friday March 15
day 075 of the year
Fri Mar (100%)
```

**Gotchas:** the date components shift with the time zone, so a log line made with `ftime("%H:%M")` shows the local time. A time too big to convert gives an error, not a wrong date. `%s` gives epoch seconds and the `%n` microsecond extension is supported too; `TZ` set with `/setenv` is honoured.

**Status:** see [Status](Status.md).

## mktime(year [, month [, day [, hour [, minute [, second [, microsecond]]]]]])

**What it is for:** the opposite of `ftime`: converting a calendar date into an absolute time, e.g. for "how long until the festival" macros.

**Syntax:** `mktime(year, month, day, hour, minute, second, microsecond)`, trailing arguments optional. A missing month or day is 1, every other missing field is 0. The date is read in the local time zone.

**Returns:** the time, or -1 if the date cannot be represented. Out-of-range fields roll over (`mktime(2024, 2, 30)` is March 1).

```tf
/echo $[ftime("%Y-%m-%d %H:%M", mktime(2024, 3, 15, 12, 30))]
/echo $[mktime(2024, 3, 15, 12, 30) - mktime(2024, 3, 15, 12, 0)]
/echo $[ftime("%d/%m", mktime(2024, 2, 30))]
```
```output
2024-03-15 12:30
0:30
01/03
```

**Gotchas:** note that the difference of two absolute times is printed in tf's `h:mm` duration style, `0:30` for half an hour). Round-tripping through `ftime` is the safe way to test it since the absolute number depends on your time zone.

**Status:** see [Status](Status.md).

## cputime()

**What it is for:** measuring how much processor time tf itself has used, to profile a heavy macro.

**Syntax:** `cputime()` returns a real number of seconds, or -1 where the system cannot say.

```tf
/echo $[cputime() >= 0]
/test t0 := cputime()
/echo $[cputime() - t0 >= 0]
```
```output
1
1
```

**Gotchas:** the resolution depends on the operating system. Assign it with `:=` rather than through a `%`-substitution to keep full precision. `/runtime` (below) uses it.

**Status:** see [Status](Status.md).

## idle([world])

**What it is for:** finding out how long since the last keypress (no argument) or since text last arrived from a world (with a world name), for away messages and idle timers.

**Syntax:** `idle()` seconds since the last key typed; `idle(s)` seconds since the last line was received from world `s`, or -1 on error.

```tf-noverify
/echo $[idle() >= 0]
/def away_check = /if (idle() > 300) /send afk%; /endif
```
```output
1
```

**Gotchas:** the value is a real with microsecond resolution. Needs a host that knows about keyboard input and connections (capability `WorldHost` for `idle(world)`); see Status for what this build supports.

**Status:** see [Status](Status.md). Host capability: `WorldHost`.

## sidle([world])

**What it is for:** the sending-side partner of `idle`: seconds since you last *sent* a line to the current or named world, for anti-idle keepalives.

**Syntax:** `sidle()` for the current world, `sidle(s)` for world `s`; -1 on error.

```tf
/def keepalive = /if (sidle() > 240) /send look%; /endif
/echo $[sidle() >= 0]
```
```output
1
```

**Gotchas:** a world that has never been sent to counts from when the connection was made. Needs a host with connections (`WorldHost`).

**Status:** see [Status](Status.md). Host capability: `WorldHost`.

## Library

### /time [format] (Library)

**What it is for:** printing the current time at the keyboard, optionally in a custom format.

**Syntax:** `/time` (uses `%time_format`) or `/time format`. On a typed line write each percent of the format twice (`%%Y`), because `%Y` would otherwise be substituted as a variable before the macro sees it.

**Returns:** echoes `ftime(format, time())`.

```tf-noverify
/time %%Y
```
```output
2026
```

**Gotchas:** the date shown is of course the year you run it. In a macro body the same format needs `%%%%Y` (two levels of substitution).

**Status:** library macro from `stdlib.tf`; see [Status](Status.md).

### /runtime command (Library)

**What it is for:** timing how long a command takes, wall-clock and CPU.

**Syntax:** `/runtime command...` runs the command and then reports `real=<seconds> cpu=<seconds>` on the screen.

```tf-noverify
/runtime /echo hi
```
```output
hi
```

**Gotchas:** the `real=... cpu=...` line goes to the screen after the command's own output and its numbers differ on every run. The command's own result code is passed through.

**Status:** library macro from `stdlib.tf`; see [Status](Status.md).
