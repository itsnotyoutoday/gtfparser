package pcre

import (
	"sort"
	"strings"
	"sync"
	"unicode"
)

type pred struct {
	f   func(rune) bool
	neg bool
}

// charSet is a character class: ranges and predicates, optionally negated
// and caseless.
type charSet struct {
	neg    bool
	fold   bool
	utf    bool
	ranges []rune // lo,hi pairs
	preds  []pred
	ascii  [128]bool
}

func (c *charSet) addRange(lo, hi rune) { c.ranges = append(c.ranges, lo, hi) }
func (c *charSet) add(r rune)           { c.ranges = append(c.ranges, r, r) }
func (c *charSet) addPred(f func(rune) bool, neg bool) {
	c.preds = append(c.preds, pred{f, neg})
}

func (c *charSet) inRanges(r rune) bool {
	for i := 0; i < len(c.ranges); i += 2 {
		if r >= c.ranges[i] && r <= c.ranges[i+1] {
			return true
		}
	}
	return false
}

func (c *charSet) inPreds(r rune) bool {
	for _, p := range c.preds {
		if p.f(r) != p.neg {
			return true
		}
	}
	return false
}

// slow evaluates membership.  Caseless matching folds the explicit
// characters and ranges only; predicates (\w, \p{..}, POSIX classes) are
// taken as they are, as PCRE2 does.
func (c *charSet) slow(r rune) bool {
	in := c.inRanges(r) || c.inPreds(r)
	if !in && c.fold {
		for _, f := range foldOrbit(r, c.utf) {
			if f != r && c.inRanges(f) {
				in = true
				break
			}
		}
	}
	return in != c.neg
}

func (c *charSet) finalize() {
	for r := rune(0); r < 128; r++ {
		c.ascii[r] = c.slow(r)
	}
}

func (c *charSet) has(r rune) bool {
	if r < 128 {
		return c.ascii[r]
	}
	return c.slow(r)
}

// foldOrbit returns r and every rune that case-folds to the same thing.
func foldOrbit(r rune, utf bool) []rune {
	if !utf && r >= 128 {
		return []rune{r}
	}
	orb := []rune{r}
	for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
		if !utf && f >= 128 {
			continue
		}
		orb = append(orb, f)
	}
	return orb
}

// ---- property tables --------------------------------------------------

// rtab is a sorted list of lo,hi code point pairs.
type rtab []rune

func (t rtab) has(r rune) bool {
	n := len(t) / 2
	i := sort.Search(n, func(i int) bool { return t[2*i+1] >= r })
	return i < n && t[2*i] <= r
}

func mergeTabs(ts ...rtab) rtab {
	var pairs [][2]rune
	for _, t := range ts {
		for i := 0; i < len(t); i += 2 {
			pairs = append(pairs, [2]rune{t[i], t[i+1]})
		}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i][0] < pairs[j][0] })
	var out rtab
	for _, p := range pairs {
		if n := len(out); n > 0 && p[0] <= out[n-1]+1 {
			if p[1] > out[n-1] {
				out[n-1] = p[1]
			}
			continue
		}
		out = append(out, p[0], p[1])
	}
	return out
}

var (
	tabOnce sync.Once
	cats    map[string]rtab // normalised category name -> table (incl. one-letter groups)
	scx, sc map[string]rtab
	tWord   rtab // \w beyond ASCII: L N Mn Pc
	tSpace  rtab // \s beyond ASCII
	tGraph  rtab
	tLC     rtab
)

var scriptAlias = map[string]string{
	"latn": "latin", "grek": "greek", "cyrl": "cyrillic", "hani": "han", "hira": "hiragana",
	"kana": "katakana", "hang": "hangul", "arab": "arabic", "hebr": "hebrew", "zyyy": "common",
	"zinh": "inherited", "qaai": "inherited", "armn": "armenian", "geor": "georgian", "deva": "devanagari",
	"beng": "bengali", "thai": "thai", "laoo": "lao", "tibt": "tibetan", "mong": "mongolian",
	"ethi": "ethiopic", "khmr": "khmer", "taml": "tamil", "telu": "telugu", "knda": "kannada",
	"mlym": "malayalam", "sinh": "sinhala", "gujr": "gujarati", "guru": "gurmukhi", "orya": "oriya",
	"mymr": "myanmar", "bopo": "bopomofo", "copt": "coptic",
}

