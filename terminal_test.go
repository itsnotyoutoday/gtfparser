package gtfparser

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"weak"
)

// kHost is a fake client implementing every optional terminal capability.
type kHost struct {
	out, sent []string
	events    []Event
	keys      map[string]string
	world     string
	open      map[string]bool
	cols, ln  int
	statusVal map[string]string
	shown     []string
	prompts   []string
	more      *kMore
	winl      int
}

type kMore struct {
	paused bool
	size   int
	scroll int
}

func newKHost() *kHost {
	return &kHost{keys: map[string]string{}, open: map[string]bool{}, statusVal: map[string]string{}, cols: 80, ln: 24}
}

func (h *kHost) Echo(s string)         { h.out = append(h.out, s) }
func (h *kHost) Send(w, s string) bool { h.sent = append(h.sent, s); return true }
func (h *kHost) Notify(e Event)        { h.events = append(h.events, e) }
func (h *kHost) KeyCode(n string) string {
	return h.keys[strings.ToLower(n)]
}
func (h *kHost) Size() (int, int)          { return h.cols, h.ln }
func (h *kHost) Connect(w string) error    { h.open[w] = true; return nil }
func (h *kHost) Disconnect(w string) error { delete(h.open, w); return nil }
func (h *kHost) Foreground(w string) error { h.world = w; return nil }
func (h *kHost) ForegroundWorld() string   { return h.world }
func (h *kHost) Connected(w string) bool   { return h.open[w] }
func (h *kHost) DisplayLine(w, l string)   { h.shown = append(h.shown, w+":"+l) }
func (h *kHost) SetPrompt(w, t string)     { h.prompts = append(h.prompts, w+":"+t) }
func (h *kHost) StatusValue(n string) (string, bool) {
	v, ok := h.statusVal[n]
	return v, ok
}

func (h *kHost) kinds(kind string) []Event {
	var r []Event
	for _, e := range h.events {
		if e.Kind == kind {
			r = append(r, e)
		}
	}
	return r
}

func newKB(t *testing.T) (*kHost, *Interp) {
	t.Helper()
	h := newKHost()
	in := New(h, WithoutEnvironment())
	in.Warn = func(s string) { h.out = append(h.out, "!"+s) }
	return h, in
}

func call(t *testing.T, in *Interp, name string, args ...Value) Value {
	t.Helper()
	v, err := in.callFunc(name, args)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return v
}

func bufState(in *Interp) string {
	b := in.Input()
	return fmt.Sprintf("[%s|%s]", b.Text()[:b.Cursor()], b.Text()[b.Cursor():])
}

func TestInputBufferFunctions(t *testing.T) {
	_, in := newKB(t)
	changes := 0
	in.Input().OnChange = func() { changes++ }
	in.ExecFull("/input hello world")
	if got := bufState(in); got != "[hello world|]" {
		t.Fatal(got)
	}
	if call(t, in, "kbgoto", Int(5)).Int() != 5 || call(t, in, "kbhead").String() != "hello" || call(t, in, "kbtail").String() != " world" {
		t.Fatal("kbgoto/head/tail", bufState(in))
	}
	if call(t, in, "kbgoto", Int(99)).Int() != 11 || call(t, in, "kbgoto", Int(-3)).Int() != 0 {
		t.Fatal("kbgoto clamps")
	}
	if call(t, in, "kblen").Int() != 11 || call(t, in, "kbpoint").Int() != 0 {
		t.Fatal("kblen/kbpoint")
	}
	call(t, in, "kbgoto", Int(6))
	if call(t, in, "kbdel", Int(11)).Int() != 6 || bufState(in) != "[hello |]" {
		t.Fatal("kbdel right", bufState(in))
	}
	if call(t, in, "kbdel", Int(0)).Int() != 0 || bufState(in) != "[|]" {
		t.Fatal("kbdel left", bufState(in))
	}
	if changes == 0 {
		t.Fatal("OnChange never called")
	}
	// word and match functions
	in.Input().Set("foo bar_baz (a[b])", 0)
	if r := call(t, in, "kbwordright", Int(0)).Int(); r != 3 {
		t.Fatalf("wordright %d", r)
	}
	if r := call(t, in, "kbwordleft", Int(7)).Int(); r != 4 {
		t.Fatalf("wordleft %d", r)
	}
	if r := call(t, in, "kbmatch", Int(8)).Int(); r != 17 {
		t.Fatalf("match %d", r)
	}
	if r := call(t, in, "kbmatch", Int(13)).Int(); r != 16 {
		t.Fatalf("match[ %d", r)
	}
	if r := call(t, in, "kbmatch", Int(17)).Int(); r != 12 {
		t.Fatalf("match) backward %d", r)
	}
	if r := call(t, in, "kbmatch", Int(0)).Int(); r != 17 {
		t.Fatalf("match fwd search %d", r)
	}
	if r := call(t, in, "kbmatch", Int(18)).Int(); r != -1 {
		t.Fatalf("match none %d", r)
	}
	// UTF-8: positions are bytes, steps are characters
	in.Input().Set("héllo", -1)
	if call(t, in, "kbcharback", Int(1)).Int() != 5 || call(t, in, "kbcharback", Int(4)).Int() != 1 {
		t.Fatal("charback")
	}
	in.Input().Set("héllo", 1)
	if call(t, in, "kbcharfwd", Int(1)).Int() != 3 {
		t.Fatal("charfwd over é")
	}
	// argument counts are checked as in tf
	if _, err := in.callFunc("kbcharback", nil); err == nil || !strings.Contains(err.Error(), "found 0 arguments, expected 1") {
		t.Fatalf("arg check: %v", err)
	}
	if _, err := in.callFunc("kbhead", []Value{Int(1)}); err == nil {
		t.Fatal("kbhead takes no arguments")
	}
}

