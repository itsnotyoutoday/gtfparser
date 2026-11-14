package gtfparser

import (
	"os"
	"os/exec"
	"sort"
	"strings"
)

// ---------------------------------------------------------------- stubs & small commands

func cmdListvar(in *Interp, args string) Value {
	pat := ""
	opts := ""
	for strings.HasPrefix(args, "-") {
		var w string
		w, args = firstWord(args)
		opts += w
	}
	pat = strings.TrimSpace(args)
	short := strings.Contains(opts, "s")
	var names []string
	for k := range in.globals {
		if pat == "" || globMatch(pat, k) {
			names = append(names, k)
		}
	}
	sort.Strings(names)
	for _, k := range names {
		if short {
			in.echo(k)
		} else {
			in.echo("/set " + k + "=" + in.globals[k].String())
		}
	}
	return Int(int64(len(names)))
}

func cmdVersion(in *Interp, args string) Value {
	in.echo("% " + in.globals["version"].String())
	return Int(1)
}

func init() {
	builtinCmds["bind"] = cmdBind
	builtinCmds["unbind"] = cmdUnbind
	builtinCmds["restrict"] = cmdRestrict
	builtinCmds["sh"] = cmdSh
	builtinCmds["lcd"] = cmdLcd
	builtinCmds["listvar"] = cmdListvar
	builtinCmds["version"] = cmdVersion

	f := builtinFuncs
	f["columns"] = func(in *Interp, a []Value) (Value, error) { c, _ := in.termSize(); return Int(int64(c)), nil }
	f["lines"] = func(in *Interp, a []Value) (Value, error) { _, l := in.termSize(); return Int(int64(l)), nil }
	f["whatis"] = func(in *Interp, a []Value) (Value, error) {
		if len(a) == 0 {
			return Str(""), nil
		}
		return Str(typeName(a[0])), nil
	}
	f["log10"] = func(in *Interp, a []Value) (Value, error) { return Real(log10(fnum(a, 0))), nil }
	f["ln"] = f["log"]
	f["tan"] = func(in *Interp, a []Value) (Value, error) { return Real(tan(fnum(a, 0))), nil }
	f["gethostname"] = func(in *Interp, a []Value) (Value, error) { h, _ := os.Hostname(); return Str(h), nil }
}

// /bind [seq] = body  is /def -b'seq' = body
func cmdBind(in *Interp, args string) Value {
	for strings.HasPrefix(args, "-") {
		_, args = firstWord(args)
	}
	seq, body := args, ""
	if i := strings.Index(args, "="); i >= 0 {
		seq, body = strings.TrimSpace(args[:i]), strings.TrimLeft(args[i+1:], " \t")
	}
	if seq == "" {
		return Int(0)
	}
	m := &Macro{Matching: "glob", Priority: 1, Bind: seq, Body: body}
	// rebinding a sequence replaces its old macro
	if o := in.FindBinding(seq); o != nil {
		in.killMacro(o)
	}
	in.Define(m)
	return Int(1)
}

func cmdUnbind(in *Interp, args string) Value {
	seq := strings.TrimSpace(args)
	if o := in.FindBinding(seq); o != nil {
		in.killMacro(o)
		return Int(1)
	}
	return Int(0)
}

// /sh [command]: run a shell command, showing its output.
func cmdSh(in *Interp, args string) Value {
	cmd := strings.TrimSpace(args)
	if !in.allow(OpShell, "/sh", cmd) {
		return Int(0)
	}
	if cmd == "" {
		return in.unsupported("/sh (interactive shell)")
	}
	c := exec.Command("sh", "-c", cmd)
	c.Env = in.childEnv()
	out, err := c.CombinedOutput()
	for _, l := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if l != "" || len(out) > 0 {
			in.echo(l)
		}
	}
	return Bool(err == nil)
}

// /lcd [dir]: change (or show) the working directory.
func cmdLcd(in *Interp, args string) Value {
	dir := strings.TrimSpace(args)
	if !in.allow(OpDirChange, "/lcd", dir) {
		return Int(0)
	}
	if dir == "" {
		wd, _ := os.Getwd()
		in.echo("% Current directory is " + wd)
		return Int(1)
	}
	if strings.HasPrefix(dir, "~") {
		if h, err := os.UserHomeDir(); err == nil {
			dir = h + dir[1:]
		}
	}
	if err := os.Chdir(dir); err != nil {
		in.warn("/lcd: %v", err)
		return Int(0)
	}
	return Int(1)
}
