package gtfparser

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// SubAttr is one part of a partial hilite (/def -P): attributes applied to
// the text matched by subexpression Subexp of the trigger regexp (-1 = the
// text left of the match, -2 = right of it).
type SubAttr struct {
	Subexp int
	Attr   Attr
}

// Indices into Macro.Used (the counters /list -u shows).
const (
	UsedName = iota // called by name
	UsedTrig        // run as a trigger
	UsedHook        // run as a hook
	UsedKey         // run from a key binding
	usedN
)

// Macro is a /def'd macro: optionally a trigger, hook or key binding. Its
// exported fields are the macro database record; treat them as read-only
// except through Interp methods (they feed indexes).
type Macro struct {
	ID   int
	Name string
	Body string

	// Cond is the -E expression ("" = none): the macro is only a match when
	// it evaluates true.
	Cond string
	// Bind is the -b key sequence ("" = none); Keyname the deprecated -B name.
	Bind    string
	Keyname string

	// Trig, HArgs and WType are the -t trigger pattern, the pattern for -h
	// hook arguments, and the -T world-type pattern (nil = not set).
	Trig, HArgs, WType *Pattern
	// Hooks is the set of events (-h) the macro hooks; IsHook says it is a
	// hook at all.
	Hooks  uint64
	IsHook bool

	// World is the -w world restriction ("" = any world).
	World string

	Priority  int // -p
	Prob      int // -c (percent chance of running)
	Shots     int // -n remaining shots (0 = permanent)
	Fall      bool
	Quiet     bool
	Invisible bool

	Attr           Attr // -a (and -f)
	MatchAttr      Attr // -A: the line's attributes must equal this
	CheckMatchAttr bool
	SubAttr        []SubAttr // -P

	// Used counts how often the macro ran by each route.
	Used [usedN]int

	// Matching is the match style in force when the macro was defined
	// (informational; each Pattern carries its own style).
	Matching string

	// Compatibility mirrors of the parsed fields above, kept for clients
	// that read the macro as text: Trigger is the -t pattern, Hook the -h
	// events (and argument pattern) as /list prints them, Attrs the -a
	// attributes as text, Gag the gag attribute, WorldType the -T pattern.
	// The engine sets them when the macro is installed and never reads them
	// back; a Go caller of Define may fill them instead of Trig/Hooks/Attr
	// (see Define).
	Trigger   string
	Hook      string
	Attrs     string
	Gag       bool
	WorldType string

	fromSpec bool
	dead     bool
	trigList *mlist
	trigNode *mnode
	hookNode [64]*mnode
	nsub     int // subexpression count of the trigger regexp
	relAttr  bool
	tnext    *Macro
}

// HasTrigger reports whether the macro has a -t pattern.
func (m *Macro) HasTrigger() bool { return m.Trig != nil }

// TriggerText returns the -t pattern text ("" if none).
func (m *Macro) TriggerText() string {
	if m.Trig == nil {
		return ""
	}
	return m.Trig.Str
}

// cmdErr reports an error the way tf's eprintf does: "CMD -o: message".
func (in *Interp) cmdErr(cmd string, opt byte, f string, a ...any) {
	if cmd == "" {
		in.warn("%s", fmt.Sprintf(f, a...))
	} else if opt != 0 {
		in.warn("%s -%c: %s", cmd, opt, fmt.Sprintf(f, a...))
	} else {
		in.warn("%s: %s", cmd, fmt.Sprintf(f, a...))
	}
}

// ---------------------------------------------------------------- option parsing

// optReader is tf's startopt/nextopt over an argument string.
type optReader struct {
	in     *Interp
	cmd    string
	s      string
	off    int
	opts   string
	inword bool
	cur    byte // option being parsed, for messages
}

