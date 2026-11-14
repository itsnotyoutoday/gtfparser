package gtfparser

// Per-interpreter state for the data/state builtins (history, limit, logs,
// worlds, watchdog, features), plus the small helpers they share: TF option
// scanning, pattern matching in a /matching style, strftime.
//
// The state hangs off the Interp through ds(): a side table keyed by the
// Interp's address, dropped when the Interp is garbage collected. Nothing here
// is shared between Interps. (If Interp ever grows a field for it, ds() is the
// only function that has to change.)

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"
	"weak"
)

type dataState struct {
	h  histState
	lm limitState
	lg logState
	wd worldDB
	w  watchState
	ft map[string]bool // features declared by the client (see SetFeature)

	owner weak.Pointer[Interp] // guards against an Interp reusing a dead one's address
}

var (
	dsMu  sync.Mutex
	dsTab = map[uintptr]*dataState{}
)

func (in *Interp) ds() *dataState {
	k := uintptr(unsafe.Pointer(in))
	dsMu.Lock()
	defer dsMu.Unlock()
	if d, ok := dsTab[k]; ok && d.owner.Value() == in {
		return d
	}
	// (an entry whose owner was collected but whose cleanup has not run yet is replaced)
	d := &dataState{ft: map[string]bool{}, owner: weak.Make(in)}
	d.h.init()
	d.w.init()
	dsTab[k] = d
	runtime.AddCleanup(in, func(d *dataState) {
		dsMu.Lock()
		if dsTab[k] == d {
			delete(dsTab, k)
		}
		dsMu.Unlock()
	}, d)
	return d
}

// ---------------------------------------------------------------- options

// tfOpt is one option found by scanOpts.
type tfOpt struct {
	c    byte
	arg  string // string argument ("w:" "t:" ...) or the text of a "#" argument
	ival int64  // value of a "#" argument
}

// scanOpts parses leading options the way TF's startopt/nextopt do. spec is
// TF's option string: a letter alone is a flag, "x:" takes a string argument
// (attached, optionally quoted), "x#" takes a numeric expression. Options
// end at a word not starting with '-', at "-"/"--" (consumed), at '=' or the
// end. It returns the options and the remaining text; ok is false after an
// error (already reported).
func (in *Interp) scanOpts(cmd, spec, text string) (opts []tfOpt, rest string, ok bool) {
	i := 0
	inword := false
	cur := byte(0)
	fail := func(f string, a ...any) ([]tfOpt, string, bool) {
		if cur != 0 {
			in.warn("%s -%c: %s", cmd, cur, fmt.Sprintf(f, a...))
		} else {
			in.warn("%s: %s", cmd, fmt.Sprintf(f, a...))
		}
		return nil, "", false
	}
	for {
		cur = 0
		if !inword {
			for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
				i++
			}
			if i >= len(text) || text[i] != '-' {
				return opts, text[i:], true
			}
			i++
			if i < len(text) && text[i] == '-' {
				i++
				if i < len(text) && text[i] != ' ' && text[i] != '\t' {
					return fail("invalid option syntax: --%c", text[i])
				}
			}
			if i >= len(text) || text[i] == ' ' || text[i] == '\t' {
				for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
					i++
				}
				return opts, text[i:], true
			}
		} else if text[i] == '=' {
			return opts, text[i:], true
		}
		c := text[i]
		cur = c
		p := strings.IndexByte(spec, c)
		if p < 0 || c == ':' || c == '#' || c == '@' {
			if c != '?' {
				in.warn("%s -%c: invalid option", cmd, c)
			}
			in.warn("%s: options: %s", cmd, optHelp(spec))
			return nil, "", false
		}
		i++
		o := tfOpt{c: c}
		if p+1 < len(spec) && spec[p+1] == '@' {
			n, ok := parseTFNumber(text[i:])
			if !ok {
				return fail("invalid time")
			}
			o.arg = text[i : len(text)-len(n.rest)]
			i = len(text) - len(n.rest)
		} else if p+1 < len(spec) && (spec[p+1] == ':' || spec[p+1] == '#') {
			numeric := spec[p+1] == '#'
			if i < len(text) && (text[i] == '"' || text[i] == '\'' || text[i] == '`') {
				v, j := parseOptArg(text, i)
				o.arg, i = v, j
			} else {
				j := i
				for j < len(text) && text[j] != ' ' && text[j] != '\t' {
					j++
				}
				o.arg, i = text[i:j], j
			}
			if numeric {
				v, err := in.EvalExpr(o.arg)
				if err != nil {
					return fail("%v", err)
				}
				o.ival = v.Int()
			}
		}
		opts = append(opts, o)
		inword = i < len(text) && text[i] != ' ' && text[i] != '\t'
	}
}