func TestTypeTextInsertOverwriteKbnum(t *testing.T) {
	_, in := newKB(t)
	in.Input().Set("abcdef", 2)
	in.TypeText("XY")
	if bufState(in) != "[abXY|cdef]" {
		t.Fatal(bufState(in))
	}
	in.SetVar("insert", "off")
	in.globals["insert"] = Str("off")
	in.setGlobal("insert", Str("off"))
	in.TypeText("12")
	if bufState(in) != "[abXY12|ef]" {
		t.Fatal(bufState(in))
	}
	in.TypeText("789")
	if bufState(in) != "[abXY12789|]" {
		t.Fatal(bufState(in))
	}
	in.setGlobal("insert", Str("on"))
	in.setGlobal("kbnum", Str("+3"))
	in.TypeText("z")
	if bufState(in) != "[abXY12789zzz|]" || in.Var("kbnum") != "" {
		t.Fatal(bufState(in), in.Var("kbnum"))
	}
	in.Input().Clear()
	in.Input().Set("héllo", 0)
	in.setGlobal("insert", Str("off"))
	in.TypeText("ab")
	if in.Input().Text() != "abllo" {
		t.Fatalf("overwrite must replace whole characters: %q", in.Input().Text())
	}
}

func TestDokeyEditing(t *testing.T) {
	_, in := newKB(t)
	run := func(s string) string { in.ExecFull(s); return bufState(in) }
	in.Input().Set("one two three", -1)
	for _, c := range []struct{ cmd, want string }{
		{"/dokey bword", "[one two |]"},
		{"/dokey bword", "[one |]"},
		{"/dokey home", "[|one ]"},
		{"/dokey dch", "[|ne ]"},
		{"/dokey right", "[n|e ]"},
		{"/dokey left", "[|ne ]"},
		{"/dokey wright", "[ne| ]"},
		{"/dokey wleft", "[|ne ]"},
		{"/dokey end", "[ne |]"},
		{"/dokey bspc", "[ne|]"},
		{"/dokey dline", "[|]"},
	} {
		if got := run(c.cmd); got != c.want {
			t.Fatalf("%s: got %s want %s", c.cmd, got, c.want)
		}
	}
	in.Input().Set("alpha beta gamma", 6)
	if got := run("/dokey dword"); got != "[alpha | gamma]" {
		t.Fatal(got)
	}
	in.Input().Set("alpha beta gamma", 6)
	if got := run("/dokey deol"); got != "[alpha |]" {
		t.Fatal(got)
	}
	// a numeric prefix repeats the operation
	in.Input().Set("abcdefghij", -1)
	in.setGlobal("kbnum", Str("3"))
	if got := run("/dokey bspc"); got != "[abcdefg|]" {
		t.Fatal(got)
	}
	// the return value is the new cursor
	if v := in.ExecFull("/dokey home"); v.Int() != 0 {
		t.Fatal(v)
	}
	// UP/DOWN move by %wrapsize
	in.resetKbnum()
	in.setGlobal("wrapsize", Str("3"))
	in.Input().Set("abcdefghij", 0)
	run("/dokey down")
	run("/dokey down")
	if in.Input().Cursor() != 6 {
		t.Fatal(in.Input().Cursor())
	}
	run("/dokey up")
	if in.Input().Cursor() != 3 {
		t.Fatal(in.Input().Cursor())
	}
	// a /dokey_<name> macro takes over an editing operation
	in.Exec("/def dokey_home = /test kbgoto(2)")
	in.Input().Set("abcdef", 5)
	if got := run("/dokey home"); got != "[ab|cdef]" {
		t.Fatalf("macro override: %s", got)
	}
	// unknown operations fail loudly
	h, in2 := newKB(t)
	if v := in2.ExecFull("/dokey frobnicate"); v.Truth() {
		t.Fatal("unknown dokey must fail")
	}
	if !strings.Contains(strings.Join(h.out, "|"), "No editing function frobnicate") {
		t.Fatal(h.out)
	}
	// every documented name is accepted somewhere
	for _, n := range DokeyNames() {
		h, in3 := newKB(t)
		in3.ExecFull("/dokey " + n)
		for _, o := range h.out {
			if strings.Contains(o, "No editing function") {
				t.Errorf("/dokey %s not implemented", n)
			}
		}
	}
}

func TestDokeyClientOperations(t *testing.T) {
	h, in := newKB(t)
	for _, n := range []string{"REDRAW", "CLEAR", "REFRESH", "SELFLUSH"} {
		in.ExecFull("/dokey " + strings.ToLower(n))
	}
	var got []string
	for _, e := range h.kinds("dokey") {
		got = append(got, e.Args[0])
	}
	if strings.Join(got, ",") != "REDRAW,CLEAR,REFRESH,SELFLUSH" {
		t.Fatalf("events %v", got)
	}
	// without a Notifier these cannot work, and say so
	th := &tHost{}
	in2 := New(th)
	in2.Warn = func(s string) { th.out = append(th.out, "!"+s) }
	if v := in2.ExecFull("/dokey redraw"); v.Truth() {
		t.Fatal("must fail")
	}
	if len(th.out) == 0 || !strings.Contains(th.out[0], "not supported by this client") {
		t.Fatal(th.out)
	}
}

