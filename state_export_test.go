package gtfparser

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------- the busy session

// stClock is a shared fake clock.
type stClock struct{ t time.Time }

func (c *stClock) now() time.Time { return c.t }

type sxOpts struct {
	lib   bool   // load the real stdlib.tf (GTF_TF_LIB)
	files string // directory for log / stream / quote files ("" = none)
	noEnv bool
}

// sxNew makes an interpreter the way a client would: options, host, clock.
func sxNew(t testing.TB, clk *stClock, o sxOpts) (*Interp, *stHost) {
	t.Helper()
	h := &stHost{connected: map[string]bool{"w1": true, "w2": true}, fg: "w1", visible: 1}
	var opts []Option
	if o.noEnv {
		opts = append(opts, WithoutEnvironment())
	}
	in := New(h, opts...)
	in.Warn = func(s string) { h.warns = append(h.warns, s) }
	in.Now = clk.now
	if lib := tfLibDir(); lib != "" {
		in.LibDirs = []string{lib}
	}
	if o.lib {
		lib := tfLibDir()
		if err := in.LoadFile(filepath.Join(lib, "stdlib.tf")); err != nil {
			t.Fatal(err)
		}
		h.take()
		h.warns = nil
	}
	return in, h
}

// sxRun runs one line (a /def as a file would load it) and returns everything
// it showed.
func sxRun(in *Interp, h *stHost, line string) string { return sxRunLine(in, h, line) }

func sxRunLine(in *Interp, h *stHost, line string) string {
	h.out, h.warns = nil, nil
	if strings.HasPrefix(line, "/def") || strings.HasPrefix(line, "/undef") {
		in.Exec(line)
	} else {
		in.ExecFull(line)
	}
	return sxTake(h)
}

func sxTake(h *stHost) string {
	var b strings.Builder
	for _, o := range h.out {
		b.WriteString(o + "\n")
	}
	for _, w := range h.warns {
		b.WriteString("!" + w + "\n")
	}
	h.out, h.warns = nil, nil
	return b.String()
}

// sxBusy builds a session that has used most of what the state covers.
func sxBusy(t testing.TB, clk *stClock, o sxOpts) (*Interp, *stHost) {
	in, h, _ := sxBusyOut(t, clk, o)
	return in, h
}

