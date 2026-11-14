package gtfparser

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

// Native library commands.
//
// tf's stdlib.tf defines a few dozen commands as invisible macros (/prompt,
// /first, /listgag, /lp ...). They are plain language and state operations, so
// the engine implements them itself: every client then has them, whether or
// not it loads stdlib.tf, and a script no longer has to be parsed to run them.
//
// Each one is registered as an ordinary command (so a user's /def shadows it)
// AND bound to the library's macro when stdlib.tf defines it: when a macro
// named N is installed with exactly the definition stdlib.tf gives N
// (compared by fingerprint, see nativeFP), the macro keeps its number, name,
// flags and printed body, so /list, /undef, /edit and /ismacro see exactly what
// they see in real tf, but running it runs the Go implementation. A macro with
// the same name and a different body is an ordinary script macro.
//
// Natives execute in the same kind of scope a script macro does (positional
// parameters, locals, %?, getopts), and report warnings under the macro's name,
// so the output is the same as the script's.

// nativeLib maps each library command to its implementation.
var nativeLib = map[string]cmdFunc{}

// nativeNames lists them in registration order (for documentation and tests).
var nativeNames []string

func regNative(name string, fn cmdFunc) {
	nativeLib[name] = fn
	nativeNames = append(nativeNames, name)
	builtinCmds[name] = func(in *Interp, args string) Value {
		// not a library macro here (stdlib.tf not loaded, or the user removed
		// it): run the implementation in a macro-like scope
		return in.callMacro(&Macro{Name: name, native: fn}, strings.TrimRight(args, " \t"), false)
	}
}

// NativeLibraryCommands returns the names of the commands tf's stdlib.tf
// defines as macros that this package implements itself.
func NativeLibraryCommands() []string { return append([]string(nil), nativeNames...) }

// WithoutNativeLibrary keeps library macros as script: with it, a macro
// defined by stdlib.tf runs its own body (the commands stay available as
// builtins when no macro of that name exists). Used for A/B testing.
func WithoutNativeLibrary() Option { return func(in *Interp) { in.noNative = true } }

// IsNative reports whether the macro runs a Go implementation.
func (m *Macro) IsNative() bool { return m.native != nil }

// nativeFP is the fingerprint of a macro definition: a hash of its printed
// definition with white space collapsed.
func nativeFP(m *Macro) string {
	s := strings.Join(strings.Fields(strings.ReplaceAll(m.printDef(false), "\t", " ")), " ")
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:8])
}

// bindNative attaches the Go implementation to a freshly installed macro that
// is exactly stdlib.tf's definition of a library command.
func (in *Interp) bindNative(m *Macro) {
	if in.noNative || m.Name == "" || !m.Invisible {
		return
	}
	fn := nativeLib[m.Name]
	if fn == nil || nativeFPs[m.Name] != nativeFP(m) {
		return
	}
	m.native = fn
	if in.nativeSeen == nil {
		in.nativeSeen = map[string]bool{}
	}
	in.nativeSeen[m.Name] = true
}

// nativeHidden: stdlib.tf defined this library command as a macro, so (as in
// real tf, where it is not a builtin) it exists only while that macro does.
func (in *Interp) nativeHidden(name string) bool {
	return in.nativeSeen != nil && in.nativeSeen[name]
}

// ---------------------------------------------------------------- helpers

// ret makes v the value of the running command, as /return and /result do.
func (in *Interp) ret(v Value) Value { return in.leaf(v) }

// stmts runs macro statements in the current scope, like a macro body.
func (in *Interp) stmts(s ...string) Value { return in.execStmts(s) }

// cond evaluates an /if or /while condition expression.
func (in *Interp) cond(e string) bool { return in.condition(e) }

// getopts is the getopts() call every option-taking library macro starts with.
func (in *Interp) getopts(spec string, init ...Value) bool {
	a := append([]Value{Str(spec)}, init...)
	v, err := in.callFuncTf("getopts", a, true)
	if err != nil {
		in.warn("%v", err)
		return false
	}
	return v.Truth()
}

// callFn calls a builtin function by name as an expression would.
func (in *Interp) callFn(name string, a ...Value) Value {
	v, err := in.callFuncTf(name, a, true)
	if err != nil {
		in.warn("%v", err)
		return Int(0)
	}
	return v
}

func (in *Interp) local(name string) Value { v, _ := in.lookup(name); return v }

func bint(b bool) Value {
	if b {
		return Int(1)
	}
	return Int(0)
}

