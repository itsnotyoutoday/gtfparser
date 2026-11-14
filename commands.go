package gtfparser

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type cmdFunc func(in *Interp, args string) Value

var builtinCmds = map[string]cmdFunc{}

func init() {
	for k, v := range map[string]cmdFunc{
		"echo":    cmdEcho,
		"set":     cmdSet,
		"setenv":  cmdSet,
		"let":     cmdLet,
		"unset":   cmdUnset,
		"test":    cmdTest,
		"return":  cmdReturn,
		"result":  cmdResult,
		"break":   cmdBreak,
		"eval":    cmdEval,
		"send":    cmdSend,
		"def":     cmdDef,
		"undef":   cmdUndef,
		"load":    cmdLoad,
		"require": cmdRequire,
		"loaded":  cmdLoaded,
		"quit":    cmdQuit,
		"exit":    cmdExit,
		"export":  cmdExport,
		"for":     cmdFor,
		"shift":   cmdShift,
	} {
		builtinCmds[k] = v
	}
}

func cmdEcho(in *Interp, args string) Value {
	var o EchoOpts
loop:
	for {
		w, r := firstWord(args)
		switch {
		case w == "-e" || w == "-r" || w == "-n":
			args = r
		case w == "-p":
			o.Markup = true
			args = r
		case w == "-w":
			o.World, args = firstWord(r)
		case strings.HasPrefix(w, "-a") && len(w) > 2:
			o.Attrs = w[2:]
			args = r
		case strings.HasPrefix(w, "-w") && len(w) > 2:
			o.World = w[2:]
			args = r
		default:
			break loop
		}
	}
	in.echoOpts(o, args)
	return Int(1)
}

func cmdSend(in *Interp, args string) Value {
	world := ""
	for {
		w, r := firstWord(args)
		if w == "-w" {
			world, args = firstWord(r)
			_, args = splitFirst(r)
			continue
		}
		if strings.HasPrefix(w, "-w") && len(w) > 2 {
			world, args = w[2:], r
			continue
		}
		if w == "-n" {
			args = r
			continue
		}
		break
	}
	return Bool(in.Host.Send(world, args))
}

func splitFirst(s string) (string, string) { return firstWord(s) }

// nameValue splits "name=value" or "name value".
func nameValue(args string) (name, val string, has bool) {
	args = strings.TrimLeft(args, " \t")
	i := 0
	for i < len(args) && args[i] != ' ' && args[i] != '\t' && args[i] != '=' {
		i++
	}
	name = args[:i]
	if i >= len(args) {
		return name, "", false
	}
	if args[i] == '=' {
		return name, args[i+1:], true
	}
	return name, strings.TrimLeft(args[i:], " \t"), true
}

func cmdSet(in *Interp, args string) Value {
	name, val, has := nameValue(args)
	if name == "" {
		return Int(0)
	}
	if !has {
		v, ok := in.lookup(name)
		if ok {
			in.echo("% " + name + "=" + v.String())
		}
		return Bool(ok)
	}
	return Bool(in.setGlobal(name, Str(val)))
}

func cmdLet(in *Interp, args string) Value {
	name, val, has := nameValue(args)
	if name == "" || !has {
		return Int(0)
	}
	in.fr.vars[name] = Str(val)
	return Int(1)
}

func cmdUnset(in *Interp, args string) Value {
	name, _ := firstWord(args)
	delete(in.globals, name)
	return Int(1)
}

func cmdTest(in *Interp, args string) Value {
	v, err := in.evalExpr(args)
	if err != nil {
		in.warn("/test: %v", err)
		return Int(0)
	}
	return v
}

func cmdReturn(in *Interp, args string) Value {
	in.retv = Str("")
	if strings.TrimSpace(args) != "" {
		v, err := in.evalExpr(args)
		if err != nil {
			in.warn("/return: %v", err)
		}
		in.retv = v
	}
	in.ctl = ctlReturn
	return in.retv
}

func cmdResult(in *Interp, args string) Value {
	v := cmdReturn(in, args)
	if !in.fr.asFunc {
		in.echo(v.String())
	}
	return v
}

func cmdBreak(in *Interp, args string) Value {
	n := 1
	if w, _ := firstWord(args); w != "" {
		if k, err := strconv.Atoi(w); err == nil && k > 0 {
			n = k
		}
	}
	in.ctl, in.brk = ctlBreak, n
	return Int(0)
}

// cmdEval runs its text as a macro body: substitution happens as each
// statement executes (so inside a macro the text is substituted twice).
func cmdEval(in *Interp, args string) Value {
	level := ""
	if strings.HasPrefix(args, "-s") {
		w, r := firstWord(args)
		level, args = w[2:], r
	}
	old := in.noExp
	defer func() { in.noExp = old }()
	in.noExp = level == "0" || level == "off" || level == "literal"
	in.pushFrame(in.fr.name, in.fr.text)
	in.fr.words, in.fr.offs, in.fr.regs = in.fr.parent.words, in.fr.parent.offs, in.fr.parent.regs
	in.fr.asFunc = in.fr.parent.asFunc
	res := in.execText(args)
	in.popFrame()
	return res
}

