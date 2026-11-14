package gtfparser

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

type denyAll struct{}

func (denyAll) Allow(op Op, detail string) error { return fmt.Errorf("denied by policy (%s)", op) }

func TestRestrictLevelsAndPolicy(t *testing.T) {
	h := &tHost{}
	in := New(h)
	in.Warn = func(s string) { h.out = append(h.out, "!"+s) }
	in.ExecFull("/sh echo hi")
	if len(h.out) == 0 || h.out[0] != "hi" {
		t.Fatalf("unrestricted /sh should run: %v", h.out)
	}
	h.out = nil
	in.ExecFull("/restrict shell")
	in.ExecFull("/sh echo hi")
	if !strings.Contains(strings.Join(h.out, "|"), "!/sh: restricted") {
		t.Fatalf("restricted /sh must say so: %v", h.out)
	}
	h.out = nil
	in.ExecFull("/restrict none")
	if !strings.Contains(strings.Join(h.out, "|"), "cannot be lowered") || in.Restriction() != RestrictShell {
		t.Fatalf("level must not lower: %v %d", h.out, in.Restriction())
	}
	// a client policy vetoes independently of /restrict
	h2 := &tHost{}
	in2 := New(h2, WithPolicy(denyAll{}))
	in2.Warn = func(s string) { h2.out = append(h2.out, "!"+s) }
	in2.ExecFull("/load /etc/hosts")
	if !strings.Contains(strings.Join(h2.out, "|"), "denied by policy") {
		t.Fatalf("policy ignored: %v", h2.out)
	}
}

func TestOverrideAndDisable(t *testing.T) {
	h := &tHost{}
	in := New(h)
	in.Warn = func(s string) { h.out = append(h.out, "!"+s) }
	in.RegisterCommand("hello", func(in *Interp, a string) Value { in.echo("hi " + a); return Int(1) })
	in.ExecFull("/hello world")
	in.DisableCommand("hello")
	in.ExecFull("/hello again")
	in.RegisterFunction("twice", func(in *Interp, a []Value) (Value, error) { return Int(a[0].Int() * 2), nil })
	in.ExecFull("/echo $[twice(21)]")
	want := []string{"hi world", "!/hello: disabled", "42"}
	if strings.Join(h.out, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v want %v", h.out, want)
	}
	// a registered override of a builtin wraps the original
	orig, _ := in.Command("echo")
	in.RegisterCommand("echo", func(in *Interp, a string) Value { return orig(in, "<"+a+">") })
	h.out = nil
	in.ExecFull("/@echo x")
	if len(h.out) != 1 || h.out[0] != "<x>" {
		t.Fatalf("override: %v", h.out)
	}
}

func TestUnimplementedFailsLoudly(t *testing.T) {
	h := &tHost{}
	in := New(h)
	in.Warn = func(s string) { h.out = append(h.out, "!"+s) }
	if v := in.ExecFull("/connect somewhere"); v.Truth() {
		t.Fatal("unsupported command must fail")
	}
	if len(h.out) == 0 || !strings.Contains(h.out[0], "not supported by this client") {
		t.Fatalf("must warn: %v", h.out)
	}
}

func TestLimitsAndInterrupt(t *testing.T) {
	h := &tHost{}
	in := New(h, WithLimits(Limits{MaxRecur: 10, MaxInstr: 5000}))
	in.Warn = func(s string) { h.out = append(h.out, "!"+s) }
	in.Exec("/def loop = /while (1) /test 1%; /done")
	in.ExecFull("/loop")
	if !strings.Contains(strings.Join(h.out, "|"), "max_instr") {
		t.Fatalf("runaway loop not stopped: %v", h.out)
	}
	h.out = nil
	in.Exec("/def rec = /rec")
	in.ExecFull("/rec")
	if !strings.Contains(strings.Join(h.out, "|"), "too many nested") {
		t.Fatalf("recursion not limited: %v", h.out)
	}
	// scripts cannot raise the ceiling
	in.ExecFull("/set max_instr=999999999")
	if in.maxInstr() != 5000 {
		t.Fatalf("ceiling exceeded: %d", in.maxInstr())
	}
}

func TestRunnerInterruptAndConcurrency(t *testing.T) {
	var mu sync.Mutex
	var out []string
	h := &lockedHost{mu: &mu, out: &out}
	r := NewRunner(New(h))
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ { // many goroutines, one interpreter, no races
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r.Do(func(in *Interp) { in.ExecFull(fmt.Sprintf("/set n%d=%d", i, i)) })
		}(i)
	}
	wg.Wait()
	r.Do(func(in *Interp) { in.Exec("/def spin = /while (1) /test 1%; /done") })
	done := make(chan struct{})
	go func() { r.Do(func(in *Interp) { in.ExecFull("/spin") }); close(done) }()
	time.Sleep(30 * time.Millisecond)
	r.Interrupt()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("interrupt did not stop the loop")
	}
	r.Close()
}

type lockedHost struct {
	mu  *sync.Mutex
	out *[]string
}

func (h *lockedHost) Echo(s string)         { h.mu.Lock(); *h.out = append(*h.out, s); h.mu.Unlock() }
func (h *lockedHost) Send(w, s string) bool { return true }

func TestTwoInterpsDoNotInterfere(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			h := &tHost{}
			in := New(h)
			for j := 0; j < 200; j++ {
				in.ExecFull(fmt.Sprintf("/set x=%d", j))
				in.ExecFull("/test regmatch('(\\\\d+)', '%{x}')")
				in.ExecFull("/echo $[%{x} =/ '*5*']")
			}
		}(i)
	}
	wg.Wait()
}
