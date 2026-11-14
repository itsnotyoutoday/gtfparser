package gtfparser

import (
	"fmt"
	"reflect"
	"strings"
)

// Call is what a table-declared command handler receives: the interpreter
// (embedded, so c.Echo, c.Var, c.Run... work directly) plus the command's
// input already parsed.
type Call struct {
	*Interp
	Name string   // the command as invoked, without the slash
	Raw  string   // everything after the command name
	Rest string   // Raw with the declared -options removed (like %{*} after getopts)
	Args []string // Rest split into words
	opts map[byte]Value
}

// Has reports whether -x was given.
func (c *Call) Has(opt byte) bool { _, ok := c.opts[opt]; return ok }

// Opt is the string value of -x (empty if absent).
func (c *Call) Opt(opt byte) string { return c.opts[opt].String() }

// Int is the integer value of -x, or def if absent.
func (c *Call) Int(opt byte, def int64) int64 {
	if v, ok := c.opts[opt]; ok {
		return v.Int()
	}
	return def
}

// Arg is the i'th word after the options ("" if there is none).
func (c *Call) Arg(i int) string {
	if i < len(c.Args) {
		return c.Args[i]
	}
	return ""
}

// Fail prints a warning prefixed with the command name and returns 0, the
// conventional way for a handler to report a problem.
func (c *Call) Fail(format string, a ...any) Value {
	c.Warnf("/%s: %s", c.Name, fmt.Sprintf(format, a...))
	return Int(0)
}

// Func turns an ordinary Go function into a FuncSpec: the engine converts the
// script's arguments to the parameter types and the result back to a TF value.
//
//	gtfparser.Func("clamp", func(v, lo, hi int) int { return max(lo, min(v, hi)) })
//
// Parameters may be int types, float64, string, bool, or Value (passed through);
// a final variadic parameter takes any number of further arguments. The
// function may return one value, optionally followed by an error (reported as
// a script error). The number of arguments is checked for you.
func Func(name string, fn any) FuncSpec {
	v := reflect.ValueOf(fn)
	t := v.Type()
	if t.Kind() != reflect.Func {
		panic("gtfparser.Func: " + name + " is not a function")
	}
	if t.NumOut() > 2 || (t.NumOut() == 2 && !t.Out(1).Implements(errorType)) {
		panic("gtfparser.Func: " + name + " must return (T) or (T, error)")
	}
	lo, hi := t.NumIn(), t.NumIn()
	if t.IsVariadic() {
		lo, hi = t.NumIn()-1, 0
	}
	usage := make([]string, t.NumIn())
	for i := range usage {
		usage[i] = typeLabel(t.In(i))
		if t.IsVariadic() && i == t.NumIn()-1 {
			usage[i] = typeLabel(t.In(i).Elem()) + "..."
		}
	}
	return FuncSpec{
		Name:    name,
		Usage:   strings.Join(usage, ", "),
		MinArgs: lo,
		MaxArgs: hi,
		Fn: func(in *Interp, a []Value) (Value, error) {
			args := make([]reflect.Value, len(a))
			for i, x := range a {
				pi := i
				if pi > t.NumIn()-1 {
					pi = t.NumIn() - 1
				}
				pt := t.In(pi)
				if t.IsVariadic() && i >= t.NumIn()-1 {
					pt = t.In(t.NumIn() - 1).Elem()
				}
				args[i] = toGo(x, pt)
			}
			out := v.Call(args)
			if len(out) == 2 && !out[1].IsNil() {
				return Int(0), out[1].Interface().(error)
			}
			if len(out) == 0 {
				return Int(1), nil
			}
			return fromGo(out[0]), nil
		},
	}
}

var errorType = reflect.TypeOf((*error)(nil)).Elem()
var valueType = reflect.TypeOf(Value{})

func typeLabel(t reflect.Type) string {
	switch {
	case t == valueType:
		return "value"
	case t.Kind() == reflect.String:
		return "string"
	case t.Kind() == reflect.Bool:
		return "bool"
	case t.Kind() == reflect.Float32 || t.Kind() == reflect.Float64:
		return "number"
	}
	return "int"
}

func toGo(x Value, t reflect.Type) reflect.Value {
	out := reflect.New(t).Elem()
	switch {
	case t == valueType:
		out.Set(reflect.ValueOf(x))
	case t.Kind() == reflect.String:
		out.SetString(x.String())
	case t.Kind() == reflect.Bool:
		out.SetBool(x.Truth())
	case t.Kind() == reflect.Float32 || t.Kind() == reflect.Float64:
		out.SetFloat(x.Float())
	case t.Kind() >= reflect.Int && t.Kind() <= reflect.Int64:
		out.SetInt(x.Int())
	case t.Kind() >= reflect.Uint && t.Kind() <= reflect.Uint64:
		out.SetUint(uint64(x.Int()))
	}
	return out
}

func fromGo(r reflect.Value) Value {
	switch {
	case r.Type() == valueType:
		return r.Interface().(Value)
	case r.Kind() == reflect.String:
		return Str(r.String())
	case r.Kind() == reflect.Bool:
		return Bool(r.Bool())
	case r.Kind() == reflect.Float32 || r.Kind() == reflect.Float64:
		return Real(r.Float())
	case r.Kind() >= reflect.Int && r.Kind() <= reflect.Int64:
		return Int(r.Int())
	case r.Kind() >= reflect.Uint && r.Kind() <= reflect.Uint64:
		return Int(int64(r.Uint()))
	}
	return Str(fmt.Sprint(r.Interface()))
}
