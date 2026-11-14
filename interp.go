package gtfparser

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type ctl int

const (
	ctlNone ctl = iota
	ctlBreak
	ctlReturn
	ctlExit // /exit: abandons everything up to the enclosing /load
)

type frame struct {
	vars   map[string]Value
	parent *frame
	name   string
	text   string // raw positional text
	words  []string
	offs   []int
	regs   []string // %P0.. from the last regexp match in scope
	pl, pr string
	asFunc bool
	noOut  bool // tfout closed in this scope
}

// Interp is one TinyFugue scripting environment.
type Interp struct {
	noNative                bool // WithoutNativeLibrary
	nativeSeen              map[string]bool
	rawSetValue             bool // the running /set or /let line had a value part before expansion
	Host                    Host
	globals                 map[string]Value
	macros                  map[string]*Macro
	mac                     *macState // trigger, hook and match-style state (see triggers.go)
	list                    []*Macro
	nextID                  int
	fr                      *frame
	lastRet                 Value
	retv                    Value
	ctl                     ctl
	brk                     int
	depth                   int
	loaded                  map[string]bool
	noExp                   bool
	noEnv                   bool
	Quit                    bool
	streams                 map[string]*stream
	nextFd                  int
	tfin                    []string
	pendingArgs             []string
	templates               map[string]*template
	stmtCache               map[string]*compiled
	exprCache               map[string]*exprEntry
	procs                   []*Proc
	nextPID                 int
	cmds                    map[string]cmdFunc // this interpreter's builtin commands (see registry.go)
	funcs                   map[string]fn      // ... and functions
	disabled                map[string]bool
	restriction             int
	limits                  Limits
	interrupted             atomic.Bool
	instr                   int64
	nest                    int
	asFuncCmd               int    // >0 while a builtin command runs as a function
	exitReq                 bool   // /exit ran: the enclosing /load stops (see LoadFile)
	curCmd                  string // name of the running macro, builtin command or function (diagnostics)
	subWarned, dollarWarned bool   // one-time expression warnings (tf's statics)
	fnParent                string // the command that was running when the current builtin function was called
	inEval                  int    // depth of /eval (see warnEval)
	loadWarned              string // file already warned about for readable passwords
	quietLoad               int    // depth of /load -q (suppresses 'Loading commands from')
	loadName                string // file being loaded, and the lines of the logical line being run
	loadStart               int
	loadLine                int
	userRes                 Value // tf's user_result: the value of the last command that sets one (not /set, /let); what a macro called as a function returns
	readDepth               int
	mail                    map[string]*mailInfo
	subFail                 bool // a $[...] substitution failed: the statement and its macro are abandoned
	exported                map[string]bool
	cmdSpecs                map[string]CommandSpec
	funcSpecs               map[string]FuncSpec
	world                   string // see SetWorld
	// Policy, if set, can veto privileged operations on top of /restrict.
	Policy Policy
	// Now replaces the clock (tests); nil means time.Now.
	Now func() time.Time
	// LibDirs are searched by /load and /require for relative names.
	LibDirs []string
	// Warn receives interpreter diagnostics (TF prints these as "% ...").
	Warn func(string)
}

// Option configures New.
type Option func(*Interp)

// WithLimits sets hard ceilings on script resource use.
func WithLimits(l Limits) Option { return func(in *Interp) { in.limits = l } }

// WithPolicy installs a Policy that can veto privileged operations.
func WithPolicy(p Policy) Option { return func(in *Interp) { in.Policy = p } }

// WithRestriction starts the interpreter at a /restrict level.
func WithRestriction(level int) Option { return func(in *Interp) { in.restriction = level } }

// WithLibDirs sets the directories /load and /require search.
func WithLibDirs(dirs ...string) Option { return func(in *Interp) { in.LibDirs = dirs } }

// WithoutEnvironment keeps the process environment out of script variables.
func WithoutEnvironment() Option { return func(in *Interp) { in.noEnv = true } }

func New(h Host, opts ...Option) *Interp {
	in := &Interp{
		limits:    DefaultLimits(),
		templates: map[string]*template{},
		stmtCache: map[string]*compiled{},
		exprCache: map[string]*exprEntry{},
		exported:  map[string]bool{},
		Host:      h,
		globals:   map[string]Value{},
		macros:    map[string]*Macro{},
		loaded:    map[string]bool{},
		lastRet:   Int(1),
	}
	in.fr = &frame{vars: map[string]Value{}, name: ""}
	for _, o := range opts {
		o(in)
	}
	in.initRegistry()
	defer func() {
		// the keys tf itself binds before any library is loaded
		for _, b := range [][2]string{{"^W", "/DOKEY BWORD"}, {"^R", "/DOKEY REFRESH"}, {"^V", "/DOKEY LNEXT"}} {
			in.Define(&Macro{Matching: "glob", Priority: 1, Invisible: true, Bind: b[0], Body: b[1]})
		}
	}()
	in.Warn = func(s string) { h.Echo("% " + s) }
	for _, e := range environ(in.noEnv) { // the environment is imported as variables
		if i := strings.IndexByte(e, '='); i > 0 {
			in.globals[e[:i]] = Str(e[i+1:])
		}
	}
	for name, sp := range specialVars {
		in.globals[name] = sp.initial()
	}
	setSigFigs(15)
	in.globals["TFVERSION"] = Str("5.0 beta 8")
	in.globals["version"] = Str("TinyFugue (gtf) version 5.0 beta 8")
	// the constants stdlib.tf's "/test pi:=2 * acos(0)" and "/test e:=exp(1)" define
	for name, src := range map[string]string{"pi": "2 * acos(0)", "e": "exp(1)"} {
		if v, err := in.evalExpr(src); err == nil {
			in.globals[name] = v
		}
	}
	return in
}

