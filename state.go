package gtfparser

// State export: SaveState and LoadState carry everything a running session
// holds between two commands from one Interp to another, so a client can
// re-exec itself ("/restart") and continue where it was. docs/state-export.md
// describes the format, the version rules and what is (not) carried.
//
// The encoding is one JSON document, the envelope:
//
//	{"magic":"gtfparser-state","format":1,"minReader":1,
//	 "sections":{"globals":{"v":1,"required":true,"data":{...}}, ...}}
//
// Every part of the state is a section with its own version. A reader accepts
// every older version of every section it knows, skips unknown optional
// sections, and refuses (ErrStateTooNew) a required section, or an envelope,
// it cannot understand. The output is canonical: the same state always yields
// the same bytes.

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"sort"
	"strconv"
	"time"
	"unicode/utf8"
)

// StateFormat is the envelope version this package writes and the newest one
// it reads.
const StateFormat = 1

const stateMagic = "gtfparser-state"

// maxStateBytes bounds what LoadState reads (a state file is a few hundred
// kilobytes at most; this only stops a runaway input).
const maxStateBytes = 1 << 30

var (
	// ErrStateTooNew: the input was written by a newer engine and holds a
	// part (or an envelope) this one cannot understand. The Interp is left
	// untouched.
	ErrStateTooNew = errors.New("gtfparser: state was written by a newer version")
	// ErrStateCorrupt: the input is not a state document, is truncated, or is
	// inconsistent. The Interp is left untouched.
	ErrStateCorrupt = errors.New("gtfparser: corrupt state")
	// ErrStateIncompatible: the state is well formed but cannot be applied to
	// this Interp (for example a macro uses a match style the client has not
	// registered yet). The Interp is left untouched.
	ErrStateIncompatible = errors.New("gtfparser: state does not fit this interpreter")
)

type stateEnvelope struct {
	Magic     string                  `json:"magic"`
	Format    int                     `json:"format"`
	MinReader int                     `json:"minReader"`
	Sections  map[string]stateSection `json:"sections"`
}

type stateSection struct {
	V        int             `json:"v"`
	Required bool            `json:"required"`
	Data     json.RawMessage `json:"data"`
}

// stateSections lists the sections in the order they are decoded and applied.
// maxV is the newest version of the section this engine understands; dec
// decodes (and migrates) any version from 1 to maxV into ld.
var stateSections = []struct {
	name     string
	required bool
	maxV     int
	save     func(in *Interp, c *saveCtx) any
	dec      func(in *Interp, v int, raw json.RawMessage, ld *stLoad) error
	apply    func(in *Interp, ld *stLoad)
}{
	{"interp", true, 1, secSaveInterp, secDecInterp, secApplyInterp},
	{"globals", true, 1, secSaveGlobals, secDecGlobals, secApplyGlobals},
	{"locals", true, 1, secSaveLocals, secDecLocals, secApplyLocals},
	{"macros", true, 1, secSaveMacros, secDecMacros, secApplyMacros},
	{"worlds", true, 1, secSaveWorlds, secDecWorlds, secApplyWorlds},
	{"history", true, 1, secSaveHistory, secDecHistory, secApplyHistory},
	{"procs", true, 1, secSaveProcs, secDecProcs, secApplyProcs},
	{"streams", false, 1, secSaveStreams, secDecStreams, secApplyStreams},
	{"terminal", false, 1, secSaveTerminal, secDecTerminal, secApplyTerminal},
	{"limit", false, 1, secSaveLimit, secDecLimit, secApplyLimit},
	{"watch", false, 1, secSaveWatch, secDecWatch, secApplyWatch},
	{"mail", false, 1, secSaveMail, secDecMail, secApplyMail},
	{"origins", true, 1, secSaveOrigins, secDecOrigins, secApplyOrigins}, // after macros, procs and streams; omitted when empty
}

// saveCtx is what the section writers share.
type saveCtx struct {
	now    time.Time
	noEcho bool // the keyboard line belongs to a world whose server echoes: never saved
}

