package gtfparser

import "sync/atomic"

// Limits are hard ceilings an embedding program sets on what scripts may
// consume. Zero means "use TinyFugue's own default". Scripts can still adjust
// the TF variables max_recur and max_instr, but never past these ceilings.
type Limits struct {
	MaxRecur    int   // nested macro calls (TF default 100)
	MaxInstr    int64 // statements executed per top-level execution (TF default 1000000)
	MaxLoadDeep int   // nested /load (default 32)
	MaxOutput   int   // echoed lines per top-level execution; 0 = unlimited
}

// DefaultLimits are used when the client supplies none.
func DefaultLimits() Limits {
	return Limits{MaxRecur: 100, MaxInstr: 1000000, MaxLoadDeep: 32}
}

func (in *Interp) maxRecur() int {
	n := int(in.globals["max_recur"].Int())
	if n <= 0 || (in.limits.MaxRecur > 0 && n > in.limits.MaxRecur) {
		n = in.limits.MaxRecur
	}
	return n
}

func (in *Interp) maxInstr() int64 {
	n := in.globals["max_instr"].Int()
	if n <= 0 || (in.limits.MaxInstr > 0 && n > in.limits.MaxInstr) {
		n = in.limits.MaxInstr
	}
	return n
}

// Interrupt asks the running script to stop (TF's Ctrl-C). It is safe to call
// from any goroutine; the interpreter checks the flag between statements.
func (in *Interp) Interrupt() { in.interrupted.Store(true) }

// tick is called once per statement: it enforces max_instr and interrupts.
// It returns false when execution must unwind.
func (in *Interp) tick() bool {
	if in.interrupted.Load() {
		in.interrupted.Store(false)
		in.warn("Interrupted")
		in.ctl = ctlExit
		return false
	}
	in.instr++
	if m := in.maxInstr(); m > 0 && in.instr > m {
		in.warn("max_instr (%d) exceeded; execution aborted", m)
		in.instr = 0
		in.ctl = ctlExit
		return false
	}
	return true
}

var _ = atomic.Bool{}
