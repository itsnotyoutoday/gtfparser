package gtfparser

import (
	"fmt"
	"strings"
)

// HookClass says where a hook event's default message is displayed.
type HookClass int

const (
	// HookScreen: the message goes to the screen (TF's tferr stream).
	HookScreen HookClass = iota
	// HookAlert: the message goes to the alert stream (the status line).
	HookAlert
	// HookWorld: the message goes to the event's world; if that world is not
	// the foreground world it is also an alert.
	HookWorld
)

// HookEventInfo describes one hook event. The table (HookEvents) is the
// contract between the interpreter and a client: the client calls
// Interp.FireHook(name, args...) when the event occurs, with the arguments
// documented here.
type HookEventInfo struct {
	Name  string
	Class HookClass
	// XSock: an alert raised for this event is cleared when the world is
	// brought to the foreground.
	XSock bool
	// Optional events exist only when the client enables them with
	// EnableHookEvents (TF builds them in only with the matching protocol).
	Optional bool
	// Args documents the arguments FireHook expects.
	Args string
	// Doc says when the client should fire the event and what the default
	// message / action is.
	Doc string
	// msg builds the default message from the arguments ("" = none). A
	// leading '!' marks an error-style message (shown through Interp.Warn).
	msg func(a []string) string
}

func arg(a []string, i int) string {
	if i < len(a) {
		return a[i]
	}
	return ""
}

func rest(a []string, i int) string {
	if i < len(a) {
		return strings.Join(a[i:], " ")
	}
	return ""
}

