package gtfparser

import "strings"

// Helpers for the library commands that a client owns (see docs/stdlib-split.md).
//
// A client that implements one of these commands itself (rather than running
// stdlib.tf's macro) can register the generic part of it from here, so the
// logic is not written twice: the helpers take no client-specific state, and
// they behave exactly like the stdlib.tf macro they port.

// macroScoped runs fn in a macro-like scope named name, so positional
// parameters, locals and diagnostics behave as in the macro it replaces.
func macroScoped(name string, fn cmdFunc) CmdFunc {
	return func(in *Interp, args string) Value {
		return in.callMacro(&Macro{Name: name, native: fn}, strings.TrimRight(args, " \t"), false)
	}
}

// SetCommand returns a command that does what stdlib.tf's
// "/def -i <name> = /set <variable> %*" does: /<name> <value> sets the
// variable <variable> (an empty argument sets it empty, as a macro's does;
// typed "/set x" would list it). stdlib.tf defines /visual, /gpri, /hpri,
// /lpquote and /shpause this way; the engine's own /lp, /wrap and the like
// are built from it.
func SetCommand(name, variable string) CmdFunc {
	return macroScoped(name, func(in *Interp, args string) Value { return in.setCmd(variable, args) })
}

// EchoPrefixCommand returns the command behind /kecho, /mecho and /qecho
// (stdlib.tf's ~do_prefix): with prefix "k", "/kecho on|off|0|1" sets kecho,
// "/kecho text" sets kprefix to text, shows it and turns kecho on. For prefix
// "m", "all" and "2" are also levels of mecho. Diagnostics are reported under
// ~do_prefix, the macro stdlib.tf does the work in.
func EchoPrefixCommand(prefix string) CmdFunc {
	return macroScoped("~do_prefix", func(in *Interp, args string) Value {
		switch {
		case globMatch("{|off|0|on|1}", args), prefix == "m" && globMatch("{all|2}", args):
			return in.setCmd(prefix+"echo", args)
		}
		in.dispatchLine("/set "+prefix+"prefix="+args, func() bool { return true })
		in.plain("/set " + prefix + "prefix")
		return in.plain("/set " + prefix + "echo=1")
	})
}

// AddWorldCommand returns stdlib.tf's /addworld: it parses
// "[-pxe] [-T<type>] [-s<srchost>]" and calls the addworld() function with
// the arguments in the order that function takes them. With a non-empty
// worldType it is the matching /addtiny, /addlp, /addlpp, /adddiku or
// /addtelnet: "/add<type> args" is "/addworld -T<type> args".
func AddWorldCommand(name, worldType string) CmdFunc {
	return macroScoped(name, func(in *Interp, args string) Value {
		if worldType != "" {
			args = "-T" + worldType + " " + args
			in.fr.setArgs(args)
		}
		if !in.getopts("pxeT:s:", Str("")) {
			return in.plain("/return 0")
		}
		flags := ""
		for _, p := range [][2]string{{"p", "opt_p"}, {"x", "opt_x"}, {"e", "opt_e"}} {
			if in.local(p[1]).Truth() {
				flags += p[0]
			}
		}
		w := in.words()
		arg := func(i int) Value {
			if i < len(w) {
				return Str(w[i])
			}
			return Str("")
		}
		T, s := in.local("opt_T"), in.local("opt_s")
		var a []Value
		switch {
		case in.cond(`({1} =/ "default")`):
			a = []Value{arg(0), T, Str(""), Str(""), arg(1), arg(2), arg(3), Str(flags), s}
		case len(w) <= 4:
			a = []Value{arg(0), T, arg(1), arg(2), Str(""), Str(""), arg(3), Str(flags), s}
		default:
			a = []Value{arg(0), T, arg(3), arg(4), arg(1), arg(2), arg(5), Str(flags), s}
		}
		v, err := in.callFuncTf("addworld", a, true)
		if err != nil {
			in.warn("%v", err)
			return in.ret(Int(0))
		}
		return in.ret(v)
	})
}
