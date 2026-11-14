package gtfparser

import (
	"fmt"
	"sort"
	"strings"
)

// Macro is a /def'd macro: optionally a trigger, hook or key binding.
type Macro struct {
	ID        int
	Name      string
	Body      string
	Trigger   string
	Hook      string
	Bind      string
	Matching  string // simple, glob, regexp
	Shots     int
	Priority  int
	Fall      bool // -F fall-through
	Gag       bool
	Cond      string // -E
	Invisible bool   // -i
	OnlyInvis bool   // -I (list filter)
	Quiet     bool   // -q
	WorldType string // -T
	hasBody   bool
	set       map[byte]string // options given on the /def or /list line
	Attrs     string
	World     string
	re        *rx
}

// rawCmds: commands whose arguments skip substitution (none: only top-level
// lines are unsubstituted, and that is handled by Exec).
var rawCmds = map[string]bool{}

// splitOptWord pulls the next /def option word: -ttext, -t'quoted text',
// -h"event pattern".
func splitOptWord(s string) (word, rest string) {
	i := 0
	for i < len(s) && s[i] != ' ' && s[i] != '\t' {
		if (s[i] == '"' || s[i] == '\'' || s[i] == '`') && i > 1 {
			q := s[i]
			i++
			for i < len(s) && s[i] != q {
				if s[i] == '\\' {
					i++
				}
				i++
			}
		}
		i++
	}
	if i > len(s) {
		i = len(s)
	}
	return s[:i], strings.TrimLeft(s[i:], " \t")
}

func unquoteOpt(v string) string {
	if len(v) >= 2 && (v[0] == '"' || v[0] == '\'' || v[0] == '`') && v[len(v)-1] == v[0] {
		q := v[0]
		inner := v[1 : len(v)-1]
		var b strings.Builder
		for i := 0; i < len(inner); i++ {
			if inner[i] == '\\' && i+1 < len(inner) && (inner[i+1] == q || inner[i+1] == '\\') {
				i++
			}
			b.WriteByte(inner[i])
		}
		return b.String()
	}
	return v
}

// defFlags take no argument; defArgs take the rest of their word.
const (
	defFlags = "FfiIq"
	defArgs  = "mnEthbBpcwTaP"
)

func (in *Interp) parseDef(args string) (*Macro, error) {
	m := &Macro{Matching: in.globals["matching"].String(), Priority: 1, set: map[byte]string{}}
	rest := strings.TrimLeft(args, " \t")
	for strings.HasPrefix(rest, "-") && len(rest) > 1 {
		var w string
		w, rest = splitOptWord(rest)
		if w == "--" {
			break
		}
		for j := 1; j < len(w); j++ {
			c := w[j]
			if strings.IndexByte(defFlags, c) >= 0 {
				m.set[c] = ""
				switch c {
				case 'i':
					m.Invisible = true
				case 'F':
					m.Fall = true
				case 'I':
					m.OnlyInvis = true
				case 'q':
					m.Quiet = true
				}
				continue
			}
			if strings.IndexByte(defArgs, c) < 0 {
				return nil, fmt.Errorf("/def: invalid option '%c'", c)
			}
			val := unquoteOpt(w[j+1:])
			m.set[c] = val
			switch c {
			case 'm':
				m.Matching = val
			case 'n':
				m.Shots = int(Str(val).Int())
			case 'p':
				if v, err := in.evalExpr(val); err == nil {
					m.Priority = int(v.Int())
				}
			case 'c':
			case 'E':
				m.Cond = val
			case 't':
				m.Trigger = val
			case 'h':
				m.Hook = val
			case 'b':
				m.Bind = val
			case 'B':
			case 'T':
				m.WorldType = val
			case 'w':
				m.World = val
			case 'a':
				m.Attrs = val
			case 'P':
				m.Attrs += "P" + val
				m.Matching = "regexp"
			}
			break
		}
	}
	if m.Matching == "" {
		m.Matching = "glob"
	}
	// [name] [= body]
	name := ""
	if !strings.HasPrefix(rest, "=") {
		i := 0
		for i < len(rest) && rest[i] != ' ' && rest[i] != '\t' && rest[i] != '=' {
			i++
		}
		name, rest = rest[:i], strings.TrimLeft(rest[i:], " \t")
	}
	if strings.HasPrefix(rest, "=") {
		m.Body = strings.TrimLeft(rest[1:], " \t")
		m.hasBody = true
	}
	m.Name = name
	return m, nil
}

// Define installs a macro, replacing one of the same name.
func (in *Interp) Define(m *Macro) {
	if m.Name != "" {
		if old, ok := in.macros[m.Name]; ok {
			in.removeFromList(old)
		}
		in.macros[m.Name] = m
	}
	in.nextID++
	m.ID = in.nextID
	if m.Matching == "regexp" && m.Trigger != "" {
		var err error
		if m.re, err = compileRegexp(m.Trigger); err != nil {
			in.warn("DEF -t: %v", err)
		}
	}
	in.list = append(in.list, m)
}

func (in *Interp) removeFromList(m *Macro) {
	for i, x := range in.list {
		if x == m {
			in.list = append(in.list[:i], in.list[i+1:]...)
			return
		}
	}
}

func (in *Interp) Undef(name string) bool {
	m, ok := in.macros[name]
	if !ok {
		return false
	}
	delete(in.macros, name)
	in.removeFromList(m)
	return true
}

