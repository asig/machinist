/*
 * This file is part of Machinist.
 *
 * Copyright (C) 2026 Andreas Signer <asigner@gmail.com>
 *
 * Machinist is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * Machinist is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with Machinist.  If not, see <https://www.gnu.org/licenses/>.
 */

// Package expr implements the small expression language used inside
// ${...} placeholders of configuration strings.
//
// Grammar:
//
//	expr    = term { ("+" | "-") term }
//	term    = unary { ("*" | "/" | "%") unary }
//	unary   = "-" unary | postfix
//	postfix = primary { "." name | "[" expr "]" }
//	primary = int | string | "true" | "false" | list | name | "(" expr ")"
//	list    = "[" [ expr { "," expr } ] "]"
//
// Values are int64, string, bool, []any (lists) and map[string]any (objects,
// only reachable through variables). Booleans stringify as "on" and "off".
// "+" adds ints, concatenates a string with
// a scalar, concatenates two lists and appends a scalar to a list.
package expr

import (
	"fmt"
	"strconv"
	"strings"
)

// Eval expands all ${...} placeholders in template using env for variable
// lookup. A template that consists of exactly one placeholder returns the
// typed value of the expression. Any other template returns a string. The
// sequence "$${" produces a literal "${".
func Eval(template string, env map[string]any) (any, error) {
	if !strings.Contains(template, "${") {
		return template, nil
	}

	var (
		literal strings.Builder
		parts   []any
		exprs   int
	)
	for i := 0; i < len(template); {
		switch {
		case strings.HasPrefix(template[i:], "$${"):
			literal.WriteString("${")
			i += 3
		case strings.HasPrefix(template[i:], "${"):
			p := &parser{s: template, pos: i + 2, env: env}
			value, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			p.skipSpace()
			if !p.consume('}') {
				return nil, p.errorf("expected \"}\"")
			}
			if literal.Len() > 0 {
				parts = append(parts, literal.String())
				literal.Reset()
			}
			parts = append(parts, value)
			exprs++
			i = p.pos
		default:
			literal.WriteByte(template[i])
			i++
		}
	}

	if exprs == 1 && literal.Len() == 0 && len(parts) == 1 {
		return parts[0], nil
	}
	if literal.Len() > 0 {
		parts = append(parts, literal.String())
	}
	var out strings.Builder
	for _, part := range parts {
		text, err := stringify(part)
		if err != nil {
			return nil, err
		}
		out.WriteString(text)
	}
	return out.String(), nil
}

// Stringify converts a scalar value to its string form. Booleans become
// "on" and "off", as in property serialization.
func Stringify(v any) (string, error) { return stringify(v) }

func stringify(v any) (string, error) {
	switch v := v.(type) {
	case string:
		return v, nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case bool:
		// Same mapping as QEMU property serialization (spec 3.4).
		if v {
			return "on", nil
		}
		return "off", nil
	default:
		return "", fmt.Errorf("cannot use %s as a string", typeName(v))
	}
}

func typeName(v any) string {
	switch v.(type) {
	case string:
		return "string"
	case int64:
		return "integer"
	case bool:
		return "boolean"
	case []any:
		return "list"
	case map[string]any:
		return "object"
	default:
		return fmt.Sprintf("%T", v)
	}
}

type parser struct {
	s   string
	pos int
	env map[string]any
}

// Error is a syntax or evaluation error at a position in a template.
type Error struct {
	Template string
	// Column is the 1-based byte offset in Template.
	Column int
	Msg    string
}

func (e *Error) Error() string {
	return fmt.Sprintf("expression %q: column %d: %s", e.Template, e.Column, e.Msg)
}

func (p *parser) errorf(format string, args ...any) error {
	return &Error{Template: p.s, Column: p.pos + 1, Msg: fmt.Sprintf(format, args...)}
}

func (p *parser) skipSpace() {
	for p.pos < len(p.s) && (p.s[p.pos] == ' ' || p.s[p.pos] == '\t') {
		p.pos++
	}
}

func (p *parser) consume(c byte) bool {
	if p.pos < len(p.s) && p.s[p.pos] == c {
		p.pos++
		return true
	}
	return false
}

func (p *parser) parseExpr() (any, error) {
	left, err := p.parseTerm()
	if err != nil {
		return nil, err
	}
	for {
		p.skipSpace()
		switch {
		case p.consume('+'):
			right, err := p.parseTerm()
			if err != nil {
				return nil, err
			}
			if left, err = add(left, right); err != nil {
				return nil, p.errorf("%v", err)
			}
		case p.consume('-'):
			right, err := p.parseTerm()
			if err != nil {
				return nil, err
			}
			if left, err = arith('-', left, right); err != nil {
				return nil, p.errorf("%v", err)
			}
		default:
			return left, nil
		}
	}
}

func (p *parser) parseTerm() (any, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		p.skipSpace()
		if p.pos >= len(p.s) || !strings.ContainsRune("*/%", rune(p.s[p.pos])) {
			return left, nil
		}
		op := p.s[p.pos]
		p.pos++
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		if left, err = arith(op, left, right); err != nil {
			return nil, p.errorf("%v", err)
		}
	}
}