func (in *Interp) SetVar(name, val string) { in.globals[name] = Str(val) }

func (in *Interp) Var(name string) string {
	v, _ := in.lookup(name)
	return v.String()
}

// cmdErr is an error that names the builtin function it came from, so the
// diagnostic reads "% file, line N: mod: division by zero" as in tf.
type cmdErr struct {
	cmd string
	err error
}

func (e *cmdErr) Error() string { return e.err.Error() }
func (e *cmdErr) Unwrap() error { return e.err }

// warn reports a diagnostic the way tf's eprintf does: "file, line N: "
// while loading, then the name of the command, macro or function that is
// running ("/cmd: " at the start of the message names a builtin command and
// is shown as tf shows it, upper case), then the message.
// warnEval is warn for diagnostics of the substitution/compile phase of a
// line: while /eval runs, tf shows EVAL as the command for these (but not for
// errors raised later, when the evaluated text runs).
func (in *Interp) warnEval(f string, a ...any) {
	if in.curCmd == "" && in.inEval > 0 {
		in.curCmd = "EVAL"
		defer func() { in.curCmd = "" }()
	}
	in.warn(f, a...)
}

func (in *Interp) warn(f string, a ...any) {
	cmd := in.curCmd
	for _, x := range a {
		if ce, ok := x.(*cmdErr); ok {
			cmd = ce.cmd
		}
	}
	msg := fmt.Sprintf(f, a...)
	if len(msg) > 2 && msg[0] == '/' {
		if i := strings.Index(msg, ": "); i > 1 {
			// "/cmd: msg" and "/cmd -x: msg" name a builtin command, shown as tf shows it
			name, opt, _ := strings.Cut(msg[1:i], " ")
			if !strings.ContainsAny(name, "/ ") && tfCmdSet[strings.ToLower(name)] && (opt == "" || (len(opt) == 2 && opt[0] == '-')) {
				cmd, msg = strings.ToUpper(name), msg[i+2:]
				if opt != "" {
					cmd += " " + opt
				}
			}
		}
	}
	if cmd != "" {
		// a message that names the function or command already named by the
		// context prefix ("addworld(): restricted") must not repeat it
		for _, p := range []string{cmd + "(): ", cmd + ": "} {
			if strings.HasPrefix(msg, p) {
				msg = msg[len(p):]
				break
			}
		}
	}
	var b strings.Builder
	if in.loadName != "" {
		if in.loadStart == in.loadLine {
			fmt.Fprintf(&b, "%s, line %d: ", in.loadName, in.loadLine)
		} else {
			fmt.Fprintf(&b, "%s, lines %d-%d: ", in.loadName, in.loadStart, in.loadLine)
		}
	}
	if cmd != "" {
		b.WriteString(cmd + ": ")
	}
	in.Warn(b.String() + msg)
}

// ---------------------------------------------------------------- variables

func (in *Interp) lookup(name string) (Value, bool) {
	for f := in.fr; f != nil; f = f.parent {
		if v, ok := f.vars[name]; ok {
			return v, true
		}
	}
	v, ok := in.globals[name]
	if !ok {
		return Str(""), false
	}
	return v, ok
}

// assign sets the nearest existing variable, or creates a global.
func (in *Interp) assign(name string, v Value) {
	for f := in.fr; f != nil; f = f.parent {
		if _, ok := f.vars[name]; ok {
			f.vars[name] = v
			return
		}
	}
	in.setGlobal(name, v)
}

func (in *Interp) pushFrame(name, text string) *frame {
	f := &frame{vars: map[string]Value{}, parent: in.fr, name: name}
	f.setArgs(text)
	in.fr = f
	return f
}

func (in *Interp) popFrame() { in.fr = in.fr.parent }

func (f *frame) setArgs(text string) {
	f.text = text
	f.words, f.offs = nil, nil
	i := 0
	for i < len(text) {
		for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
			i++
		}
		if i >= len(text) {
			break
		}
		st := i
		for i < len(text) && text[i] != ' ' && text[i] != '\t' {
			i++
		}
		f.words = append(f.words, text[st:i])
		f.offs = append(f.offs, st)
	}
}

// argFrame is the frame whose positional parameters are visible.
func (in *Interp) argFrame() *frame { return in.fr }

// selector resolves the inside of %{...} (or the bare form): a variable
// name, positional parameter, or special, with an optional "-default".
func (in *Interp) selector(sel string) string {
	name, def, hasDef := splitSelector(sel)
	v := in.selValue(name)
	if v == "" && hasDef {
		return in.expand(def)
	}
	return v
}

func splitSelector(sel string) (name, def string, hasDef bool) {
	i := 0
	switch {
	case strings.HasPrefix(sel, "-L"):
		i = 2
		for i < len(sel) && sel[i] >= '0' && sel[i] <= '9' {
			i++
		}
	case strings.HasPrefix(sel, "-") && len(sel) > 1 && sel[1] >= '0' && sel[1] <= '9':
		i = 1
		for i < len(sel) && sel[i] >= '0' && sel[i] <= '9' {
			i++
		}
	case len(sel) > 0 && (sel[0] == '*' || sel[0] == '#' || sel[0] == '?'):
		i = 1
	default:
		for i < len(sel) && isIdentChar(sel[i]) {
			i++
		}
	}
	name = sel[:i]
	if i < len(sel) && sel[i] == '-' {
		return name, sel[i+1:], true
	}
	return name, "", false
}

