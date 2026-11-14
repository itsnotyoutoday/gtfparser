package gtfparser

// The sections of the state document (see state.go): for each, the JSON
// shape, the writer, the decoder (which builds the objects the apply step
// installs, changing nothing in the Interp) and the apply step (which cannot
// fail: everything that can has been done by the decoder).

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// stLoad holds what the decoders built.
type stLoad struct {
	seen map[string]bool

	interp  sInterp
	globals map[string]Value
	locals  map[string]Value
	mac     builtMacros
	worlds  builtWorlds
	hist    builtHistory
	procs   []*Proc
	remain  []time.Duration
	streams []sStream
	term    sTerminal
	status  statusState
	prompts map[string]string
	echoOff map[string]bool
	limit   map[string]*limitFilter
	watch   sWatch
	mail    map[string]*mailInfo
	origins sOrigins
}

func (ld *stLoad) check() error { return nil }

func jdec(raw json.RawMessage, v any) error {
	if t := bytes.TrimSpace(raw); len(t) == 0 || string(t) == "null" {
		return fmt.Errorf("%w: empty section", ErrStateCorrupt)
	}
	return json.Unmarshal(raw, v)
}

func trueKeys(m map[string]bool) []bs {
	var out []bs
	for _, k := range sortedKeys(m) {
		if m[k] {
			out = append(out, bs(k))
		}
	}
	return out
}

func boolSet(l []bs) map[string]bool {
	m := map[string]bool{}
	for _, s := range l {
		m[string(s)] = true
	}
	return m
}

func (in *Interp) rootFrame() *frame {
	f := in.fr
	for f.parent != nil {
		f = f.parent
	}
	return f
}

// ---------------------------------------------------------------- interp

type sInterp struct {
	NextID       int  `json:"nextID"`
	NextPID      int  `json:"nextPID"`
	NextFd       int  `json:"nextFd"`
	LastRet      sVal `json:"lastRet"`
	RetV         sVal `json:"retv"`
	UserRes      sVal `json:"userRes"`
	SubWarned    bool `json:"subWarned,omitempty"`
	DollarWarned bool `json:"dollarWarned,omitempty"`
	Restriction  int  `json:"restriction"`
	NativeSeen   []bs `json:"nativeSeen,omitempty"`
	Loaded       []bs `json:"loaded,omitempty"`
	Exported     []bs `json:"exported,omitempty"`

	lastRet, retv, userRes Value
}

func secSaveInterp(in *Interp, c *saveCtx) any {
	return sInterp{
		NextID: in.nextID, NextPID: in.nextPID, NextFd: in.nextFd,
		LastRet: encVal(in.lastRet), RetV: encVal(in.retv), UserRes: encVal(in.userRes),
		SubWarned: in.subWarned, DollarWarned: in.dollarWarned,
		Restriction: in.restriction,
		NativeSeen:  trueKeys(in.nativeSeen), Loaded: trueKeys(in.loaded), Exported: trueKeys(in.exported),
	}
}

func secDecInterp(in *Interp, v int, raw json.RawMessage, ld *stLoad) error {
	s := &ld.interp
	if err := jdec(raw, s); err != nil {
		return err
	}
	var err error
	if s.lastRet, err = decVal(s.LastRet); err != nil {
		return err
	}
	if s.retv, err = decVal(s.RetV); err != nil {
		return err
	}
	if s.userRes, err = decVal(s.UserRes); err != nil {
		return err
	}
	if s.NextID < 0 || s.NextPID < 0 || s.NextFd < 0 || s.Restriction < 0 || s.Restriction > RestrictWorld {
		return fmt.Errorf("%w: counter out of range", ErrStateCorrupt)
	}
	return nil
}

func secApplyInterp(in *Interp, ld *stLoad) {
	s := &ld.interp
	in.nextID, in.nextPID, in.nextFd = s.NextID, s.NextPID, s.NextFd
	in.lastRet, in.retv, in.userRes = s.lastRet, s.retv, s.userRes
	in.subWarned, in.dollarWarned = s.SubWarned, s.DollarWarned
	if s.Restriction > in.restriction { // a restriction is never lifted by a load
		in.restriction = s.Restriction
	}
	in.nativeSeen = nil
	if len(s.NativeSeen) > 0 {
		in.nativeSeen = boolSet(s.NativeSeen)
	}
	in.loaded = boolSet(s.Loaded)
	in.exported = boolSet(s.Exported)
}

// ---------------------------------------------------------------- globals and locals

type sNV struct {
	N bs   `json:"n"`
	V sVal `json:"v"`
}

type sGlobals struct {
	Vars []sNV `json:"vars"`
}

func encVars(m map[string]Value) []sNV {
	out := make([]sNV, 0, len(m))
	for _, n := range sortedKeys(m) {
		out = append(out, sNV{bs(n), encVal(m[n])})
	}
	return out
}

func decVars(l []sNV) (map[string]Value, error) {
	m := make(map[string]Value, len(l))
	for _, e := range l {
		v, err := decVal(e.V)
		if err != nil {
			return nil, fmt.Errorf("variable %q: %v", string(e.N), err)
		}
		m[string(e.N)] = v
	}
	return m, nil
}

func secSaveGlobals(in *Interp, c *saveCtx) any { return sGlobals{Vars: encVars(in.globals)} }

