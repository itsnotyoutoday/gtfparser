package gtfparser

// The world database (/addworld, addworld(), /unworld, /renameworld,
// /listworlds, /saveworld, world_info()), and the connection commands and
// functions that sit on top of it (/connect, /dc, /fg, /listsockets,
// is_connected(), is_open(), fg_world(), idle(), sidle(), nactive()). The
// database is core; actual sockets belong to the client (WorldHost,
// SocketHost).

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// World is one entry of the world database.
type World struct {
	Name      string
	Type      string
	Host      string
	Port      string
	Character string
	Password  string
	MFile     string
	SrcHost   string // -s: local address to bind
	Charset   string // character set (the default_charset at definition time)
	NoProxy   bool   // 'p'
	SSL       bool   // 'x'
	Echo      bool   // 'e'
	Temp      bool   // made by "/connect host port"; dropped when the connection ends unless renamed

	hist *history
}

// WorldInfo is a copy of a World for clients.
type WorldInfo = World

type worldDB struct {
	worlds  []*World
	def     *World // the "default" world (not in worlds, has no history)
	cur     string // name of the current world ("" = none)
	unnamed int
}

func fold(s string) string { return strings.ToLower(s) }

func (db *worldDB) find(name string) *World {
	if name == "" {
		if len(db.worlds) > 0 {
			return db.worlds[0]
		}
		return nil
	}
	for _, w := range db.worlds {
		if strings.EqualFold(w.Name, name) {
			return w
		}
	}
	return nil
}

func (in *Interp) worldHistory(name string) *history {
	if w := in.ds().wd.find(name); w != nil && name != "" {
		return w.hist
	}
	return nil
}

func (in *Interp) namedOrCurrentWorld(who, name string) *World {
	db := &in.ds().wd
	var w *World
	if name != "" {
		w = db.find(name)
	} else if db.cur != "" {
		w = db.find(db.cur)
	}
	if w == nil {
		in.warn("%s: No world %s", who, name)
	}
	return w
}

// ---------------------------------------------------------------- exported API

// SetCurrentWorld says which world the interpreter is currently acting for
// (TF's xworld: the world of the socket whose text is being processed, or of
// the foreground socket for typed commands). It drives the world_* variables
// (${world_name}, ${world_host}, ...), /recall's default history, is_open()
// with no argument and so on. "" means none.
func (in *Interp) SetCurrentWorld(name string) {
	in.ds().wd.cur = name
	in.refreshWorldVars()
}

// CurrentWorld returns the current world's name ("" if none).
func (in *Interp) CurrentWorld() string { return in.ds().wd.cur }

// WorldMacroBody answers TF's ${world_name}, ${world_host}, ... (a macro body
// lookup that, for names starting "world_", reads the current world's
// world_info() field). It returns false for any other name, so the caller can
// fall back to ordinary macros.
func (in *Interp) WorldMacroBody(name string) (string, bool) {
	if !strings.HasPrefix(name, "world_") || !validWorldField(name[6:]) {
		return "", false
	}
	cur := in.ds().wd.cur
	if cur == "" || in.ds().wd.find(cur) == nil {
		return "", false
	}
	return in.worldField(cur, name[6:], true), true
}

var worldVarFields = []string{"name", "type", "character", "password", "host", "port", "mfile", "login", "proxy", "src", "cipher"}

func (in *Interp) refreshWorldVars() {
	for _, f := range worldVarFields {
		delete(in.globals, "world_"+f)
	}
	db := &in.ds().wd
	if db.cur == "" || db.find(db.cur) == nil {
		return
	}
	for _, f := range worldVarFields {
		in.globals["world_"+f] = Str(in.worldField(db.cur, f, true))
	}
}

// Worlds returns copies of the defined worlds in definition order (the
// "default" world, if any, comes first).
func (in *Interp) Worlds() []WorldInfo {
	db := &in.ds().wd
	var out []WorldInfo
	if db.def != nil {
		out = append(out, *db.def)
	}
	for _, w := range db.worlds {
		out = append(out, *w)
	}
	return out
}

// World returns a copy of the named world ("default" is the default world).
func (in *Interp) World(name string) (WorldInfo, bool) {
	db := &in.ds().wd
	if strings.EqualFold(name, "default") && db.def != nil {
		return *db.def, true
	}
	if name == "" {
		return WorldInfo{}, false
	}
	if w := db.find(name); w != nil {
		return *w, true
	}
	return WorldInfo{}, false
}

