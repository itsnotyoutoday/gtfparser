package gtfparser

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
)

// Expression parsing builds closures; evaluation happens when they run, so
// && style short-circuiting and ?: only evaluate what they must.

type eval func() (Value, error)

type exprParser struct {
	in  *Interp
	s   string
	pos int
}

type exprEntry struct {
	f   eval
	err error
}

// EvalExpr evaluates a TF expression.
func (in *Interp) EvalExpr(src string) (Value, error) {
	defer in.enter()()
	return in.evalExpr(src)
}

// evalExpr compiles src once (the compiled closure tree is cached per
// interpreter, keyed by source text) and runs it.
func (in *Interp) evalExpr(src string) (Value, error) {
	e := in.exprCache[src]
	if e == nil {
		p := &exprParser{in: in, s: src}
		f, err := p.parseComma()
		if err == nil {
			p.ws()
			if p.pos < len(p.s) {
				err = fmt.Errorf("expression: unexpected %q", p.s[p.pos:])
			}
		}
		e = &exprEntry{f: f, err: err}
		if len(in.exprCache) >= cacheMax {
			in.exprCache = map[string]*exprEntry{}
		}
		in.exprCache[src] = e
	}
	if e.err != nil {
		return Int(0), e.err
	}
	return e.f()
}

func (p *exprParser) ws() {
	for p.pos < len(p.s) && (p.s[p.pos] == ' ' || p.s[p.pos] == '\t' || p.s[p.pos] == '\n') {
		p.pos++
	}
}

func (p *exprParser) peek(tok string) bool {
	p.ws()
	return strings.HasPrefix(p.s[p.pos:], tok)
}

func (p *exprParser) take(tok string) bool {
	if p.peek(tok) {
		p.pos += len(tok)
		return true
	}
	return false
}

func (p *exprParser) parseComma() (eval, error) {
	l, err := p.parseAssign()
	if err != nil {
		return nil, err
	}
	for p.take(",") {
		r, err := p.parseAssign()
		if err != nil {
			return nil, err
		}
		l0 := l
		l = func() (Value, error) {
			if _, err := l0(); err != nil {
				return Int(0), err
			}
			return r()
		}
	}
	return l, nil
}

var assignOps = []string{":=", "+=", "-=", "*=", "/="}

func (p *exprParser) parseAssign() (eval, error) {
	start := p.pos
	p.ws()
	// an assignment target is a bare identifier
	if id, ok := p.ident(); ok {
		for _, op := range assignOps {
			if p.peek(op) {
				p.pos += len(op)
				rhs, err := p.parseAssign()
				if err != nil {
					return nil, err
				}
				return func() (Value, error) {
					r, err := rhs()
					if err != nil {
						return Int(0), err
					}
					if op != ":=" {
						cur, _ := p.in.lookup(id)
						r, err = arith(op[:1], cur, r)
						if err != nil {
							return Int(0), err
						}
					}
					p.in.assign(id, r)
					return r, nil
				}, nil
			}
		}
	}
	p.pos = start
	return p.parseTernary()
}

func (p *exprParser) parseTernary() (eval, error) {
	c, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if !p.peek("?") {
		return c, nil
	}
	p.take("?")
	var a eval
	if !p.peek(":") || p.peek(":=") {
		a, err = p.parseAssign()
		if err != nil {
			return nil, err
		}
	}
	if !p.take(":") {
		return nil, errors.New("expression: expected ':' in conditional")
	}
	b, err := p.parseAssign()
	if err != nil {
		return nil, err
	}
	return func() (Value, error) {
		cv, err := c()
		if err != nil {
			return cv, err
		}
		if cv.Truth() {
			if a == nil {
				return cv, nil
			}
			return a()
		}
		return b()
	}, nil
}

func (p *exprParser) parseOr() (eval, error) {
	l, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.peek("|") && !p.peek("||") {
		p.take("|")
		r, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		l0 := l
		l = func() (Value, error) {
			v, err := l0()
			if err != nil || v.Truth() {
				return v, err
			}
			return r()
		}
	}
	return l, nil
}

