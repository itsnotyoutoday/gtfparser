package pcre

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type kind uint8

const (
	kEmpty kind = iota
	kLit
	kAny // any char (n.flag: dotall)
	kClass
	kSeq
	kAlt
	kRepeat
	kGroup  // capturing group n
	kAtomic // (?>...)
	kLook   // lookaround
	kBackref
	kBol
	kEol
	kBeginText // \A
	kEndText   // \z
	kEndTextNL // \Z
	kWordB
	kNotWordB
	kStartMatch // \G
	kKeep       // \K
	kCond
	kFail
	kGrapheme // \X
	kAnyByte  // \C
)

const (
	modeGreedy = iota
	modeLazy
	modePossessive
)

type node struct {
	k        kind
	r        rune
	fold     bool
	flag     bool // dotall (kAny), multiline (kBol/kEol), dollar-endonly (kEol)
	flag2    bool
	set      *charSet
	subs     []*node
	min, max int // max<0: unbounded
	mode     int
	n        int // group / backref number
	name     string
	behind   bool
	neg      bool
	cond     *node // kCond: assertion node, or nil
	condN    int   // kCond group test (when cond==nil)
	condName string
	ref      *node // kCond with a named test: backref node resolved after parsing
	off      int
}

type flags struct{ i, s, m, x, xx, n, U bool }

type parser struct {
	src       string
	pos       int
	opt       Options
	fl        flags
	ncap      int
	names     []string // index = group number
	nameIdx   map[string]int
	dupNames  bool
	total     int
	ambiguous bool
	refs      []*node
	look      int // lookaround nesting depth
	kInLook   bool
	inQuote   bool    // inside \Q..\E within a character class
	lateErr   error   // error reported only after references are resolved
	pend      []*node // literal nodes emitted ahead of the current atom (\Q..\E)
}

func (p *parser) errAt(off int, msg string) error { return &Error{Offset: off, Msg: msg} }
func (p *parser) err(msg string) error            { return &Error{Offset: p.pos, Msg: msg} }

func (p *parser) capNames() []string {
	out := make([]string, p.ncap+1)
	copy(out, p.names)
	return out
}

func (p *parser) parseTop() (*node, error) {
	p.fl = flags{i: p.opt.Caseless, s: p.opt.DotAll, m: p.opt.Multiline, x: p.opt.Extended}
	p.names = []string{""}
	p.nameIdx = map[string]int{}
	n, err := p.parseAlt(false)
	if err != nil {
		return nil, err
	}
	if p.pos < len(p.src) { // must be ')'
		return nil, p.errAt(p.pos+1, "unmatched closing parenthesis")
	}
	if p.kInLook {
		return nil, p.errAt(len(p.src), "\\K is not allowed in lookarounds (but see PCRE2_EXTRA_ALLOW_LOOKAROUND_BSK)")
	}
	return n, nil
}

// resolve checks backreferences and named references after parsing.
func (p *parser) resolve(root *node) error {
	for _, n := range p.refs {
		if n.name != "" {
			idx, ok := p.nameIdx[n.name]
			if !ok {
				return p.errAt(n.off, "reference to non-existent subpattern")
			}
			n.n = idx
		}
		if n.n > p.ncap || n.n < 1 {
			return p.errAt(n.off, "reference to non-existent subpattern")
		}
	}
	return p.lateErr
}

func (p *parser) peek() byte {
	if p.pos < len(p.src) {
		return p.src[p.pos]
	}
	return 0
}

func (p *parser) more() bool { return p.pos < len(p.src) }

func (p *parser) nextRune() rune {
	if p.opt.UTF {
		r, w := utf8.DecodeRuneInString(p.src[p.pos:])
		p.pos += w
		return r
	}
	r := rune(p.src[p.pos])
	p.pos++
	return r
}

// skipX skips (?#...) comments and, in extended mode, whitespace and # comments.
func (p *parser) skipX() error {
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		switch {
		case c == '(' && strings.HasPrefix(p.src[p.pos:], "(?#"):
			end := strings.IndexByte(p.src[p.pos:], ')')
			if end < 0 {
				return p.errAt(len(p.src), "missing ) after (?# comment")
			}
			p.pos += end + 1
		case c == '\\' && strings.HasPrefix(p.src[p.pos:], "\\E"):
			p.pos += 2
		case c == '\\' && strings.HasPrefix(p.src[p.pos:], "\\Q\\E"):
			p.pos += 4
		case p.fl.x && (c == ' ' || (c >= 9 && c <= 13)):
			p.pos++
		case p.fl.x && c == '#':
			for p.pos < len(p.src) && p.src[p.pos] != '\n' {
				p.pos++
			}
		default:
			return nil
		}
	}
	return nil
}

func (p *parser) parseAlt(reset bool) (*node, error) {
	var branches []*node
	startCap, maxCap := p.ncap, p.ncap
	for {
		seq, err := p.parseSeq()
		if err != nil {
			return nil, err
		}
		branches = append(branches, seq)
		if p.peek() == '|' && p.more() {
			p.pos++
			if reset {
				if p.ncap > maxCap {
					maxCap = p.ncap
				}
				p.ncap = startCap
			}
			continue
		}
		break
	}
	if reset && p.ncap < maxCap {
		p.ncap = maxCap
	}
	if len(branches) == 1 {
		return branches[0], nil
	}
	return &node{k: kAlt, subs: branches}, nil
}

