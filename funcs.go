package gtfparser

import (
	"errors"
	"math"
	"math/rand"
	"os"
	"strings"
	"time"
)

type fn func(in *Interp, a []Value) (Value, error)

var builtinFuncs = map[string]fn{}

func argN(a []Value, n int, name string) error {
	if len(a) < n {
		return errors.New(name + "(): too few arguments")
	}
	return nil
}

func init() {
	for k, v := range map[string]fn{
		"strlen":  func(in *Interp, a []Value) (Value, error) { return Int(int64(len(str(a, 0)))), argN(a, 1, "strlen") },
		"tolower": func(in *Interp, a []Value) (Value, error) { return Str(strings.ToLower(str(a, 0))), nil },
		"toupper": func(in *Interp, a []Value) (Value, error) { return Str(strings.ToUpper(str(a, 0))), nil },
		"strcat": func(in *Interp, a []Value) (Value, error) {
			var b strings.Builder
			for _, v := range a {
				b.WriteString(v.String())
			}
			return Str(b.String()), nil
		},
		"strrep": func(in *Interp, a []Value) (Value, error) {
			n := int(num(a, 1))
			if n < 0 {
				n = 0
			}
			return Str(strings.Repeat(str(a, 0), n)), nil
		},
		"substr": func(in *Interp, a []Value) (Value, error) {
			s := str(a, 0)
			st := int(num(a, 1))
			if st < 0 {
				st = 0
			}
			if st > len(s) {
				st = len(s)
			}
			end := len(s)
			if len(a) > 2 {
				if l := int(num(a, 2)); st+l < end {
					end = st + l
				}
			}
			if end < st {
				end = st
			}
			return Str(s[st:end]), nil
		},
		"strstr": func(in *Interp, a []Value) (Value, error) {
			return Int(int64(strings.Index(str(a, 0), str(a, 1)))), nil
		},
		"strchr": func(in *Interp, a []Value) (Value, error) {
			return Int(int64(strings.IndexAny(str(a, 0), str(a, 1)))), nil
		},
		"strrchr": func(in *Interp, a []Value) (Value, error) {
			return Int(int64(strings.LastIndexAny(str(a, 0), str(a, 1)))), nil
		},
		"strcmp": func(in *Interp, a []Value) (Value, error) {
			return Int(int64(strings.Compare(str(a, 0), str(a, 1)))), nil
		},
		"strncmp": func(in *Interp, a []Value) (Value, error) {
			x, y, n := str(a, 0), str(a, 1), int(num(a, 2))
			if len(x) > n {
				x = x[:n]
			}
			if len(y) > n {
				y = y[:n]
			}
			return Int(int64(strings.Compare(x, y))), nil
		},
		"replace": func(in *Interp, a []Value) (Value, error) {
			if len(a) < 3 {
				return Str(""), errors.New("replace(): too few arguments")
			}
			return Str(strings.ReplaceAll(str(a, 2), str(a, 0), str(a, 1))), nil
		},
		"abs":   func(in *Interp, a []Value) (Value, error) { return mathN(a, math.Abs, true), nil },
		"sqrt":  func(in *Interp, a []Value) (Value, error) { return Real(math.Sqrt(fnum(a, 0))), nil },
		"pow":   func(in *Interp, a []Value) (Value, error) { return Real(math.Pow(fnum(a, 0), fnum(a, 1))), nil },
		"sin":   func(in *Interp, a []Value) (Value, error) { return Real(math.Sin(fnum(a, 0))), nil },
		"cos":   func(in *Interp, a []Value) (Value, error) { return Real(math.Cos(fnum(a, 0))), nil },
		"exp":   func(in *Interp, a []Value) (Value, error) { return Real(math.Exp(fnum(a, 0))), nil },
		"log":   func(in *Interp, a []Value) (Value, error) { return Real(math.Log(fnum(a, 0))), nil },
		"trunc": func(in *Interp, a []Value) (Value, error) { return Int(int64(fnum(a, 0))), nil },
		"mod": func(in *Interp, a []Value) (Value, error) {
			d := int64(num(a, 1))
			if d == 0 {
				return Int(0), errors.New("mod(): division by zero")
			}
			return Int(int64(num(a, 0)) % d), nil
		},
		"rand": func(in *Interp, a []Value) (Value, error) {
			switch len(a) {
			case 0:
				return Int(rand.Int63()), nil
			case 1:
				if n := int64(num(a, 0)); n > 0 {
					return Int(rand.Int63n(n)), nil
				}
				return Int(0), nil
			}
			lo, hi := int64(num(a, 0)), int64(num(a, 1))
			if hi < lo {
				return Int(lo), nil
			}
			return Int(lo + rand.Int63n(hi-lo+1)), nil
		},
		"ascii": func(in *Interp, a []Value) (Value, error) {
			s := str(a, 0)
			if s == "" {
				return Int(0), nil
			}
			return Int(int64(s[0])), nil
		},
		"char": func(in *Interp, a []Value) (Value, error) { return Str(string(rune(int64(num(a, 0))))), nil },
		"time": func(in *Interp, a []Value) (Value, error) { return Int(time.Now().Unix()), nil },
		"eval": func(in *Interp, a []Value) (Value, error) {
			return in.ExecFull(str(a, 0)), nil
		},
		"regmatch": fnRegmatch,
		"getopts":  fnGetopts,
		"systype":  func(in *Interp, a []Value) (Value, error) { return Str("unix"), nil },
		"cputime": func(in *Interp, a []Value) (Value, error) {
			return Real(float64(time.Now().UnixNano()%1e12) / 1e9), nil
		},
		"acos":   func(in *Interp, a []Value) (Value, error) { return Real(math.Acos(fnum(a, 0))), nil },
		"asin":   func(in *Interp, a []Value) (Value, error) { return Real(math.Asin(fnum(a, 0))), nil },
		"atan":   func(in *Interp, a []Value) (Value, error) { return Real(math.Atan(fnum(a, 0))), nil },
		"getpid": func(in *Interp, a []Value) (Value, error) { return Int(int64(os.Getpid())), nil },
		"echo":   func(in *Interp, a []Value) (Value, error) { in.echo(str(a, 0)); return Int(1), nil }, // echo(text [, attrs [, inline [, dest]]])
		"send":   func(in *Interp, a []Value) (Value, error) { return Bool(in.Host.Send(str(a, 1), str(a, 0))), nil },
	} {
		builtinFuncs[k] = v
	}
}

