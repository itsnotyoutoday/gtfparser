package gtfparser

import (
	"strings"
	"testing"
)

func TestLoadValidatesBeforeInstalling(t *testing.T) {
	h := &tHost{}
	in := New(h)
	ok := func(in *Interp, a string) Value { return Int(1) }
	bad := []Extension{
		{Commands: []CommandSpec{{Name: "", Fn: ok}}},
		{Commands: []CommandSpec{{Name: "x"}}},
		{Commands: []CommandSpec{{Name: "x", Fn: ok}, {Name: "X", Fn: ok}}},
		{Commands: []CommandSpec{{Name: "x", Fn: ok, MinArgs: 3, MaxArgs: 1}}},
		{Functions: []FuncSpec{{Name: "f"}}},
	}
	for i, e := range bad {
		if err := in.Load(e); err == nil {
			t.Errorf("table %d should be rejected", i)
		}
	}
	// a rejected table installs nothing, even its good entries
	err := in.Load(Extension{Commands: []CommandSpec{{Name: "good", Fn: ok}, {Name: "bad"}}})
	if err == nil {
		t.Fatal("expected error")
	}
	if _, found := in.Command("good"); found {
		t.Fatal("partial install")
	}
}

func TestLoadArgCountsPrivilegesAndOverrides(t *testing.T) {
	h := &tHost{}
	in := New(h, WithRestriction(RestrictFile))
	in.Warn = func(s string) { h.out = append(h.out, "!"+s) }
	called := 0
	err := in.Load(Extension{Commands: []CommandSpec{
		{Name: "one", Fn: func(in *Interp, a string) Value { called++; return Int(1) }, Usage: "<x>", MinArgs: 1, MaxArgs: 1},
		{Name: "rd", Fn: func(in *Interp, a string) Value { called++; return Int(1) }, Needs: []Op{OpFileRead}},
		{Name: "echo", Fn: func(in *Interp, a string) Value { in.Echo("override:" + a); return Int(1) }},
	}})
	if err != nil {
		t.Fatal(err)
	}
	in.ExecFull("/one")
	in.ExecFull("/one a b")
	in.ExecFull("/one a")
	in.ExecFull("/rd")
	in.ExecFull("/@echo hi")
	want := "!usage: /one <x>|!usage: /one <x>|!/rd: restricted|override:hi"
	if got := strings.Join(h.out, "|"); got != want || called != 1 {
		t.Fatalf("got %q called=%d\nwant %q", got, called, want)
	}
}

func TestLoadVarsDisableAndFunctions(t *testing.T) {
	h := &tHost{}
	in := New(h)
	in.Warn = func(s string) { h.out = append(h.out, "!"+s) }
	in.Load(Extension{
		Vars:            map[string]string{"motd": "hi"},
		Functions:       []FuncSpec{{Name: "twice", Fn: func(in *Interp, a []Value) (Value, error) { return Int(a[0].Int() * 2), nil }, Usage: "n", MinArgs: 1, MaxArgs: 1}},
		DisableCommands: []string{"sh"},
	})
	in.ExecFull("/echo %{motd} $[twice(4)]")
	in.ExecFull("/echo $[twice()]")
	in.ExecFull("/sh true")
	if got := strings.Join(h.out, "|"); !strings.Contains(got, "hi 8") || !strings.Contains(got, "usage: twice(n)") || !strings.Contains(got, "/sh: disabled") {
		t.Fatalf("got %q", got)
	}
}
