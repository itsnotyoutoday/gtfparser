package gtfparser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestExitEndsLoad(t *testing.T) {
	out, _ := compatRun(t, "/def ex = /echo before%; /exit%; /echo after\n/ex\n/echo still\n")
	if got := strings.Join(out, "|"); got != "before" {
		t.Fatalf("got %q", got)
	}
}

func TestEvaluationOrder(t *testing.T) {
	// identifiers are read when the operator reduces, as in tf's stack machine
	out, _ := compatRun(t, "/def t = /echo $[y:=5, y + ++y] $[y := 5, y - --y] $[++f]\n/set f=1.5\n/t\n")
	if got := strings.Join(out, "|"); got != "12 0 2" {
		t.Fatalf("got %q", got)
	}
}

func TestStringFunctionsLikeTf(t *testing.T) {
	out, _ := compatRun(t, strings.Join([]string{
		`/def t = /echo [$[strcmp("A","a")]] [$[strncmp("abc","abd",-1)]] [$[substr("hello",-2)]] [$[substr("hello",2,-1)]] [$[strstr("hello","l",3)]] [$[strrchr("hello","lh",2)]]`,
		`/def u = /echo [$[strlen("日本語")]] [$[substr("日本語",1,1)]] [$[pad("日本",5,"x",-3)]] [$[replace("","b","banana")]] [$[toupper("hello",2)]]`,
		`/def v = /echo [$[mod(7,3)]] [$[abs(-4)]] [$[strcat("a","b","c")]] [$[strrep("ab",3)]] [$[regmatch("(a)(b)?","a")]]`,
		"/t", "/u", "/v",
	}, "\n")+"\n")
	want := []string{
		"[-32] [-1] [lo] [ll] [3] [2]",
		"[3] [本] [   日本x  ] [banana] [HEllo]",
		"[1] [4] [abc] [ababab] [2]",
	}
	if strings.Join(out, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got %q", out)
	}
}

type ttyHost struct {
	tHost
	tty bool
}

func (h *ttyHost) IsTTY() bool { return h.tty }

func TestHostDependentFunctions(t *testing.T) {
	h := &ttyHost{tty: true}
	in := New(h)
	if v := in.ExecFull(`/test isatty()`); v.Int() != 1 {
		t.Fatalf("isatty with a tty host: %v", v)
	}
	h.tty = false
	if v := in.ExecFull(`/test isatty()`); v.Int() != 0 {
		t.Fatalf("isatty without: %v", v)
	}
	if v := in.ExecFull(`/test nread()`); v.Int() != 0 {
		t.Fatalf("nread: %v", v)
	}
	in.SetReadDepth(2)
	if v := in.ExecFull(`/test nread()`); v.Int() != 2 {
		t.Fatalf("nread depth: %v", v)
	}
	// nmail(): a non-empty file whose mtime is newer than its atime is unread mail
	dir := t.TempDir()
	mf := filepath.Join(dir, "mbox")
	in.SetVar("MAIL", mf)
	if v := in.ExecFull(`/test nmail()`); v.Int() != 0 {
		t.Fatalf("nmail without a file: %v", v)
	}
	os.WriteFile(mf, []byte("From x\n"), 0o644)
	now := time.Now()
	os.Chtimes(mf, now.Add(-time.Hour), now) // atime older than mtime
	in.ExecFull(`/test nmail()`)             // first look
	os.Chtimes(mf, now.Add(-time.Hour), now.Add(time.Minute))
	if v := in.ExecFull(`/test nmail()`); v.Int() != 1 {
		t.Fatalf("nmail with unread mail: %v", v)
	}
}

func TestBuiltinCommandsAsFunctions(t *testing.T) {
	out, _ := compatRun(t, strings.Join([]string{
		`/def t = /echo [$[def("zz = /echo zap") > 0]] [$[set("qq=1")]] [$[qq]] [$[shift()]] [$[limit()]]`,
		"/t a b",
		"/def s = /test shift()%; /echo %1 %{#}",
		"/s a b c",
		"/shift -1",
		"/echo ok",
	}, "\n")+"\n")
	want := "[1] [1] [1] [1] [0]|b 2|ok"
	if strings.Join(out, "|") != want {
		t.Fatalf("got %q", out)
	}
}

func TestStreams(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.txt")
	out, warns := compatRun(t, strings.Join([]string{
		`/def t = /test q := tfopen()%; /test tfwrite(q, "alpha")%; /test tfread(q, a)%; ` +
			`/echo [%a] [$[tfclose(q)]] [$[tfclose(12345)]] [$[tfflush(12345)]] [$[tfreadable("o")]] [$[tfopen("x")]]`,
		`/def u = /test w := tfopen("` + f + `", "w")%; /echo [$[tfwrite(w, "x")]] [$[tfwrite(w, "y")]] [$[tfreadable(w)]] [$[tfclose(w)]]`,
		`/def p = /test p := tfopen("echo hi; echo there", "p")%; /test tfread(p, v)%; /echo [%v] [$[tfclose(p)]]`,
		"/t", "/u", "/p",
	}, "\n")+"\n")
	want := []string{"[alpha] [0] [-1] [-1] [1] [-1]", "[1] [1] [1] [0]", "[hi] [0]"}
	if strings.Join(out, "\n") != strings.Join(want, "\n") {
		t.Fatalf("out %q warns %q", out, warns)
	}
	wantW := []string{
		"CASE, line 4: tfclose: 12345: bad handle",
		"CASE, line 4: tfflush: 12345: bad handle",
		"CASE, line 4: tfreadable: stream o is not readable",
		"CASE, line 4: tfopen: invalid mode 'x'",
		"CASE, line 5: tfreadable: stream 2 is not readable",
	}
	if strings.Join(warns, "\n") != strings.Join(wantW, "\n") {
		t.Fatalf("warnings %q", warns)
	}
	data, _ := os.ReadFile(f)
	if string(data) != "x\ny\n" {
		t.Fatalf("file: %q", data)
	}
}

func TestLoadLineRules(t *testing.T) {
	// ';' and '#' in the first column are comments, also inside a continued line;
	// a trailing backslash continues a line unless a '%' precedes it
	script := "# a comment\n" +
		"; another\n" +
		"/echo one \\\n" +
		"# comment in the middle\n" +
		"   two\n" +
		"/echo three %\\\n" +
		"/echo four\\\\\n" +
		"/echo five\n"
	out, _ := compatRun(t, script)
	want := []string{"one two", "three %\\", "four\\/echo five"}
	if strings.Join(out, "|") != strings.Join(want, "|") {
		t.Fatalf("got %q", out)
	}
}
