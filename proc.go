package gtfparser

import (
	"bufio"
	"os"
	"os/exec"
	"sort"
	"strings"
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
	cnt, cmd := firstWord(args)
	if cnt == "" || cmd == "" {
		in.warn("/repeat: missing count or command")
		return Int(0)
	}
	p.Count = int(Str(cnt).Int())
	if cnt == "i" {
		p.Count = -1
	}
	p.Cmd = cmd
	if sync {
		res := Int(1)
		for n := 0; p.Count < 0 || n < p.Count; n++ {
			res = in.ExecFull(cmd)
		}
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
		in.warn("/quote: no source given")
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
			in.warn("/quote: %s: no such file", body)
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
	ok := false
	for _, w := range strings.Fields(args) {
		if in.killProc(int(Str(w).Int())) {
			ok = true
		}
	}
	return Bool(ok)
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
