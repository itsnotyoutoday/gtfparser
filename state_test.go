package gtfparser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// stHost is a fake client implementing every optional capability the state
// builtins use.
type stHost struct {
	out, warns []string
	events     []Event
	socks      []SocketInfo
	fg         string
	connected  map[string]bool
	connects   []string
	connOpts   []ConnectOptions
	discs      []string
	fgs        []string
	kbd        time.Time
	helps      []string
	reboots    []bool
	visible    int // what Redraw reports
	redraws    []string
}

func (h *stHost) Echo(s string)         { h.out = append(h.out, s) }
func (h *stHost) Send(w, s string) bool { return true }
func (h *stHost) Notify(e Event)        { h.events = append(h.events, e) }
func (h *stHost) Connect(w string) error {
	h.connects = append(h.connects, w)
	return nil
}
func (h *stHost) ConnectWith(w string, o ConnectOptions) error {
	h.connects = append(h.connects, w)
	h.connOpts = append(h.connOpts, o)
	return nil
}
func (h *stHost) Disconnect(w string) error { h.discs = append(h.discs, w); return nil }
func (h *stHost) Foreground(w string) error { h.fgs = append(h.fgs, w); h.fg = w; return nil }
func (h *stHost) ForegroundWorld() string   { return h.fg }
func (h *stHost) Connected(w string) bool   { return h.connected[w] }
func (h *stHost) Sockets() []SocketInfo     { return h.socks }
func (h *stHost) LastKeyboard() time.Time   { return h.kbd }
func (h *stHost) Help(t string)             { h.helps = append(h.helps, t) }
func (h *stHost) Reboot(f bool) error       { h.reboots = append(h.reboots, f); return nil }
func (h *stHost) Redraw(w string) int       { h.redraws = append(h.redraws, w); return h.visible }

func (h *stHost) kinds() string {
	var k []string
	for _, e := range h.events {
		k = append(k, e.Kind+":"+strings.Join(e.Args, ","))
	}
	return strings.Join(k, " ")
}

func newST(t *testing.T, opts ...Option) (*Interp, *stHost) {
	t.Helper()
	h := &stHost{connected: map[string]bool{}, visible: 1}
	in := New(h, opts...)
	in.Warn = func(s string) { h.warns = append(h.warns, s) }
	in.Now = func() time.Time { return time.Date(2024, 3, 5, 14, 7, 9, 0, time.UTC) }
	return in, h
}

func (h *stHost) take() []string { o := h.out; h.out = nil; return o }

func TestHistoryFeedAndExport(t *testing.T) {
	in, h := newST(t)
	in.ExecFull(`/test addworld("w1", "", "", "")`)
	in.RecordWorldLine("w1", "hello")
	in.RecordWorldLine("w1", "world")
	in.RecordLocalLine("local one")
	in.RecordInput("look")
	in.RecordInput("look") // consecutive duplicate collapses
	in.RecordInput("get all")
	if got := in.HistoryLines(HistWorld, "w1"); strings.Join(got, "|") != "hello|world" {
		t.Fatalf("world history: %v", got)
	}
	if got := in.HistoryLines(HistGlobal, ""); strings.Join(got, "|") != "hello|world|local one" {
		t.Fatalf("global history: %v", got)
	}
	if got := in.HistoryLines(HistLocal, ""); strings.Join(got, "|") != "local one" {
		t.Fatalf("local history: %v", got)
	}
	// the input history always ends with the blank line being typed
	if got := in.HistoryLines(HistInput, ""); strings.Join(got, "|") != "look|get all|" {
		t.Fatalf("input history: %q", got)
	}
	ls, first := in.History(HistWorld, "w1")
	if first != 1 || len(ls) != 2 || !ls[0].Time.Equal(in.Now()) {
		t.Fatalf("History: %v %d", ls, first)
	}
	h.take()
	in.ExecFull("/recall -ww1 -q 1-")
	if got := strings.Join(h.take(), "|"); got != "hello|world" {
		t.Fatalf("recall: %q", got)
	}
	// header and footer appear when the output goes to the screen
	in.ExecFull("/recall -ww1 /1")
	if got := h.take(); len(got) != 3 || !strings.Contains(got[0], "Recall start") || got[1] != "world" || !strings.Contains(got[2], "Recall end") {
		t.Fatalf("recall framing: %q", got)
	}
	// current world is the default history
	in.SetCurrentWorld("w1")
	in.ExecFull("/recall -q 1-")
	if got := strings.Join(h.take(), "|"); got != "hello|world" {
		t.Fatalf("recall (current world): %q", got)
	}
	in.SetCurrentWorld("")
	in.ExecFull("/recall -q #1-")
	if got := strings.Join(h.take(), "|"); got != "1: hello|2: world|3: local one" {
		t.Fatalf("recall (global): %q", got)
	}
}

