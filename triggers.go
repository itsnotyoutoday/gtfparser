package gtfparser

import (
	"math/rand"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------- client-facing types

// WorldInfoHost lets the interpreter resolve world context: which world a
// line or event belongs to by default, what type a world is (for /def -T),
// and whether a world exists (for /def -w). A client without worlds need not
// implement it: -w and -T restrictions then never match / are not validated.
type WorldInfoHost interface {
	// CurrentWorld is the world whose text or event is being processed (tf's
	// xworld), "" if none.
	CurrentWorld() string
	// WorldType returns a world's type string ("" if none).
	WorldType(world string) string
	// WorldExists reports whether the named world is defined.
	WorldExists(world string) bool
}

// ctxWorld is the world the interpreter is acting for: the one set with
// SetWorld, else what the host reports.
func (in *Interp) ctxWorld() string {
	if w := in.CurrentWorld(); w != "" {
		return w
	}
	if w, ok := in.Host.(WorldInfoHost); ok {
		return w.CurrentWorld()
	}
	if w, ok := in.Host.(WorldHost); ok {
		return w.ForegroundWorld()
	}
	return ""
}

func (in *Interp) worldType(world string) string {
	if w, ok := in.Host.(WorldInfoHost); ok {
		return w.WorldType(world)
	}
	if wi, ok := in.World(world); ok {
		return wi.Type
	}
	return ""
}

// LineInput is a line of text received from a world, to be run through
// triggers.
type LineInput struct {
	// Text is the visible text of the line (ANSI already decoded).
	Text string
	// Attr is the line's incoming attributes (e.g. from ANSI at its start).
	Attr Attr
	// Line, if set, replaces Text/Attr and may carry per-character attributes.
	Line *AttrString
	// World is the world the line came from ("" = the current world).
	World string
	// Local and GlobalOnly narrow which triggers are considered, as
	// /trigger -w and -g do: by default both global triggers and the world's
	// own are matched.
	WorldOnly  bool
	GlobalOnly bool
	// Dry only matches: no macro body runs and no shot is consumed (the
	// attributes of the selected macros are still applied), so a client can
	// recompute the display effects of replayed history without side effects.
	Dry bool
}

// MacroRun records one macro the matcher selected.
type MacroRun struct {
	Macro *Macro
	// Name is the name (or "#number") the macro ran as.
	Name string
	// Hook is the event name for a hook, "" for a trigger.
	Hook string
	// Args is the text the macro was called with.
	Args string
	// Ran: the body was executed. false when the macro was only an attribute
	// match, had an empty body, or lost its probability roll.
	Ran bool
	// Counted: Ran and not -q (this is what /trigger's result counts).
	Counted bool
	// Skipped says why a selected macro did not run: "", "prob", "rate",
	// "recursion".
	Skipped string
}

// LineResult is what ProcessLine tells the client about one line.
type LineResult struct {
	// Original is the line as received.
	Original string
	// Line is the line to display, with attributes applied by triggers and
	// text replaced by /substitute. Render it with Line.Spans(ctx).
	Line AttrString
	// Gag: the line should not be displayed (but may still be recorded /
	// recalled, as in tf). NoLog, NoHistory, NoActivity, Bell mirror the
	// corresponding line attributes.
	Gag, NoLog, NoHistory, NoActivity, Bell bool
	// Substituted: a trigger replaced the text with /substitute.
	Substituted bool
	// Matched lists the macros selected (attributes applied and/or run), in
	// order. Matches is the same list with where each one matched.
	Matched []*Macro
	Matches []Match
	// Ran lists every selected macro with what happened to it.
	Ran []MacroRun
	// Count is the number of non-quiet macros that ran (tf's /trigger result;
	// a client fires BGTRIG for a background world when it is > 0).
	Count int
	// ShotsUsed lists macros whose last shot was consumed (now undefined).
	ShotsUsed []*Macro
	// Suppressed: a pre-line filter vetoed trigger processing.
	Suppressed bool
}

// Match is one trigger macro that matched a line, with where it matched, so a
// front end can apply hilites in its own display model.
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

func matchOf(m *Macro, text string, caps []int) Match {
	mt := Match{Macro: m, Span: [2]int{0, len(text)}}
	if m.Trig != nil && m.Trig.kind == pkRegexp && len(caps) >= 2 {
		mt.Span = [2]int{caps[0], caps[1]}
		for i := 0; i+1 < len(caps); i += 2 {
			mt.Groups = append(mt.Groups, [2]int{caps[i], caps[i+1]})
		}
	}
	return mt
}

// Display returns the spans to draw, using the interpreter's attribute
// variables.
func (r *LineResult) Spans(in *Interp) []Span { return r.Line.Spans(in.attrCtx()) }

// MacroEventKind identifies a TriggerObserver event.
type MacroEventKind int

const (
	// EvSelected: the macro matched and was chosen (its attributes applied).
	EvSelected MacroEventKind = iota
	// EvRun: the macro body is about to execute.
	EvRun
	// EvSkipProb: the macro was chosen but lost its -c probability roll.
	EvSkipProb
	// EvSkipRate: the macro was chosen but %max_trig / %max_hook was exceeded.
	EvSkipRate
	// EvShotsExhausted: the macro used its last -n shot and was removed.
	EvShotsExhausted
	// EvSubstitute: substitute() replaced the line text (Text is the new text).
	EvSubstitute
	// EvCalled: a macro was called by name (accounting for /list -u).
	EvCalled
)

// MacroEvent is delivered to TriggerObservers.
type MacroEvent struct {
	Kind  MacroEventKind
	Macro *Macro
	// Hook is the event name for hook activity, "" for triggers.
	Hook  string
	Text  string
	World string
}

// TriggerObserver receives a notification for every step of trigger, hook and
// macro-call processing, for debugging UIs and accounting. Calls are
// synchronous and on the interpreter's goroutine; an observer must not call
// back into the interpreter.
type TriggerObserver interface{ MacroEvent(MacroEvent) }

// AddObserver registers o; the returned function unregisters it.
func (in *Interp) AddObserver(o TriggerObserver) (remove func()) {
	ms := in.macs()
	ms.obs = append(ms.obs, o)
	return func() {
		for i, x := range ms.obs {
			if x == o {
				ms.obs = append(ms.obs[:i:i], ms.obs[i+1:]...)
				return
			}
		}
	}
}

func (in *Interp) observe(e MacroEvent) {
	for _, o := range in.mac.obs {
		o.MacroEvent(e)
	}
}

// LineFilter runs before triggers see a line. It may edit the input; returning
// false suppresses trigger processing for the line (the result is marked
// Suppressed and the line is returned unchanged).
type LineFilter func(in *Interp, l *LineInput) bool

// ResultFilter runs after triggers have run and may adjust the result (for
// example force a gag).
type ResultFilter func(in *Interp, r *LineResult)

// AddPreLineFilter and AddPostLineFilter install filters; the returned
// function removes it.
func (in *Interp) AddPreLineFilter(f LineFilter) (remove func()) {
	ms := in.macs()
	ms.pre = append(ms.pre, &f)
	return func() {
		for i, x := range ms.pre {
			if x == &f {
				ms.pre = append(ms.pre[:i:i], ms.pre[i+1:]...)
				return
			}
		}
	}
}

func (in *Interp) AddPostLineFilter(f ResultFilter) (remove func()) {
	ms := in.macs()
	ms.post = append(ms.post, &f)
	return func() {
		for i, x := range ms.post {
			if x == &f {
				ms.post = append(ms.post[:i:i], ms.post[i+1:]...)
				return
			}
		}
	}
}

// SetRand replaces the random source used for -c probabilities and for
// choosing among equal-priority non-fall-thru macros: f(n) returns a number in
// [0, n). Tests inject a deterministic one.
func (in *Interp) SetRand(f func(n int) int) {
	if f == nil {
		f = rand.Intn
	}
	in.macs().rng = f
}

// ---------------------------------------------------------------- state

type mnode struct {
	m          *Macro
	next, prev *mnode
}

type mlist struct{ head, tail *mnode }

// rprecedes reports whether a must come before b in a trigger/hook list:
// higher priority first, fall-thrus first within a priority.
func rprecedes(a, b *Macro) bool {
	if a.Priority != b.Priority {
		return a.Priority > b.Priority
	}
	return a.Fall && !b.Fall
}

// insert adds m before the first node that does not strictly precede it
// (so, among equals, the newest macro comes first, as in tf).
func (l *mlist) insert(m *Macro) *mnode {
	n := &mnode{m: m}
	cur := l.head
	for cur != nil && rprecedes(cur.m, m) {
		cur = cur.next
	}
	if cur == nil { // append
		n.prev = l.tail
		if l.tail != nil {
			l.tail.next = n
		} else {
			l.head = n
		}
		l.tail = n
		return n
	}
	n.next, n.prev = cur, cur.prev
	if cur.prev != nil {
		cur.prev.next = n
	} else {
		l.head = n
	}
	cur.prev = n
	return n
}

func (l *mlist) remove(n *mnode) {
	if n.prev != nil {
		n.prev.next = n.next
	} else {
		l.head = n.next
	}
	if n.next != nil {
		n.next.prev = n.prev
	} else {
		l.tail = n.prev
	}
}

type maxCtr struct {
	bucket [2]int64
	count  [2]int
}

type macState struct {
	styles     map[string]MatchStyle
	hookOn     [64]bool
	binds      map[string]*Macro
	dead       []*Macro
	matchDepth int
	trigGlobal mlist
	trigWorld  map[string]*mlist
	hooks      [64]mlist
	rng        func(n int) int
	obs        []TriggerObserver
	pre        []*LineFilter
	post       []*ResultFilter
	hookFilter []*HookFilter
	incoming   *AttrString // the line being processed (for substitute())
	res        *LineResult
	hres       *HookResult // the hook event being fired (collects Ran)
	trigCtr    maxCtr
	hookCtr    maxCtr
	runDepth   int
	libLoad    int // >0 while a library file (under LibDirs) is being loaded
}

// macs returns this interpreter's macro/trigger/hook state, creating it on
// first use.
func (in *Interp) macs() *macState {
	if in.mac == nil {
		in.mac = &macState{
			styles:    map[string]MatchStyle{},
			binds:     map[string]*Macro{},
			trigWorld: map[string]*mlist{},
			rng:       rand.Intn,
		}
	}
	return in.mac
}

func (in *Interp) trigInsert(m *Macro) {
	ms := in.macs()
	l := &ms.trigGlobal
	if m.World != "" {
		k := strings.ToLower(m.World)
		wl := ms.trigWorld[k]
		if wl == nil {
			wl = &mlist{}
			ms.trigWorld[k] = wl
		}
		l = wl
	}
	m.trigList = l
	m.trigNode = l.insert(m)
}

func (in *Interp) hookInsert(m *Macro) {
	ms := in.macs()
	for i := range hookTable {
		if m.Hooks&(1<<uint(i)) != 0 {
			m.hookNode[i] = ms.hooks[i].insert(m)
		}
	}
}

// nukeDead unlinks dead macros from the trigger and hook lists (tf's
// nuke_dead_macros); it runs once no matching pass is in progress.
func (in *Interp) nukeDead() {
	ms := in.macs()
	for _, m := range ms.dead {
		if m.trigNode != nil {
			m.trigList.remove(m.trigNode)
			m.trigNode = nil
		}
		for i := range m.hookNode {
			if m.hookNode[i] != nil {
				ms.hooks[i].remove(m.hookNode[i])
				m.hookNode[i] = nil
			}
		}
	}
	ms.dead = ms.dead[:0]
}

// ---------------------------------------------------------------- the matcher

type cand struct {
	m    *Macro
	caps []int
	text string
}

type matchReq struct {
	hook   int // hook index, or -1 for a trigger
	text   string
	line   *AttrString // line attributes are applied to (may be nil)
	world  string
	global bool
	wlocal bool
	list   int  // 0 run; 1 list names (/trigger -n); 2 list in full (-l)
	dry    bool // select and apply attributes, but run nothing
	// display, if set, is called once the macros are chosen and before queued
	// (hook) macros run: this is where a hook's message is shown.
	display func()
}

func (in *Interp) flag(name string) bool {
	return in.globals[name].String() != "off" && in.globals[name].Truth()
}

// flagCache reads the on/off variables a matching pass needs at most once.
type flagCache struct {
	in    *Interp
	known uint8
	vals  uint8
}

func (f *flagCache) get(bit uint8, name string) bool {
	if f.known&bit == 0 {
		f.known |= bit
		if f.in.flag(name) {
			f.vals |= bit
		}
	}
	return f.vals&bit != 0
}
func (f *flagCache) borg() bool   { return f.get(1, "borg") }
func (f *flagCache) hilite() bool { return f.get(2, "hilite") }
func (f *flagCache) gag() bool    { return f.get(4, "gag") }

// hookOrTrigName is the label for observers/mecho.
func (r *matchReq) hookName() string {
	if r.hook < 0 {
		return ""
	}
	return hookTable[r.hook].Name
}

// findAndRun is tf's find_and_run_matches: select the macros matching a
// trigger line or hook event by priority, apply their attributes, and run
// them. It returns the number of non-quiet macros run.
func (in *Interp) findAndRun(rq *matchReq) int {
	ms := in.macs()
	ms.matchDepth++
	ran := in.findAndRun1(rq, ms)
	ms.matchDepth--
	if ms.matchDepth == 0 && len(ms.dead) > 0 {
		in.nukeDead()
	}
	return ran
}

func (in *Interp) findAndRun1(rq *matchReq, ms *macState) (ran int) {
	var gnode, wnode *mnode
	if rq.hook >= 0 {
		gnode = ms.hooks[rq.hook].head
	} else {
		gnode = ms.trigGlobal.head
		if rq.world != "" {
			if wl := ms.trigWorld[strings.ToLower(rq.world)]; wl != nil {
				wnode = wl.head
			}
		} else if len(ms.trigWorld) > 0 {
			// no particular world (SetWorld("")): world macros are not
			// restricted, so merge them all into one priority-ordered list
			var all mlist
			for _, wl := range ms.trigWorld {
				for n := wl.head; n != nil; n = n.next {
					all.insert(n.m)
				}
			}
			wnode = all.head
		}
	}
	// flags are read lazily: most macros are plain triggers that need only borg
	fl := flagCache{in: in}
	var wtype string
	wtypeKnown := false

	text := rq.text
	if rq.hook < 0 && rq.line != nil {
		text = rq.line.Text
	}
	lowerlimit := -1
	var nfbuf, qbuf [4]cand
	nonfall, queue := nfbuf[:0], qbuf[:0]
	header := 0

	for gnode != nil || wnode != nil {
		var node *mnode
		switch {
		case wnode == nil:
			node, gnode = gnode, gnode.next
		case gnode == nil:
			node, wnode = wnode, wnode.next
		case rprecedes(gnode.m, wnode.m):
			node, gnode = gnode, gnode.next
		default:
			node, wnode = wnode, wnode.next
		}
		m := node.m
		if m.Priority < lowerlimit && rq.list == 0 {
			break
		}
		if m.dead {
			continue
		}
		if rq.hook < 0 {
			if !(m.Prob > 0 && fl.borg() || m.relAttr && fl.hilite() || m.Attr&AttrGag != 0 && fl.gag()) {
				continue
			}
		}
		if m.World != "" && rq.world != "" && !strings.EqualFold(m.World, rq.world) {
			continue
		}
		if rq.wlocal && m.World == "" {
			continue
		}
		if !rq.global && m.World == "" {
			continue
		}
		if m.WType != nil {
			if rq.world == "" {
				continue
			}
			if !wtypeKnown {
				wtype, wtypeKnown = in.worldType(rq.world), true
			}
			if ok, _ := m.WType.Match(wtype); !ok {
				continue
			}
		}
		if m.Cond != "" && !in.condEvalFor(m) {
			continue
		}
		var caps []int
		if rq.hook >= 0 {
			if m.HArgs != nil {
				var ok bool
				if ok, caps = m.HArgs.Match(text); !ok {
					continue
				}
			}
		} else {
			if m.CheckMatchAttr && (rq.line == nil || rq.line.Attr != m.MatchAttr) {
				continue
			}
			var ok bool
			if ok, caps = m.Trig.Match(text); !ok {
				continue
			}
		}
		c := cand{m, caps, text}
		if rq.list == 0 {
			if m.Fall {
				if rq.line != nil {
					in.applyAttrs(m, caps, text, rq, fl.hilite(), fl.gag())
				}
				in.observe0(EvSelected, m, rq, text, caps)
				if rq.hook >= 0 {
					queue = append(queue, c)
				} else if !rq.dry {
					ran += in.runMatch(c.m, c.text, c.caps, rq)
					if rq.line != nil {
						text = rq.line.Text // in case of /substitute
					}
				}
			} else {
				lowerlimit = m.Priority
				nonfall = append(nonfall, c)
			}
		} else {
			if lowerlimit < 0 {
				ran++
			}
			if header < 3 && m.Priority < lowerlimit {
				in.echo("% The following matching macros would not be applied:")
				header = 3
			}
			if header < 2 && !m.Fall {
				w := ""
				if ran > 1 {
					w = "also "
				}
				in.echo("% One of the following macros would " + w + "be applied:")
				lowerlimit = m.Priority
				header = 2
			}
			if header < 1 {
				in.echo("% All of the following macros would be applied:")
				header = 1
			}
			fc := " "
			if m.Fall {
				fc = "F"
			}
			switch {
			case rq.list > 1:
				in.echo(m.printDef(false))
			case m.Name != "":
				in.echo("% " + fc + " " + padLeft(strconv.Itoa(m.Priority), 10) + " " + m.Name)
			default:
				in.echo("% " + fc + " " + padLeft(strconv.Itoa(m.Priority), 10) + " #" + strconv.Itoa(m.ID))
			}
		}
	}

	if rq.list == 0 {
		if n := len(nonfall); n > 0 {
			// select exactly one of the non-fall-thrus (tf collects them in
			// reverse and indexes from the head)
			pick := ms.rng(n)
			c := nonfall[n-1-pick]
			if rq.line != nil {
				in.applyAttrs(c.m, c.caps, c.text, rq, fl.hilite(), fl.gag())
			}
			in.observe0(EvSelected, c.m, rq, c.text, c.caps)
			if rq.hook >= 0 {
				queue = append(queue, c)
			} else if !rq.dry {
				ran += in.runMatch(c.m, c.text, c.caps, rq)
			}
		}
		if rq.display != nil {
			rq.display()
		}
		for _, c := range queue {
			ran += in.runMatch(c.m, c.text, c.caps, rq)
		}
	} else {
		what, verb := "Text", "triggered"
		if rq.hook >= 0 {
			what, verb = "Event", "hooked"
		}
		plural := "s"
		if ran == 1 {
			plural = ""
		}
		in.echo("% " + what + " would have " + verb + " " + strconv.Itoa(ran) + " macro" + plural + ".")
	}
	return ran
}

func padLeft(s string, n int) string {
	for len(s) < n {
		s = " " + s
	}
	return s
}

func (in *Interp) observe0(k MacroEventKind, m *Macro, rq *matchReq, text string, caps []int) {
	if len(in.mac.obs) > 0 {
		in.observe(MacroEvent{Kind: k, Macro: m, Hook: rq.hookName(), Text: text, World: rq.world})
	}
	if hr := in.mac.hres; hr != nil && k == EvSelected && rq.hook >= 0 {
		hr.Ran = append(hr.Ran, m)
	}
	if res := in.mac.res; res != nil && k == EvSelected && rq.hook < 0 {
		res.Matched = append(res.Matched, m)
		res.Matches = append(res.Matches, matchOf(m, text, caps))
	}
}

// condEval evaluates a -E expression in a scope with no arguments.
func (in *Interp) condEval(expr string) bool {
	f := in.pushFrame("", "")
	_ = f
	ok := in.condition(expr)
	in.popFrame()
	return ok
}

// condEvalFor evaluates the -E condition of m under m's origin: it is code the
// origin wrote and may call guarded functions.
func (in *Interp) condEvalFor(m *Macro) bool {
	if m.origin != "" {
		in.pushOrigin(m.origin)
		defer in.popOrigin()
	}
	return in.condEval(m.Cond)
}

// applyAttrs is apply_attrs_of_match.
func (in *Interp) applyAttrs(m *Macro, caps []int, text string, rq *matchReq, hilite, gag bool) {
	if !hilite && !gag {
		return
	}
	line := rq.line
	ctx := in.attrCtx()
	switch {
	case !hilite:
		line.Attr = ctx.Adj(line.Attr, m.Attr&AttrGag)
	case !gag:
		line.Attr = ctx.Adj(line.Attr, m.Attr&^AttrGag)
	default:
		line.Attr = ctx.Adj(line.Attr, m.Attr)
	}
	if rq.hook >= 0 || len(m.SubAttr) == 0 || !hilite || len(line.Text) == 0 || m.Trig == nil || m.Trig.kind != pkRegexp || caps == nil {
		return
	}
	line.ensureChars(0)
	cur := caps
	offset := 0
	for {
		for _, sa := range m.SubAttr {
			var start, end int
			switch sa.Subexp {
			case -1:
				start, end = 0, cur[0]
			case -2:
				start, end = cur[1], len(line.Text)
			default:
				if sa.Subexp*2+1 >= len(cur) {
					continue
				}
				start, end = cur[sa.Subexp*2], cur[sa.Subexp*2+1]
			}
			for i := start; i >= 0 && i < end && i < len(line.Chars); i++ {
				line.Chars[i] = ctx.Adj(line.Chars[i], sa.Attr)
			}
		}
		if offset == cur[1] {
			break // offset wouldn't move
		}
		offset = cur[1]
		if offset >= len(line.Text) {
			break
		}
		cur = m.Trig.re.match(text, offset)
		if cur == nil {
			break
		}
	}
}

// testMax is test_max_counter: the %max_trig / %max_hook rate limiter.
func (in *Interp) testMax(c *maxCtr, maxVar, flagVar, label string) bool {
	const bucketsize = 5
	now := in.clock().Unix() / bucketsize
	if now != c.bucket[1] {
		c.bucket[0] = now - 1
		c.bucket[1] = now
		c.count[0] = 1
		c.count[1] = 0
	}
	c.count[1]++
	max := int(in.globals[maxVar].Int())
	if c.count[0]+c.count[1] > max {
		in.setGlobal(flagVar, Int(0))
		in.cmdErr("", 0, "%s rate exceeded %%%s (%d) in less than %ds.  Setting %%%s off.", label, maxVar, max, 2*bucketsize, flagVar)
		in.notify("rate", label, flagVar)
		return false
	}
	return true
}

func (in *Interp) clock() time.Time {
	if in.Now != nil {
		return in.Now()
	}
	return time.Now()
}

// runMatch is run_match: execute a selected macro.
func (in *Interp) runMatch(m *Macro, text string, caps []int, rq *matchReq) int {
	ms := in.mac
	hook := rq.hook >= 0
	mr := MacroRun{Macro: m, Name: m.Name, Hook: rq.hookName(), Args: text}
	if m.Name == "" {
		mr.Name = "#" + strconv.Itoa(m.ID)
	}
	record := func() {
		if ms.res != nil {
			ms.res.Ran = append(ms.res.Ran, mr)
		}
	}
	if !hook {
		if !in.flag("borg") {
			return 0
		}
		if rq.text != "" || rq.line != nil {
			if mt := int(in.globals["max_trig"].Int()); mt > 0 && !in.testMax(&ms.trigCtr, "max_trig", "borg", "Trigger") {
				mr.Skipped = "rate"
				record()
				in.observe0(EvSkipRate, m, rq, text, nil)
				return 0
			}
		}
	} else if int(in.globals["max_hook"].Int()) > 0 && !in.testMax(&ms.hookCtr, "max_hook", "hook", "Hook") {
		mr.Skipped = "rate"
		record()
		in.observe0(EvSkipRate, m, rq, text, nil)
		return 0
	}
	ran := 0
	if hook && in.flag("hook") || !hook && in.flag("borg") {
		if m.Prob == 100 || ms.rng(100) < m.Prob {
			if m.Shots > 0 {
				m.Shots--
				if m.Shots == 0 {
					in.killMacro(m)
					if ms.res != nil {
						ms.res.ShotsUsed = append(ms.res.ShotsUsed, m)
					}
					in.observe0(EvShotsExhausted, m, rq, text, nil)
				}
			}
			me := in.globals["mecho"]
			mechoLevel := 0
			switch me.String() {
			case "on":
				mechoLevel = 1
			case "all":
				mechoLevel = 2
			}
			inv := 0
			if m.Invisible {
				inv = 1
			}
			if mechoLevel > inv {
				label := "TRIGGER"
				hn := ""
				if hook {
					hn, label = hookTable[rq.hook].Name, " HOOK"
				}
				in.Host.Echo(strings.Repeat(in.globals["mprefix"].String(), in.depth+ms.matchDepth) + hn + label + ": /" + mr.Name + " " + text)
			}
			if m.Body != "" {
				in.observe0(EvRun, m, rq, text, nil)
				in.runBody(m, text, caps, hook)
				mr.Ran = true
				mr.Counted = !m.Quiet
				if mr.Counted {
					ran++
				}
			}
		} else {
			mr.Skipped = "prob"
			in.observe0(EvSkipProb, m, rq, text, nil)
		}
	}
	record()
	return ran
}

// runBody runs a trigger/hook macro's body with the matched text as its
// arguments and the regexp sub-matches visible as %Pn.
func (in *Interp) runBody(m *Macro, text string, caps []int, hook bool) {
	if in.depth >= in.maxRecur() {
		in.warn("recursion count exceeded %%max_recur (%d)", in.maxRecur())
		return
	}
	if hook {
		m.Used[UsedHook]++
	} else {
		m.Used[UsedTrig]++
	}
	name := m.Name
	if name == "" {
		name = "#" + strconv.Itoa(m.ID)
	}
	in.depth++
	oldCmd := in.curCmd
	in.curCmd = name
	defer func() { in.curCmd = oldCmd }()
	if m.origin != "" {
		in.pushOrigin(m.origin)
		defer in.popOrigin()
	}
	f := in.pushFrame(name, text)
	if caps != nil {
		f.regs, f.pl, f.pr = capStrings(text, caps)
	}
	oldExp := in.noExp
	in.noExp = false
	if m.native != nil {
		m.native(in, text)
	} else {
		in.execText(m.Body)
	}
	in.noExp = oldExp
	in.ctl = ctlNone
	in.popFrame()
	in.depth--
}

// capStrings turns offset pairs into the %P0.. strings and %PL/%PR.
func capStrings(text string, caps []int) (regs []string, l, r string) {
	for i := 0; i+1 < len(caps); i += 2 {
		if caps[i] < 0 {
			regs = append(regs, "")
		} else {
			regs = append(regs, text[caps[i]:caps[i+1]])
		}
	}
	if len(caps) >= 2 && caps[0] >= 0 {
		l, r = text[:caps[0]], text[caps[1]:]
	}
	return regs, l, r
}

// noteCall is called when a macro is invoked by name (accounting and
// observers).
func (in *Interp) noteCall(m *Macro) {
	m.Used[UsedName]++
	if in.mac != nil && len(in.mac.obs) > 0 {
		in.observe(MacroEvent{Kind: EvCalled, Macro: m})
	}
}

// ---------------------------------------------------------------- ProcessLine

// ProcessLine runs a line received from the current world through the
// triggers and reports whether it was gagged; see Process and ProcessText for
// the richer interfaces.
func (in *Interp) ProcessLine(line string) (gag bool) {
	return in.Process(line, true).Gag
}

// Process runs the trigger stage over one received line and reports what
// matched (see LineResult). With live false it only matches: no macro body
// runs and no shot is consumed, so a client can recompute the display effects
// of replayed history without side effects. It is ProcessText for the
// current world with the line's text only.
func (in *Interp) Process(line string, live bool) LineResult {
	return *in.ProcessText(LineInput{Text: line, Dry: !live})
}

// ProcessText is what a client calls for each line of text received from a
// world, before displaying it: it selects and runs triggers by priority,
// applies their attributes (gag, hilite, partial hilites ...), honours
// /substitute, and reports everything in the LineResult. The client then
// displays Result.Line unless Result.Gag, logs unless NoLog, records in
// history unless NoHistory, and counts activity unless NoActivity.
func (in *Interp) ProcessText(li LineInput) *LineResult {
	ms := in.macs()
	for _, f := range ms.pre {
		if !(*f)(in, &li) {
			line := AttrString{Text: li.Text, Attr: li.Attr}
			if li.Line != nil {
				line = *li.Line
			}
			return &LineResult{Original: line.Text, Line: line, Suppressed: true}
		}
	}
	var line AttrString
	if li.Line != nil {
		line = *li.Line
		line.Chars = append([]Attr(nil), line.Chars...)
		if li.Line.Chars == nil {
			line.Chars = nil
		}
	} else {
		line = AttrString{Text: li.Text, Attr: li.Attr}
	}
	res := &LineResult{Original: line.Text}
	world := li.World
	if world == "" {
		world = in.ctxWorld()
	}
	oldInc, oldRes := ms.incoming, ms.res
	ms.incoming, ms.res = &line, res
	rq := &matchReq{hook: -1, line: &line, world: world, global: !li.WorldOnly, wlocal: li.WorldOnly, dry: li.Dry}
	if li.GlobalOnly {
		rq.world = ""
	}
	res.Count = in.findAndRun(rq)
	ms.incoming, ms.res = oldInc, oldRes
	res.Line = line
	res.Substituted = line.Text != res.Original
	res.Gag = line.Attr&AttrGag != 0
	res.NoLog = line.Attr&AttrNoLog != 0
	res.NoHistory = line.Attr&AttrNoHistory != 0
	res.NoActivity = line.Attr&AttrNoActivity != 0
	res.Bell = line.Attr&AttrBell != 0
	for _, f := range ms.post {
		(*f)(in, res)
	}
	return res
}

// ---------------------------------------------------------------- /trigger, substitute

func cmdTrigger(in *Interp, args string) Value {
	if !in.flag("borg") {
		return Int(0)
	}
	rest := strings.TrimLeft(args, " \t")
	var world string
	useDefault, global := true, false
	hook := -1
	list := 0
	r := &optReader{in: in, cmd: "TRIGGER", s: rest, opts: "gw:h:nl"}
	for {
		c, arg, _, _ := r.next()
		if c == 0 {
			break
		}
		switch c {
		case '?':
			return Int(0)
		case 'g':
			useDefault, global = false, true
		case 'w':
			useDefault = false
			if arg == "" {
				arg = in.ctxWorld()
			} else if wi, ok := in.Host.(WorldInfoHost); ok && !wi.WorldExists(arg) {
				in.cmdErr("TRIGGER", 'w', "No world %s", arg)
				return Int(0)
			}
			world = arg
		case 'h':
			if hook = in.hookByName(arg); hook < 0 {
				return Int(0)
			}
		case 'n':
			if list == 0 {
				list = 1
			}
		case 'l':
			list = 2
		}
	}
	text := r.s[r.off:]
	if useDefault {
		global = true
		world = in.ctxWorld()
	}
	ms := in.macs()
	line := AttrString{Text: text}
	oldInc, oldRes := ms.incoming, ms.res
	ms.incoming, ms.res = &line, nil
	rq := &matchReq{hook: hook, text: text, world: world, global: global, list: list}
	if hook >= 0 {
		rq.line = nil
	} else {
		rq.line = &line
	}
	n := in.findAndRun(rq)
	ms.incoming, ms.res = oldInc, oldRes
	return Int(int64(n))
}

// substitute replaces the text of the line being triggered.
func (in *Interp) substitute(text string, attrs string, inline bool) bool {
	ms := in.macs()
	if ms.incoming == nil {
		in.cmdErr("", 0, "not called from trigger")
		return false
	}
	ctx := in.attrCtx()
	a, _, err := ctx.Parse(attrs, 0)
	if err != nil {
		in.warn("%v", err)
		return false
	}
	src := AttrString{Text: text}
	var ns AttrString
	if inline {
		if ns, err = ctx.DecodeAttr(src, 0, 0); err != nil {
			in.warn("%v", err)
			return false
		}
	} else {
		ns = src
	}
	ns.Attr = ctx.Adj(ms.incoming.Attr, src.Attr)
	ns.Attr = ctx.Adj(ns.Attr, a)
	*ms.incoming = ns
	if len(ms.obs) > 0 {
		in.observe(MacroEvent{Kind: EvSubstitute, Text: ns.Text})
	}
	return true
}

func fnSubstitute(in *Interp, a []Value) (Value, error) {
	inline := false
	if len(a) > 2 {
		var ok bool
		if inline, ok = in.flagArg(str(a, 2), "arg 3 (inline)"); !ok {
			return Int(0), nil
		}
	}
	return Bool(in.substitute(str(a, 0), str(a, 1), inline)), nil
}

// split_args: "pattern = body".
func splitArgs(s string) (pattern, body string) {
	if i := strings.IndexByte(s, '='); i >= 0 {
		return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:])
	}
	return strings.TrimSpace(s), ""
}

