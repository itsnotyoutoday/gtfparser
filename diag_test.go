package gtfparser

import (
	"strings"
	"testing"
)

// noConn is a host with no connection: Send fails, as tf's does when not
// connected.
type noConn struct{ tHost }

func (h *noConn) Send(w, s string) bool { return false }

// TestDiagnosticsMatchTf pins the wording and the context prefix of the most
// common diagnostics (taken from real tf's output; see testdata/warnings and
// docs/diagnostics.md). Each script runs as a loaded file, so every message
// carries "CASE, line N: ".
func TestDiagnosticsMatchTf(t *testing.T) {
	cases := []struct{ name, script, want string }{
		{"load missing", "/load /nonexistent/zz.tf", "CASE, line 1: LOAD: /nonexistent/zz.tf: No such file or directory"},
		{"load -q missing still fails", "/load -q /nonexistent/zz.tf", "CASE, line 1: LOAD: /nonexistent/zz.tf: No such file or directory"},
		{"loadfail hook gags", "/def -hloadfail -ag gl\n/load /nonexistent/zz.tf", ""},
		{"load missing filename", "/load", "CASE, line 1: LOAD: missing filename"},
		{"nofunc in macro", "/def f = /test nofunc(1)\n/f", "CASE, line 2: f: nofunc: no such function"},
		{"arity", "/test strlen()", "CASE, line 1: strlen: found 0 arguments, expected 1"},
		{"expr syntax", "/test 1 +", "CASE, line 1: expression syntax error: expected operand, found end of body."},
		{"if body done", "/def f = /echo before%; /if (1) /done%; /endif\n/f", "CASE, line 2: f: unexpected /DONE in IF body"},
		{"while body endif", "/def f = /while (1) /endif%; /done\n/f", "CASE, line 2: f: unexpected /ENDIF in WHILE body"},
		{"endif at top", "/def f = /endif\n/f", "CASE, line 2: f: unexpected /ENDIF in outer block"},
		{"unterminated if", "/def f = /if (1) /echo x\n/f", "CASE, line 2: f: macro syntax error: expected /endif, found end of body."},
		{"unterminated while", "/def f = /while (1) /echo x\n/f", "CASE, line 2: f: macro syntax error: expected /done, found end of body."},
		{"condition junk", "/def f = /if (1 2) /echo x%; /endif\n/f", "CASE, line 2: f: condition syntax error: expected operator or ')', found '2'."},
		{"break arg", "/def f = /while (1) /break x%; /done\n/f", "CASE, line 2: f: /BREAK syntax error: expected positive integer, found 'x'."},
		{"missing separator after endif", "/def f = /if (1) /echo x%; /endif /echo y\n/f", "CASE, line 2: f: macro syntax error: expected end of statement after /endif, found '/echo'.  (Possibly missing '%;')"},
		{"pipe into text", "/def f = /echo x %| hello\n/f", "CASE, line 2: f: Piping input to a server command is not allowed."},
		{"pipe at end", "/def f = /echo hi %|\n/f", "CASE, line 2: f: '%|' must be followed by another command."},
		{"unmatched $[", "/eval /echo $[1+1", "CASE, line 1: EVAL: unmatched $["},
		{"unmatched (", "/eval /echo $(/echo hi", "CASE, line 1: EVAL: unmatched ("},
		{"@ not builtin", "/@nosuch", "CASE, line 1: nosuch: not a builtin command"},
		{"recursion", "/set max_recur=10\n/def r = /r\n/r", "CASE, line 3: r: recursion count exceeded %max_recur (10)"},
		{"repeat count", "/repeat -1 0 /echo x", "CASE, line 1: REPEAT: invalid repeat count (0)"},
		{"repeat garbage", "/repeat -1 3x /echo x", "CASE, line 1: REPEAT: repeat count followed by garbage (x /ec)"},
		{"repeat missing", "/repeat -1 3", "CASE, line 1: REPEAT: missing command"},
		{"repeat -S i", "/repeat -S i /echo x", "CASE, line 1: REPEAT: -S i would cause an infinite busy loop."},
		{"kill", "/kill 999999", "CASE, line 1: KILL: no process 999999"},
		{"quote", "/quote -1 x", "CASE, line 1: QUOTE: missing command character"},
		{"set illegal", "/set a-b=1", "CASE, line 1: SET: illegal variable name: a-b"},
		{"set space equals", "/set foo =bar", "CASE, line 1: Warning: '=' following space is part of value."},
		{"export", "/export nosuchvar", "CASE, line 1: EXPORT: nosuchvar not defined."},
		{"unset pos", "/unset histsize", "CASE, line 1: UNSET: histsize must be a positive integer, so can not be unset."},
		{"local shadows", "/set gv=1\n/def f = /let gv=2\n/f", "CASE, line 3: f: Warning:  Local variable \"gv\" overshadows global variable of same name."},
		{"glob unmatched", `/test "a" =/ "[a"`, "CASE, line 1: glob error: unmatched '['"},
		{"regexp error", `/test regmatch("(", "a")`, "CASE, line 1: regmatch: regexp error: character 1: missing closing parenthesis"},
		{"fwrite OS text", `/test fwrite("/nonexistent/dir/x", "a")`, "CASE, line 1: fwrite: /nonexistent/dir/x: No such file or directory"},
		{"save OS text", "/save /nonexistent/dir/x.tf", "CASE, line 1: SAVE: /nonexistent/dir/x.tf: No such file or directory"},
		{"mul overflow", "/test 1e308 * 10", "CASE, line 1: * operator: arithmetic overflow"},
		{"mula overflow", "/test z := 1e308\n/test z *= 10", "CASE, line 2: MULA operator: arithmetic overflow"},
		{"undef number", "/undef #9999", "CASE, line 1: UNDEF: no macro with number 9999\nCASE, line 1: UNDEF: Macro \"#9999\" was not defined."},
		{"def bad option", "/def -Z foo = bar", "CASE, line 1: DEF -Z: invalid option\nCASE, line 1: DEF: options: -p<integer> -c<integer> -b<string> -B<string> -E<string> -t<string> -w<string> -h<string> -A<string> -a<string> -f<string> -P<string> -T<string> -FiI -n<integer> -1 -m<string> -q"},
		{"getopts invalid option", "/def f = /if (!getopts(\"a:bcd#e\", \"\")) /return 0%; /endif\n/f -b -Z", "CASE, line 2: f -Z: invalid option\nCASE, line 2: f: options: -a<string> -bc -d<integer> -e"},
		{"non-command line aborts load", "/echo a\nhello\n/echo b", "CASE, line 2: Invalid command. Aborting.\nCASE: line 2: last command is incomplete because of trailing \\"},
		{"indented line", "/echo a\n  /echo b", "CASE: line 1: Warning: possibly missing trailing \\"},
		{"final backslash blanks", "/echo a\\ \n/echo b", "CASE, line 1: Warning: whitespace following final '\\'"},
	}
	for _, c := range cases {
		out, warns := compatRun(t, c.script+"\n")
		got := strings.Join(warns, "\n")
		if got != c.want {
			t.Errorf("%s:\n got  %q\n want %q (out %q)", c.name, got, c.want, out)
		}
	}
}

