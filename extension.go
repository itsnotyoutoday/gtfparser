package gtfparser

import (
	"fmt"
	"sort"
	"strings"
)

// CommandSpec declares one builtin command. A table of specs is the preferred
// way to add commands: the engine validates the table, checks argument counts
// and privileges before calling Fn, and keeps the Usage/Help text so help
// listings can be generated from it.
type CommandSpec struct {
	Name     string            // without the leading slash
	Aliases  []string          // other names for the same command
	Do       func(*Call) Value // the implementation, given parsed input (preferred)
	Fn       CmdFunc           // alternative: the raw implementation, given the argument text
	Options  string            // -options in tf's getopts syntax: "v" flag, "n#" integer, "s:" string, "t@" time
	Usage    string            // e.g. "<name> [greeting]"
	Help     string            // one line
	Category string            // groups commands in help listings
	MinArgs  int               // words after the options
	MaxArgs  int               // 0 = no limit
	Needs    []Op              // privileged operations the command performs (checked against /restrict and Policy)
}

// FuncSpec declares one builtin function.
type FuncSpec struct {
	Name    string
	Fn      FuncFunc
	Usage   string
	Help    string
	MinArgs int
	MaxArgs int // 0 = no limit
	Needs   []Op
}

// Extension bundles everything a client adds to an interpreter, as data.
// Entries load in order, so a later table overrides an earlier one (and the
// built-ins).
type Extension struct {
	Name             string
	Commands         []CommandSpec
	Functions        []FuncSpec
	Vars             map[string]string // initial variable values
	DisableCommands  []string
	DisableFunctions []string
}

// Load validates ext and installs it. It changes nothing if validation fails.
func (in *Interp) Load(ext Extension) error {
	seen := map[string]bool{}
	for _, c := range ext.Commands {
		n := strings.ToLower(c.Name)
		switch {
		case n == "" || strings.ContainsAny(n, " \t/"):
			return fmt.Errorf("extension %q: bad command name %q", ext.Name, c.Name)
		case c.Fn == nil && c.Do == nil:
			return fmt.Errorf("extension %q: command /%s has no implementation", ext.Name, n)
		case c.Fn != nil && c.Do != nil:
			return fmt.Errorf("extension %q: command /%s sets both Do and Fn", ext.Name, n)
		case seen["/"+n]:
			return fmt.Errorf("extension %q: command /%s listed twice", ext.Name, n)
		case c.MaxArgs != 0 && c.MaxArgs < c.MinArgs:
			return fmt.Errorf("extension %q: command /%s has MaxArgs < MinArgs", ext.Name, n)
		}
		seen["/"+n] = true
		for _, al := range c.Aliases {
			a := strings.ToLower(al)
			if a == "" || strings.ContainsAny(a, " \t/") || seen["/"+a] {
				return fmt.Errorf("extension %q: bad or duplicate alias %q for /%s", ext.Name, al, n)
			}
			seen["/"+a] = true
		}
	}
	for _, f := range ext.Functions {
		switch {
		case f.Name == "" || strings.ContainsAny(f.Name, " \t()"):
			return fmt.Errorf("extension %q: bad function name %q", ext.Name, f.Name)
		case f.Fn == nil:
			return fmt.Errorf("extension %q: function %s() has no implementation", ext.Name, f.Name)
		case seen[f.Name+"()"]:
			return fmt.Errorf("extension %q: function %s() listed twice", ext.Name, f.Name)
		case f.MaxArgs != 0 && f.MaxArgs < f.MinArgs:
			return fmt.Errorf("extension %q: function %s() has MaxArgs < MinArgs", ext.Name, f.Name)
		}
		seen[f.Name+"()"] = true
	}
	for _, c := range ext.Commands {
		in.installCommand(c)
	}
	for _, f := range ext.Functions {
		in.installFunction(f)
	}
	for k, v := range ext.Vars {
		in.setGlobal(k, Str(v))
	}
	for _, n := range ext.DisableCommands {
		in.DisableCommand(n)
	}
	for _, n := range ext.DisableFunctions {
		in.DisableFunction(n)
	}
	return nil
}

