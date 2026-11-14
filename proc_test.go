package gtfparser

import (
	"testing"
	"time"
)

func TestRepeatSchedule(t *testing.T) {
	h := &tHost{}
	in := New(h)
	clock := time.Unix(1000, 0)
	in.Now = func() time.Time { return clock }
	in.Exec("/repeat -2 3 /echo tick")
	in.RunDue()
	if len(h.out) != 0 {
		t.Fatalf("ran before its time: %v", h.out)
	}
	next, ok := in.NextDue()
	if !ok || !next.Equal(clock.Add(2*time.Second)) {
		t.Fatalf("NextDue = %v %v", next, ok)
	}
	for i := 0; i < 5; i++ {
		clock = clock.Add(2 * time.Second)
		in.RunDue()
	}
	if len(h.out) != 3 {
		t.Fatalf("want 3 ticks, got %v", h.out)
	}
	if _, ok := in.NextDue(); ok {
		t.Fatal("repeat should be finished")
	}
}

func TestKill(t *testing.T) {
	h := &tHost{}
	in := New(h)
	in.Exec("/repeat -60 i /echo x")
	if len(in.procs) != 1 {
		t.Fatal("no proc")
	}
	in.Exec("/kill 1")
	if len(in.procs) != 0 {
		t.Fatal("not killed")
	}
}
