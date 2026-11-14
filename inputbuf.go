package gtfparser

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// InputBuffer is the keyboard line being edited: text and a byte-offset
// cursor. A client feeds keystrokes through Interp.KeyPressed (or edits it
// directly) and draws from Text/Cursor; scripts edit it with the kb*()
// functions, /input and /dokey. All positions are byte offsets, as in TF.
type InputBuffer struct {
	text string
	pos  int
	// OnChange, if set, is called after every change to the text or cursor
	// (the client redraws the input line).
	OnChange func()
}

func (b *InputBuffer) changed() {
	if b.OnChange != nil {
		b.OnChange()
	}
}

// Text is the whole line; Cursor the cursor's byte offset; Len its length.
func (b *InputBuffer) Text() string { return b.text }
func (b *InputBuffer) Cursor() int  { return b.pos }
func (b *InputBuffer) Len() int     { return len(b.text) }

// Set replaces the contents. cursor < 0 puts the cursor at the end.
func (b *InputBuffer) Set(text string, cursor int) {
	b.text = text
	if cursor < 0 || cursor > len(text) {
		cursor = len(text)
	}
	b.pos = cursor
	b.changed()
}

// Clear empties the buffer.
func (b *InputBuffer) Clear() { b.Set("", 0) }

// Input returns the interpreter's keyboard buffer.
func (in *Interp) Input() *InputBuffer { return &in.ts().kb }

// ---------------------------------------------------------------- input history

// inputHist is TF's input history: a queue of lines whose last element is the
// line being typed. index is the line currently recalled (== last when none).
type inputHist struct {
	lines []string
	index int
}

func (h *inputHist) reset() { h.lines = []string{""}; h.index = 0 }
func (h *inputHist) last() int {
	return len(h.lines) - 1
}
func (h *inputHist) sync() { h.index = h.last() }

// record is record_input(): save a submitted line.
func (h *inputHist) record(in *Interp, line string) {
	h.sync()
	dup := len(h.lines) > 1 && h.lines[len(h.lines)-2] == line
	if !dup {
		h.lines[h.last()] = line
		h.lines = append(h.lines, "")
		if max := in.varInt("histsize"); max > 0 && len(h.lines) > max {
			h.lines = h.lines[len(h.lines)-max:]
		}
		h.sync()
	}
}

// recall is recall_input(n, mode): mode 0 steps n lines, 1 searches n times
// for a line starting with the current input, 2 jumps to the first/last line.
func (h *inputHist) recall(cur string, n, mode int) (string, bool) {
	if h.index == h.last() {
		h.lines[h.last()] = cur
	}
	stop := h.last()
	if n < 0 {
		stop = 0
	}
	if h.index == stop {
		return "", false
	}
	dir := 1
	if n < 0 {
		dir = -1
	}
	var i int
	pat, hasPat := "", false
	if mode == 2 {
		i = stop
	} else {
		i = h.index + dir
		if mode == 1 {
			pat, hasPat = h.lines[h.last()], true
		}
	}
	if n < 0 {
		n = -n
	}
	for {
		s := h.lines[i]
		if !hasPat || (len(s) > len(pat) && strings.HasPrefix(s, pat)) {
			n--
			if n == 0 {
				break
			}
		}
		if i == stop {
			return "", false
		}
		i += dir
	}
	h.index = i
	return h.lines[i], true
}

// InputHistory returns the recorded input lines, oldest first.
func (in *Interp) InputHistory() []string {
	h := &in.ts().hist
	return append([]string(nil), h.lines[:len(h.lines)-1]...)
}

// ---------------------------------------------------------------- editing primitives

func (in *Interp) kbBuf() *InputBuffer {
	in.PollVars()
	return &in.ts().kb
}

func (b *InputBuffer) charBack(n int) int {
	pos := b.pos
	for ; n > 0 && pos > 0; n-- {
		pos--
		for pos > 0 && !utf8.RuneStart(b.text[pos]) {
			pos--
		}
	}
	return pos
}

func (b *InputBuffer) charFwd(n int) int {
	pos := b.pos
	for ; n > 0 && pos < len(b.text); n-- {
		pos++
		for pos < len(b.text) && !utf8.RuneStart(b.text[pos]) {
			pos++
		}
	}
	return pos
}

