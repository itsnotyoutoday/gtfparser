package gtfparser

// Terminal-side core state: everything about the keyboard, the status line,
// paging and prompts that needs no terminal to compute. A client feeds it
// (keystrokes, "more" counters, server echo state) and draws from it (input
// buffer, status rows, events).
//
// The Interp struct lives in interp.go, so the per-interpreter state kept here
// hangs off a weak-keyed registry instead of a field. It is released when the
// Interp is garbage collected (or by ReleaseTerminal). The integrator may
// replace ts() with a plain field; see docs/terminal-hooks.md.

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"weak"
)

// MoreState is one world's output-window paging state (TF's Screen: paused,
// nback, nnew, ...). The client owns the output buffer and reports it with
// SetMoreState; the interpreter answers morepaused(), moresize() and
// morescroll() from it (or from a MoreHost if the client implements one).
type MoreState struct {
	Paused bool // output is stopped at a "More" prompt (or by /dokey pause)
	Back   int  // physical lines below the bottom of the window (nback)
	New    int  // of those, lines never displayed (nnew); New <= Back
	// BackFiltered and NewFiltered are the same counts restricted to lines
	// matching the current /limit (moresize "l"). They equal Back and New when
	// no limit is active.
	BackFiltered, NewFiltered int
	// Above is how many lines of history exist above the top of the window,
	// i.e. how far morescroll(-n) can reverse-scroll.
	Above int
	// Active means the world has unseen text (counted by nactive()).
	Active bool
}

// MoreHost lets a client that owns the output window answer the paging
// functions itself, instead of using the counters fed through SetMoreState.
// The other small interfaces below (WindowHost, LineHost, PromptHost,
// StatusHost) are independent optional capabilities.
type MoreHost interface {
	// MorePaused reports whether output of world ("" = current) is paused.
	MorePaused(world string) bool
	// MoreSize is moresize(): flags may contain 'n' (new lines only), 'l'
	// (lines matching /limit) and 'a' (count noactivity lines too).
	MoreSize(flags, world string) int
	// MoreScroll is morescroll(n) on the display window; it returns the
	// number of lines actually scrolled.
	MoreScroll(n int) int
}

// WindowHost reports the output window height for winlines(). Without it the
// height is derived from the screen size, %visual, %isize and %status_height.
type WindowHost interface{ WinLines() int }

// LineHost displays a line that fake_recv() injected for a world, after the
// trigger pipeline ran (it is not called for gagged lines). Without it the
// line is shown with Host.Echo.
type LineHost interface {
	DisplayLine(world, line string)
}

// PromptHost is told when a world's prompt changes (prompt(), /prompt).
// Without it the prompt is only kept in core state (see Prompt).
type PromptHost interface{ SetPrompt(world, text string) }

// StatusHost lets a client supply the text of an internal status field
// (@world, @more, @read, @active, @log, @mail, @clock). When ok is false the
// interpreter evaluates the %status_int_<name> expression as real tf does.
type StatusHost interface {
	StatusValue(name string) (text string, ok bool)
}

type termState struct {
	kb       InputBuffer
	hist     inputHist
	lnext    bool   // LNEXT: the next key is literal
	curKeys  string // keystrokes held back as a possible key-sequence prefix
	inKeys   int    // nesting of KeyPressed (macros run from keys)
	pending  bool   // NEWLINE seen; the line runs when it is safe
	keyTime  int64  // unix nano of the last keystroke
	st       statusState
	more     map[string]*MoreState
	prompts  map[string]string
	echoOff  map[string]bool // worlds whose server asked us not to echo
	snap     map[string]string
	watch    map[string]bool
	varHooks []func(name, old, new string)
	polling  bool
}

var (
	termMu     sync.Mutex
	termStates = map[weak.Pointer[Interp]]*termState{}
)

