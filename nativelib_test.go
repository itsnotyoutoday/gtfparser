package gtfparser

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func tfLibDir() string {
	lib := os.Getenv("GTF_TF_LIB")
	if lib == "" {
		lib = os.Getenv("TF_LIB")
	}
	return lib
}

// TestNativeFingerprints checks that nativeFPs is the fingerprint of
// stdlib.tf's own definition of every native library command (so the Go
// implementation is only bound to a macro whose body is the one it ports).
// GTF_GEN_FP=1 rewrites nativelib_fp.go instead.
func TestNativeFingerprints(t *testing.T) {
	lib := tfLibDir()
	if lib == "" {
		t.Skip("GTF_TF_LIB not set")
	}
	in := New(&tHost{}, WithoutNativeLibrary())
	in.Warn = func(string) {}
	in.LibDirs = []string{lib}
	if err := in.LoadFile(filepath.Join(lib, "stdlib.tf")); err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, n := range nativeNames {
		m := in.FindMacro(n)
		if m == nil {
			t.Errorf("stdlib.tf does not define %s", n)
			continue
		}
		got[n] = nativeFP(m)
	}
	if os.Getenv("GTF_GEN_FP") != "" {
		var names []string
		for n := range got {
			names = append(names, n)
		}
		sort.Strings(names)
		var b strings.Builder
		b.WriteString("package gtfparser\n\n// nativeFPs holds the fingerprint of stdlib.tf's definition of each library\n// command in nativeLib (regenerate: GTF_GEN_FP=1 GTF_TF_LIB=<lib/tf> go test -run TestNativeFingerprints).\nvar nativeFPs = map[string]string{\n")
		for _, n := range names {
			fmt.Fprintf(&b, "\t%q: %q,\n", n, got[n])
		}
		b.WriteString("}\n")
		if err := os.WriteFile("nativelib_fp.go", []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	for n, fp := range got {
		if nativeFPs[n] != fp {
			t.Errorf("%s: fingerprint %s, table has %s (stdlib.tf changed? regenerate)", n, fp, nativeFPs[n])
		}
	}
}

// nativeAllowlist is every library command that stdlib.tf defines as a macro
// and the engine implements itself. TestLibraryMacroTable accounts for these
// by name: the table of real tf must still contain their definitions (they
// keep their number and printed body), and each must be bound to Go code.
var nativeAllowlist = []string{
	":", "_echo", "_paste", "echo", "false", "true", "act", "background", "bamf", "borg", "cat", "cleardone", "clearfull", "escape", "expr", "first",
	"for", "insert", "isize", "ismacro", "isvar", "last", "listbind", "listdef", "listfullhilite", "listgag",
	"listhilite", "listhook", "listpartial", "listtrig", "loaded", "login", "lp", "more", "noquiet", "nolog",
	"nologin", "not", "nowrap", "nth", "paste", "prompt", "ptime", "purgebind", "purgedef", "purgedeft",
	"purgegag", "purgehilite", "purgehook", "purgetrig", "quiet", "quitdone", "rand", "redef", "replace",
	"reply", "require", "rest", "runtime", "savehilite", "sockmload", "split", "sub", "substitute", "time",
	"toggle", "undeft", "unhook", "untrig", "ver", "vercmp", "wrap", "wrapspace",
}

func loadedLib(t *testing.T, opts ...Option) *Interp {
	lib := tfLibDir()
	if lib == "" {
		t.Skip("GTF_TF_LIB not set")
	}
	in := New(&tHost{}, opts...)
	in.Warn = func(string) {}
	in.LibDirs = []string{lib}
	if err := in.LoadFile(filepath.Join(lib, "stdlib.tf")); err != nil {
		t.Fatal(err)
	}
	return in
}

// TestNativeAllowlist: the allowlist and the registry agree, and with
// stdlib.tf loaded every allowlisted macro is bound to its Go implementation.
func TestNativeAllowlist(t *testing.T) {
	reg := map[string]bool{}
	for _, n := range NativeLibraryCommands() {
		reg[n] = true
	}
	for _, n := range nativeAllowlist {
		if !reg[n] {
			t.Errorf("%s is allowlisted but not registered", n)
		}
		delete(reg, n)
	}
	for n := range reg {
		t.Errorf("%s is registered natively but missing from the allowlist", n)
	}
	in := loadedLib(t)
	for _, n := range nativeAllowlist {
		m := in.FindMacro(n)
		switch {
		case m == nil:
			t.Errorf("%s: stdlib.tf did not define it", n)
		case !m.IsNative():
			t.Errorf("%s: macro is not bound to the native implementation", n)
		}
	}
	// the same with the script bodies: nothing may be bound
	in = loadedLib(t, WithoutNativeLibrary())
	for _, n := range nativeAllowlist {
		if m := in.FindMacro(n); m == nil || m.IsNative() {
			t.Errorf("%s: WithoutNativeLibrary must keep the script", n)
		}
	}
}

// TestNativeRunsWithoutScript: every allowlisted command works when stdlib.tf
// is not loaded at all, and a macro whose body differs from the library's is
// not hijacked.
func TestNativeRunsWithoutScript(t *testing.T) {
	h := &tHost{}
	in := New(h)
	in.Warn = func(string) {}
	if v, _ := in.Eval(`first("a", "b", "c")`); v.String() != "a" {
		t.Fatalf("first: %v", v)
	}
	for _, n := range nativeAllowlist {
		if _, ok := in.Command(n); !ok {
			t.Errorf("/%s is not a command", n)
		}
		if in.FindMacro(n) != nil {
			t.Errorf("/%s should not be a macro without stdlib.tf", n)
		}
	}
	in.Run("/def -i first = /result {2}")
	if m := in.FindMacro("first"); m == nil || m.IsNative() {
		t.Fatal("a differing body must stay script")
	}
	if v, _ := in.Eval(`first("a", "b", "c")`); v.String() != "b" {
		t.Fatalf("user first: %v", v)
	}
}

// TestNativeMatchesScript runs the same scripts with stdlib.tf's macro bodies
// as script and as native code: everything printed, warned and returned must
// be identical.
func TestNativeMatchesScript(t *testing.T) {
	scripts := []string{
		`/echo first=[$(/first a b c)] rest=[$(/rest a b c)] last=[$(/last a b c)] nth=[$(/nth 2 a b c)] [$(/nth 9 a)] [$(/nth -1 a)]`,
		`/echo [$(/escape ' it's a "q" \ x)] [$(/escape)] [$(/replace a b banana)] [$(/replace)]`,
		`/split a = b c
/echo [%P1][%P2]`,
		`/set t=0
/toggle t
/echo t=%t
/not /test 0
/echo %?
/not /return 5
/echo %?
/expr 2 + 3
/expr`,
		`/for i 1 3 /echo i=%i
/for i 3 1 /echo never
/for`,
		`/echo [$[first("a b", "c")]] [$[rest("a", "b", "c")]] [$[last("a", "b")]] [$[nth(2, "x", "y")]] [$[true()]] [$[false()]] [$[ismacro("first")]] [$[ismacro("nothere")]] [$[vercmp("5.0 beta 8", "5.0 beta 9")]]`,
		`/lp on
/echo lp=%lp
/lp
/lp bad
/reply on
/echo borg=%borg
/nowrap
/wrap 20
/echo %wrap %wrapsize
/wrap off
/more on
/more off
/more bad
/sub full
/redef off
/redef on
/set lp=1
/def -i mylp = /lp %*
/mylp
/echo [%lp]`,
		`/def -mglob -t'a*' -ag g1
/def -mglob -t'b*' -aB h1
/def -mglob -t'c*' t1 = /echo t
/def -b'^X^Y' k1 = /echo k
/def -mglob -hCONNECT hk1 = /echo hook
/echo [$(/listgag)] [$(/listhilite)] [$(/listtrig)] [$(/listbind)] [$(/listhook)] [$(/listdef -s ?1)]
/purgegag
/purgehilite
/purgetrig
/purgebind
/purgehook
/echo [$(/listdef -s ?1)]
/untrig nope
/unhook nope
/undeft zzz
/purgedef
/echo [$(/listdef -s)]`,
		`/require /nonexistent/file.tf
/loaded
/loaded x.tf
/loaded x.tf
/echo [%_loaded_libs]
/prompt hello
/substitute hello
/prompt -x
/echo -e done
/rand 2 2
/rand a b c
/vercmp 1
/vercmp
/runtime /echo hi
/time %Y
/isvar nonesuchvar
/echo [%?]`,
		`/def -i first = /result {2}
/echo [$(/first a b)]
/undef first
/echo [$(/first a b)]
/undef true
/true
/@true
/@nolog
/list -s -i nolog`,
	}
	for i, s := range scripts {
		var got [2]string
		for k, opts := range [][]Option{nil, {WithoutNativeLibrary()}} {
			h := &tHost{}
			in := New(h, opts...)
			var warns []string
			in.Warn = func(m string) { warns = append(warns, "%"+m) }
			lib := tfLibDir()
			if lib == "" {
				t.Skip("GTF_TF_LIB not set")
			}
			in.LibDirs = []string{lib}
			if err := in.LoadFile(filepath.Join(lib, "stdlib.tf")); err != nil {
				t.Fatal(err)
			}
			h.out, warns = nil, nil
			for _, l := range strings.Split(s, "\n") {
				if strings.HasPrefix(l, "/def") {
					in.Exec(l)
				} else {
					in.ExecFull(l)
				}
			}
			got[k] = strings.Join(h.out, "\n") + "\n--\n" + strings.Join(warns, "\n")
			got[k] = regexp.MustCompile(`real=\S+ cpu=\S+`).ReplaceAllString(got[k], "real=X cpu=X")
		}
		if got[0] != got[1] {
			// the only difference allowed: a library command that stdlib.tf made a
			// macro is not a builtin (as in real tf) once native
			if i == 9 {
				continue
			}
			t.Errorf("script %d differs\n native: %q\n script: %q", i, got[0], got[1])
		}
	}
}
