package gtfparser

import (
	"os"
	"strings"
	"time"
)

// TTYHost is implemented by hosts that run on a terminal; isatty() reports it.
type TTYHost interface{ IsTTY() bool }

func fnIsatty(in *Interp, a []Value) (Value, error) {
	if h, ok := in.Host.(TTYHost); ok {
		return Bool(h.IsTTY()), nil
	}
	return Int(0), nil
}

// nread() is the number of nested keyboard reads in progress. The core never
// blocks for the keyboard, so it is the depth a host reports with SetReadDepth.
func fnNread(in *Interp, a []Value) (Value, error) { return Int(int64(in.readDepth)), nil }

// SetReadDepth lets a host that implements keyboard reads (tfread("i") from a
// command line command) report how many are nested, for nread().
func (in *Interp) SetReadDepth(n int) { in.readDepth = n }

// read() is the deprecated way to read a line of input; it reads the input
// stream like tfread() does and returns "" at end of input.
func fnRead(in *Interp, a []Value) (Value, error) {
	in.warn("Warning: read() is deprecated.  Use tfread() instead.")
	in.warn("keyboard can only be read from a command line command.")
	return Str(""), nil
}

type mailInfo struct {
	mtime, size int64
	flag        int
	known       bool
}

// mailFiles is the list check_mail() watches: %TFMAILPATH (blank separated,
// backslash escapes a blank) or else %MAIL.
func (in *Interp) mailFiles() []string {
	p := in.Var("TFMAILPATH")
	if p == "" {
		if m := in.Var("MAIL"); m != "" {
			return []string{m}
		}
		return nil
	}
	var out []string
	var cur strings.Builder
	for i := 0; i < len(p); i++ {
		switch {
		case p[i] == '\\' && i+1 < len(p):
			i++
			cur.WriteByte(p[i])
		case p[i] == ' ' || p[i] == '\t':
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteByte(p[i])
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// fnNmail counts the mail files holding unread mail, by the rules of
// check_mail() in util.c (a file is unread when its mtime is later than its
// atime).
func fnNmail(in *Interp, a []Value) (Value, error) {
	if d, ok := in.globals["maildelay"]; ok {
		if sec, _, ok := valTime(d); ok && sec <= 0 {
			return Int(0), nil
		}
	}
	if in.mail == nil {
		in.mail = map[string]*mailInfo{}
	}
	n := 0
	for _, name := range in.mailFiles() {
		info := in.mail[name]
		if info == nil {
			info = &mailInfo{mtime: -2, size: -2}
			in.mail[name] = info
		}
		fi, err := os.Stat(in.expandFilename(name))
		if err != nil || fi.IsDir() {
			info.mtime, info.size, info.flag = -1, -1, 0
			continue
		}
		mt, at := fi.ModTime().Unix(), time.Time{}.Unix()
		if a, ok := statAtime(fi); ok {
			at = a
		}
		switch {
		case fi.Size() == 0:
			info.flag = 0
		case info.size == -2:
			info.flag = 0 // first look: no way to tell if it was read
		case mt > at:
			info.flag = 1
			n++
		case info.size < 0 && mt == at:
			info.flag = 1
			n++
		case mt > info.mtime || mt < at:
			info.flag = 0
		default:
			n += info.flag
		}
		info.mtime, info.size = mt, fi.Size()
	}
	return Int(int64(n)), nil
}

func init() {
	f := builtinFuncs
	f["isatty"] = fnIsatty
	f["nread"] = fnNread
	f["nmail"] = fnNmail
	f["read"] = fnRead
}