func (p *parser) parseSeq() (*node, error) {
	var items []*node
	for {
		if err := p.skipX(); err != nil {
			return nil, err
		}
		if !p.more() {
			break
		}
		c := p.src[p.pos]
		if c == '|' || c == ')' {
			break
		}
		atom, err := p.parseAtom()
		if err != nil {
			return nil, err
		}
		items = append(items, p.pend...)
		p.pend = nil
		if err := p.skipX(); err != nil {
			return nil, err
		}
		min, max, ok, err := p.parseQuant()
		if err != nil {
			return nil, err
		}
		if ok {
			if atom == nil || !repeatable(atom) {
				return nil, p.err("quantifier does not follow a repeatable item")
			}
			if err := p.skipX(); err != nil {
				return nil, err
			}
			mode := modeGreedy
			if p.more() && p.src[p.pos] == '?' {
				mode = modeLazy
				p.pos++
			} else if p.more() && p.src[p.pos] == '+' {
				mode = modePossessive
				p.pos++
			}
			if p.fl.U && mode != modePossessive {
				mode = modeGreedy + modeLazy - mode
			}
			atom = &node{k: kRepeat, subs: []*node{atom}, min: min, max: max, mode: mode}
			if err := p.skipX(); err != nil {
				return nil, err
			}
			if _, _, again, err := p.parseQuant(); err != nil {
				return nil, err
			} else if again {
				return nil, p.err("quantifier does not follow a repeatable item")
			}
		}
		if atom != nil {
			items = append(items, atom)
		}
	}
	switch len(items) {
	case 0:
		return &node{k: kEmpty}, nil
	case 1:
		return items[0], nil
	}
	return &node{k: kSeq, subs: items}, nil
}

// parseQuant parses a quantifier at p.pos.
func (p *parser) parseQuant() (min, max int, ok bool, err error) {
	if !p.more() {
		return
	}
	switch p.src[p.pos] {
	case '*':
		p.pos++
		return 0, -1, true, nil
	case '+':
		p.pos++
		return 1, -1, true, nil
	case '?':
		p.pos++
		return 0, 1, true, nil
	case '{':
		save := p.pos
		min, max, good, e := p.braceQuant()
		if e != nil {
			return 0, 0, false, e
		}
		if !good {
			p.pos = save
			return 0, 0, false, nil
		}
		return min, max, true, nil
	}
	return
}

// braceQuant parses {n} {n,} {n,m} {,m}; good=false means it is a literal brace.
func (p *parser) braceQuant() (min, max int, good bool, err error) {
	i := p.pos + 1
	s := p.src
	skip := func() {
		for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
			i++
		}
	}
	num := func() (int, bool, bool) { // value, present, tooBig
		st := i
		v := 0
		big := false
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			v = v*10 + int(s[i]-'0')
			if v > 65535 {
				big = true
				v = 65536
			}
			i++
		}
		return v, i > st, big
	}
	skip()
	lo, haveLo, big1 := num()
	skip()
	hi := lo
	haveHi := haveLo
	big2 := false
	comma := false
	if i < len(s) && s[i] == ',' {
		comma = true
		i++
		skip()
		hi, haveHi, big2 = num()
		skip()
	}
	if i >= len(s) || s[i] != '}' || (!haveLo && !haveHi) {
		return 0, 0, false, nil
	}
	if !haveLo && !comma {
		return 0, 0, false, nil
	}
	i++
	if big1 || big2 {
		return 0, 0, false, p.errAt(i-1, "number too big in {} quantifier")
	}
	if !haveLo {
		lo = 0
	}
	if comma && !haveHi {
		hi = -1
	}
	if hi >= 0 && lo > hi {
		return 0, 0, false, p.errAt(i-1, "numbers out of order in {} quantifier")
	}
	p.pos = i
	return lo, hi, true, nil
}

// braceIsQuant reports whether a valid {n,m} quantifier starts at p.pos.
func (p *parser) braceIsQuant() bool {
	save := p.pos
	_, _, good, err := p.braceQuant()
	p.pos = save
	return err == nil && good
}

func (p *parser) lit(r rune) *node {
	n := &node{k: kLit, r: r, fold: p.fl.i}
	if n.fold && len(foldOrbit(r, p.opt.UTF)) == 1 {
		n.fold = false
	}
	return n
}

func (p *parser) newSet() *charSet {
	return &charSet{fold: p.fl.i, utf: p.opt.UTF}
}

func (p *parser) classNode(cs *charSet) *node {
	cs.finalize()
	return &node{k: kClass, set: cs}
}

func (p *parser) parseAtom() (*node, error) {
	start := p.pos
	c := p.src[p.pos]
	switch c {
	case '(':
		return p.parseGroup()
	case '[':
		if end, ok := p.posixLike(); ok {
			p.pos = end
			return nil, p.err("POSIX named classes are supported only within a class")
		}
		return p.parseClass()
	case '.':
		p.pos++
		return &node{k: kAny, flag: p.fl.s}, nil
	case '^':
		p.pos++
		return &node{k: kBol, flag: p.fl.m}, nil
	case '$':
		p.pos++
		return &node{k: kEol, flag: p.fl.m, flag2: p.opt.DollarEndOnly}, nil
	case '\\':
		p.pos++
		return p.parseEscape(start)
	case '*', '+', '?':
		p.pos++
		return nil, p.err("quantifier does not follow a repeatable item")
	case '{':
		save := p.pos
		_, _, good, err := p.braceQuant()
		if err != nil {
			return nil, err
		}
		if good {
			return nil, p.err("quantifier does not follow a repeatable item")
		}
		p.pos = save
		p.pos++
		return p.lit('{'), nil
	}
	return p.lit(p.nextRune()), nil
}

