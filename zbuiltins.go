package gtfparser

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ---------------------------------------------------------------- tfio

type stream struct {
	f  *os.File
	r  *bufio.Reader
	w  bool
	id string
}

func (in *Interp) stream(id string) *stream {
	if in.streams == nil {
		return nil
	}
	return in.streams[id]
}

func fnTfopen(in *Interp, a []Value) (Value, error) {
	path, mode := str(a, 0), str(a, 1)
	if mode == "" {
		mode = "r"
	}
	op := OpFileRead
	if mode[0] != 'r' {
		op = OpFileWrite
	}
	if mode[0] == 'p' {
		op = OpShell
	}
	if !in.allow(op, "tfopen()", path) {
		return Int(-1), nil
	}
	var f *os.File
	var err error
	if strings.HasPrefix(path, "~/") {
		if h, e := os.UserHomeDir(); e == nil {
			path = filepath.Join(h, path[2:])
		}
	}
	switch mode[0] {
	case 'w':
		f, err = os.Create(path)
	case 'a':
		f, err = os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	default:
		f, err = os.Open(path)
	}
	if err != nil {
		return Int(-1), nil
	}
	if in.streams == nil {
		in.streams = map[string]*stream{}
	}
	in.nextFd++
	id := fmt.Sprint(in.nextFd)
	st := &stream{f: f, w: mode[0] != 'r', id: id}
	if !st.w {
		st.r = bufio.NewReader(f)
	}
	in.streams[id] = st
	return Int(int64(in.nextFd)), nil
}

func fnTfclose(in *Interp, a []Value) (Value, error) {
	id := str(a, 0)
	if id == "o" {
		in.fr.noOut = true
		return Int(1), nil
	}
	if id == "i" || id == "e" {
		return Int(1), nil
	}
	st := in.stream(id)
	if st == nil {
		return Int(0), nil
	}
	st.f.Close()
	delete(in.streams, id)
	return Int(1), nil
}

func fnTfwrite(in *Interp, a []Value) (Value, error) {
	if len(a) == 1 { // tfwrite(text): to tfout
		in.echo(str(a, 0))
		return Int(1), nil
	}
	id, text := str(a, 0), str(a, 1)
	if id == "o" {
		in.echo(text)
		return Int(1), nil
	}
	st := in.stream(id)
	if st == nil || !st.w {
		return Int(0), nil
	}
	_, err := st.f.WriteString(text + "\n")
	return Bool(err == nil), nil
}

func fnTfread(in *Interp, a []Value) (Value, error) {
	if len(a) <= 1 { // tfread([var]): read a line from tfin
		if len(in.tfin) == 0 {
			return Int(-1), nil
		}
		line := in.tfin[0]
		in.tfin = in.tfin[1:]
		if len(a) == 1 {
			in.assign(str(a, 0), Str(line))
			return Int(int64(len(line))), nil
		}
		return Str(line), nil
	}
	st := in.stream(str(a, 0))
	if st == nil || st.r == nil {
		return Int(-1), nil
	}
	line, err := st.r.ReadString('\n')
	if err != nil && line == "" {
		return Int(-1), nil
	}
	line = strings.TrimRight(line, "\r\n")
	if len(a) > 1 { // tfread(fd, var)
		in.assign(str(a, 1), Str(line))
		return Int(int64(len(line))), nil
	}
	return Str(line), nil
}

func fnTfreadable(in *Interp, a []Value) (Value, error) {
	st := in.stream(str(a, 0))
	return Bool(st != nil && st.r != nil), nil
}

// ---------------------------------------------------------------- stubs & small commands

// transform /gag, /hilite, /trig, /hook into /def.
func defShortcut(prefix string) cmdFunc {
	return func(in *Interp, args string) Value {
		w := args
		for strings.HasPrefix(w, "-") {
			_, w = firstWord(w)
		}
		pat, body := w, ""
		if i := strings.Index(w, "="); i >= 0 && prefix != "-h" {
			pat, body = strings.TrimSpace(w[:i]), strings.TrimSpace(w[i+1:])
		}
		spec := prefix + " "
		if prefix == "-h" {
			spec = "-h'" + strings.ReplaceAll(pat, "'", "\\'") + "' = " + body
			return cmdDef(in, spec)
		}
		return cmdDef(in, spec+"-t'"+strings.ReplaceAll(pat, "'", "\\'")+"' = "+body)
	}
}