func optHelp(spec string) string {
	var b strings.Builder
	for i := 0; i < len(spec); i++ {
		hasArg := i+1 < len(spec) && (spec[i+1] == ':' || spec[i+1] == '#' || spec[i+1] == '@')
		if i == 0 || spec[i-1] == ':' || spec[i-1] == '#' || spec[i-1] == '@' || hasArg {
			b.WriteString(" -")
		}
		b.WriteByte(spec[i])
		if i+1 < len(spec) && spec[i+1] == ':' {
			b.WriteString("<string>")
			i++
		} else if i+1 < len(spec) && spec[i+1] == '#' {
			b.WriteString("<integer>")
			i++
		} else if i+1 < len(spec) && spec[i+1] == '@' {
			b.WriteString("<time>")
			i++
		}
	}
	return strings.TrimLeft(b.String(), " ")
}

// numArg is TF's numarg(): a non-negative integer at the start of *s; on a
// bad argument it warns and returns -1. Blanks after it are skipped.
func (in *Interp) numArg(cmd string, s *string) int {
	t := *s
	res := -1
	if len(t) > 0 && t[0] >= '0' && t[0] <= '9' {
		j := 0
		for j < len(t) && t[j] >= '0' && t[j] <= '9' {
			j++
		}
		n, err := strconv.Atoi(t[:j])
		if err != nil {
			n = int(^uint(0) >> 1)
		}
		res, t = n, t[j:]
	} else {
		in.warn("%s: invalid or missing numeric argument", cmd)
		j := 0
		for j < len(t) && t[j] != ' ' && t[j] != '\t' {
			j++
		}
		t = t[j:]
	}
	*s = strings.TrimLeft(t, " \t")
	return res
}

// ---------------------------------------------------------------- patterns

// matchStyle returns a matcher for pat in the given /matching style
// (simple, glob, regexp, substr; "" = the %matching variable).
func (in *Interp) matchStyle(cmd, style, pat string) (func(string) bool, bool) {
	if style == "" {
		style = in.globals["matching"].String()
	}
	switch style {
	case "simple":
		return func(s string) bool { return s == pat }, true
	case "substr":
		return func(s string) bool { return strings.Contains(s, pat) }, true
	case "regexp":
		re, err := in.compileRe(pat)
		if err != nil {
			in.warn("%s: %v", cmd, err)
			return nil, false
		}
		return func(s string) bool { return re.match(s, 0) != nil }, true
	}
	return func(s string) bool { return globMatch(pat, s) }, true
}

// matchStyleName validates an -m argument (a name or its number).
func (in *Interp) matchStyleName(cmd string, arg string) (string, bool) {
	names := []string{"simple", "glob", "regexp", "substr"}
	for i, n := range names {
		if strings.EqualFold(n, arg) || arg == strconv.Itoa(i) {
			return n, true
		}
	}
	in.warn(`%s -m: Invalid -m value "%s". Valid values are: simple (0), glob (1), regexp (2), substr (3)`, cmd, arg)
	return "", false
}

// ---------------------------------------------------------------- time