func repeatable(n *node) bool {
	switch n.k {
	case kBol, kEol, kBeginText, kEndText, kEndTextNL, kWordB, kNotWordB, kStartMatch, kKeep, kFail:
		return false
	}
	return true
}

func isNameStart(r rune) bool { return r == '_' || unicode.IsLetter(r) }
func isNameChar(r rune) bool  { return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) }

// parseName reads a group name terminated by term.
func (p *parser) parseName(term byte) (string, error) {
	start := p.pos
	if !p.more() {
		return "", p.err("subpattern name expected")
	}
	first := true
	for p.more() && p.src[p.pos] != term {
		save := p.pos
		r := p.nextRune()
		if first && !isNameStart(r) {
			if unicode.IsDigit(r) {
				return "", p.err("subpattern name must start with a non-digit")
			}
			p.pos = save
			return "", p.err("subpattern name expected")
		}
		if !isNameChar(r) {
			p.pos = save
			return "", p.err("syntax error in subpattern name (missing terminator?)")
		}
		first = false
	}
	if !p.more() {
		return "", p.err("syntax error in subpattern name (missing terminator?)")
	}
	if first {
		return "", p.err("subpattern name expected")
	}
	name := p.src[start:p.pos]
	p.pos++
	if len(name) > 128 {
		return "", p.err("subpattern name is too long (maximum 32 code units)")
	}
	return name, nil
}

func (p *parser) closeParen() error {
	if !p.more() || p.src[p.pos] != ')' {
		return p.err("missing closing parenthesis")
	}
	p.pos++
	return nil
}

// body parses an alternation up to the closing paren with flags scoped to it.
func (p *parser) body(reset bool) (*node, error) {
	saved := p.fl
	n, err := p.parseAlt(reset)
	if err != nil {
		return nil, err
	}
	if err := p.closeParen(); err != nil {
		return nil, err
	}
	p.fl = saved
	return n, nil
}

func (p *parser) lookBody(reset bool) (*node, error) {
	p.look++
	n, err := p.body(reset)
	p.look--
	return n, err
}

func (p *parser) newGroup(name string) (int, error) {
	p.ncap++
	idx := p.ncap
	for len(p.names) <= idx {
		p.names = append(p.names, "")
	}
	if name != "" {
		if prev, dup := p.nameIdx[name]; dup && prev != idx && !p.dupNames {
			return 0, p.err("two named subpatterns have the same name (PCRE2_DUPNAMES not set)")
		}
		if _, dup := p.nameIdx[name]; !dup {
			p.nameIdx[name] = idx
		}
		p.names[idx] = name
	}
	return idx, nil
}

func (p *parser) capGroup(name string, off int) (*node, error) {
	idx, err := p.newGroup(name)
	if err != nil {
		return nil, err
	}
	sub, err := p.body(false)
	if err != nil {
		return nil, err
	}
	return &node{k: kGroup, n: idx, subs: []*node{sub}}, nil
}

func (p *parser) parseGroup() (*node, error) {
	start := p.pos
	p.pos++ // (
	if p.peek() == '*' && p.more() {
		return p.parseVerb(start)
	}
	if p.peek() != '?' || !p.more() {
		if p.fl.n {
			sub, err := p.body(false)
			return nonNil(sub), err
		}
		return p.capGroup("", start)
	}
	p.pos++ // ?
	if !p.more() {
		return nil, p.err("unrecognized character after (? or (?-")
	}
	c := p.src[p.pos]
	switch c {
	case '#':
		for p.more() && p.src[p.pos] != ')' {
			p.pos++
		}
		if !p.more() {
			return nil, p.err("missing ) after (?# comment")
		}
		p.pos++
		return nil, nil
	case ':':
		p.pos++
		sub, err := p.body(false)
		return nonNil(sub), err
	case '|':
		p.pos++
		sub, err := p.body(true)
		return nonNil(sub), err
	case '>':
		p.pos++
		sub, err := p.body(false)
		if err != nil {
			return nil, err
		}
		return &node{k: kAtomic, subs: []*node{sub}}, nil
	case '=', '!':
		p.pos++
		sub, err := p.lookBody(false)
		if err != nil {
			return nil, err
		}
		return &node{k: kLook, neg: c == '!', subs: []*node{sub}, off: start}, nil
	case '<':
		if p.pos+1 < len(p.src) && (p.src[p.pos+1] == '=' || p.src[p.pos+1] == '!') {
			neg := p.src[p.pos+1] == '!'
			p.pos += 2
			sub, err := p.lookBody(false)
			if err != nil {
				return nil, err
			}
			return &node{k: kLook, behind: true, neg: neg, subs: []*node{sub}, off: start}, nil
		}
		p.pos++
		name, err := p.parseName('>')
		if err != nil {
			return nil, err
		}
		return p.capGroup(name, start)
	case '\'':
		p.pos++
		name, err := p.parseName('\'')
		if err != nil {
			return nil, err
		}
		return p.capGroup(name, start)
	case 'P':
		p.pos++
		switch p.peek() {
		case '<':
			p.pos++
			name, err := p.parseName('>')
			if err != nil {
				return nil, err
			}
			return p.capGroup(name, start)
		case '=':
			p.pos++
			off := p.pos
			name, err := p.parseName(')')
			if err != nil {
				return nil, err
			}
			ref := &node{k: kBackref, name: name, fold: p.fl.i, off: off}
			p.refs = append(p.refs, ref)
			return ref, nil
		case '>':
			return nil, p.err("recursion and subroutine calls are not supported")
		}
		return nil, p.err("unrecognized character after (?P")
	case '(':
		return p.parseCond(start)
	case 'R', '&', '+', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return nil, p.err("recursion and subroutine calls are not supported")
	case '-':
		if p.pos+1 < len(p.src) && p.src[p.pos+1] >= '0' && p.src[p.pos+1] <= '9' {
			return nil, p.err("recursion and subroutine calls are not supported")
		}
	}
	return p.parseFlags()
}

