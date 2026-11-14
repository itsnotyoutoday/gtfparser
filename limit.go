package gtfparser

// /limit, /relimit, /unlimit: a display filter on the foreground screen. The
// filter is core state (per screen, i.e. per world); the client asks
// Limited() whether a line may be shown and redraws on the "limit" event.

import "strings"

type limitFilter struct {
	enabled bool
	hasPat  bool
	pat     string
	style   string // the match style of pat (what state export needs to rebuild match)
	match   func(string) bool
	attr    bool // -a: only lines that have attributes
	sense   bool // false for -v
}

type limitState struct{ screens map[string]*limitFilter }

// LimitHost is a client that has a screen to filter. After the core changes
// the filter for a world it calls Redraw; the client re-renders and returns how
// many lines are visible under the new filter, which lets /limit say "No lines
// matched criteria." exactly as TF does. A host without a screen (no LimitHost)
// has no lines to show, so every /limit finds none, as in tf running in batch
// mode. The "limit" Event is raised on every change as well.
type LimitHost interface {
	Redraw(world string) (visible int)
}

func (in *Interp) screenKey() string {
	if fg := in.fgName(); fg != "" {
		return fg
	}
	return in.CurrentWorld()
}

func (in *Interp) screenFilter(key string) *limitFilter {
	ls := &in.ds().lm
	if ls.screens == nil {
		ls.screens = map[string]*limitFilter{}
	}
	f := ls.screens[key]
	if f == nil {
		f = &limitFilter{sense: true}
		ls.screens[key] = f
	}
	return f
}

// LimitActive reports whether a /limit is in effect on the foreground screen
// (the display screen: the foreground world's).
func (in *Interp) LimitActive() bool { return in.LimitActiveFor(in.screenKey()) }

// LimitActiveFor is LimitActive for one world's screen ("" for the screen
// shown when no world is in the foreground).
func (in *Interp) LimitActiveFor(world string) bool {
	return in.screenFilter(world).enabled
}

// Limited reports whether the foreground screen's /limit hides line (always
// false when no limit is in effect).
func (in *Interp) Limited(line string) bool { return in.LimitedFor(in.screenKey(), line, "") }

// LimitedFor is Limited for a given world's screen, with the line's display
// attributes (needed by "/limit -a").
func (in *Interp) LimitedFor(world, line, attrs string) bool {
	f := in.screenFilter(world)
	if !f.enabled {
		return false
	}
	if f.attr {
		return !interesting(attrs)
	}
	if f.match == nil { // -v without a pattern: nothing matches
		return f.sense
	}
	return f.match(line) != f.sense
}

func interesting(attrs string) bool { return strings.ContainsAny(attrs, "nxGAgurfidBhEWIC") }

func (in *Interp) limitRedraw(key string, f *limitFilter) bool {
	state := "off"
	if f.enabled {
		state = "on"
	}
	in.notify("limit", key, state)
	if lh, ok := in.Host.(LimitHost); ok {
		return lh.Redraw(key) > 0
	}
	return !f.enabled // no screen: nothing is visible, so a limit can never match
}

func cmdLimit(in *Interp, args string) Value {
	opts, rest, ok := in.scanOpts("/limit", "avm:", args)
	if !ok {
		return Int(0)
	}
	key := in.screenKey()
	f := in.screenFilter(key)
	style := ""
	attr, sense := false, true
	for _, o := range opts {
		switch o.c {
		case 'a':
			attr = true
		case 'v':
			sense = false
		case 'm':
			s, ok := in.matchStyleName("/limit", o.arg)
			if !ok {
				return Int(0)
			}
			style = s
		}
	}
	pat := strings.TrimRight(rest, " \t")
	if len(opts) == 0 && pat == "" {
		return Bool(f.enabled)
	}
	nf := &limitFilter{enabled: true, attr: attr, sense: sense, hasPat: pat != "", pat: pat, style: style}
	if nf.style == "" {
		nf.style = in.globals["matching"].String()
	}
	if pat != "" {
		m, ok := in.matchStyle("/limit", style, pat)
		if !ok {
			return Int(0)
		}
		nf.match = m
	}
	old := *f
	*f = *nf
	if !in.limitRedraw(key, f) {
		in.Warn("No lines matched criteria.")
		*f = old
		f.enabled = false
		in.limitRedraw(key, f)
		return Int(0)
	}
	return Int(1)
}

func cmdRelimit(in *Interp, args string) Value {
	key := in.screenKey()
	f := in.screenFilter(key)
	if !f.hasPat && !f.attr {
		in.warn("No previous limit.")
		return Int(0)
	}
	f.enabled = true
	if !in.limitRedraw(key, f) {
		in.Warn("No lines matched criteria.")
		f.enabled = false
		in.limitRedraw(key, f)
		return Int(0)
	}
	return Int(1)
}

func cmdUnlimit(in *Interp, args string) Value {
	key := in.screenKey()
	f := in.screenFilter(key)
	if !f.enabled {
		return Int(0)
	}
	f.enabled = false
	in.limitRedraw(key, f)
	return Int(1)
}

func init() {
	builtinCmds["limit"] = cmdLimit
	builtinCmds["relimit"] = cmdRelimit
	builtinCmds["unlimit"] = cmdUnlimit
}

// LimitFilter returns a snapshot of world's /limit filter as a function
// reporting whether a line is hidden (line text, display attributes), or nil
// when no limit is in effect. The function holds no reference to the
// interpreter and is safe to call from any goroutine, so a client can keep it
// next to its scrollback and filter there without going through the
// interpreter's goroutine. Take a new snapshot when the "limit" event or
// LimitHost.Redraw says the filter changed.
func (in *Interp) LimitFilter(world string) func(line, attrs string) bool {
	f := in.screenFilter(world)
	if !f.enabled {
		return nil
	}
	c := *f
	return func(line, attrs string) bool {
		if c.attr {
			return !interesting(attrs)
		}
		if c.match == nil {
			return c.sense
		}
		return c.match(line) != c.sense
	}
}