func TestInputHistoryAndSubmit(t *testing.T) {
	h, in := newKB(t)
	in.Exec("/def hello = /echo hi %1")
	for _, l := range []string{"/hello one", "/hello two", "/hello two", "say foo"} {
		in.Input().Set(l, -1)
		in.SubmitInput()
	}
	if got := strings.Join(in.InputHistory(), "|"); got != "/hello one|/hello two|say foo" {
		t.Fatalf("history (dup suppressed): %s", got)
	}
	if strings.Join(h.out, "|") != "hi one|hi two|hi two" || strings.Join(h.sent, "|") != "say foo" {
		t.Fatalf("execution: out=%v sent=%v", h.out, h.sent)
	}
	if in.Input().Text() != "" {
		t.Fatal("buffer must be cleared")
	}
	// recall back and forth
	in.ExecFull("/dokey recallb")
	if in.Input().Text() != "say foo" {
		t.Fatal(in.Input().Text())
	}
	in.ExecFull("/dokey recallb")
	in.ExecFull("/dokey recallb")
	if in.Input().Text() != "/hello one" {
		t.Fatal(in.Input().Text())
	}
	n := len(h.events)
	in.ExecFull("/dokey recallb") // no more: beeps
	if in.Input().Text() != "/hello one" || len(h.events) != n+1 || h.events[n].Kind != "beep" {
		t.Fatal("recall past the start must beep", h.events[n:])
	}
	in.ExecFull("/dokey recallf")
	if in.Input().Text() != "/hello two" {
		t.Fatal(in.Input().Text())
	}
	in.ExecFull("/dokey recallend")
	if in.Input().Text() != "" {
		t.Fatalf("recallend returns to the empty line, got %q", in.Input().Text())
	}
	in.ExecFull("/dokey recallbeg")
	if in.Input().Text() != "/hello one" {
		t.Fatal(in.Input().Text())
	}
	// searching recalls lines that begin with what is typed
	in.Input().Clear()
	in.ts().hist.sync()
	in.Input().Set("say", -1)
	in.ExecFull("/dokey searchb")
	if in.Input().Text() != "say foo" {
		t.Fatalf("search: %q", in.Input().Text())
	}
	// ^old^new substitution repeats the latest matching line
	h.out = nil
	in.Input().Set("^two^three", -1)
	in.SubmitInput()
	if strings.Join(h.out, "|") != "hi three" {
		t.Fatal(h.out)
	}
	h.out = nil
	in.Input().Set("^zzz^y", -1)
	in.SubmitInput()
	if strings.Join(h.out, "|") != "% No match." {
		t.Fatal(h.out)
	}
	// %kecho shows the line
	h.out = nil
	in.setGlobal("kecho", Str("on"))
	in.setGlobal("kprefix", Str("> "))
	in.Input().Set("/hello k", -1)
	in.SubmitInput()
	if strings.Join(h.out, "|") != "> /hello k|hi k" {
		t.Fatal(h.out)
	}
}

func TestKeyPressedBindings(t *testing.T) {
	h, in := newKB(t)
	in.Exec("/bind ^A = /echo bound-A")
	in.Exec("/bind ^[x = /echo bound-esc-x")
	in.Exec("/bind ^[[A = /echo up-arrow")
	in.Exec("/bind ^X^Y = /echo xy")
	in.KeyPressed("ab")
	if bufState(in) != "[ab|]" {
		t.Fatal(bufState(in))
	}
	in.KeyPressed("\x01")
	if strings.Join(h.out, "|") != "bound-A" {
		t.Fatal(h.out)
	}
	// a prefix is held until the sequence completes
	in.KeyPressed("\x1b")
	if in.PendingKeys() != "\x1b" || bufState(in) != "[ab|]" {
		t.Fatal("prefix must be held", in.PendingKeys())
	}
	in.KeyPressed("x")
	if h.out[len(h.out)-1] != "bound-esc-x" || in.PendingKeys() != "" {
		t.Fatal(h.out)
	}
	in.KeyPressed("\x1b[A")
	if h.out[len(h.out)-1] != "up-arrow" {
		t.Fatal(h.out)
	}
	// split across calls
	in.KeyPressed("\x18")
	in.KeyPressed("\x19")
	if h.out[len(h.out)-1] != "xy" {
		t.Fatal(h.out)
	}
	// an unbound sequence is typed
	in.KeyPressed("\x1bq")
	if !strings.HasSuffix(in.Input().Text(), "\x1bq") {
		t.Fatalf("%q", in.Input().Text())
	}
	// FlushKeys gives up on a held prefix
	in.Input().Clear()
	in.KeyPressed("\x1b")
	in.FlushKeys()
	if in.Input().Text() != "\x1b" || in.PendingKeys() != "" {
		t.Fatalf("%q %q", in.Input().Text(), in.PendingKeys())
	}
	// the default bindings: ^W deletes a word, ^V makes the next key literal
	in.Input().Set("foo bar", -1)
	in.KeyPressed("\x17")
	if in.Input().Text() != "foo " {
		t.Fatal(in.Input().Text())
	}
	in.KeyPressed("\x16\x01")
	if in.Input().Text() != "foo \x01" {
		t.Fatalf("%q", in.Input().Text())
	}
	// backspace and DEL
	in.KeyPressed("\x7f\b")
	if in.Input().Text() != "foo" {
		t.Fatalf("%q", in.Input().Text())
	}
}

func TestKeyPressedEnterAndOrder(t *testing.T) {
	h, in := newKB(t)
	in.Exec("/def hi = /echo hi %1")
	in.KeyPressed("/hi there\rsecond line\nabc")
	if strings.Join(h.out, "|") != "hi there" || strings.Join(h.sent, "|") != "second line" {
		t.Fatalf("out=%v sent=%v", h.out, h.sent)
	}
	if in.Input().Text() != "abc" {
		t.Fatalf("%q", in.Input().Text())
	}
	// /dokey newline from inside a key macro runs the line after the macro
	h.out = nil
	in.Input().Clear()
	in.Exec("/bind ^G = /input /echo from-bind%; /dokey newline%; /echo macro-end")
	in.KeyPressed("\x07")
	if strings.Join(h.out, "|") != "macro-end|from-bind" {
		t.Fatalf("%v", h.out)
	}
	// from the command line it runs at once
	h.out = nil
	in.Input().Set("/echo now", -1)
	in.ExecFull("/dokey newline")
	if strings.Join(h.out, "|") != "now" {
		t.Fatalf("%v", h.out)
	}
}