// tfStrftime formats t like TF's tftime: %@ (seconds.micro), %. (micro),
// %s, %F, %T plus the usual C strftime conversions.
func tfStrftime(f string, t time.Time) string {
	if f == "" {
		f = "%c"
	}
	var b strings.Builder
	for i := 0; i < len(f); i++ {
		if f[i] != '%' || i+1 >= len(f) {
			b.WriteByte(f[i])
			continue
		}
		i++
		switch c := f[i]; c {
		case '@':
			fmt.Fprintf(&b, "%d.%06d", t.Unix(), t.Nanosecond()/1000)
		case '.':
			fmt.Fprintf(&b, "%06d", t.Nanosecond()/1000)
		case 's':
			fmt.Fprintf(&b, "%d", t.Unix())
		case 'F':
			b.WriteString(t.Format("2006-01-02"))
		case 'T':
			b.WriteString(t.Format("15:04:05"))
		case 'a':
			b.WriteString(t.Format("Mon"))
		case 'A':
			b.WriteString(t.Format("Monday"))
		case 'b', 'h':
			b.WriteString(t.Format("Jan"))
		case 'B':
			b.WriteString(t.Format("January"))
		case 'c':
			b.WriteString(t.Format("Mon Jan _2 15:04:05 2006"))
		case 'C':
			fmt.Fprintf(&b, "%02d", t.Year()/100)
		case 'd':
			b.WriteString(t.Format("02"))
		case 'D':
			b.WriteString(t.Format("01/02/06"))
		case 'e':
			b.WriteString(t.Format("_2"))
		case 'H':
			b.WriteString(t.Format("15"))
		case 'I':
			b.WriteString(t.Format("03"))
		case 'j':
			fmt.Fprintf(&b, "%03d", t.YearDay())
		case 'k':
			fmt.Fprintf(&b, "%2d", t.Hour())
		case 'l':
			h := t.Hour() % 12
			if h == 0 {
				h = 12
			}
			fmt.Fprintf(&b, "%2d", h)
		case 'm':
			b.WriteString(t.Format("01"))
		case 'M':
			b.WriteString(t.Format("04"))
		case 'n':
			b.WriteByte('\n')
		case 'p':
			b.WriteString(t.Format("PM"))
		case 'P':
			b.WriteString(strings.ToLower(t.Format("PM")))
		case 'r':
			b.WriteString(t.Format("03:04:05 PM"))
		case 'R':
			b.WriteString(t.Format("15:04"))
		case 'S':
			b.WriteString(t.Format("05"))
		case 't':
			b.WriteByte('\t')
		case 'u':
			w := int(t.Weekday())
			if w == 0 {
				w = 7
			}
			fmt.Fprintf(&b, "%d", w)
		case 'w':
			fmt.Fprintf(&b, "%d", int(t.Weekday()))
		case 'x':
			b.WriteString(t.Format("01/02/06"))
		case 'X':
			b.WriteString(t.Format("15:04:05"))
		case 'y':
			b.WriteString(t.Format("06"))
		case 'Y':
			b.WriteString(t.Format("2006"))
		case 'z':
			b.WriteString(t.Format("-0700"))
		case 'Z':
			b.WriteString(t.Format("MST"))
		case '%':
			b.WriteByte('%')
		default:
			b.WriteByte('%')
			b.WriteByte(c)
		}
	}
	return b.String()
}

// ---------------------------------------------------------------- features

// The features real tf reports, in its order.
var featureNames = []string{"256colors", "core", "float", "ftime", "history", "IPv6", "locale", "MCCPv1", "MCCPv2", "process", "SOCKS", "ssl", "subsecond", "TZ"}

// builtinFeatures are the ones the core always provides.
var builtinFeatures = map[string]bool{"float": true, "ftime": true, "history": true, "locale": true, "process": true, "subsecond": true, "TZ": true}

// SetFeature declares an optional capability the client implements (for
// example "256colors", "IPv6", "MCCPv1", "MCCPv2", "ssl", "SOCKS", "core").
// /features reports them; a feature the client never declares is off.
func (in *Interp) SetFeature(name string, on bool) { in.ds().ft[name] = on }

// Feature reports whether the named feature is available.
func (in *Interp) Feature(name string) bool {
	if on, ok := in.ds().ft[name]; ok {
		return on
	}
	return builtinFeatures[name]
}

// FeatureString is what /features prints: "+float -ssl ...".
func (in *Interp) FeatureString() string {
	var b strings.Builder
	for i, n := range featureNames {
		if i > 0 {
			b.WriteByte(' ')
		}
		if in.Feature(n) {
			b.WriteByte('+')
		} else {
			b.WriteByte('-')
		}
		b.WriteString(n)
	}
	return b.String()
}

func cmdFeatures(in *Interp, args string) Value {
	args = strings.TrimSpace(args)
	if args == "" {
		in.echo(in.FeatureString())
		return Int(1)
	}
	for _, n := range featureNames {
		if strings.EqualFold(n, args) {
			return Bool(in.Feature(n))
		}
	}
	return Int(0)
}

// ---------------------------------------------------------------- /core, /reboot

// /core is TF's debugging aid (dump core). Here it raises a "core" event; a
// client that wants a dump (or a panic) implements it, otherwise it is
// refused loudly.
func cmdCore(in *Interp, args string) Value {
	if _, ok := in.Host.(Notifier); !ok {
		return in.unsupported("/core")
	}
	in.notify("core", strings.TrimSpace(args))
	return Int(1)
}

func cmdReboot(in *Interp, args string) Value {
	opts, rest, ok := in.scanOpts("/reboot", "y", args)
	if !ok {
		return Int(0)
	}
	force := len(opts) > 0
	if strings.TrimSpace(rest) != "" {
		in.warn("/reboot: unexpected argument: %s", strings.TrimSpace(rest))
		return Int(0)
	}
	rh, ok := in.Host.(RebootHost)
	if !ok {
		return in.unsupported("/reboot")
	}
	f := "0"
	if force {
		f = "1"
	}
	in.notify("reboot", f)
	if err := rh.Reboot(force); err != nil {
		in.warn("/reboot: %v", err)
		return Int(0)
	}
	return Int(1)
}

