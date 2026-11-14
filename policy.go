package gtfparser

import "strings"

// Op is a privileged operation scripts may attempt.
type Op int

const (
	OpShell     Op = iota // /sh, /quote !, popen, uncompression
	OpFileRead            // /load, /quote ', tfopen(r)
	OpFileWrite           // /save, /saveworld, /log, tfopen(w|a)
	OpDirChange           // /lcd
	OpNewWorld            // /addworld, /connect host port
)

func (o Op) String() string {
	return [...]string{"shell", "file read", "file write", "directory change", "new world"}[o]
}

// Policy lets an embedding program veto privileged operations (on top of the
// /restrict level). Allow returns nil to permit. detail names the target
// (a file, a command line, a world).
type Policy interface {
	Allow(op Op, detail string) error
}

// Restriction levels, as /restrict sets them (a level can be raised, never lowered).
const (
	RestrictNone = iota
	RestrictShell
	RestrictFile
	RestrictWorld
)

var restrictNames = []string{"NONE", "SHELL", "FILE", "WORLD"}

// Restriction returns the current /restrict level.
func (in *Interp) Restriction() int { return in.restriction }

// SetRestriction raises the restriction level (lowering it is refused).
func (in *Interp) SetRestriction(level int) bool {
	if level < in.restriction || level > RestrictWorld {
		return false
	}
	if level != in.restriction {
		in.restriction = level
		in.notify("restrict", restrictNames[level])
	}
	return true
}

func levelFor(op Op) int {
	switch op {
	case OpShell:
		return RestrictShell
	case OpFileRead, OpFileWrite, OpDirChange:
		return RestrictFile
	}
	return RestrictWorld
}

// allow checks whether op is permitted for who (the command/function name,
// for the message). It warns and returns false when it is not.
func (in *Interp) allow(op Op, who, detail string) bool {
	if in.EffectiveRestriction() >= levelFor(op) {
		in.warn("%s: restricted", who)
		return false
	}
	if in.Policy != nil {
		if err := in.Policy.Allow(op, detail); err != nil {
			in.warn("%s: %v", who, err)
			return false
		}
	}
	return true
}

func cmdRestrict(in *Interp, args string) Value {
	w, _ := firstWord(args)
	if w == "" {
		r := in.EffectiveRestriction()
		in.echo("% restriction level: " + strings.ToLower(restrictNames[r]))
		return Int(int64(r))
	}
	for i, n := range restrictNames {
		if strings.EqualFold(w, n) {
			if len(in.origStack) > 0 {
				// code running under an origin raises its own origins' levels,
				// never the global one (see origin.go)
				if i < in.EffectiveRestriction() {
					in.warn("/restrict: restriction level cannot be lowered")
					return Int(0)
				}
				in.raiseOrigins(i)
				return Int(int64(i))
			}
			if !in.SetRestriction(i) {
				in.warn("/restrict: restriction level cannot be lowered")
				return Int(0)
			}
			return Int(int64(i))
		}
	}
	in.warn("/restrict: invalid level %q (NONE, SHELL, FILE or WORLD)", w)
	return Int(0)
}
