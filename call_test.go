package gtfparser

import (
	"strings"
	"testing"
)

func TestCallOptionsAndAliases(t *testing.T) {
	h := &tHost{}
	in := New(h, WithoutEnvironment())
	in.Warn = func(s string) { h.out = append(h.out, "!"+s) }
	var got *Call
	err := in.AddCommand(CommandSpec{Name: "t", Aliases: []string{"tt"}, Options: "ab:n#", MinArgs: 1, Usage: "u",
		Do: func(c *Call) Value { got = c; return Int(7) }})
	if err != nil {
		t.Fatal(err)
	}
	v := in.Run("/tt -a -b'x y' -n5 -- one two")
	if v.Int() != 7 || got == nil {
		t.Fatalf("not called: %v", h.out)
	}
	if !got.Has('a') || got.Has('z') || got.Opt('b') != "x y" || got.Int('n', 0) != 5 || got.Int('q', 9) != 9 {
		t.Fatalf("options: %+v", got.opts)
	}
	if got.Rest != "one two" || len(got.Args) != 2 || got.Arg(1) != "two" || got.Arg(5) != "" || got.Name != "t" {
		t.Fatalf("args: %q %v name=%q", got.Rest, got.Args, got.Name)
	}
	in.Run("/t")
	in.Run("/t -z x")
	if want := "!usage: /t u|!/t: invalid option 'z'|!/t: options: -a -b<string> -n<integer>"; strings.Join(h.out, "|") != want {
		t.Fatalf("got %q", strings.Join(h.out, "|"))
	}
}

func TestFuncBinding(t *testing.T) {
	h := &tHost{}
	in := New(h, WithoutEnvironment())
	in.Warn = func(s string) { h.out = append(h.out, "!"+s) }
	in.AddFunction(Func("add", func(a, b int) int { return a + b }))
	in.AddFunction(Func("half", func(x float64) float64 { return x / 2 }))
	in.AddFunction(Func("join", func(sep string, parts ...string) string { return strings.Join(parts, sep) }))
	in.AddFunction(Func("odd", func(n int) (bool, error) { return n%2 == 1, nil }))
	in.AddFunction(Func("boom", func() (string, error) { return "", errBoom }))
	in.Run("/echo $[add(2, 3)] $[half(5)] $[join('-', 'a', 'b', 'c')] $[odd(3)] $[add('4x', 1)]")
	in.Run("/echo $[add(1)]")
	in.Run("/echo $[boom()]")
	want := "5 2.5 a-b-c 1 5|!usage: add(int, int)|!boom!"
	if got := strings.Join(h.out, "|"); !strings.HasPrefix(got, want[:20]) || !strings.Contains(got, "usage: add(int, int)") || !strings.Contains(got, "boom") {
		t.Fatalf("got %q", got)
	}
}

var errBoom = errString("boom!")

type errString string

func (e errString) Error() string { return string(e) }
