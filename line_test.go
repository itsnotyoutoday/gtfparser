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
	if in.CurrentWorld() != "FOO" {
		t.Fatal("World()")
	}
}

type echoH struct {
	tHost
	got []EchoOpts
}

func (h *echoH) EchoTo(o EchoOpts, s string) {
	h.got = append(h.got, o)
	h.out = append(h.out, "to:"+s)
}

func TestEchoOptionsReachEchoHost(t *testing.T) {
	h := &echoH{}
	in := New(h)
	in.ExecFull("/echo -aBCred -wfoo hello")
	in.ExecFull("/echo plain")
	in.ExecFull("/echo -p @{Cgreen}x")
	if len(h.got) != 2 || h.got[0] != (EchoOpts{World: "foo", Attrs: "BCred"}) || !h.got[1].Markup {
		t.Fatalf("%+v out=%v", h.got, h.out)
	}
	if h.out[0] != "to:hello" || h.out[1] != "plain" {
		t.Fatalf("%v", h.out)
	}
}

func TestUndefID(t *testing.T) {
	_, in := run(t, "/def -t'a' = x\n/def named = y")
	n := len(in.Macros())
	ms := in.Macros()
	anon, named := ms[n-2], ms[n-1]
	if !in.UndefID(anon.ID) || len(in.Macros()) != n-1 || in.UndefID(anon.ID) {
		t.Fatalf("%v", in.Macros())
	}
	if !in.UndefID(named.ID) || len(in.Macros()) != n-2 {
		t.Fatal("named macro not removed")
	}
	if _, ok := in.macros["named"]; ok {
		t.Fatal("named macro left in the name index")
	}
}

func TestHookFromTypedLineExpandsItsBody(t *testing.T) {
	h, in := run(t, "/def -hSEND shk = /echo hooked %*")
	in.noExp = true // as inside Exec of a typed line
	in.Hook("SEND", "say hi")
	in.noExp = false
	if len(h.out) != 1 || h.out[0] != "hooked say hi" {
		t.Fatalf("%v", h.out)
	}
}

func TestFireHookReturnsWhatRan(t *testing.T) {
	_, in := run(t, "/def -hSEND -ag a1 = /echo x\n/def -hCONNECT c1 = /echo y")
	f := in.FireHook("SEND", "hello")
	if len(f) != 1 || f[0].Name != "a1" || f[0].Attrs != "g" {
		t.Fatalf("%v", f)
	}
}

type limHost struct{ tHost }

func (limHost) Redraw(string) int { return 1 }

func TestLimitFilterSnapshot(t *testing.T) {
	in := New(&limHost{})
	if in.LimitFilter("w") != nil {
		t.Fatal("no limit yet")
	}
	in.SetWorld("w")
	in.ExecFull("/limit -mglob foo*")
	f := in.LimitFilter("w")
	if f == nil || f("foo bar", "") || !f("bar", "") {
		t.Fatalf("filter wrong")
	}
	in.ExecFull("/unlimit")
	if f("foo bar", "") || !f("bar", "") {
		t.Fatal("a snapshot must not change under later /unlimit")
	}
	if in.LimitFilter("w") != nil {
		t.Fatal("unlimit")
	}
}