// setCmd runs "/set name args" the way the macro "/set name %*" does: the
// value part exists even when args is empty.
func (in *Interp) setCmd(name, args string) Value {
	return in.dispatchLine("/set "+name+" "+args, func() bool { return true })
}

func (in *Interp) plain(line string) Value {
	return in.dispatchLine(line, func() bool { return false })
}

// words returns the positional parameters.
func (in *Interp) words() []string { return in.fr.words }

// ---------------------------------------------------------------- commands

func init() {
	for _, n := range []string{"nolog", "nowrap", "nologin", "noquiet"} {
		n := n
		regNative(n, func(in *Interp, _ string) Value {
			if n == "nolog" {
				return in.plain("/log off")
			}
			return in.setCmd(n[2:], "off")
		})
	}
	// /<name> %*  ->  /set <name> %*
	for _, p := range [][2]string{{"reply", "borg"}, {"background", "background"}, {"bamf", "bamf"}, {"borg", "borg"},
		{"clearfull", "clearfull"}, {"cleardone", "cleardone"}, {"insert", "insert"}, {"login", "login"}, {"lp", "lp"},
		{"quiet", "quiet"}, {"quitdone", "quitdone"}, {"redef", "redef"}, {"sockmload", "sockmload"}, {"sub", "sub"},
		{"isize", "isize"}, {"ptime", "ptime"}, {"wrapspace", "wrapspace"}} {
		v := p[1]
		regNative(p[0], func(in *Interp, args string) Value { return in.setCmd(v, args) })
	}
	regNative("echo", nEcho)
	regNative("_echo", func(in *Interp, args string) Value { return in.ret(in.echoOpts(EchoOpts{}, args)) })
	regNative("true", func(in *Interp, _ string) Value { return in.ret(Int(1)) })
	regNative("false", func(in *Interp, _ string) Value { return in.ret(Int(0)) })
	regNative(":", func(in *Interp, _ string) Value { return in.ret(Int(1)) })
	regNative("act", func(in *Interp, args string) Value { return in.plain("/trig " + args) })
	regNative("more", nMore)
	regNative("wrap", nWrap)

	regNative("prompt", nPrompt)
	regNative("substitute", nSubstitute)
	regNative("loaded", nLoaded)
	regNative("require", nRequire)

	regNative("not", func(in *Interp, args string) Value {
		in.dispatchLine("/@eval "+args, func() bool { return false })
		return in.stmts("/@test !%?")
	})
	regNative("expr", func(in *Interp, args string) Value { return in.plain("/result " + args) })
	regNative("toggle", func(in *Interp, _ string) Value { return in.stmts("/@test %1 := !%1") })
	regNative("for", func(in *Interp, _ string) Value {
		return in.stmts("/@eval /let %1=%2%%; /while ( %1 <= %3 ) %-3%%; /@test ++%1%%; /done")
	})

	regNative("first", func(in *Interp, _ string) Value { return in.plain("/result {1}") })
	regNative("rest", func(in *Interp, _ string) Value { return in.plain("/result {-1}") })
	regNative("last", func(in *Interp, _ string) Value { return in.plain("/result {L}") })
	regNative("nth", func(in *Interp, _ string) Value { return in.plain(`/result {1} > 0 ? shift({1}), {1} : ""`) })
	regNative("replace", func(in *Interp, _ string) Value { return in.plain("/result replace({1}, {2}, {-2})") })
	regNative("split", func(in *Interp, _ string) Value {
		return in.plain(`/@test regmatch("^([^=]*[^ =])? *=? *(.*)", {*})`)
	})
	regNative("escape", nEscape)

	regNative("time", func(in *Interp, _ string) Value { return in.stmts(`/@test echo(ftime({*-%%{time_format}})), time()`) })
	regNative("rand", nRand)
	regNative("ver", func(in *Interp, _ string) Value {
		return in.stmts(`/result regmatch('version (.*). %% Copyright', $$(/version)), {P1}`)
	})
	regNative("vercmp", nVercmp)
	regNative("runtime", func(in *Interp, _ string) Value {
		return in.stmts(`/let real=`, `/let cpu=`, `/test real:=time(), cpu:=cputime()`, `/eval -s0 %{*}`,
			`/let result=%?`, `/_echo real=$[time() - real] cpu=$[cputime() - cpu]`, `/return result`)
	})

	regNative("ismacro", func(in *Interp, _ string) Value {
		return in.stmts(`/test tfclose("o")`, `/@list -s -i %{*-@}`)
	})
	regNative("isvar", func(in *Interp, _ string) Value {
		return in.stmts(`/test tfclose("o")`, `/listvar -msimple -- %*`)
	})

	regNative("cat", nCat)
	regNative("paste", nPaste)
	regNative("_paste", nPasteLine)

	// list and purge families
	for _, p := range [][2]string{
		{"listfullhilite", "/list -mglob -h0 -b{} -t'%s' -aurfdhbBC0"},
		{"listpartial", "/list -mglob -h0 -t'%s' -P"},
		{"listgag", "/list -mglob -h0 -b{} -t'%s' -ag"},
		{"listtrig", "/list -mglob -h0 -b{} -t'%s' -an"},
		{"listbind", "/list -mglob -h0 -b'%s'"},
		{"listhook", "/list -mglob -h'%s'"},
		{"purgehilite", "/purge -mglob -h0 -b{} -t'%s' -aurfdhbBC0"},
		{"purgegag", "/purge -mglob -h0 -b{} -t'%s' -ag"},
		{"purgetrig", "/purge -mglob -h0 -b{} -t'%s' -an"},
		{"purgedeft", "/purge -mglob -h0 -b{} -t'%s' ?*"},
		{"purgebind", "/purge -mglob -h0 -b'%s'"},
		{"purgehook", "/purge -mglob -h'%s'"},
	} {
		tmpl := p[1]
		regNative(p[0], func(in *Interp, args string) Value {
			return in.plain(strings.Replace(tmpl, "%s", in.escapeLib("'", args), 1))
		})
	}
	regNative("listdef", func(in *Interp, args string) Value { return in.plain("/list " + args) })
	regNative("listhilite", func(in *Interp, _ string) Value { return in.stmts("/listfullhilite", "/listpartial") })
	regNative("purgedef", func(in *Interp, _ string) Value { return in.plain("/purge -mglob -h0 -b{} - " + in.argOr("?*")) })
	regNative("untrig", nUntrig)
	regNative("unhook", nUnhook)
	regNative("undeft", func(in *Interp, args string) Value { return in.plain("/untrig -anGgurfdhbBC0 - " + args) })
	regNative("savehilite", func(in *Interp, _ string) Value {
		return in.stmts(`/save %{1-${HILITEFILE}} -mglob -h0 -b{} -t -aurfdhbBC0`, `/save -a %{1-${HILITEFILE}} -mglob -h0 -t -P`)
	})
}

