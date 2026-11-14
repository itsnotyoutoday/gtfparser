package pcre

import (
	"strings"
	"unicode/utf8"
)

// mfn matches the rest of the pattern (this node and its static continuation)
// at position i.
type mfn func(m *matcher, i int) bool

type matcher struct {
	re      *Regexp
	s       string
	utf     bool
	ucp     bool
	from    int // \G position
	notBOL  bool
	caps    []int
	tent    []int // start offsets of open groups
	slots   []int // loop counters / lookbehind origins
	kstart  int
	end     int
	atomEnd int
	steps   int
	limit   int
	depth   int
	aborted bool
}

// tick counts a backtracking step and reports whether matching must stop.
func (m *matcher) tick() bool {
	m.steps++
	if m.steps > m.limit {
		m.aborted = true
	}
	return m.aborted
}

func (m *matcher) addSteps(n int) bool {
	m.steps += n
	if m.steps > m.limit {
		m.aborted = true
	}
	return m.aborted
}

// decode returns the character at i and its byte width.  Invalid UTF-8
// bytes are one-byte characters with value -1, which no item matches.
func (m *matcher) decode(i int) (rune, int) {
	if i >= len(m.s) {
		return -1, 0
	}
	b := m.s[i]
	if b < 0x80 || !m.utf {
		return rune(b), 1
	}
	r, w := utf8.DecodeRuneInString(m.s[i:])
	if r == utf8.RuneError && w <= 1 {
		return -1, 1
	}
	return r, w
}

func (m *matcher) decodeBack(i int) (rune, int) {
	if i <= 0 {
		return -1, 0
	}
	b := m.s[i-1]
	if b < 0x80 || !m.utf {
		return rune(b), 1
	}
	r, w := utf8.DecodeLastRuneInString(m.s[:i])
	if r == utf8.RuneError && w <= 1 {
		return -1, 1
	}
	return r, w
}

func (m *matcher) isWord(r rune) bool {
	if r < 0 {
		return false
	}
	if m.ucp {
		return isUWord(r)
	}
	return isASCIIWord(r)
}

// nextStart returns the next position at which a match may start after a
// failed attempt at i, or -1 when the subject is exhausted.  Without UTF it
// is the next byte.  In UTF mode it models PCRE2_MATCH_INVALID_UTF, which
// matches within the valid chunks of the subject: a position that follows an
// invalid byte cannot be on a continuation byte, and an empty chunk (between
// invalid bytes, or at the end) is not tried, except at the first position.
func (m *matcher) nextStart(i, first int) int {
	if i >= len(m.s) {
		return -1
	}
	prev, w := m.decode(i)
	i += w
	if !m.utf || prev >= 0 {
		return i
	}
	for {
		for i < len(m.s) && m.s[i]&0xc0 == 0x80 {
			i++
		}
		if i <= first {
			return i
		}
		if i == len(m.s) {
			return -1
		}
		if r, _ := m.decode(i); r >= 0 {
			return i
		}
		i++ // an empty chunk: skip the invalid byte
	}
}

func indexFrom(s string, i int, sub string) int {
	j := strings.Index(s[i:], sub)
	if j < 0 {
		return -1
	}
	return i + j
}

// ---- static analysis -------------------------------------------------

func isAnchored(n *node) bool {
	switch n.k {
	case kBeginText, kStartMatch:
		return true
	case kBol:
		return !n.flag
	case kSeq:
		return isAnchored(n.subs[0])
	case kGroup, kAtomic:
		return isAnchored(n.subs[0])
	case kAlt:
		for _, s := range n.subs {
			if !isAnchored(s) {
				return false
			}
		}
		return true
	}
	return false
}

func leadingLiteral(n *node, utf bool) (string, bool) {
	var items []*node
	switch n.k {
	case kSeq:
		items = n.subs
	case kLit:
		items = []*node{n}
	default:
		return "", false
	}
	var b strings.Builder
	for _, it := range items {
		if it.k != kLit || it.fold {
			break
		}
		if utf {
			b.WriteRune(it.r)
		} else {
			b.WriteByte(byte(it.r))
		}
	}
	return b.String(), b.Len() > 0
}