// nonNil turns the body of a non-capturing group into a node; a bare
// assertion is wrapped so that the group, unlike the assertion, may be quantified.
func nonNil(n *node) *node {
	if n == nil {
		return &node{k: kEmpty}
	}
	if !repeatable(n) {
		return &node{k: kSeq, subs: []*node{n}}
	}
	return n
}

// parseFlags handles (?imsx-imsx) and (?imsx-imsx:...).  p.pos is after "(?".
func (p *parser) parseFlags() (*node, error) {
	fl := p.fl
	on := true
	if p.peek() == '^' {
		fl.i, fl.m, fl.s, fl.x, fl.xx, fl.n = false, false, false, false, false, false
		p.pos++
	}
	for p.more() {
		c := p.src[p.pos]
		p.pos++
		switch c {
		case 'i':
			fl.i = on
		case 'm':
			fl.m = on
		case 's':
			fl.s = on
		case 'n':
			fl.n = on
		case 'U':
			fl.U = on
		case 'J':
			p.dupNames = on
		case 'x':
			if p.peek() == 'x' && p.more() {
				p.pos++
				fl.xx = on
			}
			fl.x = on
			if !on {
				fl.xx = false
			}
		case '-':
			if !on {
				return nil, p.err("unrecognized character after (? or (?-")
			}
			on = false
		case ')':
			p.fl = fl
			return nil, nil
		case ':':
			saved := p.fl
			p.fl = fl
			sub, err := p.parseAlt(false)
			if err != nil {
				return nil, err
			}
			if err := p.closeParen(); err != nil {
				return nil, err
			}
			p.fl = saved
			return nonNil(sub), nil
		default:
			return nil, p.err("unrecognized character after (? or (?-")
		}
	}
	return nil, p.err("missing closing parenthesis")
}

func (p *parser) parseVerb(start int) (*node, error) {
	p.pos++ // *
	end := strings.IndexByte(p.src[p.pos:], ')')
	if end < 0 {
		return nil, p.err("(*VERB) not recognized or malformed")
	}
	verb := p.src[p.pos : p.pos+end]
	p.pos += end + 1
	switch verb {
	case "FAIL", "F":
		return &node{k: kFail}, nil
	}
	return nil, p.errAt(p.pos-1, "(*VERB) not recognized or malformed")
}

func (p *parser) parseCond(start int) (*node, error) {
	// at "(" of the condition, after "(?"
	cn := &node{k: kCond, off: start}
	condNameOff := 0
	save := p.pos
	p.pos++ // (
	c := p.peek()
	switch {
	case c == '?' && p.more():
		p.pos = save
		a, err := p.parseAtom()
		if err != nil {
			return nil, err
		}
		if a == nil || a.k != kLook {
			return nil, p.errAt(save, "assertion expected after (?( or (?(?C)")
		}
		cn.cond = a
	case c >= '0' && c <= '9' || c == '+' || c == '-':
		sign := byte(0)
		if c == '+' || c == '-' {
			sign = c
			p.pos++
		}
		st := p.pos
		for p.more() && p.src[p.pos] >= '0' && p.src[p.pos] <= '9' {
			p.pos++
		}
		if st == p.pos || p.peek() != ')' {
			return nil, p.err("malformed number or name after (?(")
		}
		v, _ := strconv.Atoi(p.src[st:p.pos])
		p.pos++
		off := save
		switch sign {
		case '-':
			v = p.ncap + 1 - v
			off = p.pos - 1
		case '+':
			v = p.ncap + v
			off = p.pos - 1
		}
		if v < 1 {
			return nil, p.errAt(p.pos-1, "reference to non-existent subpattern")
		}
		cn.ref = &node{k: kBackref, n: v, off: off}
		p.refs = append(p.refs, cn.ref)
	case c == '<' || c == '\'':
		p.pos++
		condNameOff = p.pos
		term := byte('>')
		if c == '\'' {
			term = '\''
		}
		name, err := p.parseName(term)
		if err != nil {
			return nil, err
		}
		if err := p.closeParen(); err != nil {
			return nil, err
		}
		cn.condName = name
	default:
		st := p.pos
		condNameOff = st
		for p.more() && p.src[p.pos] != ')' {
			p.pos++
		}
		if !p.more() {
			return nil, p.err("missing closing parenthesis for condition")
		}
		name := p.src[st:p.pos]
		if name == "" {
			return nil, p.errAt(st, "subpattern name expected")
		}
		p.pos++
		switch {
		case name == "DEFINE":
			cn.condName = "\x00DEFINE"
		case name == "R" || strings.HasPrefix(name, "R&") || (len(name) > 1 && name[0] == 'R' && name[1] >= '0' && name[1] <= '9'):
			return nil, p.errAt(st, "recursion and subroutine calls are not supported")
		case name == "VERSION" || strings.HasPrefix(name, "VERSION"):
			return nil, p.errAt(st, "unsupported condition")
		default:
			cn.condName = name
		}
	}
	saved := p.fl
	body, err := p.parseAlt(false)
	if err != nil {
		return nil, err
	}
	if err := p.closeParen(); err != nil {
		return nil, err
	}
	p.fl = saved
	branches := []*node{body}
	if body.k == kAlt {
		branches = body.subs
	}
	if cn.condName == "\x00DEFINE" {
		return &node{k: kEmpty}, nil
	}
	if len(branches) > 2 && p.lateErr == nil {
		// PCRE2 reports a bad group reference in the condition before this.
		p.lateErr = p.errAt(start, "conditional subpattern contains more than two branches")
	}
	if len(branches) > 2 {
		branches = branches[:2]
	}
	cn.subs = []*node{branches[0], {k: kEmpty}}
	if len(branches) == 2 {
		cn.subs[1] = branches[1]
	}
	if cn.condName != "" {
		cn.ref = &node{k: kBackref, name: cn.condName, off: condNameOff}
		p.refs = append(p.refs, cn.ref)
	}
	return cn, nil
}