func TestKbnumPrefix(t *testing.T) {
	h, in := newKB(t)
	// the library binds ^[1.. to /set kbnum=+N
	in.Exec("/bind ^[0 = /set kbnum=+0")
	for d := 1; d <= 9; d++ {
		in.Exec(fmt.Sprintf("/bind ^[%d = /set kbnum=+%d", d, d))
	}
	in.Exec("/bind ^[- = /set kbnum=-")
	in.Exec("/bind ^B = /echo kbnum=%kbnum")
	in.KeyPressed("\x1b4")
	in.KeyPressed("2") // a further digit extends the prefix
	if in.Var("kbnum") != "+42" {
		t.Fatalf("kbnum=%q", in.Var("kbnum"))
	}
	in.KeyPressed("\x02") // the macro sees the count as its local %kbnum and the global is reset
	if len(h.out) != 1 || h.out[0] != "kbnum=42" || in.Var("kbnum") != "" {
		t.Fatalf("%v %q", h.out, in.Var("kbnum"))
	}
	// text typed after a prefix is repeated
	in.KeyPressed("\x1b3")
	in.KeyPressed("a")
	if in.Input().Text() != "aaa" {
		t.Fatalf("%q", in.Input().Text())
	}
	// backspace takes the count too
	in.KeyPressed("\x1b2\x7f")
	if in.Input().Text() != "a" {
		t.Fatalf("%q", in.Input().Text())
	}
	// the count is capped by %max_kbnum
	in.setGlobal("max_kbnum", Int(50))
	in.KeyPressed("\x1b9")
	in.KeyPressed("99")
	if in.Var("kbnum") != "+50" {
		t.Fatalf("%q", in.Var("kbnum"))
	}
}

func TestKeyBindingByName(t *testing.T) {
	h, in := newKB(t)
	h.keys["up"] = "\x1b[A"
	in.Exec("/def -Bup keyup = /echo got-up")
	in.KeyPressed("\x1b[A")
	if strings.Join(h.out, "|") != "got-up" {
		t.Fatal(h.out)
	}
	if code, ok := in.KeyCode("UP"); !ok || code != "\x1b[A" {
		t.Fatal(code, ok)
	}
	if v := call(t, in, "keycode", Str("Up")).String(); v != "\x1b[A" {
		t.Fatalf("%q", v)
	}
	if v := call(t, in, "keycode", Str("F5")).String(); v != "" {
		t.Fatalf("known name without a code: %q", v)
	}
	h.out = nil
	call(t, in, "keycode", Str("Bogus"))
	if len(h.out) != 1 || !strings.Contains(h.out[0], "unknown key name") {
		t.Fatal(h.out)
	}
	bs := in.Bindings()
	found := false
	for _, b := range bs {
		if b.Seq == "\x1b[A" && b.Macro.Name == "keyup" {
			found = true
		}
	}
	if !found {
		t.Fatal("Bindings must list the -B macro")
	}
}

func TestPrintToAscii(t *testing.T) {
	for in, want := range map[string]string{
		"^A": "\x01", "^[[A": "\x1b[A", "^?": "\x7f", `\033[B`: "\x1b[B", `\0x1b`: "\x1b", `\27`: "\x1b",
		`\^`: "^", "^@": "\x80", "abc": "abc", `^X^V`: "\x18\x16",
	} {
		if got := PrintToAscii(in); got != want {
			t.Errorf("PrintToAscii(%q) = %q, want %q", in, got, want)
		}
	}
	if AsciiToPrint("\x1b[A\x7f^") != `^[[A^?\^` {
		t.Fatal(AsciiToPrint("\x1b[A\x7f^"))
	}
}

func TestBeepLocalechoSuspend(t *testing.T) {
	h, in := newKB(t)
	in.ExecFull("/beep")
	in.ExecFull("/beep 2")
	if ev := h.kinds("beep"); len(ev) != 2 || ev[0].Args[0] != "3" || ev[1].Args[0] != "2" {
		t.Fatalf("%v", h.events)
	}
	in.ExecFull("/beep off")
	in.ExecFull("/beep 5")
	if len(h.kinds("beep")) != 2 || in.Var("beep") != "off" {
		t.Fatal("beep off must stop beeps", h.events, in.Var("beep"))
	}
	in.ExecFull("/beep on")
	if in.Var("beep") != "on" {
		t.Fatal()
	}
	if in.ExecFull("/beep -3").Truth() {
		t.Fatal("negative count fails")
	}
	// localecho keeps state and tells the client
	h.world = "w1"
	if !in.ExecFull("/localecho").Truth() {
		t.Fatal("echo is on by default")
	}
	in.ExecFull("/localecho off")
	if in.ExecFull("/localecho").Truth() || in.LocalEcho("w1") {
		t.Fatal("echo should be off")
	}
	if ev := h.kinds("localecho"); len(ev) != 1 || ev[0].Args[0] != "w1" || ev[0].Args[1] != "off" {
		t.Fatalf("%v", ev)
	}
	h.world = "w2"
	if !in.LocalEcho("") {
		t.Fatal("state is per world")
	}
	in.SetLocalEcho("w1", true)
	h.world = "w1"
	if !in.LocalEcho("") {
		t.Fatal("client can report the negotiated state")
	}
	in.ExecFull("/suspend")
	if len(h.kinds("suspend")) != 1 {
		t.Fatal(h.events)
	}
	// no Notifier: beep, localecho and suspend fail loudly
	th := &tHost{}
	in2 := New(th)
	in2.Warn = func(s string) { th.out = append(th.out, "!"+s) }
	in2.ExecFull("/beep")
	in2.ExecFull("/suspend")
	in2.ExecFull("/localecho off")
	if len(th.out) != 3 {
		t.Fatalf("%v", th.out)
	}
	for _, o := range th.out {
		if !strings.Contains(o, "not supported by this client") {
			t.Fatal(o)
		}
	}
}