func (p *exprParser) parseAnd() (eval, error) {
	l, err := p.parseRel()
	if err != nil {
		return nil, err
	}
	for p.peek("&") && !p.peek("&&") {
		p.take("&")
		r, err := p.parseRel()
		if err != nil {
			return nil, err
		}
		l0 := l
		l = func() (Value, error) {
			v, err := l0()
			if err != nil || !v.Truth() {
				return v, err
			}
			return r()
		}
	}
	return l, nil
}

func (p *exprParser) binLevel(ops []string, next func() (eval, error)) (eval, error) {
	l, err := next()
	if err != nil {
		return nil, err
	}
	for {
		var op string
		for _, o := range ops {
			if p.peek(o) {
				op = o
				break
			}
		}
		if op == "" {
			return l, nil
		}
		p.take(op)
		r, err := next()
		if err != nil {
			return nil, err
		}
		l0, op0 := l, op
		l = func() (Value, error) {
			a, err := l0()
			if err != nil {
				return a, err
			}
			b, err := r()
			if err != nil {
				return b, err
			}
			return binary(op0, a, b)
		}
	}
}

// Relational binds looser than equality in TF's operator table.
func (p *exprParser) parseRel() (eval, error) {
	return p.binLevel([]string{"<=", ">=", "<", ">"}, p.parseEq)
}
func (p *exprParser) parseEq() (eval, error) {
	// longest operators first so "=~" and "==" are not read as "="
	return p.binLevel([]string{"==", "!=", "=~", "!~", "=/", "!/", "="}, p.parseAdd)
}
func (p *exprParser) parseAdd() (eval, error) {
	return p.binLevel([]string{"+", "-"}, p.parseMul)
}
func (p *exprParser) parseMul() (eval, error) {
	return p.binLevel([]string{"*", "/"}, p.parseUnary)
}

