package gtfparser

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// MatchStyle is a pattern-matching style usable with /def -m<name>. TF has
// four built in (simple, glob, regexp, substr); a client can add its own with
// Interp.RegisterMatchStyle, for example a client-specific wildcard dialect or
// a literal-word matcher. A style compiles a pattern once, when the macro is
// defined, so matching a line costs only Matcher.Match.
type MatchStyle interface {
	// Compile prepares pattern. A returned error is shown to the user as the
	// reason the /def (or /list, /purge ...) failed.
	Compile(pattern string) (Matcher, error)
}

// Matcher tests text against one compiled pattern.
type Matcher interface {
	// Match reports whether text matches. If it does and the style has
	// sub-matches, caps holds byte offsets {start0, end0, start1, end1, ...}
	// (-1 for a group that did not participate): they supply %P0, %P1, ...,
	// %PL and %PR to the macro body. caps may be nil.
	Match(text string) (ok bool, caps []int)
}

// MatchStyleFuncs adapts two functions to a MatchStyle.
type MatchStyleFunc func(pattern string) (Matcher, error)

func (f MatchStyleFunc) Compile(p string) (Matcher, error) { return f(p) }

// MatcherFunc adapts a function to a Matcher.
type MatcherFunc func(text string) (bool, []int)

func (f MatcherFunc) Match(t string) (bool, []int) { return f(t) }

// Names of the built-in match styles, in tf's numbering (-m0 .. -m3).
const (
	MatchSimple = "simple"
	MatchGlob   = "glob"
	MatchRegexp = "regexp"
	MatchSubstr = "substr"
)

var builtinStyles = []string{MatchSimple, MatchGlob, MatchRegexp, MatchSubstr}

type patKind uint8

const (
	pkNone patKind = iota
	pkSimple
	pkGlob
	pkRegexp
	pkSubstr
	pkCustom
)

// Pattern is a compiled trigger, hook-argument, or world-type pattern.
type Pattern struct {
	Str   string // the pattern text as written
	Style string // the match style name
	kind  patKind
	g     *gpat
	re    *rx
	cm    Matcher
	lit   string // lower-cased literal prefix of a glob (cheap reject)
	// regexp prefilter: a literal every match must start with (rAnch) or
	// contain, checked before running the regexp.
	rlit      string
	rAnch     bool
	rCaseless bool
}

// Match tests text. caps is non-nil only for styles with sub-matches.
func (p *Pattern) Match(text string) (ok bool, caps []int) {
	if p == nil {
		return true, nil
	}
	switch p.kind {
	case pkSimple:
		return text == p.Str, nil
	case pkSubstr:
		return strings.Contains(text, p.Str), nil
	case pkGlob:
		if p.lit != "" {
			if len(text) < len(p.lit) || !asciiPrefixFold(text, p.lit) {
				return false, nil
			}
		}
		return p.g.match(0, text, 0), nil
	case pkRegexp:
		if p.rlit != "" && !p.reMaybe(text) {
			return false, nil
		}
		loc := p.re.match(text, 0)
		return loc != nil, loc
	case pkCustom:
		return p.cm.Match(text)
	}
	return true, nil
}

// asciiPrefixFold reports whether s begins with lit (already lower case),
// ignoring ASCII case.
func asciiPrefixFold(s, lit string) bool {
	for i := 0; i < len(lit); i++ {
		c := s[i]
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != lit[i] {
			return false
		}
	}
	return true
}

// globPrefix is the run of plain ASCII characters a glob starts with.
func globPrefix(p string) string {
	i := 0
	for i < len(p) {
		c := p[i]
		if c >= 0x80 || c == '*' || c == '?' || c == '[' || c == '{' || c == '\\' || c == '}' || c == '|' {
			break
		}
		i++
	}
	return strings.ToLower(p[:i])
}