func (in *Interp) ts() *termState {
	key := weak.Make(in)
	termMu.Lock()
	defer termMu.Unlock()
	if s, ok := termStates[key]; ok {
		return s
	}
	s := &termState{
		more:    map[string]*MoreState{},
		prompts: map[string]string{},
		echoOff: map[string]bool{},
		watch:   map[string]bool{},
	}
	s.hist.reset()
	s.st.init()
	termStates[key] = s
	runtime.AddCleanup(in, func(k weak.Pointer[Interp]) {
		termMu.Lock()
		delete(termStates, k)
		termMu.Unlock()
	}, key)
	return s
}

// ReleaseTerminal drops the keyboard, status and paging state of in. It is
// called automatically when in is garbage collected; call it to free the state
// of an interpreter that is still referenced.
func (in *Interp) ReleaseTerminal() {
	termMu.Lock()
	delete(termStates, weak.Make(in))
	termMu.Unlock()
}

// ---------------------------------------------------------------- helpers

func (in *Interp) varOn(name string) bool {
	v, ok := in.globals[name]
	return ok && v.Truth()
}

func (in *Interp) varInt(name string) int {
	v, ok := in.globals[name]
	if !ok {
		return 0
	}
	return int(v.Int())
}

// curWorld is the foreground world's name ("" with no WorldHost or no world).
func (in *Interp) curWorld() string {
	if w, ok := in.Host.(WorldHost); ok {
		return w.ForegroundWorld()
	}
	return ""
}

func (in *Interp) hasNotifier() bool {
	_, ok := in.Host.(Notifier)
	return ok
}

// ---------------------------------------------------------------- variable change hooks

// OnVarChange registers fn to be called when a watched variable changes. All
// of TF's special variables are watched, plus the status line's variable
// fields and anything passed to WatchVar. Changes are detected when the
// interpreter next enters one of the terminal entry points (KeyPressed,
// StatusRows, the kb* functions, ...) or when the client calls PollVars; for
// immediate notification apply the one-line setGlobal patch in
// docs/terminal-hooks.md, which makes setGlobal call varChanged directly.
func (in *Interp) OnVarChange(fn func(name, old, new string)) {
	s := in.ts()
	s.varHooks = append(s.varHooks, fn)
}

// WatchVar adds a user variable to the set OnVarChange reports.
func (in *Interp) WatchVar(name string) { in.ts().watch[name] = true }

func (in *Interp) watchedNames(s *termState) []string {
	names := make([]string, 0, len(specialVars)+len(s.watch)+8)
	for n := range specialVars {
		names = append(names, n)
	}
	for n := range s.watch {
		names = append(names, n)
	}
	names = append(names, "status_fields", "kbnum")
	for _, n := range s.st.varNames() {
		names = append(names, n)
	}
	return names
}

// PollVars compares watched variables with their last seen values and reports
// changes (see OnVarChange). It is cheap and idempotent.
func (in *Interp) PollVars() {
	s := in.ts()
	if s.polling {
		return
	}
	s.polling = true
	defer func() { s.polling = false }()
	first := s.snap == nil
	if first {
		// an assignment to %status_fields before the first poll must still be seen
		s.snap = map[string]string{"status_fields": s.st.lastVar}
	}
	for _, n := range in.watchedNames(s) {
		cur := ""
		if v, ok := in.globals[n]; ok {
			cur = v.String()
		}
		old, seen := s.snap[n]
		if !seen {
			s.snap[n] = cur
			if !first && cur != "" {
				in.varChanged(n, "", cur)
			}
			continue
		}
		if old != cur {
			in.varChanged(n, old, cur)
		}
	}
}

