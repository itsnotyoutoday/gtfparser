// Command gtfrun runs TinyFugue script files through gtfparser (the batch-mode
// runner used by the differential tests against real tf; the interactive client is a separate program).
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"runtime/pprof"
	"strings"

	"github.com/itsnotyoutoday/gtfparser"
)

type stdHost struct {
	w  *bufio.Writer
	in *gtfparser.Interp
}

func (h *stdHost) Echo(s string) { fmt.Fprintln(h.w, s) }

// WorldInfoHost: worlds are the ones the script defined (/addworld); none is current.
func (h *stdHost) CurrentWorld() string      { return "" }
func (h *stdHost) WorldType(w string) string { return "" }
func (h *stdHost) WorldExists(w string) bool { _, ok := h.in.World(w); return ok }
func (h *stdHost) Send(w, s string) bool     { return false } // never connected

// IsTTY: the reference tf the differential tests compare with runs on a terminal.
func (h *stdHost) IsTTY() bool { return true }

// CanVisual: the reference runs with TERM=dumb, where visual mode is refused.
func (h *stdHost) CanVisual() bool { return false }

func main() {
	file := flag.String("f", "", "script file to load")
	lib := flag.String("L", "", "library directory")
	nolib := flag.Bool("n", false, "do not load stdlib.tf")
	inter := flag.Bool("i", false, "run -f like typed input: each line is substituted and run (definitions load as from a file); used to check documentation examples")
	prof := flag.String("cpuprofile", "", "write a CPU profile to this file")
	flag.Parse()
	if *prof != "" {
		f, _ := os.Create(*prof)
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	h := &stdHost{w: bufio.NewWriter(os.Stdout)}
	defer h.w.Flush()
	in := gtfparser.New(h)
	h.in = in
	in.SetFeature("256colors", true)
	in.Warn = func(m string) { fmt.Fprintln(os.Stderr, "% "+m) }
	if *lib != "" {
		in.LibDirs = []string{*lib}
	}
	if *lib != "" && !*nolib {
		if err := in.LoadFile(*lib + "/stdlib.tf"); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
	if *file != "" && *inter {
		runInteractive(in, *file)
	} else if *file != "" {
		if err := in.LoadFile(*file); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

// runInteractive runs a file the way documentation examples read: ordinary
// lines go through Run (full substitution), definitions and /load go through
// Exec (as in a script file), and a trailing backslash joins lines.
func runInteractive(in *gtfparser.Interp, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var cur string
	flush := func() {
		line := strings.TrimSpace(cur)
		cur = ""
		switch {
		case line == "" || strings.HasPrefix(line, ";"):
		case strings.HasPrefix(line, "/def ") || strings.HasPrefix(line, "/load ") || strings.HasPrefix(line, "/require "):
			in.Exec(line)
		default:
			in.Run(line)
		}
	}
	for _, l := range strings.Split(string(data), "\n") {
		if strings.HasSuffix(l, "\\") {
			cur += strings.TrimSuffix(l, "\\") + " "
			continue
		}
		cur += l
		flush()
	}
	if cur != "" {
		flush()
	}
}