// DefineWorld defines or redefines a world exactly as addworld() does (empty
// fields leave an existing world's values alone). It reports whether the
// definition was accepted.
func (in *Interp) DefineWorld(w WorldInfo) bool {
	flags := ""
	if w.NoProxy {
		flags += "p"
	}
	if w.SSL {
		flags += "x"
	}
	if w.Echo {
		flags += "e"
	}
	return in.newWorld(w.Name, w.Type, w.Host, w.Port, w.Character, w.Password, w.MFile, flags, w.SrcHost, w.Temp) != nil
}

// RemoveWorld deletes a world; the client calls it for a temporary world
// whose connection has closed (see Temp). It fails for a world in use.
func (in *Interp) RemoveWorld(name string) bool {
	db := &in.ds().wd
	w := db.find(name)
	if w == nil || name == "" {
		return false
	}
	if in.worldInUse(w.Name) {
		return false
	}
	in.nukeWorld(w)
	return true
}

// WorldClosed tells the core that the connection to a world has ended: a
// temporary world (what "/connect host port" makes) is forgotten.
func (in *Interp) WorldClosed(name string) {
	if w := in.ds().wd.find(name); w != nil && name != "" && w.Temp {
		in.nukeWorld(w)
	}
}

func (in *Interp) worldInUse(name string) bool {
	for _, s := range in.sockets() {
		if strings.EqualFold(s.World, name) && s.State < SockZombie {
			return true
		}
	}
	if wh, ok := in.Host.(WorldHost); ok {
		if _, listed := in.Host.(SocketHost); !listed {
			return wh.Connected(name)
		}
	}
	return false
}

// ---------------------------------------------------------------- core operations

var addworldFlagOK = "xpe"

// newWorld is TF's new_world. temp marks a "(unnamedN)" connection world.
// name == "" with temp creates the unnamed world. Returns nil after
// reporting an error.
func (in *Interp) newWorld(name, typ, host, port, char, pass, mfile, flags, myhost string, temp bool) *World {
	db := &in.ds().wd
	unnamed := name == "" && temp
	if !unnamed && (name == "" || strings.ContainsRune(name, ' ') || (name[0] == '(' && (host != "" || port != "") && !temp)) {
		in.warn("illegal world name: %s", name)
		return nil
	}
	var w *World
	redef := false
	switch {
	case !unnamed && strings.EqualFold(name, "default"):
		if db.def != nil {
			w, redef = db.def, true
		} else {
			w = &World{}
			db.def = w
		}
	case !unnamed && db.find(name) != nil:
		w, redef = db.find(name), true
	default:
		if (host == "") != (port == "") {
			in.warn("world %s: host and port must be both blank or both non-blank.", name)
			return nil
		}
		w = &World{hist: &history{}}
		db.worlds = append(db.worlds, w)
	}
	if unnamed {
		db.unnamed++
		w.Name = fmt.Sprintf("(unnamed%d)", db.unnamed)
	} else {
		w.Name = name
	}
	set := func(dst *string, v string) {
		if v != "" {
			*dst = v
		}
	}
	set(&w.Character, char)
	set(&w.Password, pass)
	set(&w.Host, host)
	set(&w.Port, port)
	set(&w.MFile, mfile)
	set(&w.Type, typ)
	set(&w.SrcHost, myhost)
	for _, c := range flags {
		switch c {
		case 'x':
			w.SSL = true
		case 'p', 'f', '0':
			w.NoProxy = true
		case 'e':
			w.Echo = true
		}
	}
	if temp {
		w.Temp = true
	}
	w.Charset = in.globals["default_charset"].String()
	if redef {
		in.FireHookArgs("REDEF", "world", w.Name)
	}
	if w.Name == db.cur || (db.cur != "" && strings.EqualFold(db.cur, w.Name)) {
		in.refreshWorldVars()
	}
	return w
}

func (in *Interp) nukeWorld(w *World) {
	db := &in.ds().wd
	for i, x := range db.worlds {
		if x == w {
			db.worlds = append(db.worlds[:i], db.worlds[i+1:]...)
			break
		}
	}
	in.closeLog(w.hist)
	in.RemoveWorldMacros(w.Name)                          // remove_world_macros
	for _, p := range append([]*Proc(nil), in.procs...) { // kill_procs_by_world
		if p.World != "" && strings.EqualFold(p.World, w.Name) {
			in.killProc(p.PID)
		}
	}
	if strings.EqualFold(db.cur, w.Name) {
		db.cur = ""
		in.refreshWorldVars()
	}
}

// ---------------------------------------------------------------- addworld()

