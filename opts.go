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
// parseOptions parses leading -x options of text according to spec (TF's
// getopts syntax: a letter; a letter followed by ':' takes a string, '#' an
// integer, '@' a time). It returns the options found, the remaining text
// and, on failure, the error and the usage line, as real tf words them.
func parseOptions(spec, text string) (opts map[byte]Value, rest string, errMsg, usage string) {
	kinds := map[byte]byte{}
	var order []byte
	for i := 0; i < len(spec); i++ {
		c := spec[i]
		kinds[c] = 0
		order = append(order, c)
		if i+1 < len(spec) && (spec[i+1] == ':' || spec[i+1] == '#' || spec[i+1] == '@') {
			kinds[c] = spec[i+1]
			i++
		}
	}
	opts = map[byte]Value{}
	usageLine := func() string {
		var flags string
		var parts []string
		for _, c := range order {
			if kinds[c] == 0 {
				flags += string(c)
			}
		}
		if flags != "" {
			parts = append(parts, "-"+flags)
		}
		for _, c := range order {
			switch kinds[c] {
			case ':':
				parts = append(parts, fmt.Sprintf("-%c<string>", c))
			case '#':
				parts = append(parts, fmt.Sprintf("-%c<integer>", c))
			case '@':
				parts = append(parts, fmt.Sprintf("-%c<time>", c))
			}
		}
		return strings.Join(parts, " ")
	}
	i := 0
	skip := func() {
		for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
			i++
		}
	}
	for {
		skip()
		if i >= len(text) || text[i] != '-' {
			break
		}
		if i+1 >= len(text) || text[i+1] == ' ' || text[i+1] == '\t' {
			// a lone "-" ends the options and is consumed
			i++
			skip()
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
				return nil, "", fmt.Sprintf("invalid option '%c'", c), usageLine()
			}
			i++
			if k == 0 {
				opts[c] = Int(1)
				continue
			}
			val, j := parseOptArg(text, i)
			i = j
			switch k {
			case '#':
				opts[c] = Int(Str(val).Int())
			case '@':
				opts[c] = Str(val).Num()
			default:
				opts[c] = Str(val)
			}
			break
		}
	}
	return opts, text[i:], "", usageLine()
}

// getopts(options [, init]) parses leading -x options of the current macro's
// arguments into local variables opt_x, then shifts them off.
func fnGetopts(in *Interp, a []Value) (Value, error) {
	spec := str(a, 0)
	f := in.fr
	if len(a) > 1 {
		for i := 0; i < len(spec); i++ {
			c := spec[i]
			if c == ':' || c == '#' || c == '@' {
				continue
			}
			f.vars["opt_"+string(c)] = a[1]
		}
	}
	opts, rest, errMsg, usage := parseOptions(spec, f.text)
	if errMsg != "" {
		in.warn("%s: %s", f.name, errMsg)
		in.warn("%s: options: %s", f.name, usage)
		return Int(0), nil
	}
	for c, v := range opts {
		f.vars["opt_"+string(c)] = v
	}
	f.setArgs(rest)
	return Int(1), nil
}
