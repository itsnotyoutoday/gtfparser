package gtfparser

// Host is what every front end provides: a place to show text and a way to
// send a line to a world. Everything else is optional; a command that needs
// a capability the host lacks warns "not supported by this client" and
// returns 0 (it never silently succeeds).
type Host interface {
	// Echo shows text to the user (TF's tfout / screen).
	Echo(text string)
	// Send sends a line to the named world (empty = current); false if
	// there is no such connection.
	Send(world, text string) bool
}

// WorldHost is implemented by clients that manage connections: it backs
// /connect, /dc, /fg and the connection-state functions.
type WorldHost interface {
	Connect(world string) error
	Disconnect(world string) error
	Foreground(world string) error
	ForegroundWorld() string
	Connected(world string) bool
}

// Event is a notification the interpreter raises when state a client may
// need to react to changes (the interpreter owns the state; the client owns
// the display).
type Event struct {
	Kind string   // "beep", "localecho", "status", "limit", "log", "suspend", "restrict", ...
	Args []string // event-specific
}

// Notifier receives Events. Optional.
type Notifier interface{ Notify(Event) }

// HelpHost shows help topics (the interpreter has no help text of its own).
type HelpHost interface{ Help(topic string) }

func (in *Interp) notify(kind string, args ...string) {
	if n, ok := in.Host.(Notifier); ok {
		n.Notify(Event{Kind: kind, Args: args})
	}
}

// KeyHost knows the terminal's key sequences (termcap/terminfo): keycode().
// Without it keycode() answers "" ("the terminal defines no such key").
type KeyHost interface{ KeyCode(name string) string }

// TermHost reports the display size for columns() and lines(). Without it the
// documented defaults of 80x24 are used.
type TermHost interface{ Size() (cols, lines int) }

func (in *Interp) termSize() (int, int) {
	if t, ok := in.Host.(TermHost); ok {
		return t.Size()
	}
	return 80, 24
}