func TestHistoryCapacityAndNumbering(t *testing.T) {
	in, h := newST(t)
	in.ExecFull("/set histsize=3")
	in.ExecFull(`/test addworld("w", "", "", "")`)
	for i := 1; i <= 5; i++ {
		in.RecordWorldLine("w", fmt.Sprint("l", i))
	}
	ls, first := in.History(HistWorld, "w")
	if len(ls) != 3 || first != 3 || ls[0].Text != "l3" {
		t.Fatalf("capacity: %v first=%d", ls, first)
	}
	in.ExecFull("/recall -ww -q #1-")
	if got := strings.Join(h.take(), "|"); got != "3: l3|4: l4|5: l5" {
		t.Fatalf("numbers must survive trimming: %q", got)
	}
	in.ExecFull("/histsize -ww 2")
	if got := in.HistoryLines(HistWorld, "w"); strings.Join(got, "|") != "l4|l5" {
		t.Fatalf("shrink: %v", got)
	}
	if out := strings.Join(h.take(), "|"); out != "% world history capacity changed to 2 lines." {
		t.Fatalf("histsize: %q", out)
	}
	// an unallocated world history reports (and later takes) %histsize
	in.ExecFull(`/test addworld("w2", "", "", "")`)
	in.ExecFull("/histsize -ww2")
	if out := strings.Join(h.take(), "|"); out != "% world history capacity is 3 lines." {
		t.Fatalf("histsize default: %q", out)
	}
}

func TestRecordlineAttrsAndTime(t *testing.T) {
	in, h := newST(t)
	in.ExecFull(`/test addworld("w", "", "", "")`)
	in.ExecFull("/recordline -ww -t1000000000 old")
	in.ExecFull("/recordline -ww -ag hidden")
	in.ExecFull("/recordline -ww -aG nothist")
	ls, _ := in.History(HistWorld, "w")
	if len(ls) != 2 || ls[0].Time.Unix() != 1000000000 || ls[1].Attrs != "g" {
		t.Fatalf("recordline: %+v", ls)
	}
	in.ExecFull("/recall -ww -q 1-")
	if got := strings.Join(h.take(), "|"); got != "old" {
		t.Fatalf("gagged lines are skipped: %q", got)
	}
	in.ExecFull("/recall -ww -ag -q 1-")
	if got := strings.Join(h.take(), "|"); got != "old|hidden" {
		t.Fatalf("-ag shows them: %q", got)
	}
	if v := in.ExecFull("/recordline -ww -a@ x"); v.Truth() {
		t.Fatal("bad attribute must fail")
	}
	if len(h.warns) == 0 || !strings.Contains(h.warns[len(h.warns)-1], "invalid display attribute") {
		t.Fatalf("warns: %v", h.warns)
	}
}

func TestRecallInputStepping(t *testing.T) {
	in, _ := newST(t)
	in.RecordInput("one")
	in.RecordInput("two")
	in.RecordInput("three")
	l, ok := in.RecallInput(-1, 0, "typing")
	if !ok || l != "three" {
		t.Fatalf("back 1: %q %v", l, ok)
	}
	l, _ = in.RecallInput(-1, 0, "typing")
	if l != "two" {
		t.Fatalf("back 2: %q", l)
	}
	l, _ = in.RecallInput(1, 0, "")
	l, _ = in.RecallInput(1, 0, "")
	if l != "typing" {
		t.Fatalf("forward restores the line being typed: %q", l)
	}
	if _, ok := in.RecallInput(1, 0, "typing"); ok {
		t.Fatal("no step past the end")
	}
}

