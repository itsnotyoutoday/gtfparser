package pcre_test

import (
	"strings"
	"testing"

	"github.com/itsnotyoutoday/gtfparser/pcre"
)

func benchMatch(b *testing.B, pat, subj string) {
	re, err := pcre.CompileTF(pat)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		re.Match(subj, 0, false)
	}
}

func BenchmarkSays(b *testing.B) {
	benchMatch(b, `^(\w+) says, "(.*)"$`, `Bob says, "hello there, how are you today?"`)
}
func BenchmarkLiteralMiss(b *testing.B) {
	benchMatch(b, `needle`, strings.Repeat("haystack ", 100))
}
func BenchmarkClassScan(b *testing.B) {
	benchMatch(b, `\d+\.\d+`, strings.Repeat("abc ", 100)+"3.14")
}
func BenchmarkAlt(b *testing.B) {
	benchMatch(b, `(?:foo|bar|baz)+qux`, strings.Repeat("foobarbaz", 20)+"qux")
}
func BenchmarkBackref(b *testing.B) {
	benchMatch(b, `(\w+) \1`, strings.Repeat("word ", 50)+"dup dup")
}