func secDecGlobals(in *Interp, v int, raw json.RawMessage, ld *stLoad) error {
	var s sGlobals
	if err := jdec(raw, &s); err != nil {
		return err
	}
	m, err := decVars(s.Vars)
	if err != nil {
		return err
	}
	ld.globals = m
	return nil
}

func secApplyGlobals(in *Interp, ld *stLoad) {
	for _, keep := range []string{"TFVERSION", "version"} { // what this build says about itself
		if v, ok := in.globals[keep]; ok {
			ld.globals[keep] = v
		}
	}
	in.globals = ld.globals
}

func secSaveLocals(in *Interp, c *saveCtx) any { return sGlobals{Vars: encVars(in.rootFrame().vars)} }

func secDecLocals(in *Interp, v int, raw json.RawMessage, ld *stLoad) error {
	var s sGlobals
	if err := jdec(raw, &s); err != nil {
		return err
	}
	m, err := decVars(s.Vars)
	ld.locals = m
	return err
}

func secApplyLocals(in *Interp, ld *stLoad) { in.rootFrame().vars = ld.locals }

// ---------------------------------------------------------------- macros

type sPat struct {
	S     bs `json:"s"`
	Style bs `json:"style"`
}

type sSub struct {
	Subexp int    `json:"n"`
	Attr   uint32 `json:"a"`
}

type sMacro struct {
	ID        int      `json:"id"`
	Name      bs       `json:"name,omitempty"`
	Body      bs       `json:"body,omitempty"`
	Cond      bs       `json:"cond,omitempty"`
	Bind      bs       `json:"bind,omitempty"`
	Keyname   bs       `json:"keyname,omitempty"`
	Trig      *sPat    `json:"trig,omitempty"`
	HArgs     *sPat    `json:"hargs,omitempty"`
	WType     *sPat    `json:"wtype,omitempty"`
	Hooks     []string `json:"hooks,omitempty"` // event names
	IsHook    bool     `json:"isHook,omitempty"`
	World     bs       `json:"world,omitempty"`
	Priority  int      `json:"pri"`
	Prob      int      `json:"prob"`
	Shots     int      `json:"shots,omitempty"`
	Fall      bool     `json:"fall,omitempty"`
	Quiet     bool     `json:"quiet,omitempty"`
	Invisible bool     `json:"invisible,omitempty"`
	Attr      uint32   `json:"attr,omitempty"`
	MatchAttr uint32   `json:"matchAttr,omitempty"`
	CheckAttr bool     `json:"checkAttr,omitempty"`
	Sub       []sSub   `json:"sub,omitempty"`
	Used      []int    `json:"used,omitempty"`
	Matching  bs       `json:"matching,omitempty"`
	Native    bool     `json:"native,omitempty"`
}

type sIDList struct {
	Key bs    `json:"key"`
	IDs []int `json:"ids"`
}

type sMacros struct {
	Macros     []sMacro  `json:"macros"` // in number order
	TrigGlobal []int     `json:"trigGlobal,omitempty"`
	TrigWorld  []sIDList `json:"trigWorld,omitempty"` // key: lower-cased world
	Hooks      []sIDList `json:"hooks,omitempty"`     // key: event name
	HookOn     []string  `json:"hookOn,omitempty"`
}

type builtMacros struct {
	list       []*Macro
	trigGlobal []*Macro
	trigWorld  map[string][]*Macro
	hooks      [64][]*Macro
	hookOn     [64]bool
}

func encPat(p *Pattern) *sPat {
	if p == nil {
		return nil
	}
	return &sPat{bs(p.Str), bs(p.Style)}
}

func listIDs(l *mlist) []int {
	var ids []int
	for n := l.head; n != nil; n = n.next {
		if !n.m.dead {
			ids = append(ids, n.m.ID)
		}
	}
	return ids
}

func secSaveMacros(in *Interp, c *saveCtx) any {
	s := sMacros{Macros: []sMacro{}}
	ms := in.macs()
	for _, m := range in.list {
		sm := sMacro{ID: m.ID, Name: bs(m.Name), Body: bs(m.Body), Cond: bs(m.Cond), Bind: bs(m.Bind), Keyname: bs(m.Keyname),
			Trig: encPat(m.Trig), HArgs: encPat(m.HArgs), WType: encPat(m.WType), IsHook: m.IsHook,
			World: bs(m.World), Priority: m.Priority, Prob: m.Prob, Shots: m.Shots, Fall: m.Fall, Quiet: m.Quiet,
			Invisible: m.Invisible, Attr: uint32(m.Attr), MatchAttr: uint32(m.MatchAttr), CheckAttr: m.CheckMatchAttr,
			Matching: bs(m.Matching), Native: m.native != nil}
		for i := range hookTable {
			if m.Hooks&(1<<uint(i)) != 0 {
				sm.Hooks = append(sm.Hooks, hookTable[i].Name)
			}
		}
		for _, a := range m.SubAttr {
			sm.Sub = append(sm.Sub, sSub{a.Subexp, uint32(a.Attr)})
		}
		for _, n := range m.Used {
			if n != 0 {
				sm.Used = append([]int(nil), m.Used[:]...)
				break
			}
		}
		s.Macros = append(s.Macros, sm)
	}
	s.TrigGlobal = listIDs(&ms.trigGlobal)
	for _, k := range sortedKeys(ms.trigWorld) {
		if ids := listIDs(ms.trigWorld[k]); len(ids) > 0 {
			s.TrigWorld = append(s.TrigWorld, sIDList{bs(k), ids})
		}
	}
	for i := range hookTable {
		if ids := listIDs(&ms.hooks[i]); len(ids) > 0 {
			s.Hooks = append(s.Hooks, sIDList{bs(hookTable[i].Name), ids})
		}
		if ms.hookOn[i] {
			s.HookOn = append(s.HookOn, hookTable[i].Name)
		}
	}
	return s
}

