package gtfparser

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
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
		case w == "-": // ends the options
			args = r
			break loop
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
	if o.World != "" && !in.worldKnown("ECHO", o.World) {
		old := in.curCmd
		in.curCmd = "echo" // tf runs /echo as a macro of that name
		in.warn("No world %s", o.World)
		in.curCmd = old
		return Int(0)
	}
	return in.echoOpts(o, args)
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

// varNameEnd is spanvar(): the length of the variable name at the start of s.
func varNameEnd(s string) int {
	i := 0
	for i < len(s) && (s[i] == '_' || s[i] >= 'a' && s[i] <= 'z' || s[i] >= 'A' && s[i] <= 'Z' || i > 0 && s[i] >= '0' && s[i] <= '9') {
		i++
	}
	return i
}

// setDelim is setdelim() in variable.c: it reads what follows the name in
// /set and /let. kind: -1 invalid, 0 no value, 1 a value (val).
func (in *Interp) setDelim(rest string) (kind int, val string) {
	switch {
	case rest == "":
		return 0, ""
	case rest[0] == ' ' || rest[0] == '\t':
		rest = strings.TrimLeft(rest, " \t")
		if rest == "" {
			return 0, ""
		}
		if rest[0] == '=' {
			in.warn("Warning: '=' following space is part of value.")
		}
		return 1, rest
	case rest[0] == '=':
		return 1, rest[1:]
	}
	return -1, ""
}

// varNameSpecial matches the names tf's looks_like_special_sub accepts.
func varNameSpecial(n string) bool {
	switch {
	case n == "R", n == "PL", n == "PR":
		return true
	case len(n) > 0 && (n[0] == 'L' || n[0] == 'P'):
		for _, c := range n[1:] {
			if c < '0' || c > '9' {
				return false
			}
		}
		return n[0] == 'L' || len(n) > 1
	}
	return false
}

func (in *Interp) setArgs(cmd, args string) (name, val string, kind int) {
	args = strings.TrimLeft(args, " \t")
	n := varNameEnd(args)
	name = args[:n]
	kind, val = in.setDelim(args[n:])
	if kind == -1 {
		j := 0
		for j < len(args) && args[j] != ' ' && args[j] != '\t' && args[j] != '=' {
			j++
		}
		old := in.curCmd
		in.curCmd = cmd // an invalid name falls through to the command proper, which is named
		in.warn("illegal variable name: %s", args[:j])
		in.curCmd = old
	}
	return
}

func cmdSet(in *Interp, args string) Value {
	if strings.TrimSpace(args) == "" {
		return Int(0)
	}
	name, val, kind := in.setArgs("SET", args)
	switch kind {
	case -1:
		return Int(0)
	case 0:
		if v, ok := in.globals[name]; ok {
			in.echo("% " + name + "=" + v.String())
			return Int(1)
		}
		in.echo("% " + name + " not set globally")
		return Int(0)
	}
	return Bool(in.setGlobal(name, Str(val)))
}

func cmdLet(in *Interp, args string) Value {
	if strings.TrimSpace(args) == "" {
		return Int(0)
	}
	name, val, kind := in.setArgs("LET", args)
	if kind == -1 {
		return Int(0)
	}
	if kind == 0 {
		if v, ok := in.fr.vars[name]; ok {
			in.echo("% " + name + "=" + v.String())
			return Int(1)
		}
		in.echo("% " + name + " not set locally")
		return Int(0)
	}
	if _, has := in.fr.vars[name]; !has {
		if _, g := in.globals[name]; g {
			in.FireHookArgs("SHADOW", name)
		}
	}
	in.fr.vars[name] = Str(val)
	return Int(1)
}

func cmdUnset(in *Interp, args string) Value {
	name := strings.TrimSpace(args)
	if sp, ok := specialVars[name]; ok && sp.kind == "pos" {
		in.warn("%s must be a positive integer, so can not be unset.", name)
		return Int(0)
	}
	if _, ok := in.globals[name]; !ok {
		return Int(0)
	}
	delete(in.globals, name)
	return Int(1)
}

func cmdTest(in *Interp, args string) Value {
	v, err := in.evalExpr(args)
	if err != nil {
		in.warn("%v", err)
		return Int(0)
	}
	return v
}

func cmdReturn(in *Interp, args string) Value {
	in.retv = Int(0)
	if strings.TrimSpace(args) != "" {
		v, err := in.evalExpr(args)
		if err != nil {
			in.warn("%v", err)
		}
		in.retv = v
	}
	in.ctl = ctlReturn
	in.userRes = in.retv
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
	in.inEval++
	res := in.execText(args)
	in.inEval--
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

func cmdQuit(in *Interp, args string) Value { in.Quit = true; return Int(1) }
func cmdExit(in *Interp, args string) Value {
	in.ctl = ctlExit
	in.exitReq = true // ends the /load that is running, if any
	return Int(1)
}

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
		for _, c := range w[1:] {
			if c != 'q' {
				return Int(0)
			}
			quiet = true
		}
	}
	name := strings.TrimSpace(args)
	if !in.allow(OpFileRead, "/load", name) {
		return Int(0)
	}
	if name == "" {
		in.warn("/load: missing filename")
		return Int(0)
	}
	if quiet {
		in.quietLoad++
		defer func() { in.quietLoad-- }()
	}
	p, ok := in.findFile(name)
	if !ok {
		in.loadFail(name, cErr(syscall.ENOENT))
		return Int(0)
	}
	if st, err := os.Stat(p); err == nil && st.IsDir() {
		in.loadFail(name, cErr(syscall.EISDIR))
		return Int(0)
	}
	in.FireHookEvent(HookEvent{Name: "LOAD", Args: []string{p}, NoMessage: in.quietLoad > 0})
	if err := in.LoadFile(p); err != nil {
		if err != errLoadAborted {
			in.loadFail(name, cErr(err))
		}
		return Int(0)
	}
	return Int(1)
}

