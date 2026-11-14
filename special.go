package gtfparser

import (
	"math/big"
	"strings"
)

func (sp varSpec) initial() Value {
	switch sp.kind {
	case "int", "pos":
		return Str(sp.def).Num()
	case "time":
		v := Str(sp.def).Num()
		v.tt = ttDtime
		return v
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
		if n := v.Int(); n >= 0 {
			return Int(n), true
		}
		return v, false
	case "pos":
		if n := v.Int(); n >= 1 {
			return Int(n), true
		}
		return v, false
	case "time":
		// a time variable holds a duration: whole seconds print as "5.0"
		sec, usec, _ := valTime(v)
		return timeMicro(big.NewInt(sec*1000000+usec), ttDtime), true
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
			switch sp.kind {
			case "int":
				in.warn("%s must be an integer greater than or equal to 0", name)
			case "pos":
				in.warn("%s must be an integer greater than or equal to 1", name)
			case "enum":
				in.warn("Invalid %s value \"%s\".  Valid values are: %s", name, v.String(), enumList(sp.enum, func(int) bool { return true }))
			default:
				in.warn("/set: invalid value for %s: %s", name, v.String())
			}
			return false
		}
		v = cv
		if name == "visual" && v.en == 2 {
			if vh, ok := in.Host.(VisualHost); ok && !vh.CanVisual() {
				v, _ = sp.coerce(Str("off")) // refused, silently
			}
		}
		if name == "sigfigs" {
			setSigFigs(int(cv.Int()))
		}
	}
	if _, exists := in.globals[name]; !exists && varNameSpecial(name) {
		in.warn("Warning: \"%s\" conflicts with the name of a special substitution, so it will not be accessible with a \"%%%s\" or \"{%s}\" substitution.", name, name, name)
	}
	switch name {
	case "prompt_sec", "prompt_usec":
		in.warn("Warning: %s is obsolete.  Use prompt_wait instead.", name)
	case "virtscreen":
		in.warn("Warning: %s is undocumented, possibly unstable, and may be removed in a future version.", name)
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
