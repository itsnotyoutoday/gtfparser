package gtfparser

// History: the input, local, global and per-world line histories, /recordline,
// /recall and /histsize. The histories are core data owned by the Interp; a
// client feeds lines in with RecordInput / RecordLocalLine / RecordWorldLine
// and reads them back with History / HistoryLines.

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// HistLine is one recorded line.
type HistLine struct {
	Text  string
	Time  time.Time
	Attrs string // display attributes in TF's letter notation ("g" gagged, "G" not recorded, "L" not logged, ...)
}

// History selectors for the exported accessors.
const (
	HistInput  = "input"
	HistLocal  = "local"
	HistGlobal = "global"
	HistWorld  = "world"
)

type history struct {
	max   int // 0: not allocated yet (a world history takes %histsize when its first line arrives)
	lines []HistLine
	total int // number of lines ever recorded; line numbers are 1-based and keep counting
	log   *logFile
}

type histState struct {
	input, local, global *history
	inputIdx             int // line-editor position in the input history (-1 = at the end)
}

func (h *histState) init() {
	h.input = &history{max: 100}
	h.local = &history{max: 100}
	h.global = &history{max: 1000}
	h.inputIdx = -1
	h.input.add(HistLine{}) // the input history always ends with the (blank) line being typed
}

func (h *history) add(l HistLine) {
	h.lines = append(h.lines, l)
	h.total++
	if h.max > 0 && len(h.lines) > h.max {
		h.lines = h.lines[len(h.lines)-h.max:] // append reallocates (and drops the old front) when the capacity runs out
	}
}

func (h *history) resize(max int) {
	h.max = max
	if len(h.lines) > max {
		h.lines = append([]HistLine(nil), h.lines[len(h.lines)-max:]...)
	}
}

func (h *history) start() int { return h.total - len(h.lines) } // absolute index of lines[0]

func hasAttr(attrs string, c byte) bool { return strings.IndexByte(attrs, c) >= 0 }

// record stores a line (honoring the G and L attributes) and logs it.
func (in *Interp) record(h *history, l HistLine) {
	if l.Time.IsZero() {
		l.Time = in.now()
	}
	if !hasAttr(l.Attrs, 'G') {
		if h.max == 0 {
			h.max = int(in.globals["histsize"].Int())
			if h.max <= 0 {
				h.max = 1000
			}
		}
		h.add(l)
	}
	if h.log != nil && !hasAttr(l.Attrs, 'L') {
		in.writeLog(h.log, l)
	}
}

// ---------------------------------------------------------------- exported feeding / reading

// RecordInput records a line the user typed (TF's record_input): consecutive
// duplicates are collapsed, and the input log (/log -i) gets the line.
func (in *Interp) RecordInput(line string) { in.recordInput(HistLine{Text: line}, true) }

// RecordInputAttr is RecordInput for a line typed while echo was off (a
// password): pass attrs "g" so /recall and the history keys skip it.
func (in *Interp) RecordInputAttr(line, attrs string) {
	in.recordInput(HistLine{Text: line, Attrs: attrs}, true)
}

func (in *Interp) recordInput(l HistLine, log bool) {
	h := in.ds().h.input
	l.Time = in.now()
	in.ds().h.inputIdx = -1
	dup := len(h.lines) > 1 && h.lines[len(h.lines)-2].Text == l.Text
	if !dup {
		h.lines[len(h.lines)-1] = l // replaces the blank slot
		h.add(HistLine{Time: l.Time})
	}
	if log && h.log != nil {
		in.writeLog(h.log, l)
	}
}

// RecordLocalLine records a line of TF's own output (local and global histories).
func (in *Interp) RecordLocalLine(line string) { in.RecordLocalLineAttr(line, "") }

// RecordLocalLineAttr is RecordLocalLine with display attributes.
func (in *Interp) RecordLocalLineAttr(line, attrs string) {
	l := HistLine{Text: line, Attrs: attrs, Time: in.now()}
	d := in.ds()
	in.record(d.h.local, l)
	in.record(d.h.global, l)
}