// ---- escapes ---------------------------------------------------------

func isOct(c byte) bool { return c >= '0' && c <= '7' }
func hexVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}

func (p *parser) checkCode(v int) error { return p.checkCodeAt(v, p.pos) }

func (p *parser) checkCodeAt(v, off int) error {
	if p.opt.UTF {
		if v > 0x10ffff {
			return p.errAt(off, "character code point value in \\x{} or \\o{} is too large")
		}
		if v >= 0xd800 && v <= 0xdfff {
			return p.errAt(off, "disallowed Unicode code point (>= 0xd800 && <= 0xdfff)")
		}
	} else if v > 255 {
		return p.errAt(off, "character code point value in \\x{} or \\o{} is too large")
	}
	return nil
}

// charEscape handles escapes that denote a single character.  p.pos is just
// after the backslash.  ok is false (position unchanged) if c is not one.
func (p *parser) charEscape(inClass bool) (r rune, ok bool, err error) {
	c := p.src[p.pos]
	switch c {
	case 't':
		p.pos++
		return '\t', true, nil
	case 'n':
		p.pos++
		return '\n', true, nil
	case 'r':
		p.pos++
		return '\r', true, nil
	case 'f':
		p.pos++
		return '\f', true, nil
	case 'e':
		p.pos++
		return 0x1b, true, nil
	case 'a':
		p.pos++
		return 7, true, nil
	case 'c':
		p.pos++
		if !p.more() {
			return 0, false, p.err("\\c at end of pattern")
		}
		x := p.src[p.pos]
		if x < 32 || x > 126 {
			return 0, false, p.err("\\c must be followed by a printable ASCII character")
		}
		p.pos++
		if x >= 'a' && x <= 'z' {
			x -= 32
		}
		return rune(x ^ 0x40), true, nil
	case '0':
		p.pos++
		v := 0
		for k := 0; k < 2 && p.more() && isOct(p.src[p.pos]); k++ {
			v = v*8 + int(p.src[p.pos]-'0')
			p.pos++
		}
		return rune(v), true, nil
	case 'o':
		p.pos++
		if p.peek() != '{' || !p.more() {
			return 0, false, p.err("missing opening brace after \\o")
		}
		p.pos++
		v, st := 0, p.pos
		for p.more() && isOct(p.src[p.pos]) {
			v = v*8 + int(p.src[p.pos]-'0')
			if v > 0x7fffffff {
				return 0, false, p.errAt(p.pos, "character code point value in \\x{} or \\o{} is too large")
			}
			p.pos++
		}
		if err := p.braceEnd(st, "non-octal character in \\o{} (closing brace missing?)"); err != nil {
			return 0, false, err
		}
		if err := p.checkCodeAt(v, p.pos-1); err != nil {
			return 0, false, err
		}
		return rune(v), true, nil
	case 'x':
		p.pos++
		if p.peek() == '{' && p.more() {
			p.pos++
			v, st := 0, p.pos
			for p.more() && hexVal(p.src[p.pos]) >= 0 {
				v = v*16 + hexVal(p.src[p.pos])
				if v > 0x7fffffff {
					return 0, false, p.errAt(p.pos, "character code point value in \\x{} or \\o{} is too large")
				}
				p.pos++
			}
			if err := p.braceEnd(st, "non-hex character in \\x{} (closing brace missing?)"); err != nil {
				return 0, false, err
			}
			if err := p.checkCodeAt(v, p.pos-1); err != nil {
				return 0, false, err
			}
			return rune(v), true, nil
		}
		v, st := 0, p.pos
		for k := 0; k < 2 && p.more() && hexVal(p.src[p.pos]) >= 0; k++ {
			v = v*16 + hexVal(p.src[p.pos])
			p.pos++
		}
		if st == p.pos {
			return 0, false, p.err("digits missing after \\x or in \\x{} or \\o{} or \\N{U+}")
		}
		return rune(v), true, nil
	}
	if inClass && c >= '1' && c <= '7' {
		v := 0
		for k := 0; k < 3 && p.more() && isOct(p.src[p.pos]); k++ {
			v = v*8 + int(p.src[p.pos]-'0')
			p.pos++
		}
		if err := p.checkCode(v); err != nil {
			return 0, false, err
		}
		return rune(v), true, nil
	}
	if c < 0x80 && !(c >= '0' && c <= '9') && !(c >= 'a' && c <= 'z') && !(c >= 'A' && c <= 'Z') {
		p.pos++
		return rune(c), true, nil
	}
	if c >= 0x80 {
		return p.nextRune(), true, nil
	}
	return 0, false, nil
}

