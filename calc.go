package main

import (
	"math"
	"strconv"
	"strings"
)

// evaluate answers simple arithmetic in the search box. Flow Launcher does
// this through a plugin; here it is the parser below, which understands
// + - * / % ^ ( ), unary minus and five functions, and never calls anything
// that can fail at runtime.
func evaluate(expr string) (string, bool) {
	src := strings.ReplaceAll(expr, ",", ".")
	// Ignore strings that are clearly program names, not sums.
	if !looksArithmetic(src) {
		return "", false
	}
	p := &parser{src: src}
	v, ok := p.expr(0)
	if !ok || p.pos < len(p.src) {
		return "", false
	}
	if math.IsInf(v, 0) || math.IsNaN(v) {
		return "", false
	}
	if math.Abs(v-math.Round(v)) < 1e-10 && math.Abs(v) < 1e15 {
		return formatNumber(v), true
	}
	return strconv.FormatFloat(v, 'g', 12, 64), true
}

func formatNumber(v float64) string {
	neg := v < 0
	i := int64(math.Round(v))
	if neg {
		i = -i
	}
	s := strconv.FormatInt(i, 10)
	// group thousands with a thin space, the way users read numbers
	var b strings.Builder
	if neg {
		b.WriteString("-")
	}
	for idx, c := range s {
		if idx > 0 && (len(s)-idx)%3 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(c)
	}
	return b.String()
}

func looksArithmetic(s string) bool {
	hasDigit := false
	hasOp := false
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			hasDigit = true
		case strings.ContainsRune("+-*/%^", r):
			if strings.ContainsRune("+-*/%^", r) {
				hasOp = true
			}
		case r >= 'a' && r <= 'z' || r == '.' || r == ' ' || r == '(' || r == ')':
		default:
			return false
		}
	}
	return hasDigit && (hasOp || strings.Contains(s, "("))
}

type parser struct {
	src string
	pos int
}

func (p *parser) skip() {
	for p.pos < len(p.src) && p.src[p.pos] == ' ' {
		p.pos++
	}
}

// expr implements the usual precedence climbing: term (('+'|'-') term)*.
func (p *parser) expr(minPrec int) (float64, bool) {
	left, ok := p.atom()
	if !ok {
		return 0, false
	}
	for {
		p.skip()
		if p.pos >= len(p.src) {
			return left, true
		}
		op := p.src[p.pos]
		prec := precedence(op)
		if prec == 0 || prec < minPrec {
			return left, true
		}
		p.pos++
		right, ok := p.expr(prec + 1)
		if !ok {
			return 0, false
		}
		switch op {
		case '+':
			left += right
		case '-':
			left -= right
		case '*':
			left *= right
		case '/':
			if right == 0 {
				return 0, false
			}
			left /= right
		case '%':
			if right == 0 {
				return 0, false
			}
			left = math.Mod(left, right)
		case '^':
			left = math.Pow(left, right)
		}
	}
}

func precedence(op byte) int {
	switch op {
	case '+', '-':
		return 1
	case '*', '/', '%':
		return 2
	case '^':
		return 3
	}
	return 0
}

func (p *parser) atom() (float64, bool) {
	p.skip()
	if p.pos >= len(p.src) {
		return 0, false
	}
	c := p.src[p.pos]
	if c == '-' {
		p.pos++
		v, ok := p.atom()
		return -v, ok
	}
	if c == '+' {
		p.pos++
		return p.atom()
	}
	if c == '(' {
		p.pos++
		v, ok := p.expr(1)
		p.skip()
		if !ok || p.pos >= len(p.src) || p.src[p.pos] != ')' {
			return 0, false
		}
		p.pos++
		return v, true
	}
	if c == '=' {
		p.pos++
		return p.atom()
	}
	if c >= 'a' && c <= 'z' {
		start := p.pos
		for p.pos < len(p.src) && p.src[p.pos] >= 'a' && p.src[p.pos] <= 'z' {
			p.pos++
		}
		name := p.src[start:p.pos]
		p.skip()
		if p.pos >= len(p.src) || p.src[p.pos] != '(' {
			return 0, false
		}
		p.pos++
		v, ok := p.expr(1)
		p.skip()
		if !ok || p.pos >= len(p.src) || p.src[p.pos] != ')' {
			return 0, false
		}
		p.pos++
		return applyFn(name, v)
	}
	start := p.pos
	for p.pos < len(p.src) && (p.src[p.pos] >= '0' && p.src[p.pos] <= '9' || p.src[p.pos] == '.') {
		p.pos++
	}
	if start == p.pos {
		return 0, false
	}
	v, err := strconv.ParseFloat(p.src[start:p.pos], 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func applyFn(name string, v float64) (float64, bool) {
	switch name {
	case "sqrt":
		if v < 0 {
			return 0, false
		}
		return math.Sqrt(v), true
	case "sin":
		return math.Sin(v), true
	case "cos":
		return math.Cos(v), true
	case "tan":
		return math.Tan(v), true
	case "abs":
		return math.Abs(v), true
	case "ln":
		if v <= 0 {
			return 0, false
		}
		return math.Log(v), true
	case "log":
		if v <= 0 {
			return 0, false
		}
		return math.Log10(v), true
	}
	return 0, false
}