// RecordWorldLine records a line received from a world (that world's history
// and the global one; each log attached to them gets the line too).
func (in *Interp) RecordWorldLine(world, line string) { in.RecordWorldLineAttr(world, line, "") }

// RecordWorldLineAttr is RecordWorldLine with display attributes.
func (in *Interp) RecordWorldLineAttr(world, line, attrs string) {
	l := HistLine{Text: line, Attrs: attrs, Time: in.now()}
	if h := in.worldHistory(world); h != nil {
		in.record(h, l)
	}
	in.record(in.ds().h.global, l)
}

// History returns a copy of a history, oldest line first, and the number of
// the first line returned (line numbers are 1-based and never reused). kind is
// HistInput, HistLocal, HistGlobal or HistWorld (world names the world).
func (in *Interp) History(kind, world string) (lines []HistLine, first int) {
	h := in.historyOf(kind, world)
	if h == nil {
		return nil, 1
	}
	return append([]HistLine(nil), h.lines...), h.start() + 1
}

// HistoryLines is History without the attributes: just the text.
func (in *Interp) HistoryLines(kind, world string) []string {
	ls, _ := in.History(kind, world)
	out := make([]string, len(ls))
	for i, l := range ls {
		out[i] = l.Text
	}
	return out
}

func (in *Interp) historyOf(kind, world string) *history {
	d := in.ds()
	switch kind {
	case HistInput:
		return d.h.input
	case HistLocal:
		return d.h.local
	case HistGlobal:
		return d.h.global
	case HistWorld:
		return in.worldHistory(world)
	}
	return nil
}

