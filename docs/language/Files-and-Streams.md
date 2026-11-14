# Files and streams

tf does all its input and output through *streams*. Commands write to a stream
(`/echo` writes to **tfout**), and functions read and write streams you open
yourself. This page covers opening files, the built-in handles, pipes between
commands, and loading scripts.

## Handles

Every stream has a handle you pass to the `tf*` functions.

| Handle | Stream | Notes |
|---|---|---|
| `'o'` | **tfout** | where `/echo` and most commands write; captured by `$(...)`, `%\|`, `/quote` |
| `'i'` | **tfin** | what `tfread()` reads when you give no handle; fed by `%\|` |
| `'e'` | **tferr** | warnings and errors; always the screen |
| 1, 2, ... | streams you opened with `tfopen()` | integers |

## Files: `tfopen`, `tfwrite`, `tfread`, `tfclose`

`tfopen(name, mode)` opens a file and returns a handle, or `-1` on failure.
Modes: `'r'` read, `'w'` write (truncating), `'a'` append, `'p'` read the output
of a shell command, `'q'` an in-memory queue (see below).

```tf
/set f=$[tfopen('/tmp/gtfdoc_f1.txt', 'w')]
/test tfwrite(f, 'first line')
/test tfwrite(f, 'second line')
/test tfclose(f)
/set r=$[tfopen('/tmp/gtfdoc_f1.txt', 'r')]
/while (tfread(r, line) >= 0) /echo got: %line%; /done
/test tfclose(r)
```
```output
got: first line
got: second line
```

`tfwrite(handle, text)` writes one line. `tfread(handle, var)` reads one line into
the variable `var` (created globally if needed) and returns its length, or `-1`
at end of file. Always `tfclose` what you open, so data is flushed.

Failures are quiet: they return `-1`.

```tf
/echo [$[tfopen('/nonexistent/dir/x', 'r')]]
/echo [$[tfread(999, z)]]
```
```output
[-1]
[-1]
```

Queues and shell pipes:

* `tfopen()` (no arguments, or mode `'q'`) opens a **queue**: lines you
  `tfwrite` can be read back with `tfread`, in order. A queue is a handy
  place to pass lines between commands.
* `tfopen('command', 'p')` runs a shell command and reads its output (it needs
  shell access, which an embedder may refuse).

```tf
/test q := tfopen()
/test tfwrite(q, 'alpha')
/test tfwrite(q, 'beta')
/echo [$[tfread(q, a)]] [%a] [$[tfread(q, b)]] [%b] [$[tfread(q, c)]]
/test tfclose(q)
/set p=$[tfopen('echo piped output', 'p')]
/test tfread(p, l)
/echo [%l] [$[tfread(p, l2)]]
/test tfclose(p)
```
```output
[5] [alpha] [4] [beta] [-1]
[piped output] [-1]
```

Reading an empty queue returns `-1`, like end of file, but more lines may be
written later. Appending:

```tf
/test fa := tfopen('/tmp/gtfdoc_f2.txt', 'w')
/test tfwrite(fa, 'one')
/test tfclose(fa)
/test fa := tfopen('/tmp/gtfdoc_f2.txt', 'a')
/test tfwrite(fa, 'two')
/test tfclose(fa)
/quote -S -decho 'line: ' '/tmp/gtfdoc_f2.txt
```
```output
```

> Embedding programs can veto file access through a policy; a vetoed `tfopen`
> returns `-1`.

## Pipes

`cmd1 %| cmd2` connects **tfout** of the first to **tfin** of the second. The
first runs to completion and then the second reads what it wrote, with
`tfread()` on the default input, until `tfread` returns `-1`.

```tf
/def src = /echo apple%; /echo banana%; /echo cherry
/def sink = /while (tfread(line) >= 0) /echo [%line]%; /done
/src %| /sink
```
```output
[apple]
[banana]
[cherry]
```

