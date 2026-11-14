package pcre

import (
	"strings"
	"unicode"
)

type compiler struct {
	utf, ucp bool
	ncap     int
	nslots   int
	names    map[string]int
	err      error
}

type stepFn func(m *matcher, i int) int // width of the character matched at i, or -1

func (c *compiler) slot(n int) int {
	s := c.nslots
	c.nslots += n
	return s
}

func (c *compiler) compileTop(root *node) (mfn, error) {
	f := c.comp(root, func(m *matcher, i int) bool { m.end = i; return true })
	return f, c.err
}

func isSingle(n *node) bool { return n.k == kLit || n.k == kAny || n.k == kClass }

func containsCap(n *node) bool {
	if n.k == kGroup {
		return true
	}
	for _, s := range n.subs {
		if containsCap(s) {
			return true
		}
	}
	return n.cond != nil && containsCap(n.cond)
}

func (c *compiler) single(n *node) stepFn {
	switch n.k {
	case kLit:
		r := n.r
		if n.fold {
			orb := foldOrbit(r, c.utf)
			return func(m *matcher, i int) int {
				x, w := m.decode(i)
				for _, o := range orb {
					if x == o {
						return w
					}
				}
				return -1
			}
		}
		if r < 128 || !c.utf {
			b := byte(r)
			return func(m *matcher, i int) int {
				if i < len(m.s) && m.s[i] == b {
					return 1
				}
				return -1
			}
		}
		enc := string(r)
		return func(m *matcher, i int) int {
			if strings.HasPrefix(m.s[i:], enc) {
				return len(enc)
			}
			return -1
		}
	case kAny:
		dotall := n.flag
		return func(m *matcher, i int) int {
			x, w := m.decode(i)
			if x < 0 || (!dotall && x == '\n') {
				return -1
			}
			return w
		}
	default: // kClass
		set := n.set
		return func(m *matcher, i int) int {
			x, w := m.decode(i)
			if x < 0 || !set.has(x) {
				return -1
			}
			return w
		}
	}
}

func (c *compiler) comp(n *node, next mfn) mfn {
	switch n.k {
	case kEmpty:
		return next
	case kFail:
		return func(m *matcher, i int) bool { return false }
	case kLit, kAny, kClass:
		st := c.single(n)
		return func(m *matcher, i int) bool {
			w := st(m, i)
			if w < 0 {
				return false
			}
			return next(m, i+w)
		}
	case kSeq:
		return c.seq(n.subs, next)
	case kAlt:
		fns := make([]mfn, len(n.subs))
		for k, s := range n.subs {
			fns[k] = c.comp(s, next)
		}
		return func(m *matcher, i int) bool {
			for _, f := range fns {
				if m.tick() {
					return false
				}
				if f(m, i) {
					return true
				}
			}
			return false
		}
	case kRepeat:
		return c.repeat(n, next)
	case kGroup:
		return c.group(n, next)
	case kAtomic:
		return c.atomic(n, next)
	case kLook:
		return c.look(n, next)
	case kBackref:
		return c.backref(n, next)
	case kCond:
		return c.cond(n, next)
	case kBol:
		multi := n.flag
		return func(m *matcher, i int) bool {
			if i == 0 {
				return !m.notBOL && next(m, i)
			}
			return multi && i < len(m.s) && m.s[i-1] == '\n' && next(m, i)
		}
	case kEol:
		multi, endOnly := n.flag, n.flag2
		return func(m *matcher, i int) bool {
			n := len(m.s)
			ok := i == n
			if !ok {
				if multi {
					ok = m.s[i] == '\n'
				} else if !endOnly {
					ok = i == n-1 && m.s[i] == '\n'
				}
			}
			return ok && next(m, i)
		}
	case kBeginText:
		return func(m *matcher, i int) bool { return i == 0 && next(m, i) }
	case kEndText:
		return func(m *matcher, i int) bool { return i == len(m.s) && next(m, i) }
	case kEndTextNL:
		return func(m *matcher, i int) bool {
			n := len(m.s)
			return (i == n || (i == n-1 && m.s[i] == '\n')) && next(m, i)
		}
	case kStartMatch:
		return func(m *matcher, i int) bool { return i == m.from && next(m, i) }
	case kWordB, kNotWordB:
		want := n.k == kWordB
		return func(m *matcher, i int) bool {
			a, _ := m.decodeBack(i)
			b, _ := m.decode(i)
			return (m.isWord(a) != m.isWord(b)) == want && next(m, i)
		}
	case kKeep:
		return func(m *matcher, i int) bool {
			old := m.kstart
			m.kstart = i
			if next(m, i) {
				return true
			}
			m.kstart = old
			return false
		}
	case kAnyByte:
		return func(m *matcher, i int) bool { return i < len(m.s) && next(m, i+1) }
	case kGrapheme:
		return func(m *matcher, i int) bool {
			j := m.grapheme(i)
			return j > i && next(m, j)
		}
	}
	panic("pcre: unknown node")
}