var (
	reL  = regexp.MustCompile(`^(-?)L(\d*)$`)
	reP  = regexp.MustCompile(`^P(\d+|L|R)$`)
	reNn = regexp.MustCompile(`^-?\d+$`)
)

func (in *Interp) selValue(name string) string {
	f := in.fr
	switch name {
	case "":
		return ""
	case "*":
		return f.text
	case "#":
		return strconv.Itoa(len(f.words))
	case "?":
		return in.lastRet.String()
	case "0":
		return f.name
	case "R":
		if len(f.words) == 0 {
			return ""
		}
		return f.words[rand.Intn(len(f.words))]
	}
	if reNn.MatchString(name) {
		n, _ := strconv.Atoi(name)
		if n > 0 {
			if n <= len(f.words) {
				return f.words[n-1]
			}
			return ""
		}
		k := -n // all except the first k
		if k >= len(f.words) {
			return ""
		}
		return f.text[f.offs[k]:]
	}
	if m := reL.FindStringSubmatch(name); m != nil {
		n := 1
		if m[2] != "" {
			n, _ = strconv.Atoi(m[2])
		}
		if m[1] == "" {
			if n >= 1 && n <= len(f.words) {
				return f.words[len(f.words)-n]
			}
			return ""
		}
		if n >= len(f.words) {
			return ""
		}
		end := len(f.words) - n
		if f.asFunc { // function arguments may be empty: tf joins the remaining ones
			return strings.Join(f.words[:end], " ")
		}
		return strings.TrimRight(f.text[:f.offs[end-1]+len(f.words[end-1])], " \t")
	}
	if m := reP.FindStringSubmatch(name); m != nil {
		for fr := in.fr; fr != nil; fr = fr.parent {
			if fr.regs == nil {
				continue
			}
			switch m[1] {
			case "L":
				return fr.pl
			case "R":
				return fr.pr
			}
			n, _ := strconv.Atoi(m[1])
			if n < len(fr.regs) {
				return fr.regs[n]
			}
			return ""
		}
		return ""
	}
	v, _ := in.lookup(name)
	return v.String()
}

// ---------------------------------------------------------------- expansion

// expand performs TF macro-body substitution on s. The text is compiled once
// into a template of literal and substitution segments (cached per
// interpreter), so a hot macro body is not re-scanned on every run.
func (in *Interp) expand(s string) string {
	if in.noExp || strings.IndexAny(s, "\\%$") < 0 {
		return s
	}
	t := in.templates[s]
	if t == nil {
		t = compileTemplate(s)
		if len(in.templates) >= cacheMax {
			in.templates = map[string]*template{}
		}
		in.templates[s] = t
	}
	if len(t.segs) == 1 && t.segs[0].kind == segLit {
		return t.segs[0].lit
	}
	var b strings.Builder
	b.Grow(len(s))
	raw := false
	if len(t.segs) > 0 && t.hasBack {
		if v, ok := in.globals["backslash"]; ok && v.String() == "off" {
			raw = true
		}
	}
	for i := range t.segs {
		sg := &t.segs[i]
		switch sg.kind {
		case segLit:
			b.WriteString(sg.lit)
		case segBack:
			if raw {
				b.WriteString(sg.alt)
			} else {
				b.WriteString(sg.lit)
			}
		default:
			b.WriteString(in.runSub(sg.sub, sg.lit))
		}
	}
	return b.String()
}

const (
	segLit byte = iota
	segBack
	segSub
)

type seg struct {
	kind byte
	sub  byte
	lit  string // literal text, or the substitution argument
	alt  string // segBack: the text when %backslash is off
}

type template struct {
	segs    []seg
	hasBack bool
}

const cacheMax = 8192

func compileTemplate(s string) *template {
	t := &template{}
	var lit strings.Builder
	flush := func() {
		if lit.Len() > 0 {
			t.segs = append(t.segs, seg{kind: segLit, lit: lit.String()})
			lit.Reset()
		}
	}
	for i := 0; i < len(s); {
		c := s[i]
		switch c {
		case '\\':
			if i+1 >= len(s) {
				lit.WriteByte('\\')
				i++
				continue
			}
			n := s[i+1]
			out := string(n)
			adv := 2
			if n >= '0' && n <= '9' {
				j := i + 1
				base := 10
				if strings.HasPrefix(s[j:], "0x") || strings.HasPrefix(s[j:], "0X") {
					base, j = 16, j+2
				} else if n == '0' {
					base = 8
				}
				st := j
				for j < len(s) && digitIn(s[j], base) {
					j++
				}
				if j > st {
					v, _ := strconv.ParseInt(s[st:j], base, 32)
					out, adv = string(rune(v)), j-i
				}
			}
			flush()
			t.segs = append(t.segs, seg{kind: segBack, lit: out, alt: "\\"})
			t.hasBack = true
			// when backslash processing is off only the backslash itself is
			// emitted and scanning resumes right after it
			if adv != 1 {
				i += adv
				// the character(s) after the backslash must still be emitted
				// literally in raw mode
				t.segs[len(t.segs)-1].alt = "\\" + s[i-adv+1:i]
			}
		case '%', '$':
			j := i
			for j < len(s) && s[j] == c {
				j++
			}
			if j-i >= 2 { // compression: n signs become n-1
				lit.WriteString(strings.Repeat(string(c), j-i-1))
				i = j
				continue
			}
			n, kind, arg := parseSub(s[i:])
			if n == 0 {
				lit.WriteByte(c)
				i++
				continue
			}
			flush()
			t.segs = append(t.segs, seg{kind: segSub, sub: kind, lit: arg})
			i += n
		default:
			lit.WriteByte(c)
			i++
		}
	}
	flush()
	return t
}

