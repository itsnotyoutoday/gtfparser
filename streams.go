package gtfparser

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// ---------------------------------------------------------------- tfio
//
// tfopen() and friends as tfio.c implements them: numbered user streams
// (files r/w/a, queues q, pipes p/P) and the letter handles i, o, e and a.

type stream struct {
	f         *os.File // file or pipe (reading end); nil for a queue
	r         *bufio.Reader
	w         io.Writer // where tfwrite goes (files, P pipes)
	canRead   bool
	canWrite  bool
	queue     bool
	lines     []string // queue contents
	cmd       *exec.Cmd
	mode      byte
	name      string
	id        string
	autoflush bool
	eof       bool
	origin    string // the origin that opened it (see origin.go)
}

func (in *Interp) stream(id string) *stream {
	if in.streams == nil {
		return nil
	}
	return in.streams[id]
}

// cErr renders an error the way strerror(3) does.
func cErr(err error) string {
	var en syscall.Errno
	msg := err.Error()
	if errors.As(err, &en) {
		msg = en.Error()
	} else if pe, ok := err.(*os.PathError); ok {
		msg = pe.Err.Error()
	}
	if msg != "" {
		msg = strings.ToUpper(msg[:1]) + msg[1:]
	}
	return msg
}

func (in *Interp) openStream(name, mode string) (*stream, error) {
	st := &stream{mode: mode[0], name: name, autoflush: true}
	switch mode[0] {
	case 'q':
		st.queue, st.canRead, st.canWrite = true, true, true
		return st, nil
	case 'p', 'P':
		if !in.allow(OpShell, "tfopen()", name) {
			return nil, syscall.EPERM
		}
		cmd := exec.Command("sh", "-c", name)
		cmd.Env = in.childEnv()
		out, err := cmd.StdoutPipe()
		if err != nil {
			return nil, err
		}
		if mode[0] == 'P' {
			w, err := cmd.StdinPipe()
			if err != nil {
				return nil, err
			}
			st.w, st.canWrite = w, true
		}
		if err := cmd.Start(); err != nil {
			return nil, err
		}
		st.cmd, st.canRead = cmd, true
		st.r = bufio.NewReader(out)
		return st, nil
	}
	op := OpFileRead
	if mode[0] != 'r' {
		op = OpFileWrite
	}
	if !in.allow(op, "tfopen()", name) {
		return nil, syscall.EPERM
	}
	var f *os.File
	var err error
	switch mode[0] {
	case 'w':
		f, err = os.Create(name)
	case 'a':
		f, err = os.OpenFile(name, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o666)
	default:
		f, err = os.Open(name)
	}
	if err != nil {
		return nil, err
	}
	if fi, e := f.Stat(); e == nil && fi.IsDir() {
		f.Close()
		return nil, syscall.EISDIR
	}
	st.f = f
	if mode[0] == 'r' {
		st.canRead = true
		st.r = bufio.NewReader(f)
	} else {
		st.canWrite, st.w = true, f
	}
	return st, nil
}

// tfopen([name,] mode): note that a single argument is the mode; with no
// arguments the mode is "q", an unnamed queue.
func fnTfopen(in *Interp, a []Value) (Value, error) {
	name, mode := "", "q"
	switch len(a) {
	case 1:
		mode = a[0].String()
	case 2:
		name, mode = a[0].String(), a[1].String()
	}
	if len(mode) != 1 || !strings.Contains("rwaPpq", mode) {
		in.warn("invalid mode '%s'", mode)
		return Int(-1), nil
	}
	if in.EffectiveRestriction() >= 2 && mode != "q" { // RESTRICT_FILE
		in.warn("restricted")
		return Int(-1), nil
	}
	name = in.expandFilename(name)
	if name == "" && mode != "q" {
		in.warn("%s: %s", name, cErr(syscall.ENOENT))
		return Int(-1), nil
	}
	st, err := in.openStream(name, mode)
	if err != nil {
		in.warn("%s: %s", name, cErr(err))
		return Int(-1), nil
	}
	if in.streams == nil {
		in.streams = map[string]*stream{}
	}
	st.origin = in.CurrentOrigin()
	in.nextFd++
	st.id = fmt.Sprint(in.nextFd)
	in.streams[st.id] = st
	return Int(int64(in.nextFd)), nil
}

// findStream is find_tfile(): letters name the standard streams, numbers the
// user's. ok is false (with a warning) for a bad handle.
func (in *Interp) findStream(h string) (st *stream, std byte, ok bool) {
	if len(h) == 1 && (h[0]|0x20 >= 'a' && h[0]|0x20 <= 'z') {
		switch c := h[0] | 0x20; c {
		case 'i', 'o', 'e', 'a':
			return nil, c, true
		}
	} else if s := in.stream(fmt.Sprint(cAtoi(h))); s != nil {
		if !in.streamAllowed(s) {
			in.warn("restricted")
			return nil, 0, false
		}
		return s, 0, true
	}
	in.warn("%s: bad handle", h)
	return nil, 0, false
}

