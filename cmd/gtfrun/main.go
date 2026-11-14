// Command gtfrun runs TinyFugue script files through gtfparser (the batch-mode
// runner used by the differential tests against real tf; the interactive client is a separate program).
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"runtime/pprof"

	"github.com/itsnotyoutoday/gtfparser"
)

type stdHost struct{ w *bufio.Writer }

func (h *stdHost) Echo(s string)         { fmt.Fprintln(h.w, s) }
func (h *stdHost) Send(w, s string) bool { fmt.Fprintln(h.w, "[send] "+s); return true }

func main() {
	file := flag.String("f", "", "script file to load")
	lib := flag.String("L", "", "library directory")
	nolib := flag.Bool("n", false, "do not load stdlib.tf")
	prof := flag.String("cpuprofile", "", "write a CPU profile to this file")
	flag.Parse()
	if *prof != "" {
		f, _ := os.Create(*prof)
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	h := &stdHost{bufio.NewWriter(os.Stdout)}
	defer h.w.Flush()
	in := gtfparser.New(h)
	in.Warn = func(m string) { fmt.Fprintln(os.Stderr, "% "+m) }
	if *lib != "" {
		in.LibDirs = []string{*lib}
	}
	if *lib != "" && !*nolib {
		if err := in.LoadFile(*lib + "/stdlib.tf"); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
	if *file != "" {
		if err := in.LoadFile(*file); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