// varChanged is what setGlobal calls after assigning a variable (see
// docs/terminal-hooks.md); PollVars calls it for changes it detects.
func (in *Interp) varChanged(name, old, new string) {
	s := in.ts()
	if s.snap == nil {
		s.snap = map[string]string{}
	}
	s.snap[name] = new
	switch name {
	case "status_fields":
		in.statusFieldsVarChanged(old, new)
	case "status_height":
		if n := int(Str(new).Int()); n > maxStatusHeight {
			in.warn("status_height must be <= %d", maxStatusHeight)
			in.globals[name] = Str(old).Num()
			s.snap[name] = old
			return
		}
		in.FitScreen()
		in.notify("status")
	case "visual", "isize":
		in.FitScreen()
	case "more":
		if !in.varOn("more") {
			in.togMoreOff()
		}
	case "kbnum":
		in.notify("status", name)
	}
	if strings.HasPrefix(name, "status_") || s.st.usesVar(name) {
		s.st.warned = map[string]bool{}
		in.notify("status", name)
	}
	in.notify("var", name, old, new)
	for _, h := range s.varHooks {
		h(name, old, new)
	}
}

// FitScreen makes the display variables consistent with the screen size, as
// tf's setup_screen does when visual mode starts or %isize / %status_height
// change: visual mode is refused on a screen too small for it, and the input
// window and status area are shrunk until the output window has two lines.
// A client calls it after a resize as well.
func (in *Interp) FitScreen() {
	if !in.varOn("visual") {
		return
	}
	_, lines := in.termSize()
	if lines < 1+1+2 {
		in.warn("Screen is too small for visual mode.")
		in.setGlobal("visual", Str("off"))
		return
	}
	isize, sh := in.varInt("isize"), in.varInt("status_height")
	if isize+sh+2 <= lines {
		return
	}
	if 1+sh+2 <= lines {
		isize = lines - (sh + 2)
	} else {
		isize = 1
		sh = lines - (isize + 2)
	}
	in.setGlobal("isize", Int(int64(isize)))
	in.setGlobal("status_height", Int(int64(sh)))
}

// LastKeyTime is when the last keystroke reached KeyPressed (the time idle()
// measures from); the zero time if none has.
func (in *Interp) LastKeyTime() time.Time {
	if t := in.ts().keyTime; t != 0 {
		return time.Unix(0, t)
	}
	return time.Time{}
}

// ---------------------------------------------------------------- terminal settings snapshot

// TermSettings is the display-related variables as plain values, for clients
// that implement the display (they re-read it on a "var" event).
type TermSettings struct {
	Columns, Lines, WinLines                   int
	Visual, Insert, More, Wrap, Scroll         bool
	WrapSize, WrapSpace, WrapPunct, Isize      int
	StatusHeight                               int
	StatusAttr, StatusPad                      string
	Tabsize                                    int
	Wordpunct                                  string
	Virtscreen, Expnonvis, Beep, Kecho, Keypad bool
}

// TermSettings returns the current values of the display variables.
func (in *Interp) TermSettings() TermSettings {
	in.PollVars()
	c, l := in.termSize()
	return TermSettings{
		Columns: c, Lines: l, WinLines: in.winlines(),
		Visual: in.varOn("visual"), Insert: in.varOn("insert"), More: in.varOn("more"),
		Wrap: in.varOn("wrap"), Scroll: in.varOn("scroll"),
		WrapSize: in.varInt("wrapsize"), WrapSpace: in.varInt("wrapspace"),
		WrapPunct: in.varInt("wrappunct"), Isize: in.varInt("isize"),
		StatusHeight: in.varInt("status_height"),
		StatusAttr:   in.globals["status_attr"].String(), StatusPad: in.globals["status_pad"].String(),
		Tabsize: in.varInt("tabsize"), Wordpunct: in.globals["wordpunct"].String(),
		Virtscreen: in.varOn("virtscreen"), Expnonvis: in.varOn("expnonvis"),
		Beep: in.varOn("beep"), Kecho: in.varOn("kecho"), Keypad: in.varOn("keypad"),
	}
}

// ---------------------------------------------------------------- beep, localecho, suspend

// bell asks the client to beep n times, if %beep allows it (dobell()).
func (in *Interp) bell(n int) {
	if n <= 0 || !in.varOn("beep") {
		return
	}
	if !in.hasNotifier() {
		in.warn("beep: not supported by this client")
		return
	}
	in.notify("beep", strconv.Itoa(n))
}