// globCheck verifies glob syntax like tf's smatch_check().
func globCheck(pat string) error {
	inword := false
	i := 0
	for i < len(pat) {
		switch pat[i] {
		case '\\':
			i++
			if i < len(pat) {
				i++
			}
		case '[':
			j := i
			found := -1
			for j++; j < len(pat); j++ {
				if pat[j] == '\\' {
					j++
					continue
				}
				if pat[j] == ']' {
					found = j
					break
				}
			}
			if found < 0 {
				return fmt.Errorf("glob error: unmatched '['")
			}
			i = found + 1
		case '{':
			if inword {
				return fmt.Errorf("glob error: nested '{'")
			}
			if !(i == 0 || pat[i-1] == ' ' || pat[i-1] == '\t' || strings.IndexByte("*?]", pat[i-1]) >= 0) {
				return fmt.Errorf("glob error: '%c' before '{' can never match", pat[i-1])
			}
			inword = true
			i++
		case '}':
			if !(i+1 >= len(pat) || pat[i+1] == ' ' || pat[i+1] == '\t' || strings.IndexByte("*?[", pat[i+1]) >= 0) {
				return fmt.Errorf("glob error: '%c' after '}' can never match", pat[i+1])
			}
			inword = false
			i++
		default:
			if inword && (pat[i] == ' ' || pat[i] == '\t') {
				return fmt.Errorf("glob error: space inside '{...}' can never match")
			}
			i++
		}
	}
	if inword {
		return fmt.Errorf("glob error: unmatched '{'")
	}
	return nil
}

// compilePattern builds a Pattern of the given style.
func (in *Interp) compilePattern(style, s string) (*Pattern, error) {
	p := &Pattern{Str: s, Style: style}
	switch style {
	case MatchSimple:
		p.kind = pkSimple
	case MatchSubstr:
		p.kind = pkSubstr
	case MatchGlob:
		if err := globCheck(s); err != nil {
			return nil, err
		}
		p.kind = pkGlob
		p.g = parseGlob(s)
		p.lit = globPrefix(s)
	case MatchRegexp:
		if i := firstUnescaped(s, '{'); i >= 0 && i+1 < len(s) && (s[i+1] >= '0' && s[i+1] <= '9' || s[i+1] == ',') {
			if in.globals["warn_curly_re"].String() != "off" {
				in.warn("Warning: regexp contains '{', which has a new meaning in version 5.0.  (This warning can be disabled with '/set warn_curly_re=off'.)")
			}
		}
		re, err := compileRegexp(s)
		if err != nil {
			return nil, err
		}
		p.kind, p.re = pkRegexp, re
		p.rlit, p.rAnch, p.rCaseless = regexpLiteral(s)
	default:
		st, ok := in.macs().styles[style]
		if !ok {
			return nil, in.badStyle(style)
		}
		m, err := st.Compile(s)
		if err != nil {
			return nil, err
		}
		p.kind, p.cm = pkCustom, m
	}
	return p, nil
}

func firstUnescaped(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' {
			i++
		} else if s[i] == c {
			return i
		}
	}
	return -1
}

func (in *Interp) styleNames() []string {
	names := append([]string(nil), builtinStyles...)
	var extra []string
	for n := range in.macs().styles {
		extra = append(extra, n)
	}
	sort.Strings(extra)
	return append(names, extra...)
}

func (in *Interp) badStyle(s string) error {
	var parts []string
	for i, n := range in.styleNames() {
		parts = append(parts, fmt.Sprintf("%s (%d)", n, i))
	}
	return fmt.Errorf("Invalid -m value \"%s\".  Valid values are: %s", s, strings.Join(parts, ", "))
}

// styleByName resolves -m's argument (a name or number) to a style name.
func (in *Interp) styleByName(s string) (string, error) {
	names := in.styleNames()
	for _, n := range names {
		if strings.EqualFold(n, s) {
			return n, nil
		}
	}
	if s != "" && s[0] >= '0' && s[0] <= '9' {
		n := 0
		ok := true
		for _, c := range s {
			if c < '0' || c > '9' {
				ok = false
				break
			}
			n = n*10 + int(c-'0')
		}
		if ok && n < len(names) {
			return names[n], nil
		}
	}
	return "", in.badStyle(s)
}

// RegisterMatchStyle installs a custom pattern style, selectable with
// /def -m<name> (and /list, /purge ...). The built-in names cannot be
// replaced. Note the %matching variable only takes the built-in names; use
// -m with custom styles.
func (in *Interp) RegisterMatchStyle(name string, s MatchStyle) error {
	name = strings.ToLower(name)
	for _, b := range builtinStyles {
		if b == name {
			return fmt.Errorf("match style %q is built in", name)
		}
	}
	if name == "" || strings.ContainsAny(name, " \t|") {
		return fmt.Errorf("invalid match style name %q", name)
	}
	in.macs().styles[name] = s
	return nil
}

