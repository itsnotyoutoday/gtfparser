package gtfparser

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// String functions as tf's UTF-8 build implements them (expr.c): lengths and
// offsets count characters; a stray byte that is not part of a valid
// sequence counts as one character.

func (in *Interp) tooBig(string) error {
	return fmt.Errorf("result would exceed the string size limit (%d bytes)", in.maxString())
}

func u8clen(s string, i int) int {
	n := 1
	if s[i] >= 0xC0 {
		for i+n < len(s) && s[i+n]&0xC0 == 0x80 {
			n++
		}
	}
	return n
}

func u8nchars(s string) int {
	c := 0
	for b := 0; b < len(s); b += u8clen(s, b) {
		c++
	}
	return c
}

// u8byteoff is the byte offset of character number c (clamped to len(s)).
func u8byteoff(s string, c int) int {
	b := 0
	for ; c > 0 && b < len(s); c-- {
		b += u8clen(s, b)
	}
	return b
}

func u8inset(c, set string) bool {
	for b := 0; b < len(set); b += u8clen(set, b) {
		if set[b:b+u8clen(set, b)] == c {
			return true
		}
	}
	return false
}

// boundCheck is bound_check() in expr.c: clamp to max, negative counts from max.
func boundCheck(v, max int64) int64 {
	if v > max {
		return max
	}
	if v < 0 {
		v += max
		if v < 0 {
			v = 0
		}
	}
	return v
}

// optArg is optional_int_arg(): argument number n (1-based) if present.
func optArg(a []Value, n int, max, def int64) int64 {
	if len(a) >= n {
		return boundCheck(a[n-1].Int(), max)
	}
	return def
}

func fnStrlen(in *Interp, a []Value) (Value, error) {
	return Int(int64(u8nchars(str(a, 0)))), nil
}

func fnSubstr(in *Interp, a []Value) (Value, error) {
	s := str(a, 0)
	clen := int64(u8nchars(s))
	i := boundCheck(a[1].Int(), clen)
	j := optArg(a, 3, clen-i, clen-i)
	bs := u8byteoff(s, int(i))
	be := u8byteoff(s, int(i+j))
	return Str(s[bs:be]), nil
}

func fnStrstr(in *Interp, a []Value) (Value, error) {
	s := str(a, 0)
	j := optArg(a, 3, int64(u8nchars(s)), 0)
	off := u8byteoff(s, int(j))
	k := strings.Index(s[off:], str(a, 1))
	if k < 0 {
		return Int(-1), nil
	}
	return Int(int64(u8nchars(s[:off+k]))), nil
}

func fnStrchr(in *Interp, a []Value) (Value, error) {
	s, set := str(a, 0), str(a, 1)
	j := optArg(a, 3, int64(u8nchars(s)), 0)
	for b := u8byteoff(s, int(j)); b < len(s); j++ {
		l := u8clen(s, b)
		if u8inset(s[b:b+l], set) {
			return Int(j), nil
		}
		b += l
	}
	return Int(-1), nil
}

func fnStrrchr(in *Interp, a []Value) (Value, error) {
	s, set := str(a, 0), str(a, 1)
	clen := int64(u8nchars(s))
	i := optArg(a, 3, clen-1, clen-1)
	for ; i >= 0; i-- {
		b := u8byteoff(s, int(i))
		if b >= len(s) {
			continue
		}
		if u8inset(s[b:b+u8clen(s, b)], set) {
			return Int(i), nil
		}
	}
	return Int(-1), nil
}

// cStrcmp is strcmp(3) as the reference libc returns it: the difference of
// the first differing bytes.
func cStrcmp(x, y string, n int) int64 {
	for i := 0; n < 0 || i < n; i++ {
		var cx, cy int
		if i < len(x) {
			cx = int(x[i])
		}
		if i < len(y) {
			cy = int(y[i])
		}
		if cx != cy {
			return int64(cx - cy)
		}
		if cx == 0 {
			break
		}
	}
	return 0
}

func fnStrcmp(in *Interp, a []Value) (Value, error) {
	return Int(cStrcmp(str(a, 0), str(a, 1), -1)), nil
}