func TestLogFile(t *testing.T) {
	dir := t.TempDir()
	in, h := newST(t)
	in.ExecFull(`/test addworld("w", "", "", "")`)
	in.SetVar("log_prefix", "[%t]%x ") // %t is the time; other % pairs are copied
	in.SetVar("log_time_format", "%H:%M")
	gl, wl := filepath.Join(dir, "g.log"), filepath.Join(dir, "w.log")
	if v := in.ExecFull("/log -g " + gl); !v.Truth() {
		t.Fatalf("log: %v", h.warns)
	}
	in.ExecFull("/log -ww " + wl)
	if in.LogCount() != 2 {
		t.Fatalf("nlog: %d", in.LogCount())
	}
	in.RecordWorldLine("w", "from the world")
	in.RecordLocalLine("from tf")
	in.RecordLocalLineAttr("quiet", "L") // L: never logged
	in.RecordWorldLine("nosuchworld", "x")
	g, _ := os.ReadFile(gl)
	w, _ := os.ReadFile(wl)
	if string(g) != "[14:07]%x from the world\n[14:07]%x from tf\n[14:07]%x x\n" {
		t.Fatalf("global log: %q", g)
	}
	if string(w) != "[14:07]%x from the world\n" {
		t.Fatalf("world log: %q", w)
	}
	if !in.LogLine("", "direct") {
		t.Fatal("LogLine")
	}
	if in.LogLine("w2", "no") {
		t.Fatal("LogLine to a world without a log must say so")
	}
	h.take()
	in.ExecFull("/log")
	if got := strings.Join(h.take(), "|"); got != "% Logging global output to "+gl+"|% Logging world w output to "+wl {
		t.Fatalf("listing: %q", got)
	}
	if !strings.Contains(h.kinds(), "log:on,"+gl) {
		t.Fatalf("events: %s", h.kinds())
	}
	in.ExecFull("/log -ww OFF")
	in.ExecFull("/log OFF")
	if in.LogCount() != 0 {
		t.Fatal("OFF")
	}
	in.ExecFull("/log")
	if got := strings.Join(h.take(), "|"); got != "% Logging disabled." {
		t.Fatalf("after off: %q", got)
	}
	in.RecordLocalLine("after")
	g2, _ := os.ReadFile(gl)
	if !strings.HasSuffix(string(g2), "direct\n") {
		t.Fatalf("closed log must not grow: %q", g2)
	}
}

func TestLogWrapAndAnsi(t *testing.T) {
	dir := t.TempDir()
	in, _ := newST(t)
	in.ExecFull("/set log_prefix=")
	in.ExecFull("/set wraplog=on")
	in.ExecFull("/set wrapsize=10")
	in.ExecFull("/set wrapspace=2")
	f := filepath.Join(dir, "x.log")
	in.ExecFull("/log " + f)
	in.RecordLocalLine("aaa bbb ccc ddd eee")
	b, _ := os.ReadFile(f)
	if string(b) != "aaa bbb \n  ccc ddd \n  eee\n" {
		t.Fatalf("wrapped: %q", b)
	}
	in.ExecFull("/set wraplog=off")
	in.ExecFull("/set ansi_log=on")
	in.RecordLocalLineAttr("red", "Cred")
	in.RecordLocalLineAttr("bold", "B")
	b, _ = os.ReadFile(f)
	if !strings.HasSuffix(string(b), "\x1b[31mred\x1b[0m\n\x1b[1mbold\x1b[0m\n") {
		t.Fatalf("ansi: %q", b)
	}
}

func TestLogPolicyAndRestrict(t *testing.T) {
	dir := t.TempDir()
	in, h := newST(t, WithPolicy(denyAll{}))
	if v := in.ExecFull("/log " + filepath.Join(dir, "no.log")); v.Truth() {
		t.Fatal("policy must veto /log")
	}
	if _, err := os.Stat(filepath.Join(dir, "no.log")); err == nil {
		t.Fatal("file created despite policy")
	}
	if v := in.ExecFull("/saveworld " + filepath.Join(dir, "w.tf")); v.Truth() {
		t.Fatal("policy must veto /saveworld")
	}
	if v := in.ExecFull("/save " + filepath.Join(dir, "m.tf")); v.Truth() {
		t.Fatal("policy must veto /save")
	}
	if len(h.warns) != 3 {
		t.Fatalf("warns: %v", h.warns)
	}
	in2, h2 := newST(t, WithRestriction(RestrictFile))
	in2.ExecFull("/log")
	in2.ExecFull("/saveworld x")
	in2.ExecFull("/save x")
	if len(h2.warns) != 3 || !strings.Contains(h2.warns[0], "restricted") {
		t.Fatalf("restricted: %v", h2.warns)
	}
}