func (in *Interp) buildPat(p *sPat, who string, id int) (*Pattern, error) {
	if p == nil {
		return nil, nil
	}
	pat, err := in.compilePattern(string(p.Style), string(p.S))
	if err != nil {
		return nil, fmt.Errorf("%w: macro #%d %s pattern %q (style %s): %v", ErrStateIncompatible, id, who, string(p.S), string(p.Style), err)
	}
	return pat, nil
}

func secDecMacros(in *Interp, v int, raw json.RawMessage, ld *stLoad) error {
	var s sMacros
	if err := jdec(raw, &s); err != nil {
		return err
	}
	b := &ld.mac
	byID := map[int]*Macro{}
	prev := 0
	for _, sm := range s.Macros {
		if sm.ID <= prev {
			return fmt.Errorf("%w: macro numbers are not increasing at #%d", ErrStateCorrupt, sm.ID)
		}
		prev = sm.ID
		m := &Macro{ID: sm.ID, Name: string(sm.Name), Body: string(sm.Body), Cond: string(sm.Cond),
			Bind: string(sm.Bind), Keyname: string(sm.Keyname), IsHook: sm.IsHook, World: string(sm.World),
			Priority: sm.Priority, Prob: sm.Prob, Shots: sm.Shots, Fall: sm.Fall, Quiet: sm.Quiet,
			Invisible: sm.Invisible, Attr: Attr(sm.Attr), MatchAttr: Attr(sm.MatchAttr), CheckMatchAttr: sm.CheckAttr,
			Matching: string(sm.Matching), fromSpec: true}
		var err error
		if m.Trig, err = in.buildPat(sm.Trig, "trigger", sm.ID); err != nil {
			return err
		}
		if m.HArgs, err = in.buildPat(sm.HArgs, "hook-argument", sm.ID); err != nil {
			return err
		}
		if m.WType, err = in.buildPat(sm.WType, "world-type", sm.ID); err != nil {
			return err
		}
		for _, h := range sm.Hooks {
			i := hookIndex(h)
			if i < 0 {
				return fmt.Errorf("%w: macro #%d hooks unknown event %q", ErrStateIncompatible, sm.ID, h)
			}
			m.Hooks |= 1 << uint(i)
		}
		for _, a := range sm.Sub {
			m.SubAttr = append(m.SubAttr, SubAttr{a.Subexp, Attr(a.Attr)})
		}
		if len(m.SubAttr) > 0 {
			if m.Trig == nil || m.Trig.kind != pkRegexp {
				return fmt.Errorf("%w: macro #%d has -P attributes without a regexp trigger", ErrStateCorrupt, sm.ID)
			}
			m.nsub = m.Trig.re.groups()
		}
		m.relAttr = m.Attr&AttrDisplay != 0 || len(m.SubAttr) > 0
		copy(m.Used[:], sm.Used)
		if sm.Native && !in.noNative {
			m.native = nativeLib[m.Name]
		}
		m.syncText()
		byID[m.ID] = m
		b.list = append(b.list, m)
	}
	ids := func(l []int, what string) ([]*Macro, error) {
		var out []*Macro
		for _, id := range l {
			m := byID[id]
			if m == nil {
				return nil, fmt.Errorf("%w: %s names unknown macro #%d", ErrStateCorrupt, what, id)
			}
			out = append(out, m)
		}
		return out, nil
	}
	var err error
	if b.trigGlobal, err = ids(s.TrigGlobal, "trigger list"); err != nil {
		return err
	}
	b.trigWorld = map[string][]*Macro{}
	for _, l := range s.TrigWorld {
		if b.trigWorld[string(l.Key)], err = ids(l.IDs, "world trigger list"); err != nil {
			return err
		}
	}
	for _, l := range s.Hooks {
		i := hookIndex(string(l.Key))
		if i < 0 {
			return fmt.Errorf("%w: hook list for unknown event %q", ErrStateIncompatible, string(l.Key))
		}
		if b.hooks[i], err = ids(l.IDs, "hook list"); err != nil {
			return err
		}
	}
	for _, h := range s.HookOn {
		if i := hookIndex(h); i >= 0 {
			b.hookOn[i] = true
		}
	}
	return nil
}

func (l *mlist) appendNode(m *Macro) *mnode {
	n := &mnode{m: m, prev: l.tail}
	if l.tail != nil {
		l.tail.next = n
	} else {
		l.head = n
	}
	l.tail = n
	return n
}