func digitIn(c byte, base int) bool {
	switch base {
	case 8:
		return c >= '0' && c <= '7'
	case 16:
		return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
	}
	return c >= '0' && c <= '9'
}

// expandOne is scanSub for the expression parser.
func (in *Interp) expandOne(s string) (int, func() string) { return in.scanSub(s) }

// scanSub recognises one substitution at the start of s and returns how many
// bytes it spans and a function producing its text. n == 0: not a substitution.
func (in *Interp) scanSub(s string) (int, func() string) {
	n, kind, arg := parseSub(s)
	if n == 0 {
		return 0, nil
	}
	return n, func() string { return in.runSub(kind, arg) }
}

// substitution kinds
const (
	subBrace byte = iota // %{selector[-default]}
	subBare              // %name %1 %* ...
	subExpr              // $[expr]
	subCmd               // $(cmd)
	subMacro             // ${name}
	subErr               // a malformed substitution: arg is tf's complaint
)

func (in *Interp) runSub(kind byte, arg string) string {
	switch kind {
	case subBrace:
		return in.selector(arg)
	case subBare:
		return in.selValue(arg)
	case subExpr:
		v, err := in.evalExpr(arg)
		if err != nil {
			if se, ok := err.(*syntaxError); ok {
				in.warnEval("%s", strings.Replace(se.msg, "expected end of expression", "expected operator or ']'", 1))
			} else {
				in.warn("%v", err)
			}
			in.subFail = true
		}
		return v.String()
	case subCmd:
		return in.captureCmd(arg)
	case subErr:
		in.warnEval("%s", arg)
		in.subFail = true
		return ""
	}
	arg = in.expand(arg)                     // the name may itself contain substitutions: ${%{L}}
	if b, ok := in.WorldMacroBody(arg); ok { // ${world_name} and friends read the current world
		return b
	}
	if m, ok := in.findMacro(arg); ok {
		return m.Body
	}
	if strings.HasPrefix(arg, "world_") && validWorldField(arg[6:]) {
		return "" // no current world: tf expands the world macros to nothing
	}
	in.echo("% macro not defined: " + arg)
	return ""
}

// parseSub recognises one substitution at the start of s: its length (0 = not
// a substitution), kind and argument. It does not depend on interpreter state.
func parseSub(s string) (int, byte, string) {
	if len(s) < 2 {
		return 0, 0, ""
	}
	switch s[0] {
	case '%':
		c := s[1]
		if c == '{' {
			end := matchClose(s, 1, '{', '}')
			if end < 0 {
				return len(s), subErr, "unmatched { or bad substitution"
			}
			if !validBraceSel(s[2:end]) {
				return end + 1, subErr, braceSelError(s[2:end])
			}
			return end + 1, subBrace, s[2:end]
		}
		j := 1
		switch {
		case c == '*' || c == '#' || c == '?':
			j = 2
		case c == '-':
			if len(s) > 2 && (s[2] >= '0' && s[2] <= '9' && s[2] != '0' || s[2] == 'L') {
				j = 2
				if s[2] == 'L' {
					j = 3
				}
				for j < len(s) && s[j] >= '0' && s[j] <= '9' {
					j++
				}
			} else {
				return 2, subErr, "illegal character '-' in substitution"
			}
		case c >= '0' && c <= '9':
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
		case isIdentStart(c):
			for j < len(s) && isIdentChar(s[j]) {
				j++
			}
		case c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == ';' || c == '|':
			return 0, 0, "" // "% " is left alone
		default:
			return 1 + 0, subBare, "" // an empty variable name: the % vanishes, the character stays
		}
		if j < len(s) && s[j] == '-' && c != '#' && c != '?' {
			// %name-default: the default runs to the next blank or %;
			k := j + 1
			for k < len(s) && s[k] != ' ' && s[k] != '\t' && !(s[k] == '%' && k+1 < len(s) && (s[k+1] == ';' || s[k+1] == '|')) {
				if s[k] == '\\' && k+1 < len(s) {
					k += 2
					continue
				}
				if s[k] == '%' || s[k] == '$' {
					if n, _, _ := parseSub(s[k:]); n > 0 {
						k += n
						continue
					}
				}
				k++
			}
			return k, subBrace, s[1:k]
		}
		return j, subBare, s[1:j]
	case '$':
		switch s[1] {
		case '[':
			if end := matchClose(s, 1, '[', ']'); end >= 0 {
				return end + 1, subExpr, s[2:end]
			}
			return len(s), subErr, "unmatched $["
		case '(':
			if end := matchCloseCmd(s, 1); end >= 0 {
				return end + 1, subCmd, s[2:end]
			}
			return len(s), subErr, "unmatched ("
		case '{':
			if end := matchClose(s, 1, '{', '}'); end >= 0 {
				return end + 1, subMacro, s[2:end]
			}
			return len(s), subErr, "unmatched ${"
		}
	}
	return 0, 0, ""
}

