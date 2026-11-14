package gtfparser

import "testing"

func TestProcessReportsMatchesWithoutSideEffects(t *testing.T) {
	h, in := run(t, "/def -t'danger*' -ag -n1 dg = flee\n/def -mregexp -t'(a+)(b+)' -P1Cred rx = rxbody")
	r := in.Process("danger ahead", false)
	if len(r.Matches) != 1 || r.Matches[0].Macro.Name != "dg" || r.Matches[0].Span != [2]int{0, 12} {
		t.Fatalf("matches: %+v", r.Matches)
	}
	if len(h.sent) != 0 {
		t.Fatalf("display-only run sent %v", h.sent)
	}
	if _, ok := in.macros["dg"]; !ok {
		t.Fatal("display-only run consumed the shot")
	}
	r = in.Process("xaabbx", false)
	if len(r.Matches) != 1 || r.Matches[0].Span != [2]int{1, 5} || len(r.Matches[0].Groups) != 3 || r.Matches[0].Groups[1] != [2]int{1, 3} {
		t.Fatalf("regexp match: %+v", r.Matches)
	}
	in.Process("danger ahead", true)
	if len(h.sent) != 1 || h.sent[0] != "flee" {
		t.Fatalf("live run: %v", h.sent)
	}
	if _, ok := in.macros["dg"]; ok {
		t.Fatal("live run did not consume the -n1 shot")
	}
}

func TestWorldScopedMacros(t *testing.T) {
	h, in := run(t, "/def -t'ping' -wfoo p1 = one\n/def -t'ping' -wbar -F p2 = two")
	in.SetWorld("bar")
	in.Process("ping", true)
	if len(h.sent) != 1 || h.sent[0] != "two" {
		t.Fatalf("world bar: %v", h.sent)
	}
	in.SetWorld("FOO")
	in.Process("ping", true)
	if len(h.sent) != 2 || h.sent[1] != "one" {
		t.Fatalf("world FOO: %v", h.sent)
	}
	if in.World() != "FOO" {
		t.Fatal("World()")
	}
}