func TestLimit(t *testing.T) {
	in, h := newST(t)
	h.fg = "w"
	h.visible = 7
	if v := in.ExecFull("/limit"); v.Truth() {
		t.Fatal("no limit yet")
	}
	if v := in.ExecFull("/limit *orc*"); !v.Truth() {
		t.Fatalf("limit: %v", h.warns)
	}
	if !in.LimitActive() || !in.Limited("a goblin") || in.Limited("an orc!") {
		t.Fatal("filter semantics")
	}
	if !strings.Contains(h.kinds(), "limit:w,on") || len(h.redraws) != 1 || h.redraws[0] != "w" {
		t.Fatalf("event/redraw: %s %v", h.kinds(), h.redraws)
	}
	if v := in.ExecFull("/limit"); !v.Truth() {
		t.Fatal("limit() reports a limit in effect")
	}
	in.ExecFull("/limit -v *orc*")
	if in.Limited("a goblin") || !in.Limited("an orc!") {
		t.Fatal("-v inverts")
	}
	in.ExecFull("/limit -mregexp ^g")
	if in.Limited("goblin") || !in.Limited("orc") {
		t.Fatal("-m regexp")
	}
	in.ExecFull("/limit -a")
	if !in.LimitedFor("w", "x", "") || in.LimitedFor("w", "x", "B") {
		t.Fatal("-a keeps lines with attributes")
	}
	in.ExecFull("/unlimit")
	if in.LimitActive() || in.Limited("anything") {
		t.Fatal("unlimit")
	}
	if v := in.ExecFull("/unlimit"); v.Truth() {
		t.Fatal("second unlimit returns 0")
	}
	in.ExecFull("/relimit")
	if !in.LimitActive() {
		t.Fatal("relimit restores the last limit")
	}
	// per world: another screen is unaffected
	h.fg = "other"
	if in.LimitActive() {
		t.Fatal("limits are per screen")
	}
	// the client finds nothing to show
	h.fg = "w"
	in.ExecFull("/unlimit")
	h.visible = 0
	h.warns = nil
	if v := in.ExecFull("/limit nomatch"); v.Truth() || in.LimitActive() {
		t.Fatal("a limit with no visible lines is reverted")
	}
	if len(h.warns) != 1 || h.warns[0] != "No lines matched criteria." {
		t.Fatalf("warns: %v", h.warns)
	}
}

func TestWorldDatabase(t *testing.T) {
	in, h := newST(t)
	in.ExecFull(`/test addworld("foo", "tiny", "h.example", "4000", "bob", "pw", "mf", "xe", "src")`)
	w, ok := in.World("FOO") // names are case-insensitive
	if !ok || w.Host != "h.example" || w.Port != "4000" || w.Character != "bob" || w.Password != "pw" || w.MFile != "mf" || w.SrcHost != "src" || !w.SSL || !w.Echo || w.NoProxy {
		t.Fatalf("world: %+v", w)
	}
	if !in.DefineWorld(WorldInfo{Name: "bar", Type: "lp", Host: "b", Port: "1", NoProxy: true}) {
		t.Fatal("DefineWorld")
	}
	if in.DefineWorld(WorldInfo{Name: "bad name"}) || in.DefineWorld(WorldInfo{Name: "half", Host: "x"}) {
		t.Fatal("invalid definitions must be refused")
	}
	if len(in.Worlds()) != 2 {
		t.Fatalf("worlds: %v", in.Worlds())
	}
	// ${world_*} follow the current world
	in.SetCurrentWorld("foo")
	if in.Var("world_name") != "foo" || in.Var("world_character") != "bob" || in.Var("world_host") != "h.example" {
		t.Fatalf("world vars: %q %q", in.Var("world_name"), in.Var("world_host"))
	}
	in.ExecFull("/echo %{world_name}:%{world_type}")
	if got := h.out[len(h.out)-1]; got != "foo:tiny" {
		t.Fatalf("subst: %q", got)
	}
	in.ExecFull(`/test addworld("foo", "tiny2")`) // redefinition refreshes them
	if in.Var("world_type") != "tiny2" {
		t.Fatal("world vars not refreshed on redefinition")
	}
	in.ExecFull("/renameworld foo foo2")
	if in.CurrentWorld() != "foo2" || in.Var("world_name") != "foo2" {
		t.Fatal("rename of the current world")
	}
	if !strings.Contains(h.kinds(), "renameworld:foo,foo2") {
		t.Fatalf("events: %s", h.kinds())
	}
	in.ExecFull("/unworld foo2")
	if in.CurrentWorld() != "" || in.Var("world_name") != "" {
		t.Fatal("removing the current world clears it")
	}
	if _, ok := in.World("foo2"); ok {
		t.Fatal("unworld")
	}
	if !in.RemoveWorld("bar") || in.RemoveWorld("bar") {
		t.Fatal("RemoveWorld")
	}
}