func (p *parser) parseUnary() (any, error) {
	p.skipSpace()
	if p.consume('-') {
		value, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		n, ok := value.(int64)
		if !ok {
			return nil, p.errorf("cannot negate %s", typeName(value))
		}
		return -n, nil
	}
	return p.parsePostfix()
}

func (p *parser) parsePostfix() (any, error) {
	value, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for {
		switch {
		case p.consume('.'):
			name, err := p.parseName()
			if err != nil {
				return nil, err
			}
			if value, err = p.field(value, name); err != nil {
				return nil, err
			}
		case p.consume('['):
			index, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			p.skipSpace()
			if !p.consume(']') {
				return nil, p.errorf("expected \"]\"")
			}
			if value, err = p.index(value, index); err != nil {
				return nil, err
			}
		default:
			return value, nil
		}
	}
}

func (p *parser) field(value any, name string) (any, error) {
	object, ok := value.(map[string]any)
	if !ok {
		return nil, p.errorf("cannot read property %q of %s", name, typeName(value))
	}
	result, ok := object[name]
	if !ok {
		return nil, p.errorf("unknown property %q", name)
	}
	return result, nil
}

func (p *parser) index(value, index any) (any, error) {
	switch value := value.(type) {
	case []any:
		n, ok := index.(int64)
		if !ok {
			return nil, p.errorf("list index must be an integer, got %s", typeName(index))
		}
		if n < 0 || n >= int64(len(value)) {
			return nil, p.errorf("list index %d out of range (length %d)", n, len(value))
		}
		return value[n], nil
	case map[string]any:
		key, ok := index.(string)
		if !ok {
			return nil, p.errorf("object key must be a string, got %s", typeName(index))
		}
		return p.field(value, key)
	default:
		return nil, p.errorf("cannot index %s", typeName(value))
	}
}

func (p *parser) parseName() (string, error) {
	start := p.pos
	for p.pos < len(p.s) {
		c := p.s[p.pos]
		if c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' && p.pos > start {
			p.pos++
			continue
		}
		break
	}
	if p.pos == start {
		return "", p.errorf("expected a name")
	}
	return p.s[start:p.pos], nil
}

func (p *parser) parsePrimary() (any, error) {
	p.skipSpace()
	if p.pos >= len(p.s) {
		return nil, p.errorf("unexpected end of expression")
	}
	c := p.s[p.pos]
	switch {
	case c >= '0' && c <= '9':
		start := p.pos
		for p.pos < len(p.s) && p.s[p.pos] >= '0' && p.s[p.pos] <= '9' {
			p.pos++
		}
		n, err := strconv.ParseInt(p.s[start:p.pos], 10, 64)
		if err != nil {
			return nil, p.errorf("invalid integer: %v", err)
		}
		return n, nil
	case c == '\'' || c == '"':
		return p.parseString(c)
	case c == '(':
		p.pos++
		value, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		p.skipSpace()
		if !p.consume(')') {
			return nil, p.errorf("expected \")\"")
		}
		return value, nil
	case c == '[':
		return p.parseList()
	}

	name, err := p.parseName()
	if err != nil {
		return nil, err
	}
	switch name {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}
	value, ok := p.env[name]
	if !ok {
		return nil, p.errorf("unknown variable %q", name)
	}
	return value, nil
}

func (p *parser) parseString(quote byte) (any, error) {
	p.pos++
	var out strings.Builder
	for p.pos < len(p.s) {
		c := p.s[p.pos]
		switch {
		case c == quote:
			p.pos++
			return out.String(), nil
		case c == '\\' && p.pos+1 < len(p.s):
			out.WriteByte(p.s[p.pos+1])
			p.pos += 2
		default:
			out.WriteByte(c)
			p.pos++
		}
	}
	return nil, p.errorf("unterminated string")
}

func (p *parser) parseList() (any, error) {
	p.pos++
	list := []any{}
	p.skipSpace()
	if p.consume(']') {
		return list, nil
	}
	for {
		value, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		list = append(list, value)
		p.skipSpace()
		if p.consume(']') {
			return list, nil
		}
		if !p.consume(',') {
			return nil, p.errorf("expected \",\" or \"]\"")
		}
	}
}

func add(a, b any) (any, error) {
	if x, ok := a.(int64); ok {
		if y, ok := b.(int64); ok {
			return x + y, nil
		}
	}
	if list, ok := a.([]any); ok {
		out := append([]any(nil), list...)
		if other, ok := b.([]any); ok {
			return append(out, other...), nil
		}
		if _, ok := b.(map[string]any); ok {
			return nil, fmt.Errorf("cannot append object to list")
		}
		return append(out, b), nil
	}
	x, err := stringify(a)
	if err != nil {
		return nil, err
	}
	y, err := stringify(b)
	if err != nil {
		return nil, err
	}
	return x + y, nil
}

func arith(op byte, a, b any) (any, error) {
	x, okA := a.(int64)
	y, okB := b.(int64)
	if !okA || !okB {
		return nil, fmt.Errorf("operator %q needs integers, got %s and %s", string(op), typeName(a), typeName(b))
	}
	switch op {
	case '-':
		return x - y, nil
	case '*':
		return x * y, nil
	}
	if y == 0 {
		return nil, fmt.Errorf("division by zero")
	}
	if op == '/' {
		return x / y, nil
	}
	return x % y, nil
}