// listMatch reports whether macro m satisfies the options given to /list.
func (in *Interp) listMatch(spec, m *Macro, style string) bool {
	_, wantInv := spec.set['i']
	if m.Invisible && !wantInv && !spec.OnlyInvis {
		return false
	}
	if spec.OnlyInvis && !m.Invisible {
		return false
	}
	pm := func(pat, s string) bool {
		if pat == "{}" || pat == "^$" {
			return s == ""
		}
		switch style {
		case "simple":
			return pat == s
		case "regexp":
			re, err := compileRegexp(pat)
			return err == nil && re.match(s, 0) != nil
		case "substr":
			return strings.Contains(s, pat)
		}
		return globMatch(pat, s)
	}
	for c, v := range spec.set {
		switch c {
		case 't':
			if v == "" && m.Trigger == "" || v != "" && !pm(v, m.Trigger) {
				return false
			}
		case 'b':
			if v == "" && m.Bind == "" || v != "" && !pm(v, m.Bind) {
				return false
			}
		case 'h':
			if v == "0" && m.Hook != "" || v == "" && m.Hook == "" || v != "" && v != "0" && !pm(v, m.Hook) {
				return false
			}
		case 'T':
			if v == "" && m.WorldType == "" || v != "" && !pm(v, m.WorldType) {
				return false
			}
		case 'E':
			if v == "" && m.Cond == "" || v != "" && !pm(v, m.Cond) {
				return false
			}
		case 'a':
			if v == "" && m.Attrs == "" || v != "" && !strings.ContainsAny(m.Attrs, v) {
				return false
			}
		case 'n':
			if m.Shots != int(Str(v).Int()) {
				return false
			}
		case 'p':
			if x, err := in.evalExpr(v); err != nil || m.Priority != int(x.Int()) {
				return false
			}
		case 'w':
			if v != "" && !pm(v, m.World) {
				return false
			}
		case 'F':
			if !m.Fall {
				return false
			}
		case 'q':
			if !m.Quiet {
				return false
			}
		}
	}
	if spec.Name != "" {
		if strings.HasPrefix(spec.Name, "#") {
			if int(Str(spec.Name[1:]).Int()) != m.ID {
				return false
			}
		} else if !pm(spec.Name, m.Name) {
			return false
		}
	}
	if spec.hasBody && !pm(spec.Body, m.Body) {
		return false
	}
	return true
}

func cmdList(in *Interp, args string) Value {
	short := false
	// -s and -S are list options; the rest are /def options compared to macros
	var rest []string
	text := strings.TrimLeft(args, " \t")
	for strings.HasPrefix(text, "-") && len(text) > 1 {
		var w string
		w, text = splitOptWord(text)
		if w == "-s" {
			short = true
			continue
		}
		if w == "-S" {
			continue
		}
		rest = append(rest, w)
	}
	spec, err := in.parseDef(strings.Join(rest, " ") + " " + text)
	if err != nil {
		in.warn("/list: %v", err)
		return Int(0)
	}
	style := spec.Matching
	last := 0
	for _, m := range in.Macros() {
		if !in.listMatch(spec, m, style) {
			continue
		}
		last = m.ID
		if short {
			in.echo(m.printShort())
		} else {
			in.echo(m.printDef(true))
		}
	}
	return Int(int64(last))
}

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

func cmdPurge(in *Interp, args string) Value {
	n := 0
	pat := strings.TrimSpace(args)
	for _, m := range in.Macros() {
		if pat == "" || globMatch(pat, m.Name) {
			in.Undef(m.Name)
			n++
		}
	}
	return Int(int64(n))
}