func TestWorldInfoFunction(t *testing.T) {
	in, h := newST(t)
	in.ExecFull(`/test addworld("foo", "tiny", "h", "1", "bob", "pw", "mf", "", "src")`)
	check := func(expr, want string) {
		t.Helper()
		h.out = nil
		in.ExecFull("/echo $[" + expr + "]")
		if len(h.out) != 1 || h.out[0] != want {
			t.Fatalf("%s = %q, want %q (%v)", expr, h.out, want, h.warns)
		}
	}
	check(`world_info("foo", "character")`, "bob")
	check(`world_info("foo", "src")`, "src")
	check(`world_info("nosuch", "name")`, "")
	check(`world_info("name")`, "")
	in.SetCurrentWorld("foo")
	check(`world_info("name")`, "foo")
	check(`world_info()`, "foo")
	check(`world_info("host")`, "h")
	// the current world's host and port are those of the connection
	h.socks = []SocketInfo{{World: "foo", Host: "10.0.0.1", Port: "9", State: SockConnected, Login: true, Cipher: "AES"}}
	check(`world_info("host")`, "10.0.0.1")
	check(`world_info("foo", "host")`, "h")
	check(`world_info("login")`, "1")
	check(`world_info("cipher")`, "AES")
	check(`world_info("proxy")`, "0")
	h.warns = nil
	check(`world_info("bogus")`, "")
	if len(h.warns) != 1 || !strings.Contains(h.warns[0], "illegal field name") {
		t.Fatalf("warns: %v", h.warns)
	}
}

func TestConnectCommands(t *testing.T) {
	in, h := newST(t)
	in.ExecFull(`/test addworld("foo", "", "h", "4000")`)
	h.socks = nil
	if v := in.ExecFull("/connect foo"); !v.Truth() || strings.Join(h.connects, ",") != "foo" {
		t.Fatalf("connect: %v %v", h.connects, h.warns)
	}
	// "/connect host port" makes a temporary world the client can read
	in.ExecFull("/connect -qx example.org 23")
	if len(h.connects) != 2 || h.connects[1] != "(unnamed1)" {
		t.Fatalf("temp world: %v", h.connects)
	}
	o := h.connOpts[len(h.connOpts)-1]
	if !o.Quiet || !o.SSL || o.NoLogin {
		t.Fatalf("options: %+v", o)
	}
	w, ok := in.World("(unnamed1)")
	if !ok || !w.Temp || w.Host != "example.org" || w.Port != "23" || !w.SSL {
		t.Fatalf("temp world: %+v", w)
	}
	in.ExecFull("/listworlds -s")
	if strings.Contains(strings.Join(h.out, "|"), "unnamed") {
		t.Fatal("temporary worlds are not listed")
	}
	h.out = nil
	in.ExecFull("/listworlds -u -s")
	if !strings.Contains(strings.Join(h.out, "|"), "(unnamed1)") {
		t.Fatalf("-u lists them: %v", h.out)
	}
	in.WorldClosed("(unnamed1)")
	if _, ok := in.World("(unnamed1)"); ok {
		t.Fatal("a closed temporary world is forgotten")
	}
	// renaming keeps it
	in.ExecFull("/connect example.org 24")
	in.ExecFull("/renameworld (unnamed2) keeper")
	in.WorldClosed("keeper")
	if w, ok := in.World("keeper"); !ok || w.Temp {
		t.Fatal("renamed temporary worlds become permanent")
	}
	if v := in.ExecFull("/connect nosuch"); v.Truth() || !strings.Contains(h.warns[len(h.warns)-1], "no such world") {
		t.Fatalf("unknown world: %v", h.warns)
	}
	// restriction: arbitrary connections are refused
	in2, h2 := newST(t, WithRestriction(RestrictWorld))
	if v := in2.ExecFull("/connect example.org 23"); v.Truth() || len(h2.connects) != 0 {
		t.Fatalf("restricted connect: %v", h2.warns)
	}
	// a host that cannot connect says so
	bare := &tHost{}
	in3 := New(bare)
	var w3 []string
	in3.Warn = func(s string) { w3 = append(w3, s) }
	in3.ExecFull(`/test addworld("foo", "", "h", "1")`)
	if v := in3.ExecFull("/connect foo"); v.Truth() || len(w3) == 0 || !strings.Contains(w3[0], "not supported by this client") {
		t.Fatalf("unsupported: %v", w3)
	}
}

