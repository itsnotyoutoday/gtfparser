package gtfparser

import "testing"

// Scripts that never use origins must not get slower: these run the paths the
// origin code touches (macro calls, trigger bodies, processes, privileged-op
// checks) with no origin configured.

func BenchmarkOriginFreeMacroCall(b *testing.B) {
	in, _ := newTrigInterp(b)
	in.Exec("/def inner = /test x := {1} + 1")
	in.Exec("/def outer = /inner %{1}%; /inner %{1}")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		in.ExecFull("/outer 5")
	}
}

func BenchmarkOriginFreeTriggerBody(b *testing.B) {
	in, _ := newTrigInterp(b)
	in.Exec("/def -t'foo*' -E'1' t1 = /test x := 1")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		procLine(in, "foo bar")
	}
}

func BenchmarkOriginFreeAllowCheck(b *testing.B) {
	in, _ := newTrigInterp(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		in.allow(OpFileRead, "/load", "x")
	}
}

func BenchmarkOriginFreeDefine(b *testing.B) {
	in, _ := newTrigInterp(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		in.Exec("/def -t'zz*' bm = /echo x")
		in.Exec("/def -t'zy*' bm = /echo y")
	}
}
