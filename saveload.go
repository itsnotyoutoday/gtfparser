package gtfparser

// /save (macro definitions to a file), /edit, /undefn, /liststreams.

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// /edit and /undefn (and /save, /def, /list, /purge) live in macro.go.

// cmdEdit changes an existing macro in place: options and body given on the
// command line replace the old ones, everything else (and the macro's number
// and position) is kept.
func cmdEdit(in *Interp, args string) Value {
	if strings.TrimSpace(args) == "" {
		in.warn("/edit: You must specify a macro.")
		return Int(0)
	}
	sp := in.parseSpec("EDIT", args, false)
	if sp == nil {
		return Int(0)
	}
	// TF takes everything between the options and the '=' as the name
	name := sp.m.Name
	if name == "" {
		in.warn("/edit: You must specify a macro.")
		return Int(0)
	}
	if strings.HasPrefix(name, "$") {
		return in.unsupported("/edit $<text> (selecting a macro by the text it triggers on)")
	}
	old, found := in.findMacro(name)
	if !found {
		in.warn("/edit: macro %s does not exist", name)
		return Int(0)
	}
	// the macro's own match style is the default for patterns given here
	style := sp.mflag
	if !sp.mGiven && old.Matching != "" {
		style = old.Matching
	}
	var err error
	nm := old.editCopy()
	if sp.trigStr != nil {
		if sp.m.Trig, err = in.compilePattern(style, *sp.trigStr); err != nil {
			in.cmdErr("EDIT", 't', "%v", err)
			return Int(0)
		}
	}
	if sp.hasHook && sp.hookHasPat {
		if sp.m.HArgs, err = in.compilePattern(style, sp.hookPat); err != nil {
			in.cmdErr("EDIT", 'h', "%v", err)
			return Int(0)
		}
	}
	if sp.wtStr != nil {
		if sp.m.WType, err = in.compilePattern(style, *sp.wtStr); err != nil {
			in.cmdErr("EDIT", 'T', "%v", err)
			return Int(0)
		}
	}
	if sp.hasBody {
		nm.Body = sp.m.Body
	}
	if sp.trigStr != nil {
		nm.Trig = sp.m.Trig
	}
	if sp.noHook {
		nm.IsHook, nm.Hooks, nm.HArgs = false, 0, nil
	} else if sp.hasHook {
		nm.IsHook, nm.Hooks, nm.HArgs = true, sp.hookMask, sp.m.HArgs
	}
	if sp.bindSet {
		nm.Bind = sp.m.Bind
	}
	if sp.keySet {
		nm.Keyname = sp.m.Keyname
	}
	if sp.exprSet {
		nm.Cond = sp.m.Cond
	}
	if sp.wtStr != nil {
		nm.WType = sp.m.WType
	}
	switch sp.world {
	case 1:
		nm.World = in.ctxWorld()
	case 2:
		nm.World = ""
	case 3:
		nm.World = sp.worldName
	}
	if sp.attrSet {
		nm.Attr = sp.m.Attr
	}
	if sp.m.CheckMatchAttr {
		nm.MatchAttr, nm.CheckMatchAttr = sp.m.MatchAttr, true
	}
	if len(sp.m.SubAttr) > 0 {
		nm.SubAttr = sp.m.SubAttr
	}
	if sp.mGiven && (sp.trigStr != nil || sp.hasHook) {
		nm.Matching = style
	}
	if sp.shots >= 0 {
		nm.Shots = sp.shots
	}
	if sp.pri >= 0 {
		nm.Priority = sp.pri
	}
	if sp.prob >= 0 {
		nm.Prob = sp.prob
	}
	if sp.fall > 0 {
		nm.Fall = true
	}
	if sp.quiet > 0 {
		nm.Quiet = true
	}
	if sp.invis != 0 {
		nm.Invisible = true
	}
	return Int(int64(in.completeMacro(nm, old.ID)))
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
		if st.mode == 'w' || st.mode == 'a' {
			flush = "off"
			if st.autoflush {
				flush = "on"
			}
		}
		mode := st.mode
		if st.f != nil && st.canWrite {
			mode = streamModeChar(st.f, true)
		}
		in.echo(fmt.Sprintf("%6d   %c   %3s  %s", n, mode, flush, st.name))
	}
	return Int(int64(len(ids)))
}

func init() {
	builtinCmds["edit"] = cmdEdit
	builtinCmds["liststreams"] = cmdListstreams
}