// next returns the next option letter, its argument text, and whether it is
// an expression-valued option's integer. ok=false with c==0 means end of
// options; c=='?' means error (already reported).
func (r *optReader) next() (c byte, arg string, num int, hasArg bool) {
	r.cur = 0
	s := r.s
	if !r.inword {
		for r.off < len(s) && (s[r.off] == ' ' || s[r.off] == '\t') {
			r.off++
		}
		if r.off >= len(s) || s[r.off] != '-' {
			return 0, "", 0, false
		}
		r.off++
		if r.off < len(s) && s[r.off] == '-' {
			r.off++
			if r.off < len(s) && s[r.off] != ' ' && s[r.off] != '\t' {
				r.in.cmdErr(r.cmd, 0, "invalid option syntax: --%c", s[r.off])
				return '?', "", 0, false
			}
		}
		if r.off >= len(s) || s[r.off] == ' ' || s[r.off] == '\t' {
			for r.off < len(s) && (s[r.off] == ' ' || s[r.off] == '\t') {
				r.off++
			}
			return 0, "", 0, false
		}
	} else if r.off < len(s) && s[r.off] == '=' {
		return 0, "", 0, false
	}
	c = s[r.off]
	r.cur = c
	k := strings.IndexByte(r.opts, c)
	if c == '#' || c == ':' || c == '@' || k < 0 {
		r.in.cmdErr(r.cmd, c, "invalid option")
		var help strings.Builder
		o := r.opts
		dash := true
		for i := 0; i < len(o); i++ {
			isPunct := i+1 < len(o) && (o[i+1] == ':' || o[i+1] == '#' || o[i+1] == '@')
			if dash || isPunct {
				help.WriteString(" -")
			}
			if o[i] != '-' {
				help.WriteByte(o[i])
			}
			dash = false
			if i+1 < len(o) && o[i+1] == ':' {
				help.WriteString("<string>")
				i++
				dash = true
			} else if i+1 < len(o) && o[i+1] == '#' {
				help.WriteString("<integer>")
				i++
				dash = true
			}
		}
		r.in.cmdErr(r.cmd, 0, "options:%s", help.String())
		return '?', "", 0, false
	}
	r.off++
	kind := byte(0)
	if k+1 < len(r.opts) && (r.opts[k+1] == ':' || r.opts[k+1] == '#') {
		kind = r.opts[k+1]
	}
	if kind != 0 {
		var val string
		if r.off < len(s) && (s[r.off] == '"' || s[r.off] == '\'' || s[r.off] == '`') {
			q := s[r.off]
			j := r.off + 1
			var b strings.Builder
			for j < len(s) && s[j] != q {
				if s[j] == '\\' && j+1 < len(s) && (s[j+1] == q || s[j+1] == '\\') {
					j++
				}
				b.WriteByte(s[j])
				j++
			}
			if j >= len(s) {
				r.in.cmdErr(r.cmd, c, "unmatched %c", q)
				return '?', "", 0, false
			}
			r.off = j + 1
			val = b.String()
		} else {
			j := r.off
			for j < len(s) && s[j] != ' ' && s[j] != '\t' {
				j++
			}
			val = s[r.off:j]
			r.off = j
		}
		if kind == '#' {
			v, err := r.in.EvalExpr(val)
			if err != nil {
				r.in.cmdErr(r.cmd, c, "%v", err)
				return '?', "", 0, false
			}
			num = int(v.Int())
		}
		hasArg = true
		arg = val
	}
	r.inword = r.off < len(s) && s[r.off] != ' ' && s[r.off] != '\t'
	return c, arg, num, hasArg
}

// ---------------------------------------------------------------- macro spec

type macroSpec struct {
	m *Macro
	// -1 = unspecified ("don't care" for /list, default for /def)
	pri, prob, shots int
	fall, quiet      int
	invis            int // 0 default, 1 -i, 2 -I
	hasHook, noHook  bool
	hookMask         uint64
	hookPat          string
	hookHasPat       bool
	trigStr, wtStr   *string
	world            int // 0 unspecified, 1 any, 2 none, 3 named
	worldName        string
	mflag            string
	bindSet, keySet  bool
	exprSet          bool
	hasName, hasBody bool
	num              int
	short, used      bool
	sortName         bool
	rawMF            string // raw -m (for error reporting only)
	attrSet          bool
	mGiven           bool // -m (or -P) was given
}