// Macros lists every defined macro in definition order.
func (in *Interp) Macros() []*Macro {
	out := append([]*Macro(nil), in.list...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (m *Macro) matches(line string) (ok bool, regs []string, left, right string) {
	switch m.Matching {
	case "regexp":
		if m.re == nil {
			return false, nil, "", ""
		}
		loc := m.re.match(line, 0)
		if loc == nil {
			return false, nil, "", ""
		}
		for i := 0; i+1 < len(loc); i += 2 {
			if loc[i] < 0 {
				regs = append(regs, "")
			} else {
				regs = append(regs, line[loc[i]:loc[i+1]])
			}
		}
		return true, regs, line[:loc[0]], line[loc[1]:]
	case "simple":
		return line == m.Trigger, nil, "", ""
	case "substr":
		return strings.Contains(line, m.Trigger), nil, "", ""
	}
	return globMatch(m.Trigger, line), nil, "", ""
}

// ProcessLine runs the triggers matching a line received from a world. It
// reports whether the line was gagged.
func (in *Interp) ProcessLine(line string) (gag bool) {
	var hits []*Macro
	for _, m := range in.list {
		if m.Trigger == "" {
			continue
		}
		if m.Cond != "" && !in.condition(m.Cond) {
			continue
		}
		hits = append(hits, m)
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Priority > hits[j].Priority })
	topPri := 0
	first := true
	for _, m := range hits {
		if !first && m.Priority < topPri {
			break
		}
		ok, regs, l, r := m.matches(line)
		if !ok {
			continue
		}
		if first {
			first, topPri = false, m.Priority
		}
		if m.Gag {
			gag = true
		}
		in.runTrigger(m, line, regs, l, r)
		if m.Shots > 0 {
			m.Shots--
			if m.Shots == 0 {
				in.Undef(m.Name)
				in.removeFromList(m)
			}
		}
		if !m.Fall {
			break
		}
	}
	return gag
}

func (in *Interp) runTrigger(m *Macro, line string, regs []string, l, r string) {
	in.depth++
	f := in.pushFrame(m.Name, line)
	if regs != nil {
		f.regs, f.pl, f.pr = regs, l, r
	}
	oldExp := in.noExp
	in.noExp = false
	in.execText(m.Body)
	in.noExp = oldExp
	in.ctl = ctlNone
	in.popFrame()
	in.depth--
}

// Hook fires macros hooked on an event.
func (in *Interp) Hook(event, args string) {
	for _, m := range append([]*Macro(nil), in.list...) {
		if m.Hook == "" {
			continue
		}
		ev, pat := firstWord(m.Hook)
		match := false
		for _, e := range strings.Split(ev, "|") {
			if strings.EqualFold(e, event) {
				match = true
			}
		}
		if !match || (pat != "" && !globMatch(pat, args)) {
			continue
		}
		in.depth++
		in.pushFrame(m.Name, args)
		in.execText(m.Body)
		in.ctl = ctlNone
		in.popFrame()
		in.depth--
	}
}

func qesc(s string) string {
	return strings.NewReplacer("\\", "\\\\", "'", "\\'").Replace(s)
}

// printDef renders a macro as TF's /list does: "/def [options] name = body".
func (m *Macro) printDef(num bool) string {
	var b strings.Builder
	if num {
		fmt.Fprintf(&b, "%% %d: ", m.ID)
	}
	b.WriteString("/def ")
	if m.Invisible {
		b.WriteString("-i ")
	}
	if m.Trigger != "" || m.Hook != "" {
		f := ""
		if m.Fall {
			f = "F"
		}
		fmt.Fprintf(&b, "-%sp%d ", f, m.Priority)
	}
	if m.Attrs != "" {
		b.WriteString("-a" + m.Attrs + " ")
	}
	if m.Shots > 0 {
		fmt.Fprintf(&b, "-n%d ", m.Shots)
	}
	if m.World != "" {
		fmt.Fprintf(&b, "-w'%s' ", qesc(m.World))
	}
	mflag := ""
	if m.WorldType != "" {
		if m.Matching != mflag {
			b.WriteString("-m" + m.Matching + " ")
		}
		mflag = m.Matching
		fmt.Fprintf(&b, "-T'%s' ", qesc(m.WorldType))
	}
	if m.Cond != "" {
		fmt.Fprintf(&b, "-E'%s' ", qesc(m.Cond))
	}
	if m.Trigger != "" {
		if m.Matching != mflag {
			b.WriteString("-m" + m.Matching + " ")
		}
		mflag = m.Matching
		fmt.Fprintf(&b, "-t'%s' ", qesc(m.Trigger))
	}
	if m.Hook != "" {
		ev, args := firstWord(m.Hook)
		if args != "" {
			if m.Matching != mflag {
				b.WriteString("-m" + m.Matching + " ")
			}
			fmt.Fprintf(&b, "-h'%s %s' ", ev, qesc(args))
		} else {
			b.WriteString("-h" + ev + " ")
		}
	}
	if m.Bind != "" {
		fmt.Fprintf(&b, "-b'%s' ", qesc(m.Bind))
	}
	if m.Quiet {
		b.WriteString("-q ")
	}
	if strings.HasPrefix(m.Name, "-") {
		b.WriteString("- ")
	}
	if m.Name != "" {
		b.WriteString(m.Name + " ")
	}
	if m.Body != "" {
		b.WriteString("= " + m.Body)
	}
	return b.String()
}

// printShort is the "/list -s" form.
func (m *Macro) printShort() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%% %d: ", m.ID)
	if m.Gag {
		b.WriteString("(gag) ")
	} else if m.Trigger != "" {
		b.WriteString("(trig) ")
	}
	if m.Trigger != "" {
		fmt.Fprintf(&b, "'%s' ", qesc(m.Trigger))
	}
	if m.Bind != "" {
		fmt.Fprintf(&b, "(bind) '%s' ", qesc(m.Bind))
	}
	if m.Hook != "" {
		ev, _ := firstWord(m.Hook)
		b.WriteString("(hook) " + ev + " ")
	}
	if m.Name != "" {
		b.WriteString(m.Name + " ")
	}
	return b.String()
}
