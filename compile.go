package gtfparser

import (
	"fmt"
	"strings"
)

// Structural check of a statement list, the part of tf's compiler (list() in
// expand.c) that decides whether /if, /while and friends are well formed. A
// macro body or typed line that is not well formed is not run at all: tf
// prints one diagnostic and drops it. The check runs once per distinct text
// (the result is cached next to the split statements) and its diagnostics are
// replayed each time the text runs.

type blockKind int

const (
	bOuter blockKind = iota
	bIf
	bElseif
	bThen
	bElse
	bWhile
	bDo
)

func (b blockKind) label() string {
	switch b {
	case bIf:
		return "IF"
	case bElseif:
		return "ELSEIF"
	case bThen:
		return "THEN"
	case bElse:
		return "ELSE"
	case bWhile:
		return "WHILE"
	case bDo:
		return "DO"
	}
	return ""
}

// blankStmt marks a statement that tf sends as a blank line to the server
// (the statement left empty after "/if (cond)" or "/else" when "%;" follows).
const blankStmt = "\x01"

type compiled struct {
	stmts  []string
	diags  []string // messages, replayed by every run (a "Warning: " prefix is part of the text)
	failed bool
}

var ctlKeywords = map[string]bool{
	"break": true, "do": true, "done": true, "else": true, "elseif": true, "endif": true,
	"if": true, "then": true, "while": true,
}

// specialKeywords are the keywords tf treats as special commands (not pipeable).
var specialKeywords = map[string]bool{
	"break": true, "do": true, "done": true, "else": true, "elseif": true, "endif": true,
	"if": true, "let": true, "result": true, "return": true, "set": true, "setenv": true,
	"test": true, "then": true, "while": true,
}

// foundText is error_text() in expand.c: the token at the start of s.
func foundText(s string) string {
	if s == "" {
		return "end of body"
	}
	end := 1
	c := s[0]
	if isAlnumByte(c) || c == '"' || c == '\'' || c == '`' || c == '/' {
		for end < len(s) && isAlnumByte(s[end]) {
			end++
		}
	}
	return "'" + s[:end] + "'"
}

