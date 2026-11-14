# Getting started

## 1. Install

    go get github.com/itsnotyoutoday/gtfparser

## 2. Hello, script

```go
package main

import (
    "fmt"
    "os"

    "github.com/itsnotyoutoday/gtfparser"
)

func main() {
    in := gtfparser.New(gtfparser.Console(os.Stdout))

    in.Run("/echo hello from tf")
    v, _ := in.Eval("strlen('abc') * 2")
    fmt.Println(v) // 6
}
```

`Console` is a ready-made [host](Embedding-Guide.md#the-host): it prints what
scripts echo. `Run` executes text the way a macro body runs; `Eval` evaluates
one expression.

## 3. Share data both ways

```go
in.Run("/set score=40")            // a script sets a variable...
fmt.Println(in.Var("score"))       // ...Go reads it: 40
in.SetVar("bonus", "2")            // Go sets one...
in.Run("/echo $[score + bonus]")   // ...the script uses it: 42
```

## 4. Define macros and feed it lines

```tf
/def square = /result {1} * {1}
/echo 7 squared is $[square(7)]
```
```output
7 squared is 49
```

```tf
/def -t'You hit *' hit = /echo nice hit on %{-2}!
/trigger You hit the orc
```
```output
nice hit on the orc!
```

A *trigger* (`-t`) is a macro attached to a pattern. In your program, text from
the outside world goes through `in.ProcessLine(line)`; any triggers whose
pattern matches fire.

## 5. Run a script file

    go install github.com/itsnotyoutoday/gtfparser/cmd/gtfrun@latest
    gtfrun -n -f myscript.tf

or from Go:

```go
if err := in.LoadFile("myscript.tf"); err != nil { log.Fatal(err) }
```

A script file is a sequence of tf lines: `/def` macros, `/set` variables,
`;` comments, and `\` at the end of a line to continue it.

## 6. Use TinyFugue's standard library

tf ships a library of useful macros (`/first`, `/rest`, `/nth`, `/toggle`,
`/split`, aliases, and much more). It runs unmodified:

```go
in := gtfparser.New(host, gtfparser.WithLibDirs("/path/to/tf-lib"))
in.LoadFile("/path/to/tf-lib/stdlib.tf")
```

```tf-noverify
/echo $(/nth 2 apple banana cherry)      banana      (needs stdlib.tf loaded)
```

## 7. Add your own commands

```go
in.AddCommand(gtfparser.CommandSpec{
    Name: "shout", Usage: "<text>", Help: "echo in capitals", MinArgs: 1,
    Do: func(c *gtfparser.Call) gtfparser.Value {
        c.Echo(strings.ToUpper(c.Rest))
        return gtfparser.Int(1)
    },
})
in.Run("/shout hello there")   // HELLO THERE
```

Bigger sets of commands and functions are declared as a table; see
[Extending the engine](Extending-the-Engine.md).

## Where next

- Learn the language: [Language guide](language/README.md)
- Embed it properly: [Embedding guide](Embedding-Guide.md)
- Ideas: [Use cases](Use-Cases.md)
