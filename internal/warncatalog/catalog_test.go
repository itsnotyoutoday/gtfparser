// Package warncatalog only holds a test: it checks that the diagnostics catalog used by
// scripts/warncheck.py (testdata/warnings/catalog.tsv) is well formed. No tf binary is needed.
package warncatalog

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var cols = []string{"key", "kind", "file", "line", "func", "template", "context", "trigger", "status", "case", "note"}

const dir = "../../testdata/warnings"

func readLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		out = append(out, sc.Text())
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestCatalog(t *testing.T) {
	lines := readLines(t, filepath.Join(dir, "catalog.tsv"))
	if len(lines) < 2 {
		t.Fatal("catalog.tsv is empty")
	}
	if got := strings.Split(lines[0], "\t"); strings.Join(got, ",") != strings.Join(cols, ",") {
		t.Fatalf("header = %v, want %v", got, cols)
	}
	kinds := map[string]bool{"E": true, "W": true, "O": true, "T": true, "H": true, "I": true}
	contexts := map[string]bool{"eprefix": true, "none": true, "custom": true}
	caseRe := regexp.MustCompile(`(?m)^;;; (\S+)`)
	cases := map[string]bool{}
	files, _ := filepath.Glob(filepath.Join(dir, "*.tf"))
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range caseRe.FindAllStringSubmatch(string(b), -1) {
			if cases[m[1]] {
				t.Errorf("case %s defined twice", m[1])
			}
			cases[m[1]] = true
		}
	}
	seen := map[string]bool{}
	testable := 0
	for i, l := range lines[1:] {
		if strings.TrimSpace(l) == "" {
			continue
		}
		p := strings.Split(l, "\t")
		if len(p) != len(cols) {
			t.Errorf("row %d: %d fields, want %d: %.60s", i+2, len(p), len(cols), l)
			continue
		}
		key := p[0]
		if key == "" || seen[key] {
			t.Errorf("row %d: empty or duplicate key %q", i+2, key)
		}
		seen[key] = true
		if !kinds[p[1]] {
			t.Errorf("%s: bad kind %q", key, p[1])
		}
		if !contexts[p[6]] && !strings.HasPrefix(key, "ctx.") {
			t.Errorf("%s: bad context %q", key, p[6])
		}
		if p[5] == "" {
			t.Errorf("%s: empty template", key)
		}
		switch p[8] {
		case "testable":
			testable++
			if p[9] != key || !cases[key] {
				t.Errorf("%s: testable but no case %q in testdata/warnings/*.tf", key, p[9])
			}
		case "untestable":
			if p[10] == "" || p[10] == "TODO" {
				t.Errorf("%s: untestable without a reason", key)
			}
			if cases[key] {
				t.Errorf("%s: marked untestable but a case exists", key)
			}
		default:
			t.Errorf("%s: bad status %q", key, p[8])
		}
	}
	for c := range cases {
		if !seen[c] {
			t.Errorf("case %s has no catalog entry", c)
		}
	}
	if len(seen) < 300 {
		t.Errorf("catalog has only %d entries", len(seen))
	}
	t.Logf("%d entries, %d testable, %d cases", len(seen), testable, len(cases))
}

func TestKnown(t *testing.T) {
	cat := map[string]bool{}
	for _, l := range readLines(t, filepath.Join(dir, "catalog.tsv"))[1:] {
		if p := strings.SplitN(l, "\t", 2); len(p) > 0 {
			cat[p[0]] = true
		}
	}
	classes := map[string]bool{"body": true, "context": true, "missing": true, "extra": true, "invalid": true, "*": true}
	seen := map[string]bool{}
	for _, l := range readLines(t, filepath.Join(dir, "known.tsv")) {
		if strings.TrimSpace(l) == "" || strings.HasPrefix(l, "#") {
			continue
		}
		p := strings.Split(l, "\t")
		if len(p) < 3 || p[2] == "" {
			t.Errorf("known.tsv row needs key, class and reason: %q", l)
			continue
		}
		if p[0] != "*" && !cat[p[0]] {
			t.Errorf("known.tsv: %s is not in the catalog", p[0])
		}
		if !classes[p[1]] {
			t.Errorf("known.tsv: bad class %q for %s", p[1], p[0])
		}
		if k := p[0] + "/" + p[1]; seen[k] {
			t.Errorf("known.tsv: duplicate %s", k)
		} else {
			seen[k] = true
		}
	}
}