func TestDisconnectForegroundAndFunctions(t *testing.T) {
	in, h := newST(t)
	in.ExecFull(`/test addworld("a", "", "h", "1")`)
	in.ExecFull(`/test addworld("b", "", "h", "2")`)
	h.socks = []SocketInfo{
		{World: "a", Host: "h", Port: "1", Addr: "10.0.0.1", State: SockConnected, NewLines: 3, Active: true, LastRecv: in.Now().Add(-90 * time.Second), LastSend: in.Now().Add(-5 * time.Second)},
		{World: "b", Host: "h", Port: "2", Addr: "10.0.0.2", State: SockConnected, OldLines: 4, NewLines: 12, Compressed: true, Proxy: true, LastRecv: in.Now().Add(-2 * time.Hour)},
		{World: "dead", State: SockDead},
	}
	h.fg = "a"
	in.SetCurrentWorld("a")
	exp := func(e, want string) {
		t.Helper()
		h.out = nil
		in.ExecFull("/echo $[" + e + "]")
		if len(h.out) != 1 || h.out[0] != want {
			t.Fatalf("%s = %q want %q", e, h.out, want)
		}
	}
	exp("fg_world()", "a")
	exp("is_connected()", "1")
	exp("is_connected(\"b\")", "1")
	exp("is_connected(\"dead\")", "0")
	exp("is_open(\"b\")", "1")
	exp("is_open(\"dead\")", "0")
	exp("is_open(\"nosuch\")", "0")
	exp("nactive()", "1")
	exp("nactive(\"a\")", "3")
	exp("nactive(\"b\")", "0")
	exp("idle(\"a\")", "90.0")
	exp("sidle(\"a\")", "5.0")
	exp("sidle()", "5.0")
	exp("idle(\"nosuch\")", "0.0")
	h.kbd = in.Now().Add(-1500 * time.Millisecond)
	exp("idle()", "1.5")
	// /listsockets
	h.out = nil
	in.ExecFull("/listsockets")
	want := []string{
		"       LINES IDLE TYPE        NAME            HOST                       PORT",
		"*C         fg   1m             a               h                          1",
		" Cp+  4+  12   2h             b               h                          2",
		" !           0s                                                         ",
	}
	_ = want
	if len(h.out) != 4 || !strings.HasPrefix(h.out[0], "    ") || !strings.Contains(h.out[0], "LINES IDLE TYPE") {
		t.Fatalf("listsockets: %q", h.out)
	}
	if !strings.HasPrefix(h.out[1], "*C  ") || !strings.Contains(h.out[1], "fg   1m") || !strings.HasSuffix(h.out[1], "h                          1") {
		t.Fatalf("row a: %q", h.out[1])
	}
	if !strings.HasPrefix(h.out[2], " cP") || !strings.Contains(h.out[2], "  4+  12   2h") {
		t.Fatalf("row b (compressed, proxy, old+new lines): %q", h.out[2])
	}
	h.out = nil
	in.ExecFull("/listsockets -s b*")
	if strings.Join(h.out, "|") != "b" {
		t.Fatalf("-s: %q", h.out)
	}
	// /dc and /fg
	in.ExecFull("/dc")
	in.ExecFull("/dc b")
	if strings.Join(h.discs, ",") != "a,b" {
		t.Fatalf("dc: %v", h.discs)
	}
	if v := in.ExecFull("/dc dead"); v.Truth() {
		t.Fatal("dc on a dead socket")
	}
	in.ExecFull("/dc -all")
	if strings.Join(h.discs, ",") != "a,b,a,b" {
		t.Fatalf("dc -all: %v", h.discs)
	}
	if v := in.ExecFull("/fg b"); v.Int() != 1 || h.fg != "b" {
		t.Fatalf("fg: %v %s", v, h.fg)
	}
	if v := in.ExecFull("/fg b"); v.Int() != 2 {
		t.Fatalf("already in the foreground returns 2: %v", v)
	}
	in.ExecFull("/fg -<")
	if h.fg != "a" {
		t.Fatalf("fg -<: %s", h.fg)
	}
	in.ExecFull("/fg ->")
	if h.fg != "b" {
		t.Fatalf("fg ->: %s", h.fg)
	}
	in.ExecFull("/fg -n")
	if h.fg != "" {
		t.Fatalf("fg -n: %q", h.fg)
	}
	in.ExecFull("/fg")
	if h.fg != "a" {
		t.Fatalf("fg with no argument selects the first socket: %q", h.fg)
	}
	if v := in.ExecFull("/fg nosuch"); v.Truth() {
		t.Fatal("fg nosuch")
	}
}

