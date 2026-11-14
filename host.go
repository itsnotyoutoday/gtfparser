package gtfparser

import "time"

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

// ---------------------------------------------------------------------------
// Optional capabilities used by the world/connection builtins (worlds.go,
// misc_state.go). A host implements only what it supports; a command that
// needs more says "not supported by this client".

// SockState is the state of a connection, in TF's order.
type SockState int

const (
	SockNew        SockState = iota // not started
	SockResolving                   // hostname lookup pending
	SockResolved                    // lookup done
	SockConnecting                  // connect pending
	SockConnected                   // connection established
	SockOpen                        // open without a connection
	SockZombie                      // connection dead, some text unseen
	SockDead                        // connection dead, all text seen
)

// SocketInfo describes one connection (a world with a socket).
type SocketInfo struct {
	World      string
	Host, Port string    // as connected to
	Addr       string    // numeric address ("" until resolved)
	State      SockState //
	Proxy      bool      // via a proxy
	Compressed bool      // MCCP active
	SSL        bool
	Subneg     bool      // in a telnet subnegotiation
	Login      bool      // auto-login in effect
	Cipher     string    // SSL cipher name
	NewLines   int       // unseen lines
	OldLines   int       // seen lines still in the background buffer
	Active     bool      // has unseen text
	LastRecv   time.Time // last text received
	LastSend   time.Time // last text sent
}

// SocketHost lists connections (in TF's socket-list order, the first being
// what "/fg" with no argument selects). Without it, /listsockets reports no
// sockets and is_open()/is_connected() fall back to WorldHost.Connected.
type SocketHost interface {
	Sockets() []SocketInfo
}

// IdleHost reports keyboard idleness for idle() with no argument.
type IdleHost interface{ LastKeyboard() time.Time }

// ConnectOptions are the options of /connect.
type ConnectOptions struct {
	NoLogin    bool // -l
	Quiet      bool // -q
	SSL        bool // -x
	Foreground bool // -f
	Background bool // -b
}

// ConnectHost is a WorldHost that honors /connect's options. The world has
// already been defined (a "(unnamedN)" world for "/connect host port") in the
// Interp's world database; the host reads it with Interp.World.
type ConnectHost interface {
	ConnectWith(world string, opts ConnectOptions) error
}

// FgHost is a WorldHost that can switch the foreground quietly (/fg -q).
type FgHost interface {
	ForegroundQuiet(world string, quiet bool) error
}

// RebootHost restarts the client in place, preserving its sessions when it
// can (TF's /reboot re-execs itself). Optional.
type RebootHost interface {
	// Reboot restarts the client. Interactive confirmation is the client's
	// business; force is true when -y was given.
	Reboot(force bool) error
}