func str(a []Value, i int) string {
	if i < len(a) {
		return a[i].String()
	}
	return ""
}
func num(a []Value, i int) float64 { return fnum(a, i) }
func fnum(a []Value, i int) float64 {
	if i < len(a) {
		return a[i].Float()
	}
	return 0
}

func mathN(a []Value, f func(float64) float64, keepInt bool) Value {
	if len(a) > 0 {
		if n := a[0].Num(); n.k == kInt && keepInt {
			return Int(int64(f(float64(n.i))))
		}
	}
	return Real(f(fnum(a, 0)))
}

func minmax(a []Value, max bool) (Value, error) {
	if len(a) == 0 {
		return Int(0), errors.New("min/max: no arguments")
	}
	best := a[0].Num()
	for _, v := range a[1:] {
		n := v.Num()
		if (max && n.Float() > best.Float()) || (!max && n.Float() < best.Float()) {
			best = n
		}
	}
	return best, nil
}

// fnRegmatch implements regmatch(pattern, string): on a match it sets the
// %P0.. substitutions for the current scope.
func fnRegmatch(in *Interp, a []Value) (Value, error) {
	re, err := compileRegexp(str(a, 0))
	if err != nil {
		return Int(0), err
	}
	s := str(a, 1)
	loc := re.match(s, 0)
	if loc == nil {
		return Int(0), nil
	}
	var regs []string
	for i := 0; i+1 < len(loc); i += 2 {
		if loc[i] < 0 {
			regs = append(regs, "")
		} else {
			regs = append(regs, s[loc[i]:loc[i+1]])
		}
	}
	in.fr.regs, in.fr.pl, in.fr.pr = regs, s[:loc[0]], s[loc[1]:]
	return Int(int64(len(regs))), nil
}

func log10(x float64) float64 { return math.Log10(x) }
func tan(x float64) float64   { return math.Tan(x) }