func TestUnworldRefusesWorldInUse(t *testing.T) {
	in, h := newST(t)
	in.ExecFull(`/test addworld("a", "", "h", "1")`)
	h.socks = []SocketInfo{{World: "a", State: SockConnected}}
	if v := in.ExecFull("/unworld a"); v.Truth() {
		t.Fatal("a world with a socket cannot be removed")
	}
	if _, ok := in.World("a"); !ok {
		t.Fatal("must still exist")
	}
	h.socks = nil
	in.ExecFull("/unworld a")
	if _, ok := in.World("a"); ok {
		t.Fatal("removed once closed")
	}
}

func TestWatchdogAndWatchname(t *testing.T) {
	in, h := newST(t)
	in.ExecFull(`/test addworld("w", "", "", "")`)
	in.ExecFull("/watchdog 2 4")
	h.take()
	feed := func(l string) bool {
		g := in.CheckWatch("w", l)
		in.RecordWorldLine("w", l)
		return g
	}
	// a line seen 2 times among the last 4 (not counting the previous one) is gagged
	got := []bool{feed("spam"), feed("x"), feed("spam"), feed("y"), feed("spam")}
	if fmt.Sprint(got) != "[false false false false true]" {
		t.Fatalf("watchdog: %v", got)
	}
	in.ExecFull("/watchdog off")
	if feed("spam") || feed("spam") {
		t.Fatal("watchdog off")
	}
	// watchname defines a gag macro for a name that starts too many lines
	in.ExecFull("/watchname 2 3")
	h.take()
	var gagged []bool
	for _, l := range []string{"Bob says hi", "Ann waves", "Bob says bye", "Bob says again"} {
		gagged = append(gagged, feed(l))
	}
	if fmt.Sprint(gagged) != "[false false true true]" {
		t.Fatalf("watchname: %v", gagged)
	}
	found := false
	for _, m := range in.Macros() {
		if m.Trigger == "{Bob}*" && m.Gag {
			found = true
		}
	}
	if !found {
		t.Fatal("watchname must define a gag macro")
	}
	if got := h.take(); len(got) == 0 || !strings.Contains(got[0], `Watchname: gagging "{Bob}*"`) {
		t.Fatalf("message: %v", got)
	}
}

func TestFeatures(t *testing.T) {
	in, h := newST(t)
	in.ExecFull("/features")
	if got := h.take(); len(got) != 1 || got[0] != "-256colors -core +float +ftime +history -IPv6 +locale -MCCPv1 -MCCPv2 +process -SOCKS -ssl +subsecond +TZ" {
		t.Fatalf("features: %v", got)
	}
	in.SetFeature("ssl", true)
	in.SetFeature("256colors", true)
	if v := in.ExecFull("/features ssl"); !v.Truth() {
		t.Fatal("ssl declared")
	}
	if v := in.ExecFull("/features SOCKS"); v.Truth() {
		t.Fatal("SOCKS not declared")
	}
	if v := in.ExecFull("/features nosuch"); v.Truth() {
		t.Fatal("unknown feature")
	}
	if !strings.HasPrefix(in.FeatureString(), "+256colors ") {
		t.Fatal(in.FeatureString())
	}
}

func TestHelpRebootCore(t *testing.T) {
	in, h := newST(t)
	in.ExecFull("/help")
	in.ExecFull("/help worlds")
	if strings.Join(h.helps, ",") != "summary,worlds" {
		t.Fatalf("help: %v", h.helps)
	}
	in.ExecFull("/reboot -y")
	in.ExecFull("/reboot")
	if fmt.Sprint(h.reboots) != "[true false]" {
		t.Fatalf("reboot: %v", h.reboots)
	}
	if v := in.ExecFull("/reboot now"); v.Truth() {
		t.Fatal("reboot takes no argument")
	}
	in.ExecFull("/core boom")
	if !strings.Contains(h.kinds(), "core:boom") || !strings.Contains(h.kinds(), "reboot:1") {
		t.Fatalf("events: %s", h.kinds())
	}
	// without the capabilities they fail loudly
	bare := &tHost{}
	in2 := New(bare)
	var w []string
	in2.Warn = func(s string) { w = append(w, s) }
	for _, c := range []string{"/help", "/reboot", "/core"} {
		if v := in2.ExecFull(c); v.Truth() {
			t.Fatalf("%s must fail", c)
		}
	}
	if len(w) != 3 {
		t.Fatalf("warns: %v", w)
	}
}