// hookTable lists the events in TinyFugue's order (which is also the order
// /list prints a multi-event hook in).
var hookTable = []HookEventInfo{
	{Name: "ACTIVITY", Class: HookAlert, XSock: true, Args: "world",
		Doc: "First activity in a background world. Default message: '% Activity in world <world>' (alert).",
		msg: func(a []string) string { return "% Activity in world " + arg(a, 0) }},
	{Name: "ATCP", Optional: true, Args: "text", Doc: "ATCP telnet subnegotiation (optional event)."},
	{Name: "BAMF", Class: HookWorld, XSock: true, Args: "world",
		Doc: "A portal moved the connection (bamf). Default message: '% Bamfing to <world>'.",
		msg: func(a []string) string { return "% Bamfing to " + arg(a, 0) }},
	{Name: "BGTEXT", Args: "world", Doc: "Text was printed in a background world; fire after the line is displayed."},
	{Name: "BGTRIG", Class: HookAlert, XSock: true, Args: "world, text",
		Doc: "A trigger ran on text from a background world (fire when ProcessLine counted a non-quiet macro). Default message: '% Trigger in world <world>' (alert).",
		msg: func(a []string) string { return "% Trigger in world " + arg(a, 0) }},
	{Name: "CONFAIL", Class: HookWorld, XSock: true, Args: "world, reason...",
		Doc: "Connection failed. Default message: '% Connection to <world> failed: <reason>'.",
		msg: func(a []string) string { return "% Connection to " + arg(a, 0) + " failed: " + rest(a, 1) }},
	{Name: "CONFLICT", Args: "macro",
		Doc: "A macro was defined with the name of a builtin command (fired by /def). Default message: warning: macro \"<macro>\" conflicts with the builtin command.",
		msg: func(a []string) string {
			return "!warning: macro \"" + arg(a, 0) + "\" conflicts with the builtin command."
		}},
	{Name: "CONNECT", Class: HookWorld, XSock: true, Args: "world [cipher]",
		Doc: "A connection was established. Default message: '% Connected to <world>[ using cipher <cipher>].'",
		msg: func(a []string) string {
			if len(a) > 1 {
				return "% Connected to " + a[0] + " using cipher " + a[1] + "."
			}
			return "% Connected to " + arg(a, 0) + "."
		}},
	{Name: "DISCONNECT", Class: HookWorld, XSock: true, Args: "world [reason...]",
		Doc: "The server closed the connection, or the client's disconnect command ran (not /dc). Default message: '% Connection to <world> closed: <reason>.' / 'closed by foreign host.'",
		msg: func(a []string) string {
			if len(a) > 1 {
				return "% Connection to " + a[0] + " closed: " + rest(a, 1) + "."
			}
			return "% Connection to " + arg(a, 0) + " closed by foreign host."
		}},
	{Name: "GMCP", Optional: true, Args: "text", Doc: "GMCP message (optional event)."},
	{Name: "GMCP_LOGIN", Optional: true, Args: "world", Doc: "GMCP subnegotiation after connecting (optional event)."},
	{Name: "ICONFAIL", Class: HookWorld, XSock: true, Args: "world, reason...",
		Doc: "Intermediate (proxy/portal) connection failed. Default message: '% Intermediate connection to <world> failed: <reason>'.",
		msg: func(a []string) string { return "% Intermediate connection to " + arg(a, 0) + " failed: " + rest(a, 1) }},
	{Name: "KILL", Args: "pid", Doc: "A process ended (/kill, or it finished)."},
	{Name: "LOAD", Args: "file", Doc: "A file is being loaded. Default message (unless /load -q): '% Loading commands from <file>.'",
		msg: func(a []string) string { return "% Loading commands from " + arg(a, 0) + "." }},
	{Name: "LOADFAIL", Args: "file, reason...", Doc: "/load failed. Default message: '<file>: <reason>' (error style).",
		msg: func(a []string) string { return "!" + arg(a, 0) + ": " + rest(a, 1) }},
	{Name: "LOG", Args: "file", Doc: "Logging started. Default message: '% Logging to file <file>'.",
		msg: func(a []string) string { return "% Logging to file " + arg(a, 0) }},
	{Name: "LOGIN", Args: "world, character, password",
		Doc: "Automatic login: fire after CONNECT when %login is on and the world has a character and password; the default action (sending the login line) belongs to the client."},
	{Name: "MAIL", Class: HookAlert, Args: "file", Doc: "New mail. Default message: '% You have new mail in <file>' (alert).",
		msg: func(a []string) string { return "% You have new mail in " + arg(a, 0) }},
	{Name: "MORE", Doc: "The more-prompt was shown or cleared."},
	{Name: "MSDP", Optional: true, Args: "text", Doc: "MSDP message (optional event)."},
	{Name: "NOMACRO", Args: "name", Doc: "A command/macro name was not found. Default message: '<name>: no such command or macro' (error style).",
		msg: func(a []string) string { return "!" + arg(a, 0) + ": no such command or macro" }},
	{Name: "OPTION102", Optional: true, Args: "text", Doc: "Telnet option 102 subnegotiation (optional event)."},
	{Name: "PENDING", Class: HookWorld, XSock: true, Args: "world [address]",
		Doc: "Connection in progress. Default message: '% Hostname lookup for <world> in progress.' / '% Trying to connect to <world>: <address>.'",
		msg: func(a []string) string {
			if len(a) > 1 {
				return "% Trying to connect to " + a[0] + ": " + rest(a, 1) + "."
			}
			return "% Hostname lookup for " + arg(a, 0) + " in progress."
		}},
	{Name: "PREACTIVITY", Args: "world", Doc: "Fire immediately before a line is displayed in a background world."},
	{Name: "PROCESS", Args: "pid", Doc: "A process started."},
	{Name: "PROMPT", Args: "text", Doc: "text is a partial (unterminated) line from the server. If a hook ran, the client should treat the prompt as consumed."},
	{Name: "PROXY", Args: "world", Doc: "A proxy connection to <world> completed."},
	{Name: "REDEF", Args: "type, name", Doc: "Something was redefined (macro, binding, world ...). Default message: '% Redefined <type> <name>' (error style).",
		msg: func(a []string) string { return "!Redefined " + arg(a, 0) + " " + arg(a, 1) }},
	{Name: "RESIZE", Args: "columns, lines", Doc: "The window was resized."},
	{Name: "SEND", Args: "text",
		Doc: "Text is about to be sent. If HookResult.Count > 0 the client must not send the text (the hook handled it)."},
	{Name: "SHADOW", Args: "var_name", Doc: "A local variable shadows a global. Default message: '% Warning: Local variable \"<var>\" overshadows global variable of same name.' (error style).",
		msg: func(a []string) string {
			return "!Warning:  Local variable \"" + arg(a, 0) + "\" overshadows global variable of same name."
		}},
	{Name: "SHELL", Args: "type, command", Doc: "A shell command is being executed. Default message: '% Executing <type>: <command>'.",
		msg: func(a []string) string { return "% Executing " + arg(a, 0) + ": " + rest(a, 1) }},
	{Name: "SIGHUP", Doc: "SIGHUP caught."},
	{Name: "SIGTERM", Doc: "SIGTERM caught."},
	{Name: "SIGUSR1", Doc: "SIGUSR1 caught."},
	{Name: "SIGUSR2", Doc: "SIGUSR2 caught."},
	{Name: "SQUAT_CONNECT", Optional: true, Args: "device, role", Doc: "A remote client (squatter peer) connected and authenticated (optional event, enabled by a client that serves remote peers)."},
	{Name: "SQUAT_DISCONNECT", Optional: true, Args: "device, role", Doc: "A remote client (squatter peer) disconnected (optional event)."},
	{Name: "SQUAT_PAIR", Optional: true, Args: "device, role", Doc: "A new remote client was paired (optional event)."},
	{Name: "WORLD", Class: HookWorld, Args: "world", Doc: "The foreground world changed. The client may supply a message with FireHookEvent."},
}

