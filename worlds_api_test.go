package gtfparser

import (
	"errors"
	"strings"
	"testing"
)

type worldEvents struct {
	tHost
	ev []string
}

func (h *worldEvents) Notify(e Event) { h.ev = append(h.ev, e.Kind+" "+strings.Join(e.Args, " ")) }

func TestWorldAPI(t *testing.T) {
	h := &worldEvents{}
	in := New(h)
	w, err := in.PutWorld(WorldInfo{Name: "alpha", Host: "a.example", Port: "4000", Character: "bob", Password: "pw"})
	if err != nil || w.Rev == 0 {
		t.Fatalf("put: %v %+v", err, w)
	}
	r1 := w.Rev
	w2, err := in.PutWorld(WorldInfo{Name: "ALPHA", Host: "b.example", Port: "4001"})
	if err != nil || w2.Rev <= r1 || w2.Character != "" || w2.Password != "" {
		t.Fatalf("replace must clear omitted fields and bump rev: %v %+v", err, w2)
	}
	if got, _ := in.World("alpha"); got.Rev != w2.Rev {
		t.Fatal("World must report the stored rev")
	}
	for _, bad := range []WorldInfo{{Name: ""}, {Name: "a b", Host: "h", Port: "1"}, {Name: "(x)"}, {Name: "default"}, {Name: "n", Host: "h"}} {
		if _, err := in.PutWorld(bad); !errors.Is(err, ErrWorldInvalid) {
			t.Errorf("%+v: want ErrWorldInvalid, got %v", bad, err)
		}
	}
	if _, err := in.RenameWorld("alpha", "beta"); err != nil {
		t.Fatal(err)
	}
	if _, err := in.PutWorld(WorldInfo{Name: "gamma"}); err != nil {
		t.Fatal(err)
	}
	if _, err := in.RenameWorld("beta", "gamma"); !errors.Is(err, ErrWorldExists) {
		t.Fatalf("rename onto existing: %v", err)
	}
	if _, err := in.RenameWorld("nope", "x"); !errors.Is(err, ErrWorldNotFound) {
		t.Fatal(err)
	}
	if err := in.DeleteWorld("beta"); err != nil {
		t.Fatal(err)
	}
	if err := in.DeleteWorld("beta"); !errors.Is(err, ErrWorldNotFound) {
		t.Fatal(err)
	}
	w3, _ := in.PutWorld(WorldInfo{Name: "beta"})
	if w3.Rev <= w2.Rev {
		t.Fatal("revisions must not be reused")
	}
	want := "world add alpha|world update ALPHA|renameworld ALPHA beta|world add gamma|world remove beta|world add beta"
	if got := strings.Join(h.ev, "|"); got != want {
		t.Fatalf("events:\n got %s\nwant %s", got, want)
	}
}

func TestLocalWorldCommandsNotify(t *testing.T) {
	h := &worldEvents{}
	in := New(h)
	in.ExecFull(`/test addworld("w1","","h","1")`)
	in.ExecFull(`/test addworld("w1","","h","2")`)
	in.ExecFull("/renameworld w1 w2")
	in.ExecFull("/unworld w2")
	got := strings.Join(h.ev, "|")
	if want := "world add w1|world update w1|renameworld w1 w2|world remove w2"; got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestWorldDefinitionsAndFile(t *testing.T) {
	in := New(&tHost{})
	in.PutWorld(WorldInfo{Name: "w", Host: "h", Port: "1", Character: "c", Password: `p"w`})
	d := in.WorldDefinitions()
	if len(d) != 1 || !strings.Contains(d[0], `addworld("w"`) {
		t.Fatalf("%v", d)
	}
	if _, ok := in.WorldFile(); ok {
		t.Fatal("no WORLDFILE macro defined yet")
	}
	in.Exec("/def WORLDFILE = ~/x.world")
	if f, ok := in.WorldFile(); !ok || !strings.HasSuffix(f, "/x.world") {
		t.Fatalf("%q %v", f, ok)
	}
}