func secApplyMacros(in *Interp, ld *stLoad) {
	b := &ld.mac
	ms := in.macs()
	in.list = b.list
	in.macros = map[string]*Macro{}
	ms.binds = map[string]*Macro{}
	ms.dead = nil
	ms.trigGlobal = mlist{}
	ms.trigWorld = map[string]*mlist{}
	ms.hooks = [64]mlist{}
	ms.hookOn = b.hookOn
	listed := map[*Macro]bool{}
	for _, m := range b.list {
		if m.Name != "" {
			in.macros[macroKey(m.Name)] = m
		}
		if m.Bind != "" {
			ms.binds[m.Bind] = m
		}
	}
	for _, m := range b.trigGlobal {
		if m.World == "" && m.Trig != nil && !listed[m] {
			listed[m] = true
			m.trigList, m.trigNode = &ms.trigGlobal, ms.trigGlobal.appendNode(m)
		}
	}
	for _, k := range sortedKeys(b.trigWorld) {
		l := &mlist{}
		ms.trigWorld[k] = l
		for _, m := range b.trigWorld[k] {
			if m.World != "" && strings.ToLower(m.World) == k && m.Trig != nil && !listed[m] {
				listed[m] = true
				m.trigList, m.trigNode = l, l.appendNode(m)
			}
		}
	}
	for _, m := range b.list { // a trigger the saved lists did not place goes where a /def would put it
		if m.Trig != nil && !listed[m] {
			in.trigInsert(m)
		}
	}
	for i := range b.hooks {
		for _, m := range b.hooks[i] {
			if m.IsHook && m.Hooks&(1<<uint(i)) != 0 && m.hookNode[i] == nil {
				m.hookNode[i] = ms.hooks[i].appendNode(m)
			}
		}
	}
	for _, m := range b.list {
		if m.IsHook {
			for i := range hookTable {
				if m.Hooks&(1<<uint(i)) != 0 && m.hookNode[i] == nil {
					m.hookNode[i] = ms.hooks[i].insert(m)
				}
			}
		}
		if m.ID > in.nextID {
			in.nextID = m.ID
		}
	}
}

// ---------------------------------------------------------------- histories and worlds

type sLine struct {
	T    bs     `json:"t,omitempty"`
	Time string `json:"time,omitempty"`
	A    bs     `json:"a,omitempty"`
}

type sHist struct {
	Max   int     `json:"max"`
	Total int     `json:"total"`
	Lines []sLine `json:"lines"`
	Log   bs      `json:"log,omitempty"`
}

type sWorld struct {
	Name      bs     `json:"name"`
	Type      bs     `json:"type,omitempty"`
	Host      bs     `json:"host,omitempty"`
	Port      bs     `json:"port,omitempty"`
	Character bs     `json:"char,omitempty"`
	Password  bs     `json:"pass,omitempty"`
	MFile     bs     `json:"mfile,omitempty"`
	SrcHost   bs     `json:"src,omitempty"`
	Charset   bs     `json:"charset,omitempty"`
	NoProxy   bool   `json:"noproxy,omitempty"`
	SSL       bool   `json:"ssl,omitempty"`
	Echo      bool   `json:"echo,omitempty"`
	Temp      bool   `json:"temp,omitempty"`
	Hist      *sHist `json:"hist,omitempty"`
}

type sWorlds struct {
	Worlds  []sWorld `json:"worlds"`
	Default *sWorld  `json:"default,omitempty"`
	Cur     bs       `json:"cur,omitempty"`
	Unnamed int      `json:"unnamed,omitempty"`
}

type builtWorlds struct {
	worlds  []*World
	def     *World
	cur     string
	unnamed int
}

func encHist(h *history, blank func(HistLine) bool) *sHist {
	if h == nil {
		return nil
	}
	s := &sHist{Max: h.max, Total: h.total, Lines: make([]sLine, 0, len(h.lines))}
	for _, l := range h.lines {
		sl := sLine{T: bs(l.Text), Time: encTime(l.Time), A: bs(l.Attrs)}
		if blank != nil && blank(l) {
			sl.T = ""
		}
		s.Lines = append(s.Lines, sl)
	}
	if h.log != nil {
		s.Log = bs(h.log.name)
	}
	return s
}

func decHist(s *sHist) (*history, string, error) {
	if s == nil {
		return &history{}, "", nil
	}
	if s.Max < 0 || s.Total < len(s.Lines) {
		return nil, "", fmt.Errorf("%w: history counters out of range", ErrStateCorrupt)
	}
	h := &history{max: s.Max, total: s.Total, lines: make([]HistLine, 0, len(s.Lines))}
	for _, l := range s.Lines {
		t, err := decTime(l.Time)
		if err != nil {
			return nil, "", fmt.Errorf("%w: bad time %q", ErrStateCorrupt, l.Time)
		}
		h.lines = append(h.lines, HistLine{Text: string(l.T), Time: t, Attrs: string(l.A)})
	}
	return h, string(s.Log), nil
}

func encWorld(w *World, withHist bool) sWorld {
	sw := sWorld{Name: bs(w.Name), Type: bs(w.Type), Host: bs(w.Host), Port: bs(w.Port), Character: bs(w.Character),
		Password: bs(w.Password), MFile: bs(w.MFile), SrcHost: bs(w.SrcHost), Charset: bs(w.Charset),
		NoProxy: w.NoProxy, SSL: w.SSL, Echo: w.Echo, Temp: w.Temp}
	if withHist {
		sw.Hist = encHist(w.hist, nil)
		if sw.Hist == nil {
			sw.Hist = &sHist{Lines: []sLine{}}
		}
	}
	return sw
}