func TestMoreFunctionsCoreState(t *testing.T) {
	h, in := newKB(t)
	h.world = "w"
	in.SetMoreState("w", MoreState{Paused: true, Back: 30, New: 12, Above: 100})
	if call(t, in, "morepaused").Int() != 1 || call(t, in, "morepaused", Str("w")).Int() != 1 {
		t.Fatal("morepaused")
	}
	if call(t, in, "moresize").Int() != 30 || call(t, in, "moresize", Str("n")).Int() != 12 {
		t.Fatal("moresize")
	}
	if _, err := in.callFunc("moresize", []Value{Str("q")}); err != nil {
		t.Fatal(err)
	}
	// scroll forward 10: 10 scrolled, 20 left, new capped
	if got := call(t, in, "morescroll", Int(10)).Int(); got != 10 {
		t.Fatal(got)
	}
	if st := in.MoreState("w"); st.Back != 20 || st.New != 12 || st.Above != 110 || !st.Paused {
		t.Fatalf("%+v", st)
	}
	if got := call(t, in, "morescroll", Int(25)).Int(); got != 20 {
		t.Fatal(got)
	}
	if st := in.MoreState("w"); st.Back != 0 || st.New != 0 || st.Paused {
		t.Fatalf("reaching the bottom unpauses: %+v", st)
	}
	if got := call(t, in, "morescroll", Int(5)).Int(); got != 0 {
		t.Fatal("not paused: nothing to scroll", got)
	}
	// reverse scroll is limited by the history above
	in.SetMoreState("w", MoreState{Above: 7})
	if got := call(t, in, "morescroll", Int(-10)).Int(); got != -7 {
		t.Fatal(got)
	}
	if st := in.MoreState("w"); st.Back != 7 || st.Above != 0 || !st.Paused {
		t.Fatalf("%+v", st)
	}
	// other worlds: only counted when active or backed up beyond the new lines
	in.SetMoreState("other", MoreState{Back: 5, New: 5})
	if call(t, in, "moresize", Str(""), Str("other")).Int() != 0 {
		t.Fatal("inactive other world")
	}
	in.SetMoreState("other", MoreState{Back: 5, New: 5, Active: true})
	if call(t, in, "moresize", Str("n"), Str("other")).Int() != 5 || call(t, in, "nactive").Int() != 1 || call(t, in, "nactive", Str("other")).Int() != 5 {
		t.Fatal("active other world")
	}
	// /limit counts
	in.SetMoreState("w", MoreState{Back: 9, New: 4, BackFiltered: 3, NewFiltered: 1})
	if call(t, in, "moresize", Str("l")).Int() != 3 || call(t, in, "moresize", Str("ln")).Int() != 1 {
		t.Fatal("filtered counts")
	}
	if len(h.kinds("more")) == 0 {
		t.Fatal("scrolling must notify the client")
	}
}

type kMoreHost struct {
	*kHost
	paused  bool
	scrolls []int
}

func (m *kMoreHost) MorePaused(w string) bool { return m.paused }
func (m *kMoreHost) MoreSize(f, w string) int { return 42 }
func (m *kMoreHost) MoreScroll(n int) int     { m.scrolls = append(m.scrolls, n); return n / 2 }
func (m *kMoreHost) WinLines() int            { return 17 }

func TestMoreHostIsAuthoritative(t *testing.T) {
	m := &kMoreHost{kHost: newKHost(), paused: true}
	in := New(m)
	if call(t, in, "morepaused").Int() != 1 || call(t, in, "moresize").Int() != 42 || call(t, in, "winlines").Int() != 17 {
		t.Fatal("host answers")
	}
	if call(t, in, "morescroll", Int(8)).Int() != 4 || len(m.scrolls) != 1 || m.scrolls[0] != 8 {
		t.Fatal(m.scrolls)
	}
	in.ExecFull("/dokey page")
	in.ExecFull("/dokey hpageback")
	in.ExecFull("/dokey line")
	if fmt.Sprint(m.scrolls) != "[8 17 -8 1]" {
		t.Fatalf("%v", m.scrolls)
	}
	in.setGlobal("kbnum", Str("2"))
	in.ExecFull("/dokey lineback")
	if m.scrolls[len(m.scrolls)-1] != -2 {
		t.Fatal(m.scrolls)
	}
}

func TestDokeyPauseFlush(t *testing.T) {
	h, in := newKB(t)
	h.world = "w"
	in.Exec("/def -hMORE onmore = /echo more-hook")
	if v := in.ExecFull("/dokey pause"); v.Int() != 1 || !in.MoreState("w").Paused {
		t.Fatal("pause")
	}
	if v := in.ExecFull("/dokey pause"); v.Truth() {
		t.Fatal("already paused returns 0")
	}
	if strings.Join(h.out, "|") != "more-hook" {
		t.Fatalf("MORE hook must run once: %v", h.out)
	}
	in.SetMoreState("w", MoreState{Paused: true, Back: 9, New: 3})
	in.ExecFull("/dokey flush")
	if st := in.MoreState("w"); st.Paused || st.Back != 0 || st.New != 0 {
		t.Fatalf("%+v", st)
	}
	// turning %more off releases a pause
	in.setGlobal("more", Str("on"))
	in.PollVars()
	in.SetMoreState("w", MoreState{Paused: true, Back: 4, New: 4})
	in.setGlobal("more", Str("off"))
	in.PollVars()
	if in.MoreState("w").Paused {
		t.Fatal("more off must release")
	}
}