// addNewMacro is add_new_macro: the shorthand commands build their macros
// through it.
func (in *Interp) addNewMacro(trig string, hook uint64, hasHook bool, hargs string, hasHargs bool, body string, pri, prob int, attr Attr) int {
	m := &Macro{Body: body, Priority: pri, Prob: prob, Attr: attr, fromSpec: true, Matching: in.defaultStyle()}
	style := in.defaultStyle()
	var err error
	if trig != "" || !hasHook {
		if m.Trig, err = in.compilePattern(style, trig); err != nil {
			in.warn("%v", err)
			return 0
		}
	}
	if hasHook {
		m.IsHook, m.Hooks = true, hook
		if hasHargs {
			if m.HArgs, err = in.compilePattern(style, hargs); err != nil {
				in.warn("%v", err)
				return 0
			}
		}
	}
	return in.completeMacro(m, 0)
}

func cmdHilite(in *Interp, args string) Value {
	if strings.TrimSpace(args) == "" {
		in.setGlobal("hilite", Int(1))
		in.echo("% Hilites enabled.")
		return Int(0)
	}
	p, b := splitArgs(args)
	return Int(int64(in.addNewMacro(p, 0, false, "", false, b, int(in.globals["hpri"].Int()), 100, AttrHilite)))
}

func cmdGag(in *Interp, args string) Value {
	if strings.TrimSpace(args) == "" {
		in.setGlobal("gag", Int(1))
		in.echo("% Gags enabled.")
		return Int(0)
	}
	p, b := splitArgs(args)
	return Int(int64(in.addNewMacro(p, 0, false, "", false, b, int(in.globals["gpri"].Int()), 100, AttrGag)))
}

func cmdTrigpc(in *Interp, args string) Value {
	rest := strings.TrimLeft(args, " \t")
	num := func() int {
		w, r := firstWord(rest)
		if w == "" || w[0] < '0' || w[0] > '9' {
			in.cmdErr("TRIGPC", 0, "invalid or missing numeric argument")
			rest = strings.TrimLeft(r, " \t")
			return -1
		}
		rest = strings.TrimLeft(r, " \t")
		n, _ := strconv.Atoi(strings.TrimRightFunc(w, func(c rune) bool { return c < '0' || c > '9' }))
		return n
	}
	pri := num()
	if pri < 0 {
		return Int(0)
	}
	prob := num()
	if prob < 0 {
		return Int(0)
	}
	p, b := splitArgs(rest)
	return Int(int64(in.addNewMacro(p, 0, false, "", false, b, pri, prob, 0)))
}

func init() {
	builtinCmds["trigger"] = cmdTrigger
	builtinCmds["hilite"] = cmdHilite
	builtinCmds["gag"] = cmdGag
	builtinCmds["trigpc"] = cmdTrigpc
	builtinFuncs["substitute"] = fnSubstitute
}