func cmdTrigger(in *Interp, args string) Value {
	for strings.HasPrefix(args, "-") {
		_, args = firstWord(args)
	}
	in.ProcessLine(args)
	return Int(1)
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
	builtinCmds["list"] = cmdList
	builtinCmds["listvar"] = cmdListvar
	builtinCmds["purge"] = cmdPurge
	builtinCmds["trigger"] = cmdTrigger
	builtinCmds["version"] = cmdVersion
	builtinCmds["gag"] = defShortcut("-ag")
	builtinCmds["hilite"] = defShortcut("-ah")
	builtinCmds["trig"] = defShortcut("")
	builtinCmds["hook"] = defShortcut("-h")
	rawCmds["trigger"] = false

	f := builtinFuncs
	f["tfopen"], f["tfclose"], f["tfwrite"], f["tfread"], f["tfreadable"] = fnTfopen, fnTfclose, fnTfwrite, fnTfread, fnTfreadable
	f["columns"] = func(in *Interp, a []Value) (Value, error) { c, _ := in.termSize(); return Int(int64(c)), nil }
	f["lines"] = func(in *Interp, a []Value) (Value, error) { _, l := in.termSize(); return Int(int64(l)), nil }
	f["tfflush"] = func(in *Interp, a []Value) (Value, error) { return Int(1), nil }
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
	f["filename"] = func(in *Interp, a []Value) (Value, error) {
		p := str(a, 0)
		if strings.HasPrefix(p, "~/") {
			if h, err := os.UserHomeDir(); err == nil {
				p = filepath.Join(h, p[2:])
			}
		}
		return Str(p), nil
	}
	f["pad"] = fnPad
	f["ftime"] = func(in *Interp, a []Value) (Value, error) {
		t := time.Now()
		if len(a) > 1 {
			t = time.Unix(int64(fnum(a, 1)), 0)
		}
		return Str(strftime(str(a, 0), t)), nil
	}
}

// pad(s, n, s, n, ...): each string padded to the width that follows it
// (negative = left-justified).
func fnPad(in *Interp, a []Value) (Value, error) {
	var b strings.Builder
	for i := 0; i+1 < len(a); i += 2 {
		s, w := a[i].String(), int(a[i+1].Int())
		switch {
		case w < 0 && len(s) < -w:
			b.WriteString(s + strings.Repeat(" ", -w-len(s)))
		case w > 0 && len(s) < w:
			b.WriteString(strings.Repeat(" ", w-len(s)) + s)
		default:
			b.WriteString(s)
		}
	}
	if len(a)%2 == 1 {
		b.WriteString(a[len(a)-1].String())
	}
	return Str(b.String()), nil
}

func strftime(f string, t time.Time) string {
	if f == "" {
		f = "%c"
	}
	rep := map[byte]string{'Y': "2006", 'm': "01", 'd': "02", 'H': "15", 'M': "04", 'S': "05", 'y': "06", 'b': "Jan", 'a': "Mon", 'A': "Monday", 'B': "January", 'p': "PM", 'I': "03", 'Z': "MST", 'e': "_2", 'j': "002"}
	var b strings.Builder
	for i := 0; i < len(f); i++ {
		if f[i] == '%' && i+1 < len(f) {
			i++
			if l, ok := rep[f[i]]; ok {
				b.WriteString(t.Format(l))
			} else if f[i] == 'c' {
				b.WriteString(t.Format("Mon Jan _2 15:04:05 2006"))
			} else if f[i] == '%' {
				b.WriteByte('%')
			}
			continue
		}
		b.WriteByte(f[i])
	}
	return b.String()
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
	for _, o := range in.list {
		if o.Bind == seq {
			in.removeFromList(o)
			if o.Name != "" {
				delete(in.macros, o.Name)
			}
			break
		}
	}
	in.Define(m)
	return Int(1)
}

func cmdUnbind(in *Interp, args string) Value {
	seq := strings.TrimSpace(args)
	for _, o := range in.list {
		if o.Bind == seq {
			in.removeFromList(o)
			if o.Name != "" {
				delete(in.macros, o.Name)
			}
			return Int(1)
		}
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
