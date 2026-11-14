package gtfparser

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// The frozen version-1 document: written by SaveState of sxFixtureSession when
// the format was introduced and never regenerated. Every later reader must load
// it. (GTF_GEN_STATE_FIXTURE=1 rewrites it; only do that for a new fixture file.)
const stateFixtureV1 = "testdata/state/v1-busy.json"

func sxFixtureSession(t testing.TB) (*Interp, *stHost, *stClock) {
	clk := sxClock()
	in, h, out := sxBusyOut(t, clk, sxOpts{noEnv: true})
	if out != "" {
		t.Fatalf("fixture session printed: %s", out)
	}
	return in, h, clk
}

func TestStateFixtureV1(t *testing.T) {
	if os.Getenv("GTF_GEN_STATE_FIXTURE") != "" {
		in, _, _ := sxFixtureSession(t)
		os.MkdirAll(filepath.Dir(stateFixtureV1), 0o755)
		if err := os.WriteFile(stateFixtureV1, sxSave(t, in), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(stateFixtureV1)
	if err != nil {
		t.Fatal(err)
	}
	clk := sxClock()
	clk.t = clk.t.Add(1500 * time.Millisecond) // the fixture session's clock when it was saved
	in, h := sxNew(t, clk, sxOpts{noEnv: true})
	sxLoad(t, in, data)
	// loading the frozen file and saving again gives it back while the writer
	// still writes the versions the file has
	var env stateEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatal(err)
	}
	same := true
	for _, s := range stateSections {
		if env.Sections[s.name].V != s.maxV && s.name != "origins" { // origins: only written when used
			same = false
		}
	}
	if same {
		if again := sxSave(t, in); !bytes.Equal(again, data) {
			t.Errorf("v1 file does not survive load and save\n%s", firstDiff(data, again))
		}
	}

	checks := []struct{ cmd, want string }{
		{`/echo %{alpha} %{beta} %{gamma} %{ptime} %{dt} %{toplevel}`, "5 hello world 1.5 2.0 1:30 local top\n"},
		{`/ps -s`, "1\n2\n3\n4\n"},
		{`/listworlds -s`, "default\nw1\nw2\n"},
		{`/list -s trig_glob`, "% 4: (trig) 'foo*' trig_glob \n"},
		{`/test echo(world_info("w1", "password"))`, "s3cret\n"},
		{`/test echo(strcat(kbhead(), "|", kbtail()))`, "half| typed line\n"},
	}
	for _, c := range checks {
		if got := sxRun(in, h, c.cmd); got != c.want {
			t.Errorf("%s = %q, want %q", c.cmd, got, c.want)
		}
	}
	if m := in.FindMacro("trig_re"); m == nil || m.ID != 5 || len(m.SubAttr) != 2 || m.Trig.Style != "regexp" {
		t.Errorf("trig_re = %+v", m)
	}
	if got := in.HistoryLines(HistWorld, "w1"); len(got) != 3 || got[2] != "caf\xe9 latin1 bytes" {
		t.Errorf("w1 history %q", got)
	}
	if lines := in.HistoryLines(HistInput, ""); len(lines) < 6 || lines[5] != "" {
		t.Errorf("input history %q (the line typed with echo off must be blank)", lines)
	}
}

func envMutate(t *testing.T, data []byte, f func(*stateEnvelope)) []byte {
	t.Helper()
	var env stateEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatal(err)
	}
	f(&env)
	out, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// target is an interpreter with some state of its own, so "untouched" means something.
func sxTarget(t *testing.T) (*Interp, *stHost, []byte) {
	clk := sxClock()
	in, h := sxNew(t, clk, sxOpts{noEnv: true})
	for _, l := range []string{`/set mine=1`, `/def -t'mine*' mymac = /echo mine`, `/repeat -9 i /echo mine`, `/test addworld("mw", "", "h", "1")`} {
		sxRun(in, h, l)
	}
	return in, h, sxSave(t, in)
}

func TestStateVersionRules(t *testing.T) {
	good, _ := os.ReadFile(stateFixtureV1)
	cases := []struct {
		name string
		mut  func(*stateEnvelope)
		want error // nil: loads
	}{
		{"newer envelope", func(e *stateEnvelope) { e.Format, e.MinReader = 2, 2 }, ErrStateTooNew},
		{"newer writer, readable envelope", func(e *stateEnvelope) { e.Format = 7 }, nil},
		{"required section too new", func(e *stateEnvelope) {
			s := e.Sections["globals"]
			s.V = 2
			e.Sections["globals"] = s
		}, ErrStateTooNew},
		{"required unknown section", func(e *stateEnvelope) {
			e.Sections["hologram"] = stateSection{V: 1, Required: true, Data: json.RawMessage(`{}`)}
		}, ErrStateTooNew},
		{"optional unknown section", func(e *stateEnvelope) {
			e.Sections["hologram"] = stateSection{V: 3, Data: json.RawMessage(`{"x":[1,2,3]}`)}
		}, nil},
		{"optional known section too new", func(e *stateEnvelope) {
			e.Sections["limit"] = stateSection{V: 9, Data: json.RawMessage(`"from the future"`)}
		}, nil},
		{"section missing", func(e *stateEnvelope) { delete(e.Sections, "procs") }, nil},
		{"version zero", func(e *stateEnvelope) {
			s := e.Sections["macros"]
			s.V = 0
			e.Sections["macros"] = s
		}, ErrStateCorrupt},
		{"bad magic", func(e *stateEnvelope) { e.Magic = "other" }, ErrStateCorrupt},
		{"format zero", func(e *stateEnvelope) { e.Format = 0 }, ErrStateCorrupt},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in, _, before := sxTarget(t)
			err := in.LoadState(bytes.NewReader(envMutate(t, good, c.mut)))
			if c.want == nil {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
			if after := sxSave(t, in); !bytes.Equal(before, after) {
				t.Fatalf("a refused load changed the interpreter\n%s", firstDiff(before, after))
			}
		})
	}
	t.Run("skipped optional section leaves that part alone", func(t *testing.T) {
		in, h, _ := sxTarget(t)
		sxRun(in, h, `/limit foo*`)
		data := envMutate(t, good, func(e *stateEnvelope) {
			e.Sections["limit"] = stateSection{V: 9, Data: json.RawMessage(`null`)}
		})
		if err := in.LoadState(bytes.NewReader(data)); err != nil {
			t.Fatal(err)
		}
		if !in.LimitActive() {
			t.Error("the /limit of the target was lost")
		}
		if in.Var("alpha") != "5" || in.FindMacro("mymac") != nil {
			t.Error("the rest of the state was not replaced")
		}
	})
}