func secSaveWorlds(in *Interp, c *saveCtx) any {
	db := &in.ds().wd
	s := sWorlds{Worlds: []sWorld{}, Cur: bs(db.cur), Unnamed: db.unnamed}
	for _, w := range db.worlds {
		s.Worlds = append(s.Worlds, encWorld(w, true))
	}
	if db.def != nil {
		d := encWorld(db.def, false)
		s.Default = &d
	}
	return s
}

func decWorld(sw sWorld, ld *stLoad) (*World, error) {
	w := &World{Name: string(sw.Name), Type: string(sw.Type), Host: string(sw.Host), Port: string(sw.Port),
		Character: string(sw.Character), Password: string(sw.Password), MFile: string(sw.MFile),
		SrcHost: string(sw.SrcHost), Charset: string(sw.Charset), NoProxy: sw.NoProxy, SSL: sw.SSL, Echo: sw.Echo, Temp: sw.Temp}
	if sw.Hist != nil {
		h, log, err := decHist(sw.Hist)
		if err != nil {
			return nil, err
		}
		w.hist = h
		if log != "" {
			ld.hist.logs = append(ld.hist.logs, logSpec{h, log})
		}
	}
	return w, nil
}

func secDecWorlds(in *Interp, v int, raw json.RawMessage, ld *stLoad) error {
	var s sWorlds
	if err := jdec(raw, &s); err != nil {
		return err
	}
	b := &ld.worlds
	seen := map[string]bool{}
	for _, sw := range s.Worlds {
		w, err := decWorld(sw, ld)
		if err != nil {
			return err
		}
		if w.hist == nil {
			w.hist = &history{}
		}
		k := strings.ToLower(w.Name)
		if k == "" || seen[k] {
			return fmt.Errorf("%w: duplicate or empty world name %q", ErrStateCorrupt, w.Name)
		}
		seen[k] = true
		b.worlds = append(b.worlds, w)
	}
	if s.Default != nil {
		w, err := decWorld(*s.Default, ld)
		if err != nil {
			return err
		}
		b.def = w
	}
	b.cur, b.unnamed = string(s.Cur), s.Unnamed
	return nil
}

func secApplyWorlds(in *Interp, ld *stLoad) {
	d := in.ds()
	for _, w := range d.wd.worlds { // the old histories' log files
		in.dropLog(w.hist)
	}
	b := &ld.worlds
	d.wd = worldDB{worlds: b.worlds, def: b.def, cur: b.cur, unnamed: b.unnamed}
}

// ---------------------------------------------------------------- input and other histories

type sHistory struct {
	Input    *sHist `json:"input"`
	Local    *sHist `json:"local"`
	Global   *sHist `json:"global"`
	InputIdx int    `json:"inputIdx"`
}

type logSpec struct {
	h    *history
	name string
}

type builtHistory struct {
	input, local, global *history
	inputIdx             int
	logs                 []logSpec
}

func secSaveHistory(in *Interp, c *saveCtx) any {
	d := in.ds()
	// lines typed with echo off (recorded with the g attribute) never leave
	// the process, and neither does the line being typed if echo is off now
	last := len(d.h.input.lines) - 1
	n := 0
	blankInput := func(l HistLine) bool {
		n++
		return hasAttr(l.Attrs, AttrGag) || (c.noEcho && n-1 == last)
	}
	return sHistory{
		Input: encHist(d.h.input, blankInput), Local: encHist(d.h.local, nil), Global: encHist(d.h.global, nil),
		InputIdx: d.h.inputIdx,
	}
}

func secDecHistory(in *Interp, v int, raw json.RawMessage, ld *stLoad) error {
	var s sHistory
	if err := jdec(raw, &s); err != nil {
		return err
	}
	b := &ld.hist
	for _, x := range []struct {
		s *sHist
		d **history
	}{{s.Input, &b.input}, {s.Local, &b.local}, {s.Global, &b.global}} {
		if x.s == nil {
			return fmt.Errorf("%w: missing history", ErrStateCorrupt)
		}
		h, log, err := decHist(x.s)
		if err != nil {
			return err
		}
		*x.d = h
		if log != "" {
			b.logs = append(b.logs, logSpec{h, log})
		}
	}
	if len(b.input.lines) == 0 {
		return fmt.Errorf("%w: empty input history", ErrStateCorrupt)
	}
	b.inputIdx = s.InputIdx
	return nil
}

// dropLog closes the log file of a history that is being replaced.
func (in *Interp) dropLog(h *history) {
	if h != nil && h.log != nil {
		h.log.f.Close()
		h.log = nil
	}
}

func secApplyHistory(in *Interp, ld *stLoad) {
	d := in.ds()
	for _, h := range []*history{d.h.input, d.h.local, d.h.global} {
		in.dropLog(h)
	}
	b := &ld.hist
	d.h = histState{input: b.input, local: b.local, global: b.global, inputIdx: b.inputIdx}
}

// reopenLogs resumes logging into the files the saved histories were logging
// to, appending (a file that cannot be opened is reported and dropped).
func (in *Interp) reopenLogs(ld *stLoad) {
	d := in.ds()
	for _, ls := range ld.hist.logs {
		f, err := os.OpenFile(ls.name, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			in.Warn(fmt.Sprintf("state: cannot reopen log %s: %v", ls.name, errText(err)))
			continue
		}
		ls.h.log = &logFile{name: ls.name, f: f}
	}
	d.lg.n = 0 // nlog(): the open logs
	for _, h := range []*history{d.h.input, d.h.local, d.h.global} {
		if h.log != nil {
			d.lg.n++
		}
	}
	for _, w := range d.wd.worlds {
		if w.hist != nil && w.hist.log != nil {
			d.lg.n++
		}
	}
}

