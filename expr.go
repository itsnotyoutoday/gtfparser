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
		a, err = p.parseComma()
		if err != nil {
			return nil, err
		}
	}
	if !p.take(":") {
		return nil, errors.New("expression: expected ':' in conditional")
	}
	b, err := p.parseTernary()
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
	start := p.pos
	l, err := next()
	if err != nil {
		return nil, err
	}
	// a bare variable is read when the operator runs, not when it is parsed
	// (tf pushes identifiers and looks them up in reduce(): y - --y is 0)
	lazy := p.bareIdent(start, p.pos)
	for {
		var op string
		p.ws()
		for _, o := range ops {
			if strings.HasPrefix(p.s[p.pos:], o) {
				if (o == "+" || o == "-" || o == "*" || o == "/") && p.pos+1 < len(p.s) && p.s[p.pos+1] == '=' {
					continue // "+=" and friends are assignments
				}
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
		l0, op0, id := l, op, lazy
		l = func() (Value, error) {
			var a Value
			var err error
			if id == "" {
				a, err = l0()
				if err != nil {
					return a, err
				}
			}
			b, err := r()
			if err != nil {
				return b, err
			}
			if id != "" {
				a, _ = p.in.lookup(id)
			}
			return binary(op0, a, b)
		}
		lazy = ""
	}
}

// bareIdent reports the identifier when s[start:end] is just a variable name.
func (p *exprParser) bareIdent(start, end int) string {
	t := strings.TrimSpace(p.s[start:end])
	if t == "" || !isIdentStart(t[0]) {
		return ""
	}
	for i := 1; i < len(t); i++ {
		if !isIdentChar(t[i]) {
			return ""
		}
	}
	return t
}

// Relational and equality operators are one left-associative level in tf's
// expression parser (relational_expr in expr.c).
func (p *exprParser) parseRel() (eval, error) {
	// longest operators first so "=~" and "==" are not read as "="
	return p.binLevel([]string{"==", "!=", "=~", "!~", "=/", "!/", "<=", ">=", "<", ">", "="}, p.parseAdd)
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
		start := p.pos
		x, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		id := p.bareIdent(start, p.pos)
		if id == "" {
			return func() (Value, error) { return Int(0), errors.New("illegal object of assignment") }, nil
		}
		_ = x
		return func() (Value, error) {
			cur, _ := p.in.lookup(id)
			// tf truncates the variable to an integer first (++f on 1.5 gives
			// 2); gtfparser adds one to the real value
			r, err := reduce(op[:1], cur, Int(1), true)
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
	case p.peek("-") || p.peek("+"):
		op := p.s[p.pos : p.pos+1]
		p.pos++
		x, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return func() (Value, error) {
			v, err := x()
			if err != nil {
				return v, err
			}
			return reduce(op, v, Int(0), false)
		}, nil
	}
	return p.parsePrimaryCall()
}

// parsePrimaryCall is a primary expression plus the function-call suffix
// (tf allows blanks between the name and the parenthesis).
func (p *exprParser) parsePrimaryCall() (eval, error) {
	p.ws()
	if p.pos < len(p.s) && isIdentStart(p.s[p.pos]) {
		save := p.pos
		id, _ := p.ident()
		p.ws()
		if p.pos < len(p.s) && p.s[p.pos] == '(' {
			return p.call(id)
		}
		p.pos = save
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
	if ar, ok := funcArity[name]; ok {
		if _, isBuiltin := p.in.funcs[name]; isBuiltin && (len(args) < ar[0] || (ar[1] >= 0 && len(args) > ar[1])) {
			var msg string
			switch {
			case ar[0] == ar[1]:
				msg = fmt.Sprintf("%s: found %d arguments, expected %d", name, len(args), ar[0])
			case ar[1] < 0:
				msg = fmt.Sprintf("%s: found %d arguments, expected at least %d", name, len(args), ar[0])
			default:
				msg = fmt.Sprintf("%s: found %d arguments, expected between %d and %d", name, len(args), ar[0], ar[1])
			}
			return nil, errors.New(msg)
		}
	} else if _, isFn := p.in.funcs[name]; !isFn && len(args) > 1 {
		if _, isCmd := p.in.cmds[strings.ToLower(name)]; isCmd && p.in.isTfCommand(name) {
			return nil, fmt.Errorf("%s: command called as function must have 0 or 1 argument", name)
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
		xf, yf := x.Float(), y.Float()
		switch {
		case xf < yf:
			c = -1
		case xf > yf:
			c = 1
		case xf != yf: // NaN: every comparison but != is false
			if op == "!=" {
				return Int(1), nil
			}
			return Int(0), nil
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

// arith applies an arithmetic operator the way reduce_arithmetic() in tf's
// expr.c does. Integer addition and subtraction work on 32-bit ints (an
// overflow promotes the result to a float), integer multiplication is exact
// only while the result fits 32 bits, and the integer divisor is truncated to
// 32 bits (so dividing by 2^32 is a division by zero).
func arith(op string, a, b Value) (Value, error) { return reduce(op, a, b, true) }

// reduce is arith with an optional right operand: unary -/+ pass binary=false
// and the single operand in a.
func reduce(op string, a, b Value, binary bool) (Value, error) {
	x, y := a.Num(), b.Num()
	last := y
	if !binary {
		last = x
	}
	// tf's promotion order is INT < DECIMAL (time) < FLOAT
	rank := func(k kind) int {
		switch k {
		case kInt:
			return 0
		case kDec:
			return 1
		}
		return 2
	}
	pr := rank(x.k)
	if binary && rank(y.k) > pr {
		pr = rank(y.k)
	}
	if pr < 2 && (op == "+" || op == "-") {
		// tf computes this in 32-bit ints (so 1+4294967296 is 1); gtfparser
		// uses the exact value and promotes to a float when it does not
		// fit 32 bits (see docs/deliberate-differences.md)
		var a0 int64
		if binary {
			a0 = x.Int()
		}
		a1 := last.Int()
		var sum int64
		ok := true
		if op == "-" {
			sum = a0 - a1
			ok = !(a1 > 0 && a0 < math.MinInt64+a1) && !(a1 < 0 && a0 > math.MaxInt64+a1)
		} else {
			sum = a0 + a1
			ok = !(a1 > 0 && a0 > math.MaxInt64-a1) && !(a1 < 0 && a0 < math.MinInt64-a1)
		}
		if !ok || (pr == 0 && (sum > math.MaxInt32 || sum < math.MinInt32)) {
			pr = 2
		} else if pr == 0 {
			return Int(sum), nil
		}
	}
	switch pr {
	case 0:
		switch op {
		case "*":
			r := x.i * y.i
			if x.i == 0 || y.i == 0 || (r/y.i == x.i && r >= math.MinInt32 && r <= math.MaxInt32) {
				return Int(r), nil
			}
			return Real(float64(x.i) * float64(y.i)), nil
		case "/":
			if y.i == 0 {
				return Int(0), errors.New("division by zero")
			}
			if x.i == math.MinInt64 && y.i == -1 {
				return Real(-float64(x.i)), nil
			}
			return Int(x.i / y.i), nil
		}
	case 1:
		tt := x.tt
		if binary && y.tt > tt {
			tt = y.tt
		}
		switch op {
		case "+":
			if !binary {
				return timeMicro(x.microOf(), tt), nil
			}
			return timeMicro(new(big.Int).Add(x.microOf(), y.microOf()), tt), nil
		case "-":
			if !binary {
				return timeMicro(new(big.Int).Neg(x.microOf()), tt), nil
			}
			if x.tt == ttAtime && y.tt == ttAtime {
				tt = ttDtime
			}
			return timeMicro(new(big.Int).Sub(x.microOf(), y.microOf()), tt), nil
		}
	}
	xf := x.Float()
	yf := last.Float()
	var r float64
	switch op {
	case "+":
		r = xf
		if binary {
			r = xf + yf
		}
	case "-":
		if binary {
			r = xf - yf
		} else {
			r = -xf
		}
	case "*":
		r = xf * yf
	case "/":
		r = xf / yf
	default:
		return Int(0), fmt.Errorf("expression: bad operator %s", op)
	}
	if math.IsInf(r, 0) {
		return Int(0), fmt.Errorf("%s operator: arithmetic overflow", op)
	}
	return Real(r), nil
}