func TestWinlines(t *testing.T) {
	h, in := newKB(t)
	h.cols, h.ln = 100, 40
	if call(t, in, "winlines").Int() != 39 {
		t.Fatal("non-visual: lines-1")
	}
	in.setGlobal("visual", Str("on"))
	in.setGlobal("isize", Int(3))
	in.setGlobal("status_height", Int(2))
	if got := call(t, in, "winlines").Int(); got != 35 {
		t.Fatalf("visual: lines - isize - status_height = 35, got %d", got)
	}
	ts := in.TermSettings()
	if ts.WinLines != 35 || !ts.Visual || ts.Columns != 100 || ts.StatusHeight != 2 || !ts.Insert {
		t.Fatalf("%+v", ts)
	}
}

func TestPromptAndFakeRecv(t *testing.T) {
	h, in := newKB(t)
	in.Exec("/def -t'hello *' gagger = /echo triggered %*")
	in.macros["gagger"].Gag = true
	in.Exec("/def -t'show *' shower = /echo trig-show")
	// no world: fails loudly
	if v := call(t, in, "fake_recv", Str("show x")); v.Truth() {
		t.Fatal("no current world")
	}
	if v := call(t, in, "prompt", Str("p> ")); v.Truth() {
		t.Fatal("prompt with no current socket returns 0")
	}
	h.out = nil
	h.world = "w"
	h.open["w"] = true
	if v := call(t, in, "fake_recv", Str("show me")); !v.Truth() {
		t.Fatal("fake_recv")
	}
	if strings.Join(h.out, "|") != "trig-show" || fmt.Sprint(h.shown) != "[w:show me]" {
		t.Fatalf("displayed through LineHost: %v %v", h.out, h.shown)
	}
	h.shown = nil
	h.out = nil
	call(t, in, "fake_recv", Str("hello there")) // gagged by the trigger
	if strings.Join(h.out, "|") != "triggered hello there" || len(h.shown) != 0 {
		t.Fatalf("gag: %v %v", h.out, h.shown)
	}
	// a named world must be open
	if call(t, in, "fake_recv", Str("x"), Str("nope")).Truth() {
		t.Fatal("closed world")
	}
	h.open["other"] = true
	call(t, in, "fake_recv", Str("plain"), Str("other"))
	if len(h.shown) != 1 || h.shown[0] != "other:plain" {
		t.Fatalf("LineHost: %v", h.shown)
	}
	if call(t, in, "fake_recv", Str("x"), Str(""), Str("q")).Truth() {
		t.Fatal("bad flag")
	}
	// raw: a byte stream with a trailing prompt
	h.shown = nil
	in.Exec("/def -hPROMPT hp = /echo prompt-hook %*")
	h.out = nil
	call(t, in, "fake_recv", Str("l1\r\nl2\nlogin: "), Str(""), Str("r"))
	if fmt.Sprint(h.shown) != "[w:l1 w:l2]" || strings.Join(h.out, "|") != "prompt-hook login: " {
		t.Fatalf("%v %v", h.shown, h.out)
	}
	if in.PromptText("") != "login: " || len(h.prompts) != 1 || h.prompts[0] != "w:login: " {
		t.Fatal(in.PromptText(""), h.prompts)
	}
	// prompt() sets the current world's prompt
	if !call(t, in, "prompt", Str("hp> ")).Truth() || in.PromptText("w") != "hp> " {
		t.Fatal("prompt()")
	}
	if len(h.kinds("prompt")) != 2 {
		t.Fatal(h.events)
	}
}

func TestOnVarChange(t *testing.T) {
	_, in := newKB(t)
	var got []string
	in.OnVarChange(func(n, o, nv string) { got = append(got, n+":"+o+">"+nv) })
	in.WatchVar("mything")
	in.PollVars() // baseline
	in.setGlobal("insert", Str("off"))
	in.setGlobal("mything", Str("a"))
	in.PollVars()
	in.setGlobal("mything", Str("b"))
	in.PollVars()
	in.PollVars() // no change, no event
	if strings.Join(got, ",") != "insert:on>off,mything:>a,mything:a>b" {
		t.Fatalf("%v", got)
	}
	// the patched setGlobal calls varChanged directly
	got = nil
	in.globals["wrap"] = Str("off")
	in.varChanged("wrap", "on", "off")
	if strings.Join(got, ",") != "wrap:on>off" {
		t.Fatalf("%v", got)
	}
	in.PollVars()
	if len(got) != 1 {
		t.Fatalf("a direct report must not be reported again by the poll: %v", got)
	}
}

func TestEntryPointsPollVars(t *testing.T) {
	h, in := newKB(t)
	var got []string
	in.OnVarChange(func(n, o, nv string) { got = append(got, n) })
	in.PollVars()
	in.ExecFull("/set visual=on")
	in.KeyPressed("a")
	if len(got) != 1 || got[0] != "visual" {
		t.Fatalf("KeyPressed must notice changes: %v", got)
	}
	if ev := h.kinds("var"); len(ev) != 1 || ev[0].Args[0] != "visual" {
		t.Fatal(h.events)
	}
}

