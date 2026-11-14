package gtfparser

import "strings"

// LineResult is what the trigger stage did to one received line.
type LineResult struct {
	// Gag is true when a matching macro gags the line (Macro.Gag).
	Gag bool
	// Matches lists the matching trigger macros in firing order.
	Matches []Match
}

// Match is one trigger macro that matched a line. The engine does not
// interpret display attributes itself; it reports the macro (whose Attrs
// field holds the -a/-P attribute text) and where it matched, so a front end
// can apply hilites in its own display model.
type Match struct {
	Macro *Macro
	// Span is the byte range of the whole match in the line: the matched
	// text for a regexp trigger, the whole line for the other matching modes.
	Span [2]int
	// Groups are the byte ranges of the regexp's capture groups, index 0 =
	// the whole match (so Groups[n] is %Pn); [-1,-1] for a group that did not
	// take part. Nil for non-regexp triggers.
	Groups [][2]int
}

func (in *Interp) matchOf(m *Macro, line string) Match {
	mt := Match{Macro: m, Span: [2]int{0, len(line)}}
	if m.Matching == "regexp" && m.re != nil {
		if loc := m.re.match(line, 0); loc != nil {
			mt.Span = [2]int{loc[0], loc[1]}
			for i := 0; i+1 < len(loc); i += 2 {
				mt.Groups = append(mt.Groups, [2]int{loc[i], loc[i+1]})
			}
		}
	}
	return mt
}

// SetWorld sets the world the interpreter is acting for: the world a typed
// line, a received line or a hook belongs to. It scopes macros defined with
// -w (they fire only for their world) and is what CurrentWorld reports. An embedder
// that multiplexes several connections through one interpreter sets it
// before each call (inside its Runner closure); "" means no particular world.
func (in *Interp) SetWorld(name string) { in.SetCurrentWorld(name) }

// worldOK reports whether m may fire for the current world.
func (in *Interp) worldOK(m *Macro) bool {
	cur := in.CurrentWorld()
	return m.World == "" || cur == "" || strings.EqualFold(m.World, cur)
}

// UndefID removes the macro with the given number (the one /list shows; it
// also reaches macros that have no name). It reports whether there was one.
func (in *Interp) UndefID(id int) bool {
	for _, m := range in.list {
		if m.ID == id {
			if m.Name != "" && in.macros[m.Name] == m {
				delete(in.macros, m.Name)
			}
			in.removeFromList(m)
			return true
		}
	}
	return false
}
