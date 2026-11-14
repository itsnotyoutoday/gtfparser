// Package gtfparser is an interpreter for the TinyFugue scripting language,
// written from TinyFugue's published documentation and its observable
// behaviour. It is meant to be embedded (squatterd, squatclient) and to back
// a standalone terminal client (cmd/gtf).
package gtfparser

import (
	"math"
	"math/big"
	"strconv"
	"strings"
	"sync/atomic"
)

type kind uint8

const (
	kInt kind = iota
	kReal
	kStr
	kDec // fixed-point decimal: microseconds in dec (TF's "decimal"/time type)
)

// Value is a TF value: integer, float, decimal (fixed point, 6 places; what
// TF uses for times and for literals like 1.5), or string.
type Value struct {
	k   kind
	i   int64
	f   float64
	s   string
	dec *big.Int // kDec: value * 1e6
	en  int      // enumerated variable value: 1 + index into its list (numeric value is the index)
}

func Int(n int64) Value    { return Value{k: kInt, i: n} }
func Real(f float64) Value { return Value{k: kReal, f: f} }
func Str(s string) Value   { return Value{k: kStr, s: s} }
func Bool(b bool) Value {
	if b {
		return Int(1)
	}
	return Int(0)
}

var million = big.NewInt(1000000)

func decMicro(m *big.Int) Value { return Value{k: kDec, dec: m} }

// microOf is the fixed-point value of an int or decimal.
func (v Value) microOf() *big.Int {
	if v.k == kDec {
		return v.dec
	}
	return new(big.Int).Mul(big.NewInt(v.i), million)
}

var sigFigs atomic.Int32 // %sigfigs; process-wide (it is a display setting)

func init() { sigFigs.Store(15) }

func setSigFigs(n int) { sigFigs.Store(int32(n)) }
func getSigFigs() int  { return int(sigFigs.Load()) }

func (v Value) String() string {
	switch v.k {
	case kInt:
		return strconv.FormatInt(v.i, 10)
	case kReal:
		return formatReal(v.f)
	case kDec:
		return formatDec(v.dec)
	}
	return v.s
}

// formatDec prints seconds.microseconds with trailing zeros trimmed, keeping
// at least one fractional digit ("7.0", "1.5").
func formatDec(m *big.Int) string {
	neg := m.Sign() < 0
	a := new(big.Int).Abs(m)
	q, r := new(big.Int).QuoRem(a, million, new(big.Int))
	frac := strings.TrimRight(leftPad(r.String(), 6), "0")
	if frac == "" {
		frac = "0"
	}
	s := q.String() + "." + frac
	if neg {
		s = "-" + s
	}
	return s
}

func leftPad(s string, n int) string {
	for len(s) < n {
		s = "0" + s
	}
	return s
}

// formatReal prints a float as C's "%.*g" with %sigfigs does, plus the '.'
// TF appends when the result looks like an integer.
func formatReal(f float64) string {
	s := formatReal0(f)
	if !strings.ContainsAny(s, ".eEn") {
		s += "."
	}
	return s
}

func formatReal0(f float64) string {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return strconv.FormatFloat(f, 'g', -1, 64)
	}
	if f == 0 {
		return "0"
	}
	return strconv.FormatFloat(f, 'g', getSigFigs(), 64)
}

// Num converts to a number (int, decimal or float); strings are parsed by prefix.
func (v Value) Num() Value {
	if v.en > 0 {
		return Int(int64(v.en - 1))
	}
	if v.k != kStr {
		return v
	}
	return parseNumPrefix(v.s)
}

func (v Value) Float() float64 {
	n := v.Num()
	switch n.k {
	case kInt:
		return float64(n.i)
	case kDec:
		f, _ := new(big.Float).Quo(new(big.Float).SetInt(n.dec), big.NewFloat(1e6)).Float64()
		return f
	}
	return n.f
}

func (v Value) Int() int64 {
	n := v.Num()
	switch n.k {
	case kInt:
		return n.i
	case kDec:
		return new(big.Int).Quo(n.dec, million).Int64()
	}
	return int64(n.f)
}

func (v Value) Truth() bool {
	n := v.Num()
	switch n.k {
	case kInt:
		return n.i != 0
	case kDec:
		return n.dec.Sign() != 0
	}
	return n.f != 0
}

// decFromText builds a decimal from digits: sign, integer part, fraction
// (at most 6 digits).
func decFromText(neg bool, ip, fp string) Value {
	if ip == "" {
		ip = "0"
	}
	micro, _ := new(big.Int).SetString(ip+leftPadRight(fp, 6), 10)
	if neg {
		micro.Neg(micro)
	}
	return decMicro(micro)
}

func leftPadRight(s string, n int) string {
	for len(s) < n {
		s += "0"
	}
	return s
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// parseNumPrefix reads a leading integer, decimal, float or dtime (h:m[:s])
// from s. Anything else is 0.
func parseNumPrefix(s string) Value {
	s = strings.TrimLeft(s, " \t")
	i := 0
	neg := false
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		neg = s[i] == '-'
		i++
	}
	ds := i
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	intEnd := i
	if intEnd == ds && !(i < len(s) && s[i] == '.' && i+1 < len(s) && isDigit(s[i+1])) {
		return Int(0)
	}
	// dtime: digits ':' digits [':' digits[.frac]]
	if i < len(s) && s[i] == ':' && i+1 < len(s) && isDigit(s[i+1]) {
		var parts []string
		j := ds
		fracStr := ""
		for len(parts) < 3 {
			k := j
			for k < len(s) && isDigit(s[k]) {
				k++
			}
			parts = append(parts, s[j:k])
			j = k
			if len(parts) == 3 && j < len(s) && s[j] == '.' {
				k = j + 1
				for k < len(s) && isDigit(s[k]) {
					k++
				}
				fracStr = s[j+1 : k]
				j = k
			}
			if len(parts) < 3 && j+1 < len(s) && s[j] == ':' && isDigit(s[j+1]) {
				j++
				continue
			}
			break
		}
		secs := new(big.Int)
		for _, p := range parts {
			n, _ := new(big.Int).SetString(p, 10)
			secs.Mul(secs, big.NewInt(60)).Add(secs, n)
		}
		if len(parts) == 2 { // h:m: the pair was accumulated as minutes
			secs.Mul(secs, big.NewInt(60))
		}
		if len(fracStr) > 6 {
			fracStr = fracStr[:6]
		}
		v := decFromText(neg, secs.String(), fracStr)
		return v
	}
	ip := s[ds:intEnd]
	frac := ""
	hasDot := false
	if i < len(s) && s[i] == '.' {
		hasDot = true
		k := i + 1
		for k < len(s) && isDigit(s[k]) {
			k++
		}
		frac = s[i+1 : k]
		i = k
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		j := i + 1
		if j < len(s) && (s[j] == '+' || s[j] == '-') {
			j++
		}
		if j < len(s) && isDigit(s[j]) {
			for j < len(s) && isDigit(s[j]) {
				j++
			}
			f, _ := strconv.ParseFloat(s[:j], 64)
			return Real(f)
		}
	}
	if hasDot {
		if len(frac) <= 6 {
			return decFromText(neg, ip, frac)
		}
		f, _ := strconv.ParseFloat(s[:i], 64)
		return Real(f)
	}
	n, err := strconv.ParseInt(s[:intEnd], 10, 64)
	if err != nil {
		f, _ := strconv.ParseFloat(s[:intEnd], 64)
		return Real(f)
	}
	return Int(n)
}