func (c *compiler) seq(subs []*node, next mfn) mfn {
	for k := len(subs) - 1; k >= 0; k-- {
		// merge a run of case-sensitive literals into one string compare
		if subs[k].k == kLit && !subs[k].fold {
			j := k
			for j > 0 && subs[j-1].k == kLit && !subs[j-1].fold {
				j--
			}
			if j < k {
				var b strings.Builder
				for _, l := range subs[j : k+1] {
					if c.utf {
						b.WriteRune(l.r)
					} else {
						b.WriteByte(byte(l.r))
					}
				}
				lit := b.String()
				nx := next
				next = func(m *matcher, i int) bool {
					return strings.HasPrefix(m.s[i:], lit) && nx(m, i+len(lit))
				}
				k = j
				continue
			}
		}
		next = c.comp(subs[k], next)
	}
	return next
}

func (c *compiler) group(n *node, next mfn) mfn {
	g := n.n
	closeFn := func(m *matcher, i int) bool {
		o0, o1 := m.caps[2*g], m.caps[2*g+1]
		m.caps[2*g], m.caps[2*g+1] = m.tent[g], i
		if next(m, i) {
			return true
		}
		m.caps[2*g], m.caps[2*g+1] = o0, o1
		return false
	}
	body := c.comp(n.subs[0], closeFn)
	return func(m *matcher, i int) bool {
		old := m.tent[g]
		m.tent[g] = i
		if body(m, i) {
			return true
		}
		m.tent[g] = old
		return false
	}
}

func (c *compiler) atomic(n *node, next mfn) mfn {
	sub := n.subs[0]
	hasCap := containsCap(sub)
	body := c.comp(sub, func(m *matcher, i int) bool { m.atomEnd = i; return true })
	return func(m *matcher, i int) bool {
		var saved []int
		if hasCap {
			saved = append([]int(nil), m.caps...)
		}
		ks := m.kstart
		if !body(m, i) {
			return false
		}
		if next(m, m.atomEnd) {
			return true
		}
		if hasCap {
			copy(m.caps, saved)
		}
		m.kstart = ks
		return false
	}
}

// lenRange returns the minimum and maximum length in characters of what n
// can match; max < 0 means unbounded (or not computable, e.g. a backreference).
func (c *compiler) lenRange(n *node) (min, max int) {
	switch n.k {
	case kLit, kAny, kClass:
		return 1, 1
	case kSeq:
		for _, s := range n.subs {
			lo, hi := c.lenRange(s)
			min += lo
			if max >= 0 {
				if hi < 0 {
					max = -1
				} else {
					max += hi
				}
			}
		}
		return min, max
	case kAlt, kCond:
		for k, s := range n.subs {
			lo, hi := c.lenRange(s)
			if k == 0 || lo < min {
				min = lo
			}
			if hi < 0 || max < 0 {
				max = -1
			} else if hi > max {
				max = hi
			}
		}
		return min, max
	case kGroup, kAtomic:
		return c.lenRange(n.subs[0])
	case kRepeat:
		lo, hi := c.lenRange(n.subs[0])
		min = lo * n.min
		switch {
		case hi == 0:
			max = 0
		case hi < 0 || n.max < 0:
			max = -1
		default:
			max = hi * n.max
		}
		return min, max
	case kBackref, kGrapheme:
		return 0, -1
	case kAnyByte:
		return 1, 1
	}
	return 0, 0
}