// matchCloseCmd finds the ')' ending $( ... ); a literal ')' is written "\)".
func matchCloseCmd(s string, i int) int {
	depth := 0
	for ; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

type captureHost struct {
	Host
	lines []string
}

func (c *captureHost) Echo(t string) { c.lines = append(c.lines, t) }

// captureCmd runs cmd as a macro body and returns its echoed output joined
// into one line.
func (in *Interp) captureCmd(cmd string) string {
	// in tf the closing ')' follows the text, so a final lone '%' is an (empty) substitution
	if strings.HasSuffix(cmd, "%") && !strings.HasSuffix(cmd, "%%") && !strings.HasSuffix(cmd, "\\%") {
		cmd = cmd[:len(cmd)-1]
	}
	old := in.Host
	ch := &captureHost{Host: old}
	in.Host = ch
	in.pushFrame(in.fr.name, in.fr.text)
	in.fr.words, in.fr.offs = in.fr.parent.words, in.fr.parent.offs
	oldExp := in.noExp
	in.noExp = false
	in.execText(cmd)
	in.noExp = oldExp
	in.popFrame()
	in.Host = old
	return strings.Join(ch.lines, " ")
}

// ---------------------------------------------------------------- lists

// splitList cuts a macro body into statements at %; and %| .
func splitList(text string) []string {
	var out []string
	st := 0
	pipeNext := false
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '\\':
			i++
		case '$':
			// a $( ... ) command substitution is a list of its own: its %; does not end this statement
			if i+1 < len(text) && text[i+1] == '(' {
				if end := matchCloseCmd(text, i+1); end >= 0 {
					i = end
				}
			}
		case '%':
			if i+1 < len(text) {
				if text[i+1] == '%' {
					// a run of n percent signs is one escaped (n-1)-sign text: none of them ends a statement
					j := i
					for j < len(text) && text[j] == '%' {
						j++
					}
					i = j - 1
					continue
				}
				switch text[i+1] {
				case ';', '|':
					out = append(out, strings.TrimRight(text[st:i], " \t"))
					if text[i+1] == '|' {
						pipeNext = true
					}
					st = i + 2
					i++
					if pipeNext {
						pipeNext = false
						text = text[:st] + "\x00" + text[st:] // marks a statement fed by a pipe
					}
				case '%':
					i++
				}
			}
		}
	}
	return append(out, text[st:])
}

// Exec runs a typed or loaded line at TF's "keyword" substitution level:
// nothing is substituted and %; does not split the line, except after a
// leading /if or /while (whose bodies are lists).
func (in *Interp) Exec(text string) Value {
	defer in.enter()()
	t := strings.TrimLeft(text, " \t")
	old := in.noExp
	in.noExp = true
	defer func() { in.noExp = old }()
	switch stmtWord(t) {
	case "/if", "/while", "/break", "/else", "/elseif", "/endif", "/done", "/then", "/do":
		return in.execText(t)
	}
	if t == "" {
		return Int(1)
	}
	return in.execStmts([]string{t}) // one statement, unsplit
}

// ExecFull runs text as a macro body with full substitution, like /eval
// (and like a typed line when %sub is "full").
func (in *Interp) ExecFull(text string) Value {
	defer in.enter()()
	old := in.noExp
	in.noExp = false
	defer func() { in.noExp = old }()
	return in.execText(text)
}

func (in *Interp) execText(text string) Value {
	c := in.split(text)
	if len(c.diags) > 0 {
		for _, d := range c.diags {
			in.warn("%s", d)
		}
		if c.failed {
			return in.leaf(Int(0))
		}
	}
	return in.execStmts(c.stmts)
}

// split is splitList plus the structural check, with a per-interpreter
// cache; the result must not be modified (execStmts copies before it does).
func (in *Interp) split(text string) *compiled {
	if c, ok := in.stmtCache[text]; ok {
		return c
	}
	c := in.compileStmts(splitList(text), false)
	if len(in.stmtCache) >= cacheMax {
		in.stmtCache = map[string]*compiled{}
	}
	in.stmtCache[text] = c
	return c
}

func firstWord(s string) (w, rest string) {
	s = strings.TrimLeft(s, " \t")
	i := 0
	for i < len(s) && s[i] != ' ' && s[i] != '\t' {
		i++
	}
	return s[:i], strings.TrimLeft(s[i:], " \t")
}

func (in *Interp) execStmts(stmts []string) Value {
	res := Int(1)
	copied, fed := false, false
	for i := 0; i < len(stmts); i++ {
		if in.ctl != ctlNone || !in.tick() {
			break
		}
		st := strings.TrimLeft(strings.TrimPrefix(stmts[i], "\x00"), " \t\r\n")
		if st == "" {
			continue
		}
		if stmts[i] != st {
			if !copied {
				stmts = append([]string(nil), stmts...)
				copied = true
			}
			stmts[i] = st // marker (if any) consumed
		}
		if st == blankStmt {
			res = in.leaf(in.sendLine(""))
			continue
		}
		var pipeHost *captureHost
		if i+1 < len(stmts) && strings.HasPrefix(stmts[i+1], "\x00") {
			pipeHost = &captureHost{Host: in.Host}
			in.Host = pipeHost
		}
		var w string
		if st[0] == '/' {
			w, _ = firstWord(st)
			w = strings.ToLower(w)
			if p := strings.IndexByte(w, '('); p > 0 && (w[:p] == "/if" || w[:p] == "/while" || w[:p] == "/elseif") {
				w = w[:p]
			}
		}
		switch w {
		case "/if":
			res, i = in.execIf(stmts, i)
		case "/while":
			res, i = in.execWhile(stmts, i)
		case "/else", "/elseif", "/endif", "/done", "/then", "/do":
			in.warn("%s without matching /if or /while", w)
		default:
			res = in.execOne(st)
		}
		in.lastRet = res
		if fed {
			in.tfin, fed = nil, false // what the left side of a pipe wrote lives only while its right side runs
		}
		if pipeHost != nil {
			in.Host = pipeHost.Host
			in.tfin, fed = pipeHost.lines, true
		}
	}
	return res
}

