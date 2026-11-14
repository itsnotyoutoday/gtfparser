package gtfparser

import (
	"bufio"
	"os"
	"os/exec"
	"sort"
	"strings"
	"syscall"
	"time"
)

// Proc is a /repeat or /quote process. They run on the interpreter's own
// schedule: the front end calls RunDue (or Tick) whenever NextDue says so,
// from whatever goroutine owns the interpreter.
type Proc struct {
	PID     int
	Kind    byte // 'r' repeat, 'q' quote
	Next    time.Time
	Delay   time.Duration
	Count   int // repeats left; -1 = forever
	Cmd     string
	World   string
	OnPromp bool
	Disp    byte // quote: 'e' echo, 's' send, 'x' exec
	Lines   []string
	Pre     string
	Suf     string
}

// Now is the interpreter's clock (replaceable in tests).
func (in *Interp) now() time.Time {
	if in.Now != nil {
		return in.Now()
	}
	return time.Now()
}

// parseDelay reads "seconds", "h:m" or "h:m:s" (a dtime) into a duration.
func parseDelay(s string) time.Duration {
	return time.Duration(Str(s).Float() * float64(time.Second))
}

func (in *Interp) addProc(p *Proc) int {
	in.nextPID++
	p.PID = in.nextPID
	in.procs = append(in.procs, p)
	return p.PID
}

func cmdRepeat(in *Interp, args string) Value {
	p := &Proc{Kind: 'r', Delay: parseDelay(in.globals["ptime"].String())}
	sync, noDelay := false, false
	for {
		w, r := firstWord(args)
		if !strings.HasPrefix(w, "-") || len(w) < 2 {
			break
		}
		args = r
		switch {
		case strings.HasPrefix(w, "-w"):
			p.World = w[2:]
		case w == "-n":
			noDelay = true
		case w == "-S":
			sync = true
		case w == "-P":
			p.OnPromp = true
		default:
			p.Delay = parseDelay(w[1:])
		}
	}
	ptr := strings.TrimLeft(args, " \t")
	if ptr != "" && (ptr[0] == 'i' || ptr[0] == 'I') {
		if sync {
			in.warn("/repeat: -S i would cause an infinite busy loop.")
			return Int(0)
		}
		p.Count = -1
		ptr = ptr[1:]
	} else if ptr != "" && ptr[0] >= '0' && ptr[0] <= '9' {
		j := 0
		for j < len(ptr) && ptr[j] >= '0' && ptr[j] <= '9' {
			j++
		}
		n, ok := atoiBig(ptr[:j])
		if !ok {
			in.warn("/repeat: integer value too large")
			return Int(0)
		}
		if n <= 0 {
			in.warn("/repeat: invalid repeat count (%d)", n)
			return Int(0)
		}
		p.Count, ptr = n, ptr[j:]
	} else {
		in.warn("/repeat: invalid repeat count (%s)", firstN(ptr, 5))
		return Int(0)
	}
	if ptr != "" && ptr[0] != ' ' && ptr[0] != '\t' {
		in.warn("/repeat: repeat count followed by garbage (%s)", firstN(ptr, 5))
		return Int(0)
	}
	cmd := strings.TrimLeft(ptr, " \t")
	if cmd == "" {
		in.warn("/repeat: missing command")
		return Int(0)
	}
	p.Cmd = cmd
	if sync {
		res := Int(1)
		oldCmd := in.curCmd
		in.curCmd = ""
		for n := 0; p.Count < 0 || n < p.Count; n++ {
			res = in.ExecFull(cmd)
		}
		in.curCmd = oldCmd
		return res
	}
	p.Next = in.now().Add(p.Delay)
	if noDelay {
		p.Next = in.now()
	}
	return Int(int64(in.addProc(p)))
}

func cmdQuote(in *Interp, args string) Value {
	p := &Proc{Kind: 'q', Delay: parseDelay(in.globals["ptime"].String())}
	sync := false
	disp := byte(0)
	for {
		w, r := firstWord(args)
		if !strings.HasPrefix(w, "-") || len(w) < 2 {
			break
		}
		args = r
		switch {
		case strings.HasPrefix(w, "-d"):
			switch w[2:] {
			case "echo":
				disp = 'e'
			case "exec":
				disp = 'x'
			default:
				disp = 's'
			}
		case strings.HasPrefix(w, "-w"):
			p.World = w[2:]
		case w == "-S":
			sync = true
		case w == "-P":
			p.OnPromp = true
		case strings.HasPrefix(w, "-s"):
		default:
			p.Delay = parseDelay(w[1:])
		}
	}
	// [pre] then one of '"file" !"cmd" `"tfcmd" #"recall", then [suf]
	i := strings.IndexAny(args, "'!`#")
	for i > 0 && args[i-1] == '\\' {
		j := strings.IndexAny(args[i+1:], "'!`#")
		if j < 0 {
			i = -1
			break
		}
		i += 1 + j
	}
	if i < 0 {
		in.warn("/quote: missing command character")
		return Int(0)
	}
	p.Pre = strings.ReplaceAll(args[:i], "\\", "")
	src, body := args[i], strings.TrimSpace(args[i+1:])
	if disp == 0 {
		disp = 's'
		if strings.TrimSpace(p.Pre) != "" {
			disp = 'x'
		}
	}
	p.Disp = disp
	body = strings.Trim(body, "\"")
	var lines []string
	shellStatus := 0 // exit status of a !command source: what a synchronous /quote returns
	switch src {
	case '\'':
		if in.restriction >= RestrictFile {
			in.warn("/quote: files restricted")
			return Int(0)
		}
		if !in.allow(OpFileRead, "/quote", body) {
			return Int(0)
		}
		if f, ok := in.findFile(body); ok {
			if fh, err := os.Open(f); err == nil {
				sc := bufio.NewScanner(fh)
				for sc.Scan() {
					lines = append(lines, sc.Text())
				}
				fh.Close()
			}
		} else {
			in.warn("/quote: %s: %s", body, cErr(syscall.ENOENT))
			return Int(0)
		}
	case '!':
		if !in.allow(OpShell, "/quote", body) {
			return Int(0)
		}
		sh := exec.Command("sh", "-c", body)
		sh.Env = in.childEnv()
		out, err := sh.CombinedOutput()
		if ee, ok := err.(*exec.ExitError); ok {
			shellStatus = ee.ExitCode()
		}
		lines = strings.Split(strings.TrimRight(string(out), "\n"), "\n")
		if len(out) == 0 {
			lines = nil
		}
	case '`':
		old := in.noExp
		in.noExp = false
		lines = captureLines(in, body)
		in.noExp = old
	}
	p.Lines = lines
	if sync {
		for _, l := range lines {
			in.quoteLine(p, l)
		}
		if src == '!' {
			return Int(int64(shellStatus))
		}
		return Int(1)
	}
	p.Next = in.now().Add(p.Delay)
	return Int(int64(in.addProc(p)))
}