// ---------------------------------------------------------------- processes

type sProc struct {
	PID      int    `json:"pid"`
	Kind     string `json:"kind"`
	Remain   int64  `json:"remain"` // ns until it runs (negative: overdue)
	Delay    int64  `json:"delay"`
	Count    int    `json:"count"`
	Cmd      bs     `json:"cmd"`
	World    bs     `json:"world,omitempty"`
	OnPrompt bool   `json:"onPrompt,omitempty"`
	Disp     string `json:"disp,omitempty"`
	Lines    []bs   `json:"lines,omitempty"`
	Pre      bs     `json:"pre,omitempty"`
	Suf      bs     `json:"suf,omitempty"`
}

type sProcs struct {
	Procs []sProc `json:"procs"`
}

func secSaveProcs(in *Interp, c *saveCtx) any {
	s := sProcs{Procs: []sProc{}}
	ps := append([]*Proc(nil), in.procs...)
	sort.SliceStable(ps, func(i, j int) bool { return ps[i].PID < ps[j].PID })
	for _, p := range ps {
		sp := sProc{PID: p.PID, Kind: string(p.Kind), Remain: int64(p.Next.Sub(c.now)), Delay: int64(p.Delay), Count: p.Count,
			Cmd: bs(p.Cmd), World: bs(p.World), OnPrompt: p.OnPromp, Pre: bs(p.Pre), Suf: bs(p.Suf)}
		if p.Disp != 0 {
			sp.Disp = string(p.Disp)
		}
		for _, l := range p.Lines {
			sp.Lines = append(sp.Lines, bs(l))
		}
		s.Procs = append(s.Procs, sp)
	}
	return s
}

func secDecProcs(in *Interp, v int, raw json.RawMessage, ld *stLoad) error {
	var s sProcs
	if err := jdec(raw, &s); err != nil {
		return err
	}
	seen := map[int]bool{}
	for _, sp := range s.Procs {
		if sp.PID <= 0 || seen[sp.PID] || (sp.Kind != "r" && sp.Kind != "q") || len(sp.Disp) > 1 {
			return fmt.Errorf("%w: bad process #%d", ErrStateCorrupt, sp.PID)
		}
		seen[sp.PID] = true
		p := &Proc{PID: sp.PID, Kind: sp.Kind[0], Delay: time.Duration(sp.Delay), Count: sp.Count, Cmd: string(sp.Cmd),
			World: string(sp.World), OnPromp: sp.OnPrompt, Pre: string(sp.Pre), Suf: string(sp.Suf)}
		if sp.Disp != "" {
			p.Disp = sp.Disp[0]
		}
		for _, l := range sp.Lines {
			p.Lines = append(p.Lines, string(l))
		}
		ld.procs = append(ld.procs, p)
		ld.remain = append(ld.remain, time.Duration(sp.Remain))
	}
	return nil
}

func secApplyProcs(in *Interp, ld *stLoad) {
	now := in.now()
	in.procs = nil
	for i, p := range ld.procs {
		p.Next = now.Add(ld.remain[i])
		in.procs = append(in.procs, p)
		if p.PID > in.nextPID {
			in.nextPID = p.PID
		}
	}
}

// ---------------------------------------------------------------- streams

type sStream struct {
	ID        bs     `json:"id"`
	Mode      string `json:"mode"`
	Name      bs     `json:"name,omitempty"`
	Lines     []bs   `json:"lines,omitempty"`
	Offset    int64  `json:"offset,omitempty"`
	EOF       bool   `json:"eof,omitempty"`
	AutoFlush bool   `json:"autoflush,omitempty"`
}

type sStreams struct {
	Streams []sStream `json:"streams"`
}

func secSaveStreams(in *Interp, c *saveCtx) any {
	s := sStreams{Streams: []sStream{}}
	ids := make([]int, 0, len(in.streams))
	for id := range in.streams {
		n, _ := strconv.Atoi(id)
		ids = append(ids, n)
	}
	sort.Ints(ids)
	for _, n := range ids {
		st := in.streams[strconv.Itoa(n)]
		if st == nil || st.cmd != nil { // a pipe is a child process: it cannot move
			continue
		}
		ss := sStream{ID: bs(st.id), Mode: string(st.mode), Name: bs(st.name), EOF: st.eof, AutoFlush: st.autoflush}
		if st.queue {
			for _, l := range st.lines {
				ss.Lines = append(ss.Lines, bs(l))
			}
		} else {
			if st.f == nil {
				continue
			}
			if st.mode != 'a' { // an append stream always writes at the end
				off, err := st.f.Seek(0, io.SeekCurrent)
				if err != nil {
					continue
				}
				if st.r != nil {
					off -= int64(st.r.Buffered())
				}
				ss.Offset = off
			}
		}
		s.Streams = append(s.Streams, ss)
	}
	return s
}

func secDecStreams(in *Interp, v int, raw json.RawMessage, ld *stLoad) error {
	var s sStreams
	if err := jdec(raw, &s); err != nil {
		return err
	}
	for _, ss := range s.Streams {
		if len(ss.Mode) != 1 || !strings.Contains("rwaq", ss.Mode) || ss.Offset < 0 {
			return fmt.Errorf("%w: bad stream %q", ErrStateCorrupt, string(ss.ID))
		}
	}
	ld.streams = s.Streams
	return nil
}