func (in *Interp) installCommand(c CommandSpec) {
	name := strings.ToLower(c.Name)
	impl := func(in *Interp, args string) Value {
		rest, opts := args, map[byte]Value(nil)
		if c.Options != "" || c.Do != nil {
			var errMsg, usage string
			if c.Options != "" {
				opts, rest, errMsg, usage = parseOptions(c.Options, args)
				if errMsg != "" {
					in.warn("/%s: %s", name, errMsg)
					in.warn("/%s: options: %s", name, usage)
					return Int(0)
				}
			}
		}
		words := strings.Fields(rest)
		if n := len(words); n < c.MinArgs || (c.MaxArgs > 0 && n > c.MaxArgs) {
			in.warn("usage: /%s %s", name, c.Usage)
			return Int(0)
		}
		for _, op := range c.Needs {
			if !in.allow(op, "/"+name, args) {
				return Int(0)
			}
		}
		if c.Do != nil {
			return c.Do(&Call{Interp: in, Name: name, Raw: args, Rest: strings.TrimSpace(rest), Args: words, opts: opts})
		}
		return c.Fn(in, rest)
	}
	if in.cmdSpecs == nil {
		in.cmdSpecs = map[string]CommandSpec{}
	}
	for _, n := range append([]string{name}, c.Aliases...) {
		n = strings.ToLower(n)
		in.RegisterCommand(n, impl)
		in.cmdSpecs[n] = c
	}
}

func (in *Interp) installFunction(f FuncSpec) {
	fn := f.Fn
	in.RegisterFunction(f.Name, func(in *Interp, a []Value) (Value, error) {
		if len(a) < f.MinArgs || (f.MaxArgs > 0 && len(a) > f.MaxArgs) {
			return Int(0), fmt.Errorf("usage: %s(%s)", f.Name, f.Usage)
		}
		for _, op := range f.Needs {
			if !in.allow(op, f.Name+"()", "") {
				return Int(0), fmt.Errorf("%s(): restricted", f.Name)
			}
		}
		return fn(in, a)
	})
	if in.funcSpecs == nil {
		in.funcSpecs = map[string]FuncSpec{}
	}
	in.funcSpecs[f.Name] = f
}

// CommandSpecs lists the commands loaded from tables, sorted by name, for
// help listings.
func (in *Interp) CommandSpecs() []CommandSpec {
	var out []CommandSpec
	for n, c := range in.cmdSpecs {
		if strings.EqualFold(n, c.Name) { // list a command once, not once per alias
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// AddCommand loads a single command; use an Extension for a table.
func (in *Interp) AddCommand(c CommandSpec) error {
	return in.Load(Extension{Name: c.Name, Commands: []CommandSpec{c}})
}

// AddFunction loads a single function; Func(...) builds one from a Go function.
func (in *Interp) AddFunction(f FuncSpec) error {
	return in.Load(Extension{Name: f.Name, Functions: []FuncSpec{f}})
}

// FuncSpecs lists the functions loaded from tables, sorted by name.
func (in *Interp) FuncSpecs() []FuncSpec {
	var out []FuncSpec
	for _, f := range in.funcSpecs {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// The methods below are what a command or function implementation written
// outside this package uses to talk to the user and the world.

// Echo shows text on tfout (honouring a script's tfclose("o")).
func (in *Interp) Echo(text string) { in.echo(text) }

// Warnf prints a "% ..." diagnostic through the interpreter's Warn hook.
func (in *Interp) Warnf(format string, a ...any) { in.warn(format, a...) }

// Send sends a line to a world through the host ("" = the current world).
func (in *Interp) Send(world, text string) bool { return in.Host.Send(world, text) }

// Allow checks a privileged operation against /restrict and the client's
// Policy, warning when it is refused. Handlers that do something privileged
// (and are not declared with CommandSpec.Needs) call it themselves.
func (in *Interp) Allow(op Op, who, detail string) bool { return in.allow(op, who, detail) }
