package gtfparser_test

import (
	"fmt"
	"os"
	"strings"

	"github.com/itsnotyoutoday/gtfparser"
)

func Example() {
	in := gtfparser.New(gtfparser.Console(os.Stdout))

	in.Run("/echo hello from tf")        // run commands
	v, _ := in.Eval("strlen('abc') * 2") // evaluate expressions
	fmt.Println(v)                       // 6

	in.Run("/set score=40") // scripts and Go share variables
	in.SetVar("bonus", "2")
	in.Run("/echo score is $[score + bonus]")

	// Output:
	// hello from tf
	// 6
	// score is 42
}

func ExampleInterp_Run_macros() {
	in := gtfparser.New(gtfparser.Console(os.Stdout))

	// Definitions are ordinary TinyFugue: anything that works in tf works here.
	in.Exec("/def square = /result {1} * {1}")
	in.Exec("/def -t'* tells you: *' reply = say %{-3}") // a trigger

	in.Run("/echo 7 squared is $[square(7)]")
	in.ProcessLine("Bob tells you: nice to see you") // a line arrives from the game

	// Output:
	// 7 squared is 49
	// -> say nice to see you
}

func ExampleHostFuncs() {
	var log []string
	in := gtfparser.New(gtfparser.HostFuncs{
		OnEcho: func(t string) { log = append(log, "echo:"+t) },
		OnSend: func(world, t string) bool { log = append(log, "send:"+t); return true },
	})
	in.Run("/echo shown%; look")
	fmt.Println(strings.Join(log, " | "))
	// Output: echo:shown | send:look
}

func ExampleNewRunner() {
	in := gtfparser.New(gtfparser.Console(os.Stdout))
	run := gtfparser.NewRunner(in) // the Runner owns the interpreter in one goroutine
	defer run.Close()

	// Any goroutine (a socket reader, a UI, a timer) can drive it safely.
	run.Do(func(in *gtfparser.Interp) { in.Run("/set hp=30") })
	run.Do(func(in *gtfparser.Interp) { in.Run("/echo hp is %{hp}") })

	// run.Interrupt() stops a runaway script from anywhere.

	// Output:
	// hp is 30
}

type denyFiles struct{}

func (denyFiles) Allow(op gtfparser.Op, detail string) error {
	if op == gtfparser.OpFileWrite {
		return fmt.Errorf("writing %s is not allowed here", detail)
	}
	return nil
}

func ExampleWithPolicy() {
	in := gtfparser.New(gtfparser.Console(os.Stdout),
		gtfparser.WithPolicy(denyFiles{}),                  // your rules...
		gtfparser.WithRestriction(gtfparser.RestrictShell), // ...on top of tf's own /restrict
		gtfparser.WithLimits(gtfparser.Limits{MaxInstr: 10_000}))

	in.Run("/sh rm -rf /")                            // refused: shell access is restricted
	in.Exec("/def spin = /while (1) /test 1%; /done") // a definition, as in a script file
	in.Run("/spin")                                   // stopped at the instruction limit

	// Output:
	// % /sh: restricted
	// % max_instr (10000) exceeded; execution aborted
}
