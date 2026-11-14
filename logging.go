package gtfparser

// Logging: /log, nlog() and the log writer. A log is attached to a history
// (input, local, global, or one world's); a line recorded in that history is
// also appended to its log file unless it carries the "L" attribute.

import (
	"fmt"
	"os"
	"strings"
)

type logFile struct {
	name string
	f    *os.File
}

type logState struct{ n int } // number of open logs (nlog())

// LogCount is the number of open log files.
func (in *Interp) LogCount() int { return in.ds().lg.n }

func (in *Interp) closeLog(h *history) {
	if h != nil && h.log != nil {
		h.log.f.Close()
		h.log = nil
		in.ds().lg.n--
		in.notify("log", "off")
	}
}

// LogLine appends a line to the log of a history without recording it there:
// world "" means the global log; for other lines use RecordWorldLine, which
// logs as well as records. Returns false when no log is open for it.
func (in *Interp) LogLine(world, line string) bool {
	var h *history
	if world == "" {
		h = in.ds().h.global
	} else {
		h = in.worldHistory(world)
	}
	if h == nil || h.log == nil {
		return false
	}
	in.writeLog(h.log, HistLine{Text: line, Time: in.now()})
	return true
}

// writeLog formats one line (prefix, optional ANSI attributes, optional
// wrapping) and appends it.
func (in *Interp) writeLog(lf *logFile, l HistLine) {
	if l.Time.IsZero() {
		l.Time = in.now()
	}
	prefix := in.globals["log_prefix"].String()
	var b strings.Builder
	for i := 0; i < len(prefix); i++ {
		if prefix[i] != '%' || i+1 >= len(prefix) {
			b.WriteByte(prefix[i])
			continue
		}
		i++
		if prefix[i] == 't' {
			b.WriteString(tfStrftime(in.globals["log_time_format"].String(), l.Time))
		} else {
			b.WriteByte('%')
			b.WriteByte(prefix[i])
		}
	}
	text := l.Text
	if in.globals["ansi_log"].Truth() {
		text = encodeAnsiAttrs(text, l.Attrs)
	}
	b.WriteString(text)
	full := b.String()
	if !in.globals["wraplog"].Truth() {
		fmt.Fprintln(lf.f, full)
		return
	}
	width := int(in.globals["wrapsize"].Int())
	if width <= 0 {
		width, _ = in.termSize()
	}
	space := int(in.globals["wrapspace"].Int())
	wrap := in.globals["wrap"].Truth()
	first := true
	for rest := full; ; first = false {
		indent := 0
		if !first && wrap && space < width {
			indent = space
		}
		n := wrapLen(rest, width-indent, int(in.globals["tabsize"].Int()))
		if indent > 0 {
			fmt.Fprint(lf.f, strings.Repeat(" ", indent))
		}
		fmt.Fprintln(lf.f, rest[:n])
		rest = rest[n:]
		if rest == "" {
			break
		}
	}
}

// wrapLen is how many bytes of s fit in max columns, breaking after a blank
// when it can.
func wrapLen(s string, max, tab int) int {
	if max < 1 {
		max = 1
	}
	if tab <= 0 {
		tab = 8
	}
	col := 0
	lastBlank := -1
	for i, r := range s {
		w := 1
		if r == '\t' {
			w = tab - col%tab
		}
		if col+w > max {
			if lastBlank > 0 {
				return lastBlank
			}
			if i == 0 {
				return len(string(r))
			}
			return i
		}
		col += w
		if r == ' ' || r == '\t' {
			lastBlank = i + 1
		}
	}
	return len(s)
}

// encodeAnsiAttrs wraps text in the SGR sequences for a line's attributes.
func encodeAnsiAttrs(text, attrs string) string {
	var codes []string
	for i := 0; i < len(attrs); i++ {
		switch attrs[i] {
		case 'B', 'h':
			codes = append(codes, "1")
		case 'd':
			codes = append(codes, "2")
		case 'i':
			codes = append(codes, "3")
		case 'u':
			codes = append(codes, "4")
		case 'f':
			codes = append(codes, "5")
		case 'r':
			codes = append(codes, "7")
		case 'C':
			j := i + 1
			for j < len(attrs) && attrs[j] != ',' {
				j++
			}
			name := attrs[i+1 : j]
			base := 30
			if strings.HasPrefix(name, "bg") {
				name, base = name[2:], 40
			}
			for k, c := range histColorNames {
				if c == name && k < 8 {
					codes = append(codes, fmt.Sprint(base+k))
				} else if c == name {
					codes = append(codes, fmt.Sprint(base+60+k-8))
				}
			}
			i = j
		}
	}
	if len(codes) == 0 {
		return text
	}
	return "\x1b[" + strings.Join(codes, ";") + "m" + text + "\x1b[0m"
}