func TestSaveEditUndefnStreams(t *testing.T) {
	dir := t.TempDir()
	in, h := newST(t)
	in.Exec("/def foo = /echo one")
	in.Exec("/def -t'abc*' bar = /echo two")
	f := filepath.Join(dir, "m.tf")
	if v := in.ExecFull("/save " + f + " foo"); !v.Truth() {
		t.Fatalf("save: %v", h.warns)
	}
	b, _ := os.ReadFile(f)
	if string(b) != "/def foo = /echo one\n" {
		t.Fatalf("saved: %q", b)
	}
	in.ExecFull("/save -a " + f + " -t* bar")
	b, _ = os.ReadFile(f)
	if string(b) != "/def foo = /echo one\n/def -p1 -mglob -t'abc*' bar = /echo two\n" {
		t.Fatalf("appended: %q", b)
	}
	// the saved file loads back
	in2, _ := newST(t)
	if err := in2.LoadFile(f); err != nil {
		t.Fatal(err)
	}
	if len(in2.Macros()) < 2 {
		t.Fatal("round trip")
	}
	// /edit keeps the number and replaces only what is given
	id := in.macros["bar"].ID
	if v := in.ExecFull("/edit -p5 bar = /echo changed"); v.Int() != int64(id) {
		t.Fatalf("edit returns the macro number: %v (%v)", v, h.warns)
	}
	m := in.macros["bar"]
	if m.ID != id || m.Priority != 5 || m.Trigger != "abc*" || m.Body != "/echo changed" {
		t.Fatalf("edited: %+v", m)
	}
	if v := in.ExecFull("/edit nosuch = x"); v.Truth() {
		t.Fatal("edit of a missing macro")
	}
	// /undefn by number
	n := in.macros["foo"].ID
	if v := in.ExecFull(fmt.Sprintf("/undefn %d 999999", n)); v.Int() != 1 {
		t.Fatalf("undefn: %v", v)
	}
	if _, ok := in.macros["foo"]; ok {
		t.Fatal("foo must be gone")
	}
	// /liststreams
	in.ExecFull(`/test fd1 := tfopen("` + filepath.Join(dir, "a.txt") + `", "w")`)
	in.ExecFull(`/test fd2 := tfopen("` + filepath.Join(dir, "a.txt") + `", "a")`)
	in.ExecFull(`/test fd3 := tfopen("` + filepath.Join(dir, "a.txt") + `", "r")`)
	h.out = nil
	in.ExecFull("/liststreams")
	if len(h.out) != 4 || h.out[0] != "HANDLE MODE FLUSH NAME" ||
		h.out[1] != fmt.Sprintf("%6d   w    on  %s", 1, filepath.Join(dir, "a.txt")) ||
		!strings.HasPrefix(h.out[2], "     2   a    on  ") || !strings.HasPrefix(h.out[3], "     3   r      ") {
		t.Fatalf("liststreams: %q", h.out)
	}
}

func TestSaveworldAndLoad(t *testing.T) {
	dir := t.TempDir()
	in, _ := newST(t)
	in.ExecFull(`/test addworld("foo", "tiny", "h", "1", "bob", "pw", "", "e", "")`)
	in.ExecFull(`/test addworld("default", "", "", "", "me", "x")`)
	f := filepath.Join(dir, "worlds.tf")
	if v := in.ExecFull("/saveworld " + f); v.Int() != 2 {
		t.Fatalf("saveworld: %v", v)
	}
	b, _ := os.ReadFile(f)
	want := "/test addworld(\"default\", \"\", \"\", \"\", \"me\", \"x\")\n/test addworld(\"foo\", \"tiny\", \"h\", \"1\", \"bob\", \"pw\", \"\", \"e\")\n"
	if string(b) != want {
		t.Fatalf("saved:\n%s", b)
	}
	in2, _ := newST(t)
	if v := in2.ExecFull("/load " + f); !v.Truth() {
		t.Fatal("load")
	}
	w, ok := in2.World("foo")
	if !ok || w.Character != "bob" || !w.Echo {
		t.Fatalf("loaded: %+v", w)
	}
	if d, ok := in2.World("default"); !ok || d.Character != "me" {
		t.Fatal("default world")
	}
}

func TestStateIsPerInterp(t *testing.T) {
	a, _ := newST(t)
	b, _ := newST(t)
	a.ExecFull(`/test addworld("only-a", "", "h", "1")`)
	a.RecordLocalLine("x")
	if _, ok := b.World("only-a"); ok || len(b.HistoryLines(HistGlobal, "")) != 0 {
		t.Fatal("state leaked between interpreters")
	}
}