// argOr is %{1-def}: the first parameter, or def when there is none.
func (in *Interp) argOr(def string) string {
	if w := in.words(); len(w) > 0 {
		return w[0]
	}
	return def
}

// escapeLib is "$(/escape <meta> <text>)": the library's /escape.
func (in *Interp) escapeLib(meta, text string) string {
	return escapeMeta(meta+`\`, text)
}

// escapeMeta puts a backslash before every character of text that is in meta.
func escapeMeta(meta, text string) string {
	var b strings.Builder
	for _, r := range text {
		if strings.ContainsRune(meta, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// /escape <metachars> <string>
func nEscape(in *Interp, args string) Value {
	f := in.fr
	meta := ""
	if len(f.words) > 0 {
		meta = f.words[0]
	}
	tail := ""
	if len(f.words) > 1 {
		tail = f.text[f.offs[1]:]
	}
	v := Str(escapeMeta(meta+`\`, tail))
	in.retv, in.ctl = v, ctlReturn
	in.userRes = v
	if !f.asFunc {
		in.echo(v.String())
	}
	return v
}

// /echo [-a<attr>] [-p] [-oeAr] [-w[<world>]] <text>
func nEcho(in *Interp, _ string) Value {
	v := in.fr.vars
	v["opt_a"], v["opt_w"] = Str(""), Str("()")
	for _, o := range []string{"opt_p", "opt_o", "opt_e", "opt_r", "opt_A"} {
		v[o] = Int(0)
	}
	if !in.getopts("a:poerAw:") {
		return in.ret(Int(0))
	}
	dest := "o"
	switch {
	case v["opt_w"].String() != "()":
		dest = "w" + v["opt_w"].String()
	case v["opt_e"].Truth():
		dest = "e"
	case v["opt_A"].Truth():
		dest = "a"
	case v["opt_r"].Truth():
		dest = "r"
	}
	// a client that shows attributed text or addresses worlds (EchoHost) gets
	// the options instead of a flattened line
	o := EchoOpts{Attrs: v["opt_a"].String(), Markup: v["opt_p"].Truth()}
	if dest[0] == 'w' {
		o.World = dest[1:]
	}
	if _, ok := in.Host.(EchoHost); ok && o != (EchoOpts{}) && (dest[0] == 'w' || dest[0] == 'o') {
		if o.World != "" && !in.worldKnown("ECHO", o.World) {
			in.warn("No world %s", o.World)
			return in.ret(Int(0))
		}
		return in.ret(in.echoOpts(o, in.fr.text))
	}
	return in.ret(in.callFn("echo", Str(in.fr.text), v["opt_a"], bint(v["opt_p"].Truth()), Str(dest)))
}

// /prompt [-a<attr>] [-p] <text>
func nPrompt(in *Interp, _ string) Value {
	in.fr.vars["opt_a"] = Str("")
	in.fr.vars["opt_p"] = Int(0)
	if !in.getopts("a:p") {
		return in.ret(Int(0))
	}
	d := in.callFn("decode_attr", Str(in.fr.text), in.local("opt_a"), bint(in.local("opt_p").Truth()))
	return in.ret(in.callFn("prompt", d))
}

// /substitute [-a<attr>] [-p] <text>
func nSubstitute(in *Interp, _ string) Value {
	if !in.getopts("a:p", Str("")) {
		return in.ret(Int(0))
	}
	return in.ret(in.callFn("substitute", Str(in.fr.text), in.local("opt_a"), bint(in.local("opt_p").Truth())))
}

func nLoaded(in *Interp, _ string) Value {
	name := in.expand("%{1}")
	v, err := in.evalExpr(`_loaded_libs !/ "*{` + name + `}*"`)
	if err != nil {
		in.warn("%v", err)
		v = Int(0)
	}
	in.leaf(v)
	if v.Truth() {
		in.dispatchLine("/set _loaded_libs="+in.expand("%{_loaded_libs} %{1}"), func() bool { return true })
		return in.stmts("/let _required=0")
	}
	if in.cond("(_required)") {
		return in.plain("/exit")
	}
	return v
}

func nRequire(in *Interp, _ string) Value {
	return in.stmts("/let _required=1", "/load %{-L} %{L}")
}

func nMore(in *Interp, args string) Value {
	if in.cond(`( {*} =/ "{on|1}" & ismacro("-ib'^I' = /key_tab") & ismacro("-i key_tab = /dokey page") )`) {
		in.stmts(`/echo -e %% "More" paging enabled.  Use TAB to scroll.`)
	}
	return in.setCmd("more", args)
}

func nWrap(in *Interp, args string) Value {
	if in.cond(`({*} =/ '[0-9]*')`) {
		in.dispatchLine("/set wrapsize="+args, func() bool { return true })
		return in.plain("/set wrap=1")
	}
	return in.setCmd("wrap", args)
}

func nRand(in *Interp, _ string) Value {
	switch len(in.words()) {
	case 0:
		return in.stmts("/echo $[rand()]")
	case 1:
		return in.stmts("/echo $[rand({1})]")
	case 2:
		return in.stmts("/echo $[rand({1}, {2})]")
	}
	return in.stmts("/echo -e %% %0: too many arguments")
}

func nVercmp(in *Interp, _ string) Value {
	in.stmts(`/let pat=^([0-9]+)\\.([0-9]+) (alpha|beta|gamma|stable) ([0-9]*)$$`)
	if in.cond("(!regmatch(pat, {1}))") {
		in.stmts(`/echo -e %% %0: Bad version format "%1"`)
		return in.plain("/return -2")
	}
	in.stmts("/let maj1=%P1", "/let min1=%P2", "/let lev1=%P3", "/let rev1=%P4")
	if in.cond("(!regmatch(pat, {2}))") {
		in.stmts(`/echo -e %% %0: Bad version format "%2"`)
		return in.plain("/return -2")
	}
	in.stmts("/let maj2=%P1", "/let min2=%P2", "/let lev2=%P3", "/let rev2=%P4")
	return in.plain("/return (maj1-maj2 ?: min1-min2 ?: strcmp(lev1,lev2) ?: rev1-rev2)")
}

func nUntrig(in *Interp, _ string) Value {
	if !in.getopts("a:", Str("")) {
		return in.ret(Int(0))
	}
	r := in.execOne(`/!purge -i -msimple -a%opt_a -t"$(/escape " %*)"`)
	if r.Truth() {
		in.stmts(`/echo -e %% No trigger on %*.`)
		return in.plain("/return 0")
	}
	return r
}