// TestDiagnosticsNotConnected: sending text with nowhere to send it is an
// error, with the context of what ran.
func TestDiagnosticsNotConnected(t *testing.T) {
	h := &noConn{}
	in := New(h)
	var warns []string
	in.Warn = func(s string) { warns = append(warns, s) }
	in.Exec("/def f = hello")
	in.ExecFull("/f")
	in.ExecFull("hi there")
	in.ExecFull(`/test send("x", "", "Z")`)
	want := "f: Not connected.|Not connected.|send: invalid flag Z"
	if got := strings.Join(warns, "|"); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

// TestBrokenBodyRunsNothing: a body that does not compile is dropped whole,
// including statements before the error, and a repeated error does not spin.
func TestBrokenBodyRunsNothing(t *testing.T) {
	h := &tHost{}
	in := New(h)
	n := 0
	in.Warn = func(s string) { n++ }
	in.Exec("/def f = /echo ran%; /while (1) /endif%; /done")
	in.ExecFull("/f")
	if len(h.out) != 0 || n != 1 {
		t.Errorf("out %q, %d warnings; want none and 1", h.out, n)
	}
}

// TestBlankStatementWarning: blank "%;" statements after /if (cond) and
// /else are sent as blank lines, with tf's warning.
func TestBlankStatementWarning(t *testing.T) {
	h := &noConn{}
	in := New(h)
	var warns []string
	in.Warn = func(s string) { warns = append(warns, s) }
	in.ExecFull("/if (1)%; /echo hi%; /endif")
	want := `Warning: "%;" following "/IF (...)" sends blank line to server, which is probably not what was intended.|Not connected.`
	if got := strings.Join(warns, "|"); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}