func TestStatusRowsRendering(t *testing.T) {
	h, in := newKB(t)
	in.ExecFull("/set status_pad=.")
	in.ExecFull("/status_add -c @world :1 hp:5 :1 @clock:5")
	if got := in.globals["status_fields"].String(); got != "@world :1 hp:5 :1 @clock:5" {
		t.Fatal(got)
	}
	h.statusVal["world"] = "TinyMUX"
	h.statusVal["clock"] = "12:34"
	in.setGlobal("hp", Str("42"))
	rows := in.StatusRows(40)
	if len(rows) != 1 {
		t.Fatal(len(rows))
	}
	// 40 columns: @world takes 40-1-5-1-5 = 28
	want := "TinyMUX" + strings.Repeat(".", 21) + "." + "42" + "..." + "." + "12:34"
	if rows[0].Text != want || len(rows[0].Text) != 40 {
		t.Fatalf("\n got %q\nwant %q", rows[0].Text, want)
	}
	sp := rows[0].Spans
	if len(sp) != 5 || sp[0].Col != 0 || sp[0].Width != 28 || sp[2].Col != 29 || sp[2].Width != 5 || sp[4].Col != 35 {
		t.Fatalf("%+v", sp)
	}
	if sp[2].ValueStart != 0 || sp[2].ValueLen != 2 {
		t.Fatalf("%+v", sp[2])
	}
	// the width the client draws at changes the variable field
	if r := in.StatusRows(20)[0].Text; len(r) != 20 || !strings.HasPrefix(r, "TinyMUX") {
		t.Fatalf("%q", r)
	}
	// right-justified field and attributes
	in.ExecFull("/status_add -c lab:6:Br num:-6:Cred")
	in.setGlobal("lab", Str("ab"))
	in.setGlobal("num", Str("7"))
	in.setGlobal("status_attr", Str("r"))
	in.setGlobal("status_attr_var_num", Str("u"))
	in.setGlobal("status_pad", Str(" "))
	r := in.StatusRows(20)[0]
	if r.Text != "ab    "+"     7"+strings.Repeat(" ", 8) || r.Attrs != "r" || r.Spans[0].Attrs != "rB" {
		t.Fatalf("%q attrs %q %q", r.Text, r.Attrs, r.Spans[0].Attrs)
	}
	if !strings.Contains(r.Spans[1].Text, "     7") || r.Spans[1].ValueStart != 5 || r.Spans[1].Attrs != "urCred" {
		t.Fatalf("%+v", r.Spans[1])
	}
}

func TestStatusRowsFormatVariables(t *testing.T) {
	_, in := newKB(t)
	in.ExecFull("/status_add -c @more:8 :1 insert:6 :1 @log:5")
	in.setGlobal("status_int_more", Str(`moresize() == 0 ? "" : pad("More", 4, moresize(), 4)`))
	in.setGlobal("status_int_log", Str(`strcat("L", nmail())`))
	in.setGlobal("status_var_insert", Str(`insert ? "" : "(Over)"`))
	var warns []string
	in.Warn = func(s string) { warns = append(warns, s) }
	in.SetMoreState("", MoreState{Paused: true, Back: 12, New: 12})
	rows := in.StatusRows(40)
	if !strings.HasPrefix(rows[0].Text, "More  12_") {
		t.Fatalf("%q", rows[0].Text)
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "nmail()") {
		t.Fatalf("a problem is reported once: %v", warns)
	}
	in.StatusRows(40)
	if len(warns) != 1 {
		t.Fatalf("and not again on the next redraw: %v", warns)
	}
	in.setGlobal("status_int_log", Str(`strcat("M", nmail())`))
	in.StatusRows(40)
	if len(warns) != 2 {
		t.Fatalf("but again after the format changes: %v", warns)
	}
	warns = nil
	in.setGlobal("insert", Str("off"))
	rows = in.StatusRows(40)
	if !strings.Contains(rows[0].Text, "(Over)") {
		t.Fatalf("%q", rows[0].Text)
	}
	// an unset format variable shows the variable itself
	in.ExecFull("/status_add -c score:4")
	in.setGlobal("score", Str("99"))
	if in.StatusRows(10)[0].Spans[0].Text != "99__" {
		t.Fatalf("%+v", in.StatusRows(10)[0].Spans[0])
	}
}

func TestStatusHeightAndRows(t *testing.T) {
	h, in := newKB(t)
	in.ExecFull("/status_add -r0 a:3")
	in.ExecFull("/status_add -r1 b:3")
	if n := len(in.StatusRows(20)); n != 1 {
		t.Fatalf("only %%status_height rows render: %d", n)
	}
	in.setGlobal("status_height", Int(2))
	rows := in.StatusRows(20)
	if len(rows) != 2 || rows[1].Spans[0].Field.Name != "b" {
		t.Fatalf("%+v", rows)
	}
	in.setGlobal("status_height", Int(7))
	in.PollVars()
	if in.Var("status_height") != "2" {
		t.Fatalf("status_height above %d is refused: %s", maxStatusHeight, in.Var("status_height"))
	}
	if !strings.Contains(strings.Join(h.out, "|"), "status_height must be <= 6") {
		t.Fatal(h.out)
	}
	if got := call(t, in, "status_fields", Int(1)).String(); got != "b:3" {
		t.Fatal(got)
	}
	if got := call(t, in, "status_fields", Int(9)).String(); got != "" {
		t.Fatal(got)
	}
}