// tfname is TF's tfname(): use name, else the body of the macro called def.
func (in *Interp) tfname(who, name, def string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		if m, ok := in.macros[def]; ok && m.Body != "" {
			name = m.Body
		} else {
			in.warn("%s: missing filename, and default macro %s is not defined", who, def)
			return "", false
		}
	}
	return in.expandTilde(name), true
}

func cmdLog(in *Interp, args string) Value {
	if in.Restriction() >= RestrictFile {
		in.warn("/log: restricted")
		return Int(0)
	}
	opts, rest, ok := in.scanOpts("/log", "lgiw:", args)
	if !ok {
		return Int(0)
	}
	d := in.ds()
	h, _, ok := in.pickHistory("/log", opts, nil)
	if !ok {
		return Int(0)
	}
	explicit := len(opts) > 0
	rest = strings.TrimSpace(rest)
	logged := func() []struct {
		label string
		lf    *logFile
	} {
		var out []struct {
			label string
			lf    *logFile
		}
		add := func(label string, hh *history) {
			if hh.log != nil {
				out = append(out, struct {
					label string
					lf    *logFile
				}{label, hh.log})
			}
		}
		add("Logging input to %s", d.h.input)
		add("Logging local output to %s", d.h.local)
		add("Logging global output to %s", d.h.global)
		for _, w := range d.wd.worlds {
			if w.hist != nil && w.hist.log != nil {
				out = append(out, struct {
					label string
					lf    *logFile
				}{"Logging world " + w.Name + " output to %s", w.hist.log})
			}
		}
		return out
	}
	switch {
	case !explicit && rest == "":
		if d.lg.n == 0 {
			in.echo("% Logging disabled.")
			return Int(1)
		}
		for _, e := range logged() {
			in.echo("% " + fmt.Sprintf(e.label, e.lf.name))
		}
		return Int(1)
	case strings.EqualFold(rest, "OFF"):
		if h == nil {
			if d.lg.n > 0 {
				in.closeLog(d.h.input)
				in.closeLog(d.h.local)
				in.closeLog(d.h.global)
				for _, w := range d.wd.worlds {
					in.closeLog(w.hist)
				}
			}
		} else {
			in.closeLog(h)
		}
		return Int(1)
	}
	var name string
	if strings.EqualFold(rest, "ON") || rest == "" {
		n, ok := in.tfname("/log", "", "LOGFILE")
		if !ok {
			return Int(0)
		}
		name = n
	} else {
		name = in.expandTilde(rest)
	}
	if !in.allow(OpFileWrite, "/log", name) {
		return Int(0)
	}
	f, err := os.OpenFile(name, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		in.warn("/log: %s: %v", name, errText(err))
		return Int(0)
	}
	if h == nil {
		h = d.h.global
	}
	in.closeLog(h)
	in.hookMessage("LOG", "Logging to file "+name, name)
	h.log = &logFile{name: name, f: f}
	d.lg.n++
	in.notify("log", "on", name)
	return Int(1)
}

// errText gives an OS error the way strerror does (no path prefix).
func errText(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		err = pe.Err
	}
	s := err.Error()
	if len(s) > 0 {
		s = strings.ToUpper(s[:1]) + s[1:]
	}
	return s
}

// hookMessage prints the message TF's do_hook would, then runs the macros
// hooked on event with args.
func (in *Interp) hookMessage(event, msg, args string) {
	in.warn("%s", msg)
	in.Hook(event, args)
}

func fnNlog(in *Interp, a []Value) (Value, error) { return Int(int64(in.ds().lg.n)), nil }

func init() {
	builtinCmds["log"] = cmdLog
	builtinFuncs["nlog"] = fnNlog
}