func fnTfclose(in *Interp, a []Value) (Value, error) {
	h := a[0].String()
	if len(h) == 1 {
		switch h[0] | 0x20 {
		case 'i':
			return Int(0), nil
		case 'o':
			in.fr.noOut = true
			return Int(0), nil
		case 'e':
			in.warn("tferr can not be closed.")
			return Int(-1), nil
		}
	}
	st, std, ok := in.findStream(h)
	if !ok || std != 0 {
		return Int(-1), nil
	}
	res := int64(0)
	if st.cmd != nil {
		if c, ok := st.w.(io.Closer); ok {
			c.Close()
		}
		if err := st.cmd.Wait(); err != nil {
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				res = int64(ee.ExitCode())
			} else {
				res = -1
			}
		}
	} else if st.f != nil {
		if err := st.f.Close(); err != nil {
			res = -1
		}
	}
	delete(in.streams, st.id)
	return Int(res), nil
}

// usable is find_usable_tfile(): the stream must allow the access wanted.
func (in *Interp) usable(h string, write bool) (st *stream, std byte, ok bool) {
	st, std, ok = in.findStream(h)
	if !ok {
		return nil, 0, false
	}
	bad := false
	switch {
	case std != 0:
		bad = (write && std == 'i') || (!write && std != 'i')
	case write:
		bad = !st.canWrite
	default:
		bad = !st.canRead
	}
	if bad {
		what := "read"
		if write {
			what = "writ"
		}
		in.warn("stream %s is not %sable", h, what)
		return nil, 0, false
	}
	return st, std, true
}

func fnTfwrite(in *Interp, a []Value) (Value, error) {
	text := a[len(a)-1].String()
	if len(a) > 1 {
		st, std, ok := in.usable(a[0].String(), true)
		if !ok {
			return Int(-1), nil
		}
		if std == 0 {
			return in.streamWrite(st, text), nil
		}
	}
	in.echo(text) // tfout, tferr and the alert stream all show on screen
	return Int(1), nil
}

func (in *Interp) streamWrite(st *stream, text string) Value {
	if st.queue {
		st.lines = append(st.lines, text)
		return Int(1)
	}
	if st.w == nil {
		return Int(-1)
	}
	// embedded newlines become spaces, as in tfio.c's fileputs()
	io.WriteString(st.w, strings.ReplaceAll(text, "\n", " ")+"\n")
	return Int(1)
}

func (in *Interp) tabsize() int {
	if n := int(in.globals["tabsize"].Int()); n > 0 {
		return n
	}
	return 8
}

// readLine is tfgetS(): a line without its newline, tabs expanded.
func (in *Interp) readLine(st *stream) (string, bool) {
	if st.queue {
		if len(st.lines) == 0 {
			return "", false
		}
		l := st.lines[0]
		st.lines = st.lines[1:]
		return l, true
	}
	if st.eof || st.r == nil {
		return "", false
	}
	line, err := st.r.ReadString('\n')
	if err != nil {
		st.eof = true
		if line == "" {
			return "", false
		}
	}
	line = strings.TrimSuffix(line, "\n")
	if strings.IndexByte(line, '\t') >= 0 {
		var b strings.Builder
		ts := in.tabsize()
		for i := 0; i < len(line); i++ {
			if line[i] == '\t' {
				b.WriteString(strings.Repeat(" ", ts-b.Len()%ts))
			} else {
				b.WriteByte(line[i])
			}
		}
		line = b.String()
	}
	return line, true
}

func fnTfread(in *Interp, a []Value) (Value, error) {
	last := a[len(a)-1].String()
	var line string
	var ok bool
	if len(a) > 1 {
		st, std, found := in.usable(a[0].String(), false)
		if !found {
			return Int(-1), nil
		}
		if std == 'i' {
			in.warn("keyboard can only be read from a command line command.")
			return Int(-1), nil
		}
		if std != 0 {
			line, ok = in.readTfin()
		} else {
			line, ok = in.readLine(st)
		}
	} else {
		line, ok = in.readTfin()
	}
	if !ok {
		return Int(-1), nil
	}
	in.assign(last, Str(line))
	return Int(int64(len(line))), nil
}

// readTfin takes the next line of the input stream: the output of the
// command on the left of a %| pipe.
func (in *Interp) readTfin() (string, bool) {
	if len(in.tfin) == 0 {
		return "", false
	}
	l := in.tfin[0]
	in.tfin = in.tfin[1:]
	return l, true
}

// tfreadable() is 1 when tfread() would not block, and also when the handle
// is bad or not readable (tfread fails at once then); 0 for the keyboard and
// for a pipe with nothing to read yet.
func fnTfreadable(in *Interp, a []Value) (Value, error) {
	st, std, ok := in.usable(a[0].String(), false)
	if !ok {
		return Int(1), nil
	}
	if std == 'i' {
		return Int(0), nil
	}
	if st.cmd != nil && !st.eof && st.r.Buffered() == 0 {
		return Int(0), nil
	}
	return Int(1), nil
}

func fnTfflush(in *Interp, a []Value) (Value, error) {
	st, std, ok := in.usable(a[0].String(), true)
	if !ok {
		return Int(-1), nil
	}
	if len(a) > 1 {
		on, valid := in.flagArg(a[1].String(), "argument 2")
		if !valid {
			return Int(0), nil
		}
		if std == 0 {
			st.autoflush = on
		}
	}
	return Int(1), nil
}

func init() {
	f := builtinFuncs
	f["tfopen"], f["tfclose"], f["tfwrite"], f["tfread"], f["tfreadable"], f["tfflush"] = fnTfopen, fnTfclose, fnTfwrite, fnTfread, fnTfreadable, fnTfflush
}