// braceEnd finishes a \x{...} or \o{...} whose digits start at st: it
// consumes the closing brace or reports why it is not there.
func (p *parser) braceEnd(st int, badMsg string) error {
	if p.more() && p.src[p.pos] == '}' {
		p.pos++
		if st == p.pos-1 {
			return p.errAt(p.pos-1, "digits missing after \\x or in \\x{} or \\o{} or \\N{U+}")
		}
		return nil
	}
	if end := strings.IndexByte(p.src[p.pos:], '}'); end >= 0 {
		return p.errAt(p.pos+end, badMsg)
	}
	if st == p.pos {
		return p.err("digits missing after \\x or in \\x{} or \\o{} or \\N{U+}")
	}
	return p.err(badMsg)
}

// typeSet builds the class for \d \D \s \S \w \W \h \H \v \V.
func (p *parser) typeSet(c byte, into *charSet) bool {
	lower := c | 0x20
	f := escapeFn(lower, p.opt.UCP)
	if f == nil || (lower != 'd' && lower != 's' && lower != 'w' && lower != 'h' && lower != 'v') {
		return false
	}
	into.addPred(f, c != lower)
	return true
}

func (p *parser) parseProp(neg bool, into *charSet) error {
	if !p.more() {
		return p.errAt(len(p.src), "malformed \\P or \\p sequence")
	}
	var name string
	if p.src[p.pos] == '{' {
		end := strings.IndexByte(p.src[p.pos:], '}')
		if end < 0 {
			return p.errAt(len(p.src), "malformed \\P or \\p sequence")
		}
		name = p.src[p.pos+1 : p.pos+end]
		p.pos += end + 1
	} else {
		c := p.src[p.pos]
		p.pos++
		if !(c >= 'a' && c <= 'z') && !(c >= 'A' && c <= 'Z') {
			return p.err("malformed \\P or \\p sequence")
		}
		name = string(c)
	}
	if strings.HasPrefix(name, "^") {
		neg = !neg
		name = name[1:]
	}
	f, ok := propFn(name, p.fl.i)
	if !ok {
		return p.err("unknown property after \\P or \\p")
	}
	into.addPred(f, neg)
	return nil
}

func (p *parser) parseEscape(start int) (*node, error) {
	if !p.more() {
		return nil, p.err("\\ at end of pattern")
	}
	c := p.src[p.pos]
	switch c {
	case 'd', 'D', 's', 'S', 'w', 'W', 'h', 'H', 'v', 'V':
		p.pos++
		cs := p.newSet()
		p.typeSet(c, cs)
		return p.classNode(cs), nil
	case 'p', 'P':
		p.pos++
		cs := p.newSet()
		if err := p.parseProp(c == 'P', cs); err != nil {
			return nil, err
		}
		return p.classNode(cs), nil
	case 'N':
		p.pos++
		if p.peek() == '{' && p.more() && !p.braceIsQuant() {
			if !p.opt.UTF || !strings.HasPrefix(p.src[p.pos:], "{U+") {
				return nil, p.err("PCRE2 does not support \\F, \\L, \\l, \\N{name}, \\U, or \\u")
			}
			p.pos += 3
			v, st := 0, p.pos
			for p.more() && hexVal(p.src[p.pos]) >= 0 && v <= 0x10ffff {
				v = v*16 + hexVal(p.src[p.pos])
				p.pos++
			}
			if st == p.pos || p.peek() != '}' || !p.more() {
				return nil, p.err("\\N{U+dddd} is supported only in Unicode (UTF) mode")
			}
			p.pos++
			if err := p.checkCodeAt(v, p.pos-1); err != nil {
				return nil, err
			}
			return p.lit(rune(v)), nil
		}
		return &node{k: kAny, flag: false}, nil
	case 'R':
		p.pos++
		return p.newlineSeq(), nil
	case 'X':
		p.pos++
		return &node{k: kGrapheme}, nil
	case 'C':
		p.pos++
		return &node{k: kAnyByte}, nil
	case 'b':
		p.pos++
		return &node{k: kWordB}, nil
	case 'B':
		p.pos++
		return &node{k: kNotWordB}, nil
	case 'A':
		p.pos++
		return &node{k: kBeginText}, nil
	case 'Z':
		p.pos++
		return &node{k: kEndTextNL}, nil
	case 'z':
		p.pos++
		return &node{k: kEndText}, nil
	case 'G':
		p.pos++
		return &node{k: kStartMatch}, nil
	case 'K':
		p.pos++
		if p.look > 0 {
			p.kInLook = true
		}
		return &node{k: kKeep}, nil
	case 'E':
		p.pos++
		return nil, nil
	case 'Q':
		p.pos++
		var items []*node
		for p.more() {
			if strings.HasPrefix(p.src[p.pos:], "\\E") {
				p.pos += 2
				break
			}
			items = append(items, p.lit(p.nextRune()))
		}
		if len(items) == 0 {
			return nil, nil
		}
		// A quantifier after \Q..\E applies to the last character only.
		p.pend = append(p.pend, items[:len(items)-1]...)
		return items[len(items)-1], nil
	case 'g':
		return p.parseG(start)
	case 'k':
		p.pos++
		var term byte
		switch p.peek() {
		case '<':
			term = '>'
		case '\'':
			term = '\''
		case '{':
			term = '}'
		default:
			return nil, p.err("\\k is not followed by a braced, angle-bracketed, or quoted name")
		}
		p.pos++
		off := p.pos
		name, err := p.parseName(term)
		if err != nil {
			return nil, err
		}
		ref := &node{k: kBackref, name: name, fold: p.fl.i, off: off}
		p.refs = append(p.refs, ref)
		return ref, nil
	}
	if c >= '1' && c <= '9' {
		st := p.pos
		v := 0
		for p.more() && p.src[p.pos] >= '0' && p.src[p.pos] <= '9' {
			if v < 100000 {
				v = v*10 + int(p.src[p.pos]-'0')
			}
			p.pos++
		}
		if v < 10 || c >= '8' || p.total < 0 || v <= p.total {
			if v >= 10 {
				p.ambiguous = true
			}
			ref := &node{k: kBackref, n: v, fold: p.fl.i, off: p.pos}
			p.refs = append(p.refs, ref)
			return ref, nil
		}
		p.pos = st
		val := 0
		for k := 0; k < 3 && p.more() && isOct(p.src[p.pos]); k++ {
			val = val*8 + int(p.src[p.pos]-'0')
			p.pos++
		}
		if err := p.checkCode(val); err != nil {
			return nil, err
		}
		return p.lit(rune(val)), nil
	}
	r, ok, err := p.charEscape(false)
	if err != nil {
		return nil, err
	}
	if !ok {
		p.pos++
		if strings.IndexByte("FLlUu", c) >= 0 {
			return nil, p.err("PCRE2 does not support \\F, \\L, \\l, \\N{name}, \\U, or \\u")
		}
		return nil, p.err("unrecognized character follows \\")
	}
	return p.lit(r), nil
}

