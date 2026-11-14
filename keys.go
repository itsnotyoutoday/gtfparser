package gtfparser

import (
	"sort"
	"strconv"
	"strings"
)

// ---------------------------------------------------------------- key sequences

// PrintToAscii converts a key sequence as written in /bind and /def -b ("^X"
// control notation, "\033", "\0x1b" and "\27" character codes) to the bytes the
// terminal sends. "^@" and "\0" would be NUL; as in tf they map to 0x80.
func PrintToAscii(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '^':
			i++
			if i >= len(s) {
				b.WriteByte('^')
				break
			}
			b.WriteByte(mapKeyChar(ctrlChar(s[i])))
			i++
		case c == '\\':
			i++
			if i >= len(s) {
				break
			}
			if s[i] >= '0' && s[i] <= '9' {
				n, used := strToChr(s[i:])
				i += used
				b.WriteByte(mapKeyChar(n))
			} else { // a backslash only quotes the next character
				b.WriteByte(s[i])
				i++
			}
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}

// ctrlChar is CTRL(c): '?' is DEL, otherwise the character's low five bits.
func ctrlChar(c byte) byte {
	if c == '?' {
		return 0x7f
	}
	return c & 0x1f
}

func mapKeyChar(c byte) byte {
	if c == 0 {
		return 0x80
	}
	return c
}

// strToChr reads a character code: "0x1b" hex, a leading 0 octal, else decimal.
func strToChr(s string) (byte, int) {
	base, i := 10, 0
	switch {
	case strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X"):
		base, i = 16, 2
	case len(s) > 1 && s[0] == '0':
		base, i = 8, 1
	}
	n := 0
	start := i
	for i < len(s) {
		d := -1
		switch c := s[i]; {
		case c >= '0' && c <= '9':
			d = int(c - '0')
		case c >= 'a' && c <= 'f':
			d = int(c-'a') + 10
		case c >= 'A' && c <= 'F':
			d = int(c-'A') + 10
		}
		if d < 0 || d >= base {
			break
		}
		n = n*base + d
		i++
	}
	if i == start && base != 8 {
		return 0, 0
	}
	if i == start {
		i = 1
	}
	return byte(n), i
}

// AsciiToPrint is the inverse of PrintToAscii: control characters become "^X".
func AsciiToPrint(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '^' || c == '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		case c >= 0x20 && c < 0x7f:
			b.WriteByte(c)
		case c < 0x20:
			b.WriteByte('^')
			b.WriteByte(c + 0x40)
		case c == 0x7f:
			b.WriteString("^?")
		default:
			b.WriteString("\\0x" + strconv.FormatInt(int64(c), 16))
		}
	}
	return b.String()
}

// keyNames are the names keycode() and /def -B know (tf's termcap key table).
var keyNames = []string{
	"Backspace", "Clear EOL", "Clear EOS", "Clear Screen", "Delete", "Delete Line", "Down",
	"F0", "F1", "F10", "F11", "F12", "F13", "F14", "F15", "F16", "F17", "F18", "F19",
	"F2", "F3", "F4", "F5", "F6", "F7", "F8", "F9", "Home", "Home Down", "Insert",
	"Insert Line", "KP1", "KP2", "KP3", "KP4", "KP5", "Left", "PgDn", "PgUp", "Right",
	"Scroll Down", "Scroll Up", "Up",
}

func knownKeyName(name string) bool {
	for _, n := range keyNames {
		if strings.EqualFold(n, name) {
			return true
		}
	}
	return false
}

// KeyCode is keycode(): the byte sequence the terminal sends for the named key
// ("" if the terminal defines none) and whether the name is a known key name.
// The sequences come from the host's KeyHost.
func (in *Interp) KeyCode(name string) (string, bool) {
	if !knownKeyName(name) {
		return "", false
	}
	if k, ok := in.Host.(KeyHost); ok {
		return k.KeyCode(name), true
	}
	return "", true
}

func fnKeycode(in *Interp, a []Value) (Value, error) {
	name := str(a, 0)
	code, ok := in.KeyCode(name)
	if !ok {
		in.warn("keycode: unknown key name \"%s\"", name)
		return Str(""), nil
	}
	return Str(code), nil
}

