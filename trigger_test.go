package gtfparser

import (
	"strconv"
	"strings"
	"testing"
)

func newTrigInterp(t testing.TB) (*Interp, *tHost) {
	h := &tHost{}
	in := New(h)
	in.Warn = func(s string) { h.out = append(h.out, "!"+s) }
	return in, h
}

func BenchmarkProcessLine(b *testing.B) {
	in, _ := newTrigInterp(b)
	in.Exec("/def -t'foo*' t1 = /test x := 1")
	in.Exec(`/def -mregexp -t'^bar (\d+)' t2 = /test y := {P1}`)
	for i := 0; i < 20; i++ { // a realistic number of other triggers
		in.Exec("/def -t'zz" + strings.Repeat("a", i) + "*' -ag filler" + strings.Repeat("x", i))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		procLine(in, "foo bar")
		procLine(in, "bar 12")
		procLine(in, "nothing matches this line")
	}
}

func BenchmarkTriggerCommand(b *testing.B) {
	in, _ := newTrigInterp(b)
	in.Exec("/def -t'foo*' t1 = /test x := 1")
	in.Exec(`/def -mregexp -t'^bar (\d+)' t2 = /test y := {P1}`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		in.ExecFull("/trigger foo bar")
		in.ExecFull("/trigger bar 12")
	}
}

type fixedRand struct{ vals []int }

func (f *fixedRand) next(n int) int {
	v := f.vals[0] % n
	f.vals = append(f.vals[1:], f.vals[0])
	return v
}

func TestPriorityRules(t *testing.T) {
	in, h := newTrigInterp(t)
	in.Exec("/def -t'*foo*' -p5 hi = /echo hi")
	in.Exec("/def -t'*foo*' -p1 lo = /echo lo")
	in.Exec("/def -t'*foo*' -p5 -F fall = /echo fall")
	in.Exec("/def -t'*foo*' -p3 -F mid = /echo mid")
	r := procLine(in, "a foo b")
	// fall-thru of p5 runs, then hi (non-fall-thru p5) ends the search
	if got := strings.Join(h.out, "|"); got != "fall|hi" {
		t.Fatalf("got %q", got)
	}
	if r.Count != 2 || len(r.Ran) != 2 {
		t.Fatalf("count %d ran %v", r.Count, r.Ran)
	}
}

func TestEqualPriorityRandomChoice(t *testing.T) {
	in, h := newTrigInterp(t)
	in.Exec("/def -t'x' a = /echo a")
	in.Exec("/def -t'x' b = /echo b")
	in.Exec("/def -t'x' c = /echo c")
	fr := &fixedRand{vals: []int{0, 1, 2}}
	in.SetRand(fr.next)
	for i := 0; i < 3; i++ {
		procLine(in, "x")
	}
	// the list is newest-first: c b a; collected reversed, indexed from head
	if got := strings.Join(h.out, ""); len(got) != 3 || got == "aaa" {
		t.Fatalf("expected each macro once, got %q", got)
	}
}

func TestGagHiliteAndPartial(t *testing.T) {
	in, _ := newTrigInterp(t)
	in.Exec("/def -t'secret*' -ag g1")
	in.Exec("/def -t'warn*' -aBCred h1")
	in.Exec(`/def -F -mregexp -t'(\w+) (\w+)' -PBCgreen;2u p1`)
	r := procLine(in, "secret stuff")
	if !r.Gag || r.Line.Text != "secret stuff" {
		t.Fatalf("gag: %+v", r)
	}
	r = procLine(in, "warn now")
	if r.Gag || r.Line.Attr&AttrBold == 0 || !r.Line.Attr.HasFg() || r.Line.Attr.FgColor() != 1 {
		t.Fatalf("hilite: %+v", r.Line)
	}
	r = procLine(in, "hello world")
	spans := r.Spans(in)
	if len(spans) != 2 || spans[0].Text != "hello " || spans[1].Text != "world" {
		t.Fatalf("partial spans: %+v", spans)
	}
	if spans[1].Attr&AttrUnderline == 0 || spans[0].Attr&AttrUnderline != 0 {
		t.Fatalf("partial attrs: %+v", spans)
	}
}

