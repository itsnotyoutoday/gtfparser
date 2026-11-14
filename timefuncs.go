package gtfparser

import (
	"fmt"
	"math"
	"math/big"
	"os"
	osuser "os/user"
	"strings"
	"time"
)

// tzLoc is the zone local times are shown in: TZ as the script sees it
// (tf calls localtime(3), which follows its own environment), else the
// process zone.
func (in *Interp) tzLoc() *time.Location {
	if v, ok := in.globals["TZ"]; ok {
		if s := strings.TrimPrefix(v.String(), ":"); s != "" {
			if s == "UTC" || s == "GMT" {
				return time.UTC
			}
			if l, err := time.LoadLocation(s); err == nil {
				return l
			}
		}
	}
	return time.Local
}

// valTime is valtime() in expr.c: seconds and microseconds of a number.
func valTime(v Value) (sec, usec int64, ok bool) {
	n := v.Num()
	switch n.k {
	case kInt:
		return n.i, 0, true
	case kDec:
		q, r := new(big.Int).QuoRem(n.dec, million, new(big.Int))
		return q.Int64(), r.Int64(), true
	}
	if math.IsNaN(n.f) || math.Abs(n.f) >= 1<<62 {
		return 0, 0, false
	}
	i := int64(n.f)
	return i, int64((n.f - float64(i)) * 1000000), true
}

// tftime formats a time like tftime() in util.c: a nil format (or "@") is
// seconds.micro, otherwise strftime with tf's %@ %. %s %F %T extensions.
func (in *Interp) tftime(format *string, sec, usec int64) string {
	if format == nil || *format == "@" {
		var b strings.Builder
		if sec < 0 || usec < 0 {
			usec = -usec
			if sec == 0 {
				b.WriteByte('-')
			}
		}
		fmt.Fprintf(&b, "%d.%06d", sec, usec)
		s := b.String()
		if format == nil {
			s = strings.TrimRight(s, "0")
			if strings.HasSuffix(s, ".") {
				s += "0"
			}
		}
		return s
	}
	f := *format
	if f == "" {
		f = "%c"
	}
	adjSec, adjUsec := sec, usec
	if usec < 0 {
		adjSec--
		adjUsec += 1000000
	}
	t := time.Unix(adjSec, 0).In(in.tzLoc())
	var b strings.Builder
	for i := 0; i < len(f); i++ {
		if f[i] != '%' {
			b.WriteByte(f[i])
			continue
		}
		i++
		if i >= len(f) {
			b.WriteByte('%')
			break
		}
		switch f[i] {
		case '@':
			b.WriteString(in.tftime(nil, sec, usec))
		case '.':
			fmt.Fprintf(&b, "%06d", adjUsec)
		case 's':
			fmt.Fprintf(&b, "%d", adjSec)
		case 'F':
			b.WriteString(strftime("%Y-%m-%d", t))
		case 'T':
			b.WriteString(strftime("%H:%M:%S", t))
		case 'E', 'O':
			if i+1 < len(f) {
				i++
				b.WriteString(strftime("%"+string(f[i]), t))
			}
		default:
			b.WriteString(strftime("%"+string(f[i]), t))
		}
	}
	return b.String()
}