// ---------------------------------------------------------------- bindings

// Binding is a key sequence bound to a macro by /bind, /def -b or /def -B.
type Binding struct {
	Seq   string // the bytes the terminal sends
	Macro *Macro
}

// Bindings returns the active key bindings, one per sequence (the most
// recently defined macro wins), sorted by sequence. A /def -B macro is bound
// to whatever the host's KeyCode(name) says for its key name.
func (in *Interp) Bindings() []Binding {
	best := map[string]*Macro{}
	for _, m := range in.list {
		seq := ""
		if m.Bind != "" {
			seq = PrintToAscii(m.Bind)
		} else if name := m.set['B']; name != "" {
			seq, _ = in.KeyCode(name)
		}
		if seq == "" {
			continue
		}
		if o, ok := best[seq]; !ok || m.ID > o.ID {
			best[seq] = m
		}
	}
	out := make([]Binding, 0, len(best))
	for s, m := range best {
		out = append(out, Binding{s, m})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Seq < out[j].Seq })
	return out
}

// FindKey returns the macro bound to exactly seq, if any.
func (in *Interp) FindKey(seq string) (*Macro, bool) {
	for _, b := range in.Bindings() {
		if b.Seq == seq {
			return b.Macro, true
		}
	}
	return nil, false
}

// KeyPressed feeds keystrokes (raw terminal bytes, any number) to the keyboard
// handler, as tf's handle_keyboard_input does:
//
//   - the longest bound sequence is run (its macro sees %kbnum if a numeric
//     prefix was typed);
//   - bytes that begin a longer binding are held until the sequence completes
//     (FlushKeys gives up on the pending prefix);
//   - otherwise Enter (\r, \n) submits the line, backspace/DEL erase, a digit
//     extends %kbnum when one is being entered, and anything else is typed into
//     the input buffer;
//   - /dokey lnext makes the next byte literal.
//
// A line submitted by Enter runs after the keystrokes have been processed.
func (in *Interp) KeyPressed(seq string) {
	s := in.ts()
	s.keyTime = in.now().UnixNano()
	s.curKeys += seq
	in.processKeys(false)
}

// FlushKeys gives up on a held-back partial key sequence (for example a lone
// ESC that was not followed by the rest of a sequence): the longest binding
// that matches a prefix of it runs and the rest is typed.
func (in *Interp) FlushKeys() { in.processKeys(true) }

// PendingKeys returns the held-back prefix of a possible key sequence.
func (in *Interp) PendingKeys() string { return in.ts().curKeys }

func (in *Interp) processKeys(force bool) {
	defer in.enter()()
	in.PollVars()
	s := in.ts()
	s.inKeys++
	defer func() { s.inKeys-- }()
	bind := in.Bindings()
	cur := s.curKeys
	var text strings.Builder
	flush := func() {
		if text.Len() > 0 {
			in.TypeText(text.String())
			text.Reset()
		}
	}
	i := 0
loop:
	for i < len(cur) {
		if s.pending && s.inKeys == 1 { // a submitted line runs before later keys are read
			flush()
			in.SubmitInput()
		}
		if s.lnext {
			s.lnext = false
			text.WriteByte(cur[i])
			i++
			continue
		}
		rest := cur[i:]
		var exact *Binding
		partial := false
		for k := range bind {
			b := &bind[k]
			switch {
			case strings.HasPrefix(rest, b.Seq):
				if exact == nil || len(b.Seq) > len(exact.Seq) {
					exact = b
				}
			case strings.HasPrefix(b.Seq, rest):
				partial = true
			}
		}
		if partial && !force {
			break loop
		}
		if exact != nil {
			flush()
			i += len(exact.Seq)
			in.runKeyMacro(exact.Macro)
			bind = in.Bindings()
			continue
		}
		c := rest[0]
		switch {
		case c == '\n' || c == '\r':
			flush()
			i++
			s.pending = true
		case c == '\b' || c == 0x7f:
			flush()
			i++
			in.kbdel(in.Input().charBack(in.kbnumVal()))
			in.resetKbnum()
		case in.globals["kbnum"].String() != "" && c >= '0' && c <= '9' && text.Len() == 0:
			i++
			in.kbnumDigit(c)
		default:
			text.WriteByte(c)
			i++
		}
	}
	flush()
	s.curKeys = cur[i:]
	if s.pending && s.inKeys == 1 {
		in.SubmitInput()
	}
}

