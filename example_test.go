package gtfparser_test

import (
	"fmt"
	"math"
	"strings"

	"github.com/itsnotyoutoday/gtfparser"
)

// Commands, functions, variables and disabled builtins are declared as plain
// data and loaded in one call. Handlers receive parsed input; ordinary Go
// functions become script functions with gtfparser.Func.
var toolkit = gtfparser.Extension{
	Name: "toolkit",
	Commands: []gtfparser.CommandSpec{
		{Name: "greet", Aliases: []string{"hi"}, Do: greet, Options: "s", Usage: "[-s] <name>",
			Help: "say hello (-s: shout)", MinArgs: 1, Category: "social"},
		{Name: "roll", Do: roll, Options: "n#", Usage: "[-n count] [sides]",
			Help: "roll dice", MaxArgs: 1, Category: "games"},
		{Name: "pwd", Do: pwd, Help: "show the directory", Needs: []gtfparser.Op{gtfparser.OpFileRead}},
	},
	Functions: []gtfparser.FuncSpec{
		gtfparser.Func("clamp", func(v, lo, hi int) int { return max(lo, min(v, hi)) }),
		gtfparser.Func("shout", strings.ToUpper), // any Go function will do
		gtfparser.Func("hypot", math.Hypot),
	},
	Vars:            map[string]string{"greeting": "Hello"},
	DisableCommands: []string{"sh"},
}

func greet(c *gtfparser.Call) gtfparser.Value {
	msg := c.Var("greeting") + ", " + c.Rest + "!"
	if c.Has('s') {
		msg = strings.ToUpper(msg)
	}
	c.Echo(msg)
	return gtfparser.Int(1)
}

func roll(c *gtfparser.Call) gtfparser.Value {
	sides := int64(6)
	if c.Arg(0) != "" {
		sides = gtfparser.Str(c.Arg(0)).Int()
	}
	c.Echo(fmt.Sprintf("rolling %dd%d", c.Int('n', 1), sides))
	return gtfparser.Int(1)
}

func pwd(c *gtfparser.Call) gtfparser.Value {
	c.Echo("/work")
	return gtfparser.Int(1)
}

func ExampleInterp_Load() {
	in := gtfparser.New(gtfparser.Console(nil), gtfparser.WithoutEnvironment())
	if err := in.Load(toolkit); err != nil { // the whole table is validated first
		panic(err)
	}
	in.Run("/greet world") // Hello, world!
	in.Run("/hi -s there") // an alias, with an option
	in.Run("/roll -n3 20") // options parsed for you
	in.Run("/echo $[clamp(15, 0, 10)] $[shout('hey')] $[hypot(3, 4)]")
	in.Run("/greet")        // the engine reports bad usage
	in.Run("/greet -x bob") // ...and bad options

	for _, c := range in.CommandSpecs() { // help, generated from the table
		fmt.Printf("%-6s %-8s %s\n", "/"+c.Name, c.Category, c.Help)
	}
	// Output:
	// Hello, world!
	// HELLO, THERE!
	// rolling 3d20
	// 10 HEY 5.
	// % usage: /greet [-s] <name>
	// % /greet: invalid option 'x'
	// % /greet: options: -s
	// /greet social   say hello (-s: shout)
	// /pwd            show the directory
	// /roll  games    roll dice
}
