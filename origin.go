package gtfparser

// Origin-tagged restriction.
//
// /restrict is one global level. A host that runs code from several parties
// (its own user, a remote peer, a downloaded script) needs a level per
// party. An origin is an opaque string chosen by the host ("peer:laptop");
// the host gives each origin a level with SetOriginRestriction and runs that
// party's code with ExecAs / RunAs / DoAs. Everything such code defines
// (macros, triggers, hooks, key bindings, /repeat and /quote processes, open
// streams) is tagged with the origin and later runs under it, whatever
// starts it. See docs/engine/origins.md.
//
// With no origin configured or active nothing here costs more than a nil
// check, and no output or behaviour changes.

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// SetOriginRestriction sets the restriction level (RestrictNone ..
// RestrictWorld) of an origin. RestrictNone removes the configuration: an
// origin without a level is unrestricted. Unlike /restrict this is the
// host's policy and may lower a level, but only from outside: it refuses to
// lower or remove a level while any origin's code is running (a host command
// reached from peer code cannot lift the peer's own restriction). It reports
// whether the level was set; an empty origin or an invalid level is refused.
func (in *Interp) SetOriginRestriction(origin string, level int) bool {
	if origin == "" || level < RestrictNone || level > RestrictWorld {
		return false
	}
	cur := in.origLevels[origin]
	if level < cur && len(in.origStack) > 0 {
		return false
	}
	if level == RestrictNone {
		delete(in.origLevels, origin)
		return true
	}
	if in.origLevels == nil {
		in.origLevels = map[string]int{}
	}
	in.origLevels[origin] = level
	return true
}

// OriginRestriction returns the level configured for origin (RestrictNone if
// there is none).
func (in *Interp) OriginRestriction(origin string) int { return in.origLevels[origin] }

// OriginRestrictions returns a copy of every configured origin level.
func (in *Interp) OriginRestrictions() map[string]int {
	out := make(map[string]int, len(in.origLevels))
	for k, v := range in.origLevels {
		out[k] = v
	}
	return out
}

// EffectiveRestriction is the level in force right now: the strictest of the
// global /restrict level and the level of every origin whose code is running
// (the origin of each running tagged macro or process and of each ExecAs on
// the call chain).
func (in *Interp) EffectiveRestriction() int {
	r := in.restriction
	if in.origForce > r {
		r = in.origForce
	}
	if len(in.origStack) > 0 {
		if o := in.originLevel(); o > r {
			r = o
		}
	}
	return r
}

// originLevel is the strictest configured level among the running origins.
func (in *Interp) originLevel() int {
	r := 0
	for _, o := range in.origStack {
		if l := in.origLevels[o]; l > r {
			r = l
		}
	}
	return r
}

// CurrentOrigin is the origin that new definitions are tagged with right
// now: of the origins whose code is running, the one with the highest level
// (ties: the innermost). It is "" when no origin's code is running.
func (in *Interp) CurrentOrigin() string {
	best, bl := "", -1
	for _, o := range in.origStack {
		if l := in.origLevels[o]; l >= bl {
			best, bl = o, l
		}
	}
	return best
}

// Origin is the origin that defined m ("" for the host's own macros).
func (m *Macro) Origin() string { return m.origin }

// pushOrigin / popOrigin bracket the run of tagged code.
func (in *Interp) pushOrigin(o string) { in.origStack = append(in.origStack, o) }
func (in *Interp) popOrigin()          { in.origStack = in.origStack[:len(in.origStack)-1] }

// DoAs runs fn with origin's restriction in force: everything fn defines is
// tagged with origin, and every privileged operation it attempts is checked
// against the strictest of the global level and origin's level. The origin
// stays in force for processes and macros fn starts. An empty origin runs fn
// as the host (inside another origin's code it changes nothing: the stricter
// level always wins).
func (in *Interp) DoAs(origin string, fn func()) {
	if origin == "" {
		fn()
		return
	}
	n := len(in.origStack)
	in.pushOrigin(origin)
	defer func() { in.origStack = in.origStack[:n] }()
	fn()
}

// ExecAs is Exec (a typed or loaded line) run under origin.
func (in *Interp) ExecAs(origin, line string) (v Value) {
	in.DoAs(origin, func() { v = in.Exec(line) })
	return v
}

// RunAs is Run (a script with full substitution, like /eval) run under origin.
func (in *Interp) RunAs(origin, script string) (v Value) {
	in.DoAs(origin, func() { v = in.Run(script) })
	return v
}

// EvalAs is Eval (an expression) run under origin.
func (in *Interp) EvalAs(origin, expr string) (v Value, err error) {
	in.DoAs(origin, func() { v, err = in.Eval(expr) })
	return v, err
}

// streamAllowed: code running under a restricted origin may not use a file or
// pipe stream the host (or another origin) opened: a handle is a capability
// the origin was never granted, and a pipe stream is a shell. It may use
// queues, and any stream opened under its own origin. Without an origin
// restriction (the only case in tf) every stream is usable, as in tf.
func (in *Interp) streamAllowed(s *stream) bool {
	if len(in.origStack) == 0 || s.queue {
		return true
	}
	need := RestrictFile
	if s.cmd != nil {
		need = RestrictShell
	}
	if in.originLevel() < need {
		return true
	}
	return s.origin != "" && in.originActive(s.origin)
}

// originActive reports whether o is one of the running origins.
func (in *Interp) originActive(o string) bool {
	for _, a := range in.origStack {
		if a == o {
			return true
		}
	}
	return false
}