func initTabs() {
	cats = map[string]rtab{}
	for k, v := range genCats {
		cats[k] = rtab(v)
	}
	cats["cs"] = rtab{0xd800, 0xdfff}
	group := func(letter string, subs ...string) {
		var ts []rtab
		for _, s := range subs {
			ts = append(ts, cats[s])
		}
		cats[letter] = mergeTabs(ts...)
	}
	group("l", "lu", "ll", "lt", "lm", "lo")
	group("m", "mn", "mc", "me")
	group("n", "nd", "nl", "no")
	group("p", "pc", "pd", "ps", "pe", "pi", "pf", "po")
	group("s", "sm", "sc", "sk", "so")
	group("z", "zs", "zl", "zp")
	group("c", "cc", "cf", "co", "cn", "cs")
	scx, sc = map[string]rtab{}, map[string]rtab{}
	for k, v := range genScx {
		scx[k] = rtab(v)
	}
	for k, v := range genSc {
		sc[k] = rtab(v)
	}
	tWord = mergeTabs(cats["l"], cats["n"], cats["mn"], cats["pc"])
	tSpace = mergeTabs(cats["z"], rtab{0x09, 0x0d, 0x85, 0x85})
	tLC = mergeTabs(cats["lu"], cats["ll"], cats["lt"])
	tGraph = mergeTabs(cats["l"], cats["m"], cats["n"], cats["p"], cats["s"], cats["cf"])
}

// ---- character type predicates --------------------------------------

