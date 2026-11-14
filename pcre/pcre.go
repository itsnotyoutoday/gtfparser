// Package pcre is a backtracking regular-expression engine with PCRE2
// semantics, written in pure Go (standard library only).  Matching is
// leftmost with PCRE's ordered alternation and greedy, lazy and possessive
// quantifiers; it is not leftmost-longest.
//
// Supported syntax: literals and all character escapes (\t \n \r \f \e \a
// \0 \ddd \o{} \xhh \x{h..} \cx \N{U+hhhh} \Q..\E), '.', character classes
// (ranges, negation, escapes, POSIX [[:alpha:]] and [[:^alpha:]]), \d \D \s \S
// \w \W \h \H \v \V \R \N \X \C, \p{..} and \P{..} (general categories,
// scripts, Script_Extensions, L&, Any, Xan Xwd Xsp Xps Xuc), the assertions
// \b \B \A \Z \z \G ^ $, \K, capturing, non-capturing, named ((?<n>) (?P<n>)
// (?'n')), atomic and branch-reset groups, inline flags (?imsxnUJ-^) in global
// and scoped form, quantifiers {n} {n,} {n,m} {,m} with lazy and possessive
// forms, backreferences (\1 \g{n} \g{-n} \k<n> \k{n} \k'n' (?P=n)), lookahead,
// lookbehind (bounded variable length), conditionals ((?(1)..|..), named,
// lookaround and DEFINE), comments, and (*FAIL) / (*F).
//
// Not supported: recursion and subroutine calls ((?R) (?1) (?&n) \g<n>),
// callouts, the (*ACCEPT) (*COMMIT) (*PRUNE) (*SKIP) (*THEN) verbs and
// (*UTF)-style start-of-pattern items, extended classes (?[...]),
// alphabetic assertions ((*pla:...)) and script runs, and the (?a)
// ASCII-restriction flags.  Duplicate group names ((?J)) are only partly
// handled, and \X approximates an extended grapheme cluster.
//
// With UTF the subject is UTF-8 and '.' consumes a whole character; offsets
// are always byte offsets.  Invalid UTF-8 in the subject follows
// PCRE2_MATCH_INVALID_UTF: an invalid byte is a one-byte character that no
// item can match, and matching starts only inside the valid stretches.
// With UCP, \w \d \s \b and the POSIX classes use Unicode properties; the
// property tables are generated from PCRE2 10.47 (scripts/pcre_genprops.py).
//
// The engine was checked against PCRE2 itself: scripts/pcre_diff.py runs a
// corpus through a real TinyFugue build, scripts/pcre_ref.py and
// scripts/pcre_fuzz.py drive PCRE2 directly.
package pcre

import (
	"errors"
	"fmt"
	"unicode"
	"unicode/utf8"
)

// Options selects compile-time behaviour (the PCRE2_* options of the same name).
type Options struct {
	Caseless      bool // PCRE2_CASELESS
	DotAll        bool // PCRE2_DOTALL
	DollarEndOnly bool // PCRE2_DOLLAR_ENDONLY
	Multiline     bool // PCRE2_MULTILINE
	Extended      bool // PCRE2_EXTENDED
	UTF           bool // PCRE2_UTF: pattern and subject are UTF-8
	UCP           bool // PCRE2_UCP: \w \d \s \b and POSIX classes use Unicode properties
}

// Error is a pattern compilation error.
type Error struct {
	Offset int    // byte offset in the pattern
	Msg    string // PCRE2-style message
}

func (e *Error) Error() string { return fmt.Sprintf("%s at offset %d", e.Msg, e.Offset) }

// ErrLimit is returned by MatchErr when the backtracking step limit (or the
// recursion depth limit) is exceeded.
var ErrLimit = errors.New("pcre: match limit exceeded")

// DefaultStepLimit is the default number of backtracking steps allowed per Match call.
const DefaultStepLimit = 10_000_000

const maxDepth = 200_000

// Regexp is a compiled pattern.  It is safe for concurrent use.
type Regexp struct {
	opt       Options
	prog      mfn
	ncap      int
	names     []string
	nslots    int
	stepLimit int
	anchored  bool
	prefix    string // literal every match must start with (case-sensitive), or ""
}

// NumCaptures returns the number of capturing groups.
func (re *Regexp) NumCaptures() int { return re.ncap }

// CaptureNames returns the group names; index is the group number and the
// entry is "" for unnamed groups (and for group 0).
func (re *Regexp) CaptureNames() []string {
	return append([]string(nil), re.names...)
}

// SetStepLimit changes the backtracking step limit (<=0 restores the default).
// It must not be called concurrently with Match.
func (re *Regexp) SetStepLimit(n int) {
	if n <= 0 {
		n = DefaultStepLimit
	}
	re.stepLimit = n
}