func cmdBeep(in *Interp, args string) Value {
	a := strings.TrimSpace(args)
	n := 0
	switch {
	case a == "":
		n = 3
	case strings.EqualFold(a, "on"):
		in.setGlobal("beep", Str("on"))
	case strings.EqualFold(a, "off"):
		in.setGlobal("beep", Str("off"))
	default:
		n = int(Str(a).Int())
		if n < 0 {
			return Int(0)
		}
	}
	in.bell(n)
	return Int(1)
}

// LocalEcho reports whether tf echoes the input of world ("" = current)
// itself, i.e. the server has not taken over echoing (TELNET ECHO).
func (in *Interp) LocalEcho(world string) bool {
	if world == "" {
		world = in.curWorld()
	}
	return !in.ts().echoOff[world]
}

// SetLocalEcho records the result of echo negotiation for world; a client calls
// it when the server agrees (or refuses) a change requested by /localecho.
func (in *Interp) SetLocalEcho(world string, on bool) {
	s := in.ts()
	if on {
		delete(s.echoOff, world)
	} else {
		s.echoOff[world] = true
	}
}

func cmdLocalecho(in *Interp, args string) Value {
	a := strings.TrimSpace(args)
	world := in.curWorld()
	if a == "" {
		return Bool(in.LocalEcho(world))
	}
	var on bool
	switch {
	case strings.EqualFold(a, "on"):
		on = true
	case strings.EqualFold(a, "off"):
	default:
		return Int(1)
	}
	if !in.hasNotifier() {
		return in.unsupported("/localecho")
	}
	in.SetLocalEcho(world, on)
	st := "off"
	if on {
		st = "on"
	}
	in.notify("localecho", world, st)
	return Int(1)
}

func cmdSuspend(in *Interp, args string) Value {
	if !in.hasNotifier() {
		return in.unsupported("/suspend")
	}
	in.notify("suspend")
	return Int(1)
}

// ---------------------------------------------------------------- more / paging

func (in *Interp) displayWorld() string { return in.curWorld() }

// moreKey is the paging record a world's lines go to: its own when %virtscreen
// is on (the default), the shared display otherwise.
func (in *Interp) moreKey(world string) string {
	if world == "" || !in.varOn("virtscreen") {
		return in.displayWorld()
	}
	return world
}

func (in *Interp) moreFor(world string) *MoreState {
	s := in.ts()
	k := in.moreKey(world)
	m, ok := s.more[k]
	if !ok {
		m = &MoreState{}
		s.more[k] = m
	}
	return m
}

// MoreState returns the paging state of world ("" = the current world).
func (in *Interp) MoreState(world string) MoreState { return *in.moreFor(world) }

// SetMoreState replaces the paging state of world ("" = the current world).
// The client calls it as output arrives and as the user scrolls.
func (in *Interp) SetMoreState(world string, m MoreState) {
	if m.BackFiltered == 0 && m.NewFiltered == 0 {
		m.BackFiltered, m.NewFiltered = m.Back, m.New
	}
	*in.moreFor(world) = m
	in.notify("status", "@more")
}

func (in *Interp) togMoreOff() {
	for _, m := range in.ts().more {
		m.Paused = false
	}
	in.notify("more", "off")
}

func (in *Interp) winlines() int {
	if w, ok := in.Host.(WindowHost); ok {
		return w.WinLines()
	}
	_, l := in.termSize()
	if in.varOn("visual") {
		n := l - in.varInt("isize") - in.varInt("status_height")
		if n < 1 {
			n = 1
		}
		return n
	}
	return l - 1
}

func fnWinlines(in *Interp, a []Value) (Value, error) {
	in.PollVars()
	return Int(int64(in.winlines())), nil
}

func fnMorepaused(in *Interp, a []Value) (Value, error) {
	world := ""
	if len(a) > 0 {
		world = a[0].String()
	}
	if h, ok := in.Host.(MoreHost); ok {
		return Bool(h.MorePaused(world)), nil
	}
	return Bool(in.moreFor(world).Paused), nil
}