func TestStatusFieldsVariableDeprecatedPath(t *testing.T) {
	h, in := newKB(t)
	in.setGlobal("status_fields", Str("aa:3 :2 bb"))
	if got := call(t, in, "status_fields").String(); got != "aa:3 :2 bb" {
		t.Fatalf("%q", got)
	}
	if !strings.Contains(strings.Join(h.out, "|"), "setting status_fields directly is deprecated") {
		t.Fatalf("warning: %v", h.out)
	}
	h.out = nil
	in.setGlobal("warn_status", Str("off"))
	in.setGlobal("status_fields", Str("cc:4"))
	call(t, in, "status_fields")
	if len(h.out) != 0 || len(in.StatusFields(0)) != 1 {
		t.Fatalf("%v", h.out)
	}
	// an invalid value is rejected and the old value restored
	in.setGlobal("status_fields", Str("@bogus"))
	call(t, in, "status_fields")
	if in.Var("status_fields") != "cc:4" || !strings.Contains(strings.Join(h.out, "|"), "Invalid status_fields internal status") {
		t.Fatalf("%q %v", in.Var("status_fields"), h.out)
	}
	// and the other direction: /status_add updates the variable
	in.ExecFull("/status_add dd:2")
	if in.Var("status_fields") != "cc:4 :1 dd:2" {
		t.Fatal(in.Var("status_fields"))
	}
}

func TestStatusCommandsReturnValues(t *testing.T) {
	_, in := newKB(t)
	in.Warn = func(string) {}
	for _, c := range []struct {
		cmd  string
		want bool
	}{
		{"/status_add -c a:3 b:4", true},
		{"/status_add -Anone zz", false},
		{"/status_add -x a:9", true},
		{"/status_rm nosuch", false},
		{"/status_rm a", true},
		{"/status_edit nosuch:3", false},
		{"/status_edit b:7:B", true},
		{"/status_clear -r0", true},
		{"/status_clear", false},
		{"/status_add @bogus", false},
		{"/status_add -c x:1 y z", false},
		{"/status_add -c x:1 y:0", true},
		{"/status_add -c x:3:Zz", false},
		{"/status_add -q", false},
	} {
		if got := in.ExecFull(c.cmd).Truth(); got != c.want {
			t.Errorf("%s = %v, want %v", c.cmd, got, c.want)
		}
	}
	if got := call(t, in, "status_width", Str("x")).Int(); got != 1 {
		t.Fatal(got)
	}
	if got := call(t, in, "status_width", Str("y")).Int(); got != 79 {
		t.Fatalf("variable-width field takes the rest of the 80 columns, got %d", got)
	}
	// real tf dereferences NULL here; we answer 0 with a diagnostic
	if got := call(t, in, "status_width", Str("nosuch")).Int(); got != 0 {
		t.Fatal(got)
	}
}

func TestStatusEventsAndHostValueOverride(t *testing.T) {
	h, in := newKB(t)
	in.ExecFull("/status_add -c @world:6")
	if ev := h.kinds("status"); len(ev) == 0 {
		t.Fatal("status changes must notify the client")
	}
	in.setGlobal("status_int_world", Str(`"expr"`))
	if r := in.StatusRows(10)[0].Spans[0].Text; r != "expr__" {
		t.Fatal(r)
	}
	h.statusVal["world"] = "host"
	if r := in.StatusRows(10)[0].Spans[0].Text; r != "host__" {
		t.Fatalf("the host's value wins: %s", r)
	}
}

func TestInterpsAreIndependent(t *testing.T) {
	_, a := newKB(t)
	_, b := newKB(t)
	a.Input().Set("aaa", -1)
	b.Input().Set("bbbbb", -1)
	a.ExecFull("/status_add -c x:3")
	if a.Input().Text() != "aaa" || b.Input().Text() != "bbbbb" || b.statusRowText(0) != "" {
		t.Fatal("state leaked between interpreters")
	}
}

func TestTerminalStateIsCollected(t *testing.T) {
	has := func(k weak.Pointer[Interp]) bool {
		termMu.Lock()
		defer termMu.Unlock()
		_, ok := termStates[k]
		return ok
	}
	var key weak.Pointer[Interp]
	func() {
		in := New(newKHost())
		in.Input().Set("x", -1)
		key = weak.Make(in)
		if !has(key) {
			t.Fatal("state not registered")
		}
	}()
	for i := 0; i < 20 && has(key); i++ {
		runtime.GC()
		runtime.Gosched()
	}
	if has(key) {
		t.Fatal("state of a dead Interp must be released")
	}
	in := New(newKHost())
	in.Input().Set("x", -1)
	k2 := weak.Make(in)
	in.ReleaseTerminal()
	if has(k2) {
		t.Fatal("ReleaseTerminal")
	}
	runtime.KeepAlive(in)
}

// wHost has a world but no LineHost: displayed lines go to Echo.
type wHost struct{ tHost }

func (h *wHost) Connect(string) error    { return nil }
func (h *wHost) Disconnect(string) error { return nil }
func (h *wHost) Foreground(string) error { return nil }
func (h *wHost) ForegroundWorld() string { return "w" }
func (h *wHost) Connected(w string) bool { return w == "w" }

func TestFakeRecvFallsBackToEcho(t *testing.T) {
	h := &wHost{}
	in := New(h)
	in.Exec("/def -t'a *' = /echo trig")
	v, err := in.callFunc("fake_recv", []Value{Str("a b")})
	if err != nil || !v.Truth() || strings.Join(h.out, "|") != "trig|a b" {
		t.Fatalf("%v %v %v", v, err, h.out)
	}
}

func TestConcurrentInterps(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			h := newKHost()
			in := New(h)
			in.Warn = func(string) {}
			in.Exec("/def hi = /echo %1")
			for j := 0; j < 50; j++ {
				in.KeyPressed(fmt.Sprintf("/hi %d-%d\r", i, j))
				in.ExecFull("/status_add -c a:3")
				_ = in.StatusRows(30)
			}
			if len(h.out) != 50 {
				t.Errorf("worker %d: %d lines", i, len(h.out))
			}
		}(i)
	}
	wg.Wait()
}
