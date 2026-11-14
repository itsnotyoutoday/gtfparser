# Extending the engine

Your program adds commands, functions and variables, and can switch built-ins
off. You can do it one item at a time, or declare everything as a table.

## One at a time

```go
// a command: the handler gets the input already parsed
in.AddCommand(gtfparser.CommandSpec{
    Name: "roll", Options: "n#", Usage: "[-n count] [sides]", MaxArgs: 1,
    Do: func(c *gtfparser.Call) gtfparser.Value {
        c.Echo(fmt.Sprintf("rolling %dd%s", c.Int('n', 1), cmp.Or(c.Arg(0), "6")))
        return gtfparser.Int(1)
    },
})

// a function: any ordinary Go function
in.AddFunction(gtfparser.Func("clamp", func(v, lo, hi int) int { return max(lo, min(v, hi)) }))
```

```tf-noverify
/roll -n3 20            rolling 3d20
/echo $[clamp(15, 0, 10)]   10
```

## As a table

Anything beyond a couple of items reads better as data:

```go
var toolkit = gtfparser.Extension{
    Name: "toolkit",
    Commands: []gtfparser.CommandSpec{
        {Name: "greet", Aliases: []string{"hi"}, Do: greet, Options: "s", Usage: "[-s] <name>",
            Help: "say hello (-s: shout)", MinArgs: 1, Category: "social"},
        {Name: "roll", Do: roll, Options: "n#", Usage: "[-n count] [sides]",
            Help: "roll dice", MaxArgs: 1, Category: "games"},
        {Name: "pwd", Do: pwd, Help: "show the directory",
            Needs: []gtfparser.Op{gtfparser.OpFileRead}},
    },
    Functions: []gtfparser.FuncSpec{
        gtfparser.Func("clamp", func(v, lo, hi int) int { return max(lo, min(v, hi)) }),
        gtfparser.Func("shout", strings.ToUpper),
        gtfparser.Func("hypot", math.Hypot),
    },
    Vars:            map[string]string{"greeting": "Hello"},
    DisableCommands: []string{"sh"},
}

if err := in.Load(toolkit); err != nil { log.Fatal(err) }
```

`Load` validates the **whole** table first (empty names, missing handlers,
duplicates, impossible argument ranges) and installs nothing if anything is
wrong, so a half-loaded extension never exists.

## Command specs

| Field | Meaning |
|---|---|
| `Name`, `Aliases` | the command name(s), no slash; names are case-insensitive |
| `Do func(*Call) Value` | the implementation, given parsed input (preferred) |
| `Fn func(*Interp, string) Value` | alternative: the raw implementation, given the argument text. Set exactly one of `Do`/`Fn` |
| `Options` | `-options`, in tf's getopts syntax: `v` flag, `n#` integer, `s:` string, `t@` time. `"vn#s:"` accepts `-v -n5 -s'two words'` |
| `MinArgs`, `MaxArgs` | allowed number of words after the options (`MaxArgs: 0` means no limit) |
| `Usage`, `Help`, `Category` | kept for generated help; `Usage` is printed on a bad call |
| `Needs` | privileged operations the command performs; checked against `/restrict` and your `Policy` before your code runs |

Before your handler runs, the engine has already parsed options (reporting
`invalid option 'x'` with the usage line, as tf's own commands do), checked the
word count, and checked privileges.

## The `Call` your handler receives

`Call` embeds the interpreter, so everything is on `c`:

| | |
|---|---|
| `c.Has('s')` | was `-s` given |
| `c.Opt('b')` / `c.Int('n', 1)` | an option's string / integer value (with a default) |
| `c.Arg(i)`, `c.Args` | the i'th word after the options, or all of them |
| `c.Rest` | the text after the options, verbatim |
| `c.Raw` | everything after the command name, options included |
| `c.Fail("...")` | warn as `/name: ...` and return 0 |
| `c.Echo`, `c.Warnf`, `c.Send`, `c.Var`, `c.SetVar`, `c.Run`, `c.Eval`, `c.Allow` | everything the interpreter offers |

Return a `Value`: `gtfparser.Int(1)` for success, `Int(0)` for failure is the
tf convention (scripts test it with `/if`, `/!cmd`, `%?`).

## Functions from Go functions

`gtfparser.Func(name, goFunc)` exposes a Go function with automatic argument
conversion:

| Go parameter | Script value is converted with |
|---|---|
| `int`, `int64`, `uint`... | integer value (strings like `"4x"` read as 4, as in tf) |
| `float64` | numeric value |
| `string` | string value |
| `bool` | truth (non-zero) |
| `gtfparser.Value` | passed through untouched |
| final variadic `...T` | any number of further arguments |

Returns: one value, optionally followed by an `error` (reported as a script
error). The argument count is checked for you, and the usage line is built from
the signature.

For full control write a `FuncSpec{Fn: func(in *Interp, a []Value) (Value, error)}`
yourself.

## Overriding and wrapping built-ins

Later registrations win, built-ins included:

```go
orig, _ := in.Command("echo")
in.RegisterCommand("echo", func(in *gtfparser.Interp, args string) gtfparser.Value {
    audit.Log(args)
    return orig(in, args)           // wrap, don't replace
})
```

## Disabling built-ins

```go
in.DisableCommand("sh")             // scripts get "% /sh: disabled"
in.DisableFunction("tfopen")
// or in a table: DisableCommands: []string{"sh", "quote"}
```

Disabling is separate from [`/restrict` and policies](Safety-and-Sandboxing.md):
a disabled command is gone; a restricted one still exists but refuses
privileged operations.

## Help from your tables

```go
for _, c := range in.CommandSpecs() {          // sorted; aliases listed once
    fmt.Printf("/%-8s %-10s %s\n", c.Name, c.Category, c.Help)
}
```

`FuncSpecs()` does the same for functions. Wire them into a
[`HelpHost`](Embedding-Guide.md#optional-capabilities) and `/help` is generated
from your own declarations.

## Layering several extensions

Load as many tables as you like; the later one wins on a name clash. A typical
program loads, in order: its own defaults, a user-level extension, then
per-session overrides.

## Events and state changes

When state changes that a client might need to react to (the user ran
`/beep`, `/localecho on`, tightened `/restrict`...), the engine raises an
`Event` to your host if it implements `Notifier`:

```go
func (h *myHost) Notify(e gtfparser.Event) {
    switch e.Kind {
    case "beep":     h.ring()
    case "restrict": h.log("restriction level now", e.Args[0])
    }
}
```

The engine owns the *state*; your program owns the *display*.
