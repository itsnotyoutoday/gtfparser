package gtfparser

import (
	"fmt"
	"io"
	"os"
)

// Run executes script as a macro body: full %-substitution, commands and
// plain text (sent to the current world), separated by %; — what typing it
// with %sub=full would do. It returns the value of the last command.
func (in *Interp) Run(script string) Value { return in.ExecFull(script) }

// Eval evaluates a TinyFugue expression: in.Eval("strlen('abc') * 2").
func (in *Interp) Eval(expr string) (Value, error) { return in.EvalExpr(expr) }

// HostFuncs adapts two functions to a Host, so small programs and tests need
// no new type. A nil function does nothing (and Send reports "no connection").
type HostFuncs struct {
	OnEcho func(text string)
	OnSend func(world, text string) bool
}

func (h HostFuncs) Echo(text string) {
	if h.OnEcho != nil {
		h.OnEcho(text)
	}
}

func (h HostFuncs) Send(world, text string) bool {
	return h.OnSend != nil && h.OnSend(world, text)
}

// Console is a Host that writes echoed text to w (os.Stdout if nil) and
// reports lines sent to a world as "-> line".
func Console(w io.Writer) Host {
	if w == nil {
		w = os.Stdout
	}
	return HostFuncs{
		OnEcho: func(t string) { fmt.Fprintln(w, t) },
		OnSend: func(world, t string) bool { fmt.Fprintln(w, "-> "+t); return true },
	}
}