// RecallInput steps through the input history for the line editor, like
// TF's recall_input: mode 0 steps n lines (n<0 older), mode 1 searches n times
// for an older/newer line starting with the line being typed, mode 2 goes to the
// oldest (n<0) or newest (n>0). cur is the text currently in the editor; it is
// kept so stepping back past the newest line restores it. ok is false at the end.
func (in *Interp) RecallInput(n, mode int, cur string) (line string, ok bool) {
	h := in.ds().h.input
	last := len(h.lines) - 1
	idx := in.inputIndex()
	if idx == last {
		h.lines[last] = HistLine{Text: cur, Time: in.now()}
	}
	stop := last
	if n < 0 {
		stop = 0
	}
	if idx == stop {
		return "", false
	}
	dir := 1
	if n < 0 {
		dir = -1
	}
	i := stop
	var pat string
	if mode != 2 {
		i = idx + dir
		if mode == 1 {
			pat = h.lines[last].Text
		}
	}
	if n < 0 {
		n = -n
	}
	for {
		l := h.lines[i]
		if !hasAttr(l.Attrs, 'g') && (pat == "" || (len(l.Text) > len(pat) && strings.HasPrefix(l.Text, pat))) {
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
	in.setInputIndex(i)
	return h.lines[i].Text, true
}

// ---------------------------------------------------------------- input index (editor position)

func (in *Interp) inputIndex() int {
	d := in.ds()
	if d.h.inputIdx < 0 || d.h.inputIdx >= len(d.h.input.lines) {
		return len(d.h.input.lines) - 1
	}
	return d.h.inputIdx
}

func (in *Interp) setInputIndex(i int) { in.ds().h.inputIdx = i }

// ---------------------------------------------------------------- option handling shared by /recall /recordline /histsize /log

// pickHistory resolves the -l -i -g -w options in opts. def is used when none
// was given (nil = the caller decides). ok is false after an error.
func (in *Interp) pickHistory(cmd string, opts []tfOpt, def *history) (h *history, kind string, ok bool) {
	h, kind = def, ""
	selected := 0
	for _, o := range opts {
		switch o.c {
		case 'l', 'i', 'g', 'w':
			selected++
			if selected > 1 {
				in.warn("%s -%c: only one of the -ligw options may be used.", cmd, o.c)
				return nil, "", false
			}
		}
		switch o.c {
		case 'l':
			h, kind = in.ds().h.local, HistLocal
		case 'i':
			h, kind = in.ds().h.input, HistInput
		case 'g':
			h, kind = in.ds().h.global, HistGlobal
		case 'w':
			w := in.namedOrCurrentWorld(cmd+" -w", o.arg)
			if w == nil {
				return nil, "", false
			}
			h, kind = w.hist, HistWorld
		}
	}
	return h, kind, true
}

// outIsScreen reports whether tfout currently goes to the user's screen (and
// not into a pipe or $(...)).
func (in *Interp) outIsScreen() bool {
	_, piped := in.Host.(*captureHost)
	return !piped
}

// ---------------------------------------------------------------- /recordline

func cmdRecordline(in *Interp, args string) Value {
	opts, rest, ok := in.scanOpts("/recordline", "lgiw:t@a:p", args)
	if !ok {
		return Int(0)
	}
	var histOpts []tfOpt
	var when time.Time
	attrs := ""
	decode := false
	for _, o := range opts {
		switch o.c {
		case 'l', 'g', 'i', 'w':
			histOpts = append(histOpts, o)
		case 't':
			n, ok := parseTFNumber(o.arg)
			if !ok {
				in.warn("/recordline -t: invalid time %q", o.arg)
				return Int(0)
			}
			if n.isInt {
				when = time.Unix(n.ival, 0)
			} else {
				when = time.Unix(n.sec, n.usec*1000)
			}
		case 'p':
			decode = true
		case 'a':
			a, ok := in.histParseAttrs("/recordline -a", o.arg)
			if !ok {
				return Int(0)
			}
			attrs = mergeAttrs(attrs, a)
		}
	}
	d := in.ds()
	h, kind, ok := in.pickHistory("/recordline", histOpts, d.h.global)
	if !ok {
		return Int(0)
	}
	text := rest
	if decode {
		text, attrs = decodeInlineAttrs(rest, attrs)
	}
	l := HistLine{Text: text, Time: when, Attrs: attrs}
	// the line is never logged (nolog)
	if kind == HistInput {
		in.recordInput(l, false)
		return Int(1)
	}
	if l.Time.IsZero() {
		l.Time = in.now()
	}
	if !hasAttr(l.Attrs, 'G') {
		if h.max == 0 {
			h.max = int(in.globals["histsize"].Int())
		}
		h.add(l)
	}
	return Int(1)
}

// ---------------------------------------------------------------- number parsing for recall ranges

type tfNumber struct {
	isInt   bool  // plain integer (no '.' or ':')
	ival    int64 // for integers
	hasTime bool
	hms     bool // h:m:s form: a clock time / period of day
	sec     int64
	usec    int64
	rest    string
}

// parseTFNumber is TF's parsenumber for the (DTIME|INT) set: an integer, a
// real "sec.frac" (6 places) or "h:m[:s[.f]]".
func parseTFNumber(s string) (tfNumber, bool) {
	var n tfNumber
	neg := false
	for len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		if s[0] == '-' {
			neg = !neg
		}
		s = s[1:]
	}
	j := 0
	for j < len(s) && s[j] >= '0' && s[j] <= '9' {
		j++
	}
	v, err := strconv.ParseInt(s[:j], 10, 64)
	if j == 0 {
		v = 0
	} else if err != nil {
		return n, false
	}
	rest := s[j:]
	if rest == "" || (rest[0] != '.' && rest[0] != ':') {
		n.isInt, n.ival, n.rest = true, v, rest
		if neg {
			n.ival = -v
		}
		return n, j > 0
	}
	n.hasTime, n.sec = true, v
	frac := func(r string) (int64, string) {
		var us int64
		d := 0
		for d < 6 && len(r) > 0 && r[0] >= '0' && r[0] <= '9' {
			us = us*10 + int64(r[0]-'0')
			r = r[1:]
			d++
		}
		for ; d < 6; d++ {
			us *= 10
		}
		for len(r) > 0 && r[0] >= '0' && r[0] <= '9' {
			r = r[1:]
		}
		return us, r
	}
	readInt := func(r string) (int64, string) {
		k := 0
		for k < len(r) && r[k] >= '0' && r[k] <= '9' {
			k++
		}
		x, _ := strconv.ParseInt(r[:k], 10, 64)
		return x, r[k:]
	}
	if rest[0] == ':' {
		n.hms = true
		if n.sec > 596523 {
			return n, false
		}
		n.sec *= 3600
		rest = rest[1:]
		if len(rest) > 0 && rest[0] >= '0' && rest[0] <= '9' {
			var m int64
			m, rest = readInt(rest)
			n.sec += m * 60
			if len(rest) > 0 && rest[0] == ':' {
				rest = rest[1:]
				if len(rest) > 0 && rest[0] >= '0' && rest[0] <= '9' {
					var sc int64
					sc, rest = readInt(rest)
					n.sec += sc
				}
			}
		}
		if len(rest) > 0 && rest[0] == '.' {
			n.usec, rest = frac(rest[1:])
		}
	} else {
		n.usec, rest = frac(rest[1:])
	}
	if neg {
		n.sec, n.usec = -n.sec, -n.usec
	}
	n.rest = rest
	return n, true
}

// toTime converts a time value; clock times (h:m:s) become the latest such
// time of day not in the future when abs is set (TF's abstime).
func (n tfNumber) toTime(now time.Time, abs bool) time.Time {
	if n.hms && abs {
		t := time.Date(now.Year(), now.Month(), now.Day(), int(n.sec/3600), int(n.sec/60%60), int(n.sec%60), 0, now.Location())
		if t.After(now) {
			t = t.AddDate(0, 0, -1)
		}
		return t.Add(time.Duration(n.usec) * time.Microsecond)
	}
	return time.Unix(n.sec, n.usec*1000)
}

// ---------------------------------------------------------------- attributes

var histColorNames = []string{"black", "red", "green", "yellow", "blue", "magenta", "cyan", "white", "gray", "brightred", "brightgreen", "brightyellow", "brightblue", "brightmagenta", "brightcyan", "brightwhite"}

func validColor(name string) bool {
	for _, c := range histColorNames {
		if name == c {
			return true
		}
	}
	if len(name) == 6 && strings.HasPrefix(name, "rgb") {
		for _, c := range name[3:] {
			if c < '0' || c > '5' {
				return false
			}
		}
		return true
	}
	if n, err := strconv.Atoi(name); err == nil && n >= 0 && n < 256 {
		return true
	}
	return false
}

// histParseAttrs validates TF's attribute letters ("n x G L A g u r f i d B b h E W I
// Cname Cbgname", comma separated) and returns them normalized.
func (in *Interp) histParseAttrs(cmd, s string) (string, bool) {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case ',':
		case 'n', 'x', 'G', 'L', 'A', 'g', 'u', 'r', 'f', 'i', 'd', 'B', 'b', 'h', 'E', 'W', 'I':
			b.WriteByte(c)
		case 'C':
			j := i + 1
			for j < len(s) && s[j] != ',' {
				j++
			}
			name := s[i+1 : j]
			col := strings.TrimPrefix(name, "bg")
			if !validColor(col) {
				kind := "color"
				if strings.HasPrefix(name, "bg") {
					kind = "bgcolor"
				}
				in.warn("%s: Invalid %s value \"%s\".", cmd, kind, col)
				return "", false
			}
			b.WriteString("C" + name + ",")
			i = j
		default:
			in.warn("%s: invalid display attribute '%c'", cmd, c)
			return "", false
		}
	}
	return b.String(), true
}