Plain-text commands (sent to the world) have no streams and cannot be piped, and
neither end of a pipe can be a bare `/if` or `/while` (put it in a macro).

### `$(...)` is a pipe to a variable

Command substitution is the same capture, joined with spaces:

```tf
/def lines = /echo one%; /echo two%; /echo three
/set joined=$(/lines)
/echo %joined
```
```output
one two three
```

### Silencing output: `tfclose('o')`

Closing **tfout** inside a macro stops everything *that macro* and the commands
it calls would echo for the rest of its body. Handy for quiet helpers:

```tf
/def quiet = /test tfclose('o')%; /echo you will not see this
/quiet
/echo but you see this
```
```output
but you see this
```

Closing `'i'` prevents further reads; closing `'e'` is not allowed.

## Loading scripts

`/load file` reads a file and runs each line as a top-level line (see
[Evaluation model](Evaluation-Model.md)): no substitution, `%;` does not split,
except for `/if` and `/while`. Rules for the file:

* A line beginning with `;` or `#` is a comment. Leading whitespace is stripped.
* A line ending in `\` is joined with the next (whose leading whitespace is
  dropped), so long `/def`s can be split.
* `/load` returns 1 on success, 0 if the file could not be read.
* A relative name is looked for in the current directory, then in the interpreter's
  library directories (`TFPATH` or `TFLIBDIR`; Go: `WithLibDirs`).

```tf
/test f := tfopen('/tmp/gtfdoc_lib1.tf', 'w')
/test tfwrite(f, '/echo lib1 loading')
/test tfwrite(f, '/def lib1_hello = /echo hello from lib1')
/test tfclose(f)
/load /tmp/gtfdoc_lib1.tf
/lib1_hello
/load /tmp/gtfdoc_missing.tf
/echo [%?]
```
```output
lib1 loading
hello from lib1
[0]
```

### The library convention: `/loaded` and `/require`

A file meant to be loaded once starts with `/loaded token`, where `token` is
usually its own full name. `/require file` loads the file only if it has not
been loaded before; `/load` always reads it.

```tf
/test f := tfopen('/tmp/gtfdoc_lib2.tf', 'w')
/test tfwrite(f, '/loaded /tmp/gtfdoc_lib2.tf')
/test tfwrite(f, '/echo lib2 loading')
/test tfclose(f)
/require /tmp/gtfdoc_lib2.tf
/require /tmp/gtfdoc_lib2.tf
/echo done
```
```output
lib2 loading
done
```

Use the same text as the token and as the argument to `/require` (the full path
is a good choice), so the check works however the client implements `/loaded`.

### `/exit`

`/exit` inside a loaded file stops that file (and any macro bodies running in it).
It is the early-out for guard code:

```tf
/test f := tfopen('/tmp/gtfdoc_lib3.tf', 'w')
/test tfwrite(f, '/echo part one')
/test tfwrite(f, '/exit')
/test tfwrite(f, '/echo never reached')
/test tfclose(f)
/load /tmp/gtfdoc_lib3.tf
/echo after load
```
```output
part one
after load
```

`/exit n` abandons `n` nested loads. Outside a `/load` it does nothing.

## Pitfalls

* **Forgetting `tfclose`** on a file you wrote: the data may not be on disk when
  you read it back.
* **Trying to read before the data exists:** `tfread` returns `-1` both at end of
  file and on error; test for `>= 0`, not truthiness (an empty line has length 0).
* **`%x` in a file you generate.** `tfwrite(f, '/echo %x')` typed in a macro
  substitutes `%x` first; write `%%x` to get a literal `%x` in the file.
* **Comments.** A `;` or `#` in the first column makes a comment line. A trailing
  `\` continues the line, unless a `%` comes right before it.
* **File access from untrusted scripts.** Embedders should restrict it
  (policy or `/restrict`); see the README.

## See also

[Evaluation model](Evaluation-Model.md) ·
[Processes and timers](Processes-and-Timers.md) (`/quote` reads files too) ·
[Macros](Macros.md)
