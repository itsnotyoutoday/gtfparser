package gtfparser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// compatRun loads a script as a file would (so diagnostics carry file and
// line) and returns the lines echoed and the warnings.
func compatRun(t *testing.T, script string) (out, warns []string) {
	t.Helper()
	h := &tHost{}
	in := New(h)
	in.Warn = func(s string) { warns = append(warns, s) }
	dir := t.TempDir()
	p := filepath.Join(dir, "case.tf")
	if err := os.WriteFile(p, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	in.LoadFile(p)
	for i := range warns {
		warns[i] = strings.ReplaceAll(warns[i], p, "CASE")
	}
	return h.out, warns
}

// TestDeliberateDeviations pins every script registered in
// testdata/deviations (see docs/deliberate-differences.md): gtfparser must
// keep printing the documented output.
func TestDeliberateDeviations(t *testing.T) {
	files, _ := filepath.Glob("testdata/deviations/*.tf")
	if len(files) == 0 {
		t.Fatal("no deviation scripts")
	}
	for _, f := range files {
		src, _ := os.ReadFile(f)
		want, err := os.ReadFile(strings.TrimSuffix(f, ".tf") + ".out")
		if err != nil {
			t.Fatal(err)
		}
		out, _ := compatRun(t, "/def -i E = /echo %{1}\n"+string(src))
		if got := strings.Join(out, "\n") + "\n"; got != string(want) {
			t.Errorf("%s:\n got %q\nwant %q", f, got, want)
		}
	}
}

func TestDeviationWarnings(t *testing.T) {
	_, w := compatRun(t, "/test strrep(\"x\", 99999999999999999999)\n/test pad(\"a\", 99999999999)\n/set x=9223372036854775807\n/test ++x\n")
	want := []string{
		"CASE, line 1: result would exceed the string size limit (16777216 bytes)",
		"CASE, line 2: result would exceed the string size limit (16777216 bytes)",
		"CASE, line 4: integer overflow in ++x",
	}
	// the function name leads the message exactly as for tf's own function errors
	want[0] = "CASE, line 1: strrep: " + strings.SplitN(want[0], ": ", 2)[1]
	want[1] = "CASE, line 2: pad: " + strings.SplitN(want[1], ": ", 2)[1]
	if strings.Join(w, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got %q", w)
	}
}

// TestDiagnosticFormat pins the wording and context tf prints: "file, line N:"
// while loading, the running macro or function name, tf's own messages.
func TestDiagnosticFormat(t *testing.T) {
	_, w := compatRun(t, strings.Join([]string{
		"/test 1/0",
		"/test 1.5/0",
		"/test 1/0.0",
		"/test strlen(\"a\",\"b\")",
		"/def f3 = /test mod(5,0)",
		"/f3",
		"/test 1 +",
		"/test (1",
		"/test 1 2",
		"/test \"abc",
		"/nosuchcmd",
		"/def f5 = /nosuchcmd2",
		"/f5",
		"/test 1 ? 2",
		"/def f6 = /test strcat()",
		"/f6",
		"/undef nosuchmacro",
	}, "\n")+"\n")
	want := []string{
		"CASE, line 1: division by zero",
		"CASE, line 2: / operator: arithmetic overflow",
		"CASE, line 3: / operator: arithmetic overflow",
		"CASE, line 4: strlen: found 2 arguments, expected 1",
		"CASE, line 6: mod: division by zero",
		"CASE, line 7: expression syntax error: expected operand, found end of body.",
		"CASE, line 8: expression syntax error: expected ')' after '(...', found end of body.",
		"CASE, line 9: expression syntax error: expected end of expression, found '2'.",
		"CASE, line 10: unmatched \" in string literal",
		"CASE, line 11: nosuchcmd: no such command or macro",
		"CASE, line 13: f5: nosuchcmd2: no such command or macro",
		"CASE, line 14: expression syntax error: expected ':' after '?...', found end of body.",
		"CASE, line 16: f6: strcat: found 0 arguments, expected at least 1",
		"CASE, line 17: UNDEF: Macro \"nosuchmacro\" was not defined.",
	}
	if len(w) != len(want) {
		t.Fatalf("got %d warnings %q", len(w), w)
	}
	for i := range want {
		if w[i] != want[i] {
			t.Errorf("warning %d:\n got %q\nwant %q", i, w[i], want[i])
		}
	}
}

func TestNanAndInfPrint(t *testing.T) {
	out, _ := compatRun(t, "/def t = /echo [$[0/0.0]] [$[sqrt(-1)]] [$[ln(0)]] [$[exp(1000)]] [$[-1.0*0]] [$[-2.0*1]] [$[2.0*1]]\n/t\n")
	if got := strings.Join(out, "|"); got != "[nan] [nan] [-inf] [inf] [-0] [-2] [2.]" {
		t.Fatalf("got %q", got)
	}
}