// parseSpec is macro_spec(): it turns "/def" or "/list" arguments into a
// macroSpec. list selects the extra /list options (-u -s -S).
func (in *Interp) parseSpec(cmd, args string, list bool) *macroSpec {
	sp := &macroSpec{m: &Macro{}, pri: -1, prob: -1, shots: -1, fall: -1, quiet: -1}
	opts := "usSp#c#b:B:E:t:w:h:A:a:f:P:T:FiIn#1m:q"
	if !list {
		opts = opts[3:]
	}
	r := &optReader{in: in, cmd: cmd, s: args, opts: opts}
	ctx := in.attrCtx()
	var attrs Attr
	bad := false
	var partial string
	hasPartial := false
	for !bad {
		c, arg, num, _ := r.next()
		if c == 0 {
			break
		}
		switch c {
		case '?':
			bad = true
		case 'u':
			sp.used = true
		case 's':
			sp.short = true
		case 'S':
			sp.sortName = true
		case 'm':
			st, err := in.styleByName(arg)
			if err != nil {
				in.cmdErr(cmd, 'm', "%v", err)
				bad = true
				break
			}
			if sp.mflag != "" && sp.mflag != st {
				in.cmdErr(cmd, 'm', "-m option conflicts with earlier -m or -P")
				bad = true
				break
			}
			sp.mflag = st
		case 'p':
			sp.pri = num
		case 'c':
			sp.prob = num
		case 'F':
			sp.fall = 1
		case 'i':
			sp.invis = 1
		case 'I':
			sp.invis = 2
		case 'b':
			sp.bindSet = true
			sp.m.Bind = arg
		case 'B':
			if in.globals["warn_def_B"].String() != "off" {
				in.warn("Warning: /def -B is deprecated.  See /help keys.")
			}
			sp.keySet = true
			sp.m.Keyname = arg
		case 'E':
			sp.exprSet = true
			sp.m.Cond = arg
		case 't':
			s := arg
			sp.trigStr = &s
		case 'T':
			s := arg
			sp.wtStr = &s
		case 'w':
			switch arg {
			case "", "+":
				sp.world = 1
			case "-":
				sp.world = 2
			default:
				if !in.worldKnown(cmd, arg) {
					in.cmdErr(cmd, 'w', "No world %s", arg)
					bad = true
					break
				}
				sp.world, sp.worldName = 3, arg
			}
		case 'h':
			if arg == "0" {
				sp.noHook = true
			} else {
				mask, pat, hasPat, ok := in.parseHookSpec(arg)
				if !ok {
					bad = true
					break
				}
				sp.hasHook, sp.hookMask, sp.hookPat, sp.hookHasPat = true, mask, pat, hasPat
			}
		case 'a', 'f':
			a, _, err := ctx.Parse(arg, 0)
			if err != nil {
				in.cmdErr(cmd, c, "%v", err)
				bad = true
				break
			}
			attrs = ctx.Adj(attrs, a)
			sp.attrSet = true
		case 'A':
			a, _, err := ctx.Parse(arg, 0)
			if err != nil {
				in.cmdErr(cmd, 'A', "%v", err)
				bad = true
				break
			}
			sp.m.MatchAttr = ctx.Adj(sp.m.MatchAttr, a)
			sp.m.CheckMatchAttr = true
		case 'P':
			if hasPartial {
				in.cmdErr(cmd, 'P', "-P can be given only once per macro.")
				bad = true
				break
			}
			if sp.mflag != "" && sp.mflag != MatchRegexp {
				in.cmdErr(cmd, 'P', "\"-P\" requires \"-mregexp -t<pattern>\"")
				bad = true
				break
			}
			hasPartial, partial = true, arg
			for _, part := range strings.Split(arg, ";") {
				if part == "" {
					continue
				}
				var sa SubAttr
				rest := part
				switch {
				case rest[0] == 'L':
					sa.Subexp, rest = -1, rest[1:]
				case rest[0] == 'R':
					sa.Subexp, rest = -2, rest[1:]
				default:
					j := 0
					for j < len(rest) && rest[j] >= '0' && rest[j] <= '9' {
						j++
					}
					if j == 0 {
						sa.Subexp = 0
					} else {
						sa.Subexp, _ = strconv.Atoi(rest[:j])
					}
					rest = rest[j:]
				}
				a, _, err := ctx.Parse(rest, 0)
				if err != nil {
					in.cmdErr(cmd, 'P', "%v", err)
					bad = true
					break
				}
				sa.Attr = a
				sp.m.SubAttr = append(sp.m.SubAttr, sa)
			}
			sp.mflag = MatchRegexp
		case 'n':
			sp.shots = num
		case 'q':
			sp.quiet = 1
		case '1':
			sp.shots = 1
		}
	}
	_ = partial
	if bad {
		return nil
	}
	sp.m.Attr = attrs
	sp.mGiven = sp.mflag != ""
	if sp.mflag == "" {
		sp.mflag = in.defaultStyle()
	}
	// compile the patterns
	var err error
	if sp.trigStr != nil {
		if sp.m.Trig, err = in.compilePattern(sp.mflag, *sp.trigStr); err != nil {
			in.cmdErr(cmd, 't', "%v", err)
			return nil
		}
	}
	if sp.hasHook && sp.hookHasPat {
		if sp.m.HArgs, err = in.compilePattern(sp.mflag, sp.hookPat); err != nil {
			in.cmdErr(cmd, 'h', "%v", err)
			return nil
		}
	}
	if sp.wtStr != nil {
		if sp.m.WType, err = in.compilePattern(sp.mflag, *sp.wtStr); err != nil {
			in.cmdErr(cmd, 'w', "%v", err)
			return nil
		}
	}
	if sp.hasHook {
		sp.m.Hooks, sp.m.IsHook = sp.hookMask, true
	}
	// [name] [= body]
	rest := r.s[r.off:]
	rest = strings.TrimLeft(rest, " \t\r\n")
	if rest == "" {
		return sp
	}
	eq := strings.IndexByte(rest, '=')
	nameEnd := len(rest)
	if eq >= 0 {
		nameEnd = eq
		body := strings.TrimLeft(rest[eq+1:], " \t\r\n")
		if strings.TrimRight(body, " \t\r\n") != "" {
			sp.m.Body = body
			sp.hasBody = true
		}
	}
	name := strings.TrimRight(rest[:nameEnd], " \t\r\n")
	if name != "" {
		sp.m.Name = name
		sp.hasName = true
	}
	return sp
}

// worldKnown validates a /def -w world name: against the host's WorldInfoHost
// when it has one; /edit also accepts any world in the interpreter's world
// database. A client without either never rejects a name.
func (in *Interp) worldKnown(cmd, name string) bool {
	if wi, ok := in.Host.(WorldInfoHost); ok {
		return wi.WorldExists(name)
	}
	if cmd == "EDIT" {
		_, found := in.World(name)
		return found
	}
	return true
}

// editCopy returns a copy of m for /edit to modify and reinstall: the
// definition without its place in the trigger and hook lists.
func (m *Macro) editCopy() *Macro {
	c := *m
	c.trigList, c.trigNode, c.hookNode, c.dead, c.tnext = nil, nil, [64]*mnode{}, false, nil
	c.SubAttr = append([]SubAttr(nil), m.SubAttr...)
	return &c
}

// ParseMacroSpec parses /def-style arguments into a Macro record without
// defining it; /edit and other commands that take macro options build on it.
// Unspecified numeric fields are returned as their /def defaults.
func (in *Interp) ParseMacroSpec(cmd, args string) (*Macro, error) {
	sp := in.parseSpec(cmd, args, false)
	if sp == nil {
		return nil, fmt.Errorf("%s: invalid options", cmd)
	}
	in.fillDefaults(sp)
	return sp.m, nil
}

func (in *Interp) fillDefaults(sp *macroSpec) {
	m := sp.m
	m.fromSpec = true
	// negative values mean "not given" to tf, so they get the defaults
	if sp.pri < 0 {
		sp.pri = 1
	}
	if sp.prob < 0 {
		sp.prob = 100
	}
	if sp.shots < 0 {
		sp.shots = 0
	}
	m.Priority, m.Prob, m.Shots = sp.pri, sp.prob, sp.shots
	m.Fall = sp.fall > 0
	m.Quiet = sp.quiet > 0
	m.Invisible = sp.invis != 0
	if sp.world == 3 {
		m.World = sp.worldName
	} else if sp.world == 1 {
		m.World = in.ctxWorld()
	}
	m.Matching = sp.mflag
	m.Attr &^= AttrNone
}