// kbgoto moves the cursor, clamped to the buffer; staying put rings the bell.
func (in *Interp) kbgoto(place int) int {
	b := in.kbBuf()
	if place < 0 {
		place = 0
	}
	if place > len(b.text) {
		place = len(b.text)
	}
	if place == b.pos {
		in.bell(1)
		return b.pos
	}
	b.pos = place
	b.changed()
	return b.pos
}

// kbdel deletes between the cursor and place; it returns the new cursor.
func (in *Interp) kbdel(place int) int {
	b := in.kbBuf()
	switch {
	case place >= 0 && place < b.pos:
		b.text = b.text[:place] + b.text[b.pos:]
		b.pos = place
	case place > b.pos && place <= len(b.text):
		b.text = b.text[:b.pos] + b.text[place:]
	default:
		in.bell(1)
		in.ts().hist.sync()
		return b.pos
	}
	in.ts().hist.sync()
	b.changed()
	return b.pos
}

func (in *Interp) isInWord(c byte) bool {
	if c >= 0x80 {
		return true
	}
	if c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
		return true
	}
	return strings.IndexByte(in.globals["wordpunct"].String(), c) >= 0 && c != 0
}

// kbword finds the start of the word left (dir<0) or the end of the word right
// (dir>0) of start.
func (in *Interp) kbword(start, dir int) int {
	b := in.kbBuf()
	stop := len(b.text)
	if dir < 0 {
		stop = -1
	}
	place := start
	if place < 0 {
		place = 0
	}
	if place > len(b.text) {
		place = len(b.text)
	}
	if dir < 0 {
		place--
	}
	for place != stop && !in.isInWord(b.text[place]) {
		place += dir
	}
	for place != stop && in.isInWord(b.text[place]) {
		place += dir
	}
	if dir < 0 {
		place++
	}
	return place
}

func (in *Interp) kbmatch(start int) int {
	b := in.kbBuf()
	const braces = "(){}[]"
	place := start
	if place < 0 {
		place = 0
	}
	if place > len(b.text) {
		place = len(b.text)
	}
	t := -1
	for {
		if place >= len(b.text) {
			return -1
		}
		if t = strings.IndexByte(braces, b.text[place]); t >= 0 {
			break
		}
		place++
	}
	dir := 1
	if t%2 == 1 {
		dir = -1
	}
	stop := len(b.text)
	if dir < 0 {
		stop = -1
	}
	open, closeC := braces[t], braces[t+dir]
	depth := 0
	for {
		switch b.text[place] {
		case open:
			depth++
		case closeC:
			depth--
		}
		if depth == 0 {
			return place
		}
		place += dir
		if place == stop {
			return -1
		}
	}
}

// resetKbnum clears the %kbnum numeric-prefix accumulator.
func (in *Interp) resetKbnum() {
	if v, ok := in.globals["kbnum"]; ok && v.String() != "" {
		in.setGlobal("kbnum", Str(""))
	}
}

func (in *Interp) kbnumVal() int {
	s := in.globals["kbnum"].String()
	if s == "" {
		return 1
	}
	return int(Str(s).Int())
}

// TypeText enters text into the input buffer as if typed: it honours %insert
// (insert or overwrite) and the pending %kbnum repeat count, and it makes the
// input history forget any recalled line once the buffer is non-empty.
func (in *Interp) TypeText(text string) {
	if text == "" {
		return
	}
	s := in.ts()
	b := &s.kb
	extra := 0
	if kv := in.globals["kbnum"].String(); kv != "" {
		if n := in.kbnumVal(); n > 1 {
			extra = n - 1
		}
		in.resetKbnum()
	}
	if len(b.text) > 0 {
		s.hist.sync()
	}
	ins := text
	if extra > 0 {
		r, _ := utf8.DecodeRuneInString(text)
		ins = strings.Repeat(string(r), extra) + text
	}
	switch {
	case b.pos == len(b.text), in.varOn("insert"):
		b.text = b.text[:b.pos] + ins + b.text[b.pos:]
	default: // overwrite, rune for rune
		n := utf8.RuneCountInString(ins)
		end := b.pos
		for ; n > 0 && end < len(b.text); n-- {
			_, sz := utf8.DecodeRuneInString(b.text[end:])
			end += sz
		}
		b.text = b.text[:b.pos] + ins + b.text[end:]
	}
	b.pos += len(ins)
	b.changed()
}

// ---------------------------------------------------------------- kb functions

