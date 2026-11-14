package gtfparser

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newOrig(t *testing.T, opts ...Option) (*Interp, *stHost) {
	t.Helper()
	in, h := newST(t, opts...)
	return in, h
}

func warned(h *stHost, sub string) bool {
	for _, w := range h.warns {
		if strings.Contains(w, sub) {
			return true
		}
	}
	return false
}

// Every command and function that reaches the shell, the file system or the
// network, with the lowest /restrict level that must refuse it. The table is
// the audit listed in docs/engine/origins.md.
var auditOps = []struct {
	name   string
	level  int    // lowest restriction level that refuses it
	script string // {D} is a scratch directory
	effect string // file the operation creates when it is allowed ("" = none to check)
}{
	{"/sh", RestrictShell, "/sh touch {D}/e", "e"},
	{"/quote -S !", RestrictShell, "/quote -S !touch {D}/e", "e"},
	{"/quote !", RestrictShell, "/quote !touch {D}/e", ""},
	{"/eval /sh", RestrictShell, "/eval /sh touch {D}/e", "e"},
	{"/repeat -S /sh", RestrictShell, "/repeat -S 1 /sh touch {D}/e", "e"},
	{"tfopen(cmd,p)", RestrictShell, `/test tfclose(tfopen("touch {D}/e", "p"))`, "e"},
	{"tfopen(cmd,P)", RestrictShell, `/test tfclose(tfopen("touch {D}/e", "P"))`, "e"},
	{"/load", RestrictFile, "/load {D}/s.tf", "loaded"},
	{"/require", RestrictFile, "/require s.tf", "loaded"},
	{"/log", RestrictFile, "/log {D}/e", "e"},
	{"/save", RestrictFile, "/save {D}/e", "e"},
	{"/saveworld", RestrictFile, "/saveworld {D}/e", "e"},
	{"/lcd", RestrictFile, "/lcd {D}", "cwd"},
	{"/quote -S '", RestrictFile, "/quote -S 'f {D}/s.tf", ""},
	{"/quote '", RestrictFile, "/quote '{D}/s.tf", ""},
	{"tfopen(f,r)", RestrictFile, `/test tfopen("{D}/s.tf", "r")`, ""},
	{"tfopen(f,w)", RestrictFile, `/test tfopen("{D}/e", "w")`, "e"},
	{"tfopen(f,a)", RestrictFile, `/test tfopen("{D}/e", "a")`, "e"},
	{"fwrite()", RestrictFile, `/test fwrite("{D}/e", "x")`, "e"},
	{"addworld()", RestrictWorld, `/test addworld("pw", "", "h.example", "23")`, "world"},
	{"/connect host port", RestrictWorld, "/connect h.example 4000", "connect"},
}

func auditSetup(t *testing.T) (dir string) {
	dir = t.TempDir()
	os.WriteFile(filepath.Join(dir, "s.tf"), []byte("/set loaded=1\n"), 0o644)
	t.Chdir(dir) // /lcd moves the process: keep it inside the test
	return dir
}

func auditEffect(t *testing.T, in *Interp, h *stHost, dir, eff string) bool {
	switch eff {
	case "":
		return false
	case "loaded":
		return in.Var("loaded") == "1"
	case "world":
		_, ok := in.World("pw")
		return ok
	case "connect":
		return len(h.connects) > 0
	case "cwd":
		wd, _ := os.Getwd()
		r1, _ := filepath.EvalSymlinks(wd)
		r2, _ := filepath.EvalSymlinks(dir)
		return r1 == r2 && wd != ""
	}
	_, err := os.Stat(filepath.Join(dir, eff))
	return err == nil
}