func (in *Interp) moresize(flags, world string) (int, bool) {
	limOnly, newOnly, inclA := false, false, false
	for _, c := range strings.ToLower(flags) {
		switch c {
		case 'l':
			limOnly = true
		case 'n':
			newOnly = true
		case 'a':
			inclA = true
		default:
			in.warn("moresize: illegal flag '%c'", c)
			return 0, false
		}
	}
	if h, ok := in.Host.(MoreHost); ok {
		return h.MoreSize(flags, world), true
	}
	m := in.moreFor(world)
	nnew, nback := m.New, m.Back
	if limOnly {
		nnew, nback = m.NewFiltered, m.BackFiltered
	}
	isDisplay := in.moreKey(world) == in.displayWorld()
	if isDisplay || m.Active || inclA || (!newOnly && nback > nnew) {
		if newOnly {
			return nnew, true
		}
		return nback, true
	}
	return 0, true
}

func fnMoresize(in *Interp, a []Value) (Value, error) {
	flags, world := "", ""
	if len(a) > 0 {
		flags = a[0].String()
	}
	if len(a) > 1 {
		world = a[1].String()
	}
	n, _ := in.moresize(flags, world)
	return Int(int64(n)), nil
}

func fnNactive(in *Interp, a []Value) (Value, error) {
	if len(a) > 0 {
		n, _ := in.moresize("n", a[0].String())
		return Int(int64(n)), nil
	}
	count := 0
	s := in.ts()
	for k, m := range s.more {
		if m.Active && k != in.displayWorld() {
			count++
		}
	}
	return Int(int64(count)), nil
}

// moreScroll is clear_more() on the display window's counters.
func (in *Interp) moreScroll(n int) int {
	if h, ok := in.Host.(MoreHost); ok {
		return h.MoreScroll(n)
	}
	m := in.moreFor("")
	scrolled := 0
	if n < 0 {
		k := -n
		if k > m.Above {
			k = m.Above
		}
		if k > 0 {
			m.Above -= k
			m.Back += k
			m.BackFiltered += k
			m.Paused = true
		}
		scrolled = -k
	} else {
		if !m.Paused {
			return 0
		}
		k := n
		if k > m.BackFiltered {
			k = m.BackFiltered
		}
		m.Above += k
		m.Back -= k
		m.BackFiltered -= k
		if m.Back < 0 {
			m.Back = 0
		}
		if m.New > m.Back {
			m.New = m.Back
		}
		if m.NewFiltered > m.BackFiltered {
			m.NewFiltered = m.BackFiltered
		}
		scrolled = k
		if k < n {
			m.Paused = false
		}
	}
	in.notify("more", "scroll", strconv.Itoa(scrolled))
	in.notify("status", "@more")
	return scrolled
}

func fnMorescroll(in *Interp, a []Value) (Value, error) {
	in.PollVars()
	return Int(int64(in.moreScroll(int(num(a, 0))))), nil
}

// dokeyPause is pause_screen().
func (in *Interp) dokeyPause() int {
	if h, ok := in.Host.(MoreHost); ok {
		if h.MorePaused("") {
			return 0
		}
		in.notify("dokey", "PAUSE")
		in.Hook("MORE", "")
		return 1
	}
	m := in.moreFor("")
	if m.Paused {
		return 0
	}
	m.Paused = true
	in.notify("dokey", "PAUSE")
	in.notify("status", "@more")
	in.Hook("MORE", "")
	return 1
}

// dokeyFlush is screen_end(): jump to the end of the output.
func (in *Interp) dokeyFlush() int {
	m := in.moreFor("")
	m.Back, m.New, m.BackFiltered, m.NewFiltered = 0, 0, 0, 0
	m.Paused = false
	in.notify("dokey", "FLUSH")
	in.notify("status", "@more")
	return 1
}

// ---------------------------------------------------------------- prompt and fake_recv