const hookAll = ^uint64(0)

// HookEvents returns the event table (a copy), for documentation and clients.
func HookEvents() []HookEventInfo {
	return append([]HookEventInfo(nil), hookTable...)
}

func hookIndex(name string) int {
	for i := range hookTable {
		if strings.EqualFold(hookTable[i].Name, name) {
			return i
		}
	}
	return -1
}

// EnableHookEvents turns on optional events (ATCP, GMCP, GMCP_LOGIN, MSDP,
// OPTION102) for a client that implements the matching protocols. Until then
// /def -h<event> rejects them as real tf does when built without them.
func (in *Interp) EnableHookEvents(names ...string) error {
	for _, n := range names {
		i := hookIndex(n)
		if i < 0 {
			return fmt.Errorf("unknown hook event %q", n)
		}
		in.macs().hookOn[i] = true
	}
	return nil
}

func (in *Interp) hookEnabled(i int) bool {
	return !hookTable[i].Optional || in.macs().hookOn[i]
}

// hookByName resolves a hook name for /def -h, with tf's diagnostics.
func (in *Interp) hookByName(name string) int {
	i := hookIndex(name)
	if i < 0 && strings.EqualFold(name, "BACKGROUND") { // backward compatibility
		i = hookIndex("BGTRIG")
	}
	if i >= 0 && !in.hookEnabled(i) {
		i = -1
	}
	if i < 0 {
		if strings.EqualFold(name, "CONNETFAIL") {
			in.cmdErr("DEF", 'h', "invalid hook event \"%s\"; see \"/help /connect\"", name)
		} else {
			in.cmdErr("DEF", 'h', "invalid hook event \"%s\"", name)
		}
	}
	return i
}

// parseHookSpec splits "EVENT[|EVENT...] pattern" into the event mask and the
// pattern (pattern text keeps everything after the single separator).
func (in *Interp) parseHookSpec(s string) (mask uint64, pat string, hasPat bool, ok bool) {
	if s == "" {
		return hookAll, "", false, true
	}
	i := 0
	for {
		j := i
		for j < len(s) && s[j] != ' ' && s[j] != '\t' && s[j] != '|' {
			j++
		}
		name := s[i:j]
		if name == "*" {
			mask = hookAll
		} else {
			n := in.hookByName(name)
			if n < 0 {
				return 0, "", false, false
			}
			mask |= 1 << uint(n)
		}
		if j >= len(s) {
			return mask, "", false, true
		}
		sep := s[j]
		if sep != '|' {
			return mask, s[j+1:], true, true
		}
		i = j + 1
	}
}

// hookNames renders an event mask as EVENT|EVENT in table order.
func (in *Interp) hookNames(mask uint64) string {
	var b strings.Builder
	for i := range hookTable {
		if mask&(1<<uint(i)) == 0 || !in.hookEnabled(i) {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('|')
		}
		b.WriteString(hookTable[i].Name)
	}
	return b.String()
}