func (in *Interp) condition(expr string) bool {
	v, err := in.evalExpr(expr)
	if err != nil {
		in.warn("%v", err)
		return false
	}
	return v.Truth()
}

// parenExpr takes a leading "( ... )" off s.
func parenExpr(s string) (expr, rest string, ok bool) {
	s = strings.TrimLeft(s, " \t")
	if !strings.HasPrefix(s, "(") {
		return "", s, false
	}
	end := matchClose(s, 0, '(', ')')
	if end < 0 {
		return "", s, false
	}
	return s[1:end], strings.TrimLeft(s[end+1:], " \t"), true
}

type branch struct {
	cond     string
	condList []string
	isExpr   bool
	body     []string
	hasCond  bool
	inBody   bool
}

func stmtWord(st string) string {
	w, _ := firstWord(st)
	w = strings.ToLower(w)
	if p := strings.IndexByte(w, '('); p > 0 {
		w = w[:p]
	}
	return w
}

func (in *Interp) execIf(stmts []string, i int) (Value, int) {
	var brs []branch
	cur := branch{hasCond: true}
	rest := strings.TrimLeft(stmts[i], " \t")[len("/if"):]
	depth := 0
	// a body that begins on the same statement as its condition may itself
	// open an /if whose /endif must not close this one
	inline := func(r string) {
		if stmtWord(r) == "/if" {
			depth++
		}
		cur.body = append(cur.body, r)
	}
	start := func(rest string) {
		if e, r, ok := parenExpr(rest); ok {
			cur.cond, cur.isExpr = e, true
			if r != "" {
				inline(r)
			}
		} else if strings.TrimSpace(rest) != "" {
			cur.condList = append(cur.condList, rest)
		}
	}
	start(rest)
	end := len(stmts) - 1
	for j := i + 1; j < len(stmts); j++ {
		st := strings.TrimLeft(stmts[j], " \t")
		w := stmtWord(st)
		switch {
		case w == "/if":
			depth++
		case w == "/endif" && depth > 0:
			depth--
		case w == "/endif":
			brs = append(brs, cur)
			end = j
			goto run
		case depth == 0 && (w == "/elseif" || w == "/else"):
			brs = append(brs, cur)
			cur = branch{hasCond: w == "/elseif"}
			_, r := firstWord(st)
			if w == "/elseif" {
				start(r)
			} else if r != "" {
				inline(r)
			}
			continue
		case depth == 0 && w == "/then":
			cur.inBody = true
			_, r := firstWord(st)
			if r != "" {
				inline(r)
			}
			continue
		}
		if cur.hasCond && !cur.isExpr && !cur.inBody {
			// still collecting a list-form condition until /then
			cur.condList = append(cur.condList, st)
			continue
		}
		cur.body = append(cur.body, st)
	}
	brs = append(brs, cur)
run:
	res := Int(0)
	for _, b := range brs {
		if b.hasCond {
			var ok bool
			if b.isExpr {
				ok = in.condition(b.cond)
			} else {
				ok = in.execStmts(b.condList).Truth()
			}
			if !ok {
				continue
			}
		}
		res = in.execStmts(b.body)
		break
	}
	return res, end
}