func fnAddworld(in *Interp, a []Value) (Value, error) {
	if len(a) < 2 || len(a) > 9 {
		in.warn("addworld(): wrong number of arguments")
		return Int(0), nil
	}
	n := func(i int) string { return str(a, i) }
	if !in.allow(OpNewWorld, "addworld()", n(0)) {
		return Int(0), nil
	}
	flags := ""
	if len(a) > 7 {
		for _, c := range n(7) {
			switch c {
			case 'x', 'p', 'e', 'f', '0':
				flags += string(c)
			case 'o', 'n', '1':
			default:
				in.warn("addworld(): invalid flag %c", c)
				return Int(0), nil
			}
		}
	}
	return Bool(in.newWorld(n(0), n(1), n(2), n(3), n(4), n(5), n(6), flags, n(8), false) != nil), nil
}

// ---------------------------------------------------------------- /unworld /renameworld

func cmdUnworld(in *Interp, args string) Value {
	db := &in.ds().wd
	res := 0
	for _, name := range strings.Fields(args) {
		switch {
		case strings.EqualFold(name, "default") && db.def != nil:
			db.def = nil
		default:
			w := db.find(name)
			if w == nil {
				in.warn("/unworld: No world %s", name)
				continue
			}
			if in.worldInUse(w.Name) {
				in.warn("/unworld: %s is in use.", w.Name)
				continue
			}
			in.nukeWorld(w)
			res++
		}
	}
	return Int(int64(res))
}

func cmdRenameworld(in *Interp, args string) Value {
	f := strings.Fields(args)
	if len(f) == 0 {
		in.warn("/renameworld: usage: /renameworld [<old>] <new>")
		return Int(0)
	}
	db := &in.ds().wd
	var oldn, newn string
	if len(f) == 1 {
		fg := in.fgName()
		if fg == "" {
			in.warn("/renameworld: no current connection: /renameworld <old> <new>")
			return Int(0)
		}
		oldn, newn = fg, f[0]
	} else {
		oldn, newn = f[0], f[1]
	}
	w := db.find(oldn)
	if w == nil || oldn == "" {
		in.warn("/renameworld: no such world")
		return Int(0)
	}
	if newn == "" || strings.ContainsRune(newn, ' ') || newn[0] == '(' || strings.EqualFold(newn, "default") {
		in.warn("/renameworld: illegal world name")
		return Int(0)
	}
	if o := db.find(newn); o != nil && o != w {
		in.warn("/renameworld: a world named %s already exists", newn)
		return Int(0)
	}
	was := w.Name
	w.Name = newn
	w.Temp = false
	if strings.EqualFold(db.cur, was) {
		db.cur = newn
		in.refreshWorldVars()
	}
	in.notify("renameworld", was, newn)
	in.echo("% World renamed.")
	return Int(1)
}

// ---------------------------------------------------------------- /listworlds /saveworld

const (
	lwTable = 1 << iota
	lwUnnamed
	lwShort
)

