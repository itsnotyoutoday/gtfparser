package pcre_test

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/itsnotyoutoday/gtfparser/pcre"
	"github.com/itsnotyoutoday/gtfparser/pcre/internal/pcrerun"
)

func readLines(t *testing.T, name string) []string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
}

// TestCorpus checks the engine against answers recorded from real
// TinyFugue/PCRE2 10.47 (scripts/pcre_diff.py --update).
func TestCorpus(t *testing.T) {
	f, err := os.Open("testdata/corpus.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	pairs, err := pcrerun.Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	want := readLines(t, "expected.txt")
	if len(want) != len(pairs) {
		t.Fatalf("expected.txt has %d records for %d corpus pairs; rerun scripts/pcre_diff.py --update", len(want), len(pairs))
	}
	bad := 0
	for i, p := range pairs {
		if got := pcrerun.Record(p); got != want[i] {
			bad++
			t.Errorf("#%d pattern %q subject %q\n  want %q\n  got  %q", i, p.Pattern, p.Subject, want[i], got)
		}
	}
	t.Logf("%d corpus pairs, %d mismatches", len(pairs), bad)
	if len(pairs) < 400 {
		t.Errorf("corpus has only %d pairs", len(pairs))
	}
}

// TestReference checks start offsets, NOTBOL and invalid UTF-8 subjects
// against PCRE2 run directly (scripts/pcre_ref.py).
func TestReference(t *testing.T) {
	var cases []string
	for _, l := range readLines(t, "ref_cases.txt") {
		if l != "" && !strings.HasPrefix(l, "#") {
			cases = append(cases, l)
		}
	}
	want := readLines(t, "ref_expected.txt")
	if len(cases) != len(want) {
		t.Fatalf("%d cases but %d expectations", len(cases), len(want))
	}
	for i, l := range cases {
		f := strings.Split(l, "\t")
		subj, err := pcrerun.Unescape(f[1])
		if err != nil {
			t.Fatal(err)
		}
		start, _ := strconv.Atoi(f[2])
		o := pcre.Options{Caseless: f[4] == "1", DotAll: true, DollarEndOnly: true, UTF: true, UCP: true}
		var got string
		re, err := pcre.Compile(f[0], o)
		if err != nil {
			got = "ERR"
		} else if ov := re.Match(subj, start, f[3] == "1"); ov == nil {
			got = "0"
		} else {
			parts := make([]string, len(ov))
			for j, v := range ov {
				parts[j] = strconv.Itoa(v)
			}
			got = strings.Join(parts, " ")
		}
		w := want[i]
		if strings.HasPrefix(w, "ERROR") { // PCRE2 match error such as a bad start offset
			w = "0"
		} else if strings.HasPrefix(w, "ERR") { // compile error
			w = "ERR"
		}
		if got != w {
			t.Errorf("case %d %q subject %q start %s notbol %s caseless %s:\n  want %s\n  got  %s", i, f[0], f[1], f[2], f[3], f[4], want[i], got)
		}
	}
}

func mustTF(t *testing.T, pat string) *pcre.Regexp {
	t.Helper()
	re, err := pcre.CompileTF(pat)
	if err != nil {
		t.Fatalf("CompileTF(%q): %v", pat, err)
	}
	return re
}

func TestAPI(t *testing.T) {
	re := mustTF(t, `(?<year>\d{4})-(\d\d)-(?P<day>\d\d)`)
	if re.NumCaptures() != 3 {
		t.Errorf("NumCaptures = %d", re.NumCaptures())
	}
	if got := fmt.Sprint(re.CaptureNames()); got != "[ year  day]" {
		t.Errorf("CaptureNames = %q", got)
	}
	ov := re.Match("on 2024-05-17.", 0, false)
	if fmt.Sprint(ov) != "[3 13 3 7 8 10 11 13]" {
		t.Errorf("ovector = %v", ov)
	}
	if re.Match("nothing", 0, false) != nil {
		t.Error("unexpected match")
	}
	if re.Match("x", 5, false) != nil || re.Match("x", -1, false) != nil {
		t.Error("out of range start should not match")
	}
	// unset groups are -1
	ov = mustTF(t, `(a)|(b)`).Match("b", 0, false)
	if fmt.Sprint(ov) != "[0 1 -1 -1 0 1]" {
		t.Errorf("ovector = %v", ov)
	}
}

func TestSmartCase(t *testing.T) {
	// Smart case: uppercase letters outside escapes disable CASELESS.
	for _, c := range []struct {
		pat, subj string
		want      bool
	}{
		{"abc", "ABC", true},
		{"Abc", "abc", false},
		{"Abc", "Abc", true},
		{`\Dbc`, "xBC", true}, // the D after the backslash does not count
		{"é", "É", true},
		{"É", "é", false},
		{`a\.B`, "a.b", false},
		{`(?i)Abc`, "aBC", true}, // inline flag turns it back on
	} {
		if got := mustTF(t, c.pat).Match(c.subj, 0, false) != nil; got != c.want {
			t.Errorf("CompileTF(%q) on %q: match=%v want %v", c.pat, c.subj, got, c.want)
		}
	}
	// A pattern that is not valid UTF-8 is compiled in byte mode.
	re := mustTF(t, "a\xe9b")
	if re.Match("xa\xe9b", 0, false) == nil {
		t.Error("byte-mode pattern did not match")
	}
}

func TestOptions(t *testing.T) {
	m := func(pat string, o pcre.Options, subj string) []int {
		re, err := pcre.Compile(pat, o)
		if err != nil {
			t.Fatalf("Compile(%q): %v", pat, err)
		}
		return re.Match(subj, 0, false)
	}
	eq := func(got []int, want string) {
		t.Helper()
		if fmt.Sprint(got) != want {
			t.Errorf("got %v want %s", got, want)
		}
	}
	eq(m("a.b", pcre.Options{}, "a\nb"), "[]")
	eq(m("a.b", pcre.Options{DotAll: true}, "a\nb"), "[0 3]")
	eq(m("a$", pcre.Options{}, "a\n"), "[0 1]")
	eq(m("a$", pcre.Options{DollarEndOnly: true}, "a\n"), "[]")
	eq(m("^b", pcre.Options{Multiline: true}, "a\nb"), "[2 3]")
	eq(m("^b", pcre.Options{}, "a\nb"), "[]")
	eq(m("a b # x\n c", pcre.Options{Extended: true}, "abc"), "[0 3]")
	eq(m("ABC", pcre.Options{Caseless: true}, "xabc"), "[1 4]")
	// without UTF a character is a byte; without UCP \w is ASCII
	eq(m("^.$", pcre.Options{}, "é"), "[]")
	eq(m("^.$", pcre.Options{UTF: true}, "é"), "[0 2]")
	eq(m(`\w`, pcre.Options{UTF: true}, "é"), "[]")
	eq(m(`\w`, pcre.Options{UTF: true, UCP: true}, "é"), "[0 2]")
	eq(m(`(?i)É`, pcre.Options{UTF: true}, "é"), "[0 2]")
	eq(m(`[[:alpha:]]`, pcre.Options{UTF: true}, "é"), "[]")
	eq(m(`\xe9`, pcre.Options{}, "caf\xe9"), "[3 4]")
	if _, err := pcre.Compile("\xff", pcre.Options{UTF: true}); err == nil {
		t.Error("invalid UTF-8 pattern accepted in UTF mode")
	}
}

func TestMatchStart(t *testing.T) {
	re := mustTF(t, `\bfoo\b`)
	s := "foo foo foo"
	var got []int
	for pos := 0; ; {
		ov := re.Match(s, pos, pos > 0)
		if ov == nil {
			break
		}
		got = append(got, ov[0])
		pos = ov[1]
	}
	if fmt.Sprint(got) != "[0 4 8]" {
		t.Errorf("successive matches at %v", got)
	}
	// ^ with notBOL, and lookbehind seeing before start
	if mustTF(t, `^a`).Match("a", 0, true) != nil {
		t.Error("^ matched with notBOL")
	}
	if mustTF(t, `(?<=x)a`).Match("xa", 1, false) == nil {
		t.Error("lookbehind could not see before start")
	}
	if ov := mustTF(t, `\Ga`).Match("aab", 1, false); fmt.Sprint(ov) != "[1 2]" {
		t.Errorf(`\G: %v`, ov)
	}
}

func TestErrors(t *testing.T) {
	for _, c := range []struct{ pat, msg string }{
		{"(", "missing closing parenthesis"},
		{"a)", "unmatched closing parenthesis"},
		{"[a", "missing terminating ] for character class"},
		{"a**", "quantifier does not follow a repeatable item"},
		{`\1`, "reference to non-existent subpattern"},
		{`(?<=a+)b`, "length of lookbehind assertion is not limited"},
		{`\p{Nope}`, "unknown property after \\P or \\p"},
		{`[z-a]`, "range out of order in character class"},
		{`(?<n>a)(?<n>b)`, "two named subpatterns have the same name (PCRE2_DUPNAMES not set)"},
		{`(?1)`, "recursion and subroutine calls are not supported"},
	} {
		_, err := pcre.CompileTF(c.pat)
		pe, ok := err.(*pcre.Error)
		if !ok || pe.Msg != c.msg {
			t.Errorf("CompileTF(%q) error = %v, want %q", c.pat, err, c.msg)
		}
	}
	_, err := pcre.CompileTF("ab(c")
	if err == nil || err.Error() != "missing closing parenthesis at offset 4" {
		t.Errorf("error text = %v", err)
	}
}

func TestStepLimit(t *testing.T) {
	// classic catastrophic patterns; each must stop, not hang
	for _, c := range []struct{ pat, subj string }{
		{`(a+)+$`, strings.Repeat("a", 40) + "b"},
		{`(a|aa)+$`, strings.Repeat("a", 60) + "b"},
		{`(?:a*)*b`, strings.Repeat("a", 50)},
		{`(x+x+)+y`, strings.Repeat("x", 40)},
		{`^(\w+\s?)*$`, strings.Repeat("word ", 20) + "!"},
		{`(.*)*x`, strings.Repeat("a", 40)},
		{`(a?){40}a{40}`, strings.Repeat("a", 40)},
	} {
		re := mustTF(t, c.pat)
		re.SetStepLimit(200_000)
		start := time.Now()
		ov, err := re.MatchErr(c.subj, 0, false)
		if err != pcre.ErrLimit || ov != nil {
			// a pattern may legitimately finish quickly; it must at least not hang
			t.Logf("%q: ov=%v err=%v", c.pat, ov, err)
		}
		if time.Since(start) > 5*time.Second {
			t.Errorf("%q took %v", c.pat, time.Since(start))
		}
	}
	re := mustTF(t, `(a+)+$`)
	re.SetStepLimit(100_000)
	if _, err := re.MatchErr(strings.Repeat("a", 40)+"b", 0, false); err != pcre.ErrLimit {
		t.Errorf("err = %v, want ErrLimit", err)
	}
	if re.Match(strings.Repeat("a", 40)+"b", 0, false) != nil {
		t.Error("Match returned a result after the step limit")
	}
	// the same regexp still works on easy input afterwards, and possessive/atomic forms are immune
	if re.Match("aaa", 0, false) == nil {
		t.Error("no match on easy input")
	}
	if mustTF(t, `(?>a+)+$`).Match(strings.Repeat("a", 5000)+"b", 0, false) != nil {
		t.Error("atomic pattern matched")
	}
	// deep recursion on a long subject is bounded, not a stack overflow
	long := strings.Repeat("ab", 500_000)
	if _, err := mustTF(t, `(?:a|b)*c`).MatchErr(long, 0, false); err != pcre.ErrLimit {
		t.Errorf("deep loop: err = %v", err)
	}
	// plain single-character repeats are iterative and fine on long input
	if ov := mustTF(t, `a*b`).Match(strings.Repeat("a", 3_000_000)+"b", 0, false); ov == nil {
		t.Error("a*b failed on long input")
	}
}

func TestConcurrent(t *testing.T) {
	re := mustTF(t, `(\w+)@(\w+)\.com`)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				if ov := re.Match("mail bob@example.com now", 0, false); fmt.Sprint(ov) != "[5 20 5 8 9 16]" {
					t.Errorf("ov = %v", ov)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestMisc(t *testing.T) {
	// \K and \G, captures reset on backtracking, and Cn / script extensions
	check := func(pat, subj, want string) {
		t.Helper()
		got := fmt.Sprint(mustTF(t, pat).Match(subj, 0, false))
		if got != want {
			t.Errorf("%q on %q = %s, want %s", pat, subj, got, want)
		}
	}
	check(`foo\Kbar`, "foobar", "[3 6]")
	check(`(a)|b`, "b", "[0 1 -1 -1]")
	check(`(?:(a)|b)*`, "ab", "[0 2 0 1]")
	check(`\p{Han}`, "、", "[0 3]") // Script_Extensions
	check(`\p{sc:Han}`, "、", "[]")
	check(`(?<=a|bc)d`, "bcd", "[2 3]")
	check(`(a)?(?(1)b|c)`, "c", "[0 1 -1 -1]")
	check(`(*FAIL)|x`, "x", "[0 1]")
}