func (in *Interp) closeStreamQuiet(st *stream) {
	if st.f != nil {
		st.f.Close()
	}
	if st.cmd != nil && st.cmd.Process != nil {
		st.cmd.Process.Kill()
		go st.cmd.Wait()
	}
}

func secApplyStreams(in *Interp, ld *stLoad) {
	for _, st := range in.streams {
		in.closeStreamQuiet(st)
	}
	in.streams = nil
	for _, ss := range ld.streams {
		st := &stream{mode: ss.Mode[0], name: string(ss.Name), id: string(ss.ID), autoflush: ss.AutoFlush, eof: ss.EOF}
		switch st.mode {
		case 'q':
			st.queue, st.canRead, st.canWrite = true, true, true
			for _, l := range ss.Lines {
				st.lines = append(st.lines, string(l))
			}
		case 'r':
			f, err := os.Open(st.name)
			if err != nil {
				continue
			}
			if _, err := f.Seek(ss.Offset, io.SeekStart); err != nil {
				f.Close()
				continue
			}
			st.f, st.canRead, st.r = f, true, bufio.NewReader(f)
		case 'w', 'a':
			flag := os.O_WRONLY // 'w': the file exists and keeps its content; writing resumes where it was
			if st.mode == 'a' {
				flag = os.O_APPEND | os.O_CREATE | os.O_WRONLY
			}
			f, err := os.OpenFile(st.name, flag, 0o666)
			if err != nil {
				continue
			}
			if st.mode == 'w' {
				if _, err := f.Seek(ss.Offset, io.SeekStart); err != nil {
					f.Close()
					continue
				}
			}
			st.f, st.canWrite, st.w = f, true, f
		}
		if in.streams == nil {
			in.streams = map[string]*stream{}
		}
		in.streams[st.id] = st
	}
}

// ---------------------------------------------------------------- terminal

type sStatusField struct {
	Kind  int  `json:"kind"` // 0 pad, 1 literal, 2 variable, 3 internal
	Name  bs   `json:"name,omitempty"`
	Width int  `json:"width,omitempty"`
	Right bool `json:"right,omitempty"`
	Attrs bs   `json:"attrs,omitempty"`
}

type sTerminal struct {
	Text    bs               `json:"text,omitempty"` // the keyboard line
	Cursor  int              `json:"cursor,omitempty"`
	HLines  []bs             `json:"hlines"` // the editor's input history, the line being typed last
	HIndex  int              `json:"hindex"`
	LNext   bool             `json:"lnext,omitempty"`
	CurKeys bs               `json:"curKeys,omitempty"`
	Pending bool             `json:"pending,omitempty"`
	Prompts []sNS            `json:"prompts,omitempty"`
	EchoOff []bs             `json:"echoOff,omitempty"`
	Rows    [][]sStatusField `json:"statusRows,omitempty"`
	LastVar bs               `json:"statusLastVar,omitempty"`
	Warned  []bs             `json:"statusWarned,omitempty"`
}

type sNS struct {
	N bs `json:"n"`
	S bs `json:"s"`
}

func secSaveTerminal(in *Interp, c *saveCtx) any {
	ts := in.ts()
	t := sTerminal{HIndex: ts.hist.index, LNext: ts.lnext, CurKeys: bs(ts.curKeys), Pending: ts.pending, LastVar: bs(ts.st.lastVar)}
	if !c.noEcho {
		t.Text, t.Cursor = bs(ts.kb.text), ts.kb.pos
	}
	last := len(ts.hist.lines) - 1
	for i, l := range ts.hist.lines {
		if c.noEcho && i == last {
			l = ""
		}
		t.HLines = append(t.HLines, bs(l))
	}
	if t.HLines == nil {
		t.HLines = []bs{""}
	}
	for _, k := range sortedKeys(ts.prompts) {
		t.Prompts = append(t.Prompts, sNS{bs(k), bs(ts.prompts[k])})
	}
	t.EchoOff = trueKeys(ts.echoOff)
	t.Warned = trueKeys(ts.st.warned)
	for _, row := range ts.st.rows {
		var r []sStatusField
		for _, f := range row {
			r = append(r, sStatusField{Kind: int(f.Kind), Name: bs(f.Name), Width: f.Width, Right: f.RightJust, Attrs: bs(f.Attrs)})
		}
		t.Rows = append(t.Rows, r)
	}
	return t
}

func secDecTerminal(in *Interp, v int, raw json.RawMessage, ld *stLoad) error {
	var t sTerminal
	if err := jdec(raw, &t); err != nil {
		return err
	}
	if len(t.HLines) == 0 || t.HIndex < 0 || t.HIndex >= len(t.HLines) || t.Cursor < 0 || t.Cursor > len(t.Text) || len(t.Rows) > maxStatusHeight {
		return fmt.Errorf("%w: terminal state out of range", ErrStateCorrupt)
	}
	ld.term = t
	ld.prompts = map[string]string{}
	for _, p := range t.Prompts {
		ld.prompts[string(p.N)] = string(p.S)
	}
	ld.echoOff = boolSet(t.EchoOff)
	ld.status = statusState{lastVar: string(t.LastVar), warned: boolSet(t.Warned)}
	for i, row := range t.Rows {
		for _, f := range row {
			if f.Kind < int(StatusPad) || f.Kind > int(StatusInternal) {
				return fmt.Errorf("%w: bad status field kind %d", ErrStateCorrupt, f.Kind)
			}
			ld.status.rows[i] = append(ld.status.rows[i], StatusField{Kind: StatusKind(f.Kind), Name: string(f.Name),
				Width: f.Width, RightJust: f.Right, Attrs: string(f.Attrs)})
		}
	}
	return nil
}