// kbnumDigit appends a digit to the %kbnum numeric prefix, capped at %max_kbnum.
func (in *Interp) kbnumDigit(c byte) {
	v := in.globals["kbnum"].String() + string(c)
	n := int(Str(v).Int())
	mx := in.varInt("max_kbnum")
	if neg := n < 0; mx > 0 && (n > mx || -n > mx) {
		sign := "+"
		if neg {
			sign = "-"
		}
		v = sign + strconv.Itoa(mx)
	}
	in.setGlobal("kbnum", Str(v))
}

// runKeyMacro runs a bound macro with the numeric prefix (if any) visible to
// it as the local %kbnum.
func (in *Interp) runKeyMacro(m *Macro) {
	kb := 0
	if v := in.globals["kbnum"].String(); v != "" {
		kb = int(Str(v).Int())
		in.resetKbnum()
	}
	in.invokeKey(m, kb)
}

func (in *Interp) invokeKey(m *Macro, kb int) Value {
	if kb != 0 {
		f := in.pushFrame("", "")
		f.vars["kbnum"] = Int(int64(kb))
		defer in.popFrame()
	}
	return in.callMacro(m, "", false)
}

// ---------------------------------------------------------------- /dokey

// dokeyNames lists every /dokey operation; the first block is built into tf,
// the second is what tf's library implements as /dokey_* macros (kbfunc.tf).
var dokeyNames = []string{
	"BSPC", "BWORD", "CLEAR", "DCH", "DEOL", "DLINE", "DOWN", "DWORD", "END", "FLUSH",
	"HOME", "HPAGE", "HPAGEBACK", "LEFT", "LINE", "LINEBACK", "LNEXT", "NEWLINE",
	"PAGE", "PAGEBACK", "PAUSE", "PGDN", "PGUP", "RECALLB", "RECALLBEG", "RECALLEND",
	"RECALLF", "REDRAW", "REFRESH", "RIGHT", "SEARCHB", "SEARCHF", "SELFLUSH",
	"SOCKETB", "SOCKETF", "UP", "WLEFT", "WRIGHT",
}

// DokeyNames returns the names /dokey accepts.
func DokeyNames() []string { return append([]string(nil), dokeyNames...) }

func (in *Interp) kbn() int {
	v, _ := in.lookup("kbnum")
	if n := int(v.Int()); n != 0 {
		return n
	}
	return 1
}

func (in *Interp) nthWord(n int) int {
	p := in.kbBuf().pos
	for ; n < 0; n++ {
		p = in.kbword(p, -1)
	}
	for ; n > 0; n-- {
		p = in.kbword(p, 1)
	}
	return p
}