// TestOriginAudit runs every guarded operation under a restricted origin at
// the level that must refuse it (it prints "restricted" and has no effect),
// and one level below (it works), and as an origin with no level (it works).
func TestOriginAudit(t *testing.T) {
	for _, op := range auditOps {
		op := op
		t.Run(op.name, func(t *testing.T) {
			// refused at op.level
			dir := auditSetup(t)
			in, h := newOrig(t, WithLibDirs(dir))
			in.SetOriginRestriction("peer:x", op.level)
			if op.name == "/lcd" {
				os.Mkdir(filepath.Join(dir, "sub"), 0o755)
			}
			script := strings.ReplaceAll(op.script, "{D}", dir)
			if op.name == "/lcd" {
				script = strings.ReplaceAll(op.script, "{D}", dir+"/sub")
			}
			in.ExecAs("peer:x", script)
			if testing.Verbose() {
				t.Logf("%q", h.warns)
			}
			if !warned(h, "restricted") {
				t.Fatalf("%s: not refused at level %d; warnings %q out %q", op.name, op.level, h.warns, h.out)
			}
			if op.effect != "" && op.effect != "cwd" && auditEffect(t, in, h, dir, op.effect) {
				t.Fatalf("%s: refused but had its effect", op.name)
			}
			if op.name == "/lcd" {
				if wd, _ := os.Getwd(); strings.HasSuffix(wd, "sub") {
					t.Fatalf("/lcd changed directory")
				}
			}
			if len(in.procs) != 0 {
				t.Fatalf("%s: refused but left a process", op.name)
			}
			// the host itself is not affected
			if in.EffectiveRestriction() != 0 {
				t.Fatalf("restriction leaked out of ExecAs: %d", in.EffectiveRestriction())
			}

			// allowed one level below (or with no level at all)
			for _, below := range []int{op.level - 1, 0} {
				dir := auditSetup(t)
				in, h := newOrig(t, WithLibDirs(dir))
				if below > 0 {
					in.SetOriginRestriction("peer:x", below)
				}
				if op.name == "/lcd" {
					os.Mkdir(filepath.Join(dir, "sub"), 0o755)
				}
				script := strings.ReplaceAll(op.script, "{D}", dir)
				if op.name == "/lcd" {
					script = strings.ReplaceAll(op.script, "{D}", dir+"/sub")
				}
				in.ExecAs("peer:x", script)
				if warned(h, "restricted") {
					t.Fatalf("%s refused at level %d: %q", op.name, below, h.warns)
				}
			}
		})
	}
}

// peerIn is an interpreter with the origin "peer:p" restricted to level.
func peerIn(t *testing.T, level int) (*Interp, *stHost, string) {
	t.Helper()
	dir := auditSetup(t)
	in, h := newOrig(t, WithLibDirs(dir))
	in.SetOriginRestriction("peer:p", level)
	return in, h, dir
}

func exists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

func TestOriginTriggerHookAndTimerKeepTheOrigin(t *testing.T) {
	in, h, dir := peerIn(t, RestrictShell)
	// the host defines a trigger of its own, the peer one that tries a shell
	in.Exec("/def -t'host says *' hostTrig = /sh touch " + dir + "/host")
	in.ExecAs("peer:p", "/def -t'peer says *' peerTrig = /sh touch "+dir+"/peer")
	in.ExecAs("peer:p", "/def -hCONNECT peerHook = /sh touch "+dir+"/hook")
	in.ExecAs("peer:p", "/repeat -1 2 /sh touch "+dir+"/timer")
	in.ExecAs("peer:p", "/quote -0 -dexec `/echo /sh touch "+dir+"/quoted")
	if m := in.FindMacro("peerTrig"); m == nil || m.Origin() != "peer:p" {
		t.Fatalf("trigger not tagged: %+v", m)
	}
	if in.FindMacro("hostTrig").Origin() != "" {
		t.Fatal("host macro tagged")
	}
	clock := in.now()
	in.Now = func() time.Time { return clock }
	in.ProcessLine("peer says hi")
	in.ProcessLine("host says hi")
	in.FireHook("CONNECT", "w")
	for i := 0; i < 4; i++ {
		clock = clock.Add(2 * time.Second)
		in.RunDue()
	}
	for _, f := range []string{"peer", "hook", "timer", "quoted"} {
		if exists(dir, f) {
			t.Errorf("%s ran a shell for the peer", f)
		}
	}
	if !exists(dir, "host") {
		t.Error("the host's own trigger was refused")
	}
	if !warned(h, "SH: restricted") && !warned(h, "restricted") {
		t.Errorf("no refusal printed: %q", h.warns)
	}
	if got := len(in.procs); got != 0 {
		t.Errorf("procs left: %d", got)
	}
}

func TestOriginProcessKeepsOriginAfterMacroEnds(t *testing.T) {
	in, _, dir := peerIn(t, RestrictShell)
	in.Exec("/def starter = /repeat -1 3 /sh touch " + dir + "/timer")
	in.ExecAs("peer:p", "/starter") // the host's macro, called by the peer
	if len(in.procs) != 1 || in.procs[0].origin != "peer:p" {
		t.Fatalf("process not tagged: %+v", in.procs)
	}
	clock := in.now()
	in.Now = func() time.Time { return clock }
	for i := 0; i < 3; i++ {
		clock = clock.Add(2 * time.Second)
		in.RunDue()
	}
	if exists(dir, "timer") {
		t.Fatal("a peer's timer got a shell")
	}
}