// sxBusyOut is sxBusy that also returns whatever the setup lines printed.
func sxBusyOut(t testing.TB, clk *stClock, o sxOpts) (*Interp, *stHost, string) {
	in, h := sxNew(t, clk, o)
	in.EnableHookEvents("GMCP")
	var printed strings.Builder
	sxRun := func(in *Interp, h *stHost, line string) string {
		r := sxRunLine(in, h, line)
		if r != "" && !strings.HasPrefix(line, "/log ") {
			printed.WriteString(line + " => " + r)
		}
		return r
	}
	setup := []string{
		`/set alpha=5`, `/set beta=hello world`, `/set gamma=1.5`, `/test delta := 2.75`, `/test eps := 7 * 6`,
		`/set ptime=2`, `/set wrap=off`, `/set more=on`, `/set isize=5`, `/set maildelay=30`, `/set login=off`,
		`/set hiliteattr=Cgreen`, `/set matching=regexp`, `/set matching=glob`, `/set sigfigs=12`, `/set log_prefix=%t> `,
		`/set histsize=50`, `/set status_height=2`, `/set tabsize=4`, `/set textdiv_str=-----`,
		`/test noenc := "caf\u00e9"`, `/set empty=`, `/set neg=-17`, `/set dt=1:30`,
		`/export alpha`, `/setenv GTF_STATE_TEST=exported value`,
		`/let toplevel=local top`,
		`/test addworld("w1", "tiny", "mud.example.org", "4000", "hero", "s3cret", "", "")`,
		`/test addworld("w2", "lp", "other.example.org", "23", "", "", "mfile.tf", "x")`,
		`/test addworld("default", "", "", "", "guest", "guestpw")`,
		`/def -t'foo*' trig_glob = /echo glob trigger [%*]`,
		`/def -mregexp -t'^bar (\w+) (\d+)' -P1Cred;2BCblue trig_re = /echo regexp %P1 %P2`,
		`/def -t'*shot*' -n2 shotty = /echo shot fired`,
		`/def -t'*prio*' -p5 -F prio_hi = /echo prio high`,
		`/def -t'*prio*' -p1 prio_lo = /echo prio low`,
		`/def -t'*quiet*' -q quiet_one = /echo quiet`,
		`/def -t'*cond*' -E'alpha == 5' cond_one = /echo cond ok`,
		`/def -t'gagme*' -ag gagger`,
		`/def -t'*hilite*' -ahCred hil = /echo hilited`,
		`/def -t'*world*' -ww1 world_one = /echo only in w1`,
		`/def -hCONNECT hook_conn = /echo connected %*`,
		`/def -hDISCONNECT|REDEF hook_two = /echo two %*`,
		`/def -h'SEND *secret*' hook_arg = /echo send hook`,
		`/def -T'tiny*' -t'*typed*' typed_world = /echo type match`,
		`/def -b'^Xz' -E(1) bound = /echo bound key`,
		`/def plain = /echo plain %1 and %*`,
		`/def -i hidden = /echo hidden`,
		`/def -c100 -p3 chance = /echo chance`,
		`/def multi = /echo one%; /echo two`,
		`/def -hGMCP hook_gmcp = /echo gmcp %*`,
		`/def tmpgone = /echo gone`, `/undef tmpgone`,
		`/hook off`, `/hook on`,
		`/repeat -30 i /echo tick`,
		`/repeat -5 3 /echo rep3`,
		`/repeat -w2 -7 2 /echo in-w2`,
		`/repeat -P i /echo on prompt`,
	}
	for _, l := range setup {
		sxRun(in, h, l)
	}
	if o.files != "" {
		qf := filepath.Join(o.files, "quote.txt")
		os.WriteFile(qf, []byte("first\nsecond\nthird\n"), 0o644)
		for _, l := range []string{
			`/quote -3 /echo q: '` + qf,
			`/log ` + filepath.Join(o.files, "global.log"),
		} {
			sxRun(in, h, l)
		}
		rf := filepath.Join(o.files, "read.txt")
		os.WriteFile(rf, []byte("r1\nr2\nr3\n"), 0o644)
		wf := filepath.Join(o.files, "write.txt")
		for _, l := range []string{
			`/test r := tfopen("` + rf + `", "r")`, `/test tfread(r, line)`,
			`/test w := tfopen("` + wf + `", "w")`, `/test tfwrite(w, "w1")`,
			`/test a := tfopen("` + filepath.Join(o.files, "app.txt") + `", "a")`, `/test tfwrite(a, "a1")`,
		} {
			sxRun(in, h, l)
		}
	}
	sxRun(in, h, `/test q := tfopen("", "q")`)
	sxRun(in, h, `/test tfwrite(q, "queued 1")`)
	sxRun(in, h, `/test tfwrite(q, "queued 2")`)
	in.SetCurrentWorld("w1")
	for i := 0; i < 5; i++ {
		in.RecordInput(fmt.Sprintf("typed line %d", i))
	}
	in.RecordInputAttr("hunter2", "g") // typed with echo off
	in.RecordLocalLine("local one")
	in.RecordLocalLineAttr("local two", "B")
	in.RecordWorldLine("w1", "w1 says hi")
	in.RecordWorldLine("w1", "w1 again")
	in.RecordWorldLine("w2", "w2 line")
	in.RecordWorldLineAttr("w2", "w2 hilited", "Cred")
	in.RecordWorldLine("w1", "caf\xe9 latin1 bytes")
	sxRun(in, h, `/recordline -g recorded globally`)
	sxRun(in, h, `/limit -v *zork*`)
	sxRun(in, h, `/status_add -A @clock`)
	sxRun(in, h, `/status_add -r2 "lit":3`)
	sxRun(in, h, `/status_add -r2 alpha:6`)
	in.SetPrompt("w1", "HP>")
	in.SetLocalEcho("w2", false)
	in.Input().Set("half typed line", 4)
	in.RecallInput(-1, 0, "half typed line")
	in.Input().Set("half typed line", 4)
	// a few things have happened to the macros since they were defined
	procLine(in, "foo something")
	procLine(in, "this is a shot line")
	in.FireHook("CONNECT", "w1")
	h.take()
	h.warns = nil
	clk.t = clk.t.Add(1500 * time.Millisecond)
	return in, h, printed.String()
}