func fnStrncmp(in *Interp, a []Value) (Value, error) {
	n := int(int32(a[2].Int()))
	if n == 0 {
		return Int(0), nil
	}
	return Int(cStrcmp(str(a, 0), str(a, 1), n)), nil // a negative count is a huge size_t
}

func fnStrrep(in *Interp, a []Value) (Value, error) {
	n := a[1].Int()
	s := str(a, 0)
	if n <= 0 || s == "" {
		return Str(""), nil
	}
	if n > in.maxString() || int64(len(s))*n > in.maxString() {
		return Str(""), in.tooBig("strrep")
	}
	return Str(strings.Repeat(s, int(n))), nil
}

func fnPad(in *Interp, a []Value) (Value, error) {
	var b strings.Builder
	for k := 0; k < len(a); k += 2 {
		s := a[k].String()
		clen := int64(u8nchars(s))
		var w int64
		if k+1 < len(a) {
			w = a[k+1].Int()
		}
		if w > in.maxString() || -w > in.maxString() || int64(b.Len())+clen+max(w, -w) > in.maxString() {
			return Str(""), in.tooBig("pad")
		}
		if w > clen {
			b.WriteString(strings.Repeat(" ", int(w-clen)))
		}
		b.WriteString(s)
		if -w > clen {
			b.WriteString(strings.Repeat(" ", int(-w-clen)))
		}
	}
	return Str(b.String()), nil
}

// caseFunc is tolower()/toupper(): simple one-to-one Unicode case mapping
// (tf maps single bytes through the C library, which corrupts multibyte
// text); the optional count is in characters, a negative one counts from the
// end like substr().
func caseFunc(upper bool) fn {
	return func(in *Interp, a []Value) (Value, error) {
		s := str(a, 0)
		n := int64(u8nchars(s))
		j := optArg(a, 2, n, n)
		var b strings.Builder
		var i int64
		for p := 0; p < len(s); i++ {
			l := u8clen(s, p)
			ch := s[p : p+l]
			if i < j {
				if r, size := utf8.DecodeRuneInString(ch); r != utf8.RuneError || size > 1 {
					if upper {
						r = unicode.ToUpper(r)
					} else {
						r = unicode.ToLower(r)
					}
					ch = string(r)
				} else if l == 1 && s[p] < 0x80 {
					ch = string(rune(cCaseASCII(s[p], upper)))
				}
			}
			b.WriteString(ch)
			p += l
		}
		return Str(b.String()), nil
	}
}

func cCaseASCII(c byte, upper bool) byte {
	if upper && c >= 'a' && c <= 'z' {
		return c - 32
	}
	if !upper && c >= 'A' && c <= 'Z' {
		return c + 32
	}
	return c
}

func fnReplace(in *Interp, a []Value) (Value, error) {
	old, nw, s := str(a, 0), str(a, 1), str(a, 2)
	if old == "" {
		return Str(s), nil
	}
	if len(nw) > len(old) && int64(len(s))/int64(len(old))*int64(len(nw)) > in.maxString() {
		return Str(""), in.tooBig("replace")
	}
	return Str(strings.ReplaceAll(s, old, nw)), nil
}

// char(n): below 256 the single byte n (what tf does; 128-255 is not valid
// UTF-8 on its own), from 256 the Unicode character (tf wraps modulo 256).
func fnChar(in *Interp, a []Value) (Value, error) {
	n := a[0].Int()
	switch {
	case n == 0:
		return Str(""), nil
	case n >= 256 && n <= unicode.MaxRune && !(n >= 0xD800 && n <= 0xDFFF):
		return Str(string(rune(n))), nil
	case n < 0 || n >= 256:
		return Str(""), nil
	}
	return Str(string([]byte{byte(n)})), nil
}

func fnAscii(in *Interp, a []Value) (Value, error) {
	s := str(a, 0)
	if s == "" {
		return Int(0), nil
	}
	return Int(int64(s[0])), nil
}

func fnMod(in *Interp, a []Value) (Value, error) {
	for _, v := range a {
		if n := v.Num(); n.k == kReal && (math.IsNaN(n.f) || math.Abs(n.f) >= 1<<63) {
			in.warn("real value too large to convert to integer")
			break
		}
	}
	d := a[1].Int()
	if d == 0 {
		return Int(0), errors.New("division by zero")
	}
	return Int(a[0].Int() % d), nil
}