func (p *exprParser) parseUnary() (eval, error) {
	p.ws()
	switch {
	case p.peek("++") || p.peek("--"):
		op := p.s[p.pos : p.pos+2]
		p.pos += 2
		p.ws()
		id, ok := p.ident()
		if !ok {
			return nil, errors.New("expression: ++/-- needs a variable")
		}
		return func() (Value, error) {
			cur, _ := p.in.lookup(id)
			r, err := arith(op[:1], cur, Int(1))
			if err != nil {
				return r, err
			}
			p.in.assign(id, r)
			return r, nil
		}, nil
	case p.peek("!") && !p.peek("!=") && !p.peek("!~") && !p.peek("!/"):
		p.take("!")
		x, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return func() (Value, error) {
			v, err := x()
			return Bool(!v.Truth()), err
		}, nil
	case p.peek("-"):
		p.take("-")
		x, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return func() (Value, error) {
			v, err := x()
			if err != nil {
				return v, err
			}
			n := v.Num()
			switch n.k {
			case kInt:
				return Int(-n.i), nil
			case kDec:
				return decMicro(new(big.Int).Neg(n.dec)), nil
			}
			return Real(-n.f), nil
		}, nil
	case p.peek("+"):
		p.take("+")
		x, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return func() (Value, error) {
			v, err := x()
			return v.Num(), err
		}, nil
	}
	return p.parsePrimary()
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
func isIdentChar(c byte) bool { return isIdentStart(c) || (c >= '0' && c <= '9') }

func (p *exprParser) ident() (string, bool) {
	p.ws()
	if p.pos >= len(p.s) || !isIdentStart(p.s[p.pos]) {
		return "", false
	}
	st := p.pos
	for p.pos < len(p.s) && isIdentChar(p.s[p.pos]) {
		p.pos++
	}
	return p.s[st:p.pos], true
}

func (p *exprParser) parsePrimary() (eval, error) {
	p.ws()
	if p.pos >= len(p.s) {
		return nil, errors.New("expression: unexpected end")
	}
	c := p.s[p.pos]
	switch {
	case c == '(':
		p.pos++
		e, err := p.parseComma()
		if err != nil {
			return nil, err
		}
		if !p.take(")") {
			return nil, errors.New("expression: missing ')'")
		}
		return e, nil
	case c >= '0' && c <= '9' || (c == '.' && p.pos+1 < len(p.s) && p.s[p.pos+1] >= '0' && p.s[p.pos+1] <= '9'):
		return p.number()
	case c == '"' || c == '\'' || c == '`':
		return p.quoted(c)
	case c == '{':
		end := matchClose(p.s, p.pos, '{', '}')
		if end < 0 {
			return nil, errors.New("expression: missing '}'")
		}
		sel := p.s[p.pos+1 : end]
		p.pos = end + 1
		return func() (Value, error) { return Str(p.in.selector(sel)), nil }, nil
	case c == '%':
		// macro-style substitution inside an expression
		n, text := p.in.expandOne(p.s[p.pos:])
		p.pos += n
		return func() (Value, error) { return Str(text()), nil }, nil
	case c == '$':
		n, text := p.in.expandOne(p.s[p.pos:])
		if n == 0 {
			return nil, errors.New("expression: stray '$'")
		}
		p.pos += n
		return func() (Value, error) { return Str(text()), nil }, nil
	case isIdentStart(c):
		id, _ := p.ident()
		if p.pos < len(p.s) && p.s[p.pos] == '(' {
			return p.call(id)
		}
		return func() (Value, error) {
			v, _ := p.in.lookup(id)
			return v, nil
		}, nil
	}
	return nil, fmt.Errorf("expression: unexpected %q", string(c))
}

func (p *exprParser) number() (eval, error) {
	st := p.pos
	for p.pos < len(p.s) && (p.s[p.pos] >= '0' && p.s[p.pos] <= '9') {
		p.pos++
	}
	// dtime h:m[:s[.f]]
	if p.pos+1 < len(p.s) && p.s[p.pos] == ':' && p.s[p.pos+1] >= '0' && p.s[p.pos+1] <= '9' {
		for p.pos < len(p.s) && (p.s[p.pos] >= '0' && p.s[p.pos] <= '9' || p.s[p.pos] == ':' || p.s[p.pos] == '.') {
			p.pos++
		}
	} else {
		if p.pos < len(p.s) && p.s[p.pos] == '.' {
			p.pos++
			for p.pos < len(p.s) && p.s[p.pos] >= '0' && p.s[p.pos] <= '9' {
				p.pos++
			}
		}
		if p.pos < len(p.s) && (p.s[p.pos] == 'e' || p.s[p.pos] == 'E') {
			j := p.pos + 1
			if j < len(p.s) && (p.s[j] == '+' || p.s[j] == '-') {
				j++
			}
			if j < len(p.s) && p.s[j] >= '0' && p.s[j] <= '9' {
				for j < len(p.s) && p.s[j] >= '0' && p.s[j] <= '9' {
					j++
				}
				p.pos = j
			}
		}
	}
	v := parseNumPrefix(p.s[st:p.pos])
	return func() (Value, error) { return v, nil }, nil
}

func (p *exprParser) quoted(q byte) (eval, error) {
	p.pos++
	var b strings.Builder
	for p.pos < len(p.s) {
		c := p.s[p.pos]
		if c == '\\' && p.pos+1 < len(p.s) && (p.s[p.pos+1] == q || p.s[p.pos+1] == '\\') {
			b.WriteByte(p.s[p.pos+1])
			p.pos += 2
			continue
		}
		if c == q {
			p.pos++
			s := b.String()
			return func() (Value, error) { return Str(s), nil }, nil
		}
		b.WriteByte(c)
		p.pos++
	}
	return nil, errors.New("expression: unterminated string")
}

func (p *exprParser) call(name string) (eval, error) {
	p.take("(")
	var args []eval
	p.ws()
	if !p.take(")") {
		for {
			var a eval
			var err error
			if name == "whatis" && len(args) == 0 { // a bare variable reports "id:<type>"
				save := p.pos
				if id, ok := p.ident(); ok && p.peek(")") {
					p.take(")")
					return func() (Value, error) { return Str(p.in.whatisVar(id)), nil }, nil
				} else {
					p.pos = save
				}
			}
			if name == "tfread" { // the last argument is a variable passed by name
				save := p.pos
				if id, ok := p.ident(); ok && p.peek(")") {
					a = func() (Value, error) { return Str(id), nil }
				} else {
					p.pos = save
				}
			}
			if a == nil {
				a, err = p.parseAssign()
				if err != nil {
					return nil, err
				}
			}
			args = append(args, a)
			if p.take(",") {
				continue
			}
			if p.take(")") {
				break
			}
			return nil, fmt.Errorf("expression: bad arguments to %s()", name)
		}
	}
	return func() (Value, error) {
		vals := make([]Value, len(args))
		for i, a := range args {
			v, err := a()
			if err != nil {
				return v, err
			}
			vals[i] = v
		}
		return p.in.callFunc(name, vals)
	}, nil
}

// matchClose finds the index of the bracket closing the one at s[i], aware of
// nesting, quotes and backslashes.
func matchClose(s string, i int, open, cl byte) int {
	depth := 0
	var q byte
	for ; i < len(s); i++ {
		c := s[i]
		if c == '\\' {
			i++
			continue
		}
		if q != 0 {
			if c == q {
				q = 0
			}
			continue
		}
		switch c {
		case '"', '\'', '`':
			if open == '[' || open == '(' || open == '{' {
				// quotes only matter inside expression brackets
				q = c
			}
		case open:
			depth++
		case cl:
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func binary(op string, a, b Value) (Value, error) {
	switch op {
	case "=~":
		return Bool(a.String() == b.String()), nil
	case "!~":
		return Bool(a.String() != b.String()), nil
	case "=/":
		return Bool(globMatch(b.String(), a.String())), nil
	case "!/":
		return Bool(!globMatch(b.String(), a.String())), nil
	case "+", "-", "*", "/":
		return arith(op, a, b)
	}
	x, y := a.Num(), b.Num()
	var c int
	if (x.k == kInt || x.k == kDec) && (y.k == kInt || y.k == kDec) && !(x.k == kInt && y.k == kInt) {
		c = x.microOf().Cmp(y.microOf())
	} else if x.k == kInt && y.k == kInt {
		switch {
		case x.i < y.i:
			c = -1
		case x.i > y.i:
			c = 1
		}
	} else {
		xf, yf := a.Float(), b.Float()
		switch {
		case xf < yf:
			c = -1
		case xf > yf:
			c = 1
		}
	}
	switch op {
	case "==", "=":
		return Bool(c == 0), nil
	case "!=":
		return Bool(c != 0), nil
	case "<":
		return Bool(c < 0), nil
	case "<=":
		return Bool(c <= 0), nil
	case ">":
		return Bool(c > 0), nil
	case ">=":
		return Bool(c >= 0), nil
	}
	return Int(0), fmt.Errorf("expression: unknown operator %s", op)
}

func arith(op string, a, b Value) (Value, error) {
	x, y := a.Num(), b.Num()
	if x.k == kInt && y.k == kInt {
		// TF keeps integer results only while they fit a 32-bit int;
		// anything larger becomes a real.
		var r int64
		switch op {
		case "+":
			r = x.i + y.i
		case "-":
			r = x.i - y.i
		case "*":
			if x.i == 0 || y.i == 0 {
				return Int(0), nil
			}
			r = x.i * y.i
			if r/y.i != x.i {
				return Real(float64(x.i) * float64(y.i)), nil
			}
		case "/":
			if y.i == 0 {
				return Int(0), errors.New("division by zero")
			}
			return Int(x.i / y.i), nil
		}
		if r > math.MaxInt32 || r < math.MinInt32 {
			xf, yf := float64(x.i), float64(y.i)
			switch op {
			case "+":
				return Real(xf + yf), nil
			case "-":
				return Real(xf - yf), nil
			}
			return Real(xf * yf), nil
		}
		return Int(r), nil
	}
	if (x.k == kInt || x.k == kDec) && (y.k == kInt || y.k == kDec) && (op == "+" || op == "-") {
		if op == "+" {
			return decMicro(new(big.Int).Add(x.microOf(), y.microOf())), nil
		}
		return decMicro(new(big.Int).Sub(x.microOf(), y.microOf())), nil
	}
	xf, yf := a.Float(), b.Float()
	switch op {
	case "+":
		return Real(xf + yf), nil
	case "-":
		return Real(xf - yf), nil
	case "*":
		return Real(xf * yf), nil
	case "/":
		if yf == 0 {
			return Int(0), errors.New("division by zero")
		}
		return Real(xf / yf), nil
	}
	return Int(0), fmt.Errorf("expression: bad operator %s", op)
}
