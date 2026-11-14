// Command pcrerun evaluates a corpus with the pcre package.
//
//	pcrerun corpus.txt        one result record per "pattern<TAB>subject" pair
//	pcrerun -ref cases.txt    ovector per "pattern, subject, start, notbol, caseless" line
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/itsnotyoutoday/gtfparser/pcre/internal/pcrerun"
)

func main() {
	args := os.Args[1:]
	ref := len(args) > 0 && args[0] == "-ref"
	if ref {
		args = args[1:]
	}
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: pcrerun [-ref] file")
		os.Exit(2)
	}
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	if ref {
		b, err := os.ReadFile(args[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		for _, l := range strings.Split(strings.TrimSuffix(string(b), "\n"), "\n") {
			if l == "" {
				continue
			}
			out, err := pcrerun.RefLine(l)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			fmt.Fprintln(w, out)
		}
		return
	}
	f, err := os.Open(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	pairs, err := pcrerun.Parse(f)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	for _, p := range pairs {
		fmt.Fprintln(w, pcrerun.Record(p))
	}
}
