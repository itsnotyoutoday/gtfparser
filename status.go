package gtfparser

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// The status area: up to maxStatusHeight rows of fields. The interpreter owns
// the field definitions and computes the rendered rows (StatusRows); the client
// draws them whenever it gets a "status" event.

const maxStatusHeight = 6

// StatusKind says what a status field shows.
type StatusKind int

const (
	StatusPad      StatusKind = iota // unnamed: padding between the fields beside it
	StatusLiteral                    // a quoted string, never updated
	StatusVar                        // the value of a variable (or its %status_var_<name> expression)
	StatusInternal                   // @more @world @read @active @log @mail @clock
)

// StatusField is one field of a status row.
type StatusField struct {
	Kind      StatusKind
	Name      string // literal text, variable name, or internal name without "@"
	Width     int    // columns; 0 for a variable or internal field means "all that is left"
	RightJust bool
	Attrs     string // display attributes given in the definition (canonical form)
}

// statusInternals are the internal statuses, in tf's order.
var statusInternals = []string{"more", "world", "read", "active", "log", "mail", "clock"}

type statusState struct {
	rows    [maxStatusHeight][]StatusField
	lastVar string // value of %status_fields as last synchronised
	warned  map[string]bool
}

func (st *statusState) init() { st.warned = map[string]bool{} }

func (f StatusField) isSpacer() bool { return f.Kind == StatusPad }

// fmtVar and attrVar name the variables that format and style a field.
func (f StatusField) fmtVar() string {
	switch f.Kind {
	case StatusInternal:
		return "status_int_" + f.Name
	case StatusVar:
		return "status_var_" + f.Name
	}
	return ""
}

func (f StatusField) attrVar() string {
	switch f.Kind {
	case StatusInternal:
		return "status_attr_int_" + f.Name
	case StatusVar:
		return "status_attr_var_" + f.Name
	}
	return ""
}

func (st *statusState) varNames() []string {
	var out []string
	for _, row := range st.rows {
		for _, f := range row {
			switch f.Kind {
			case StatusVar:
				out = append(out, f.Name, f.fmtVar(), f.attrVar())
			case StatusInternal:
				out = append(out, f.fmtVar(), f.attrVar())
			}
		}
	}
	return out
}