func optInt(a []Value, i int, def int) int {
	if i < len(a) {
		return int(a[i].Int())
	}
	return def
}

func init() {
	regFunc("kbhead", 0, 0, func(in *Interp, a []Value) (Value, error) {
		b := in.kbBuf()
		return Str(b.text[:b.pos]), nil
	})
	regFunc("kbtail", 0, 0, func(in *Interp, a []Value) (Value, error) {
		b := in.kbBuf()
		return Str(b.text[b.pos:]), nil
	})
	regFunc("kbpoint", 0, 0, func(in *Interp, a []Value) (Value, error) { return Int(int64(in.kbBuf().pos)), nil })
	regFunc("kblen", 0, 0, func(in *Interp, a []Value) (Value, error) { return Int(int64(len(in.kbBuf().text))), nil })
	regFunc("kbgoto", 1, 1, func(in *Interp, a []Value) (Value, error) {
		return Int(int64(in.kbgoto(int(a[0].Int())))), nil
	})
	regFunc("kbdel", 1, 1, func(in *Interp, a []Value) (Value, error) {
		return Int(int64(in.kbdel(int(a[0].Int())))), nil
	})
	regFunc("kbcharback", 1, 1, func(in *Interp, a []Value) (Value, error) {
		return Int(int64(in.kbBuf().charBack(int(a[0].Int())))), nil
	})
	regFunc("kbcharfwd", 1, 1, func(in *Interp, a []Value) (Value, error) {
		return Int(int64(in.kbBuf().charFwd(int(a[0].Int())))), nil
	})
	regFunc("kbmatch", 0, 1, func(in *Interp, a []Value) (Value, error) {
		return Int(int64(in.kbmatch(optInt(a, 0, in.kbBuf().pos)))), nil
	})
	regFunc("kbwordleft", 0, 1, func(in *Interp, a []Value) (Value, error) {
		return Int(int64(in.kbword(optInt(a, 0, in.kbBuf().pos), -1))), nil
	})
	regFunc("kbwordright", 0, 1, func(in *Interp, a []Value) (Value, error) {
		return Int(int64(in.kbword(optInt(a, 0, in.kbBuf().pos), 1))), nil
	})
	builtinCmds["input"] = func(in *Interp, args string) Value {
		in.PollVars()
		in.TypeText(args)
		return Int(1)
	}
}

// ---------------------------------------------------------------- history substitution and submit

// historySub is history_sub(): "^old^new" repeats the latest input line
// containing old, with new in its place.
func (h *inputHist) sub(line string) (string, bool) {
	body := line[1:]
	i := strings.IndexByte(body, '^')
	if i < 0 {
		return "", false
	}
	pat := body[:i]
	repl := line[1+i+1:]
	for k := 1; k < len(h.lines); k++ {
		src := h.lines[len(h.lines)-1-k]
		if loc := strings.Index(src, pat); loc >= 0 {
			return src[:loc] + repl + src[loc+len(pat):], true
		}
	}
	return "", false
}

// SubmitInput runs the input line as if the user pressed Enter (handle_input_line):
// ^old^new history substitution, %kecho, recording in the input history, then
// execution (the line is what a user typed, so it is executed at %sub level).
// It returns the line's result.
func (in *Interp) SubmitInput() Value {
	in.PollVars()
	s := in.ts()
	line := s.kb.text
	s.kb.text, s.kb.pos = "", 0
	s.pending = false
	s.kb.changed()
	if strings.HasPrefix(line, "^") {
		sub, ok := s.hist.sub(line)
		if !ok {
			in.echo("% No match.")
			return Int(0)
		}
		line = sub
	}
	if in.varOn("kecho") {
		in.Host.Echo(in.globals["kprefix"].String() + line)
	}
	s.hist.record(in, line)
	if in.globals["sub"].String() == "off" {
		return in.Exec(line)
	}
	return in.ExecFull(line)
}

// PendingInput reports whether a NEWLINE was queued (by /dokey newline inside a
// macro) and the line has not run yet; FlushInput runs it. KeyPressed does so
// itself when it finishes.
func (in *Interp) PendingInput() bool { return in.ts().pending }

// FlushInput runs a queued input line, if any.
func (in *Interp) FlushInput() {
	if in.ts().pending {
		in.SubmitInput()
	}
}

var _ = unicode.IsSpace
