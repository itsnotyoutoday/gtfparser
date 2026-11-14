// Package pcrerun evaluates a (pattern, subject) corpus with the pcre
// engine and renders results in the line format that scripts/pcre_diff.py
// extracts from real TinyFugue, so the two can be compared.
package pcrerun

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/itsnotyoutoday/gtfparser/pcre"
)

// Pair is one corpus entry.
type Pair struct{ Pattern, Subject string }

// MaxGroups is how many groups (including group 0) a record reports.
const MaxGroups = 10

// Parse reads a corpus: one "pattern<TAB>subject" per line, '#' comments and
// blank lines skipped.  In the subject, \n \t \r \f \v \e \\ \xHH (a byte) and \uHHHH are escapes.
func Parse(r io.Reader) ([]Pair, error) {
	var out []Pair
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		pat, subj, ok := strings.Cut(line, "\t")
		if !ok {
			return nil, fmt.Errorf("corpus line %q has no tab", line)
		}
		s, err := Unescape(subj)
		if err != nil {
			return nil, err
		}
		out = append(out, Pair{pat, s})
	}
	return out, sc.Err()
}

// Unescape decodes the subject escapes used in corpus files.
func Unescape(s string) (string, error) {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' {
			b.WriteByte(s[i])
			continue
		}
		i++
		if i >= len(s) {
			return "", fmt.Errorf("trailing backslash in %q", s)
		}
		switch s[i] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case 'f':
			b.WriteByte('\f')
		case 'v':
			b.WriteByte('\v')
		case 'e':
			b.WriteByte(0x1b)
		case '\\':
			b.WriteByte('\\')
		case 'x':
			if i+2 >= len(s) {
				return "", fmt.Errorf("bad \\x in %q", s)
			}
			v, err := strconv.ParseUint(s[i+1:i+3], 16, 8)
			if err != nil {
				return "", fmt.Errorf("bad \\x in %q", s)
			}
			b.WriteByte(byte(v))
			i += 2
		case 'u':
			if i+4 >= len(s) {
				return "", fmt.Errorf("bad \\u in %q", s)
			}
			v, err := strconv.ParseUint(s[i+1:i+5], 16, 32)
			if err != nil {
				return "", fmt.Errorf("bad \\u in %q", s)
			}
			b.WriteRune(rune(v))
			i += 4
		default:
			return "", fmt.Errorf("unknown escape \\%c in %q", s[i], s)
		}
	}
	return b.String(), nil
}

// esc renders a captured string the way real tf's tfwrite shows it: tf turns
// a newline into a space; backslash, carriage return and tab are escaped.
func esc(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.NewReplacer("\\", "\\\\", "\r", "\\r", "\t", "\\t").Replace(s)
}

// Record renders the result of matching p with CompileTF as one line:
// "ERR offset message" (tab separated) for a compile error, "0" for no match, otherwise
// n TAB PL TAB PR TAB P0 TAB P1 ... (n is highest matched group + 1, as
// returned by pcre2_match; groups past MaxGroups are not reported).
func Record(p Pair) string {
	re, err := pcre.CompileTF(p.Pattern)
	if err != nil {
		var pe *pcre.Error
		if errors.As(err, &pe) {
			return fmt.Sprintf("ERR\t%d\t%s", pe.Offset, pe.Msg)
		}
		return "ERR"
	}
	ov := re.Match(p.Subject, 0, false)
	if ov == nil {
		return "0"
	}
	n := 1
	for g := re.NumCaptures(); g >= 1; g-- {
		if ov[2*g] >= 0 {
			n = g + 1
			break
		}
	}
	f := []string{strconv.Itoa(n), esc(p.Subject[:ov[0]]), esc(p.Subject[ov[1]:])}
	for g := 0; g < n && g < MaxGroups; g++ {
		if ov[2*g] < 0 {
			f = append(f, "")
		} else {
			f = append(f, esc(p.Subject[ov[2*g]:ov[2*g+1]]))
		}
	}
	return strings.Join(f, "\t")
}

// RefLine evaluates one ref_cases-format line (pattern, subject, start,
// notbol, caseless; tab separated, subject escaped as in a corpus) and
// renders the result like scripts/pcre_ref.c: "ERR", "0", or the ovector.
func RefLine(line string) (string, error) {
	f := strings.Split(line, "\t")
	if len(f) != 5 {
		return "", fmt.Errorf("want 5 tab-separated fields, got %d", len(f))
	}
	subj, err := Unescape(f[1])
	if err != nil {
		return "", err
	}
	start, _ := strconv.Atoi(f[2])
	re, err := pcre.Compile(f[0], pcre.Options{Caseless: f[4] == "1", DotAll: true, DollarEndOnly: true, UTF: true, UCP: true})
	if err != nil {
		var pe *pcre.Error
		if errors.As(err, &pe) {
			return fmt.Sprintf("ERR %d %s", pe.Offset, pe.Msg), nil
		}
		return "ERR", nil
	}
	ov := re.Match(subj, start, f[3] == "1")
	if ov == nil {
		return "0", nil
	}
	parts := make([]string, len(ov))
	for i, v := range ov {
		parts[i] = strconv.Itoa(v)
	}
	return strings.Join(parts, " "), nil
}