// assertion compiles a lookaround; it returns a function that reports
// whether the assertion body matches at i (captures set inside are left in
// place on success).
func (c *compiler) assertion(n *node) func(m *matcher, i int) bool {
	sub := n.subs[0]
	if !n.behind {
		body := c.comp(sub, func(m *matcher, i int) bool { return true })
		return body
	}
	branches := []*node{sub}
	if sub.k == kAlt {
		branches = sub.subs
	}
	s := c.slot(1)
	term := func(m *matcher, e int) bool { return e == m.slots[s] }
	fns := make([]mfn, len(branches))
	los := make([]int, len(branches))
	his := make([]int, len(branches))
	for k, b := range branches {
		lo, hi := c.lenRange(b)
		if c.err == nil {
			switch {
			case hi < 0:
				c.err = &Error{Offset: n.off, Msg: "length of lookbehind assertion is not limited"}
			case hi > 255:
				c.err = &Error{Offset: n.off, Msg: "lookbehind assertion is too long"}
			}
		}
		los[k], his[k] = lo, hi
		fns[k] = c.comp(b, term)
	}
	return func(m *matcher, i int) bool {
		old := m.slots[s]
		m.slots[s] = i
		defer func() { m.slots[s] = old }()
		for k, f := range fns {
			j := i
			for l := 0; l <= his[k]; l++ {
				if l >= los[k] && f(m, j) {
					return true
				}
				if m.aborted {
					return false
				}
				r, w := m.decodeBack(j)
				if r < 0 {
					break
				}
				j -= w
			}
		}
		return false
	}
}

func (c *compiler) look(n *node, next mfn) mfn {
	test := c.assertion(n)
	hasCap := containsCap(n.subs[0])
	neg := n.neg
	return func(m *matcher, i int) bool {
		var saved []int
		if hasCap {
			saved = append([]int(nil), m.caps...)
		}
		ok := test(m, i)
		if m.aborted {
			return false
		}
		if neg {
			if ok {
				if hasCap {
					copy(m.caps, saved)
				}
				return false
			}
			return next(m, i)
		}
		if !ok {
			return false
		}
		if next(m, i) {
			return true
		}
		if hasCap {
			copy(m.caps, saved)
		}
		return false
	}
}

func (c *compiler) cond(n *node, next mfn) mfn {
	yes := c.comp(n.subs[0], next)
	no := c.comp(n.subs[1], next)
	if n.cond != nil {
		test := c.assertion(n.cond)
		neg := n.cond.neg
		return func(m *matcher, i int) bool {
			saved := append([]int(nil), m.caps...)
			ok := test(m, i) != neg
			if m.aborted {
				return false
			}
			if neg {
				copy(m.caps, saved) // captures in a failed negative test are discarded
			}
			var r bool
			if ok {
				r = yes(m, i)
			} else {
				r = no(m, i)
			}
			if !r {
				copy(m.caps, saved)
			}
			return r
		}
	}
	g := n.condN
	if n.ref != nil {
		g = n.ref.n
	}
	return func(m *matcher, i int) bool {
		if m.caps[2*g+1] >= 0 {
			return yes(m, i)
		}
		return no(m, i)
	}
}

func (c *compiler) backref(n *node, next mfn) mfn {
	g, fold, utf := n.n, n.fold, c.utf
	return func(m *matcher, i int) bool {
		s, e := m.caps[2*g], m.caps[2*g+1]
		if s < 0 || e < 0 {
			return false
		}
		sub := m.s[s:e]
		if !fold {
			return strings.HasPrefix(m.s[i:], sub) && next(m, i+len(sub))
		}
		j := i
		for k := s; k < e; {
			r1, w1 := m.decode(k)
			r2, w2 := m.decode(j)
			if w2 == 0 {
				return false
			}
			if r1 != r2 {
				if r1 < 0 || r2 < 0 {
					return false
				}
				eq := false
				for _, o := range foldOrbit(r1, utf) {
					if o == r2 {
						eq = true
						break
					}
				}
				if !eq {
					return false
				}
			}
			k += w1
			j += w2
		}
		return next(m, j)
	}
}

// grapheme approximates \X: a base character followed by combining marks,
// ZWJ sequences, variation selectors, a CRLF pair, or a pair of regional indicators.
func (m *matcher) grapheme(i int) int {
	r, w := m.decode(i)
	if w == 0 || r < 0 {
		return i
	}
	j := i + w
	if r == '\r' {
		if j < len(m.s) && m.s[j] == '\n' {
			j++
		}
		return j
	}
	if r == '\n' || (r >= 0 && r < 32) {
		return j
	}
	if r >= 0x1f1e6 && r <= 0x1f1ff {
		if r2, w2 := m.decode(j); r2 >= 0x1f1e6 && r2 <= 0x1f1ff {
			j += w2
		}
	}
	for {
		r2, w2 := m.decode(j)
		if w2 == 0 || r2 < 0 {
			return j
		}
		switch {
		case unicode.IsMark(r2) || r2 == 0x200c || (r2 >= 0x1f3fb && r2 <= 0x1f3ff):
			j += w2
		case r2 == 0x200d:
			j += w2
			if r3, w3 := m.decode(j); r3 >= 0x80 {
				j += w3
			}
		default:
			return j
		}
	}
}