// ---------------------------------------------------------------- the database

// Define installs a macro, as /def does after parsing. It is also the entry
// point for macros built in Go (key bindings the client defines at startup,
// for example). Missing defaults are filled in; redefinition semantics follow
// /def (see completeMacro). It returns the macro number, or 0 on failure.
//
// A macro may be described either with the parsed fields (Trig, Hooks, HArgs,
// Attr, WType) or, as before, with the text fields Trigger, Hook ("EVENT[|EVENT]
// [pattern]"), Attrs, Gag and WorldType, which are compiled here when the
// parsed field is unset.
func (in *Interp) Define(m *Macro) int {
	if !m.fromSpec {
		if m.Prob == 0 {
			m.Prob = 100
		}
		m.fromSpec = true
		if m.Matching == "" {
			m.Matching = in.defaultStyle()
		}
		if !in.compileTextFields(m) {
			return 0
		}
	}
	return in.completeMacro(m, 0)
}

// compileTextFields turns the text fields of a Go-built macro into the parsed
// ones.
func (in *Interp) compileTextFields(m *Macro) bool {
	var err error
	if m.Trig == nil && m.Trigger != "" {
		if m.Trig, err = in.compilePattern(m.Matching, m.Trigger); err != nil {
			in.cmdErr("DEF", 't', "%v", err)
			return false
		}
	}
	if !m.IsHook && m.Hook != "" {
		mask, pat, hasPat, ok := in.parseHookSpec(m.Hook)
		if !ok {
			return false
		}
		m.Hooks, m.IsHook = mask, true
		if hasPat && pat != "" {
			if m.HArgs, err = in.compilePattern(m.Matching, pat); err != nil {
				in.cmdErr("DEF", 'h', "%v", err)
				return false
			}
		}
	}
	if m.WType == nil && m.WorldType != "" {
		if m.WType, err = in.compilePattern(m.Matching, m.WorldType); err != nil {
			in.cmdErr("DEF", 'T', "%v", err)
			return false
		}
	}
	if m.Attr == 0 && m.Attrs != "" {
		a, err := in.ParseAttrs(m.Attrs)
		if err != nil {
			in.cmdErr("DEF", 'a', "%v", err)
			return false
		}
		m.Attr = a
	}
	if m.Gag {
		m.Attr |= AttrGag
	}
	return true
}

// syncText fills the compatibility mirrors from the parsed fields.
func (m *Macro) syncText() {
	m.Trigger, m.Hook, m.WorldType = "", "", ""
	if m.Trig != nil {
		m.Trigger = m.Trig.Str
	}
	if m.WType != nil {
		m.WorldType = m.WType.Str
	}
	if m.IsHook {
		m.Hook = hookNamesStatic(m.Hooks)
		if m.HArgs != nil && m.HArgs.Str != "" {
			m.Hook += " " + m.HArgs.Str
		}
	}
	m.Attrs = ""
	if m.Attr != 0 {
		m.Attrs = m.Attr.String()
	}
	m.Gag = m.Attr&AttrGag != 0
}

var keywordNames = map[string]bool{"break": true, "do": true, "done": true, "else": true, "elseif": true,
	"endif": true, "exit": true, "if": true, "let": true, "result": true, "return": true, "set": true,
	"setenv": true, "test": true, "then": true, "while": true}

var reservedBuiltins = map[string]bool{"eval": true}

// isBuiltinCmd reports whether name is one of real tf's builtin commands
// (the commands this package implements natively on top of that list, such as
// /loaded and /require, which tf itself defines as library macros, do not
// count).
func (in *Interp) isBuiltinCmd(name string) bool {
	n := strings.ToLower(name)
	for _, c := range tfCommands {
		if c == n {
			return true
		}
	}
	return false
}

// macroKey is the (case-insensitive) database key for a macro name.
func macroKey(name string) string {
	for i := 0; i < len(name); i++ {
		if name[i] >= 'A' && name[i] <= 'Z' {
			return strings.ToLower(name)
		}
	}
	return name
}

// findMacro looks a macro up by name (case-insensitively, as tf does) or by
// "#number".
func (in *Interp) findMacro(name string) (*Macro, bool) {
	if m, ok := in.macros[name]; ok {
		return m, true
	}
	if name == "" {
		return nil, false
	}
	if name[0] == '#' {
		n, err := strconv.Atoi(name[1:])
		if err != nil {
			return nil, false
		}
		m := in.macroByNum(n)
		return m, m != nil
	}
	m, ok := in.macros[macroKey(name)]
	return m, ok
}

func (in *Interp) macroByNum(n int) *Macro {
	i := sort.Search(len(in.list), func(i int) bool { return in.list[i].ID >= n })
	if i < len(in.list) && in.list[i].ID == n {
		return in.list[i]
	}
	return nil
}

// FindMacro returns the macro with the given name, or nil.
func (in *Interp) FindMacro(name string) *Macro {
	m, _ := in.findMacro(name)
	return m
}