func TestOriginStrictestWinsAndPropagates(t *testing.T) {
	in, h, dir := peerIn(t, RestrictShell)
	in.SetOriginRestriction("peer:q", RestrictFile)
	in.Exec("/def hostTool = /sh touch " + dir + "/tool")
	// the peer calls the host's macro: it still runs restricted
	in.ExecAs("peer:p", "/hostTool")
	if exists(dir, "tool") {
		t.Fatal("host macro called by a peer ran a shell")
	}
	in.ExecAs("peer:p", "/def fileOp = /log "+dir+"/log2")
	in.ExecAs("peer:q", "/def shellOp = /sh touch "+dir+"/q")
	// p (shell) running q's macro: q's level applies as well
	in.ExecAs("peer:p", "/shellOp")
	if exists(dir, "q") {
		t.Fatal("q's macro ran a shell")
	}
	// q (file) calling p's macro (only shell restricted) must not log
	in.ExecAs("peer:q", "/fileOp")
	if exists(dir, "log2") {
		t.Fatal("a stricter caller was loosened by calling a looser origin's macro")
	}
	// the host calls p's macro: only p's level applies, and /log is fine
	in.Exec("/fileOp")
	if !exists(dir, "log2") {
		t.Fatalf("p's macro could not log at its own level: %q", h.warns)
	}
	// a macro defined while both run is tagged with the stricter origin
	in.ExecAs("peer:p", "/def mk = /def made = /echo")
	in.ExecAs("peer:q", "/mk")
	if o := in.FindMacro("made").Origin(); o != "peer:q" {
		t.Fatalf("made under p and q is tagged %q, want the stricter peer:q", o)
	}
	// ...and after the lift nothing is left behind
	if in.EffectiveRestriction() != 0 || len(in.origStack) != 0 {
		t.Fatal("origin stack leaked")
	}
}

func TestOriginCannotLiftItsOwnRestriction(t *testing.T) {
	in, h, dir := peerIn(t, RestrictShell)
	try := func(label string, script ...string) {
		t.Helper()
		for _, l := range script {
			in.ExecAs("peer:p", l)
		}
		if exists(dir, "x") {
			t.Fatalf("%s: got a shell", label)
		}
	}
	try("restrict none", "/restrict none", "/sh touch "+dir+"/x")
	if !warned(h, "cannot be lowered") {
		t.Errorf("no 'cannot be lowered': %q", h.warns)
	}
	try("def under another name", "/def other = /sh touch "+dir+"/x", "/other")
	try("eval", "/eval /eval /eval /sh touch "+dir+"/x")
	try("test", "/test 1", "/test 1 & 1", "/sh touch "+dir+"/x")
	try("repeat -S", "/repeat -S 1 /eval /sh touch "+dir+"/x")
	try("bind", "/bind ^Ax = /sh touch "+dir+"/x")
	try("undef+redef", "/undef hostTool", "/def hostTool = /sh touch "+dir+"/x", "/hostTool")
	try("alias via def -b", "/def -b'^Ay' = /sh touch "+dir+"/x")
	try("@ form", "/@sh touch "+dir+"/x")
	try("eval of @ form", "/eval /@sh touch "+dir+"/x")
	try("load", "/load "+dir+"/nothing.tf")
	try("sh via hook", "/def -hCONNECT pwn = /sh touch "+dir+"/x")
	in.FireHook("CONNECT", "w")
	if exists(dir, "x") {
		t.Fatal("hook got a shell")
	}
	// the bound key macros run restricted too
	for _, seq := range []string{"^Ax", "^Ay"} {
		if m := in.FindBinding(seq); m != nil {
			in.callMacro(m, "", false)
		}
	}
	if exists(dir, "x") {
		t.Fatal("a key binding got a shell")
	}
	// a peer's /restrict never raises the global level (it would lock the owner out)
	in.ExecAs("peer:p", "/restrict world")
	if in.Restriction() != 0 {
		t.Fatalf("peer raised the global level to %d", in.Restriction())
	}
	if in.OriginRestriction("peer:p") != RestrictWorld {
		t.Fatalf("peer could not raise its own level: %d", in.OriginRestriction("peer:p"))
	}
	in.Exec("/sh touch " + dir + "/host")
	if !exists(dir, "host") {
		t.Fatal("the host lost its shell")
	}
	// the Go API will not lower a level while an origin's code runs
	in.RegisterCommand("lift", func(in *Interp, a string) Value {
		return Bool(in.SetOriginRestriction("peer:p", RestrictNone))
	})
	in.ExecAs("peer:p", "/lift")
	if in.OriginRestriction("peer:p") != RestrictWorld {
		t.Fatal("a host command reached from peer code lifted the restriction")
	}
	if v := in.Exec("/lift"); !v.Truth() || in.OriginRestriction("peer:p") != 0 {
		t.Fatal("the host could not lift the restriction")
	}
}

