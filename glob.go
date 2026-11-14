package gtfparser

import (
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// TF glob: * ? [set] [^set] {word|word|...} and \ escapes. Matching ignores
// case. A {...} group matches whole words: it must start at the beginning of
// the string or after a space, and end before a space or the end of string.

type gtok struct {
	kind byte // 'c' char, '*', '?', '[' set, '{' words
	r    rune
	set  []rune // set: pairs lo,hi
	neg  bool
	alts []*gpat
}

type gpat struct {
	toks []gtok
	word bool // inside {...}: wildcards do not match spaces
}

func parseGlob(p string) *gpat {
	g := &gpat{}
	rs := []rune(p)
	for i := 0; i < len(rs); i++ {
		switch c := rs[i]; c {
		case '*':
			g.toks = append(g.toks, gtok{kind: '*'})
		case '?':
			g.toks = append(g.toks, gtok{kind: '?'})
		case '\\':
			if i+1 < len(rs) {
				i++
			}
			g.toks = append(g.toks, gtok{kind: 'c', r: rs[i]})
		case '[':
			j := i + 1
			t := gtok{kind: '['}
			if j < len(rs) && rs[j] == '^' {
				t.neg = true
				j++
			}
			first := true
			for j < len(rs) && (rs[j] != ']' || first) {
				first = false
				lo := rs[j]
				if lo == '\\' && j+1 < len(rs) {
					j++
					lo = rs[j]
				}
				hi := lo
				if j+2 < len(rs) && rs[j+1] == '-' && rs[j+2] != ']' {
					hi = rs[j+2]
					j += 2
				}
				t.set = append(t.set, lo, hi)
				j++
			}
			if j >= len(rs) { // unterminated: literal '['
				g.toks = append(g.toks, gtok{kind: 'c', r: '['})
				continue
			}
			g.toks = append(g.toks, t)
			i = j
		case '{':
			j := i + 1
			depth := 0
			for j < len(rs) && !(rs[j] == '}' && depth == 0) {
				if rs[j] == '\\' {
					j++
				} else if rs[j] == '[' {
					depth++
				} else if rs[j] == ']' && depth > 0 {
					depth--
				}
				j++
			}
			if j >= len(rs) {
				g.toks = append(g.toks, gtok{kind: 'c', r: '{'})
				continue
			}
			t := gtok{kind: '{'}
			for _, a := range splitUnescaped(string(rs[i+1:j]), '|') {
				ap := parseGlob(a)
				ap.word = true
				t.alts = append(t.alts, ap)
			}
			g.toks = append(g.toks, t)
			i = j
		default:
			g.toks = append(g.toks, gtok{kind: 'c', r: c})
		}
	}
	return g
}

func splitUnescaped(s string, sep rune) []string {
	var out []string
	var b strings.Builder
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		if rs[i] == '\\' && i+1 < len(rs) {
			b.WriteRune(rs[i])
			i++
			b.WriteRune(rs[i])
			continue
		}
		if rs[i] == sep {
			out = append(out, b.String())
			b.Reset()
			continue
		}
		b.WriteRune(rs[i])
	}
	return append(out, b.String())
}

// foldEq compares literal pattern characters: tf folds case with the C
// library's tolower() on bytes, which leaves non-ASCII characters alone, so
// only ASCII letters match case-insensitively here.
func foldEq(a, b rune) bool {
	if a == b {
		return true
	}
	if a >= 0x80 || b >= 0x80 {
		return false
	}
	return unicode.ToLower(a) == unicode.ToLower(b)
}

func (t *gtok) inSet(r rune) bool {
	in := false
	lr, ur := unicode.ToLower(r), unicode.ToUpper(r)
	for i := 0; i+1 < len(t.set); i += 2 {
		lo, hi := t.set[i], t.set[i+1]
		if (r >= lo && r <= hi) || (lr >= lo && lr <= hi) || (ur >= lo && ur <= hi) {
			in = true
			break
		}
	}
	return in != t.neg
}

// match reports whether toks[ti:] matches s[si:] to the end of s.
func (g *gpat) match(ti int, s string, si int) bool {
	return g.matchEnd(ti, s, si, func(e int) bool { return e == len(s) })
}

// matchEnd is match with a continuation on the end position (used by {...}).
func (g *gpat) matchEnd(ti int, s string, si int, k func(int) bool) bool {
	for ti < len(g.toks) {
		t := &g.toks[ti]
		switch t.kind {
		case 'c':
			if si >= len(s) {
				return false
			}
			r, n := utf8.DecodeRuneInString(s[si:])
			if !foldEq(r, t.r) {
				return false
			}
			si += n
			ti++
		case '?':
			if si >= len(s) {
				return false
			}
			r, n := utf8.DecodeRuneInString(s[si:])
			if g.word && r == ' ' {
				return false
			}
			si += n
			ti++
		case '[':
			if si >= len(s) {
				return false
			}
			r, n := utf8.DecodeRuneInString(s[si:])
			if !t.inSet(r) {
				return false
			}
			si += n
			ti++
		case '*':
			for ti+1 < len(g.toks) && g.toks[ti+1].kind == '*' {
				ti++
			}
			if ti+1 == len(g.toks) && !g.word {
				return k(len(s))
			}
			for j := si; j <= len(s); {
				if g.matchEnd(ti+1, s, j, k) {
					return true
				}
				if j == len(s) {
					break
				}
				r, n := utf8.DecodeRuneInString(s[j:])
				if g.word && r == ' ' {
					return false
				}
				j += n
			}
			return false
		case '{':
			if si > 0 && s[si-1] != ' ' {
				return false
			}
			for _, a := range t.alts {
				if a.matchEnd(0, s, si, func(e int) bool {
					if e < len(s) && s[e] != ' ' {
						return false
					}
					return g.matchEnd(ti+1, s, e, k)
				}) {
					return true
				}
			}
			return false
		}
	}
	return k(si)
}

var (
	globMu    sync.Mutex
	globCache = map[string]*gpat{}
)

// globMatch matches s against a TF glob pattern.
func globMatch(pat, s string) bool {
	globMu.Lock()
	g, ok := globCache[pat]
	if !ok {
		g = parseGlob(pat)
		if len(globCache) > 4096 {
			globCache = map[string]*gpat{}
		}
		globCache[pat] = g
	}
	globMu.Unlock()
	return g.match(0, s, 0)
}