// CompilePattern compiles a pattern in the given style (one of the built-in
// names or a registered one) for use outside macros.
func (in *Interp) CompilePattern(style, pattern string) (*Pattern, error) {
	if style == "" {
		style = in.defaultStyle()
	}
	st, err := in.styleByName(style)
	if err != nil {
		return nil, err
	}
	return in.compilePattern(st, pattern)
}

// defaultStyle is the current %matching.
func (in *Interp) defaultStyle() string {
	s := in.globals["matching"].String()
	if s == "" {
		return MatchGlob
	}
	return s
}

// groups is the number of capture groups in the regexp.
func (r *rx) groups() int { return r.re.NumCaptures() }

// checkExpr parses a /def -E expression so syntax errors show at /def time.
func (in *Interp) checkExpr(src string) (eval, error) {
	p := &exprParser{in: in, s: src}
	f, err := p.parseComma()
	if err != nil {
		return nil, err
	}
	p.ws()
	if p.pos < len(p.s) {
		return nil, fmt.Errorf("expression: unexpected %q", p.s[p.pos:])
	}
	return f, nil
}

// rawCmds: commands whose arguments skip substitution (none: only top-level
// lines are unsubstituted, and that is handled by Exec).
var rawCmds = map[string]bool{}

// writeLines writes lines to a file (/save).
func (in *Interp) writeLines(name string, lines []string, appendMode bool) (string, error) {
	if strings.HasPrefix(name, "~/") {
		if h, err := os.UserHomeDir(); err == nil {
			name = filepath.Join(h, name[2:])
		}
	}
	flag := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if appendMode {
		flag = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	}
	f, err := os.OpenFile(name, flag, 0o644)
	if err != nil {
		return name, err
	}
	defer f.Close()
	for _, l := range lines {
		if _, err := f.WriteString(l + "\n"); err != nil {
			return name, err
		}
	}
	return name, nil
}

// regexpLiteral finds a literal that every match of the regexp must begin
// with (anchored) or contain, so most lines are rejected without running the
// regexp engine. It only fires for plain ASCII patterns with no alternation
// or inline options, where the conclusion is certain.
func regexpLiteral(pat string) (lit string, anchored, caseless bool) {
	if strings.ContainsRune(pat, '|') || strings.Contains(pat, "(?") {
		return "", false, false
	}
	caseless = true
	for i := 0; i < len(pat); i++ {
		c := pat[i]
		if c >= 0x80 {
			return "", false, false
		}
		if c == '\\' {
			i++
		} else if c >= 'A' && c <= 'Z' {
			caseless = false
		}
	}
	i := 0
	if strings.HasPrefix(pat, "^") {
		anchored, i = true, 1
	}
	var b strings.Builder
	for ; i < len(pat); i++ {
		c := pat[i]
		isLit := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.IndexByte(" _-,:;!\"'%@&=<>~#/", c) >= 0
		if !isLit {
			break
		}
		if i+1 < len(pat) && strings.IndexByte("*?+{", pat[i+1]) >= 0 {
			break
		}
		b.WriteByte(c)
	}
	lit = b.String()
	if caseless {
		lit = strings.ToLower(lit)
	}
	return lit, anchored, caseless
}

// reMaybe reports whether text could match (false means certainly not).
func (p *Pattern) reMaybe(text string) bool {
	lit := p.rlit
	n := len(lit)
	if len(text) < n {
		return false
	}
	eq := func(a byte, b byte) bool { // b is the pattern byte (lower-cased if caseless)
		if p.rCaseless && a >= 'A' && a <= 'Z' {
			a += 'a' - 'A'
		}
		return a == b
	}
	if p.rAnch {
		for i := 0; i < n; i++ {
			c := text[i]
			if c >= 0x80 {
				return true
			}
			if !eq(c, lit[i]) {
				return false
			}
		}
		return true
	}
	for i := 0; i < len(text); i++ {
		if text[i] >= 0x80 {
			return true
		}
	}
	for i := 0; i+n <= len(text); i++ {
		j := 0
		for j < n && eq(text[i+j], lit[j]) {
			j++
		}
		if j == n {
			return true
		}
	}
	return false
}