func TestOriginLiftAppliesToExistingMacros(t *testing.T) {
	in, _, dir := peerIn(t, RestrictShell)
	in.ExecAs("peer:p", "/def go = /sh touch "+dir+"/x")
	in.Exec("/go")
	if exists(dir, "x") {
		t.Fatal("ran restricted macro with a shell")
	}
	in.SetOriginRestriction("peer:p", RestrictNone)
	in.Exec("/go")
	if !exists(dir, "x") {
		t.Fatal("lifting the restriction did not take effect")
	}
}

func TestOriginTagsRedefinitionAndEdit(t *testing.T) {
	in, _, dir := peerIn(t, RestrictShell)
	in.Exec("/def tool = /echo host")
	if in.FindMacro("tool").Origin() != "" {
		t.Fatal("untagged expected")
	}
	// a peer redefining the macro under its own origin changes the tag
	in.ExecAs("peer:p", "/def tool = /sh touch "+dir+"/x")
	if o := in.FindMacro("tool").Origin(); o != "peer:p" {
		t.Fatalf("redefined tag %q", o)
	}
	in.Exec("/tool")
	if exists(dir, "x") {
		t.Fatal("redefined macro ran unrestricted")
	}
	// the host redefining it makes it the host's again
	in.Exec("/def tool = /sh touch " + dir + "/y")
	if in.FindMacro("tool").Origin() != "" {
		t.Fatal("host redefinition kept the tag")
	}
	in.Exec("/tool")
	if !exists(dir, "y") {
		t.Fatal("host macro refused")
	}
	// the peer /edits the host's macro: it becomes the peer's
	in.ExecAs("peer:p", "/edit tool = /sh touch "+dir+"/z")
	if in.FindMacro("tool").Origin() != "peer:p" {
		t.Fatal("peer /edit did not retag")
	}
	in.Exec("/tool")
	if exists(dir, "z") {
		t.Fatal("edited macro ran unrestricted")
	}
	// an edit by the host that does not touch the body keeps the peer's tag...
	in.Exec("/edit -p5 tool")
	if in.FindMacro("tool").Origin() != "peer:p" {
		t.Fatal("host option edit laundered the peer's body")
	}
	// ...one that replaces the body makes it the host's
	in.Exec("/edit tool = /sh touch " + dir + "/w")
	if in.FindMacro("tool").Origin() != "" {
		t.Fatal("host body edit kept the peer tag")
	}
	// identical redefinition under another origin is not a no-op
	in.Exec("/def same = /echo a")
	in.ExecAs("peer:p", "/def same = /echo a")
	if in.FindMacro("same").Origin() != "peer:p" {
		t.Fatal("identical redefinition by a peer left the host's tag")
	}
}

func TestOriginIntrospection(t *testing.T) {
	in, h, _ := peerIn(t, RestrictShell)
	in.Exec("/def mine = /echo a")
	in.ExecAs("peer:p", "/def theirs = /echo a")
	in.ExecAs("peer:p", "/def other = /echo $[current_origin()]")
	h.out = nil
	in.ExecFull("/echo [$[macro_origin('mine')]] [$[macro_origin('theirs')]] [$[current_origin()]]")
	in.ExecAs("peer:p", "/other")
	if got := strings.Join(h.out, "|"); got != "[] [peer:p] []|peer:p" {
		t.Fatalf("got %q", got)
	}
	// /list does not show tags (tf compatibility)
	h.out = nil
	in.Exec("/list theirs")
	if got := strings.Join(h.out, "|"); strings.Contains(got, "peer") {
		t.Fatalf("/list shows the origin: %q", got)
	}
	// a script's own macro of the same name wins over the extension
	in.Exec("/def macro_origin = /return 42")
	if v, _ := in.Eval("macro_origin('mine')"); v.String() != "42" {
		t.Fatalf("user macro shadowed: %v", v)
	}
}

