package gtfparser

// /save (macro definitions to a file), /edit, /undefn, /liststreams.

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

func cmdSave(in *Interp, args string) Value {
	if in.Restriction() >= RestrictFile {
		in.warn("/save: restricted")
		return Int(0)
	}
	opts, rest, ok := in.scanOpts("/save", "a", args)
	if !ok {
		return Int(0)
	}
	rest = strings.TrimLeft(rest, " \t")
	if rest == "" {
		in.warn("/save: missing filename")
		return Int(0)
	}
	name, spec := firstWord(rest)
	m, err := in.parseDef(spec)
	if err != nil {
		in.warn("/save: %v", err)
		return Int(0)
	}
	path := in.expandTilde(name)
	if !in.allow(OpFileWrite, "/save", path) {
		return Int(0)
	}
	mode, verb := os.O_TRUNC, "Writ"
	if len(opts) > 0 {
		mode, verb = os.O_APPEND, "Append"
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|mode, 0o644)
	if err != nil {
		in.warn("/save: %s: %s", name, errText(err))
		return Int(0)
	}
	defer f.Close()
	in.echo(fmt.Sprintf("%% %sing macros to %s", verb, path))
	last := 0
	for _, mac := range in.Macros() {
		if !in.listMatch(m, mac, m.Matching) {
			continue
		}
		last = mac.ID
		fmt.Fprintln(f, mac.printDef(false))
	}
	return Int(int64(last))
}

func cmdUndefn(in *Interp, args string) Value {
	res := 0
	rest := strings.TrimLeft(args, " \t")
	for rest != "" {
		n := in.numArg("/undefn", &rest)
		if n < 0 {
			continue
		}
		var found *Macro
		for _, m := range in.Macros() {
			if m.ID == n {
				found = m
				break
			}
		}
		if found == nil {
			in.warn("/undefn: no macro with number %d", n)
			continue
		}
		if found.Name != "" {
			in.Undef(found.Name)
		} else {
			in.removeFromList(found)
		}
		res++
	}
	return Int(int64(res))
}

// editName extracts the macro name from /edit's arguments: the text after the
// options and before the '='.
func editName(args string) string {
	rest := strings.TrimLeft(args, " \t")
	for strings.HasPrefix(rest, "-") && len(rest) > 1 {
		var w string
		w, rest = splitOptWord(rest)
		if w == "--" {
			break
		}
	}
	if i := strings.IndexByte(rest, '='); i >= 0 {
		rest = rest[:i]
	}
	return strings.TrimSpace(rest)
}

// cmdEdit changes an existing macro in place: options and body given on the
// command line replace the old ones, everything else (and the macro's number
// and position) is kept.
func cmdEdit(in *Interp, args string) Value {
	if strings.TrimSpace(args) == "" {
		in.warn("/edit: You must specify a macro.")
		return Int(0)
	}
	spec, err := in.parseDef(args)
	if err != nil {
		in.warn("/edit: %v", strings.TrimPrefix(err.Error(), "/def: "))
		return Int(0)
	}
	// TF takes everything between the options and the '=' as the name
	spec.Name = editName(args)
	if spec.Name == "" {
		in.warn("/edit: You must specify a macro.")
		return Int(0)
	}
	if w, ok := spec.set['w']; ok && w != "" {
		if _, found := in.World(w); !found {
			in.warn("/edit -w: No world %s", w)
			return Int(0)
		}
	}
	if strings.HasPrefix(spec.Name, "$") {
		return in.unsupported("/edit $<text> (selecting a macro by the text it triggers on)")
	}
	old, ok := in.macros[spec.Name]
	if !ok {
		in.warn("/edit: macro %s does not exist", spec.Name)
		return Int(0)
	}
	nm := *old
	set := spec.set
	if spec.hasBody {
		nm.Body, nm.hasBody = spec.Body, true
	}
	// -m only means something together with the pattern it applies to
	_, hasT := set['t']
	_, hasH := set['h']
	if _, ok := set['m']; ok && (hasT || hasH) {
		nm.Matching = spec.Matching
	}
	if hasT {
		nm.Trigger = spec.Trigger
	}
	if hasH {
		nm.Hook = spec.Hook
	}
	if _, ok := set['b']; ok {
		nm.Bind = spec.Bind
	}
	if _, ok := set['E']; ok {
		nm.Cond = spec.Cond
	}
	if _, ok := set['T']; ok {
		nm.WorldType = spec.WorldType
	}
	if _, ok := set['w']; ok {
		nm.World = spec.World
	}
	if _, ok := set['a']; ok {
		nm.Attrs = spec.Attrs
	}
	if _, ok := set['n']; ok {
		nm.Shots = spec.Shots
	}
	if _, ok := set['p']; ok {
		nm.Priority = spec.Priority
	}
	if _, ok := set['F']; ok {
		nm.Fall = true
	}
	if _, ok := set['q']; ok {
		nm.Quiet = true
	}
	if _, ok := set['i']; ok {
		nm.Invisible = true
	}
	nm.ID, nm.Name = old.ID, old.Name
	if nm.Matching == "regexp" && nm.Trigger != "" {
		re, err := compileRegexp(nm.Trigger)
		if err != nil {
			in.warn("/edit: %v", err)
			return Int(0)
		}
		nm.re = re
	} else {
		nm.re = nil
	}
	*old = nm
	return Int(int64(old.ID))
}

// ---------------------------------------------------------------- /liststreams

func cmdListstreams(in *Interp, args string) Value {
	if len(in.streams) == 0 {
		in.echo("% No open streams.")
		return Int(0)
	}
	ids := make([]int, 0, len(in.streams))
	for id := range in.streams {
		n, _ := strconv.Atoi(id)
		ids = append(ids, n)
	}
	sort.Ints(ids)
	in.echo("HANDLE MODE FLUSH NAME")
	for _, n := range ids {
		st := in.streams[strconv.Itoa(n)]
		flush := ""
		if st.w {
			flush = "on" // writes are never buffered
		}
		in.echo(fmt.Sprintf("%6d   %c   %3s  %s", n, streamModeChar(st.f, st.w), flush, st.f.Name()))
	}
	return Int(int64(len(ids)))
}

func init() {
	builtinCmds["save"] = cmdSave
	builtinCmds["undefn"] = cmdUndefn
	builtinCmds["edit"] = cmdEdit
	builtinCmds["liststreams"] = cmdListstreams
}