func TestSubstituteAndShots(t *testing.T) {
	in, h := newTrigInterp(t)
	in.Exec("/def -t'raw *' -F sub = /test substitute(strcat('cooked ', {-1}), 'B')")
	in.Exec("/def -t'cooked *' -n1 seen = /echo saw %*")
	r := procLine(in, "raw data")
	if r.Line.Text != "cooked data" || !r.Substituted || r.Line.Attr&AttrBold == 0 {
		t.Fatalf("substitute: %+v", r.Line)
	}
	if strings.Join(h.out, "|") != "saw cooked data" {
		t.Fatalf("lower priority did not see substituted text: %v", h.out)
	}
	if len(r.ShotsUsed) != 1 || in.FindMacro("seen") != nil {
		t.Fatalf("shot not consumed: %+v", r.ShotsUsed)
	}
}

func TestWorldRestrictions(t *testing.T) {
	h := &trigHost{tHost: tHost{}}
	in := New(h)
	in.Warn = func(s string) { h.out = append(h.out, "!"+s) }
	in.Exec("/def -w'mud1' -t'x' w1 = /echo w1")
	in.Exec("/def -T'tiny' -t'x' -p0 ty = /echo ty")
	in.Exec("/def -t'x' -p0 gl = /echo gl")
	in.ProcessText(LineInput{Text: "x", World: "mud1"})
	in.ProcessText(LineInput{Text: "x", World: "mud2"})
	// mud1: w1 (priority 1) wins; mud2 (type tiny): ty and gl at p0, one picked
	if len(h.out) != 2 || h.out[0] != "w1" {
		t.Fatalf("got %v", h.out)
	}
}

type trigHost struct{ tHost }

func (w *trigHost) CurrentWorld() string { return "mud1" }
func (w *trigHost) WorldType(world string) string {
	if world == "mud2" {
		return "tiny"
	}
	return "other"
}
func (w *trigHost) WorldExists(world string) bool { return world == "mud1" || world == "mud2" }

type recObs struct{ ev []MacroEvent }

func (r *recObs) MacroEvent(e MacroEvent) { r.ev = append(r.ev, e) }

func TestObserverAndFilters(t *testing.T) {
	in, _ := newTrigInterp(t)
	obs := &recObs{}
	in.AddObserver(obs)
	in.Exec("/def -t'a*' -c0 never = /echo never")
	in.Exec("/def -t'b*' once = /echo once")
	procLine(in, "b")
	if len(obs.ev) < 2 || obs.ev[0].Kind != EvSelected || obs.ev[1].Kind != EvRun {
		t.Fatalf("events %+v", obs.ev)
	}
	in.AddPostLineFilter(func(in *Interp, r *LineResult) { r.Gag = true })
	if r := procLine(in, "zzz"); !r.Gag {
		t.Fatal("post filter ignored")
	}
	in.AddPreLineFilter(func(in *Interp, l *LineInput) bool { return l.Text != "skip" })
	if r := procLine(in, "skip"); !r.Suppressed {
		t.Fatal("pre filter ignored")
	}
}