func TestOriginStreamsAreCapabilities(t *testing.T) {
	in, h, dir := peerIn(t, RestrictFile)
	// the host opens a file and a pipe; the peer guesses the handles
	in.ExecFull(`/set fdf=$[tfopen("` + dir + `/log", "w")]`)
	in.ExecFull(`/set fdp=$[tfopen("cat >/dev/null", "P")]`)
	in.ExecFull(`/set fdq=$[tfopen()]`)
	in.ExecAs("peer:p", "/test tfwrite(fdf, \"stolen\")")
	in.ExecAs("peer:p", "/test tfwrite(fdp, \"stolen\")")
	if n := strings.Count(strings.Join(h.warns, "|"), "restricted"); n < 2 {
		t.Fatalf("host streams usable by a peer: %q", h.warns)
	}
	in.Exec("/test tfclose(fdf)")
	if b, _ := os.ReadFile(filepath.Join(dir, "log")); len(b) != 0 {
		t.Fatalf("peer wrote through the host's handle: %q", b)
	}
	// queues are fine for the peer, and a stream the peer owns keeps working
	h.warns = nil
	in.ExecFull(`/set fdq=$[tfopen()]`)
	in.ExecAs("peer:p", "/test tfwrite(fdq, \"ok\")")
	if len(h.warns) != 0 {
		t.Fatalf("queue refused: %q", h.warns)
	}
	in.Exec("/test tfclose(fdp)")
	// a stream opened by the peer at a level that allows it
	in.SetOriginRestriction("peer:p", RestrictShell)
	in.RunAs("peer:p", `/set mine=$[tfopen("`+dir+`/own", "w")]`)
	in.ExecAs("peer:p", `/test tfwrite(mine, "hello")`)
	in.ExecAs("peer:p", `/test tfclose(mine)`)
	if b, _ := os.ReadFile(filepath.Join(dir, "own")); string(b) != "hello\n" {
		t.Fatalf("own stream: %q %q", b, h.warns)
	}
}

func TestOriginStatusExpressionsCannotTouchFiles(t *testing.T) {
	in, _, dir := peerIn(t, RestrictShell)
	in.Exec(`/set status_int_t=fwrite("` + dir + `/s", "x")`)
	in.evalStatusExpr(in.globals["status_int_t"].String(), "t")
	if exists(dir, "s") {
		t.Fatal("a stored status expression wrote a file")
	}
	// without origins nothing changes: tf's behaviour is kept
	in2, _ := newST(t)
	in2.evalStatusExpr(`fwrite("`+dir+`/s2", "x")`, "t")
	if !exists(dir, "s2") {
		t.Fatal("status expressions changed without origins")
	}
}

func TestOriginStateRoundTrip(t *testing.T) {
	in, _, dir := peerIn(t, RestrictShell)
	in.SetOriginRestriction("peer:q", RestrictWorld)
	in.Exec("/def mine = /echo a")
	in.ExecAs("peer:p", "/def theirs = /sh touch "+dir+"/x")
	in.ExecAs("peer:q", "/def -t'hello' trig = /echo b")
	in.ExecAs("peer:p", "/repeat -60 i /sh touch "+dir+"/y")
	in.ExecFull(`/set fd=$[tfopen("` + dir + `/f", "w")]`)
	in.ExecAs("peer:p", `/test tfopen(, "q")`)
	var b1 bytes.Buffer
	if err := in.SaveState(&b1); err != nil {
		t.Fatal(err)
	}
	in2, h2 := newST(t, WithLibDirs(dir))
	if err := in2.LoadState(bytes.NewReader(b1.Bytes())); err != nil {
		t.Fatal(err)
	}
	if in2.OriginRestriction("peer:p") != RestrictShell || in2.OriginRestriction("peer:q") != RestrictWorld {
		t.Fatalf("levels lost: %v", in2.OriginRestrictions())
	}
	for name, want := range map[string]string{"mine": "", "theirs": "peer:p", "trig": "peer:q"} {
		if got := in2.FindMacro(name).Origin(); got != want {
			t.Errorf("macro %s: origin %q want %q", name, got, want)
		}
	}
	if len(in2.procs) != 1 || in2.procs[0].origin != "peer:p" {
		t.Fatalf("process tag lost: %+v", in2.procs)
	}
	_ = h2
	var b2 bytes.Buffer
	if err := in2.SaveState(&b2); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b1.Bytes(), b2.Bytes()) {
		os.WriteFile("/private/tmp/claude-501/-Users-rock-mush/67b79c2b-ca3c-41e1-9ae4-177603d4bb02/scratchpad/b1", b1.Bytes(), 0o644)
		os.WriteFile("/private/tmp/claude-501/-Users-rock-mush/67b79c2b-ca3c-41e1-9ae4-177603d4bb02/scratchpad/b2", b2.Bytes(), 0o644)
		t.Fatalf("save/load/save differs: diff %s/b1 %s/b2", dir, dir)
	}
	in2.Exec("/theirs")
	if exists(dir, "x") {
		t.Fatal("restored macro ran unrestricted")
	}
	// a load never lowers a level
	in3, _ := newST(t)
	in3.SetOriginRestriction("peer:p", RestrictWorld)
	if err := in3.LoadState(bytes.NewReader(b1.Bytes())); err != nil {
		t.Fatal(err)
	}
	if in3.OriginRestriction("peer:p") != RestrictWorld {
		t.Fatal("load lowered a level")
	}
	// states of sessions that never used origins have no such section
	in4, _ := newST(t)
	var b4 bytes.Buffer
	in4.SaveState(&b4)
	if bytes.Contains(b4.Bytes(), []byte("origins")) {
		t.Fatal("plain state mentions origins")
	}
	// an older reader would drop the tags: the section is required
	if !bytes.Contains(b1.Bytes(), []byte(`"origins":{"v":1,"required":true`)) {
		t.Fatal("origins section is not required")
	}
}

