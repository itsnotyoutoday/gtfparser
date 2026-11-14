package gtfparser

import "testing"

func TestFitScreen(t *testing.T) {
	h, in := newKB(t)
	h.ln = 10
	in.setGlobal("visual", Str("on"))
	in.setGlobal("isize", Int(3))
	in.setGlobal("status_height", Int(1))
	in.FitScreen()
	if in.Var("isize") != "3" || call(t, in, "winlines").Int() != 6 {
		t.Fatalf("fits: isize=%s", in.Var("isize"))
	}
	in.setGlobal("isize", Int(9))
	in.FitScreen()
	if in.Var("isize") != "7" {
		t.Fatalf("isize shrinks to leave 2 output lines: %s", in.Var("isize"))
	}
	h.ln = 3
	in.setGlobal("isize", Int(3))
	in.setGlobal("status_height", Int(2))
	h.out = nil
	in.FitScreen()
	if in.Var("visual") != "off" || len(h.out) != 1 || h.out[0] != "!Screen is too small for visual mode." {
		t.Fatalf("%s %v", in.Var("visual"), h.out)
	}
	// a variable change triggers it
	h.ln = 8
	in.setGlobal("visual", Str("on"))
	in.setGlobal("isize", Int(3))
	in.setGlobal("status_height", Int(1))
	in.PollVars()
	in.setGlobal("status_height", Int(5))
	in.PollVars()
	if in.Var("isize") != "1" || in.Var("status_height") != "5" {
		t.Fatalf("isize=%s sh=%s", in.Var("isize"), in.Var("status_height"))
	}
}

func TestLastKeyTime(t *testing.T) {
	_, in := newKB(t)
	if !in.LastKeyTime().IsZero() {
		t.Fatal("no key yet")
	}
	in.KeyPressed("a")
	if in.LastKeyTime().IsZero() {
		t.Fatal("key time")
	}
}