// SaveState writes the interpreter's state to w (see docs/state-export.md for
// what it holds). Call it between commands: the macro that is running (the
// command that asked for the save, say) is ignored. The output is
// deterministic. Anything typed while the foreground world was in no-echo
// mode (a password) is left out. The state holds world passwords as /saveworld
// does, in clear text: protect the file (mode 0600) and delete it after use.
func (in *Interp) SaveState(w io.Writer) error {
	c := &saveCtx{now: in.now()}
	cw := in.CurrentWorld()
	c.noEcho = !in.LocalEcho("") || (cw != "" && !in.LocalEcho(cw))
	env := stateEnvelope{Magic: stateMagic, Format: StateFormat, MinReader: 1, Sections: map[string]stateSection{}}
	for _, s := range stateSections {
		sv := s.save(in, c)
		if sv == nil { // a section with nothing to carry is left out (origins)
			continue
		}
		data, err := marshalNoEscape(sv)
		if err != nil {
			return fmt.Errorf("gtfparser: saving state section %s: %w", s.name, err)
		}
		env.Sections[s.name] = stateSection{V: s.maxV, Required: s.required, Data: data}
	}
	out, err := marshalNoEscape(env)
	if err != nil {
		return fmt.Errorf("gtfparser: saving state: %w", err)
	}
	_, err = w.Write(out)
	return err
}

func marshalNoEscape(v any) ([]byte, error) {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	if err := e.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}

// LoadState replaces the interpreter's state with the one SaveState wrote. It
// is meant for a fresh Interp (after New, after the client installed its
// commands, functions, match styles and Host capabilities, and before any
// script ran); on one that has been used it replaces whatever the state
// covers: variables, macros, worlds, histories, processes, streams and so on.
// What the state does not mention (and what the client owns: registered
// commands and functions, Host, Policy, Limits, Now, LibDirs) is kept.
//
// The input is decoded and checked completely before anything is changed: on
// any error (ErrStateTooNew, ErrStateCorrupt, ErrStateIncompatible, or a read
// error) the Interp is exactly as it was. After a successful load /repeat and
// /quote processes are re-armed relative to the Interp's clock, so with a
// Runner nothing else is needed (it looks at NextDue after every call); a
// client that drives RunDue itself reads NextDue as usual.
func (in *Interp) LoadState(r io.Reader) (err error) {
	data, err := io.ReadAll(io.LimitReader(r, maxStateBytes+1))
	if err != nil {
		return err
	}
	if len(data) > maxStateBytes {
		return fmt.Errorf("%w: input too large", ErrStateCorrupt)
	}
	ld := &stLoad{}
	if err := in.decodeState(data, ld); err != nil {
		return err
	}
	for _, s := range stateSections {
		if ld.seen[s.name] {
			s.apply(in, ld)
		}
	}
	in.finishLoad(ld)
	return nil
}

// decodeState parses and validates the whole document and builds every object
// the apply phase installs. It changes nothing in in.
func (in *Interp) decodeState(data []byte, ld *stLoad) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%w: %v", ErrStateCorrupt, r)
		}
	}()
	var env stateEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return fmt.Errorf("%w: %v", ErrStateCorrupt, err)
	}
	if env.Magic != stateMagic {
		return fmt.Errorf("%w: not a state document", ErrStateCorrupt)
	}
	if env.MinReader > StateFormat {
		return fmt.Errorf("%w: needs a reader of format %d, this one reads %d", ErrStateTooNew, env.MinReader, StateFormat)
	}
	if env.Format < 1 {
		return fmt.Errorf("%w: bad format number %d", ErrStateCorrupt, env.Format)
	}
	known := map[string]bool{}
	for _, s := range stateSections {
		known[s.name] = true
	}
	var unknown []string
	for name, sec := range env.Sections {
		if !known[name] {
			if sec.Required {
				unknown = append(unknown, name)
			}
			continue
		}
	}
	sort.Strings(unknown)
	if len(unknown) > 0 {
		return fmt.Errorf("%w: required section %q is unknown", ErrStateTooNew, unknown[0])
	}
	// building patterns must not talk to the user (a regexp with '{' warns)
	oldWarn := in.Warn
	in.Warn = func(string) {}
	defer func() { in.Warn = oldWarn }()
	ld.seen = map[string]bool{}
	for _, s := range stateSections {
		sec, ok := env.Sections[s.name]
		if !ok {
			continue
		}
		if sec.V < 1 {
			return fmt.Errorf("%w: section %s has version %d", ErrStateCorrupt, s.name, sec.V)
		}
		if sec.V > s.maxV {
			if sec.Required {
				return fmt.Errorf("%w: section %s is version %d, this reader knows up to %d", ErrStateTooNew, s.name, sec.V, s.maxV)
			}
			continue // an optional part this reader cannot read: left as it is
		}
		if err := s.dec(in, sec.V, sec.Data, ld); err != nil {
			if errors.Is(err, ErrStateIncompatible) || errors.Is(err, ErrStateCorrupt) {
				return fmt.Errorf("section %s: %w", s.name, err)
			}
			return fmt.Errorf("section %s: %w: %v", s.name, ErrStateCorrupt, err)
		}
		ld.seen[s.name] = true
	}
	return ld.check()
}