func (in *Interp) compileStmts(stmts []string, cmdsub bool) *compiled {
	c := &compiled{stmts: stmts}
	var out []string
	cur := bOuter
	var stack []blockKind
	fail := func(f string, a ...any) { c.diags = append(c.diags, fmt.Sprintf(f, a...)); c.failed = true }
	warn := func(f string, a ...any) { c.diags = append(c.diags, "Warning: "+fmt.Sprintf(f, a...)) }
	unexpected := func(kw string) {
		where := cur.label() + " block"
		switch cur {
		case bThen:
			where = "IF body"
		case bDo:
			where = "WHILE body"
		case bOuter:
			where = "outer block"
		}
		fail("unexpected /%s in %s", strings.ToUpper(kw), where)
	}
	endFound := func() string {
		if cmdsub {
			return "')'"
		}
		return "end of body"
	}
	for i := 0; i < len(stmts) && !c.failed; i++ {
		st := stmts[i]
		pipeIn := strings.HasPrefix(st, "\x00")
		st = strings.TrimLeft(strings.TrimPrefix(st, "\x00"), " \t\r\n")
		nextSep := ""
		if i+1 < len(stmts) {
			nextSep = "%;"
			if strings.HasPrefix(stmts[i+1], "\x00") {
				nextSep = "%|"
			}
		}
		out = append(out, stmts[i])
		r := st
		isCmd, special := false, false
		first := true
		var pushedBlank bool
		for {
			r = strings.TrimLeft(r, " \t")
			if r == "" {
				break
			}
			isCmd = false
			if cur == bIf || cur == bElseif || cur == bWhile {
				if r[0] != '/' || strings.HasPrefix(r, "//") {
					if r[0] == '(' {
						p := &exprParser{in: in, s: r[1:], kind: "expression"}
						_, err := p.parseComma()
						if err != nil {
							fail("%v", err)
							break
						}
						p.ws()
						if p.pos >= len(p.s) || p.s[p.pos] != ')' {
							p.kind = "condition"
							fail("%v", p.syntaxErr("operator or ')'"))
							break
						}
						r = strings.TrimLeft(p.s[p.pos+1:], " \t")
						if r == "" && nextSep != "" {
							warn("\"%s\" following \"/%s (...)\" sends blank line to server, which is probably not what was intended.", nextSep, cur.label())
							pushedBlank = true
						}
						if cur == bWhile {
							cur = bDo
						} else {
							cur = bThen
						}
						first = false
						continue
					}
					warn("statement starting with %s in /%s condition sends text to server, which is probably not what was intended.", foundText(r), cur.label())
				}
			}
			if r[0] == '/' && !strings.HasPrefix(r, "//") {
				isCmd = true
				kw, rest := kwSplit(r[1:])
				lk := strings.ToLower(kw)
				if specialKeywords[lk] {
					special = true
				}
				old := cur
				switch lk {
				case "if", "while":
					stack = append(stack, cur)
					if lk == "if" {
						cur = bIf
					} else {
						cur = bWhile
					}
					r = rest
					first = false
					continue
				case "then":
					if old != bIf && old != bElseif {
						unexpected(lk)
						break
					}
					cur = bThen
					r, first = rest, false
					continue
				case "do":
					if old != bWhile {
						unexpected(lk)
						break
					}
					cur = bDo
					r, first = rest, false
					continue
				case "else", "elseif":
					if old != bThen {
						unexpected(lk)
						break
					}
					if lk == "else" {
						cur = bElse
						if strings.TrimSpace(rest) == "" && nextSep != "" {
							warn("\"%s\" following \"/ELSE\" sends blank line to server, which is probably not what was intended.", nextSep)
							pushedBlank = true
						}
					} else {
						cur = bElseif
					}
					r, first = rest, false
					continue
				case "endif", "done":
					if (lk == "endif" && old != bThen && old != bElse) || (lk == "done" && old != bDo) {
						unexpected(lk)
						break
					}
					cur = stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					rest = strings.TrimLeft(rest, " \t")
					if rest != "" {
						fail("macro syntax error: expected end of statement after /%s, found %s.  (Possibly missing '%%;')", lk, foundText(rest))
					}
					r = ""
					continue
				case "break":
					rest = strings.TrimLeft(rest, " \t")
					if rest != "" {
						j := 0
						if rest[0] == '+' || rest[0] == '-' {
							j++
						}
						k := j
						for k < len(rest) && rest[k] >= '0' && rest[k] <= '9' {
							k++
						}
						if k == j || strings.TrimLeft(rest[:k], "+-0") == "" {
							fail("/BREAK syntax error: expected positive integer, found %s.", foundText(rest))
							break
						}
						rest = rest[k:]
					}
					r, first = rest, false
					continue
				case "return", "result":
					if cmdsub {
						fail("/%s may be called only directly from a macro, not in $() command substitution.", strings.ToUpper(lk))
					}
				}
				if c.failed {
					break
				}
			} else if pipeIn && first {
				fail("Piping input to a server command is not allowed.")
				break
			}
			// an ordinary statement runs to the end of the list element
			if !c.failed {
				c.diags = append(c.diags, in.cmdsubDiags(r)...)
				if len(c.diags) > 0 && strings.HasPrefix(c.diags[len(c.diags)-1], "unmatched (") {
					c.failed = true
				}
			}
			r = ""
			break
		}
		if c.failed {
			break
		}
		if pushedBlank {
			out = append(out, blankStmt)
		}
		if nextSep == "%|" {
			if !isCmd {
				fail("Piping output of a server command is not allowed.")
			} else if special {
				fail("Piping output of a special command is not allowed.")
			}
		}
	}
	if !c.failed {
		if n := len(stmts); n > 0 && strings.HasPrefix(stmts[n-1], "\x00") && strings.TrimSpace(strings.TrimPrefix(stmts[n-1], "\x00")) == "" {
			fail("'%%|' must be followed by another command.")
		}
	}
	if !c.failed && cur != bOuter {
		exp := "/endif"
		switch cur {
		case bIf, bElseif:
			exp = "/then"
		case bWhile:
			exp = "/do"
		case bDo:
			exp = "/done"
		}
		fail("macro syntax error: expected %s, found %s.", exp, endFound())
	}
	c.stmts = out
	return c
}

// kwSplit cuts the keyword (up to a blank, '%' or ')') from the text after
// a statement's slash.
func kwSplit(s string) (kw, rest string) {
	i := 0
	for i < len(s) && s[i] != ' ' && s[i] != '\t' && s[i] != '%' && s[i] != ')' && s[i] != '(' {
		i++
	}
	return s[:i], s[i:]
}

// cmdsubDiags checks the $( ... ) substitutions of a statement.
func (in *Interp) cmdsubDiags(s string) []string {
	var out []string
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '$':
			if i+1 < len(s) && s[i+1] == '(' {
				end := matchCloseCmd(s, i+1)
				if end < 0 {
					return out
				}
				inner := s[i+2 : end]
				sub := in.compileStmts(splitList(inner), true)
				out = append(out, sub.diags...)
				if sub.failed {
					out = append(out, "unmatched (")
					return out
				}
				i = end
			}
		}
	}
	return out
}