// raiseOrigins is /restrict's origin form: code running under an origin
// can only raise its own origins' levels, never the global one (a peer must
// not lock the owner out).
func (in *Interp) raiseOrigins(level int) {
	for _, o := range in.origStack {
		if in.origLevels[o] < level {
			if in.origLevels == nil {
				in.origLevels = map[string]int{}
			}
			in.origLevels[o] = level
		}
	}
}

// fnMacroOrigin is macro_origin(name): the origin that defined the macro
// (name or #number), "" for the host's own, or 0 with a warning if there is
// no such macro. It is an extension (tf has no such function): a script's own
// macro of the same name takes precedence.
func fnMacroOrigin(in *Interp, a []Value) (Value, error) {
	if len(a) != 1 {
		return Int(0), fmt.Errorf("macro_origin: usage: macro_origin(name)")
	}
	m, ok := in.findMacro(strings.TrimSpace(a[0].String()))
	if !ok {
		return Int(0), nil
	}
	return Str(m.origin), nil
}

// fnCurrentOrigin is current_origin(): the origin running the code, "" for
// the host. Also an extension.
func fnCurrentOrigin(in *Interp, a []Value) (Value, error) { return Str(in.CurrentOrigin()), nil }

// extFunc looks up the origin introspection functions; callFuncTf consults it
// after user macros so a script's own macro of the same name wins.
func extFunc(name string) fn {
	switch name {
	case "macro_origin":
		return fnMacroOrigin
	case "current_origin":
		return fnCurrentOrigin
	}
	return nil
}

// ---------------------------------------------------------------- state export

// The "origins" state section carries the configured levels and the tags of
// macros, processes and streams, so a restart does not turn a peer's macros
// into the owner's. It is a required section (a reader that does not know it
// must refuse the state rather than drop the tags) and is written only when
// there is something to carry, so states of sessions that never used origins
// are unchanged.

type sOriginLevel struct {
	Origin bs  `json:"origin"`
	Level  int `json:"level"`
}

type sOriginTag struct {
	ID     string `json:"id"` // macro number, process number or stream handle
	Origin bs     `json:"origin"`
}

type sOrigins struct {
	Levels  []sOriginLevel `json:"levels,omitempty"`
	Macros  []sOriginTag   `json:"macros,omitempty"`
	Procs   []sOriginTag   `json:"procs,omitempty"`
	Streams []sOriginTag   `json:"streams,omitempty"`
}

func secSaveOrigins(in *Interp, c *saveCtx) any {
	var s sOrigins
	names := make([]string, 0, len(in.origLevels))
	for o := range in.origLevels {
		names = append(names, o)
	}
	sort.Strings(names)
	for _, o := range names {
		s.Levels = append(s.Levels, sOriginLevel{bs(o), in.origLevels[o]})
	}
	for _, m := range in.list {
		if m.origin != "" {
			s.Macros = append(s.Macros, sOriginTag{strconv.Itoa(m.ID), bs(m.origin)})
		}
	}
	ps := append([]*Proc(nil), in.procs...)
	sort.SliceStable(ps, func(i, j int) bool { return ps[i].PID < ps[j].PID })
	for _, p := range ps {
		if p.origin != "" {
			s.Procs = append(s.Procs, sOriginTag{strconv.Itoa(p.PID), bs(p.origin)})
		}
	}
	ids := make([]int, 0, len(in.streams))
	for id, st := range in.streams {
		if st.origin != "" {
			n, _ := strconv.Atoi(id)
			ids = append(ids, n)
		}
	}
	sort.Ints(ids)
	for _, n := range ids {
		s.Streams = append(s.Streams, sOriginTag{strconv.Itoa(n), bs(in.streams[strconv.Itoa(n)].origin)})
	}
	if len(s.Levels)+len(s.Macros)+len(s.Procs)+len(s.Streams) == 0 {
		return nil
	}
	return s
}

func secDecOrigins(in *Interp, v int, raw json.RawMessage, ld *stLoad) error {
	var s sOrigins
	if err := jdec(raw, &s); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, l := range s.Levels {
		if l.Origin == "" || seen[string(l.Origin)] || l.Level < RestrictShell || l.Level > RestrictWorld {
			return fmt.Errorf("%w: bad origin level for %q", ErrStateCorrupt, string(l.Origin))
		}
		seen[string(l.Origin)] = true
	}
	for _, list := range [][]sOriginTag{s.Macros, s.Procs, s.Streams} {
		for _, t := range list {
			if t.Origin == "" {
				return fmt.Errorf("%w: empty origin tag on #%s", ErrStateCorrupt, t.ID)
			}
		}
	}
	ld.origins = s
	return nil
}

func secApplyOrigins(in *Interp, ld *stLoad) {
	s := &ld.origins
	for _, l := range s.Levels { // like /restrict, a load never lowers a level
		if in.origLevels[string(l.Origin)] < l.Level {
			if in.origLevels == nil {
				in.origLevels = map[string]int{}
			}
			in.origLevels[string(l.Origin)] = l.Level
		}
	}
	for _, t := range s.Macros {
		if n, err := strconv.Atoi(t.ID); err == nil {
			if m := in.macroByNum(n); m != nil {
				m.origin = string(t.Origin)
			}
		}
	}
	for _, t := range s.Procs {
		n, _ := strconv.Atoi(t.ID)
		for _, p := range in.procs {
			if p.PID == n {
				p.origin = string(t.Origin)
			}
		}
	}
	for _, t := range s.Streams {
		if st := in.streams[t.ID]; st != nil {
			st.origin = string(t.Origin)
		}
	}
}