func secApplyTerminal(in *Interp, ld *stLoad) {
	ts := in.ts()
	t := &ld.term
	ts.kb.text, ts.kb.pos = string(t.Text), t.Cursor
	ts.hist.lines = make([]string, len(t.HLines))
	for i, l := range t.HLines {
		ts.hist.lines[i] = string(l)
	}
	ts.hist.index = t.HIndex
	ts.lnext, ts.curKeys, ts.pending = t.LNext, string(t.CurKeys), t.Pending
	ts.prompts, ts.echoOff = ld.prompts, ld.echoOff
	ts.st = ld.status
	ts.kb.changed()
}

// ---------------------------------------------------------------- limit

type sLimit struct {
	Key     bs   `json:"key"`
	Enabled bool `json:"enabled,omitempty"`
	HasPat  bool `json:"hasPat,omitempty"`
	Pat     bs   `json:"pat,omitempty"`
	Style   bs   `json:"style,omitempty"`
	Attr    bool `json:"attr,omitempty"`
	Sense   bool `json:"sense"`
}

type sLimits struct {
	Screens []sLimit `json:"screens"`
}

func secSaveLimit(in *Interp, c *saveCtx) any {
	s := sLimits{Screens: []sLimit{}}
	ls := in.ds().lm
	for _, k := range sortedKeys(ls.screens) {
		f := ls.screens[k]
		s.Screens = append(s.Screens, sLimit{Key: bs(k), Enabled: f.enabled, HasPat: f.hasPat, Pat: bs(f.pat), Style: bs(f.style), Attr: f.attr, Sense: f.sense})
	}
	return s
}

func secDecLimit(in *Interp, v int, raw json.RawMessage, ld *stLoad) error {
	var s sLimits
	if err := jdec(raw, &s); err != nil {
		return err
	}
	ld.limit = map[string]*limitFilter{}
	for _, l := range s.Screens {
		f := &limitFilter{enabled: l.Enabled, hasPat: l.HasPat, pat: string(l.Pat), style: string(l.Style), attr: l.Attr, sense: l.Sense}
		if f.pat != "" {
			m, ok := in.matchStyle("/limit", f.style, f.pat)
			if !ok {
				return fmt.Errorf("%w: /limit pattern %q does not compile", ErrStateIncompatible, f.pat)
			}
			f.match = m
		}
		ld.limit[string(l.Key)] = f
	}
	return nil
}

func secApplyLimit(in *Interp, ld *stLoad) { in.ds().lm.screens = ld.limit }

// ---------------------------------------------------------------- watchdog / watchname

type sWatch struct {
	NMatch [2]int `json:"nmatch"`
	NLines [2]int `json:"nlines"`
}

func secSaveWatch(in *Interp, c *saveCtx) any {
	w := in.ds().w
	return sWatch{w.nmatch, w.nlines}
}

func secDecWatch(in *Interp, v int, raw json.RawMessage, ld *stLoad) error {
	return jdec(raw, &ld.watch)
}

func secApplyWatch(in *Interp, ld *stLoad) {
	w := &in.ds().w
	w.nmatch, w.nlines = ld.watch.NMatch, ld.watch.NLines
}

// ---------------------------------------------------------------- mail

type sMail struct {
	Name  bs    `json:"name"`
	MTime int64 `json:"mtime"`
	Size  int64 `json:"size"`
	Flag  int   `json:"flag,omitempty"`
	Known bool  `json:"known,omitempty"`
}

type sMails struct {
	Files []sMail `json:"files"`
}

func secSaveMail(in *Interp, c *saveCtx) any {
	s := sMails{Files: []sMail{}}
	for _, k := range sortedKeys(in.mail) {
		i := in.mail[k]
		s.Files = append(s.Files, sMail{bs(k), i.mtime, i.size, i.flag, i.known})
	}
	return s
}

func secDecMail(in *Interp, v int, raw json.RawMessage, ld *stLoad) error {
	var s sMails
	if err := jdec(raw, &s); err != nil {
		return err
	}
	ld.mail = map[string]*mailInfo{}
	for _, m := range s.Files {
		ld.mail[string(m.Name)] = &mailInfo{mtime: m.MTime, size: m.Size, flag: m.Flag, known: m.Known}
	}
	return nil
}

func secApplyMail(in *Interp, ld *stLoad) { in.mail = ld.mail }

// ---------------------------------------------------------------- after the apply

// finishLoad redoes what depends on several sections: the log files, the
// world_* variables of the current world, the process-wide %sigfigs.
func (in *Interp) finishLoad(ld *stLoad) {
	if ld.seen["history"] || ld.seen["worlds"] {
		in.reopenLogs(ld)
	}
	if ld.seen["worlds"] {
		in.refreshWorldVars()
	}
	if ld.seen["globals"] {
		if v, ok := in.globals["sigfigs"]; ok {
			setSigFigs(int(v.Int()))
		}
	}
}