func (w *World) addworldLine() string {
	need := 2
	switch {
	case w.SrcHost != "":
		need = 9
	case w.NoProxy || w.SSL || w.Echo:
		need = 8
	case w.MFile != "":
		need = 7
	case w.Character != "" || w.Password != "":
		need = 6
	case w.Host != "" || w.Port != "":
		need = 4
	}
	q := func(s string) string { return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"` }
	var b strings.Builder
	b.WriteString("/test addworld(" + q(w.Name) + ", " + q(w.Type))
	rest := []string{w.Host, w.Port, w.Character, w.Password, w.MFile}
	// fields in order, with the same cut-offs as TF's "need" levels
	cut := []int{3, 4, 5, 6, 7}
	for i, v := range rest {
		if need < cut[i] {
			break
		}
		b.WriteString(", " + q(v))
	}
	if need >= 8 {
		f := ""
		if w.NoProxy {
			f += "p"
		}
		if w.SSL {
			f += "x"
		}
		if w.Echo {
			f += "e"
		}
		b.WriteString(", " + q(f))
	}
	if need >= 9 {
		b.WriteString(", " + q(w.SrcHost))
	}
	b.WriteString(")")
	return b.String()
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// listWorlds writes the matching worlds with out; it returns how many.
func (in *Interp) listWorlds(name, typ func(string) bool, out func(string), flags int, cmp func(a, b *World) bool) int {
	db := &in.ds().wd
	if flags&lwTable != 0 {
		width := int(in.globals["wrapsize"].Int())
		if width == 0 {
			width, _ = in.termSize()
		}
		if width < 40 {
			width = 40
		}
		wn, wt, wp := width/5, width/7, 6
		wh := width - wn - wt - wp - 3
		if wh < 1 {
			wh = 1
		}
		out(fmt.Sprintf("%-*s %-*s %-*s %*s", wn, "NAME", wt, "TYPE", wh, "HOST", wp, "PORT"))
	}
	var ws []*World
	all := db.worlds
	if db.def != nil {
		all = append([]*World{db.def}, all...)
	}
	for _, w := range all {
		if w.Temp && flags&lwUnnamed == 0 {
			continue
		}
		if name != nil && !name(w.Name) {
			continue
		}
		if typ != nil && !typ(w.Type) {
			continue
		}
		ws = append(ws, w)
	}
	if cmp != nil {
		sort.SliceStable(ws, func(i, j int) bool { return cmp(ws[i], ws[j]) })
	}
	for _, w := range ws {
		switch {
		case flags&lwShort != 0:
			out(w.Name)
		case flags&lwTable != 0:
			width := int(in.globals["wrapsize"].Int())
			if width == 0 {
				width, _ = in.termSize()
			}
			if width < 40 {
				width = 40
			}
			wn, wt, wp := width/5, width/7, 6
			wh := width - wn - wt - wp - 3
			if wh < 1 {
				wh = 1
			}
			out(fmt.Sprintf("%-*s %-*s %-*s %*s", wn, trunc(w.Name, wn), wt, trunc(w.Type, wt), wh, trunc(w.Host, wh), wp, trunc(w.Port, wp)))
		default:
			out(w.addworldLine())
		}
	}
	return len(ws)
}

func trunc(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func cmpName(a, b *World) bool { return fold(a.Name) < fold(b.Name) }

func cmdListworlds(in *Interp, args string) Value {
	opts, rest, ok := in.scanOpts("/listworlds", "T:uscm:S:", args)
	if !ok {
		return Int(0)
	}
	flags := lwTable
	style := ""
	typePat := ""
	haveType := false
	cmp := cmpName
	for _, o := range opts {
		switch o.c {
		case 'T':
			typePat, haveType = o.arg, true
		case 'u':
			flags |= lwUnnamed
		case 's':
			flags |= lwShort
			flags &^= lwTable
		case 'c':
			flags &^= lwTable
		case 'm':
			s, ok := in.matchStyleName("/listworlds", o.arg)
			if !ok {
				return Int(0)
			}
			style = s
		case 'S':
			n := len(o.arg)
			pre := func(full string) bool { return len(full) >= n && strings.EqualFold(full[:n], o.arg) }
			field := func(f func(*World) string) func(a, b *World) bool {
				return func(a, b *World) bool { return fold(f(a)) < fold(f(b)) }
			}
			switch {
			case n == 0 || pre("name"):
				cmp = cmpName
			case pre("type"):
				cmp = field(func(w *World) string { return w.Type })
			case pre("host"):
				cmp = field(func(w *World) string { return w.Host })
			case pre("port"):
				cmp = field(func(w *World) string { return w.Port })
			case pre("character"):
				cmp = field(func(w *World) string { return w.Character })
			case pre("-"):
				cmp = nil
			default:
				in.cmdErr("LISTWORLDS", 'S', "invalid sort criterion")
				return Int(0)
			}
		}
	}
	var typeM, nameM func(string) bool
	if haveType {
		m, ok := in.matchStyle("/listworlds -T", style, typePat)
		if !ok {
			return Int(0)
		}
		typeM = m
	}
	if p := strings.TrimRight(rest, " \t"); p != "" {
		m, ok := in.matchStyle("/listworlds", style, p)
		if !ok {
			return Int(0)
		}
		nameM = m
	}
	return Int(int64(in.listWorlds(nameM, typeM, in.echo, flags, cmp)))
}

func cmdSaveworld(in *Interp, args string) Value {
	if in.Restriction() >= RestrictFile {
		in.warn("/saveworld: restricted")
		return Int(0)
	}
	opts, rest, ok := in.scanOpts("/saveworld", "a", args)
	if !ok {
		return Int(0)
	}
	mode := os.O_TRUNC
	verb := "Writ"
	if len(opts) > 0 {
		mode, verb = os.O_APPEND, "Append"
	}
	name, ok := in.tfname("/saveworld", rest, "WORLDFILE")
	if !ok {
		return Int(0)
	}
	if !in.allow(OpFileWrite, "/saveworld", name) {
		return Int(0)
	}
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|mode, 0o600)
	if err != nil {
		in.warn("/saveworld: %s: %s", strings.TrimSpace(rest), errText(err))
		return Int(0)
	}
	defer f.Close()
	in.echo(fmt.Sprintf("%% %sing world definitions to %s", verb, name))
	n := in.listWorlds(nil, nil, func(s string) { fmt.Fprintln(f, s) }, 0, nil)
	return Int(int64(n))
}

// ---------------------------------------------------------------- world_info()

func (in *Interp) sockets() []SocketInfo {
	if sh, ok := in.Host.(SocketHost); ok {
		return sh.Sockets()
	}
	return nil
}

func (in *Interp) sockOf(world string) (SocketInfo, bool) {
	for _, s := range in.sockets() {
		if strings.EqualFold(s.World, world) {
			return s, true
		}
	}
	return SocketInfo{}, false
}

// worldField returns one world_info() field of a world. current selects the
// "current world" flavor: host and port are those of the connection.
func (in *Interp) worldField(name, field string, current bool) string {
	db := &in.ds().wd
	var w *World
	if strings.EqualFold(name, "default") && db.def != nil {
		w = db.def
	} else {
		w = db.find(name)
	}
	if w == nil || name == "" {
		return ""
	}
	s, hasSock := in.sockOf(w.Name)
	switch field {
	case "name", "":
		return w.Name
	case "type":
		return w.Type
	case "character":
		return w.Character
	case "password":
		return w.Password
	case "host":
		if current && hasSock && s.Host != "" {
			return s.Host
		}
		return w.Host
	case "port":
		if current && hasSock && s.Port != "" {
			return s.Port
		}
		return w.Port
	case "mfile":
		return w.MFile
	case "login":
		return fmt.Sprint(boolInt(hasSock && s.Login))
	case "proxy":
		return fmt.Sprint(boolInt(hasSock && s.Proxy))
	case "src":
		return w.SrcHost
	case "cipher":
		if hasSock {
			return s.Cipher
		}
		return ""
	}
	return "\x00bad"
}

func fnWorldInfo(in *Interp, a []Value) (Value, error) {
	db := &in.ds().wd
	var world, field string
	explicit := false
	switch len(a) {
	case 0:
	case 1:
		field = a[0].String()
	default:
		world, field, explicit = a[0].String(), a[1].String(), true
	}
	name := world
	if !explicit {
		name = db.cur
	}
	if field != "" && !validWorldField(field) {
		in.warn("world_info(): illegal field name '%s'", field)
		return Str(""), nil
	}
	return Str(in.worldField(name, field, !explicit)), nil
}

func validWorldField(f string) bool {
	for _, n := range worldVarFields {
		if n == f {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------- connections

func (in *Interp) worldHost() (WorldHost, bool) {
	wh, ok := in.Host.(WorldHost)
	return wh, ok
}

func (in *Interp) fgName() string {
	if wh, ok := in.worldHost(); ok {
		return wh.ForegroundWorld()
	}
	return ""
}

func cmdConnect(in *Interp, args string) Value {
	opts, rest, ok := in.scanOpts("/connect", "lqxfb", args)
	if !ok {
		return Int(0)
	}
	var co ConnectOptions
	for _, o := range opts {
		switch o.c {
		case 'l':
			co.NoLogin = true
		case 'q':
			co.Quiet = true
		case 'x':
			co.SSL = true
		case 'f':
			co.Foreground = true
		case 'b':
			co.Background = true
		}
	}
	wh, hasWH := in.worldHost()
	if !hasWH {
		return in.unsupported("/connect")
	}
	rest = strings.TrimSpace(rest)
	host, port := rest, ""
	if i := strings.IndexAny(rest, " \t"); i >= 0 {
		host, port = rest[:i], strings.TrimSpace(rest[i:])
	}
	var w *World
	if port != "" {
		if !in.allow(OpNewWorld, "/connect", host+" "+port) {
			in.warn("arbitrary connections restricted")
			return Int(0)
		}
		flags := ""
		if co.SSL {
			flags = "x"
		}
		w = in.newWorld("", "", host, port, "", "", "", flags, "", true)
		if w == nil {
			return Int(0)
		}
	} else {
		w = in.ds().wd.find(host)
		if w == nil {
			n := host
			if n == "" {
				n = "default world"
			}
			in.warn("/connect: %s: no such world", n)
			return Int(0)
		}
	}
	var err error
	if ch, ok := in.Host.(ConnectHost); ok {
		err = ch.ConnectWith(w.Name, co)
	} else if co != (ConnectOptions{}) {
		return in.unsupported("/connect options")
	} else {
		err = wh.Connect(w.Name)
	}
	if err != nil {
		in.warn("/connect: %v", err)
		if w.Temp {
			in.nukeWorld(w)
		}
		return Int(0)
	}
	return Int(1)
}

// A host without WorldHost has no sockets at all: queries and /dc, /fg answer
// as TF does with nothing connected; only /connect (which would have to make a
// socket) is "not supported by this client".

func cmdDc(in *Interp, args string) Value {
	wh, hasWH := in.worldHost()
	_, hasList := in.Host.(SocketHost)
	disc := func(world string) bool {
		if err := wh.Disconnect(world); err != nil {
			in.warn("/dc: %v", err)
			return false
		}
		return true
	}
	arg := strings.TrimSpace(args)
	switch {
	case arg == "":
		cur := in.CurrentWorld()
		if cur == "" || !in.sockLive(cur) {
			in.warn("/dc: no current socket")
			return Int(0)
		}
		if !disc(cur) {
			return Int(0)
		}
	case arg == "-all":
		if !hasWH {
			return Int(1) // no sockets
		}
		if !hasList {
			return in.unsupported("/dc -all (the client does not list its sockets)")
		}
		for _, s := range in.sockets() {
			if s.State < SockDead {
				disc(s.World)
			}
		}
	default:
		if !in.sockLive(arg) {
			in.warn("/dc: Not connected to %s", arg)
			return Int(0)
		}
		if !disc(arg) {
			return Int(0)
		}
	}
	return Int(1)
}

// sockLive: a socket to the world exists and is not dead.
func (in *Interp) sockLive(world string) bool {
	if s, ok := in.sockOf(world); ok {
		return s.State < SockDead
	}
	if _, ok := in.Host.(SocketHost); ok {
		return false
	}
	if wh, ok := in.worldHost(); ok {
		return wh.Connected(world)
	}
	return false
}

func cmdFg(in *Interp, args string) Value {
	opts, rest, ok := in.scanOpts("/fg", "nlqs<>c#", args)
	if !ok {
		return Int(0)
	}
	nosock, noerr, quiet, num := false, false, false, 0
	for _, o := range opts {
		switch o.c {
		case 'n':
			nosock = true
		case 's':
			noerr = true
		case 'q':
			quiet = true
		case '<':
			num = -1
		case '>':
			num = 1
		case 'c':
			num = int(o.ival)
		}
	}
	wh, hasWH := in.worldHost()
	_, hasList := in.Host.(SocketHost)
	cur := ""
	if hasWH {
		cur = wh.ForegroundWorld()
	}
	// go puts world (or no world) in the foreground: 2 if it already is, 1 if
	// switched, 0 on failure (TF's fg_sock).
	do := func(world string) Value {
		if strings.EqualFold(world, cur) {
			return Int(2)
		}
		var err error
		if fq, ok := in.Host.(FgHost); ok {
			err = fq.ForegroundQuiet(world, quiet)
		} else if quiet {
			return in.unsupported("/fg -q")
		} else {
			err = wh.Foreground(world)
		}
		if err != nil {
			in.warn("/fg: %v", err)
			return Int(0)
		}
		return Int(1)
	}
	if nosock {
		return do("")
	}
	var socks []SocketInfo
	if hasWH {
		if !hasList && (num != 0 || strings.TrimSpace(rest) == "") {
			return in.unsupported("/fg (the client does not list its sockets)")
		}
		socks = in.sockets()
	}
	if num != 0 {
		if len(socks) == 0 {
			return Int(0)
		}
		i := 0
		for k, s := range socks {
			if strings.EqualFold(s.World, cur) {
				i = k
			}
		}
		stop := i
		for {
			if num > 0 {
				i = (i + 1) % len(socks)
				num--
			} else {
				i = (i + len(socks) - 1) % len(socks)
				num++
			}
			if num == 0 || i == stop {
				break
			}
		}
		return do(socks[i].World)
	}
	name := strings.TrimSpace(rest)
	if name == "" {
		if len(socks) == 0 {
			if !noerr {
				in.warn("/fg: no socket named ''")
			}
			return Int(0)
		}
		return do(socks[0].World)
	}
	if !in.sockLive(name) {
		if !noerr {
			in.warn("/fg: no socket named '%s'", name)
		}
		return Int(0)
	}
	return do(name)
}

// ---------------------------------------------------------------- /listsockets

func idleText(t int) string {
	switch {
	case t < 60:
		return fmt.Sprintf("%3ds", t)
	case t/60 < 60:
		return fmt.Sprintf("%3dm", t/60)
	case t/3600 < 24:
		return fmt.Sprintf("%3dh", t/3600)
	case t/86400 < 1000:
		return fmt.Sprintf("%3dd", t/86400)
	}
	return "long"
}

func cmdListsockets(in *Interp, args string) Value {
	opts, rest, ok := in.scanOpts("/listsockets", "m:sT:nS:", args)
	if !ok {
		return Int(0)
	}
	style, typePat, haveType := "", "", false
	short, numeric := false, false
	var cmp func(a, b sockRow) bool
	for _, o := range opts {
		switch o.c {
		case 'm':
			s, ok := in.matchStyleName("/listsockets", o.arg)
			if !ok {
				return Int(0)
			}
			style = s
		case 's':
			short = true
		case 'T':
			typePat, haveType = o.arg, true
		case 'n':
			numeric = true
		case 'S':
			n := len(o.arg)
			pre := func(full string) bool { return len(full) >= n && strings.EqualFold(full[:n], o.arg) }
			sf := func(f func(sockRow) string) func(a, b sockRow) bool {
				return func(a, b sockRow) bool { return fold(f(a)) < fold(f(b)) }
			}
			switch {
			case n == 0 || pre("name"):
				cmp = sf(func(r sockRow) string { return r.s.World })
			case pre("lines"):
				cmp = func(a, b sockRow) bool { return a.s.NewLines < b.s.NewLines }
			case pre("idle"):
				cmp = func(a, b sockRow) bool { return a.s.LastRecv.After(b.s.LastRecv) }
			case pre("type"):
				cmp = sf(func(r sockRow) string { return r.typ })
			case pre("host"):
				cmp = sf(func(r sockRow) string { return r.host })
			case pre("port"):
				cmp = sf(func(r sockRow) string { return r.port })
			case pre("character"):
				cmp = sf(func(r sockRow) string { return r.char })
			case pre("-"):
				cmp = nil
			default:
				in.cmdErr("LISTSOCKETS", 'S', "invalid sort criterion")
				return Int(0)
			}
		}
	}
	var typeM, nameM func(string) bool
	if haveType {
		m, ok := in.matchStyle("/listsockets -T", style, typePat)
		if !ok {
			return Int(0)
		}
		typeM = m
	}
	if p := strings.TrimRight(rest, " \t"); p != "" {
		m, ok := in.matchStyle("/listsockets", style, p)
		if !ok {
			return Int(0)
		}
		nameM = m
	}
	socks := in.sockets()
	if len(socks) == 0 {
		if !short {
			in.warn("/listsockets: Not connected to any sockets.")
		}
		return Int(0)
	}
	var rows []sockRow
	for _, s := range socks {
		r := sockRow{s: s}
		if w, ok := in.World(s.World); ok {
			r.typ, r.host, r.port, r.char = w.Type, w.Host, w.Port, w.Character
		}
		if typeM != nil && !typeM(r.typ) {
			continue
		}
		if nameM != nil && !nameM(s.World) {
			continue
		}
		rows = append(rows, r)
	}
	if cmp != nil {
		sort.SliceStable(rows, func(i, j int) bool { return cmp(rows[i], rows[j]) })
	}
	typeW, nameW, hostW := 11, 15, 26
	if numeric {
		nameW, typeW, hostW = nameW+3, typeW+3, 20
	}
	now := in.now()
	if !short {
		in.echo(fmt.Sprintf("    %8s %4s %-*s %-*s %-*s %s", "LINES", "IDLE", typeW, "TYPE", nameW, "NAME", hostW, "HOST", "PORT"))
	}
	fg := in.fgName()
	cur := in.CurrentWorld()
	for _, r := range rows {
		s := r.s
		if short {
			in.echo(s.World)
			continue
		}
		var lb string
		if strings.EqualFold(s.World, fg) {
			lb = fmt.Sprintf("%8s", "fg")
		} else if nold := s.OldLines; nold > 0 {
			var a, b string
			if nold > 999 {
				a = fmt.Sprintf("%2dk", nold/1000)
			} else {
				a = fmt.Sprintf("%3d", nold)
			}
			if s.NewLines > 9999 {
				b = fmt.Sprintf("+%3dk", s.NewLines/1000)
			} else {
				b = fmt.Sprintf("+%4d", s.NewLines)
			}
			lb = a + b
		} else if s.NewLines > 99999 {
			lb = fmt.Sprintf("%7dk", s.NewLines/1000)
		} else {
			lb = fmt.Sprintf("%8d", s.NewLines)
		}
		idle := idleText(int(now.Sub(s.LastRecv) / time.Second))
		state := byte('#')
		switch {
		case s.State < SockConnected:
			state = '?'
		case s.State >= SockZombie:
			state = '!'
		case s.State == SockOpen:
			state = 'O'
		case s.State == SockConnected && s.Subneg:
			state = 'S'
		case s.State == SockConnected && s.SSL:
			state = 'X'
		case s.State == SockConnected:
			state = 'C'
		}
		if s.Compressed && state >= 'A' && state <= 'Z' {
			state += 'a' - 'A'
		}
		addr := ""
		if s.Addr != "" {
			if numeric {
				addr = fmt.Sprintf("%-20.20s %s", s.Addr, s.Port)
			} else {
				addr = fmt.Sprintf("%-*.*s %.6s", hostW, hostW, s.Host, s.Port)
			}
		}
		star := byte(' ')
		if strings.EqualFold(s.World, cur) {
			star = '*'
		}
		px := byte(' ')
		if s.Proxy {
			px = 'P'
		}
		in.echo(fmt.Sprintf("%c%c%c %s %s %-*.*s %-*.*s %s", star, state, px, lb, idle, typeW, typeW, r.typ, nameW, nameW, s.World, addr))
	}
	return Int(int64(len(rows)))
}

type sockRow struct {
	s                     SocketInfo
	typ, host, port, char string
}

// ---------------------------------------------------------------- connection functions

func (in *Interp) sockArgWorld(a []Value) string {
	if len(a) > 0 && a[0].String() != "" {
		return a[0].String()
	}
	return in.CurrentWorld()
}

func fnIsOpenConnected(open bool) fn {
	return func(in *Interp, a []Value) (Value, error) {
		w := in.sockArgWorld(a)
		if w == "" {
			return Int(0), nil
		}
		if s, ok := in.sockOf(w); ok {
			if open {
				return Bool(s.State >= SockConnected && s.State < SockZombie), nil
			}
			return Bool(s.State == SockConnected), nil
		}
		if _, ok := in.Host.(SocketHost); ok {
			return Int(0), nil
		}
		if wh, ok := in.worldHost(); ok {
			return Bool(wh.Connected(w)), nil
		}
		return Int(0), nil
	}
}

func fnFgWorld(in *Interp, a []Value) (Value, error) { return Str(in.fgName()), nil }

func dtimeSince(now, then time.Time) Value {
	d := now.Sub(then)
	us := d.Microseconds()
	neg := us < 0
	if neg {
		us = -us
	}
	s := fmt.Sprintf("%d.%06d", us/1000000, us%1000000)
	if neg {
		s = "-" + s
	}
	v := Str(s).Num()
	v.tt = ttDtime
	return v
}

func fnIdle(send bool) fn {
	name := "idle()"
	if send {
		name = "sidle()"
	}
	return func(in *Interp, a []Value) (Value, error) {
		if !send && len(a) == 0 {
			ih, ok := in.Host.(IdleHost)
			if !ok {
				in.unsupported(name)
				return Int(0), nil
			}
			return dtimeSince(in.now(), ih.LastKeyboard()), nil
		}
		w := in.sockArgWorld(a)
		s, ok := in.sockOf(w)
		if !ok {
			if _, listed := in.Host.(SocketHost); !listed && w != "" {
				if _, has := in.worldHost(); has {
					in.unsupported(name)
				}
			}
			return Str("0.0").Num(), nil
		}
		t := s.LastRecv
		if send {
			t = s.LastSend
		}
		if t.IsZero() {
			return Str("0.0").Num(), nil
		}
		return dtimeSince(in.now(), t), nil
	}
}

// fnNactive counts worlds with unseen activity. Connection state comes from the
// host's SocketHost when it has one; otherwise from the paging state a client
// reports through SetMoreState.
func fnNactive(in *Interp, a []Value) (Value, error) {
	if len(in.sockets()) == 0 {
		return fnNactiveMore(in, a)
	}
	if len(a) == 0 || a[0].String() == "" {
		n := 0
		for _, s := range in.sockets() {
			if s.Active {
				n++
			}
		}
		return Int(int64(n)), nil
	}
	s, ok := in.sockOf(a[0].String())
	if !ok || !s.Active {
		return Int(0), nil
	}
	return Int(int64(s.NewLines)), nil
}

func init() {
	builtinCmds["unworld"] = cmdUnworld
	builtinCmds["renameworld"] = cmdRenameworld
	builtinCmds["listworlds"] = cmdListworlds
	builtinCmds["saveworld"] = cmdSaveworld
	builtinCmds["connect"] = cmdConnect
	builtinCmds["dc"] = cmdDc
	builtinCmds["fg"] = cmdFg
	builtinCmds["listsockets"] = cmdListsockets
	f := builtinFuncs
	f["addworld"] = fnAddworld
	f["world_info"] = fnWorldInfo
	f["is_connected"] = fnIsOpenConnected(false)
	f["is_open"] = fnIsOpenConnected(true)
	f["fg_world"] = fnFgWorld
	f["idle"] = fnIdle(false)
	f["sidle"] = fnIdle(true)
	f["nactive"] = fnNactive
}