func cmdDokey(in *Interp, args string) Value {
	in.PollVars()
	name := strings.TrimSpace(args)
	up := strings.ToUpper(name)
	s := in.ts()
	b := &s.kb
	// built into tf itself
	switch up {
	case "CLEAR", "REDRAW", "REFRESH", "SELFLUSH":
		if !in.hasNotifier() {
			return in.unsupported("/dokey " + up)
		}
		in.notify("dokey", up)
		if up == "REFRESH" {
			return Int(int64(b.pos))
		}
		return Int(1)
	case "FLUSH":
		return Int(int64(in.dokeyFlush()))
	case "LNEXT":
		s.lnext = true
		in.notify("dokey", "LNEXT")
		return Int(1)
	case "NEWLINE":
		s.pending = true
		if s.inKeys == 0 && in.depth == 0 {
			in.SubmitInput()
		}
		return Int(1)
	case "PAUSE":
		return Int(int64(in.dokeyPause()))
	case "RECALLB", "RECALLF", "RECALLBEG", "RECALLEND", "SEARCHB", "SEARCHF":
		return Int(int64(in.dokeyRecall(up)))
	}
	// a user (or library) macro /dokey_name overrides the native editing ops
	if m, ok := in.macros["dokey_"+strings.ToLower(name)]; ok && name != "" {
		return in.callMacro(m, "", false)
	}
	n := in.kbn()
	switch up {
	case "BSPC":
		return Int(int64(in.kbdel(b.charBack(n))))
	case "BWORD":
		i, h := b.pos, b.text[:b.pos]
		for i > 0 && h[i-1] == ' ' {
			i--
		}
		for i > 0 && h[i-1] != ' ' {
			i--
		}
		return Int(int64(in.kbdel(i)))
	case "DCH":
		return Int(int64(in.kbdel(b.charFwd(n))))
	case "DEOL":
		return Int(int64(in.kbdel(len(b.text))))
	case "DLINE":
		in.kbgoto(0)
		return Int(int64(in.kbdel(len(b.text))))
	case "DOWN":
		return Int(int64(in.kbgoto(b.pos + in.varInt("wrapsize")*n)))
	case "UP":
		return Int(int64(in.kbgoto(b.pos - in.varInt("wrapsize")*n)))
	case "DWORD":
		return Int(int64(in.kbdel(in.nthWord(n))))
	case "END":
		return Int(int64(in.kbgoto(len(b.text))))
	case "HOME":
		return Int(int64(in.kbgoto(0)))
	case "LEFT":
		return Int(int64(in.kbgoto(b.charBack(n))))
	case "RIGHT":
		return Int(int64(in.kbgoto(b.charFwd(n))))
	case "WLEFT":
		return Int(int64(in.kbgoto(in.nthWord(-n))))
	case "WRIGHT":
		return Int(int64(in.kbgoto(in.nthWord(n))))
	case "PAGE", "PGDN":
		return Int(int64(in.moreScroll(in.winlines() * n)))
	case "PAGEBACK", "PGUP":
		return Int(int64(in.moreScroll(-in.winlines() * n)))
	case "HPAGE":
		return Int(int64(in.moreScroll(in.winlines() * n / 2)))
	case "HPAGEBACK":
		return Int(int64(in.moreScroll(-in.winlines() * n / 2)))
	case "LINE":
		return Int(int64(in.moreScroll(n)))
	case "LINEBACK":
		return Int(int64(in.moreScroll(-n)))
	case "SOCKETB":
		return in.Exec("/fg -c" + strconv.Itoa(-n))
	case "SOCKETF":
		return in.Exec("/fg -c" + strconv.Itoa(n))
	}
	in.warn("No editing function %s", name)
	return Int(0)
}

// dokeyRecall is recall_input + replace_input.
func (in *Interp) dokeyRecall(op string) int {
	s := in.ts()
	n := in.kbnumSigned()
	var line string
	var ok bool
	switch op {
	case "RECALLB":
		line, ok = s.hist.recall(s.kb.text, -n, 0)
	case "RECALLF":
		line, ok = s.hist.recall(s.kb.text, n, 0)
	case "RECALLBEG":
		line, ok = s.hist.recall(s.kb.text, -n, 2)
	case "RECALLEND":
		line, ok = s.hist.recall(s.kb.text, n, 2)
	case "SEARCHB":
		line, ok = s.hist.recall(s.kb.text, -n, 1)
	case "SEARCHF":
		line, ok = s.hist.recall(s.kb.text, n, 1)
	}
	if !ok {
		in.bell(1)
		return 0
	}
	if len(s.kb.text) > 0 {
		s.kb.text, s.kb.pos = "", 0
	}
	// handle_input_string without kbnum or history sync: the recalled line
	// replaces the buffer and the recall position is kept
	idx := s.hist.index
	s.kb.text, s.kb.pos = line, len(line)
	s.hist.index = idx
	s.kb.changed()
	return 1
}

func (in *Interp) kbnumSigned() int { return in.kbn() }

func init() {
	builtinCmds["dokey"] = cmdDokey
	regFunc("keycode", 1, 1, fnKeycode)
}