func nUnhook(in *Interp, _ string) Value {
	r := in.execOne(`/!purge -i -msimple -h"$(/escape " %*)"`)
	if r.Truth() {
		in.stmts(`/echo -e %% No hook on %*.`)
		return in.plain("/return 0")
	}
	return r
}

// /cat: send the following lines, up to ".", as one command.
func nCat(in *Interp, _ string) Value {
	in.stmts(`/echo -e %% Entering cat mode.  Type "." to end.`, `/let _line=`, `/let _all=`)
	for in.ctl == ctlNone && in.tick() && in.cond(`((tfread(_line) >= 0) & (_line !~ "."))`) {
		if in.cond(`(_line =/ "/quit")`) {
			in.stmts(`/echo -e %% Type "." to end /cat.`)
		}
		in.stmts(`/@test _all := strcat(_all, (({1} =~ "%%" & _all !~ "") ? "%%;" : ""), _line)`)
	}
	if in.ctl != ctlNone {
		return Int(0)
	}
	in.stmts(`/recordline -i %_all`)
	return in.stmts(`/@test eval(_all)`)
}

func nPasteLine(in *Interp, _ string) Value {
	if in.cond("(opt_x)") {
		return in.stmts(`/test eval({*}, 0)`)
	}
	return in.stmts(`/test send({*}, _world, opt_h ? "h" : "")`)
}

