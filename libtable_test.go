package gtfparser

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestLibraryMacroTable loads TinyFugue's stdlib.tf and compares every macro
// it defines with the table real tf produced (testdata/golden/libtable.txt).
// Set GTF_TF_LIB to the tf library directory (lib/tf) to run it.
func TestLibraryMacroTable(t *testing.T) {
	lib := os.Getenv("GTF_TF_LIB")
	if lib == "" {
		lib = os.Getenv("TF_LIB")
	}
	if lib == "" {
		t.Skip("GTF_TF_LIB not set")
	}
	golden, err := os.ReadFile(filepath.Join("testdata", "golden", "libtable.txt"))
	if err != nil {
		t.Fatal(err)
	}
	h := &tHost{}
	in := New(h)
	in.Warn = func(string) {}
	in.LibDirs = []string{lib}
	if err := in.LoadFile(filepath.Join(lib, "stdlib.tf")); err != nil {
		t.Fatal(err)
	}
	ws := regexp.MustCompile(`\s+`)
	var got []string
	for _, m := range in.Macros() {
		if strings.HasPrefix(m.Name, "echo") && m.Name == "echo" && false {
			continue
		}
		got = append(got, strings.TrimSpace(ws.ReplaceAllString(strings.ReplaceAll(m.printDef(false), "\t", " "), " ")))
	}
	want := strings.Split(strings.TrimRight(string(golden), "\n"), "\n")
	// real tf listed the table with the harness's own echo/E macros removed
	var filtered []string
	for _, g := range got {
		filtered = append(filtered, g)
	}
	wi := map[string]int{}
	for _, w := range want {
		wi[w]++
	}
	missing := 0
	for _, g := range filtered {
		if wi[g] > 0 {
			wi[g]--
		}
	}
	for w, n := range wi {
		if n > 0 {
			missing++
			if missing <= 5 {
				t.Errorf("macro in real tf's table but not ours: %.140s", w)
			}
		}
	}
	if missing > 0 {
		t.Errorf("%d macros differ (want %d, got %d)", missing, len(want), len(filtered))
	}
}