func TestStateCorruptInput(t *testing.T) {
	good, _ := os.ReadFile(stateFixtureV1)
	sec := func(name, data string) string {
		return `{"magic":"gtfparser-state","format":1,"minReader":1,"sections":{"` + name + `":{"v":1,"required":true,"data":` + data + `}}}`
	}
	table := []struct {
		name, in string
		want     error
	}{
		{"empty", ``, ErrStateCorrupt},
		{"garbage", `hello`, ErrStateCorrupt},
		{"null", `null`, ErrStateCorrupt},
		{"array", `[]`, ErrStateCorrupt},
		{"object", `{}`, ErrStateCorrupt},
		{"binary", "\x00\x01\xff\xfe", ErrStateCorrupt},
		{"bad value kind", sec("globals", `{"vars":[{"n":"a","v":{"k":"zz"}}]}`), ErrStateCorrupt},
		{"bad float", sec("globals", `{"vars":[{"n":"a","v":{"k":"r","f":"xyz"}}]}`), ErrStateCorrupt},
		{"bad decimal", sec("globals", `{"vars":[{"n":"a","v":{"k":"d","d":"1.5"}}]}`), ErrStateCorrupt},
		{"bad string object", sec("globals", `{"vars":[{"n":{"b64":"!!"},"v":{"k":"i"}}]}`), ErrStateCorrupt},
		{"section data null", sec("globals", `null`), ErrStateCorrupt},
		{"section data wrong type", sec("globals", `[1,2]`), ErrStateCorrupt},
		{"macro numbers repeat", sec("macros", `{"macros":[{"id":3,"pri":1,"prob":100},{"id":3,"pri":1,"prob":100}]}`), ErrStateCorrupt},
		{"list names unknown macro", sec("macros", `{"macros":[],"trigGlobal":[9]}`), ErrStateCorrupt},
		{"unknown hook event", sec("macros", `{"macros":[{"id":1,"hooks":["NOSUCH"],"isHook":true,"pri":1,"prob":100}]}`), ErrStateIncompatible},
		{"bad pattern", sec("macros", `{"macros":[{"id":1,"trig":{"s":"(","style":"regexp"},"pri":1,"prob":100}]}`), ErrStateIncompatible},
		{"unknown style", sec("macros", `{"macros":[{"id":1,"trig":{"s":"x","style":"nosuch"},"pri":1,"prob":100}]}`), ErrStateIncompatible},
		{"duplicate world", sec("worlds", `{"worlds":[{"name":"a"},{"name":"A"}]}`), ErrStateCorrupt},
		{"bad time", sec("history", `{"input":{"max":1,"total":1,"lines":[{"time":"yesterday"}]},"local":{"max":1,"total":0,"lines":[]},"global":{"max":1,"total":0,"lines":[]},"inputIdx":-1}`), ErrStateCorrupt},
		{"empty input history", sec("history", `{"input":{"max":1,"total":0,"lines":[]},"local":{"max":1,"total":0,"lines":[]},"global":{"max":1,"total":0,"lines":[]},"inputIdx":-1}`), ErrStateCorrupt},
		{"bad process", sec("procs", `{"procs":[{"pid":0,"kind":"r"}]}`), ErrStateCorrupt},
		{"bad stream", sec("streams", `{"streams":[{"id":"1","mode":"zz"}]}`), ErrStateCorrupt},
		{"restriction", sec("interp", `{"restriction":99,"lastRet":{"k":"i"},"retv":{"k":"i"},"userRes":{"k":"i"}}`), ErrStateCorrupt},
	}
	for _, c := range table {
		t.Run(c.name, func(t *testing.T) {
			in, _, before := sxTarget(t)
			err := in.LoadState(strings.NewReader(c.in))
			if !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
			if after := sxSave(t, in); !bytes.Equal(before, after) {
				t.Fatalf("a refused load changed the interpreter")
			}
		})
	}
	t.Run("truncated", func(t *testing.T) {
		in, _, before := sxTarget(t)
		for n := 0; n < len(good); n += 97 {
			if err := in.LoadState(bytes.NewReader(good[:n])); err == nil {
				t.Fatalf("a %d byte prefix loaded", n)
			}
		}
		if after := sxSave(t, in); !bytes.Equal(before, after) {
			t.Fatal("a refused load changed the interpreter")
		}
	})
	t.Run("mutated", func(t *testing.T) {
		rng := rand.New(rand.NewSource(7))
		for i := 0; i < 400; i++ {
			in, _, before := sxTarget(t)
			d := append([]byte(nil), good...)
			for k := 0; k < 1+rng.Intn(4); k++ {
				d[rng.Intn(len(d))] = byte(rng.Intn(256))
			}
			if rng.Intn(4) == 0 {
				d = d[:rng.Intn(len(d))]
			}
			if err := in.LoadState(bytes.NewReader(d)); err != nil {
				if after := sxSave(t, in); !bytes.Equal(before, after) {
					t.Fatalf("iteration %d: a refused load (%v) changed the interpreter", i, err)
				}
			}
		}
	})
	t.Run("reader failure", func(t *testing.T) {
		in, _, before := sxTarget(t)
		if err := in.LoadState(io_errReader{}); err == nil {
			t.Fatal("no error")
		}
		if after := sxSave(t, in); !bytes.Equal(before, after) {
			t.Fatal("changed")
		}
	})
}

