package gtfparser

// SetWorld sets the world the interpreter is acting for: the world a typed
// line, a received line or a hook belongs to. It scopes macros defined with
// -w (they fire only for their world) and is what CurrentWorld reports. An embedder
// that multiplexes several connections through one interpreter sets it
// before each call (inside its Runner closure); "" means no particular world.
func (in *Interp) SetWorld(name string) { in.SetCurrentWorld(name) }

// UndefID removes the macro with the given number (the one /list shows; it
// also reaches macros that have no name). It reports whether there was one.
func (in *Interp) UndefID(id int) bool {
	m := in.macroByNum(id)
	if m == nil {
		return false
	}
	in.killMacro(m)
	return true
}
