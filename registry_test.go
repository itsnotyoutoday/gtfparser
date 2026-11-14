package gtfparser

import (
	"strings"
	"testing"
)

func TestPlaceholders(t *testing.T) {
	in := New(Console(&strings.Builder{}))
	ph := in.Placeholders()
	seen := map[string]bool{}
	for _, n := range ph {
		seen[n] = true
	}
	// implemented builtins are not listed; stubs are
	if seen["/echo"] || seen["strlen()"] {
		t.Fatalf("implemented builtin listed as placeholder: %v", ph)
	}
	for _, n := range ph {
		if strings.HasPrefix(n, "/") {
			if _, ok := builtinCmds[n[1:]]; ok {
				t.Errorf("%s is implemented but listed", n)
			}
		} else if _, ok := builtinFuncs[strings.TrimSuffix(n, "()")]; ok {
			t.Errorf("%s is implemented but listed", n)
		}
	}
	// replacing a stub through the public API does not rewrite the list
	if len(ph) > 0 {
		ph[0] = "x"
		if in.Placeholders()[0] == "x" {
			t.Error("Placeholders returned internal slice")
		}
	}
}
