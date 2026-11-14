package gtfparser

import (
	"fmt"
	"sync"

	"github.com/itsnotyoutoday/gtfparser/pcre"
)

// rx is the regexp seam: everything in the interpreter that needs a regular
// expression goes through compileRegexp/match. It is backed by the pcre
// package, a PCRE2-compatible engine configured as TinyFugue configures
// PCRE2 (dot matches newline, "$" matches only at the very end, smart-case,
// UTF-8 with Unicode properties).
type rx struct{ re *pcre.Regexp }

var (
	rxMu    sync.Mutex
	rxCache = map[string]*rx{}
)

// compileRe is compileRegexp plus tf's warning about the '{' whose meaning
// changed in version 5.0 (given each time a pattern is compiled).
func (in *Interp) compileRe(pat string) (*rx, error) {
	if i := firstUnescaped(pat, '{'); i >= 0 && i+1 < len(pat) && (pat[i+1] >= '0' && pat[i+1] <= '9' || pat[i+1] == ',') {
		if in.globals["warn_curly_re"].String() != "off" {
			in.warn("Warning: regexp contains '{', which has a new meaning in version 5.0.  (This warning can be disabled with '/set warn_curly_re=off'.)")
		}
	}
	return compileRegexp(pat)
}

func compileRegexp(pat string) (*rx, error) {
	rxMu.Lock()
	r0, ok := rxCache[pat]
	rxMu.Unlock()
	if ok {
		return r0, nil
	}
	re, err := pcre.CompileTF(pat)
	if err != nil {
		if pe, ok := err.(*pcre.Error); ok {
			return nil, fmt.Errorf("regexp error: character %d: %s", pe.Offset, pe.Msg)
		}
		return nil, err
	}
	r := &rx{re}
	rxMu.Lock()
	if len(rxCache) > 2048 {
		rxCache = map[string]*rx{}
	}
	rxCache[pat] = r
	rxMu.Unlock()
	return r, nil
}

// match returns the capture offsets (byte pairs, -1 = unset) or nil.
func (r *rx) match(s string, start int) []int {
	return r.re.Match(s, start, start > 0)
}