// Compile compiles pattern with the given options.
func Compile(pattern string, o Options) (*Regexp, error) {
	if o.UTF && !utf8.ValidString(pattern) {
		return nil, &Error{Offset: firstInvalid(pattern), Msg: "UTF-8 error: illegal byte"}
	}
	p := &parser{src: pattern, opt: o, total: -1}
	root, err := p.parseTop()
	if err != nil {
		return nil, err
	}
	if p.ambiguous {
		// \NNN was provisionally a backreference; reparse knowing the group count.
		q := &parser{src: pattern, opt: o, total: p.ncap}
		if root, err = q.parseTop(); err != nil {
			return nil, err
		}
		p = q
	}
	if err := p.resolve(root); err != nil {
		return nil, err
	}
	c := &compiler{utf: o.UTF, ucp: o.UCP, ncap: p.ncap, names: p.nameIdx}
	prog, err := c.compileTop(root)
	if err != nil {
		return nil, err
	}
	re := &Regexp{
		opt: o, prog: prog, ncap: p.ncap, nslots: c.nslots,
		names: p.capNames(), stepLimit: DefaultStepLimit,
		anchored: isAnchored(root),
	}
	if lit, ok := leadingLiteral(root, o.UTF); ok {
		re.prefix = lit
	}
	return re, nil
}

func firstInvalid(s string) int {
	for i := 0; i < len(s); {
		r, w := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && w == 1 {
			return i
		}
		i += w
	}
	return len(s)
}

// CompileTF compiles pattern the way TinyFugue does: DOLLAR_ENDONLY, DOTALL,
// UTF and UCP are on, and CASELESS is on unless the pattern contains an
// upper-case letter ("smart case"; the character after a backslash is
// skipped).  A pattern that is not valid UTF-8 is compiled in byte mode.
func CompileTF(pattern string) (*Regexp, error) {
	o := Options{Caseless: true, DotAll: true, DollarEndOnly: true, UTF: true, UCP: true}
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		if c == '\\' {
			if i+1 < len(pattern) {
				i++
			}
		} else if c >= 0x80 {
			r, w := utf8.DecodeRuneInString(pattern[i:])
			if r != utf8.RuneError && unicode.IsUpper(r) {
				o.Caseless = false
				break
			}
			i += w - 1
		} else if c >= 'A' && c <= 'Z' {
			o.Caseless = false
			break
		}
	}
	if !utf8.ValidString(pattern) {
		o.UTF, o.UCP = false, false
	}
	return Compile(pattern, o)
}

// MustCompileTF is like CompileTF but panics on error.
func MustCompileTF(pattern string) *Regexp {
	re, err := CompileTF(pattern)
	if err != nil {
		panic(err)
	}
	return re
}

// Match searches s for the leftmost match beginning at or after byte offset
// start.  It returns nil if there is no match (or the step limit was hit; use
// MatchErr to tell the difference).  Otherwise it returns the offset vector:
// 2*(NumCaptures()+1) byte offsets, -1 for groups that did not participate.
// Lookbehind may look before start.  ^ matches at offset 0 only, and not at
// all when notBOL is set.
func (re *Regexp) Match(s string, start int, notBOL bool) []int {
	v, _ := re.MatchErr(s, start, notBOL)
	return v
}

// MatchErr is Match but also reports ErrLimit when matching was abandoned.
func (re *Regexp) MatchErr(s string, start int, notBOL bool) ([]int, error) {
	if start < 0 || start > len(s) {
		return nil, nil
	}
	m := &matcher{
		re: re, s: s, utf: re.opt.UTF, ucp: re.opt.UCP, from: start, notBOL: notBOL,
		caps:  make([]int, 2*(re.ncap+1)),
		tent:  make([]int, re.ncap+1),
		slots: make([]int, re.nslots),
		limit: re.stepLimit, kstart: -1,
	}
	for i := range m.caps {
		m.caps[i] = -1
	}
	i := start
	if re.opt.UTF {
		for i < len(s) && s[i]&0xc0 == 0x80 { // a character cannot start on a continuation byte
			i++
		}
	}
	first := i
	for i >= 0 && i <= len(s) {
		if re.prefix != "" {
			j := indexFrom(s, i, re.prefix)
			if j < 0 {
				break
			}
			i = j
		}
		if re.prog(m, i) {
			m.caps[0], m.caps[1] = i, m.end
			if m.kstart >= 0 {
				m.caps[0] = m.kstart
			}
			return m.caps, nil
		}
		if m.aborted {
			return nil, ErrLimit
		}
		if re.anchored {
			break
		}
		i = m.nextStart(i, first)
	}
	return nil, nil
}
