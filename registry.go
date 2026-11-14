package gtfparser

import "strings"

// CmdFunc implements a builtin command. args is the text after the command
// name (already substituted, unless the line was a top-level one).
type CmdFunc = cmdFunc

// FuncFunc implements a builtin function.
type FuncFunc = fn

// The package-level tables are the DEFAULTS. Each Interp copies them at New,
// so a client can add, override or disable commands and functions for one
// interpreter without touching any other.
var defaultCmds = map[string]cmdFunc{}
var defaultFuncs = map[string]fn{}

func (in *Interp) initRegistry() {
	in.cmds = make(map[string]cmdFunc, len(builtinCmds))
	for k, v := range builtinCmds {
		in.cmds[k] = v
	}
	in.funcs = make(map[string]fn, len(builtinFuncs))
	for k, v := range builtinFuncs {
		in.funcs[k] = v
	}
	in.disabled = map[string]bool{}
	// every builtin real tf has but this package lacks fails loudly
	for _, n := range tfCommands {
		if _, ok := in.cmds[n]; !ok {
			name := n
			in.cmds[n] = func(in *Interp, args string) Value { return in.unsupported("/" + name) }
		}
	}
	for _, n := range tfFunctions {
		if _, ok := in.funcs[n]; !ok {
			name := n
			in.funcs[n] = func(in *Interp, a []Value) (Value, error) {
				in.unsupported(name + "()")
				return Int(0), nil
			}
		}
	}
}

// RegisterCommand installs (or replaces) the builtin command /name.
func (in *Interp) RegisterCommand(name string, f CmdFunc) {
	name = strings.ToLower(name)
	in.cmds[name] = f
	delete(in.disabled, "/"+name)
}

// RegisterFunction installs (or replaces) the builtin function name().
func (in *Interp) RegisterFunction(name string, f FuncFunc) {
	in.funcs[name] = f
	delete(in.disabled, name+"()")
}

// DisableCommand makes /name fail with a "disabled" warning. User macros of
// the same name are unaffected; use it to keep scripts away from a feature.
func (in *Interp) DisableCommand(name string) {
	name = strings.ToLower(name)
	in.disabled["/"+name] = true
}

// DisableFunction makes name() fail with a "disabled" error.
func (in *Interp) DisableFunction(name string) { in.disabled[name+"()"] = true }

// Command returns the current implementation of /name, so an override can
// wrap the default.
func (in *Interp) Command(name string) (CmdFunc, bool) {
	f, ok := in.cmds[strings.ToLower(name)]
	return f, ok
}

// Function returns the current implementation of name().
func (in *Interp) Function(name string) (FuncFunc, bool) {
	f, ok := in.funcs[name]
	return f, ok
}

// unsupported is what a command does when the host lacks the capability it
// needs: say so, and fail. Nothing in this package silently pretends to work.
func (in *Interp) unsupported(what string) Value {
	in.warn("%s: not supported by this client", what)
	return Int(0)
}