func sxClock() *stClock { return &stClock{time.Date(2024, 3, 5, 14, 7, 9, 250000000, time.UTC)} }

func sxSave(t testing.TB, in *Interp) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := in.SaveState(&b); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func sxLoad(t testing.TB, in *Interp, data []byte) {
	t.Helper()
	if err := in.LoadState(bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------- differential

// sxProbe is what the differential script runs in both interpreters.
func sxProbe(t *testing.T, a, b *Interp, ha, hb *stHost, clk *stClock) {
	t.Helper()
	same := func(what string, ga, gb string) {
		t.Helper()
		if ga != gb {
			t.Errorf("%s differs:\n--- original\n%s\n--- restored\n%s", what, ga, gb)
		}
	}
	both := func(line string) {
		t.Helper()
		same(line, sxRun(a, ha, line), sxRun(b, hb, line))
	}
	cmds := []string{
		`/list`, `/list -s`, `/list -i`, `/list -u`, `/list hook*`, `/listvar`, `/listvar -v`, `/listvar -x`,
		`/listworlds`, `/listworlds -s`, `/listworlds -u`, `/listbind`, `/listdef`, `/listhook`, `/listtrig`, `/listgag`,
		`/listhilite`, `/listpartial`, `/ps`, `/ps -s`, `/limit`,
		`/recall -g`, `/recall -l`, `/recall -ww1`, `/recall -ww2 -t%H`, `/recall -i 1-4`, `/recall -l`, `/recall -g 3`, `/recall -l 1-2`,
		`/recall -t%H:%M:%S -g 2-4`, `/log`, `/restrict`, `/localecho`, `/watchdog`, `/watchname`, `/features`,
		`/echo %{alpha} %{beta} %{gamma} %{delta} %{eps} %{ptime} %{dt} %{neg} [%{empty}] %{noenc}`,
		`/echo %{toplevel} %{world_name} %{world_host} %{world_character}`,
		`/test echo(strcat(whatis("alpha"), " ", whatis("gamma"), " ", whatis("delta"), " ", whatis("ptime"), " ", whatis("wrap")))`,
		`/test echo(strcat(world_info("w2", "host"), " ", world_info("w1", "type"), " ", world_info("default", "character"), " ", world_info("w1", "password")))`,
		`/test echo(strcat(nlog(), " ", nactive(), " ", is_open("w1"), " ", idle("w1")))`,
		`/test echo(strcat(kbhead(), "|", kbtail(), "|", kbpoint()))`,
		`/test echo(prompt())`,
		`/test echo(strcat(tfread(q, x), ":", x, " ", tfread(q, y), ":", y))`,
		`/dokey recallb`, `/test echo(strcat(kbhead(), "|", kbtail()))`,
		`/def -i newmacro = /echo new`, `/def -t'newt*' newtrig = /echo nt`, `/test echo(ismacro("newtrig"))`,
		`/def -t'*prio*' -p5 prio_mid = /echo prio mid`,
		`/def -t'*prio*' -p5 -F prio_hi2 = /echo prio hi2`,
		`/list prio*`, `/list -s prio*`,
		`/echo [%_loaded_libs]`, `/loaded stdlib.tf`, `/ismacro lp`, `/lp on`, `/lp off`, `/undef lp`, `/ismacro lp`, `/lp`, `/first a b c`,
		`/undef plain`, `/list plain`, `/undef shotty`,
		`/def -n3 -t'again*' again = /echo again`, `/list again`,
		`/repeat -4 2 /echo late`, `/ps`,
		`/kill 2`, `/ps`,
		`/set alpha=6`, `/echo %{alpha}`, `/unset beta`, `/listvar`,
		`/addworld -T'tiny' w3 mud3.example.org 55`, `/listworlds`,
		`/unworld w2`, `/listworlds`,
	}
	for _, c := range cmds {
		both(c)
	}
	for _, line := range []string{
		"foo bar baz", "bar word 42", "bar word 42 and more", "a shot line", "shot again", "shot third",
		"prio check", "quiet please", "cond test", "gagme now", "hilite this", "world only", "typed match",
		"nothing matches", "again 1", "again 2", "again 3", "again 4",
	} {
		for _, w := range []string{"w1", "w2"} {
			ra := a.ProcessText(LineInput{Text: line, World: w})
			rb := b.ProcessText(LineInput{Text: line, World: w})
			same("process "+w+": "+line, fmt.Sprintf("%v %v %v %d %v|%s", ra.Gag, ra.Line, ra.Matched != nil, ra.Count, macIDs(ra.Matched), sxTake(ha)),
				fmt.Sprintf("%v %v %v %d %v|%s", rb.Gag, rb.Line, rb.Matched != nil, rb.Count, macIDs(rb.Matched), sxTake(hb)))
		}
	}
	for _, ev := range [][2]string{{"CONNECT", "w1"}, {"DISCONNECT", "w2"}, {"SEND", "my secret text"}, {"SEND", "plain"}, {"REDEF", "macro x"}, {"GMCP", "Room.Info {}"}} {
		ma, mb := a.FireHook(ev[0], ev[1]), b.FireHook(ev[0], ev[1])
		same("hook "+ev[0], fmt.Sprint(macIDs(ma))+sxTake(ha), fmt.Sprint(macIDs(mb))+sxTake(hb))
	}
	same("prompt", a.PromptText("w1")+fmt.Sprint(a.LocalEcho("w2"), a.LocalEcho("w1"), a.CurrentWorld()), b.PromptText("w1")+fmt.Sprint(b.LocalEcho("w2"), b.LocalEcho("w1"), b.CurrentWorld()))
	same("input line", fmt.Sprint(a.Input().Text(), a.Input().Cursor(), a.InputHistory()), fmt.Sprint(b.Input().Text(), b.Input().Cursor(), b.InputHistory()))
	goLevel := func(in *Interp) string {
		ws := in.Worlds()
		for i := range ws {
			ws[i].hist = nil // a pointer
		}
		input := strings.Join(in.HistoryLines(HistInput, ""), "|")
		input = strings.Replace(input, "hunter2", "", 1) // typed with echo off: never carried
		return fmt.Sprintf("%+v %q %q %q", ws, in.HistoryLines(HistWorld, "w1"), in.HistoryLines(HistGlobal, ""), input)
	}
	same("go-level worlds and histories", goLevel(a), goLevel(b))
	same("bindings", fmt.Sprint(bindingsText(a)), fmt.Sprint(bindingsText(b)))
	for _, k := range []string{"^W", "^Xz", "^R"} {
		ma, _ := a.FindKey(k)
		mb, _ := b.FindKey(k)
		same("key "+k, fmt.Sprint(macIDs([]*Macro{ma})), fmt.Sprint(macIDs([]*Macro{mb})))
	}
	same("keys typed", fmt.Sprint(typeKeys(a, ha, "^Xz")), fmt.Sprint(typeKeys(b, hb, "^Xz")))
	// the clock runs: processes fire at the same moments with the same results
	for i := 0; i < 70; i++ {
		clk.t = clk.t.Add(time.Second)
		a.RunDue()
		b.RunDue()
		same(fmt.Sprintf("tick %d", i), sxTake(ha), sxTake(hb))
		if i%10 == 0 {
			both(`/ps`)
		}
		na, oka := a.NextDue()
		nb, okb := b.NextDue()
		if oka != okb || !na.Equal(nb) {
			t.Fatalf("NextDue differs at %d: %v %v / %v %v", i, na, oka, nb, okb)
		}
	}
	a.Prompt()
	b.Prompt()
	same("prompt procs", sxTake(ha), sxTake(hb))
	for _, c := range []string{`/ps`, `/list`, `/listvar`, `/recall -g`} {
		both(c)
	}
	// numbering continues the same way
	for _, c := range []string{`/def zzfinal = /echo z`, `/def -t'q*' zzfinal2 = /echo z2`, `/list zzfinal*`, `/repeat -9 i /echo late`, `/ps`} {
		both(c)
	}
	if sa, sb := sxSave(t, a), sxSave(t, b); !bytes.Equal(sa, sb) {
		t.Errorf("state after the probe differs (%d vs %d bytes)\n%s", len(sa), len(sb), firstDiff(sa, sb))
	}
}

func macIDs(ms []*Macro) []int {
	var ids []int
	for _, m := range ms {
		if m != nil {
			ids = append(ids, m.ID)
		} else {
			ids = append(ids, 0)
		}
	}
	return ids
}

func bindingsText(in *Interp) string {
	var b strings.Builder
	for _, x := range in.Bindings() {
		fmt.Fprintf(&b, "%q=%d;", x.Seq, x.Macro.ID)
	}
	return b.String()
}

func typeKeys(in *Interp, h *stHost, seq string) string {
	in.KeyPressed(PrintToAscii(seq))
	return sxTake(h)
}

func firstDiff(a, b []byte) string {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	lo := i - 60
	if lo < 0 {
		lo = 0
	}
	hi := func(x []byte) int {
		if i+80 < len(x) {
			return i + 80
		}
		return len(x)
	}
	return fmt.Sprintf("first difference at byte %d:\n  a: %s\n  b: %s", i, a[lo:hi(a)], b[lo:hi(b)])
}

func sxCompare(t *testing.T, o sxOpts) {
	t.Helper()
	clk := sxClock()
	o.files = t.TempDir()
	a, ha := sxBusy(t, clk, o)
	data := sxSave(t, a)
	o.lib = false // the new process never runs stdlib.tf
	b, hb := sxNew(t, clk, o)
	b.SetCurrentWorld("") // the client has not told it anything yet
	sxLoad(t, b, data)
	if again := sxSave(t, b); !bytes.Equal(data, again) {
		t.Fatalf("save, load, save is not stable\n%s", firstDiff(data, again))
	}
	sxProbe(t, a, b, ha, hb, clk)
}

func TestStateDifferentialNoLibrary(t *testing.T) { sxCompare(t, sxOpts{}) }

func TestStateDifferentialWithLibrary(t *testing.T) {
	if tfLibDir() == "" {
		t.Skip("GTF_TF_LIB not set")
	}
	sxCompare(t, sxOpts{lib: true})
}

func TestStateDifferentialScriptLibrary(t *testing.T) {
	if tfLibDir() == "" {
		t.Skip("GTF_TF_LIB not set")
	}
	// the library macros run as script, not as the Go ports
	clk := sxClock()
	mk := func() (*Interp, *stHost) {
		h := &stHost{connected: map[string]bool{"w1": true}, fg: "w1", visible: 1}
		in := New(h, WithoutNativeLibrary())
		in.Warn = func(s string) { h.warns = append(h.warns, s) }
		in.Now = clk.now
		in.LibDirs = []string{tfLibDir()}
		return in, h
	}
	a, ha := mk()
	if err := a.LoadFile(filepath.Join(tfLibDir(), "stdlib.tf")); err != nil {
		t.Fatal(err)
	}
	sxRun(a, ha, `/def -t'x*' xx = /echo x`)
	data := sxSave(t, a)
	b, hb := mk()
	sxLoad(t, b, data)
	if again := sxSave(t, b); !bytes.Equal(data, again) {
		t.Fatalf("not stable\n%s", firstDiff(data, again))
	}
	for _, c := range []string{`/list`, `/listvar`, `/list -i`, `/echo $[ismacro("lp")]`, `/lp`} {
		if x, y := sxRun(a, ha, c), sxRun(b, hb, c); x != y {
			t.Errorf("%s differs:\n%s\n---\n%s", c, x, y)
		}
	}
}

// the busy session prints nothing while it is built: a script typo in it
// would silently weaken every other test
func TestStateBusySessionIsClean(t *testing.T) {
	_, _, out := sxBusyOut(t, sxClock(), sxOpts{files: t.TempDir()})
	if out != "" {
		t.Fatalf("setup produced output:\n%s", out)
	}
}

// ---------------------------------------------------------------- state equality helpers

func TestStateCanonical(t *testing.T) {
	clk := sxClock()
	a, _ := sxBusy(t, clk, sxOpts{files: t.TempDir()})
	s1, s2 := sxSave(t, a), sxSave(t, a)
	if !bytes.Equal(s1, s2) {
		t.Fatal("two saves of the same state differ")
	}
	var env stateEnvelope
	if err := json.Unmarshal(s1, &env); err != nil {
		t.Fatal(err)
	}
	for _, s := range stateSections {
		sec, ok := env.Sections[s.name]
		if !ok || sec.V != s.maxV || sec.Required != s.required {
			t.Errorf("section %s: %+v", s.name, sec)
		}
	}
	if env.Magic != stateMagic || env.Format != StateFormat || env.MinReader != 1 {
		t.Errorf("envelope: %+v", env)
	}
}

var _ = errors.New