func captureLines(in *Interp, cmd string) []string {
	old := in.Host
	ch := &captureHost{Host: old}
	in.Host = ch
	in.ExecFull(cmd)
	in.Host = old
	return ch.lines
}

func (in *Interp) quoteLine(p *Proc, l string) {
	text := p.Pre + l + p.Suf
	switch p.Disp {
	case 'e':
		in.echo(text)
	case 'x':
		in.Exec(text) // a quoted line runs like a typed one: no macro substitution
	default:
		in.Host.Send(p.World, text)
	}
}

// RunDue runs every process whose time has come; it returns when none is due.
func (in *Interp) RunDue() {
	now := in.now()
	for {
		var due *Proc
		for _, p := range in.procs {
			if !p.OnPromp && !p.Next.After(now) && (due == nil || p.Next.Before(due.Next)) {
				due = p
			}
		}
		if due == nil {
			return
		}
		in.stepProc(due)
	}
}

func (in *Interp) stepProc(p *Proc) {
	old := in.curCmd
	in.curCmd = "" // processes run from the main loop, under no command
	defer func() { in.curCmd = old }()
	switch p.Kind {
	case 'r':
		in.ExecFull(p.Cmd)
		if p.Count > 0 {
			p.Count--
		}
		if p.Count == 0 {
			in.killProc(p.PID)
			return
		}
	case 'q':
		if len(p.Lines) == 0 {
			in.killProc(p.PID)
			return
		}
		l := p.Lines[0]
		p.Lines = p.Lines[1:]
		in.quoteLine(p, l)
		if len(p.Lines) == 0 {
			in.killProc(p.PID)
			return
		}
	}
	p.Next = p.Next.Add(p.Delay)
	if p.Delay <= 0 {
		p.Next = in.now().Add(time.Millisecond)
	}
}

// Prompt runs the processes waiting for a prompt (-P).
func (in *Interp) Prompt() {
	for _, p := range append([]*Proc(nil), in.procs...) {
		if p.OnPromp {
			in.stepProc(p)
		}
	}
}

// NextDue is when the earliest timed process wants to run.
func (in *Interp) NextDue() (time.Time, bool) {
	var best time.Time
	ok := false
	for _, p := range in.procs {
		if p.OnPromp {
			continue
		}
		if !ok || p.Next.Before(best) {
			best, ok = p.Next, true
		}
	}
	return best, ok
}

func (in *Interp) killProc(pid int) bool {
	for i, p := range in.procs {
		if p.PID == pid {
			in.procs = append(in.procs[:i], in.procs[i+1:]...)
			return true
		}
	}
	return false
}

func cmdKill(in *Interp, args string) Value {
	failed := 0
	for ptr := strings.TrimLeft(args, " \t"); ptr != ""; {
		var w string
		w, ptr = firstWord(ptr)
		n, ok := atoiBig(w)
		if w[0] < '0' || w[0] > '9' || !ok {
			if w[0] >= '0' && w[0] <= '9' {
				in.warn("/kill: integer value too large")
			} else {
				in.warn("/kill: invalid or missing numeric argument")
			}
			return Int(0)
		}
		if !in.killProc(n) {
			in.warn("/kill: no process %d", n)
			failed++
		}
	}
	return Bool(failed == 0)
}

func cmdPs(in *Interp, args string) Value {
	short := strings.Contains(args, "-s")
	ps := append([]*Proc(nil), in.procs...)
	sort.Slice(ps, func(i, j int) bool { return ps[i].PID < ps[j].PID })
	for _, p := range ps {
		if short {
			in.echo(Int(int64(p.PID)).String())
			continue
		}
		next := p.Next.Sub(in.now()).Seconds()
		in.echo(strings.TrimSpace(Str("% "+Int(int64(p.PID)).String()).String()) + " " + string(p.Kind) + " " + Real(next).String() + " " + p.Cmd)
	}
	return Int(int64(len(ps)))
}

func init() {
	builtinCmds["repeat"] = cmdRepeat
	builtinCmds["quote"] = cmdQuote
	builtinCmds["kill"] = cmdKill
	builtinCmds["ps"] = cmdPs
}

// atoiBig parses a run of digits as a C int: ok is false when it overflows.
func atoiBig(s string) (int, bool) {
	n := 0
	for i := 0; i < len(s) && s[i] >= '0' && s[i] <= '9'; i++ {
		n = n*10 + int(s[i]-'0')
		if n > 1<<31-1 {
			return 0, false
		}
	}
	return n, true
}

func firstN(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
