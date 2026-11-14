package gtfparser

import (
	"strings"
	"testing"
)

type tHost struct{ out, sent []string }

func (h *tHost) Echo(s string)         { h.out = append(h.out, s) }
func (h *tHost) Send(w, s string) bool { h.sent = append(h.sent, s); return true }

func run(t *testing.T, script string) (*tHost, *Interp) {
	t.Helper()
	h := &tHost{}
	in := New(h)
	for _, l := range strings.Split(strings.TrimSpace(script), "\n") {
		if strings.HasPrefix(l, "/def") {
			in.Exec(l) // definitions are loaded unsubstituted, as from a file
		} else {
			in.ExecFull(l)
		}
	}
	return h, in
}

func TestDocExamples(t *testing.T) {
	cases := []struct{ name, script, want string }{
		{"advice", "/def advice = whisper %1 = Let the wookie win.\n/advice R2D2", "sent:whisper R2D2 = Let the wookie win."},
		{"greet default", "/set ending=meister\n/def greet = :waves to %{1-Jack}%{ending}.\n/greet", "sent::waves to Jackmeister."},
		{"greet arg", "/set ending=meister\n/def greet = :waves to %{1-Jack}%{ending}.\n/greet Dave", "sent::waves to Davemeister."},
		{"expr table", "/set X=5\n/echo $[3 + X * 2]", "out:13"},
		{"str eq", `/echo $["foo" =~ "bar"]`, "out:0"},
		{"glob", "/set name=Hawkeye\n/echo $[name =/ 'hawk*'] $[name =/ 'Hawk*']", "out:1 1"},
		{"string vs num", "/set X=5\n/echo $[X =~ \"+5\"] $[X == \"+5\"]", "out:0 1"},
		{"visual", "/set X=5\n/set visual=1\n/echo $[visual & (X > 0)]", "out:1"},
		{"int div", "/echo $[7/2] $[7.0/2]", "out:3 3.5"},
		{"while count", "/def count = /let i=1%; /while (i <= {1}) /echo %{i}%; /let i=$[i+1]%; /done\n/count 3", "out:1|2|3"},
		{"return func", "/def square = /return {1}*{1}\n/echo $[square(7)]", "out:49"},
		{"recursion", "/def fact = /if ({1} <= 1) /return 1%; /else /return {1}*fact({1}-1)%; /endif\n/echo $[fact(5)]", "out:120"},
		{"if/elseif/else", "/set a=2\n/if (a==1) /echo one%; /elseif (a==2) /echo two%; /else /echo other%; /endif", "out:two"},
		{"eval", "/def showvar = /eval /echo %{1} is %%{%{1}}.\n/set borg=on\n/showvar borg", "out:borg is on."},
		{"for", "/for i 1 3 /echo n%%i", "out:n1|n2|n3"},
		{"cmdsub", "/echo [$(/echo hi)]", "out:[hi]"},
		{"percent compress", "/echo 100%% sure $$x", "out:100% sure $x"},
		{"break", "/let i=0%; /while (1) /let i=$[i+1]%; /if (i>3) /break%; /endif%; /done%; /echo %i", "out:4"},
		{"regmatch", "/test regmatch('foo(.*)', 'foobar')\n/echo %{P1}", "out:bar"},
		{"neg", "/def f = /return 0\n/if (!f()) /echo no%; /endif", "out:no"},
		{"list if", "/if /test 1%; /then /echo yes%; /endif", "out:yes"},
	}
	for _, c := range cases {
		h, _ := run(t, c.script)
		got := ""
		if len(h.sent) > 0 {
			got = "sent:" + strings.Join(h.sent, "|")
		} else {
			got = "out:" + strings.Join(h.out, "|")
		}
		if got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}