type io_errReader struct{}

func (io_errReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

func TestStateMatchStyleMustBeRegistered(t *testing.T) {
	clk := sxClock()
	a, ha := sxNew(t, clk, sxOpts{})
	style := MatchStyleFunc(func(p string) (Matcher, error) {
		return MatcherFunc(func(s string) (bool, []int) { return strings.HasPrefix(s, p), nil }), nil
	})
	if err := a.RegisterMatchStyle("prefix", style); err != nil {
		t.Fatal(err)
	}
	sxRun(a, ha, `/def -mprefix -t'hello' pm = /echo prefix matched`)
	data := sxSave(t, a)
	b, hb := sxNew(t, clk, sxOpts{})
	before := sxSave(t, b)
	if err := b.LoadState(bytes.NewReader(data)); !errors.Is(err, ErrStateIncompatible) {
		t.Fatalf("err = %v", err)
	}
	if !bytes.Equal(before, sxSave(t, b)) {
		t.Fatal("changed")
	}
	b.RegisterMatchStyle("prefix", style)
	sxLoad(t, b, data)
	procLine(b, "hello there")
	if got := sxTake(hb); got != "prefix matched\n" {
		t.Fatalf("got %q", got)
	}
}

func TestStateNoEchoInputIsNeverSaved(t *testing.T) {
	clk := sxClock()
	a, ha := sxNew(t, clk, sxOpts{})
	sxRun(a, ha, `/test addworld("w1", "", "h", "1")`)
	a.SetCurrentWorld("w1")
	a.RecordInput("visible command")
	a.RecordInputAttr("hunter2", "g")
	a.Input().Set("sw0rdfish", -1)
	a.RecallInput(-1, 0, "sw0rdfish")
	a.SetLocalEcho("w1", false)
	data := sxSave(t, a)
	for _, secret := range []string{"hunter2", "sw0rdfish"} {
		if bytes.Contains(data, []byte(secret)) {
			t.Errorf("%q is in the saved state", secret)
		}
	}
	b, _ := sxNew(t, clk, sxOpts{})
	sxLoad(t, b, data)
	if b.Input().Text() != "" {
		t.Errorf("input line %q", b.Input().Text())
	}
	if b.LocalEcho("w1") {
		t.Error("the world's no-echo state was lost")
	}
	if got := b.HistoryLines(HistInput, ""); len(got) < 2 || got[0] != "visible command" {
		t.Errorf("history %q", got)
	}
	// with echo on, the line being typed travels
	a.SetLocalEcho("w1", true)
	a.Input().Set("half typed", 4)
	b2, _ := sxNew(t, clk, sxOpts{})
	sxLoad(t, b2, sxSave(t, a))
	if b2.Input().Text() != "half typed" || b2.Input().Cursor() != 4 {
		t.Errorf("input line %q at %d", b2.Input().Text(), b2.Input().Cursor())
	}
}

func TestStateReplacesAndNeverLowersRestriction(t *testing.T) {
	clk := sxClock()
	a, ha := sxNew(t, clk, sxOpts{})
	sxRun(a, ha, `/restrict shell`)
	data := sxSave(t, a)
	b, _ := sxNew(t, clk, sxOpts{})
	sxLoad(t, b, data)
	if b.Restriction() != RestrictShell {
		t.Errorf("restriction %d", b.Restriction())
	}
	c, _ := sxNew(t, clk, sxOpts{})
	c.SetRestriction(RestrictWorld)
	sxLoad(t, c, data)
	if c.Restriction() != RestrictWorld {
		t.Errorf("a load lowered the restriction to %d", c.Restriction())
	}
}

func TestStateFilesAndStreams(t *testing.T) {
	dir := t.TempDir()
	clk := sxClock()
	a, ha := sxBusy(t, clk, sxOpts{files: dir})
	if a.LogCount() != 1 {
		t.Fatal("no log")
	}
	sxRun(a, ha, `/test p := tfopen("echo piped", "p")`)
	data := sxSave(t, a)
	b, hb := sxNew(t, clk, sxOpts{})
	sxLoad(t, b, data)
	if b.LogCount() != 1 {
		t.Errorf("nlog() = %d", b.LogCount())
	}
	b.RecordLocalLine("after the restart")
	if got, _ := os.ReadFile(filepath.Join(dir, "global.log")); !strings.Contains(string(got), "after the restart") {
		t.Errorf("log: %q", got)
	}
	// the read stream continues where it was, the write stream where it was, the append stream appends
	for _, l := range []string{`/test tfread(r, x)`, `/test echo(x)`, `/test tfwrite(w, "w2")`, `/test tfwrite(a, "a2")`, `/test tfclose(w)`, `/test tfclose(a)`, `/test echo(tfread(q, z) . z)`} {
		sxRun(b, hb, l)
	}
	if got := sxRun(b, hb, `/test echo(x)`); got != "r2\n" {
		t.Errorf("read stream resumed at %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "write.txt")); string(got) != "w1\nw2\n" {
		t.Errorf("write.txt = %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "app.txt")); string(got) != "a1\na2\n" {
		t.Errorf("app.txt = %q", got)
	}
	// the pipe is gone, its handle is not reused
	if got := sxRun(b, hb, `/test echo(tfread(p, x))`); !strings.Contains(got, "bad handle") {
		t.Errorf("pipe handle: %q", got)
	}
	if got := sxRun(b, hb, `/test echo(tfopen("", "q"))`); got != "6\n" {
		t.Errorf("next handle %q", got)
	}
	// a log that cannot be reopened is dropped with a warning
	c, hc := sxNew(t, clk, sxOpts{})
	os.RemoveAll(dir)
	sxLoad(t, c, data)
	if c.LogCount() != 0 || len(hc.warns) == 0 {
		t.Errorf("nlog %d, warnings %q", c.LogCount(), hc.warns)
	}
}

// syncHost collects /echo output from the runner's goroutine.
type syncHost struct {
	mu  sync.Mutex
	out []string
	ch  chan string
}

func (h *syncHost) Echo(s string) {
	h.mu.Lock()
	h.out = append(h.out, s)
	h.mu.Unlock()
	select {
	case h.ch <- s:
	default:
	}
}
func (h *syncHost) Send(w, s string) bool { return true }

func TestStateProcessesRunUnderARunner(t *testing.T) {
	a := New(&syncHost{ch: make(chan string, 16)}) // the real clock
	a.ExecFull(`/repeat -0.15 3 /echo tick`)
	a.ExecFull(`/repeat -60 i /echo never`)
	data := sxSave(t, a)
	h := &syncHost{ch: make(chan string, 16)}
	b := New(h)
	r := NewRunner(b)
	defer r.Close()
	var err error
	r.Do(func(in *Interp) { err = in.LoadState(bytes.NewReader(data)) })
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		select {
		case s := <-h.ch:
			if s != "tick" {
				t.Fatalf("got %q", s)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("tick %d never came", i)
		}
	}
	var ps string
	r.Do(func(in *Interp) { in.ExecFull(`/ps -s`) })
	h.mu.Lock()
	ps = strings.Join(h.out[len(h.out)-1:], "")
	h.mu.Unlock()
	if ps != "2" {
		t.Errorf("/ps -s after the repeat finished: %q", ps)
	}
}

func TestStateLoadReplacesWhatItCovers(t *testing.T) {
	clk := sxClock()
	a, _ := sxNew(t, clk, sxOpts{noEnv: true})
	data := sxSave(t, a)
	b, hb, _ := sxTarget(t)
	sxLoad(t, b, data)
	if sxSave(t, b); b.FindMacro("mymac") != nil || b.Var("mine") != "" || len(b.procs) != 0 || len(b.Worlds()) != 0 {
		t.Error("the old state survived a load")
	}
	if got := sxRun(b, hb, `/def after = /echo after`); got != "" {
		t.Error(got)
	}
	if m := b.FindMacro("after"); m == nil || m.ID != 4 { // numbering restarts with the saved state (the two ^W/^R/^V keys + 1)
		t.Errorf("after = %+v", m)
	}
}

func TestStateAllValueKinds(t *testing.T) {
	clk := sxClock()
	a, ha := sxNew(t, clk, sxOpts{noEnv: true})
	for _, l := range []string{`/test nan := 0.0/0.0`, `/test inf := 1e308 * 10`, `/test big := 9223372036854775807`, `/test neg := -9223372036854775807 - 1`,
		`/test f := 3.14159265358979`, `/test d := 0.000001`, `/test s := "tab\there \"quoted\" {braces} %percent"`, `/test u := "é世"`, `/set e1=`} {
		sxRun(a, ha, l)
	}
	a.globals["rawbytes"] = Str("a\xffb\x80")
	b, hb := sxNew(t, clk, sxOpts{noEnv: true})
	sxLoad(t, b, sxSave(t, a))
	for n, v := range a.globals {
		if _, ok := specialVars[n]; ok {
			continue
		}
		got, ok := b.globals[n]
		if !ok || got.String() != v.String() || got.k != v.k || got.tt != v.tt || got.en != v.en {
			t.Errorf("%s: %+v became %+v", n, v, got)
		}
	}
	if x, y := sxRun(a, ha, `/listvar -v`), sxRun(b, hb, `/listvar -v`); x != y {
		t.Errorf("/listvar -v differs")
	}
	_ = fmt.Sprint()
}