// PromptText returns the current prompt of world ("" = current).
func (in *Interp) PromptText(world string) string {
	if world == "" {
		world = in.curWorld()
	}
	return in.ts().prompts[world]
}

// SetPrompt sets world's prompt (what prompt() does for the current world).
func (in *Interp) SetPrompt(world, text string) {
	if world == "" {
		world = in.curWorld()
	}
	in.ts().prompts[world] = text
	if h, ok := in.Host.(PromptHost); ok {
		h.SetPrompt(world, text)
	}
	in.notify("prompt", world, text)
	in.Prompt() // run processes waiting for a prompt (/repeat -P, /quote -P)
}

func (in *Interp) haveCurrentWorld() (string, bool) {
	w, ok := in.Host.(WorldHost)
	if !ok {
		return "", false
	}
	name := w.ForegroundWorld()
	return name, name != ""
}

func fnPrompt(in *Interp, a []Value) (Value, error) {
	world, ok := in.haveCurrentWorld()
	if !ok {
		return Int(0), nil
	}
	in.SetPrompt(world, str(a, 0))
	return Int(1), nil
}

func fnFakeRecv(in *Interp, a []Value) (Value, error) {
	text, world, flags := str(a, 0), str(a, 1), str(a, 2)
	raw := false
	for _, c := range flags {
		if c != 'r' {
			in.warn("fake_recv: invalid flag %c", c)
			return Int(0), nil
		}
		raw = true
	}
	wh, ok := in.Host.(WorldHost)
	if world == "" && ok {
		world = wh.ForegroundWorld()
	}
	if !ok || world == "" || !wh.Connected(world) {
		in.warn("fake_recv: no open world %s", str(a, 1))
		return Int(0), nil
	}
	in.FakeRecv(world, text, raw)
	return Int(1), nil
}

// FakeRecv injects text as if world had sent it: each line runs the trigger
// pipeline (ProcessLine) and, unless gagged, is shown through LineHost (or
// Host.Echo). With raw the text is a byte stream: lines end at "\n" (a "\r"
// before it is dropped) and an unterminated remainder becomes the prompt
// (after the PROMPT hook), as for a real unterminated server prompt.
func (in *Interp) FakeRecv(world, text string, raw bool) {
	show := func(line string) {
		if in.ProcessLine(line) {
			return
		}
		if h, ok := in.Host.(LineHost); ok {
			h.DisplayLine(world, line)
		} else {
			in.Host.Echo(line)
		}
	}
	if !raw {
		show(text)
		return
	}
	for {
		i := strings.IndexByte(text, '\n')
		if i < 0 {
			break
		}
		show(strings.TrimSuffix(text[:i], "\r"))
		text = text[i+1:]
	}
	if text != "" {
		in.Hook("PROMPT", text)
		in.SetPrompt(world, text)
	}
}

// ---------------------------------------------------------------- registration

// regFunc registers a builtin function that, like tf, rejects calls with fewer
// than min or more than max arguments.
func regFunc(name string, min, max int, f fn) {
	builtinFuncs[name] = func(in *Interp, a []Value) (Value, error) {
		if len(a) < min || len(a) > max {
			if min == max {
				return Int(0), fmt.Errorf("%s: found %d arguments, expected %d", name, len(a), min)
			}
			return Int(0), fmt.Errorf("%s: found %d arguments, expected between %d and %d", name, len(a), min, max)
		}
		return f(in, a)
	}
}

func init() {
	builtinCmds["beep"] = cmdBeep
	builtinCmds["localecho"] = cmdLocalecho
	builtinCmds["suspend"] = cmdSuspend
	regFunc("winlines", 0, 0, fnWinlines)
	regFunc("morepaused", 0, 1, fnMorepaused)
	regFunc("moresize", 0, 2, fnMoresize)
	regFunc("morescroll", 1, 1, fnMorescroll)
	regFunc("nactive", 0, 1, fnNactive)
	regFunc("prompt", 1, 1, fnPrompt)
	regFunc("fake_recv", 1, 3, fnFakeRecv)
}

var _ = fmt.Sprint