func (p *parser) parseG(start int) (*node, error) {
	p.pos++ // g
	if !p.more() {
		return nil, p.err("a numbered reference must not be zero")
	}
	switch p.peek() {
	case '<', '\'':
		return nil, p.err("recursion and subroutine calls are not supported")
	}
	braced, bracePos := false, 0
	if p.peek() == '{' {
		braced, bracePos = true, p.pos
		p.pos++
	}
	st := p.pos
	if p.peek() == '-' || p.peek() == '+' {
		p.pos++
	}
	ds := p.pos
	for p.more() && p.src[p.pos] >= '0' && p.src[p.pos] <= '9' {
		p.pos++
	}
	if ds == p.pos {
		if !braced {
			return nil, p.err("a numbered reference must not be zero")
		}
		p.pos = st
		name, err := p.parseName('}')
		if err != nil {
			return nil, err
		}
		ref := &node{k: kBackref, name: name, fold: p.fl.i, off: st}
		p.refs = append(p.refs, ref)
		return ref, nil
	}
	v, _ := strconv.Atoi(p.src[ds:p.pos])
	switch p.src[st] {
	case '-':
		v = p.ncap + 1 - v
		if v <= 0 { // relative reference to before the first group
			if braced {
				return nil, p.errAt(bracePos, "reference to non-existent subpattern")
			}
			return nil, p.err("reference to non-existent subpattern")
		}
	case '+':
		v = p.ncap + v
	}
	if braced {
		if p.peek() != '}' || !p.more() {
			return nil, p.err("\\g is not followed by a braced, angle-bracketed, or quoted name/number or by a plain number")
		}
		p.pos++
	}
	if v <= 0 {
		return nil, p.err("reference to non-existent subpattern")
	}
	ref := &node{k: kBackref, n: v, fold: p.fl.i, off: p.pos}
	p.refs = append(p.refs, ref)
	return ref, nil
}

// newlineSeq builds \R as (?>\r\n|[\n\v\f\r\x85  ]).
func (p *parser) newlineSeq() *node {
	cs := &charSet{utf: p.opt.UTF}
	cs.addPred(isVSpace, false)
	cs.finalize()
	crlf := &node{k: kSeq, subs: []*node{{k: kLit, r: '\r'}, {k: kLit, r: '\n'}}}
	return &node{k: kAtomic, subs: []*node{{k: kAlt, subs: []*node{crlf, {k: kClass, set: cs}}}}}
}

// ---- character classes -----------------------------------------------

var posixNames = map[string]bool{"alpha": true, "lower": true, "upper": true, "alnum": true, "digit": true,
	"xdigit": true, "space": true, "blank": true, "cntrl": true, "graph": true, "print": true,
	"punct": true, "word": true, "ascii": true}

const (
	atomChar  = iota // a single character
	atomClass        // a class escape (\d, \p{..}), already added to the set
	atomNone         // nothing yet (\Q, \E)
)

// posixLike reports whether a "[:name:]" (or [.x.] / [=x=]) sequence starts
// at p.pos, which PCRE2 rejects outside a character class; end is the
// offset just past it.
func (p *parser) posixLike() (end int, ok bool) {
	if p.pos+1 >= len(p.src) {
		return 0, false
	}
	d := p.src[p.pos+1]
	if d != ':' && d != '.' && d != '=' {
		return 0, false
	}
	s := p.src
	for i := p.pos + 2; i+1 < len(s); i++ {
		switch c := s[i]; {
		case c == '\\' && (s[i+1] == ']' || s[i+1] == '\\'):
			i++
		case c == '[' && s[i+1] == d, c == ']':
			return 0, false
		case c == d && s[i+1] == ']':
			return i + 2, true
		}
	}
	return 0, false
}