// ---------------------------------------------------------------- /help

func cmdHelp(in *Interp, args string) Value {
	h, ok := in.Host.(HelpHost)
	if !ok {
		return in.unsupported("/help")
	}
	topic := strings.TrimSpace(args)
	if topic == "" {
		topic = "summary"
	}
	h.Help(topic)
	return Int(1)
}

// ---------------------------------------------------------------- watchdog / watchname

type watchState struct {
	nmatch, nlines [2]int // [0]=watchname [1]=watchdog
}

func (w *watchState) init() {
	w.nmatch = [2]int{4, 2}
	w.nlines = [2]int{5, 5}
}

const (
	wName = 0
	wDog  = 1
)

func cmdWatch(which int, name string) cmdFunc {
	return func(in *Interp, args string) Value {
		w := &in.ds().w
		args = strings.TrimSpace(args)
		switch {
		case args == "":
			st := "dis"
			if in.globals[name].Truth() {
				st = "en"
			}
			in.echo(fmt.Sprintf("%% %s %sabled.", name, st))
			return Int(1)
		case strings.EqualFold(args, "off"):
			in.setGlobal(name, Str("off"))
			in.echo(fmt.Sprintf("%% %s disabled.", name))
			return Int(1)
		case strings.EqualFold(args, "on"):
		default:
			match := in.numArg("/"+name, &args)
			if match < 0 {
				return Int(0)
			}
			of := in.numArg("/"+name, &args)
			if of < 0 {
				return Int(0)
			}
			w.nmatch[which], w.nlines[which] = match, of
		}
		in.setGlobal(name, Str("on"))
		in.echo(fmt.Sprintf("%% %s enabled, searching for %d out of %d lines", name, w.nmatch[which], w.nlines[which]))
		in.notify(name, "on", strconv.Itoa(w.nmatch[which]), strconv.Itoa(w.nlines[which]))
		return Int(1)
	}
}

// CheckWatch is what a client calls for each line received from a world,
// before the line is recorded (TF's is_watchdog/is_watchname). It returns
// true when the line should be gagged: %watchdog suppresses a line repeated
// too often, and %watchname defines a gag macro for a name that keeps
// starting lines.
func (in *Interp) CheckWatch(world, line string) (gag bool) {
	if !in.globals["gag"].Truth() {
		return false
	}
	h := in.worldHistory(world)
	if h == nil {
		return false
	}
	w := &in.ds().w
	size := len(h.lines)
	prior := func(i int) string { return h.lines[size-1-i].Text }
	if in.globals["watchdog"].Truth() {
		n := w.nlines[wDog]
		if n >= size {
			n = size - 1
		}
		hits := 0
		for i := n; i > 0; i-- {
			if prior(i) == line {
				hits++
				if hits == w.nmatch[wDog] {
					return true
				}
			}
		}
	}
	if in.globals["watchname"].Truth() && line != "" && line[0] != ' ' && line[0] != '\t' {
		end := strings.IndexAny(line, " \t")
		if end < 0 {
			end = len(line)
		}
		n := w.nlines[wName]
		if n >= size {
			n = size - 1
		}
		hits := 1
		for i := n; i > 0; i-- {
			old := prior(i)
			if len(old) >= end && old[:end] == line[:end] {
				hits++
				if hits == w.nmatch[wName] {
					break
				}
			}
		}
		if hits >= w.nmatch[wName] {
			pat := "{" + line[:end] + "}*"
			in.echo(fmt.Sprintf("%% Watchname: gagging \"%s\"", pat))
			in.Define(&Macro{Matching: "glob", Trigger: pat, Priority: int(in.globals["gpri"].Int()), Attr: AttrGag})
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------- misc helpers

func sortedKeys[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func (in *Interp) expandTilde(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if h, err := os.UserHomeDir(); err == nil {
			return h + p[1:]
		}
	}
	return p
}

func init() {
	builtinCmds["features"] = cmdFeatures
	builtinCmds["core"] = cmdCore
	builtinCmds["reboot"] = cmdReboot
	builtinCmds["help"] = cmdHelp
	builtinCmds["watchname"] = cmdWatch(wName, "watchname")
	builtinCmds["watchdog"] = cmdWatch(wDog, "watchdog")
}