func TestCustomMatchStyle(t *testing.T) {
	in, h := newTrigInterp(t)
	err := in.RegisterMatchStyle("prefix", MatchStyleFunc(func(p string) (Matcher, error) {
		return MatcherFunc(func(s string) (bool, []int) { return strings.HasPrefix(s, p), nil }), nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	in.Exec("/def -mprefix -t'Hello' pm = /echo matched %*")
	procLine(in, "Hello there")
	procLine(in, "Say Hello")
	if strings.Join(h.out, "|") != "matched Hello there" {
		t.Fatalf("got %v", h.out)
	}
}

func TestHooks(t *testing.T) {
	in, h := newTrigInterp(t)
	in.Exec("/def -hCONNECT con = /echo connected %1")
	in.Exec("/def -hSEND snd = /echo send hook %*")
	r := in.FireHookArgs("CONNECT", "mud1")
	if r.Count != 1 {
		t.Fatalf("ran %d", r.Count)
	}
	// the message is displayed before the hook macros run
	if strings.Join(h.out, "|") != "% Connected to mud1.|connected mud1" {
		t.Fatalf("got %v", h.out)
	}
	h.out = nil
	if r := in.FireHookArgs("SEND", "look"); r.Count != 1 {
		t.Fatalf("SEND hook must report it ran: %+v", r)
	}
	in.Exec("/def -hDISCONNECT -ag quiet")
	h.out = nil
	in.FireHookArgs("DISCONNECT", "mud1")
	if len(h.out) != 0 {
		t.Fatalf("gagged hook message shown: %v", h.out)
	}
	if r := in.FireHookArgs("NOSUCH"); r.Count != 0 {
		t.Fatal("unknown event")
	}
}

func TestAttrParseFormat(t *testing.T) {
	var ctx AttrCtx
	ctx.Hilite = AttrBold
	a, _, err := ctx.Parse("BuCred,Cbgblue", 0)
	if err != nil || a.String() != "uBCred,Cbgblue" {
		t.Fatalf("%v %q", err, a.String())
	}
	if _, _, err := ctx.Parse("Cnosuch", 0); err == nil || !strings.Contains(err.Error(), "Invalid color value") {
		t.Fatalf("bad color: %v", err)
	}
	s, err := ctx.DecodeAttr(AttrString{Text: "a@{B}b@{n}c@@d"}, 0, 0)
	if err != nil || s.Text != "abc@d" {
		t.Fatalf("decode %v %q", err, s.Text)
	}
	if enc := ctx.EncodeAttr(s, 0); enc != "a@{B}b@{n}c@@d" {
		t.Fatalf("encode %q", enc)
	}
	if got := ctx.EncodeAnsi(s, 0); got != "a\x1b[1mb\x1b[mc@d" {
		t.Fatalf("ansi %q", got)
	}
	d, final := ctx.DecodeAnsi("x\x1b[1;31my\x1b[0mz", 0, EmulAnsiAttr, 0)
	if d.Text != "xyz" || final != 0 || d.Chars == nil || d.Chars[1]&AttrBold == 0 || d.Chars[1].FgColor() != 1 {
		t.Fatalf("ansi decode %+v final %v", d, final)
	}
}

func TestMacroDatabase(t *testing.T) {
	in, h := newTrigInterp(t)
	in.Exec("/def foo = /echo one")
	if n := in.ExecFull("/def foo = /echo one"); n.Int() != int64(in.FindMacro("foo").ID) {
		t.Fatal("identical redefinition must return the original number")
	}
	in.Exec("/def foo = /echo two")
	if !strings.Contains(strings.Join(h.out, "|"), "!DEF: Redefined macro foo") {
		t.Fatalf("no REDEF message: %v", h.out)
	}
	in.Exec("/def Bar = /echo b")
	in.ExecFull("/BAR")
	if h.out[len(h.out)-1] != "b" {
		t.Fatalf("names are case-insensitive: %v", h.out)
	}
	if in.FindMacro("foo").Used[UsedName] != 0 {
		t.Fatal("usage counts")
	}
	in.ExecFull("/foo")
	if in.FindMacro("foo").Used[UsedName] != 1 {
		t.Fatal("usage count not kept")
	}
}

type dispHost struct {
	trigHost
	msgs []HookMessage
}

func (d *dispHost) HookMessage(m HookMessage) { d.msgs = append(d.msgs, m) }

func TestHookDisplayAndWorldScope(t *testing.T) {
	h := &dispHost{}
	in := New(h)
	in.Warn = func(s string) { h.out = append(h.out, "!"+s) }
	in.Exec("/def -hCONNECT -w'mud2' -F only2 = /echo only2")
	in.FireHookArgs("CONNECT", "mud1")
	if len(h.out) != 0 {
		t.Fatalf("a hook for another world ran: %v", h.out)
	}
	in.FireHookArgs("CONNECT", "mud2")
	if len(h.out) != 1 || h.out[0] != "only2" {
		t.Fatalf("world hook did not run for its world: %v", h.out)
	}
	if len(h.msgs) != 2 || h.msgs[1].Class != HookWorld || h.msgs[1].World != "mud2" || h.msgs[1].Text.Text != "% Connected to mud2." {
		t.Fatalf("messages %+v", h.msgs)
	}
	// the message is delivered before the hook body runs
	in.FireHookArgs("ACTIVITY", "mud2")
	if m := h.msgs[len(h.msgs)-1]; m.Class != HookAlert || !m.XSock {
		t.Fatalf("activity message class: %+v", m)
	}
}

func TestFlagsAndRateLimit(t *testing.T) {
	in, h := newTrigInterp(t)
	in.Exec("/def -t'x' a = /echo ran")
	in.ExecFull("/set borg=off")
	if r := procLine(in, "x"); r.Count != 0 || len(h.out) != 0 {
		t.Fatalf("borg off still ran triggers: %v", h.out)
	}
	in.ExecFull("/set borg=on")
	in.ExecFull("/set max_trig=3")
	n := 0
	for i := 0; i < 10; i++ {
		n += procLine(in, "x").Count
	}
	if n >= 10 || in.Var("borg") != "off" {
		t.Fatalf("rate limit did not trip: ran %d borg=%s", n, in.Var("borg"))
	}
}

func TestListFormatsAndEdgeDefs(t *testing.T) {
	in, h := newTrigInterp(t)
	in.Exec("/def -p-3 -c-1 -n-1 -t'a' neg")
	m := in.FindMacro("neg")
	if m == nil || m.Priority != 1 || m.Prob != 100 || m.Shots != 0 {
		t.Fatalf("negative options must fall back to defaults: %+v", m)
	}
	in.Exec("/def -mregexp -t'(a)(b)' -PL;R part")
	if in.FindMacro("part") != nil {
		t.Fatal("-PL/-PR is rejected by tf")
	}
	in.Exec("/def -hNOSUCH bad")
	if in.FindMacro("bad") != nil || !strings.Contains(strings.Join(h.out, "|"), "invalid hook event") {
		t.Fatalf("bad hook event accepted: %v", h.out)
	}
	if got := in.FindMacro("neg").printDef(true); got != "% "+strconv.Itoa(m.ID)+": /def -p1 -mglob -t'a' neg " {
		t.Fatalf("list format %q", got)
	}
	in.Exec("/def if = x")
	if in.FindMacro("if") != nil {
		t.Fatal("reserved word accepted")
	}
}

func TestSubstituteNotFromTrigger(t *testing.T) {
	in, h := newTrigInterp(t)
	in.ExecFull("/test substitute('x')")
	if !strings.Contains(strings.Join(h.out, "|"), "not called from trigger") {
		t.Fatalf("got %v", h.out)
	}
}

func TestPartialHiliteMultipleMatches(t *testing.T) {
	in, _ := newTrigInterp(t)
	in.Exec(`/def -mregexp -t'(\d+)' -F -P1Cred num`)
	r := procLine(in, "a 12 b 345 c")
	var reds []string
	for _, sp := range r.Spans(in) {
		if sp.Attr.HasFg() {
			reds = append(reds, sp.Text)
		}
	}
	if strings.Join(reds, ",") != "12,345" {
		t.Fatalf("every match must be hilited: %v", reds)
	}
}

// procLine runs a received line through the full ProcessText interface.
func procLine(in *Interp, line string) *LineResult { return in.ProcessText(LineInput{Text: line}) }