// ---- repetition ------------------------------------------------------

func (c *compiler) repeat(n *node, next mfn) mfn {
	sub := n.subs[0]
	min, max, mode := n.min, n.max, n.mode
	if max == 1 && min == 1 {
		return c.comp(sub, next)
	}
	if max == 0 {
		return next
	}
	if isSingle(sub) {
		return c.repeatSingle(c.single(sub), min, max, mode, next)
	}
	if mode == modePossessive {
		g := *n
		g.mode = modeGreedy
		return c.atomic(&node{k: kAtomic, subs: []*node{&g}}, next)
	}
	if min == 0 && max == 1 {
		body := c.comp(sub, next)
		if mode == modeLazy {
			return func(m *matcher, i int) bool {
				if next(m, i) {
					return true
				}
				if m.tick() {
					return false
				}
				return body(m, i)
			}
		}
		return func(m *matcher, i int) bool {
			if m.tick() {
				return false
			}
			if body(m, i) {
				return true
			}
			return !m.aborted && next(m, i)
		}
	}
	return c.loop(sub, min, max, mode == modeLazy, next)
}

func (c *compiler) repeatSingle(st stepFn, min, max, mode int, next mfn) mfn {
	switch mode {
	case modeLazy:
		return func(m *matcher, i int) bool {
			j, n := i, 0
			for ; n < min; n++ {
				w := st(m, j)
				if w < 0 {
					return false
				}
				j += w
			}
			for {
				if next(m, j) {
					return true
				}
				if m.aborted || (max >= 0 && n >= max) || m.tick() {
					return false
				}
				w := st(m, j)
				if w < 0 {
					return false
				}
				j += w
				n++
			}
		}
	default:
		poss := mode == modePossessive
		return func(m *matcher, i int) bool {
			j, n := i, 0
			for max < 0 || n < max {
				w := st(m, j)
				if w < 0 {
					break
				}
				j += w
				n++
			}
			if n < min || m.addSteps(n) {
				return false
			}
			if poss {
				return next(m, j)
			}
			for {
				if next(m, j) {
					return true
				}
				if m.aborted || n <= min || m.tick() {
					return false
				}
				n--
				_, w := m.decodeBack(j)
				j -= w
			}
		}
	}
}

// loop compiles a general counted repetition of a complex body.
func (c *compiler) loop(sub *node, min, max int, lazy bool, next mfn) mfn {
	s := c.slot(2) // s: iteration count, s+1: start of current iteration
	var body, try mfn
	bodyNext := func(m *matcher, i int) bool {
		// An empty iteration ends the loop, except in the copies PCRE2
		// unrolls for a counted minimum (those are not loop iterations).
		if c := m.slots[s]; i == m.slots[s+1] && (c > min || (max < 0 && c == min)) {
			return next(m, i)
		}
		return try(m, i)
	}
	body = c.comp(sub, bodyNext)
	if lazy {
		try = func(m *matcher, i int) bool {
			if m.tick() {
				return false
			}
			cnt := m.slots[s]
			if cnt >= min {
				if next(m, i) {
					return true
				}
				if m.aborted {
					return false
				}
			}
			if max >= 0 && cnt >= max {
				return false
			}
			if m.depth++; m.depth > maxDepth {
				m.aborted = true
			}
			os := m.slots[s+1]
			m.slots[s], m.slots[s+1] = cnt+1, i
			ok := !m.aborted && body(m, i)
			m.depth--
			if ok {
				return true
			}
			m.slots[s], m.slots[s+1] = cnt, os
			return false
		}
	} else {
		try = func(m *matcher, i int) bool {
			if m.tick() {
				return false
			}
			cnt := m.slots[s]
			if max < 0 || cnt < max {
				if m.depth++; m.depth > maxDepth {
					m.aborted = true
				}
				os := m.slots[s+1]
				m.slots[s], m.slots[s+1] = cnt+1, i
				ok := !m.aborted && body(m, i)
				m.depth--
				if ok {
					return true
				}
				m.slots[s], m.slots[s+1] = cnt, os
				if m.aborted {
					return false
				}
			}
			return cnt >= min && next(m, i)
		}
	}
	return func(m *matcher, i int) bool {
		oc, os := m.slots[s], m.slots[s+1]
		m.slots[s], m.slots[s+1] = 0, -1
		if try(m, i) {
			return true
		}
		m.slots[s], m.slots[s+1] = oc, os
		return false
	}
}
