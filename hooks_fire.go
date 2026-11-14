package gtfparser

import "strings"

// HookEvent is one occurrence of a hook event, as a client reports it.
type HookEvent struct {
	// Name is the event (see HookEvents), case-insensitive.
	Name string
	// Args are the event's arguments (see HookEventInfo.Args). They are joined
	// with spaces to form the text hook patterns match, unless ArgText is set.
	Args []string
	// ArgText overrides the joined Args as the text patterns are matched
	// against (when HasArgText).
	ArgText    string
	HasArgText bool
	// Message overrides the event's default message (when HasMsg); NoMessage
	// suppresses it. A leading '!' marks an error-style message.
	Message   string
	HasMsg    bool
	NoMessage bool
	// World is the world the event belongs to (default: the current world),
	// used to match /def -w and -T restrictions.
	World string
}

// HookResult reports what firing an event did.
type HookResult struct {
	// Count is the number of non-quiet hook macros that ran. For SEND it means
	// "the hook handled the text: do not send it"; for PROMPT, "the prompt was
	// consumed".
	Count int
	// Ran lists the hook macros that ran (quiet ones included), so a client
	// can honour a gag attribute on, say, a SEND hook.
	Ran []*Macro
	// Message is the event's message after hook attributes were applied
	// (valid when HasMessage).
	Message    AttrString
	HasMessage bool
	// Gag: a hook gagged the message.
	Gag bool
	// Class says where the message belongs (see HookClass).
	Class HookClass
	// Suppressed: a HookFilter vetoed the event.
	Suppressed bool
}

// HookFilter runs before an event's hooks are matched; it may edit the event
// or return false to drop it entirely.
type HookFilter func(in *Interp, ev *HookEvent) bool

// AddHookFilter installs a filter; the returned function removes it.
func (in *Interp) AddHookFilter(f HookFilter) (remove func()) {
	ms := in.macs()
	ms.hookFilter = append(ms.hookFilter, &f)
	return func() {
		for i, x := range ms.hookFilter {
			if x == &f {
				ms.hookFilter = append(ms.hookFilter[:i:i], ms.hookFilter[i+1:]...)
				return
			}
		}
	}
}

// HookMessage is a hook event's message, handed to a client to display where
// its class says.
type HookMessage struct {
	Event string
	Class HookClass
	// XSock: an alert should be cleared when World is brought to the foreground.
	XSock bool
	World string
	Text  AttrString
}

// HookDisplayHost is implemented by clients that want to place hook messages
// themselves (status-line alerts, per-world output). Without it every message
// is shown with Host.Echo (error-style ones through Interp.Warn).
type HookDisplayHost interface{ HookMessage(HookMessage) }

// FireHook reports that a client event occurred, with its arguments joined
// into one string, and returns the hook macros that ran (see FireHookArgs).
func (in *Interp) FireHook(event, args string) []*Macro {
	ev := HookEvent{Name: event, Args: strings.Fields(args), ArgText: args, HasArgText: true}
	return in.FireHookEvent(ev).Ran
}

// Hook fires an event, ignoring the result.
func (in *Interp) Hook(event, args string) { in.FireHook(event, args) }

// FireHookArgs reports that a client event occurred; see HookEvents for the
// event names and arguments. It runs the matching hooks and displays the
// event's message (through HookDisplayHost or Host.Echo) before the hook
// macros run, exactly as tf's do_hook() does, and returns the result.
func (in *Interp) FireHookArgs(event string, args ...string) *HookResult {
	return in.FireHookEvent(HookEvent{Name: event, Args: args})
}

// FireHookEvent is FireHook with full control over the event.
func (in *Interp) FireHookEvent(ev HookEvent) *HookResult {
	ms := in.macs()
	i := hookIndex(ev.Name)
	if i < 0 || !in.hookEnabled(i) {
		in.warn("FireHook: unknown hook event %q", ev.Name)
		return &HookResult{}
	}
	for _, f := range ms.hookFilter {
		if !(*f)(in, &ev) {
			return &HookResult{Suppressed: true}
		}
	}
	info := &hookTable[i]
	res := &HookResult{Class: info.Class}
	args := ev.ArgText
	if !ev.HasArgText {
		args = strings.Join(ev.Args, " ")
	}
	msg := ""
	switch {
	case ev.NoMessage:
	case ev.HasMsg:
		msg = ev.Message
	case info.msg != nil:
		msg = info.msg(ev.Args)
	}
	world := ev.World
	if world == "" {
		// events about a world ("world" is their first argument) belong to it
		if strings.HasPrefix(info.Args, "world") && len(ev.Args) > 0 {
			world = ev.Args[0]
		} else {
			world = in.ctxWorld()
		}
	}
	errStyle := strings.HasPrefix(msg, "!")
	if errStyle {
		msg = msg[1:]
	}
	var line AttrString
	if msg != "" {
		line.Text = msg
		res.HasMessage = true
	}
	shown := false
	show := func() {
		if shown {
			return
		}
		shown = true
		res.Message = line
		res.Gag = line.Attr&AttrGag != 0
		if res.HasMessage {
			in.showHookMessage(info, world, line, errStyle)
		}
	}
	if in.flag("hook") || in.flag("hilite") || in.flag("gag") {
		rq := &matchReq{hook: i, text: args, world: world, global: true, display: show}
		if res.HasMessage {
			rq.line = &line
		}
		oldH := ms.hres
		ms.hres = res
		res.Count = in.findAndRun(rq)
		ms.hres = oldH
	}
	show()
	return res
}

func (in *Interp) showHookMessage(info *HookEventInfo, world string, line AttrString, errStyle bool) {
	if h, ok := in.Host.(HookDisplayHost); ok {
		if errStyle {
			line.Text = "% " + line.Text
		}
		h.HookMessage(HookMessage{Event: info.Name, Class: info.Class, XSock: info.XSock, World: world, Text: line})
		return
	}
	if line.Attr&AttrGag != 0 {
		return
	}
	if errStyle {
		in.warn("%s", line.Text)
		return
	}
	if h, ok := in.Host.(AttrEchoHost); ok {
		h.EchoAttr(line)
		return
	}
	in.Host.Echo(line.Text)
}

func cmdHook(in *Interp, args string) Value {
	a := strings.TrimSpace(args)
	switch {
	case a == "":
		s := "dis"
		if in.flag("hook") {
			s = "en"
		}
		in.echo("% Hooks " + s + "abled")
		return Int(1)
	case strings.EqualFold(a, "off"):
		in.setGlobal("hook", Int(0))
		return Int(1)
	case strings.EqualFold(a, "on"):
		in.setGlobal("hook", Int(1))
		return Int(1)
	}
	pat, body := splitArgs(args)
	mask, hargs, hasPat, ok := in.parseHookSpec(pat)
	if !ok {
		return Int(0)
	}
	return Int(int64(in.addNewMacro("", mask, true, hargs, hasPat && hargs != "", body, 0, 100, 0)))
}

func init() {
	builtinCmds["hook"] = cmdHook
}

// sendLine sends a plain-text line from a macro body to the current world,
// first offering it to the SEND hook: if a hook handled it, it is not sent
// (unless the hook was defined with -q, which does not count).
func (in *Interp) sendLine(line string) Value {
	if in.flag("hook") {
		if in.FireHookArgs("SEND", line).Count > 0 {
			return Int(1)
		}
	}
	return Bool(in.Host.Send("", line))
}

// noMacro is what a call to an undefined command does: the NOMACRO hook
// (whose default message is the "no such command or macro" error).
func (in *Interp) noMacro(name string) { in.FireHookArgs("NOMACRO", name) }