func (p *parser) parseClass() (*node, error) {
	p.pos++ // [
	cs := p.newSet()
	p.inQuote = false
	if p.peek() == '^' && p.more() {
		cs.neg = true
		p.pos++
	}
	first := true
	for {
		if !p.more() {
			return nil, p.errAt(len(p.src), "missing terminating ] for character class")
		}
		c := p.src[p.pos]
		if !p.inQuote {
			if p.fl.xx && (c == ' ' || c == '\t') {
				p.pos++
				continue
			}
			if c == ']' && !first {
				p.pos++
				break
			}
			// POSIX class
			if c == '[' {
				if stop, ok := p.posixLike(); ok {
					end := stop - 2 - (p.pos + 2)
					if p.src[p.pos+1] != ':' {
						return nil, p.errAt(stop, "POSIX collating elements are not supported")
					}
					name := p.src[p.pos+2 : p.pos+2+end]
					neg := false
					if strings.HasPrefix(name, "^") {
						neg = true
						name = name[1:]
					}
					p.pos += 2 + end + 2
					if !posixNames[name] {
						return nil, p.err("unknown POSIX class name")
					}
					cs.addPred(posixFn(name, p.opt.UCP), neg)
					if err := p.noRangeAfterClass(); err != nil {
						return nil, err
					}
					first = false
					continue
				}
			}
		}
		lo, kind, err := p.classAtom(cs)
		if err != nil {
			return nil, err
		}
		if kind != atomNone {
			first = false
		}
		switch kind {
		case atomNone:
			continue
		case atomClass:
			if err := p.noRangeAfterClass(); err != nil {
				return nil, err
			}
			continue
		}
		for !p.inQuote && strings.HasPrefix(p.src[p.pos:], "\\E") {
			p.pos += 2
		}
		if p.inQuote || p.peek() != '-' || p.pos+1 >= len(p.src) || p.src[p.pos+1] == ']' {
			cs.add(lo)
			continue
		}
		p.pos++ // -
		var hi rune
		for {
			if p.peek() == ']' && p.more() && !p.inQuote {
				cs.add(lo)
				cs.add('-')
				break
			}
			if !p.more() {
				return nil, p.errAt(len(p.src), "missing terminating ] for character class")
			}
			h, k, err := p.classAtom(nil)
			if err != nil {
				return nil, err
			}
			if k == atomNone {
				continue
			}
			if k == atomClass {
				return nil, p.err("invalid range in character class")
			}
			hi = h
			if hi < lo {
				return nil, p.err("range out of order in character class")
			}
			cs.addRange(lo, hi)
			break
		}
	}
	return p.classNode(cs), nil
}

// noRangeAfterClass rejects "-x" after a class escape or POSIX class, which
// PCRE2 treats as an invalid range ("-]" is fine).
func (p *parser) noRangeAfterClass() error {
	if !p.inQuote && p.peek() == '-' && p.pos+1 < len(p.src) && p.src[p.pos+1] != ']' {
		p.pos++
		return p.err("invalid range in character class")
	}
	return nil
}

// classAtom parses one class element.  Single characters are returned with
// kind atomChar; class escapes are added to into (when non-nil).
func (p *parser) classAtom(into *charSet) (r rune, kind int, err error) {
	if p.inQuote {
		if strings.HasPrefix(p.src[p.pos:], "\\E") {
			p.pos += 2
			p.inQuote = false
			return 0, atomNone, nil
		}
		r = p.nextRune()
		if strings.HasPrefix(p.src[p.pos:], "\\E") {
			p.pos += 2
			p.inQuote = false
		}
		return r, atomChar, nil
	}
	c := p.src[p.pos]
	if c != '\\' {
		return p.nextRune(), atomChar, nil
	}
	p.pos++
	if !p.more() {
		return 0, 0, p.err("\\ at end of pattern")
	}
	e := p.src[p.pos]
	switch e {
	case 'b':
		p.pos++
		return 8, atomChar, nil
	case 'd', 'D', 's', 'S', 'w', 'W', 'h', 'H', 'v', 'V':
		p.pos++
		if into != nil {
			p.typeSet(e, into)
		}
		return 0, atomClass, nil
	case 'p', 'P':
		p.pos++
		if into == nil {
			into = &charSet{}
		}
		return 0, atomClass, p.parseProp(e == 'P', into)
	case 'Q':
		p.pos++
		p.inQuote = true
		return 0, atomNone, nil
	case 'E':
		p.pos++
		return 0, atomNone, nil
	case 'N':
		p.pos++
		return 0, 0, p.err("\\N is not supported in a class")
	case 'g', 'k': // literal in a class, as in Perl
		p.pos++
		return rune(e), atomChar, nil
	case 'R', 'X', 'B', 'A', 'Z', 'z', 'G', 'K', 'C':
		p.pos++
		return 0, 0, p.err("escape sequence is invalid in character class")
	}
	if e >= '8' && e <= '9' {
		p.pos++
		return rune(e), atomChar, nil
	}
	r, ok, err := p.charEscape(true)
	if err != nil {
		return 0, 0, err
	}
	if !ok {
		p.pos++
		if strings.IndexByte("FLlUu", e) >= 0 {
			return 0, 0, p.err("PCRE2 does not support \\F, \\L, \\l, \\N{name}, \\U, or \\u")
		}
		return 0, 0, p.err("unrecognized character follows \\")
	}
	return r, atomChar, nil
}