// macroEqual is macro_equal(): exact equivalence except for the number.
func macroEqual(a, b *Macro) bool {
	if a.Invisible != b.Invisible || a.Shots != b.Shots || a.Fall != b.Fall || a.Quiet != b.Quiet ||
		a.Prob != b.Prob || a.Priority != b.Priority || a.Attr != b.Attr || len(a.SubAttr) != len(b.SubAttr) ||
		a.World != b.World || a.IsHook != b.IsHook || a.Bind != b.Bind || a.Keyname != b.Keyname ||
		a.Name != b.Name || a.Body != b.Body || a.Cond != b.Cond {
		return false
	}
	if a.CheckMatchAttr != b.CheckMatchAttr || a.MatchAttr != b.MatchAttr {
		return false
	}
	for _, x := range a.SubAttr {
		found := false
		for _, y := range b.SubAttr {
			if x.Subexp == y.Subexp {
				if x.Attr != y.Attr {
					return false
				}
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if a.IsHook && a.Hooks != b.Hooks {
		return false
	}
	pe := func(x, y *Pattern) bool {
		if (x == nil) != (y == nil) {
			return false
		}
		return x == nil || x.Str == y.Str && x.Style == y.Style
	}
	return pe(a.Trig, b.Trig) && pe(a.HArgs, b.HArgs) && pe(a.WType, b.WType)
}

// completeMacro is complete_macro(): validate, handle redefinition, and
// install. num/at are for /edit (keep the number); 0 allocates a new one.
func (in *Interp) completeMacro(m *Macro, num int) int {
	var builtin bool
	if m.Name != "" {
		if strings.ContainsRune("#@!/", rune(m.Name[0])) || strings.ContainsAny(m.Name, " \t") {
			in.cmdErr("DEF", 0, "illegal macro name \"%s\".", m.Name)
			return 0
		}
		lname := strings.ToLower(m.Name)
		builtin = in.isBuiltinCmd(m.Name)
		if keywordNames[lname] || builtin && reservedBuiltins[lname] {
			in.cmdErr("DEF", 0, "\"%s\" is a reserved word.", m.Name)
			return 0
		}
		if builtin {
			in.FireHook("CONFLICT", m.Name)
		}
	}
	if m.Cond != "" {
		if _, err := in.checkExpr(m.Cond); err != nil {
			in.cmdErr("DEF", 'E', "%v", err)
			return 0
		}
	}
	if len(m.SubAttr) > 0 {
		if m.Trig == nil || m.Trig.kind != pkRegexp {
			in.cmdErr("DEF", 0, "\"-P\" requires \"-mregexp -t<pattern>\"")
			return 0
		}
		n := m.Trig.re.groups()
		for i := range m.SubAttr {
			m.SubAttr[i].Attr &^= AttrNone
			// tf compares the part number as unsigned: L and R (-1, -2)
			// always fail this test, so partial hilites by L/R are rejected.
			if m.SubAttr[i].Subexp < 0 || m.SubAttr[i].Subexp > n {
				in.cmdErr("DEF", 0, "-P%d: trigger has only %d subexpressions", m.SubAttr[i].Subexp, n)
				return 0
			}
		}
		m.nsub = n
	}
	m.Attr &^= AttrNone
	m.relAttr = m.Attr&AttrDisplay != 0 || len(m.SubAttr) > 0
	if m.Bind != "" && m.Keyname != "" {
		m.Bind = ""
	}
	var orig *Macro
	if m.Name != "" {
		orig = in.macros[macroKey(m.Name)]
		if orig != nil {
			if macroEqual(orig, m) {
				return orig.ID // identical redefinition has no effect
			}
			if in.globals["redef"].String() == "off" {
				in.cmdErr("DEF", 0, "macro %s already exists", m.Name)
				return 0
			}
		}
	}
	id := in.addNumbered(m, num)
	if id <= 0 {
		return 0
	}
	if orig != nil {
		in.FireHookArgs("REDEF", "macro", orig.Name)
		in.killMacro(orig)
	}
	return id
}

// addNumbered is add_numbered_macro(): install a permanent macro.
func (in *Interp) addNumbered(m *Macro, num int) int {
	ms := in.macs()
	if m.Bind != "" {
		if orig := ms.binds[m.Bind]; orig != nil && !orig.dead {
			if macroEqual(orig, m) {
				return orig.ID
			}
			if in.globals["redef"].String() == "off" {
				in.cmdErr("DEF", 0, "Binding %s already exists.", m.Bind)
				return 0
			}
			in.killMacro(orig)
			in.FireHookArgs("REDEF", "binding", m.Bind)
		}
	}
	if num > 0 {
		m.ID = num
	} else {
		in.nextID++
		m.ID = in.nextID
	}
	m.syncText()
	in.insertByNum(m)
	if m.Name != "" {
		in.macros[macroKey(m.Name)] = m
	}
	if m.Bind != "" {
		ms.binds[m.Bind] = m
	}
	if m.Trig != nil {
		in.trigInsert(m)
	}
	if m.IsHook {
		in.hookInsert(m)
	}
	if m.Name == "" && (m.Trig != nil || m.IsHook) && m.Shots == 0 && in.globals["pedantic"].String() == "on" {
		in.warn("Warning: new macro (#%d) does not have a name.", m.ID)
	}
	return m.ID
}

func (in *Interp) insertByNum(m *Macro) {
	i := sort.Search(len(in.list), func(i int) bool { return in.list[i].ID >= m.ID })
	in.list = append(in.list, nil)
	copy(in.list[i+1:], in.list[i:])
	in.list[i] = m
}

// killMacro is kill_macro(): remove the macro from the database and mark it
// dead. It stays in the trigger and hook lists until the current matching
// pass is over, so a macro can safely remove itself or others while running.
func (in *Interp) killMacro(m *Macro) {
	if m.dead {
		return
	}
	m.dead = true
	ms := in.macs()
	in.removeFromList(m)
	if m.Name != "" {
		if in.macros[macroKey(m.Name)] == m {
			delete(in.macros, macroKey(m.Name))
		}
	}
	if m.Bind != "" && ms.binds[m.Bind] == m {
		delete(ms.binds, m.Bind)
	}
	ms.dead = append(ms.dead, m)
	if ms.matchDepth == 0 {
		in.nukeDead()
	}
}

func (in *Interp) removeFromList(m *Macro) {
	i := sort.Search(len(in.list), func(i int) bool { return in.list[i].ID >= m.ID })
	if i < len(in.list) && in.list[i] == m {
		in.list = append(in.list[:i], in.list[i+1:]...)
		return
	}
	for i, x := range in.list { // not sorted (externally built): scan
		if x == m {
			in.list = append(in.list[:i], in.list[i+1:]...)
			return
		}
	}
}

// Undef removes the macro with the given name. It reports whether there was one.
func (in *Interp) Undef(name string) bool {
	m, ok := in.findMacro(name)
	if !ok {
		return false
	}
	in.killMacro(m)
	return true
}

// Macros lists every defined macro in definition order.
func (in *Interp) Macros() []*Macro {
	return append([]*Macro(nil), in.list...)
}

// RemoveWorldMacros removes every macro restricted to world (call it when a
// world is deleted, as tf's /unworld does).
func (in *Interp) RemoveWorldMacros(world string) int {
	n := 0
	for _, m := range in.Macros() {
		if m.World != "" && strings.EqualFold(m.World, world) {
			in.killMacro(m)
			n++
		}
	}
	return n
}

// FindBinding returns the macro bound to a key sequence, or nil.
func (in *Interp) FindBinding(seq string) *Macro {
	m := in.macs().binds[seq]
	if m == nil || m.dead || in.macroByNum(m.ID) != m {
		return nil // never bound, or removed behind our back (/unbind)
	}
	return m
}

// ---------------------------------------------------------------- /def and friends

func cmdDef(in *Interp, args string) Value {
	if strings.TrimSpace(args) == "" {
		return Int(0)
	}
	sp := in.parseSpec("DEF", args, false)
	if sp == nil {
		return Int(0)
	}
	in.fillDefaults(sp)
	return Int(int64(in.completeMacro(sp.m, 0)))
}

func cmdUndef(in *Interp, args string) Value {
	n := 0
	for _, name := range strings.Fields(args) {
		m, ok := in.findMacro(name)
		if !ok {
			in.cmdErr("UNDEF", 0, "Macro \"%s\" was not defined.", name)
			continue
		}
		in.killMacro(m)
		n++
	}
	return Int(int64(n))
}

func cmdUndefn(in *Interp, args string) Value {
	n := 0
	rest := strings.TrimLeft(args, " \t")
	for rest != "" {
		w, r := firstWord(rest)
		rest = r
		if w == "" || w[0] < '0' || w[0] > '9' {
			in.cmdErr("UNDEFN", 0, "invalid or missing numeric argument")
			continue
		}
		num, _ := strconv.Atoi(strings.TrimRight(w, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_"))
		if m := in.macroByNum(num); m != nil {
			in.killMacro(m)
			n++
		} else {
			in.cmdErr("UNDEFN", 0, "no macro with number %d", num)
		}
	}
	return Int(int64(n))
}

// macroMatch is macro_match(): does macro satisfy every option spec gives?
func (in *Interp) macroMatch(sp *macroSpec, m *Macro, aux *auxPats) bool {
	sm := sp.m
	if sp.invis == 0 && m.Invisible {
		return false
	}
	if sp.invis == 2 && !m.Invisible {
		return false
	}
	if sp.shots >= 0 && sp.shots != m.Shots {
		return false
	}
	if sp.fall >= 0 && (sp.fall == 1) != m.Fall {
		return false
	}
	if sp.prob >= 0 && sp.prob != m.Prob {
		return false
	}
	if sp.pri >= 0 && sp.pri != m.Priority {
		return false
	}
	if sm.Attr != 0 && !attrMatch(sm.Attr, m.Attr) {
		return false
	}
	for _, sa := range sm.SubAttr {
		found := -1
		for j, ma := range m.SubAttr {
			if ma.Subexp == sa.Subexp {
				found = j
				break
			}
		}
		if found < 0 || !attrMatch(sa.Attr, m.SubAttr[found].Attr) {
			return false
		}
	}
	switch sp.world {
	case 2:
		if m.World != "" {
			return false
		}
	case 1:
		if m.World == "" {
			return false
		}
	case 3:
		if m.World != sp.worldName {
			return false
		}
	}
	if sp.keySet {
		if sm.Keyname == "" {
			if m.Keyname == "" {
				return false
			}
		} else if ok, _ := aux.keyname.Match(m.Keyname); !ok {
			return false
		}
	}
	if sp.bindSet {
		if sm.Bind == "" {
			if m.Bind == "" {
				return false
			}
		} else if ok, _ := aux.bind.Match(m.Bind); !ok {
			return false
		}
	}
	if sp.exprSet {
		if sm.Cond == "" {
			if m.Cond == "" {
				return false
			}
		} else {
			if m.Cond == "" {
				return false
			}
			if ok, _ := aux.expr.Match(m.Cond); !ok {
				return false
			}
		}
	}
	if sp.noHook {
		if m.IsHook {
			return false
		}
	} else if sp.hasHook {
		if !m.IsHook || sm.Hooks&m.Hooks == 0 {
			return false
		}
		if sm.HArgs != nil && sm.HArgs.Str != "" {
			s := ""
			if m.HArgs != nil {
				s = m.HArgs.Str
			}
			if ok, _ := sm.HArgs.Match(s); !ok {
				return false
			}
		}
	}
	if sm.Trig != nil {
		if sm.Trig.Str == "" {
			if m.Trig == nil {
				return false
			}
		} else {
			s := ""
			if m.Trig != nil {
				s = m.Trig.Str
			}
			if ok, _ := sm.Trig.Match(s); !ok {
				return false
			}
		}
	}
	if sm.WType != nil {
		if sm.WType.Str == "" {
			if m.WType == nil {
				return false
			}
		} else {
			s := ""
			if m.WType != nil {
				s = m.WType.Str
			}
			if ok, _ := sm.WType.Match(s); !ok {
				return false
			}
		}
	}
	if sp.num != 0 && m.ID != sp.num {
		return false
	}
	if sp.hasName {
		if ok, _ := aux.name.Match(m.Name); !ok {
			return false
		}
	}
	if sp.hasBody {
		if ok, _ := aux.body.Match(m.Body); !ok {
			return false
		}
	}
	return true
}

type auxPats struct{ name, body, bind, keyname, expr *Pattern }

func (in *Interp) initAux(sp *macroSpec) (*auxPats, bool) {
	a := &auxPats{}
	mk := func(s string, opt byte) *Pattern {
		p, err := in.compilePattern(sp.mflag, s)
		if err != nil {
			in.cmdErr("LIST", opt, "%v", err)
			return nil
		}
		return p
	}
	ok := true
	for _, x := range []struct {
		dst **Pattern
		s   string
		opt byte
	}{{&a.name, sp.m.Name, 0}, {&a.body, sp.m.Body, 0}, {&a.expr, sp.m.Cond, 'E'}, {&a.bind, sp.m.Bind, 'b'}, {&a.keyname, sp.m.Keyname, 'B'}} {
		if *x.dst = mk(x.s, x.opt); *x.dst == nil {
			ok = false
		}
	}
	return a, ok
}

// matchingMacros returns the macros matching a /list-style spec, oldest first.
func (in *Interp) matchingMacros(sp *macroSpec) ([]*Macro, bool) {
	if sp.hasName && strings.HasPrefix(sp.m.Name, "#") {
		sp.num, _ = strconv.Atoi(sp.m.Name[1:])
		sp.m.Name, sp.hasName = "", false
	}
	aux, ok := in.initAux(sp)
	if !ok {
		return nil, false
	}
	var out []*Macro
	for _, m := range in.list {
		if in.macroMatch(sp, m, aux) {
			out = append(out, m)
		}
	}
	return out, true
}

func cmdPurge(in *Interp, args string) Value {
	sp := in.parseSpec("PURGE", args, false)
	if sp == nil {
		return Int(0)
	}
	ms, ok := in.matchingMacros(sp)
	if !ok {
		return Int(0)
	}
	for _, m := range ms {
		in.killMacro(m)
	}
	return Int(int64(len(ms)))
}

func cmdList(in *Interp, args string) Value {
	sp := in.parseSpec("LIST", args, true)
	if sp == nil {
		return Int(0)
	}
	ms, ok := in.matchingMacros(sp)
	if !ok {
		return Int(0)
	}
	if sp.sortName {
		sort.SliceStable(ms, func(i, j int) bool { return strings.ToLower(ms[i].Name) < strings.ToLower(ms[j].Name) })
	}
	result := 0
	for _, m := range ms {
		result = m.ID
		if sp.short {
			in.echo(m.printShort(sp.used))
		} else {
			in.echo(m.printDef(true))
		}
	}
	return Int(int64(result))
}

// cmdSave is /save [-a] file [macro-options]: write matching macros to a file
// as /def commands.
func cmdSave(in *Interp, args string) Value {
	if strings.TrimSpace(args) == "" {
		in.cmdErr("SAVE", 0, "missing filename")
		return Int(0)
	}
	app := false
	rest := strings.TrimLeft(args, " \t")
	for strings.HasPrefix(rest, "-a") && (len(rest) == 2 || rest[2] == ' ') {
		app = true
		rest = strings.TrimLeft(rest[2:], " \t")
	}
	name, rest := firstWord(rest)
	sp := in.parseSpec("SAVE", rest, false)
	if sp == nil {
		return Int(0)
	}
	ms, ok := in.matchingMacros(sp)
	if !ok {
		return Int(0)
	}
	if !in.allow(OpFileWrite, "/save", name) {
		return Int(0)
	}
	var lines []string
	for _, m := range ms {
		lines = append(lines, m.printDef(false))
	}
	verb := "Writing"
	if app {
		verb = "Appending"
	}
	path, err := in.writeLines(name, lines, app)
	if err != nil {
		in.warn("%s: %v", name, err)
		return Int(0)
	}
	in.echo("% " + verb + " macros to " + path)
	last := 0
	if len(ms) > 0 {
		last = ms[len(ms)-1].ID
	}
	return Int(int64(last))
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
	if m.Trig != nil || m.IsHook {
		f := ""
		if m.Fall {
			f = "F"
		}
		fmt.Fprintf(&b, "-%sp%d ", f, m.Priority)
	}
	if m.Prob != 100 {
		fmt.Fprintf(&b, "-c%d ", m.Prob)
	}
	if m.Attr != 0 {
		b.WriteString("-a" + m.Attr.String() + " ")
	}
	if m.CheckMatchAttr {
		b.WriteString("-A" + m.MatchAttr.String() + " ")
	}
	mflag := ""
	if len(m.SubAttr) > 0 {
		mflag = MatchRegexp
		b.WriteString("-P")
		for i, sa := range m.SubAttr {
			if i > 0 {
				b.WriteByte(';')
			}
			switch sa.Subexp {
			case -1:
				b.WriteByte('L')
			case -2:
				b.WriteByte('R')
			default:
				b.WriteString(strconv.Itoa(sa.Subexp))
			}
			b.WriteString(sa.Attr.String())
		}
		b.WriteByte(' ')
	}
	if m.Shots > 0 {
		fmt.Fprintf(&b, "-n%d ", m.Shots)
	}
	if m.World != "" {
		fmt.Fprintf(&b, "-w'%s' ", qesc(m.World))
	}
	if m.WType != nil {
		if m.WType.Style != mflag {
			b.WriteString("-m" + m.WType.Style + " ")
		}
		mflag = m.WType.Style
		fmt.Fprintf(&b, "-T'%s' ", qesc(m.WType.Str))
	}
	if m.Cond != "" {
		fmt.Fprintf(&b, "-E'%s' ", qesc(m.Cond))
	}
	if m.Trig != nil {
		if m.Trig.Style != mflag {
			b.WriteString("-m" + m.Trig.Style + " ")
		}
		mflag = m.Trig.Style
		fmt.Fprintf(&b, "-t'%s' ", qesc(m.Trig.Str))
	}
	if m.IsHook {
		names := hookNamesStatic(m.Hooks)
		if m.HArgs != nil && m.HArgs.Str != "" {
			if m.HArgs.Style != mflag {
				mflag = m.HArgs.Style
				b.WriteString("-m" + mflag + " ")
			}
			fmt.Fprintf(&b, "-h'%s %s' ", names, qesc(m.HArgs.Str))
		} else {
			b.WriteString("-h" + names + " ")
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

// hookNamesStatic renders a mask using the full table (optional events that
// are on in the mask are shown; masks from /def never contain disabled ones
// except via "*", which prints only the built-in set).
func hookNamesStatic(mask uint64) string {
	var parts []string
	all := mask == hookAll
	for i := range hookTable {
		if mask&(1<<uint(i)) == 0 {
			continue
		}
		if all && hookTable[i].Optional {
			continue
		}
		parts = append(parts, hookTable[i].Name)
	}
	return strings.Join(parts, "|")
}

// printShort is the "/list -s" form.
func (m *Macro) printShort(used bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%% %d: ", m.ID)
	a := m.Attr
	if a&AttrNoHistory != 0 {
		b.WriteString("(nohistory) ")
	}
	if a&AttrNoLog != 0 {
		b.WriteString("(nolog) ")
	}
	if a&AttrGag != 0 {
		b.WriteString("(gag) ")
	} else if a&(AttrDisplay|AttrExclusive) != 0 {
		for _, x := range []struct {
			bit  Attr
			name string
		}{{AttrUnderline, "underline"}, {AttrReverse, "reverse"}, {AttrBold, "bold"}, {AttrItalic, "italic"}, {AttrBell, "bell"}, {AttrHilite, "hilite"}} {
			if a&x.bit != 0 {
				b.WriteString("(" + x.name + ") ")
			}
		}
		if a.HasFg() {
			b.WriteString("(" + ColorName(a.FgColor()) + ") ")
		}
		if a.HasBg() {
			b.WriteString("(bg" + ColorName(a.BgColor()) + ") ")
		}
	} else if len(m.SubAttr) > 0 {
		b.WriteString("(partial) ")
	} else if m.Trig != nil {
		b.WriteString("(trig")
		if used {
			fmt.Fprintf(&b, " %d", m.Used[UsedTrig])
		}
		b.WriteString(") ")
	}
	if m.Trig != nil {
		fmt.Fprintf(&b, "'%s' ", qesc(m.Trig.Str))
	}
	if m.Bind != "" {
		b.WriteString("(bind")
		if used {
			fmt.Fprintf(&b, " %d", m.Used[UsedKey])
		}
		fmt.Fprintf(&b, ") '%s' ", qesc(m.Bind))
		if m.Keyname != "" {
			fmt.Fprintf(&b, "(%s) ", m.Keyname)
		}
	}
	if m.IsHook {
		b.WriteString("(hook")
		if used {
			fmt.Fprintf(&b, " %d", m.Used[UsedHook])
		}
		fmt.Fprintf(&b, ") %s ", hookNamesStatic(m.Hooks))
	}
	if m.Name != "" {
		b.WriteString(m.Name + " ")
		if used {
			fmt.Fprintf(&b, "(%d) ", m.Used[UsedName])
		}
	}
	return b.String()
}

func init() {
	builtinCmds["def"] = cmdDef
	builtinCmds["undef"] = cmdUndef
	builtinCmds["undefn"] = cmdUndefn
	builtinCmds["purge"] = cmdPurge
	builtinCmds["list"] = cmdList
	builtinCmds["save"] = cmdSave
}