func mergeAttrs(a, b string) string { return a + b }

// decodeInlineAttrs handles /recordline -p: "@{attr}" sets attributes, "@@" is "@".
// Only whole-line attributes can be kept, so inline ones are folded into the line's set.
func decodeInlineAttrs(s, attrs string) (string, string) {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '@' && i+1 < len(s) {
			if s[i+1] == '@' {
				b.WriteByte('@')
				i++
				continue
			}
			if s[i+1] == '{' {
				if e := strings.IndexByte(s[i:], '}'); e > 0 {
					spec := s[i+2 : i+e]
					if spec != "n" && spec != "x" {
						attrs += spec
					}
					i += e
					continue
				}
			}
		}
		b.WriteByte(s[i])
	}
	return b.String(), attrs
}

// ---------------------------------------------------------------- /recall

func cmdRecall(in *Interp, args string) Value {
	opts, rest, ok := in.scanOpts("/recall", "ligw:a:f:t:m:vqA#B#C#", args)
	if !ok {
		return Int(0)
	}
	style := ""
	timeFmt, haveTime := "", false
	attrs := ""
	truth, quiet := true, false
	before, after := 0, 0
	var histOpts []tfOpt
	for _, o := range opts {
		switch o.c {
		case 'l', 'i', 'g', 'w':
			histOpts = append(histOpts, o)
		case 'a', 'f':
			a, ok := in.histParseAttrs("/recall -"+string(o.c), o.arg)
			if !ok {
				return Int(0)
			}
			attrs += a
		case 't':
			timeFmt, haveTime = o.arg, true
		case 'm':
			s, ok := in.matchStyleName("/recall", o.arg)
			if !ok {
				return Int(0)
			}
			style = s
		case 'v':
			truth = false
		case 'q':
			quiet = true
		case 'A':
			after = int(o.ival)
		case 'B':
			before = int(o.ival)
		case 'C':
			before, after = int(o.ival), int(o.ival)
		}
	}
	var def *history
	if w := in.CurrentWorld(); w != "" {
		if wh := in.worldHistory(w); wh != nil {
			def = wh
		}
	}
	if def == nil {
		def = in.ds().h.global
	}
	h, kind, ok := in.pickHistory("/recall", histOpts, def)
	if !ok {
		return Int(0)
	}
	_ = kind
	ptr := rest
	numbers := false
	if strings.HasPrefix(ptr, "#") {
		numbers = true
		ptr = ptr[1:]
	}
	ptr = strings.TrimLeft(ptr, " \t")

	now := in.now()
	n0, n1 := 0, h.total-1
	want := len(h.lines)
	var tv0, tv1 time.Time
	have0, have1 := false, false
	synErr := func() Value { in.warn("/recall: syntax error in recall range"); return Int(0) }
	switch {
	case ptr == "":
		in.warn("/recall: missing arguments")
		return Int(0)
	case ptr[0] == '-':
		num, ok := parseTFNumber(ptr[1:])
		if !ok {
			return synErr()
		}
		ptr = num.rest
		if num.hasTime {
			have1, tv1 = true, num.toTime(now, num.hms)
		} else {
			n0 = h.total - int(num.ival)
			n1 = n0
		}
	case ptr[0] == '/':
		j := 1
		for j < len(ptr) && ptr[j] >= '0' && ptr[j] <= '9' {
			j++
		}
		w, _ := strconv.Atoi(ptr[1:j])
		want = w
		ptr = ptr[j:]
	case ptr[0] >= '0' && ptr[0] <= '9':
		num, ok := parseTFNumber(ptr)
		if !ok {
			return synErr()
		}
		ptr = num.rest
		if num.hasTime {
			have0, tv0 = true, time.Unix(num.sec, num.usec*1000)
		} else {
			n0 = int(num.ival)
		}
		if !strings.HasPrefix(ptr, "-") { // x
			if num.hasTime {
				tv0 = now.Add(-time.Duration(num.sec)*time.Second - time.Duration(num.usec)*time.Microsecond)
			} else {
				n0 = h.total - n0
			}
		} else {
			ptr = ptr[1:]
			if num.hms {
				tv0 = num.toTime(now, true)
			}
			if !num.hasTime {
				n0--
			}
			if len(ptr) > 0 && ptr[0] >= '0' && ptr[0] <= '9' { // x-y
				y, ok := parseTFNumber(ptr)
				if !ok {
					return synErr()
				}
				ptr = y.rest
				if y.hasTime {
					have1, tv1 = true, y.toTime(now, y.hms)
				} else {
					n1 = int(y.ival) - 1
				}
			}
		}
	}
	if ptr != "" && ptr[0] != ' ' && ptr[0] != '\t' {
		in.warn("/recall: extra characters after recall range: %s", ptr)
		return Int(0)
	}
	pat := strings.TrimLeft(ptr, " \t")
	matches := func(string) bool { return true }
	if pat != "" {
		m, ok := in.matchStyle("/recall", style, pat)
		if !ok {
			return Int(0)
		}
		matches = m
	}
	if len(h.lines) == 0 {
		return Int(0)
	}
	screen := !quiet && in.outIsScreen()
	if screen {
		in.echo("================ Recall start ================")
	}
	start := h.start()
	if n0 < start {
		n0 = start
	}
	if n1 >= h.total {
		n1 = h.total - 1
	}
	showGag := hasAttr(attrs, 'g')
	gagOn := in.globals["gag"].Truth()
	at := func(i int) HistLine { return h.lines[i-start] }
	var stack []string // earliest line last; we prepend as we walk back
	push := func(s string) { stack = append(stack, s) }
	count := 0
	if n0 <= n1 && (!have0 || !have1 || !tv0.After(tv1)) {
		lastprinted := n1 + 1
		incontext := 0
		outOfRange := false
		jump := 256
		isGlobal := h == in.ds().h.global
		for i := n1; i >= start; i-- {
			if i < n0 || want <= 0 {
				if incontext > 0 {
					outOfRange = true
				} else {
					break
				}
			}
			line := at(i)
			if have1 && line.Time.After(tv1) {
				if isGlobal || jump == 0 {
					continue
				}
				for i -= jump; i >= n0; i -= jump {
					if !at(i).Time.After(tv1) {
						break
					}
				}
				i += jump
				jump = 0
				continue
			}
			if have0 && line.Time.Before(tv0) {
				if incontext > 0 {
					outOfRange = true
				} else {
					if isGlobal {
						continue
					}
					break
				}
			}
			if gagOn && hasAttr(line.Attrs, 'g') && !showGag {
				continue
			}
			var j int
			if !outOfRange && matches(line.Text) == truth {
				want--
				j = i + after
				if j >= lastprinted-1 {
					j = lastprinted - 1
				} else if (before > 0 || after > 0) && len(stack) > 0 {
					push("--")
				}
				incontext = before
			} else if incontext > 0 {
				incontext--
				j = i
			} else {
				continue
			}
			for ; j >= i; j-- {
				l := at(j)
				var b strings.Builder
				if numbers {
					fmt.Fprintf(&b, "%d: ", j+1)
				}
				if haveTime {
					if timeFmt == "" {
						b.WriteString("[" + tfStrftime(in.globals["time_format"].String(), l.Time) + "]")
					} else {
						b.WriteString(tfStrftime(timeFmt, l.Time))
					}
					b.WriteByte(' ')
				}
				b.WriteString(l.Text)
				push(b.String())
				lastprinted = j
				count++
			}
		}
	}
	for i := len(stack) - 1; i >= 0; i-- {
		in.echo(stack[i])
	}
	if screen {
		in.echo("================= Recall end =================")
	}
	return Int(int64(count))
}