func (in *Interp) execWhile(stmts []string, i int) (Value, int) {
	st := strings.TrimLeft(stmts[i], " \t")
	rest := st[len("/while"):]
	var cond string
	var condList, body []string
	isExpr := false
	depth0 := 0
	if e, r, ok := parenExpr(rest); ok {
		cond, isExpr = e, true
		if r != "" {
			if stmtWord(r) == "/while" {
				depth0++
			}
			body = append(body, r)
		}
	} else if strings.TrimSpace(rest) != "" {
		condList = append(condList, rest)
	}
	depth, end := depth0, len(stmts)-1
	inBody := isExpr
	for j := i + 1; j < len(stmts); j++ {
		s2 := strings.TrimLeft(stmts[j], " \t")
		w := stmtWord(s2)
		switch {
		case w == "/while":
			depth++
		case w == "/done" && depth > 0:
			depth--
		case w == "/done":
			end = j
			goto run
		case depth == 0 && (w == "/do"):
			inBody = true
			_, r := firstWord(s2)
			if r != "" {
				body = append(body, r)
			}
			continue
		}
		if inBody {
			body = append(body, s2)
		} else {
			condList = append(condList, s2)
		}
	}
run:
	res := Int(0)
	// %max_iter is documented but unused in tf; runaway loops end at max_instr
	for {
		var ok bool
		if isExpr {
			ok = in.condition(cond)
		} else {
			ok = in.execStmts(condList).Truth()
		}
		if !ok {
			break
		}
		res = in.execStmts(body)
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
	return res, end
}

// execOne runs a single statement: a command or text for the current world.
func (in *Interp) execOne(st string) Value {
	if st[0] != '/' {
		line := in.expand(st)
		if in.subFail {
			return in.abortStmt()
		}
		return in.leaf(in.sendLine(line))
	}
	w, _ := firstWord(st)
	lw := strings.ToLower(w)
	if rawCmds[strings.TrimLeft(lw, "/!@")] {
		neg := strings.HasPrefix(w, "/!")
		name := strings.TrimLeft(lw, "/!@")
		_, args := firstWord(st)
		return negate(in.leaf(in.runBuiltin(name, args)), neg)
	}
	line := in.expand(st)
	if in.subFail {
		return in.abortStmt()
	}
	if strings.HasPrefix(line, "//") { // "//" sends a line starting with "/"
		return in.leaf(in.sendLine(line[1:]))
	}
	if len(line) == 0 || line[0] != '/' {
		return in.leaf(in.sendLine(line))
	}
	return in.dispatchLine(line, func() bool {
		// tf decides "no value" from the unexpanded text: "/set x %*" has a value
		// part even when it expands to nothing, but a typed "/set x " has none.
		_, rawArgs := firstWord(st)
		rawArgs = strings.TrimLeft(rawArgs, " \t")
		rawArgs = rawArgs[varNameEnd(rawArgs):]
		return strings.TrimLeft(rawArgs, " \t") != ""
	})
}

// dispatchLine runs an already substituted command line ("/name args"):
// the user's macro of that name, else the builtin command. rawSet is asked
// (only for /set, /let and /setenv) whether the unexpanded line had a value
// part.
func (in *Interp) dispatchLine(line string, rawSet func() bool) Value {
	nm, args := firstWord(line)
	nm = nm[1:]
	neg := false
	if strings.HasPrefix(nm, "!") {
		neg, nm = true, nm[1:]
	}
	forceBuiltin := false
	if strings.HasPrefix(nm, "@") {
		forceBuiltin, nm = true, nm[1:]
	}
	var res Value
	if m, ok := in.findMacro(nm); ok && !forceBuiltin {
		// a macro's arguments never end in blanks (tf's argument vector is cut at the last word)
		res = in.callMacro(m, strings.TrimRight(args, " \t"), false)
	} else if _, ok := in.cmds[strings.ToLower(nm)]; ok && !in.nativeHidden(strings.ToLower(nm)) {
		ln := strings.ToLower(nm)
		in.rawSetValue = false
		if ln == "set" || ln == "let" || ln == "setenv" {
			in.rawSetValue = rawSet()
		}
		res = in.runBuiltin(ln, args)
		in.rawSetValue = false
		if ln != "set" && ln != "let" {
			in.userRes = res
		}
	} else if forceBuiltin {
		in.warnEval("%s: not a builtin command", nm)
		res = in.leaf(Int(0))
	} else {
		in.noMacro(nm)
		res = in.leaf(Int(0))
	}
	return negate(res, neg)
}

// leaf records the value of a command as the macro's result (tf's user_result).
func (in *Interp) leaf(v Value) Value {
	in.userRes = v
	return v
}

// abortStmt ends the running macro after a failed substitution, as tf does.
func (in *Interp) abortStmt() Value {
	in.subFail = false
	in.retv = Int(0)
	in.ctl = ctlReturn
	return Int(0)
}

func negate(v Value, neg bool) Value {
	if neg {
		return Bool(!v.Truth())
	}
	return v
}

func (in *Interp) runBuiltin(name, args string) Value {
	if in.disabled["/"+name] {
		in.Warn("/" + name + ": disabled")
		return Int(0)
	}
	f, ok := in.cmds[name]
	if !ok {
		in.warn("/%s: no such command", name)
		return Int(0)
	}
	if !quietCmds[name] && tfCmdSet[name] { // commands that only run an expression or a body show no name
		old := in.curCmd
		in.curCmd = upperCmd(name)
		v := f(in, args)
		in.curCmd = old
		return v
	}
	return f(in, args)
}

// callMacroArgs calls a macro with function syntax: each argument is one
// positional parameter, whatever spaces it contains.
func (in *Interp) callMacroArgs(m *Macro, args []string) Value {
	in.pendingArgs = args
	defer func() { in.pendingArgs = nil }()
	return in.callMacro(m, strings.Join(args, " "), true)
}

// callMacro runs a macro body in a new scope with the given arguments.
func (in *Interp) callMacro(m *Macro, args string, asFunc bool) Value {
	if in.depth >= in.maxRecur() {
		in.warn("recursion count exceeded %%max_recur (%d)", in.maxRecur())
		return Int(0)
	}
	if m.ID != 0 || m.native == nil { // a native run as a bare command is no macro
		in.noteCall(m)
	}
	in.depth++
	oldCmd := in.curCmd
	in.curCmd = m.Name
	f := in.pushFrame(m.Name, args)
	f.asFunc = asFunc
	if in.pendingArgs != nil {
		f.words, f.offs = nil, nil
		pos := 0
		for _, a := range in.pendingArgs {
			f.words = append(f.words, a)
			f.offs = append(f.offs, pos)
			pos += len(a) + 1
		}
		in.pendingArgs = nil
	}
	oldExp := in.noExp
	in.noExp = false
	var savedRes Value
	if asFunc {
		savedRes = in.userRes
		in.userRes = Value{k: kStr}
	}
	var res Value
	if m.native != nil {
		res = m.native(in, args)
	} else {
		res = in.execText(m.Body)
	}
	in.noExp = oldExp
	if in.ctl == ctlReturn {
		res = in.retv
	} else if asFunc {
		res = in.userRes
	}
	if asFunc {
		in.userRes = savedRes
	}
	if in.ctl != ctlExit {
		in.ctl = ctlNone
	}
	// regmatch() captures are global in tf: a macro's %P1... outlive it
	if f.regs != nil && f.parent != nil {
		f.parent.regs, f.parent.pl, f.parent.pr = f.regs, f.pl, f.pr
	}
	in.popFrame()
	in.depth--
	in.curCmd = oldCmd
	return res
}

// quietCmds run user text or expressions: tf does not name them in diagnostics.
var quietCmds = map[string]bool{"test": true, "eval": true, "return": true, "result": true, "set": true, "let": true, "setenv": true}

func (in *Interp) callFunc(name string, args []Value) (Value, error) {
	_, isTf := funcArity[name]
	return in.callFuncTf(name, args, isTf)
}

// callFuncTf is callFunc when the caller already knows whether name is one of
// tf's own builtin functions (whose diagnostics carry the function name).
func (in *Interp) callFuncTf(name string, args []Value, isTf bool) (Value, error) {
	if f, ok := in.funcs[name]; ok {
		if len(in.disabled) > 0 && in.disabled[name+"()"] {
			return Int(0), &cmdErr{name, fmt.Errorf("disabled")}
		}
		old := in.curCmd
		in.fnParent = old
		if isTf {
			in.curCmd = name
		}
		v, err := f(in, args)
		in.curCmd = old
		if err != nil && isTf {
			if _, ok := err.(*cmdErr); !ok {
				msg := strings.TrimPrefix(err.Error(), name+"(): ")
				err = &cmdErr{name, errors.New(msg)}
			}
		}
		return v, err
	}
	if ln := cmdFuncName(name); ln != "" && ln != "exit" {
		// a builtin command called as a function takes 0 or 1 argument: its
		// command line (a user macro of the same name is not consulted)
		if _, ok := in.cmds[ln]; ok {
			arg := ""
			if len(args) > 0 {
				arg = args[0].String()
			}
			if in.disabled["/"+ln] {
				return in.runBuiltin(ln, arg), nil
			}
			in.asFuncCmd++ // no command name in diagnostics: tf only sets it for a command line
			defer func() { in.asFuncCmd-- }()
			return in.cmds[ln](in, arg), nil
		}
	}
	if m, ok := in.findMacro(name); ok {
		strs := make([]string, len(args))
		for i, a := range args {
			strs[i] = a.String()
		}
		return in.callMacroArgs(m, strs), nil
	}
	if fn := nativeLib[name]; fn != nil && !in.nativeHidden(name) { // a library command: callable like its macro
		strs := make([]string, len(args))
		for i, a := range args {
			strs[i] = a.String()
		}
		return in.callMacroArgs(&Macro{Name: name, native: fn}, strs), nil
	}
	return Int(0), fmt.Errorf("%s: no such function", name)
}

// echo writes to tfout unless a scope in the chain has closed it.
func (in *Interp) echo(text string) {
	for f := in.fr; f != nil; f = f.parent {
		if f.noOut {
			return
		}
	}
	in.Host.Echo(text)
}

// echoOpts is echo with /echo's options: a host that implements EchoHost
// receives them as given; any other host gets the text with the attributes
// applied (see echoText).
func (in *Interp) echoOpts(o EchoOpts, text string) Value {
	if o == (EchoOpts{}) {
		in.echo(text)
		return Int(1)
	}
	if h, ok := in.Host.(EchoHost); ok {
		for f := in.fr; f != nil; f = f.parent {
			if f.noOut {
				return Int(1)
			}
		}
		h.EchoTo(o, text)
		return Int(1)
	}
	return in.echoText(text, o.Attrs, o.Markup, "")
}

func environ(skip bool) []string {
	if skip {
		return nil
	}
	return os.Environ()
}

// enter marks a top-level entry into the interpreter: the statement budget is
// per top-level execution, and an abort (/exit, interrupt, limits) ends there.
func (in *Interp) enter() func() {
	if in.nest == 0 {
		in.instr = 0
	}
	in.nest++
	return func() {
		in.nest--
		if in.nest == 0 && (in.ctl == ctlExit || in.ctl == ctlReturn) {
			in.ctl = ctlNone
		}
	}
}

var braceSelRE = regexp.MustCompile(`(?s)^(?:#|\?|\*(?:-.*)?|-?[0-9]+(?:-.*)?|-?[LP][0-9]*(?:-.*)?|[A-Za-z_][A-Za-z0-9_]*(?:-.*)?)$`)

// validBraceSel reports whether the inside of %{...} is something tf's
// varsub() accepts; anything else is a "bad substitution" error.
func validBraceSel(sel string) bool {
	if !braceSelRE.MatchString(sel) {
		return false
	}
	return !strings.HasPrefix(sel, "-0") || len(sel) > 2 && sel[2] >= '0' && sel[2] <= '9'
}

func braceSelError(sel string) string {
	if strings.HasPrefix(sel, "-0") || (strings.HasPrefix(sel, "-") && (len(sel) == 1 || !(sel[1] >= '1' && sel[1] <= '9' || sel[1] == 'L'))) {
		return "illegal character '-' in substitution"
	}
	i := 0
	for i < len(sel) && (isIdentChar(sel[i]) || (i == 0 && (sel[i] == '-' || sel[i] == '*' || sel[i] == '#' || sel[i] == '?'))) {
		i++
	}
	if i >= len(sel) {
		return "unmatched { or bad substitution"
	}
	return fmt.Sprintf("unmatched { or illegal character '%c'", sel[i])
}
