package gtfparser_test

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/itsnotyoutoday/gtfparser"
)

// stdoutHost is the smallest possible host: it prints what scripts echo.
type stdoutHost struct{}

func (stdoutHost) Echo(s string)         { fmt.Println(s) }
func (stdoutHost) Send(w, s string) bool { return true }

// Commands, functions, variables and disabled builtins are declared as plain
// data and loaded in one call.
var toolkit = gtfparser.Extension{
	Name: "toolkit",
	Commands: []gtfparser.CommandSpec{
		{Name: "greet", Fn: cmdGreet, Usage: "<name>", Help: "say hello", MinArgs: 1},
		{Name: "roll", Fn: cmdRoll, Usage: "[sides]", Help: "roll a die", MaxArgs: 1},
		{Name: "pwd", Fn: cmdPwd, Help: "show the directory", Needs: []gtfparser.Op{gtfparser.OpFileRead}},
	},
	Functions: []gtfparser.FuncSpec{
		{Name: "clamp", Fn: fnClamp, Usage: "value, low, high", MinArgs: 3, MaxArgs: 3},
	},
	Vars:            map[string]string{"greeting": "Hello"},
	DisableCommands: []string{"sh"},
}

func cmdGreet(in *gtfparser.Interp, args string) gtfparser.Value {
	in.Echo(in.Var("greeting") + ", " + strings.TrimSpace(args) + "!")
	return gtfparser.Int(1)
}

func cmdRoll(in *gtfparser.Interp, args string) gtfparser.Value {
	sides := int64(6)
	if strings.TrimSpace(args) != "" {
		sides = gtfparser.Str(args).Int()
	}
	n := rand.New(rand.NewSource(1)).Int63n(sides) + 1
	in.Echo(fmt.Sprintf("you rolled %d", n))
	return gtfparser.Int(n)
}

func cmdPwd(in *gtfparser.Interp, args string) gtfparser.Value {
	in.Echo("/work")
	return gtfparser.Int(1)
}

func fnClamp(in *gtfparser.Interp, a []gtfparser.Value) (gtfparser.Value, error) {
	v, lo, hi := a[0].Int(), a[1].Int(), a[2].Int()
	return gtfparser.Int(max(lo, min(v, hi))), nil
}

func ExampleInterp_Load() {
	in := gtfparser.New(stdoutHost{}, gtfparser.WithoutEnvironment())
	if err := in.Load(toolkit); err != nil { // validates the whole table first
		panic(err)
	}
	in.ExecFull("/greet world")
	in.ExecFull("/echo $[clamp(15, 0, 10)]")
	in.ExecFull("/greet") // too few arguments: the engine prints the usage
	for _, c := range in.CommandSpecs() {
		fmt.Printf("/%s  %s\n", c.Name, c.Help)
	}
	// Output:
	// Hello, world!
	// 10
	// % usage: /greet <name>
	// /greet  say hello
	// /pwd  show the directory
	// /roll  roll a die
}