func cmdFor(in *Interp, args string) Value {
	name, r := firstWord(args)
	a, r := firstWord(r)
	b, body := firstWord(r)
	lo, hi := Value(Str(a)).Int(), Value(Str(b)).Int()
	res := Int(1)
	oldExp := in.noExp
	in.noExp = false // the body is expanded afresh on every pass
	defer func() { in.noExp = oldExp }()
	in.pushFrame(in.fr.name, in.fr.text)
	in.fr.words, in.fr.offs = in.fr.parent.words, in.fr.parent.offs
	for i := lo; i <= hi; i++ {
		in.fr.vars[name] = Int(i)
		res = in.execText(body)
		if in.ctl == ctlBreak {
			in.brk--
			if in.brk <= 0 {
				in.ctl = ctlNone
			}
			break
		}
		if in.ctl != ctlNone {
			break
		}
	}
	in.popFrame()
	return res
}

func cmdDef(in *Interp, args string) Value {
	m, err := in.parseDef(args)
	if err != nil {
		in.warn("%v", err)
		return Int(0)
	}
	in.Define(m)
	return Int(1)
}

func cmdUndef(in *Interp, args string) Value {
	ok := false
	for _, n := range strings.Fields(args) {
		if in.Undef(n) {
			ok = true
		}
	}
	return Bool(ok)
}

func cmdQuit(in *Interp, args string) Value { in.Quit = true; return Int(1) }
func cmdExit(in *Interp, args string) Value { in.ctl = ctlExit; return Int(1) }

func (in *Interp) findFile(name string) (string, bool) {
	if filepath.IsAbs(name) || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "~") {
		if strings.HasPrefix(name, "~/") {
			if h, err := os.UserHomeDir(); err == nil {
				name = filepath.Join(h, name[2:])
			}
		}
		_, err := os.Stat(name)
		return name, err == nil
	}
	for _, d := range append([]string{"."}, in.LibDirs...) {
		p := filepath.Join(d, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, true
		}
	}
	return name, false
}

func cmdLoad(in *Interp, args string) Value {
	quiet := false
	for strings.HasPrefix(args, "-") {
		var w string
		w, args = firstWord(args)
		if strings.Contains(w, "q") {
			quiet = true
		}
	}
	name, _ := firstWord(args)
	if !in.allow(OpFileRead, "/load", name) {
		return Int(0)
	}
	p, ok := in.findFile(name)
	if !ok {
		if !quiet {
			in.warn("/load: %s: no such file", name)
		}
		return Int(0)
	}
	if err := in.LoadFile(p); err != nil {
		in.warn("/load: %v", err)
		return Int(0)
	}
	return Int(1)
}

func cmdRequire(in *Interp, args string) Value {
	name, _ := firstWord(args)
	if in.loaded[name] {
		return Int(1)
	}
	return cmdLoad(in, name)
}

func cmdLoaded(in *Interp, args string) Value {
	name, _ := firstWord(args)
	in.loaded[name] = true
	return Int(1)
}

// LoadFile executes a TF script file. A trailing backslash joins a line
// with the next (whose leading blanks are dropped); lines starting with ';'
// are comments.
func (in *Interp) LoadFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	in.loaded[filepath.Base(path)] = true
	if len(in.LibDirs) > 0 {
		in.globals["TFLIBDIR"] = Str(in.LibDirs[0])
	}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	var cur strings.Builder
	exited := false
	flush := func() {
		line := cur.String()
		cur.Reset()
		if t := strings.TrimSpace(line); t == "" || t[0] == ';' || t[0] == '#' && false {
			return
		}
		in.depth = 0
		in.Exec(strings.TrimLeft(line, " \t"))
		if in.ctl == ctlExit {
			exited = true
		}
		in.ctl = ctlNone
	}
	for sc.Scan() && !exited {
		l := strings.TrimRight(sc.Text(), "\r")
		if cur.Len() > 0 && strings.HasPrefix(strings.TrimLeft(l, " \t"), ";") {
			continue // a comment line inside a continued line
		}
		if cont := strings.HasSuffix(l, "\\") && !strings.HasSuffix(l, "\\\\"); cont {
			if cur.Len() > 0 {
				l = strings.TrimLeft(l, " \t")
			}
			cur.WriteString(l[:len(l)-1])
			continue
		}
		if cur.Len() > 0 {
			l = strings.TrimLeft(l, " \t")
		}
		cur.WriteString(l)
		flush()
	}
	if cur.Len() > 0 {
		flush()
	}
	return sc.Err()
}

// /export name... marks variables for export to child processes (/sh, /quote !).
func cmdExport(in *Interp, args string) Value {
	ok := false
	for _, n := range strings.Fields(args) {
		if _, has := in.lookup(n); has {
			in.exported[n] = true
			ok = true
		}
	}
	return Bool(ok)
}

// childEnv is the environment given to shell commands: the process
// environment plus variables the script exported.
func (in *Interp) childEnv() []string {
	env := environ(in.noEnv)
	for n := range in.exported {
		if v, ok := in.lookup(n); ok {
			env = append(env, n+"="+v.String())
		}
	}
	return env
}
