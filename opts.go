package gtfparser

import (
	"fmt"
	"strings"
)

func cmdShift(in *Interp, args string) Value {
	n := 1
	if w, _ := firstWord(args); w != "" {
		n = int(Str(w).Int())
	}
	in.shiftArgs(n)
	return Int(1)
}

func (in *Interp) shiftArgs(n int) {
	f := in.fr
	if n >= len(f.words) {
		f.setArgs("")
		return
	}
	f.setArgs(f.text[f.offs[n]:])
}

// parseOptArg reads an option argument starting at s[i]: a quoted string
// (with \ escapes) or a run up to the next blank. Returns the value and the
// index after it.
func parseOptArg(s string, i int) (string, int) {
	if i < len(s) && (s[i] == '"' || s[i] == '\'' || s[i] == '`') {
		q := s[i]
		var b strings.Builder
		j := i + 1
		for j < len(s) && s[j] != q {
			if s[j] == '\\' && j+1 < len(s) {
				j++
			}
			b.WriteByte(s[j])
			j++
		}
		if j < len(s) {
			j++
		}
		return b.String(), j
	}
	j := i
	for j < len(s) && s[j] != ' ' && s[j] != '\t' {
		j++
	}
	return s[i:j], j
}

// getopts(options [, init]) parses leading -x options of the current macro's
// arguments into local variables opt_x, then shifts them off.
func fnGetopts(in *Interp, a []Value) (Value, error) {
	spec := str(a, 0)
	f := in.fr
	kinds := map[byte]byte{}
	for i := 0; i < len(spec); i++ {
		c := spec[i]
		kinds[c] = 0
		if i+1 < len(spec) && (spec[i+1] == ':' || spec[i+1] == '#' || spec[i+1] == '@') {
			kinds[c] = spec[i+1]
			i++
		}
	}
	if len(a) > 1 {
		for c := range kinds {
			f.vars["opt_"+string(c)] = a[1]
		}
	}
	text := f.text
	i := 0
	skip := func() {
		for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
			i++
		}
	}
	bad := func(format string, args ...any) (Value, error) {
		in.warn("%s: "+format, append([]any{f.name}, args...)...)
		var parts []string
		var flags string
		for c, k := range kinds {
			if k == 0 {
				flags += string(c)
			}
		}
		if flags != "" {
			parts = append(parts, "-"+flags)
		}
		for c, k := range kinds {
			switch k {
			case ':':
				parts = append(parts, fmt.Sprintf("-%c<string>", c))
			case '#':
				parts = append(parts, fmt.Sprintf("-%c<integer>", c))
			case '@':
				parts = append(parts, fmt.Sprintf("-%c<time>", c))
			}
		}
		in.warn("%s: options: %s", f.name, strings.Join(parts, " "))
		return Int(0), nil
	}
	for {
		skip()
		if i >= len(text) || text[i] != '-' || i+1 >= len(text) || text[i+1] == ' ' {
			break
		}
		if strings.HasPrefix(text[i:], "--") && (i+2 == len(text) || text[i+2] == ' ' || text[i+2] == '\t') {
			i += 2
			skip()
			break
		}
		i++
		for i < len(text) && text[i] != ' ' && text[i] != '\t' {
			c := text[i]
			k, ok := kinds[c]
			if !ok {
				return bad("invalid option '%c'", c)
			}
			i++
			if k == 0 {
				f.vars["opt_"+string(c)] = Int(1)
				continue
			}
			val, j := parseOptArg(text, i)
			i = j
			switch k {
			case '#':
				f.vars["opt_"+string(c)] = Int(Str(val).Int())
			case '@':
				f.vars["opt_"+string(c)] = Str(val).Num()
			default:
				f.vars["opt_"+string(c)] = Str(val)
			}
			break
		}
	}
	f.setArgs(text[i:])
	return Int(1), nil
}