// /paste [-w<world>] [-spxtqnh] [-e<end>] [-a<abort>] [prefix]
func nPaste(in *Interp, _ string) Value {
	if !in.getopts("spnxtqhw:e:a:", Str("")) {
		return in.plain("/return 0")
	}
	if in.cond("(opt_p & opt_t)") {
		in.stmts(`/echo -e %% %0: Options -p and -t are mutually exclusive.`)
		return in.plain("/return 0")
	}
	if in.cond(`(opt_x & opt_w !~ "")`) {
		in.stmts(`/echo -e %% %0: Options -x and -w are mutually exclusive.`)
		return in.plain("/return 0")
	}
	in.stmts(`/let _prefix=$[opt_n ? "" : {*-%{paste_prefix-:|}}]`)
	if in.cond("(!opt_n)") {
		in.stmts("/shift")
	}
	in.stmts(`/let _end=%{opt_e-/endpaste}`, `/let _abort=%{opt_a-/abort}`, `/let _line=`, `/let _text=`,
		`/let _world=%{opt_w-${world_name}}`, `/let _oldlen=0`, `/let _lead=0`, `/let _read=0`)
	if in.cond("(!opt_q)") {
		in.stmts(`/echo -ep %% Entering paste mode.  Type "@{B}%{_end}@{n}" or "@{B}.@{n}" to end, or "@{B}%{_abort}@{n}" to abort.`)
	}
	for in.ctl == ctlNone && in.tick() {
		if in.cond(`((_read := tfread(_line)) < 0 | _line =/ _abort)`) {
			return in.plain("/return 0")
		}
		if in.cond(`(_line =/ _end | _line =/ ".")`) {
			break
		}
		if in.cond(`(_line =/ "/quit" | _line =/ "/help*")`) {
			in.stmts(`/echo -ep %% Type "@{B}%{_end}@{n}" or "@{B}.@{n}" to end /paste, or @{B}%{_abort}@{n} to abort.`)
		}
		switch {
		case in.cond("(opt_t)"):
			in.stmts(`/test regmatch("^ +", _line), _lead := strlen({P0})`)
			switch {
			case in.cond("(!_oldlen)"):
				in.stmts(`/test _text := _line`)
			case in.cond("(_oldlen <= _lead)"):
				in.stmts(`/test _text := strcat(_text, substr(_line, _oldlen))`)
			default:
				in.stmts(`/_paste %{_prefix} %{_text}`, `/test _text := _line`)
			}
			in.stmts(`/test _oldlen := strlen(_text)`)
		case in.cond("(!opt_p)"):
			if in.cond("(!opt_s)") {
				in.stmts(`/test regmatch(" *$$", _line)`, `/let _line=%PL`)
			}
			in.stmts(`/test _paste(_prefix=~"" ? _line : strcat(_prefix, " ", _line))`)
		case in.cond(`(regmatch("^ *$", _line))`):
			if in.cond(`(_text !~ "")`) {
				in.stmts(`/_paste %{_prefix}%{_text}`, `/_paste %{_prefix}`, `/let _text=`)
			}
		default:
			in.stmts(`/let _text=%{_text} $(/echo - %{_line})`)
		}
	}
	if in.ctl != ctlNone {
		return Int(0)
	}
	if in.cond(`((opt_p | opt_t) & _text !~ "")`) {
		in.stmts(`/_paste %{_prefix}%{_text}`)
	}
	return in.plain("/return 1")
}

var _ = strconv.Itoa