func fnAbs(in *Interp, a []Value) (Value, error) {
	n := a[0].Num()
	if n.k == kInt {
		if n.i < 0 {
			return Int(-n.i), nil
		}
		return Int(n.i), nil
	}
	return Real(math.Abs(a[0].FloatC())), nil
}

// FloatC converts as C's strtod() does for strings (hex, inf, nan, stops at
// the first character that is not part of the number), which is what tf's
// math functions use; numbers convert as usual.
func (v Value) FloatC() float64 {
	if v.k == kStr && v.en == 0 {
		return strtod(v.s)
	}
	return v.Float()
}

func strtod(s string) float64 {
	s = strings.TrimLeft(s, " \t\n\v\f\r")
	i := 0
	neg := false
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		neg = s[i] == '-'
		i++
	}
	apply := func(f float64) float64 {
		if neg {
			return -f
		}
		return f
	}
	rest := strings.ToLower(s[i:])
	switch {
	case strings.HasPrefix(rest, "infinity"), strings.HasPrefix(rest, "inf"):
		return apply(math.Inf(1))
	case strings.HasPrefix(rest, "nan"):
		return apply(math.NaN())
	case strings.HasPrefix(rest, "0x") && len(rest) > 2:
		j := 2
		var m float64
		any := false
		for j < len(rest) && isHex(rest[j]) {
			m = m*16 + float64(hexVal(rest[j]))
			j++
			any = true
		}
		if j < len(rest) && rest[j] == '.' {
			sc := 1.0 / 16
			j++
			for j < len(rest) && isHex(rest[j]) {
				m += float64(hexVal(rest[j])) * sc
				sc /= 16
				j++
				any = true
			}
		}
		if !any {
			return 0
		}
		if j < len(rest) && rest[j] == 'p' {
			k := j + 1
			if k < len(rest) && (rest[k] == '+' || rest[k] == '-') {
				k++
			}
			if k < len(rest) && isDigit(rest[k]) {
				e, _ := strconv.Atoi(rest[j+1 : scanDigits(rest, k)])
				m = math.Ldexp(m, e)
			}
		}
		return apply(m)
	}
	j := i
	for j < len(s) && isDigit(s[j]) {
		j++
	}
	if j < len(s) && s[j] == '.' {
		j++
		for j < len(s) && isDigit(s[j]) {
			j++
		}
	}
	digits := j - i
	if digits == 0 || (digits == 1 && s[i] == '.') {
		return 0
	}
	if j < len(s) && (s[j] == 'e' || s[j] == 'E') {
		k := j + 1
		if k < len(s) && (s[k] == '+' || s[k] == '-') {
			k++
		}
		if k < len(s) && isDigit(s[k]) {
			j = scanDigits(s, k)
		}
	}
	f, _ := strconv.ParseFloat(s[i:j], 64)
	return apply(f)
}

func scanDigits(s string, k int) int {
	for k < len(s) && isDigit(s[k]) {
		k++
	}
	return k
}

func isHex(c byte) bool {
	return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func hexVal(c byte) int {
	switch {
	case isDigit(c):
		return int(c - '0')
	case c >= 'a':
		return int(c-'a') + 10
	}
	return int(c-'A') + 10
}

func init() {
	f := builtinFuncs
	f["strlen"] = fnStrlen
	f["substr"] = fnSubstr
	f["strstr"] = fnStrstr
	f["strchr"] = fnStrchr
	f["strrchr"] = fnStrrchr
	f["strcmp"] = fnStrcmp
	f["strncmp"] = fnStrncmp
	f["strrep"] = fnStrrep
	f["pad"] = fnPad
	f["tolower"] = caseFunc(false)
	f["toupper"] = caseFunc(true)
	f["replace"] = fnReplace
	f["char"] = fnChar
	f["ascii"] = fnAscii
	f["mod"] = fnMod
	f["abs"] = fnAbs
	f["strcat"] = func(in *Interp, a []Value) (Value, error) {
		var b strings.Builder
		for _, v := range a {
			b.WriteString(v.String())
		}
		return Str(b.String()), nil
	}
	f["trunc"] = func(in *Interp, a []Value) (Value, error) { return Int(a[0].Int()), nil }
}
