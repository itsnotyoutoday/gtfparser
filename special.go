package gtfparser

import (
	"strings"
)

func (sp varSpec) initial() Value {
	switch sp.kind {
	case "int", "pos":
		return Str(sp.def).Num()
	case "time":
		return Str(sp.def).Num()
	case "enum":
		v, _ := sp.coerce(Str(sp.def))
		return v
	}
	return Str(sp.def)
}

// coerce converts a value for assignment to this special variable.
func (sp varSpec) coerce(v Value) (Value, bool) {
	switch sp.kind {
	case "int":
		return Int(v.Int()), true
	case "pos":
		n := v.Int()
		if n < 0 {
			return v, false
		}
		return Int(n), true
	case "time":
		return v.Num(), true
	case "enum":
		s := v.String()
		for i, e := range sp.enum {
			if strings.EqualFold(e, s) {
				return Value{k: kStr, s: e, en: i + 1}, true
			}
		}
		if v.k != kStr || looksNumeric(s) { // a number selects by index; on/off are 1/0
			if n := int(v.Num().Int()); n >= 0 && n < len(sp.enum) {
				return Value{k: kStr, s: sp.enum[n], en: n + 1}, true
			}
		}
		return v, false
	}
	return v, true
}

func looksNumeric(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	for i, c := range s {
		if !(c >= '0' && c <= '9' || (i == 0 && (c == '-' || c == '+'))) {
			return false
		}
	}
	return true
}

// setGlobal assigns a global variable, validating special variables.
func (in *Interp) setGlobal(name string, v Value) bool {
	if sp, ok := specialVars[name]; ok {
		cv, ok := sp.coerce(v)
		if !ok {
			in.warn("/set: invalid value for %s: %s", name, v.String())
			return false
		}
		v = cv
		if name == "sigfigs" {
			setSigFigs(int(cv.Int()))
		}
	}
	in.globals[name] = v
	return true
}

func typeName(v Value) string {
	if v.en > 0 {
		return "enum"
	}
	switch v.k {
	case kInt:
		return "int"
	case kDec:
		switch v.tt {
		case ttDtime:
			return "dtime"
		case ttAtime:
			return "atime"
		}
		return "decimal"
	case kReal:
		return "float"
	}
	return "string"
}

// whatisVar is whatis() applied to a bare variable name.
func (in *Interp) whatisVar(name string) string {
	v, ok := in.lookup(name)
	if !ok {
		return "id:"
	}
	if sp, ok := specialVars[name]; ok {
		switch sp.kind {
		case "pos":
			return "id:pos"
		case "time":
			return "id:dtime"
		}
	}
	return "id:" + typeName(v)
}