// loadFail reports a failed /load through the LOADFAIL hook, as tf does (the
// default message is an error line; a gagging hook, such as the one stdlib.tf
// installs around its optional local.tf, silences it).
func (in *Interp) loadFail(name, reason string) {
	old := in.curCmd
	in.curCmd = "LOAD"
	defer func() { in.curCmd = old }()
	in.FireHookArgs("LOADFAIL", name, reason)
}

func cmdRequire(in *Interp, args string) Value {
	name, _ := firstWord(args)
	if in.loaded[name] {
		return Int(1)
	}
	return cmdLoad(in, "-q "+name)
}

func cmdLoaded(in *Interp, args string) Value {
	name, _ := firstWord(args)
	in.loaded[name] = true
	return Int(1)
}

// inLibDir reports whether path is inside one of the library directories.
func (in *Interp) inLibDir(path string) bool {
	for _, d := range in.LibDirs {
		if rel, err := filepath.Rel(d, path); err == nil && !strings.HasPrefix(rel, "..") {
			return true
		}
	}
	return false
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
	if in.inLibDir(path) {
		in.macs().libLoad++
		defer func() { in.mac.libLoad-- }()
	}
	if len(in.LibDirs) > 0 {
		in.globals["TFLIBDIR"] = Str(in.LibDirs[0])
	}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	var cur strings.Builder
	exited, failed := false, false
	oldName, oldStart, oldLine, oldCmd := in.loadName, in.loadStart, in.loadLine, in.curCmd
	in.loadName, in.curCmd = path, ""
	defer func() { in.loadName, in.loadStart, in.loadLine, in.curCmd = oldName, oldStart, oldLine, oldCmd }()
	lineNo, startNo, lastCmdLine := 0, 0, 0
	newCmd := true
	for sc.Scan() && !exited {
		l := strings.TrimRight(sc.Text(), "\r")
		lineNo++
		if newCmd {
			startNo = lineNo
		}
		if l != "" && (l[0] == ';' || l[0] == '#') {
			continue // a comment: ';' or '#' in the first column (also inside a continued line)
		}
		t := strings.TrimLeft(l, " \t")
		if t != "" {
			if newCmd && (l[0] == ' ' || l[0] == '\t') && lastCmdLine > 0 {
				in.Warn(fmt.Sprintf("%s: line %d: Warning: possibly missing trailing \\", path, lastCmdLine))
			}
			lastCmdLine = lineNo
			cur.WriteString(t)
			// a trailing backslash continues the line unless it follows a '%' (tf's do_file_load)
			if strings.HasSuffix(t, "\\") {
				if len(t) < 2 || t[len(t)-2] != '%' {
					s := cur.String()
					cur.Reset()
					cur.WriteString(s[:len(s)-1])
					newCmd = false
					continue
				}
			} else if strings.HasSuffix(strings.TrimRight(t, " \t"), "\\") {
				in.loadStart, in.loadLine = startNo, lineNo
				in.curCmd = oldCmd // the loader's own diagnostics name the command that loads
				in.warn("Warning: whitespace following final '\\'")
				in.curCmd = ""
			}
		} else {
			lastCmdLine = 0
		}
		newCmd = true
		if cur.Len() == 0 {
			continue
		}
		line := cur.String()
		if line[0] != '/' {
			in.loadStart, in.loadLine = startNo, lineNo
			in.curCmd = oldCmd
			in.warn("Invalid command. Aborting.")
			in.curCmd = ""
			failed = true
			break
		}
		cur.Reset()
		in.depth = 0
		in.loadStart, in.loadLine = startNo, lineNo
		in.exitReq = false
		in.Exec(line)
		if in.exitReq {
			exited = true
			in.exitReq = false
		}
		in.ctl = ctlNone
	}
	if cur.Len() > 0 {
		in.Warn(fmt.Sprintf("%s: line %d: last command is incomplete because of trailing \\", path, lastCmdLine))
	}
	if failed {
		return errLoadAborted
	}
	return sc.Err()
}

var errLoadAborted = errors.New("load aborted")

// /export name... marks variables for export to child processes (/sh, /quote !).
func cmdExport(in *Interp, args string) Value {
	n := strings.TrimSpace(args)
	if _, has := in.globals[n]; !has {
		in.warn("%s not defined.", n)
		return Int(0)
	}
	in.exported[n] = true
	return Int(1)
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