func TestOriginAPIValidation(t *testing.T) {
	in, _ := newOrig(t)
	if in.SetOriginRestriction("", RestrictShell) || in.SetOriginRestriction("a", 4) || in.SetOriginRestriction("a", -1) {
		t.Fatal("invalid arguments accepted")
	}
	if !in.SetOriginRestriction("a", RestrictFile) || in.OriginRestriction("a") != RestrictFile {
		t.Fatal("set failed")
	}
	if !in.SetOriginRestriction("a", RestrictNone) || len(in.OriginRestrictions()) != 0 {
		t.Fatal("remove failed")
	}
	// the global level and an origin combine
	in.SetRestriction(RestrictShell)
	in.SetOriginRestriction("a", RestrictFile)
	in.DoAs("a", func() {
		if in.EffectiveRestriction() != RestrictFile || in.CurrentOrigin() != "a" {
			t.Fatal("effective level")
		}
	})
	in.DoAs("unconfigured", func() {
		if in.EffectiveRestriction() != RestrictShell {
			t.Fatal("unconfigured origin changed the level")
		}
	})
	if v, err := in.EvalAs("a", "restrict_probe"); err != nil || v.String() != "" {
		t.Logf("EvalAs: %v %v", v, err)
	}
	if v := in.RunAs("a", "/restrict"); v.Int() != RestrictFile {
		t.Fatalf("/restrict reports %d under the origin", v.Int())
	}
	if v := in.Exec("/restrict"); v.Int() != RestrictShell {
		t.Fatalf("/restrict reports %d for the host", v.Int())
	}
}

// Without any origin the new code must change nothing observable.
func TestOriginNoOriginsNoChange(t *testing.T) {
	in, h := newOrig(t)
	in.Exec("/def a = /echo hi")
	in.Exec("/def a = /echo hi") // identical: no effect
	if in.FindMacro("a").Origin() != "" || in.EffectiveRestriction() != 0 {
		t.Fatal("state leaked")
	}
	in.Exec("/a")
	if strings.Join(h.out, "|") != "hi" {
		t.Fatalf("%q", h.out)
	}
}

// A trigger's -E condition is code its author wrote; it runs under the
// author's origin like the body does.
func TestOriginTriggerConditionRunsRestricted(t *testing.T) {
	in, _, dir := peerIn(t, RestrictFile)
	in.ExecAs("peer:p", `/def -t'peer says *' -E'fwrite("`+dir+`/cond", "x")' condTrig = /echo hi`)
	in.Exec(`/def -t'host says *' -E'fwrite("` + dir + `/hostcond", "x")' hostCond = /echo hi`)
	in.ProcessLine("peer says hi")
	if exists(dir, "cond") {
		t.Fatal("a peer's -E condition wrote a file")
	}
	in.ProcessLine("host says hi")
	if !exists(dir, "hostcond") {
		t.Fatal("the host's -E condition was refused")
	}
}
