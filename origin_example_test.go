package gtfparser_test

import (
	"fmt"

	"github.com/itsnotyoutoday/gtfparser"
)

type exHost struct{}

func (exHost) Echo(s string)         { fmt.Println(s) }
func (exHost) Send(w, s string) bool { return true }

// A host that runs a remote peer's commands under their own restriction: the
// peer may define triggers, hooks and timers (they keep working and stay
// restricted), but never reaches the shell or the files, while the owner's own
// code is untouched.
func ExampleInterp_ExecAs() {
	in := gtfparser.New(exHost{})
	in.Warn = func(s string) { fmt.Println("%", s) }
	in.SetOriginRestriction("peer:alice", gtfparser.RestrictFile)

	in.ExecAs("peer:alice", "/def -t'* hello *' greet = /echo hi back")
	in.ExecAs("peer:alice", "/sh echo got a shell")
	in.ExecAs("peer:alice", "/def sneaky = /sh echo got a shell")
	in.Exec("/sneaky") // alice's macro runs under alice even when the owner calls it
	in.ProcessLine("bob hello world")

	fmt.Println(in.FindMacro("greet").Origin(), in.EffectiveRestriction())
	// Output:
	// % SH: restricted
	// % SH: restricted
	// hi back
	// peer:alice 0
}