// strftime formats t as the reference libc (macOS) does.
func strftime(f string, t time.Time) string {
	var b strings.Builder
	for i := 0; i < len(f); i++ {
		if f[i] != '%' || i+1 >= len(f) {
			b.WriteByte(f[i])
			continue
		}
		i++
		c := f[i]
		if c == 'E' || c == 'O' {
			if i+1 < len(f) {
				i++
				c = f[i]
			}
		}
		yd := t.YearDay()
		switch c {
		case 'a':
			b.WriteString(t.Format("Mon"))
		case 'A':
			b.WriteString(t.Format("Monday"))
		case 'b', 'h':
			b.WriteString(t.Format("Jan"))
		case 'B':
			b.WriteString(t.Format("January"))
		case 'c':
			b.WriteString(strftime("%a %b %e %H:%M:%S %Y", t))
		case 'C':
			fmt.Fprintf(&b, "%02d", t.Year()/100)
		case 'd':
			fmt.Fprintf(&b, "%02d", t.Day())
		case 'D':
			b.WriteString(strftime("%m/%d/%y", t))
		case 'e':
			fmt.Fprintf(&b, "%2d", t.Day())
		case 'F':
			b.WriteString(strftime("%Y-%m-%d", t))
		case 'g':
			y, _ := t.ISOWeek()
			fmt.Fprintf(&b, "%02d", y%100)
		case 'G':
			y, _ := t.ISOWeek()
			fmt.Fprintf(&b, "%d", y)
		case 'H':
			fmt.Fprintf(&b, "%02d", t.Hour())
		case 'I':
			fmt.Fprintf(&b, "%02d", hour12(t))
		case 'j':
			fmt.Fprintf(&b, "%03d", yd)
		case 'k':
			fmt.Fprintf(&b, "%2d", t.Hour())
		case 'l':
			fmt.Fprintf(&b, "%2d", hour12(t))
		case 'm':
			fmt.Fprintf(&b, "%02d", int(t.Month()))
		case 'M':
			fmt.Fprintf(&b, "%02d", t.Minute())
		case 'n':
			b.WriteByte('\n')
		case 'p':
			b.WriteString(t.Format("PM"))
		case 'r':
			b.WriteString(strftime("%I:%M:%S %p", t))
		case 'R':
			b.WriteString(strftime("%H:%M", t))
		case 's':
			fmt.Fprintf(&b, "%d", t.Unix())
		case 'S':
			fmt.Fprintf(&b, "%02d", t.Second())
		case 't':
			b.WriteByte('\t')
		case 'T':
			b.WriteString(strftime("%H:%M:%S", t))
		case 'u':
			wd := int(t.Weekday())
			if wd == 0 {
				wd = 7
			}
			fmt.Fprintf(&b, "%d", wd)
		case 'U':
			fmt.Fprintf(&b, "%02d", (yd-1+7-int(t.Weekday()))/7)
		case 'V':
			_, w := t.ISOWeek()
			fmt.Fprintf(&b, "%02d", w)
		case 'w':
			fmt.Fprintf(&b, "%d", int(t.Weekday()))
		case 'W':
			fmt.Fprintf(&b, "%02d", (yd-1+7-(int(t.Weekday())+6)%7)/7)
		case 'x':
			b.WriteString(strftime("%m/%d/%Y", t))
		case 'X':
			b.WriteString(strftime("%H:%M:%S", t))
		case 'y':
			fmt.Fprintf(&b, "%02d", t.Year()%100)
		case 'Y':
			fmt.Fprintf(&b, "%d", t.Year())
		case 'z':
			b.WriteString(t.Format("-0700"))
		case 'Z':
			b.WriteString(t.Format("MST"))
		case '%':
			b.WriteByte('%')
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

func hour12(t time.Time) int {
	h := t.Hour() % 12
	if h == 0 {
		h = 12
	}
	return h
}

func fnFtime(in *Interp, a []Value) (Value, error) {
	var sec, usec int64
	if len(a) < 2 {
		now := in.now()
		sec, usec = now.Unix(), int64(now.Nanosecond()/1000)
	} else {
		var ok bool
		if sec, usec, ok = valTime(a[1]); !ok {
			in.warn("real value too large to convert to time")
			return Str(""), nil
		}
	}
	if len(a) == 0 {
		s := in.tftime(nil, sec, usec)
		return Str(s), nil
	}
	f := a[0].String()
	return Str(in.tftime(&f, sec, usec)), nil
}

func fnTime(in *Interp, a []Value) (Value, error) { return atimeOf(in.now()), nil }

func fnMktime(in *Interp, a []Value) (Value, error) {
	f := [7]int{1970, 1, 1, 0, 0, 0, 0}
	for i := 0; i < len(a) && i < 7; i++ {
		f[i] = int(a[i].Int())
	}
	if len(a) == 0 {
		f[0] = 1970
	}
	if f[6] < 0 || f[6] > 999999 {
		return timeMicro(big.NewInt(-1000000), ttAtime), nil
	}
	t := time.Date(f[0], time.Month(f[1]), f[2], f[3], f[4], f[5], 0, in.tzLoc())
	us := int64(f[6])
	sec := t.Unix()
	if sec < 0 && us > 0 {
		sec++
		us -= 1000000
	}
	return timeMicro(big.NewInt(sec*1000000+us), ttAtime), nil
}

// expandFilename is expand_filename() in tfio.c: "~" and "~/..." use the HOME
// variable, "~user" the user's home directory (unchanged if there is none).
func (in *Interp) expandFilename(p string) string {
	if !strings.HasPrefix(p, "~") {
		return p
	}
	end := strings.IndexByte(p, '/')
	if end < 0 {
		end = len(p)
	}
	user := p[1:end]
	if user == "" {
		return in.Var("HOME") + p[end:]
	}
	if u, err := osuser.Lookup(user); err == nil {
		return u.HomeDir + p[end:]
	}
	return p
}

func fnTest(in *Interp, a []Value) (Value, error) { return in.evalExpr(a[0].String()) }

func fnFwrite(in *Interp, a []Value) (Value, error) {
	name := in.expandFilename(a[0].String())
	if !in.allow(OpFileWrite, "fwrite", name) {
		return Int(0), nil
	}
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o666)
	if err != nil {
		in.warn("%s: %s", a[0].String(), cErr(err))
		return Int(0), nil
	}
	defer f.Close()
	fmt.Fprintf(f, "%s\n", a[1].String())
	return Int(1), nil
}

func init() {
	f := builtinFuncs
	f["ftime"] = fnFtime
	f["time"] = fnTime
	f["mktime"] = fnMktime
	f["test"] = fnTest
	f["fwrite"] = fnFwrite
	f["filename"] = func(in *Interp, a []Value) (Value, error) { return Str(in.expandFilename(str(a, 0))), nil }
}