func (st *statusState) usesVar(name string) bool {
	for _, n := range st.varNames() {
		if n == name {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------- display attributes

// enumError is tf's enum2int complaint: the valid values, with the gagged
// (intermediate) ones elided.
func enumList(names []string, shown func(i int) bool) string {
	var b strings.Builder
	prefix := ""
	for i, n := range names {
		if !shown(i) {
			prefix = " ... "
			continue
		}
		fmt.Fprintf(&b, "%s%s (%d)", prefix, n, i)
		prefix = ", "
	}
	return b.String()
}

// parseAttrs parses an attribute string such as "Br" or "Cred,Cbgblue" up to
// the end or a space, returning the unparsed rest. It is the shared
// implementation in attrs.go.
func (in *Interp) parseAttrs(s string) (Attr, string, error) {
	return in.attrCtx().Parse(s, ' ')
}

// ---------------------------------------------------------------- field syntax

func isQuoteChar(c byte) bool { return c == '"' || c == '\'' || c == '`' }
func isWordChar(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_'
}

// parseStatusField parses one field spec at the start of s ("name[:width[:attrs]]")
// and returns it with the rest of s.
func (in *Interp) parseStatusField(s string) (StatusField, string, error) {
	var f StatusField
	switch {
	case s != "" && isQuoteChar(s[0]):
		q := s[0]
		var b strings.Builder
		i := 1
		for ; i < len(s) && s[i] != q; i++ {
			if s[i] == '\\' && i+1 < len(s) && (s[i+1] == q || s[i+1] == '\\') {
				i++
			}
			b.WriteByte(s[i])
		}
		if i >= len(s) {
			return f, "", fmt.Errorf("unmatched %c", q)
		}
		f.Kind, f.Name = StatusLiteral, b.String()
		s = s[i+1:]
	case s != "" && s[0] == '@':
		i := 1
		for i < len(s) && isWordChar(s[i]) {
			i++
		}
		name := s[1:i]
		idx := -1
		for k, n := range statusInternals {
			if strings.EqualFold(n, name) {
				idx = k
			}
		}
		if idx < 0 {
			if v, err := strconv.Atoi(name); err == nil && v >= 0 && v < len(statusInternals) {
				idx = v
			}
		}
		if idx < 0 {
			return f, "", fmt.Errorf("Invalid status_fields internal status value \"%s\".  Valid values are: %s",
				name, enumList(statusInternals, func(int) bool { return true }))
		}
		f.Kind, f.Name = StatusInternal, statusInternals[idx]
		s = s[i:]
	case s != "" && isWordChar(s[0]):
		i := 0
		for i < len(s) && isWordChar(s[i]) {
			i++
		}
		f.Kind, f.Name = StatusVar, s[:i]
		s = s[i:]
	default:
		f.Kind = StatusPad
	}
	if strings.HasPrefix(s, ":") {
		s = s[1:]
		for strings.HasPrefix(s, "-") {
			f.RightJust = !f.RightJust
			s = s[1:]
		}
		i := 0
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		f.Width, _ = strconv.Atoi(s[:i])
		s = s[i:]
	}
	if strings.HasPrefix(s, ":") {
		a, rest, err := in.parseAttrs(s[1:])
		if err != nil {
			return f, "", err
		}
		f.Attrs = a.String()
		s = rest
	}
	if s != "" && s[0] != ' ' && s[0] != '\t' {
		g := s
		if len(g) > 8 {
			g = g[:8]
		}
		return f, "", fmt.Errorf("garbage in status field: %s", g)
	}
	return f, s, nil
}

// text is the field's definition as tf prints it (status_fields()).
func (f StatusField) text() string {
	var b strings.Builder
	w := f.Width
	switch f.Kind {
	case StatusInternal:
		b.WriteString("@" + f.Name)
	case StatusVar:
		b.WriteString(f.Name)
	case StatusLiteral:
		b.WriteString(`"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(f.Name) + `"`)
		if w == utf8.RuneCountInString(f.Name) {
			w = 0
		}
	}
	if w == 0 && f.Attrs == "" && !f.RightJust {
		return b.String()
	}
	b.WriteByte(':')
	if w != 0 || f.RightJust {
		if f.RightJust {
			b.WriteByte('-')
		}
		b.WriteString(strconv.Itoa(w))
	}
	if f.Attrs != "" {
		b.WriteString(":" + f.Attrs)
	}
	return b.String()
}

func (in *Interp) statusRowText(row int) string {
	if row < 0 || row >= maxStatusHeight {
		return ""
	}
	var parts []string
	for _, f := range in.ts().st.rows[row] {
		parts = append(parts, f.text())
	}
	return strings.Join(parts, " ")
}

// StatusFields returns the definitions of row's fields.
func (in *Interp) StatusFields(row int) []StatusField {
	in.statusSync()
	if row < 0 || row >= maxStatusHeight {
		return nil
	}
	return append([]StatusField(nil), in.ts().st.rows[row]...)
}

// ---------------------------------------------------------------- editing the rows

func (in *Interp) statusHeight() int {
	h := in.varInt("status_height")
	if h > maxStatusHeight {
		h = maxStatusHeight
	}
	return h
}

// statusRegen is regen_status_fields: row 0 is mirrored into %status_fields
// (without triggering the deprecation warning) and clients are told.
func (in *Interp) statusRegen(row int) {
	st := &in.ts().st
	if row == 0 {
		t := in.statusRowText(0)
		st.lastVar = t
		in.globals["status_fields"] = Str(t)
		if s := in.ts(); s.snap != nil {
			s.snap["status_fields"] = t
		}
	}
	st.warned = map[string]bool{}
	in.notify("status", "fields", strconv.Itoa(row))
}

// statusSync notices an assignment to %status_fields (the deprecated way to
// define row 0) and applies it. Called before every status operation.
func (in *Interp) statusSync() {
	in.PollVars()
}

func (in *Interp) statusFieldsVarChanged(old, new string) {
	st := &in.ts().st
	if new == st.lastVar {
		return
	}
	if in.varOn("warn_status") {
		in.warn("Warning: setting status_fields directly is deprecated, and may clobber useful new " +
			"features introduced in version 5.  The recommended way to change status fields is with " +
			"/status_add, /status_rm, or /status_edit. (This warning can be disabled with \"/set warn_status=off\".)")
	}
	if !in.statusAdd("/status_fields", true, false, 0, -1, new, 0) {
		in.globals["status_fields"] = Str(old)
		if s := in.ts(); s.snap != nil {
			s.snap["status_fields"] = old
		}
	}
}

func (st *statusState) vwf(row int) int {
	for i, f := range st.rows[row] {
		if f.Width == 0 {
			return i
		}
	}
	return -1
}

func (in *Interp) totalWidth(row int) int {
	n := 0
	for _, f := range in.ts().st.rows[row] {
		n += f.Width
	}
	return n
}

// findField is find_statusfield_by_template: the first field of row (any row
// when row < 0, among the first %status_height) matching target. A full
// comparison also requires equal width and attributes when target has them.
func (in *Interp) findField(target StatusField, row int, full bool, label string) (int, int, bool) {
	st := &in.ts().st
	for r := 0; r < in.statusHeight(); r++ {
		if row >= 0 && r != row {
			continue
		}
		for i, f := range st.rows[r] {
			switch {
			case target.Kind == StatusInternal:
				if f.Kind != StatusInternal || f.Name != target.Name {
					continue
				}
			case target.Kind == StatusVar:
				if f.Kind != StatusVar || f.Name != target.Name {
					continue
				}
			case target.Kind == StatusLiteral:
				if f.Kind != StatusLiteral || !strings.EqualFold(f.Name, target.Name) {
					continue
				}
			}
			if full {
				if target.Width != 0 && target.Width != f.Width {
					continue
				}
				if target.Attrs != "" && target.Attrs != f.Attrs {
					continue
				}
			}
			return r, i, true
		}
	}
	if label != "" {
		if row < 0 {
			in.warn("%s: no such field", label)
		} else {
			in.warn("%s: no such field in row %d", label, row)
		}
	}
	return 0, 0, false
}

func (in *Interp) findFieldByName(row int, spec, who string) (int, int, bool) {
	f, _, err := in.parseStatusField(spec)
	if err != nil {
		in.warn("%s: %v", who, err)
		return 0, 0, false
	}
	return in.findField(f, row, true, spec)
}

// statusAdd is tf's status_add(): parse the field list s and insert the fields
// into row after index after (-1 = at the start); reset replaces the row.
// spacer > 0 puts a pad of that width before the new fields, < 0 after them.
func (in *Interp) statusAdd(who string, reset, nodup bool, row, after int, s string, spacer int) bool {
	st := &in.ts().st
	vwfFound := false
	if !reset {
		vwfFound = st.vwf(row) >= 0
	}
	totwidth := 0
	if !reset {
		totwidth = in.totalWidth(row)
	}
	var added []StatusField
	for {
		s = strings.TrimLeft(s, " \t")
		if s == "" {
			break
		}
		f, rest, err := in.parseStatusField(s)
		if err != nil {
			in.warn("%s: %v", who, err)
			return false
		}
		s = rest
		if f.Width == 0 && f.Kind == StatusLiteral {
			f.Width = utf8.RuneCountInString(f.Name)
		}
		if nodup {
			if _, _, ok := in.findField(f, row, false, ""); ok {
				continue
			}
		}
		if f.Width != 0 {
			totwidth += f.Width
		} else {
			if vwfFound {
				in.warn("%s: Only one variable width status field is allowed.", who)
				return false
			}
			vwfFound = true
		}
		added = append(added, f)
	}
	if spacer != 0 && len(added) > 0 && !reset && len(st.rows[row]) > 0 {
		w := spacer
		if w < 0 {
			w = -w
		}
		totwidth += w
		pad := StatusField{Kind: StatusPad, Width: w}
		if spacer >= 0 {
			added = append([]StatusField{pad}, added...)
		} else {
			added = append(added, pad)
		}
	}
	if cols, _ := in.termSize(); totwidth > cols {
		in.warn("Warning: total status width (%d) is wider than screen (%d)", totwidth, cols)
	}
	if reset {
		st.rows[row] = added
	} else {
		cur := st.rows[row]
		n := make([]StatusField, 0, len(cur)+len(added))
		n = append(n, cur[:after+1]...)
		n = append(n, added...)
		n = append(n, cur[after+1:]...)
		st.rows[row] = n
	}
	in.statusRegen(row)
	return true
}

// ---------------------------------------------------------------- commands

type statusOpts struct {
	after, before string
	hasA, hasB    bool
	spacer        int
	row           int
	nodup, reset  bool
	rest          string
}

// parseStatusOpts parses the options of the status_* commands; letters lists
// them in tf's startopt syntax ("A:B:s#r#xc").
func (in *Interp) parseStatusOpts(who, letters, args string) (statusOpts, bool) {
	o := statusOpts{spacer: 1, row: -1}
	s := strings.TrimLeft(args, " \t")
	for strings.HasPrefix(s, "-") {
		if len(s) == 1 || s[1] == ' ' || s[1] == '\t' { // a lone "-" ends the options
			s = strings.TrimLeft(s[1:], " \t")
			break
		}
		if strings.HasPrefix(s, "--") {
			s = strings.TrimLeft(s[2:], " \t")
			break
		}
		i := 1
		for i < len(s) && s[i] != ' ' && s[i] != '\t' {
			c := s[i]
			i++
			k := strings.IndexByte(letters, c)
			if k < 0 || c == ':' || c == '#' {
				in.warn("%s: invalid option '%c'", who, c)
				return o, false
			}
			takes := k+1 < len(letters) && (letters[k+1] == ':' || letters[k+1] == '#')
			if !takes {
				switch c {
				case 'x':
					o.nodup = true
				case 'c':
					o.reset = true
				}
				continue
			}
			// the argument is the rest of the word, or a quoted string
			var val string
			if i < len(s) && isQuoteChar(s[i]) {
				q := s[i]
				j := i + 1
				var b strings.Builder
				for ; j < len(s) && s[j] != q; j++ {
					if s[j] == '\\' && j+1 < len(s) && (s[j+1] == q || s[j+1] == '\\') {
						j++
					}
					b.WriteByte(s[j])
				}
				val = b.String()
				i = j + 1
			} else {
				j := i
				for j < len(s) && s[j] != ' ' && s[j] != '\t' {
					j++
				}
				val, i = s[i:j], j
			}
			switch c {
			case 'A':
				o.after, o.hasA, o.hasB = val, true, false
			case 'B':
				o.before, o.hasB, o.hasA = val, true, false
			case 's', 'r':
				v, err := in.EvalExpr(val)
				if err != nil {
					in.warn("%s: %v", who, err)
					return o, false
				}
				if c == 's' {
					o.spacer = int(v.Int())
				} else {
					o.row = int(v.Int())
				}
			}
			break
		}
		s = strings.TrimLeft(s[i:], " \t")
	}
	o.rest = s
	return o, true
}

func cmdStatusAdd(in *Interp, args string) Value {
	in.statusSync()
	o, ok := in.parseStatusOpts("/status_add", "A:B:s#r#xc", args)
	if !ok {
		return Int(0)
	}
	row, after, spacer := o.row, 0, o.spacer
	st := &in.ts().st
	switch {
	case o.hasA:
		if o.after != "" {
			r, i, ok := in.findFieldByName(row, o.after, "/status_add")
			if !ok {
				return Int(0)
			}
			row, after = r, i
		} else {
			if row < 0 {
				row = 0
			}
			after = len(st.rows[rowClamp(row)]) - 1
		}
	case o.hasB:
		spacer = -spacer
		if o.before != "" {
			r, i, ok := in.findFieldByName(row, o.before, "/status_add")
			if !ok {
				return Int(0)
			}
			row, after = r, i-1
		} else {
			if row < 0 {
				row = 0
			}
			after = -1
		}
	default:
		if row < 0 {
			row = 0
		}
		after = len(st.rows[rowClamp(row)]) - 1
	}
	if row >= maxStatusHeight {
		in.warn("/status_add: row %d is out of range (0..%d)", row, maxStatusHeight-1)
		return Int(0)
	}
	return Bool(in.statusAdd("/status_add", o.reset, o.nodup, row, after, o.rest, spacer))
}

func rowClamp(r int) int {
	if r < 0 {
		return 0
	}
	if r >= maxStatusHeight {
		return maxStatusHeight - 1
	}
	return r
}

func cmdStatusClear(in *Interp, args string) Value {
	in.statusSync()
	o, ok := in.parseStatusOpts("/status_clear", "r#", args)
	if !ok || o.row < 0 {
		return Int(0)
	}
	if o.row >= maxStatusHeight {
		return Int(1)
	}
	if o.row < in.statusHeight() {
		in.ts().st.rows[o.row] = nil
	}
	in.statusRegen(o.row)
	return Int(1)
}

func getSpacer(rows []StatusField, i int) int {
	if i < 0 || i >= len(rows) || !rows[i].isSpacer() {
		return 0
	}
	return rows[i].Width
}

func cmdStatusRm(in *Interp, args string) Value {
	in.statusSync()
	o, ok := in.parseStatusOpts("/status_rm", "r#", args)
	if !ok {
		return Int(0)
	}
	row, i, ok := in.findFieldByName(o.row, o.rest, "/status_rm")
	if !ok {
		return Int(0)
	}
	rows := in.ts().st.rows[row]
	prev, next := i-1, i+1
	rm := map[int]bool{i: true}
	switch {
	case i == 0 && getSpacer(rows, next) != 0:
		rm[next] = true
	case i == len(rows)-1 && getSpacer(rows, prev) != 0:
		rm[prev] = true
	case getSpacer(rows, prev) != 0 && getSpacer(rows, next) != 0:
		if getSpacer(rows, prev) < getSpacer(rows, next) {
			rm[prev] = true
		} else {
			rm[next] = true
		}
	}
	var out []StatusField
	for k, f := range rows {
		if !rm[k] {
			out = append(out, f)
		}
	}
	in.ts().st.rows[row] = out
	in.statusRegen(row)
	return Int(1)
}

func cmdStatusEdit(in *Interp, args string) Value {
	in.statusSync()
	o, ok := in.parseStatusOpts("/status_edit", "r#", args)
	if !ok {
		return Int(0)
	}
	f, _, err := in.parseStatusField(o.rest)
	if err != nil {
		in.warn("/status_edit: %v", err)
		return Int(0)
	}
	row, i, ok := in.findField(f, o.row, false, o.rest)
	if !ok {
		return Int(0)
	}
	st := &in.ts().st
	if f.Width == 0 {
		if v := st.vwf(row); v >= 0 && v != i {
			in.warn("/status_edit: Only one variable width status field per row is allowed.")
			return Int(0)
		}
	}
	st.rows[row][i] = f
	in.statusRegen(row)
	return Int(1)
}

func fnStatusFields(in *Interp, a []Value) (Value, error) {
	in.statusSync()
	row := 0
	if len(a) > 0 {
		row = int(a[0].Int())
	}
	return Str(in.statusRowText(row)), nil
}

func fnStatusWidth(in *Interp, a []Value) (Value, error) {
	in.statusSync()
	name := str(a, 0)
	row, i, ok := in.findFieldByName(-1, name, "status_width")
	if !ok {
		return Int(0), nil
	}
	cols, _ := in.termSize()
	lay := in.layoutRow(row, cols)
	return Int(int64(lay[i].width)), nil
}

// ---------------------------------------------------------------- layout and rendering

type laidField struct {
	col, width int
	f          StatusField
}

// layoutRow computes where each field of row sits on a screen cols wide: the
// fields before the variable-width one are packed left, those after it right,
// and the variable-width field takes what is left in the middle.
func (in *Interp) layoutRow(row, cols int) []laidField {
	fields := in.ts().st.rows[row]
	col := make([]int, len(fields))
	left, right := 0, 0
	c := 0
	vwf := -1
	for i, f := range fields {
		left = c
		col[i] = c
		if f.Width == 0 {
			vwf = i
			break
		}
		c += f.Width
	}
	if vwf >= 0 {
		c = 0
		for i := len(fields) - 1; i > vwf; i-- {
			c -= fields[i].Width
			col[i] = c
			right = -c
		}
	}
	out := make([]laidField, len(fields))
	for i, f := range fields {
		start := col[i]
		if start >= 0 {
			if start > cols {
				start = cols
			}
		} else {
			tot := left + right
			if tot < cols {
				tot = cols
			}
			start += tot
			if start > cols {
				start = cols
			}
		}
		w := f.Width
		if start >= cols {
			w = 0
		} else {
			if f.Width == 0 {
				w = cols - right - left
			}
			if w > cols-start {
				w = cols - start
			}
			if w < 0 {
				w = 0
			}
		}
		out[i] = laidField{start, w, f}
	}
	return out
}

// StatusSpan is one rendered field.
type StatusSpan struct {
	Col, Width int    // position and size in columns
	Text       string // exactly Width columns: the value, justified and padded with %status_pad
	ValueStart int    // where the value starts inside Text (runes)
	ValueLen   int    // and how long it is (runes); the padding is outside it
	Attrs      string // %status_attr + the field's attributes + %status_attr_{int,var}_name
	Field      StatusField
}

// StatusRow is one rendered row of the status area.
type StatusRow struct {
	Row   int
	Text  string       // the whole row, width columns
	Attrs string       // %status_attr, which applies to the entire row including padding
	Spans []StatusSpan // the fields, left to right
}

func (in *Interp) fieldValue(f StatusField) string {
	switch f.Kind {
	case StatusLiteral:
		return f.Name
	case StatusInternal:
		if h, ok := in.Host.(StatusHost); ok {
			if t, ok := h.StatusValue(f.Name); ok {
				return t
			}
		}
		if v, ok := in.globals[f.fmtVar()]; ok {
			return in.evalStatusExpr(v.String(), f.fmtVar())
		}
		return ""
	case StatusVar:
		if v, ok := in.globals[f.fmtVar()]; ok {
			return in.evalStatusExpr(v.String(), f.fmtVar())
		}
		return in.globalStr(f.Name)
	}
	return ""
}

// statusWarn reports a problem found while drawing the status line once, not on
// every redraw: the same message is shown again only after the status
// definitions or format variables change.
func (in *Interp) statusWarn(msg string) { in.warnOnce(in.Warn, msg) }

func (in *Interp) warnOnce(out func(string), msg string) {
	st := &in.ts().st
	if st.warned[msg] {
		return
	}
	st.warned[msg] = true
	out(msg)
}

func (in *Interp) evalStatusExpr(src, who string) string {
	old, oldWarn := in.fr.noOut, in.Warn
	in.fr.noOut = true
	in.Warn = func(m string) { in.warnOnce(oldWarn, m) } // diagnostics from functions the expression calls
	// A status field's expression is stored in a variable by whoever set it and
	// run by the host on every redraw: when origins are in use it may not touch
	// the shell, files or the network (it cannot be traced to its author).
	oldForce := in.origForce
	if len(in.origLevels) > 0 {
		in.origForce = RestrictWorld
	}
	v, err := in.EvalExpr(src)
	in.origForce = oldForce
	in.Warn = oldWarn
	in.fr.noOut = old
	if err != nil {
		in.statusWarn(who + ": " + err.Error())
		return ""
	}
	return v.String()
}

func (in *Interp) globalStr(name string) string {
	if v, ok := in.globals[name]; ok {
		return v.String()
	}
	return ""
}

func (in *Interp) varAttrs(name string) Attr {
	a, _, err := in.parseAttrs(in.globalStr(name))
	if err != nil {
		in.statusWarn(name + ": " + err.Error())
		return 0
	}
	return a
}

func fitRunes(s string, w int) []rune {
	r := []rune(s)
	if len(r) > w {
		r = r[:w]
	}
	return r
}

// StatusRows renders the status area for a screen width columns wide: one row
// per %status_height, each a list of spans ready to draw. Internal fields come
// from the client's StatusHost when it supplies a value, otherwise from the
// %status_int_<name> expression; variable fields from %status_var_<name> or the
// variable itself. A problem (a bad expression, a function the client lacks) is reported once, not on every redraw.
func (in *Interp) StatusRows(width int) []StatusRow {
	defer in.enter()()
	in.statusSync()
	padStr := in.globals["status_pad"].String()
	pad := ' '
	if padStr != "" {
		pad, _ = utf8.DecodeRuneInString(padStr)
	}
	lineAttr := in.varAttrs("status_attr")
	ctx := in.attrCtx()
	var rows []StatusRow
	for r := 0; r < in.statusHeight(); r++ {
		row := StatusRow{Row: r, Attrs: lineAttr.String()}
		var line []rune
		for _, lf := range in.layoutRow(r, width) {
			if lf.col >= width {
				break
			}
			for len(line) < lf.col {
				line = append(line, pad)
			}
			val := fitRunes(in.fieldValue(lf.f), lf.width)
			cell := make([]rune, 0, lf.width)
			vs := 0
			if lf.f.RightJust {
				for i := 0; i < lf.width-len(val); i++ {
					cell = append(cell, pad)
				}
				vs = len(cell)
				cell = append(cell, val...)
			} else {
				cell = append(cell, val...)
				for len(cell) < lf.width {
					cell = append(cell, pad)
				}
			}
			a := lineAttr
			if fa, _, err := in.parseAttrs(lf.f.Attrs); err == nil {
				a = ctx.Adj(a, fa)
			}
			if av := lf.f.attrVar(); av != "" {
				a = ctx.Adj(a, in.varAttrs(av))
			}
			row.Spans = append(row.Spans, StatusSpan{Col: lf.col, Width: lf.width, Text: string(cell),
				ValueStart: vs, ValueLen: len(val), Attrs: a.String(), Field: lf.f})
			line = append(line, cell...)
		}
		for len(line) < width {
			line = append(line, pad)
		}
		row.Text = string(line)
		rows = append(rows, row)
	}
	return rows
}

func init() {
	builtinCmds["status_add"] = cmdStatusAdd
	builtinCmds["status_clear"] = cmdStatusClear
	builtinCmds["status_edit"] = cmdStatusEdit
	builtinCmds["status_rm"] = cmdStatusRm
	regFunc("status_fields", 0, 1, fnStatusFields)
	regFunc("status_width", 1, 1, fnStatusWidth)
}
