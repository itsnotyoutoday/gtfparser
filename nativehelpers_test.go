package gtfparser

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestHelperCommandsMatchScript: the exported helpers for client-owned
// library commands behave like stdlib.tf's macros.
func TestHelperCommandsMatchScript(t *testing.T) {
	cases := [][2][]string{
		// {script macro line, helper line} pairs run against the same inputs
		{{"/addworld -T'tiny' w1 me pw host.example 4000", "/addworld -xe w2 host2 99 f.tf", "/addworld default me pw", "/addworld -pp w3 a b c d e f", "/addworld", "/addworld -Z w4"},
			{"/xaddworld -T'tiny' w1 me pw host.example 4000", "/xaddworld -xe w2 host2 99 f.tf", "/xaddworld default me pw", "/xaddworld -pp w3 a b c d e f", "/xaddworld", "/xaddworld -Z w4"}},
		{{"/addtiny t1 u p h 1", "/addlp l1 h 2", "/addlpp l2 h 3", "/adddiku d1 u p h 4", "/addtelnet n1 h 5"},
			{"/xaddtiny t1 u p h 1", "/xaddlp l1 h 2", "/xaddlpp l2 h 3", "/xadddiku d1 u p h 4", "/xaddtelnet n1 h 5"}},
		{{"/set kecho=0", "/kecho on", "/echo [%kecho]", "/kecho hello there", "/echo [%kecho] [%kprefix]", "/kecho off", "/echo [%kecho]", "/mecho all", "/echo [%mecho]", "/mecho 2", "/qecho 2", "/echo [%qecho] [%qprefix]", "/qecho"},
			{"/set kecho=0", "/xkecho on", "/echo [%kecho]", "/xkecho hello there", "/echo [%kecho] [%kprefix]", "/xkecho off", "/echo [%kecho]", "/xmecho all", "/echo [%mecho]", "/xmecho 2", "/xqecho 2", "/echo [%qecho] [%qprefix]", "/xqecho"}},
		{{"/visual on", "/echo [%visual]", "/gpri 5", "/echo [%gpri]", "/hpri 3", "/echo [%hpri]", "/lpquote on", "/shpause 1", "/echo [%shpause]", "/visual", "/echo [%visual]"},
			{"/xvisual on", "/echo [%visual]", "/xgpri 5", "/echo [%gpri]", "/xhpri 3", "/echo [%hpri]", "/xlpquote on", "/xshpause 1", "/echo [%shpause]", "/xvisual", "/echo [%visual]"}},
	}
	lib := tfLibDir()
	if lib == "" {
		t.Skip("GTF_TF_LIB not set")
	}
	for ci, c := range cases {
		var got [2]string
		for k := 0; k < 2; k++ {
			h := &tHost{}
			in := New(h)
			var warns []string
			in.Warn = func(m string) { warns = append(warns, "%"+m) }
			for _, w := range []string{"", "tiny", "lp", "lpp", "diku", "telnet"} {
				n := "xaddworld"
				if w != "" {
					n = "xadd" + w
				}
				in.RegisterCommand(n, AddWorldCommand(n, w))
			}
			for _, p := range []string{"k", "m", "q"} {
				in.RegisterCommand("x"+p+"echo", EchoPrefixCommand(p))
			}
			for _, v := range []string{"visual", "gpri", "hpri", "lpquote", "shpause"} {
				in.RegisterCommand("x"+v, SetCommand("x"+v, v))
			}
			in.LibDirs = []string{lib}
			if err := in.LoadFile(filepath.Join(lib, "stdlib.tf")); err != nil {
				t.Fatal(err)
			}
			h.out, warns = nil, nil
			for _, l := range c[k] {
				in.ExecFull(l)
			}
			in.ExecFull("/listworlds")
			s := strings.Join(h.out, "\n") + "\n--\n" + strings.Join(warns, "\n")
			s = strings.NewReplacer("xkecho", "kecho", "xmecho", "mecho", "xqecho", "qecho", "xaddworld", "addworld", "xadd", "add", "xvisual", "visual", "xgpri", "gpri", "xhpri", "hpri", "xlpquote", "lpquote", "xshpause", "shpause").Replace(s)
			got[k] = s
		}
		if got[0] != got[1] {
			t.Errorf("case %d differs\n script: %q\n helper: %q", ci, got[0], got[1])
		}
	}
}