// ---------------------------------------------------------------- /histsize

func histName(kind string) string {
	switch kind {
	case HistGlobal, HistLocal, HistInput:
		return kind
	}
	return "world"
}

func cmdHistsize(in *Interp, args string) Value {
	opts, rest, ok := in.scanOpts("/histsize", "ligw:", args)
	if !ok {
		return Int(0)
	}
	h, kind, ok := in.pickHistory("/histsize", opts, in.ds().h.global)
	if !ok {
		return Int(0)
	}
	if kind == "" {
		kind = HistGlobal
	}
	maxsize := 0
	rest = strings.TrimSpace(rest)
	if rest != "" {
		maxsize = in.numArg("/histsize", &rest)
		if maxsize <= 0 {
			return Int(0)
		}
		h.resize(maxsize)
	}
	size := h.max
	if size == 0 {
		size = int(in.globals["histsize"].Int())
	}
	verb := "is"
	if maxsize != 0 {
		verb = "changed to"
	}
	in.echo(fmt.Sprintf("%% %s history capacity %s %d lines.", histName(kind), verb, size))
	if kind == HistInput {
		in.setInputIndex(-1)
	}
	return Int(int64(size))
}

func init() {
	builtinCmds["recordline"] = cmdRecordline
	builtinCmds["recall"] = cmdRecall
	builtinCmds["histsize"] = cmdHistsize
}