// ---------------------------------------------------------------- strings and values

// bs is a string that survives JSON whatever bytes it holds: valid UTF-8 is
// written as a JSON string, anything else as {"b64":"..."}.
type bs string

func (s bs) MarshalJSON() ([]byte, error) {
	if utf8.ValidString(string(s)) {
		return json.Marshal(string(s))
	}
	return json.Marshal(struct {
		B string `json:"b64"`
	}{base64.StdEncoding.EncodeToString([]byte(s))})
}

func (s *bs) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '{' {
		var o struct {
			B *string `json:"b64"`
		}
		if err := json.Unmarshal(b, &o); err != nil {
			return err
		}
		if o.B == nil {
			return errors.New("bad string object")
		}
		d, err := base64.StdEncoding.DecodeString(*o.B)
		if err != nil {
			return err
		}
		*s = bs(d)
		return nil
	}
	var t string
	if err := json.Unmarshal(b, &t); err != nil {
		return err
	}
	*s = bs(t)
	return nil
}

// sVal is a Value: K is "i" (integer), "r" (float, F holds its bits), "s"
// (string), "d" (fixed-point decimal / time, D holds value*1e6, T the time
// type), E the enumeration index of an enumerated variable.
type sVal struct {
	K string `json:"k"`
	I int64  `json:"i,omitempty"`
	F string `json:"f,omitempty"`
	S bs     `json:"s,omitempty"`
	D string `json:"d,omitempty"`
	T uint8  `json:"t,omitempty"`
	E int    `json:"e,omitempty"`
}

func encVal(v Value) sVal {
	s := sVal{T: v.tt, E: v.en}
	switch v.k {
	case kInt:
		s.K, s.I = "i", v.i
	case kReal:
		s.K, s.F = "r", strconv.FormatUint(math.Float64bits(v.f), 16)
	case kDec:
		s.K = "d"
		if v.dec != nil {
			s.D = v.dec.String()
		} else {
			s.D = "0"
		}
	default:
		s.K, s.S = "s", bs(v.s)
	}
	return s
}

func decVal(s sVal) (Value, error) {
	v := Value{tt: s.T, en: s.E}
	switch s.K {
	case "i":
		v.k, v.i = kInt, s.I
	case "r":
		u, err := strconv.ParseUint(s.F, 16, 64)
		if err != nil {
			return Value{}, fmt.Errorf("bad float %q", s.F)
		}
		v.k, v.f = kReal, math.Float64frombits(u)
	case "d":
		n, ok := new(big.Int).SetString(s.D, 10)
		if !ok {
			return Value{}, fmt.Errorf("bad decimal %q", s.D)
		}
		v.k, v.dec = kDec, n
	case "s":
		v.k, v.s = kStr, string(s.S)
	default:
		return Value{}, fmt.Errorf("bad value kind %q", s.K)
	}
	return v, nil
}

const stateTimeLayout = time.RFC3339Nano

func encTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(stateTimeLayout)
}

func decTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse(stateTimeLayout, s)
}