func isHSpace(r rune) bool {
	switch r {
	case 9, 0x20, 0xa0, 0x1680, 0x180e, 0x202f, 0x205f, 0x3000:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

func isVSpace(r rune) bool {
	return (r >= 0x0a && r <= 0x0d) || r == 0x85 || r == 0x2028 || r == 0x2029
}

func isASCIIWord(r rune) bool {
	return r == '_' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isUWord(r rune) bool {
	if r < 128 {
		return isASCIIWord(r)
	}
	tabOnce.Do(initTabs)
	return tWord.has(r)
}

func isASCIISpace(r rune) bool { return r == ' ' || (r >= 9 && r <= 13) }
func isUSpace(r rune) bool {
	if r < 128 {
		return isASCIISpace(r)
	}
	tabOnce.Do(initTabs)
	return tSpace.has(r) || isHSpace(r) || isVSpace(r)
}
func isASCIIDigit(r rune) bool { return r >= '0' && r <= '9' }
func isUDigit(r rune) bool {
	if r < 128 {
		return isASCIIDigit(r)
	}
	tabOnce.Do(initTabs)
	return cats["nd"].has(r)
}

func catFn(names ...string) func(rune) bool {
	return func(r rune) bool {
		tabOnce.Do(initTabs)
		for _, n := range names {
			if cats[n].has(r) {
				return true
			}
		}
		return false
	}
}

// escapeFn returns the predicate for \d \s \w \h \v (lower-case letter).
func escapeFn(c byte, ucp bool) func(rune) bool {
	switch c {
	case 'd':
		if ucp {
			return isUDigit
		}
		return isASCIIDigit
	case 's':
		if ucp {
			return isUSpace
		}
		return isASCIISpace
	case 'w':
		if ucp {
			return isUWord
		}
		return isASCIIWord
	case 'h':
		return isHSpace
	case 'v':
		return isVSpace
	}
	return nil
}

func isXDigit(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

func isUGraph(r rune) bool {
	if r == 0x061c || r == 0x180e || (r >= 0x2066 && r <= 0x2069) {
		return false
	}
	tabOnce.Do(initTabs)
	return tGraph.has(r)
}

func isAlpha(r rune) bool { return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') }

// posixFn returns the predicate for a POSIX class name.
func posixFn(name string, ucp bool) func(rune) bool {
	if !ucp {
		switch name {
		case "alpha":
			return isAlpha
		case "lower":
			return func(r rune) bool { return r >= 'a' && r <= 'z' }
		case "upper":
			return func(r rune) bool { return r >= 'A' && r <= 'Z' }
		case "alnum":
			return func(r rune) bool { return isAlpha(r) || isASCIIDigit(r) }
		case "digit":
			return isASCIIDigit
		case "xdigit":
			return isXDigit
		case "space":
			return isASCIISpace
		case "blank":
			return func(r rune) bool { return r == ' ' || r == '\t' }
		case "cntrl":
			return func(r rune) bool { return r < 32 || r == 127 }
		case "graph":
			return func(r rune) bool { return r > 32 && r < 127 }
		case "print":
			return func(r rune) bool { return r >= 32 && r < 127 }
		case "punct":
			return func(r rune) bool { return r > 32 && r < 127 && !isAlpha(r) && !isASCIIDigit(r) }
		case "word":
			return isASCIIWord
		case "ascii":
			return func(r rune) bool { return r < 128 }
		}
		return nil
	}
	switch name {
	case "alpha":
		return catFn("l")
	case "lower":
		return catFn("ll")
	case "upper":
		return catFn("lu")
	case "alnum":
		return catFn("l", "n")
	case "digit":
		return isUDigit
	case "xdigit":
		return func(r rune) bool { // Unicode Hex_Digit includes the fullwidth forms
			return isXDigit(r) || (r >= 0xff10 && r <= 0xff19) || (r >= 0xff21 && r <= 0xff26) || (r >= 0xff41 && r <= 0xff46)
		}
	case "space":
		return isUSpace
	case "blank":
		return isHSpace
	case "cntrl":
		return catFn("cc")
	case "graph":
		return isUGraph
	case "print":
		return func(r rune) bool { return isUGraph(r) || catFn("zs")(r) }
	case "punct":
		p := catFn("p")
		s := catFn("s")
		return func(r rune) bool { return p(r) || (r < 128 && s(r)) }
	case "word":
		return isUWord
	case "ascii":
		// Under UCP PCRE2 takes [:ascii:] from the locale-built character
		// tables (tf passes pcre2_maketables), which in a UTF-8 locale
		// cover every code point below 256.
		return func(r rune) bool { return r < 256 }
	}
	return nil
}

// ---- Unicode properties ---------------------------------------------

func normProp(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == ' ' || r == '_' || r == '-' {
			continue
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

func tabFn(t rtab) func(rune) bool { return t.has }

// propFn resolves a \p{name} property; ok is false for unknown names.
func propFn(name string, caseless bool) (func(rune) bool, bool) {
	tabOnce.Do(initTabs)
	scriptOnly := false
	if i := strings.IndexAny(name, ":="); i >= 0 {
		switch strings.ToLower(name[:i]) {
		case "sc", "script":
			scriptOnly = true
			name = name[i+1:]
		case "scx", "scriptextensions", "gc", "generalcategory", "category":
			name = name[i+1:]
		}
	}
	n := normProp(name)
	switch n {
	case "any":
		return func(rune) bool { return true }, true
	case "l&", "lc":
		return tabFn(tLC), true
	case "xan":
		return catFn("l", "n"), true
	case "xwd":
		return isUWord, true
	case "xsp", "xps":
		return isUSpace, true
	case "xuc":
		return func(r rune) bool {
			return r == '$' || r == '@' || r == '`' || (r >= 0xa0 && r <= 0xd7ff) || r >= 0xe000
		}, true
	}
	if t, ok := cats[n]; ok && !scriptOnly {
		if caseless && (n == "lu" || n == "ll" || n == "lt") {
			return tabFn(tLC), true
		}
		return tabFn(t), true
	}
	if a, ok := scriptAlias[n]; ok {
		n = a
	}
	if scriptOnly {
		if t, ok := sc[n]; ok {
			return tabFn(t), true
		}
	}
	if t, ok := scx[n]; ok {
		return tabFn(t), true
	}
	switch n {
	case "alphabetic":
		return func(r rune) bool {
			return unicode.IsLetter(r) || unicode.Is(unicode.Nl, r) || unicode.Is(unicode.Other_Alphabetic, r)
		}, true
	case "lowercase":
		return func(r rune) bool { return unicode.Is(unicode.Ll, r) || unicode.Is(unicode.Other_Lowercase, r) }, true
	case "uppercase":
		return func(r rune) bool { return unicode.Is(unicode.Lu, r) || unicode.Is(unicode.Other_Uppercase, r) }, true
	case "whitespace":
		return unicode.IsSpace, true
	}
	return nil, false
}
