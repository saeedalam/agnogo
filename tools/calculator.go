// Package tools provides built-in tools for agnogo agents.
// Each tool is a standalone function that can be registered with agent.Tool().
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/saeedalam/agnogo"
)

// CalculatorConfig configures the expression-based calculator.
type CalculatorConfig struct {
	// MaxExpressionLen limits the length of the expression string.
	// Default: 1024
	MaxExpressionLen int
	// MaxDepth limits recursion depth of the parser to prevent stack overflow.
	// Default: 100
	MaxDepth int
}

func (c *CalculatorConfig) defaults() {
	if c.MaxExpressionLen <= 0 {
		c.MaxExpressionLen = 1024
	}
	if c.MaxDepth <= 0 {
		c.MaxDepth = 100
	}
}

// Calculator returns tools for math evaluation.
// Supports a full expression parser with operator precedence and functions.
// Backward compatible: if "operation" param is present, uses the legacy switch logic.
func Calculator(cfgs ...CalculatorConfig) []agnogo.ToolDef {
	var cfg CalculatorConfig
	if len(cfgs) > 0 {
		cfg = cfgs[0]
	}
	cfg.defaults()

	return []agnogo.ToolDef{
		{
			Name: "calculator",
			Desc: "Evaluate a math expression. Supports +, -, *, /, %, ^ (power), parentheses, unary minus, and functions: sqrt, abs, sin, cos, log, ceil, floor, round, pow. Example: '2 + 3 * (4 - 1)'. Legacy mode: pass 'operation' and 'a'/'b' params.",
			Params: agnogo.Params{
				"expression": {Type: "string", Desc: "Math expression to evaluate (e.g. '2 + 3 * (4 - 1)')"},
				"operation":  {Type: "string", Desc: "Legacy: add, subtract, multiply, divide, sqrt, power, factorial"},
				"a":          {Type: "number", Desc: "Legacy: first number"},
				"b":          {Type: "number", Desc: "Legacy: second number"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				// Context cancellation check
				if err := ctx.Err(); err != nil {
					return "", fmt.Errorf("context cancelled: %w", err)
				}

				// Legacy mode: if "operation" param is present, use old logic
				if op := args["operation"]; op != "" {
					return calcLegacy(op, args["a"], args["b"])
				}

				expr := strings.TrimSpace(args["expression"])
				if expr == "" {
					return "", fmt.Errorf("missing required parameter: 'expression' (or use legacy 'operation'+'a'+'b' params)")
				}
				if len(expr) > cfg.MaxExpressionLen {
					return "", fmt.Errorf("expression too long: %d chars (max %d)", len(expr), cfg.MaxExpressionLen)
				}

				result, err := parseExpression(expr, cfg.MaxDepth)
				if err != nil {
					return "", fmt.Errorf("parse error: %w", err)
				}

				out, _ := json.Marshal(map[string]any{
					"expression": expr,
					"result":     result,
				})
				return string(out), nil
			},
		},
	}
}

// calcLegacy handles the old operation+a+b style calls for backward compatibility.
func calcLegacy(op, aStr, bStr string) (string, error) {
	a, err := strconv.ParseFloat(aStr, 64)
	if err != nil && op != "" {
		return "", fmt.Errorf("invalid number for 'a': %q", aStr)
	}
	b, _ := strconv.ParseFloat(bStr, 64)

	var result float64
	switch op {
	case "add":
		result = a + b
	case "subtract":
		result = a - b
	case "multiply":
		result = a * b
	case "divide":
		if b == 0 {
			return "", fmt.Errorf("division by zero")
		}
		result = a / b
	case "sqrt":
		if a < 0 {
			return "", fmt.Errorf("cannot take square root of negative number: %g", a)
		}
		result = math.Sqrt(a)
	case "power":
		result = math.Pow(a, b)
	case "factorial":
		if a < 0 || a > 20 || a != math.Trunc(a) {
			return "", fmt.Errorf("factorial requires integer 0-20, got %g", a)
		}
		result = 1
		for i := 2; i <= int(a); i++ {
			result *= float64(i)
		}
	default:
		return "", fmt.Errorf("unknown operation: %q (supported: add, subtract, multiply, divide, sqrt, power, factorial)", op)
	}

	r, _ := json.Marshal(map[string]any{"operation": op, "result": result})
	return string(r), nil
}

// --- Recursive descent expression parser ---
// Grammar:
//   expr     = term (('+' | '-') term)*
//   term     = power (('*' | '/' | '%') power)*
//   power    = unary ('^' power)?          // right-associative
//   unary    = ('-' | '+') unary | call
//   call     = IDENT '(' expr (',' expr)* ')' | primary
//   primary  = NUMBER | '(' expr ')'

type exprParser struct {
	input    string
	pos      int
	depth    int
	maxDepth int
}

func parseExpression(input string, maxDepth int) (float64, error) {
	p := &exprParser{input: input, maxDepth: maxDepth}
	p.skipWhitespace()
	result, err := p.parseExpr()
	if err != nil {
		return 0, err
	}
	p.skipWhitespace()
	if p.pos < len(p.input) {
		return 0, fmt.Errorf("unexpected character at position %d: %q", p.pos, string(p.input[p.pos]))
	}
	if math.IsInf(result, 0) || math.IsNaN(result) {
		return 0, fmt.Errorf("result is not a finite number")
	}
	return result, nil
}

func (p *exprParser) enter() error {
	p.depth++
	if p.depth > p.maxDepth {
		return fmt.Errorf("expression too deeply nested (max depth %d)", p.maxDepth)
	}
	return nil
}

func (p *exprParser) leave() { p.depth-- }

func (p *exprParser) skipWhitespace() {
	for p.pos < len(p.input) && unicode.IsSpace(rune(p.input[p.pos])) {
		p.pos++
	}
}

func (p *exprParser) peek() byte {
	if p.pos < len(p.input) {
		return p.input[p.pos]
	}
	return 0
}

func (p *exprParser) parseExpr() (float64, error) {
	if err := p.enter(); err != nil {
		return 0, err
	}
	defer p.leave()

	left, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for {
		p.skipWhitespace()
		ch := p.peek()
		if ch != '+' && ch != '-' {
			break
		}
		p.pos++
		right, err := p.parseTerm()
		if err != nil {
			return 0, err
		}
		if ch == '+' {
			left += right
		} else {
			left -= right
		}
	}
	return left, nil
}

func (p *exprParser) parseTerm() (float64, error) {
	if err := p.enter(); err != nil {
		return 0, err
	}
	defer p.leave()

	left, err := p.parsePower()
	if err != nil {
		return 0, err
	}
	for {
		p.skipWhitespace()
		ch := p.peek()
		if ch != '*' && ch != '/' && ch != '%' {
			break
		}
		p.pos++
		right, err := p.parsePower()
		if err != nil {
			return 0, err
		}
		switch ch {
		case '*':
			left *= right
		case '/':
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			left /= right
		case '%':
			if right == 0 {
				return 0, fmt.Errorf("modulo by zero")
			}
			left = math.Mod(left, right)
		}
	}
	return left, nil
}

func (p *exprParser) parsePower() (float64, error) {
	if err := p.enter(); err != nil {
		return 0, err
	}
	defer p.leave()

	base, err := p.parseUnary()
	if err != nil {
		return 0, err
	}
	p.skipWhitespace()
	if p.peek() == '^' {
		p.pos++
		// Right-associative: parse recursively
		exp, err := p.parsePower()
		if err != nil {
			return 0, err
		}
		return math.Pow(base, exp), nil
	}
	return base, nil
}

func (p *exprParser) parseUnary() (float64, error) {
	if err := p.enter(); err != nil {
		return 0, err
	}
	defer p.leave()

	p.skipWhitespace()
	ch := p.peek()
	if ch == '-' {
		p.pos++
		val, err := p.parseUnary()
		if err != nil {
			return 0, err
		}
		return -val, nil
	}
	if ch == '+' {
		p.pos++
		return p.parseUnary()
	}
	return p.parseCall()
}

func (p *exprParser) parseCall() (float64, error) {
	if err := p.enter(); err != nil {
		return 0, err
	}
	defer p.leave()

	// Check if we have an identifier (function name)
	p.skipWhitespace()
	start := p.pos
	for p.pos < len(p.input) && (unicode.IsLetter(rune(p.input[p.pos])) || p.input[p.pos] == '_') {
		p.pos++
	}
	if p.pos > start {
		name := strings.ToLower(p.input[start:p.pos])
		p.skipWhitespace()
		if p.peek() == '(' {
			p.pos++ // consume '('
			args, err := p.parseArgList()
			if err != nil {
				return 0, err
			}
			return evalFunc(name, args)
		}
		// Not a function call — check if it's a known constant
		switch name {
		case "pi":
			return math.Pi, nil
		case "e":
			return math.E, nil
		default:
			return 0, fmt.Errorf("unknown identifier: %q", name)
		}
	}

	return p.parsePrimary()
}

func (p *exprParser) parseArgList() ([]float64, error) {
	var args []float64
	p.skipWhitespace()
	if p.peek() == ')' {
		p.pos++
		return args, nil
	}
	for {
		val, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		args = append(args, val)
		p.skipWhitespace()
		if p.peek() == ',' {
			p.pos++
			continue
		}
		if p.peek() == ')' {
			p.pos++
			return args, nil
		}
		return nil, fmt.Errorf("expected ',' or ')' in function arguments at position %d", p.pos)
	}
}

func (p *exprParser) parsePrimary() (float64, error) {
	if err := p.enter(); err != nil {
		return 0, err
	}
	defer p.leave()

	p.skipWhitespace()

	// Parenthesized expression
	if p.peek() == '(' {
		p.pos++
		val, err := p.parseExpr()
		if err != nil {
			return 0, err
		}
		p.skipWhitespace()
		if p.peek() != ')' {
			return 0, fmt.Errorf("expected ')' at position %d", p.pos)
		}
		p.pos++
		return val, nil
	}

	// Number
	return p.parseNumber()
}

func (p *exprParser) parseNumber() (float64, error) {
	p.skipWhitespace()
	start := p.pos
	// Optional leading sign is handled by parseUnary, not here
	for p.pos < len(p.input) && (p.input[p.pos] >= '0' && p.input[p.pos] <= '9') {
		p.pos++
	}
	if p.pos < len(p.input) && p.input[p.pos] == '.' {
		p.pos++
		for p.pos < len(p.input) && (p.input[p.pos] >= '0' && p.input[p.pos] <= '9') {
			p.pos++
		}
	}
	// Scientific notation
	if p.pos < len(p.input) && (p.input[p.pos] == 'e' || p.input[p.pos] == 'E') {
		p.pos++
		if p.pos < len(p.input) && (p.input[p.pos] == '+' || p.input[p.pos] == '-') {
			p.pos++
		}
		for p.pos < len(p.input) && (p.input[p.pos] >= '0' && p.input[p.pos] <= '9') {
			p.pos++
		}
	}
	if p.pos == start {
		if p.pos < len(p.input) {
			return 0, fmt.Errorf("unexpected character at position %d: %q", p.pos, string(p.input[p.pos]))
		}
		return 0, fmt.Errorf("unexpected end of expression")
	}
	val, err := strconv.ParseFloat(p.input[start:p.pos], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number: %q", p.input[start:p.pos])
	}
	return val, nil
}

func evalFunc(name string, args []float64) (float64, error) {
	switch name {
	case "sqrt":
		if len(args) != 1 {
			return 0, fmt.Errorf("sqrt() requires 1 argument, got %d", len(args))
		}
		if args[0] < 0 {
			return 0, fmt.Errorf("sqrt() of negative number: %g", args[0])
		}
		return math.Sqrt(args[0]), nil
	case "abs":
		if len(args) != 1 {
			return 0, fmt.Errorf("abs() requires 1 argument, got %d", len(args))
		}
		return math.Abs(args[0]), nil
	case "sin":
		if len(args) != 1 {
			return 0, fmt.Errorf("sin() requires 1 argument, got %d", len(args))
		}
		return math.Sin(args[0]), nil
	case "cos":
		if len(args) != 1 {
			return 0, fmt.Errorf("cos() requires 1 argument, got %d", len(args))
		}
		return math.Cos(args[0]), nil
	case "log":
		if len(args) != 1 {
			return 0, fmt.Errorf("log() requires 1 argument, got %d", len(args))
		}
		if args[0] <= 0 {
			return 0, fmt.Errorf("log() of non-positive number: %g", args[0])
		}
		return math.Log(args[0]), nil
	case "ceil":
		if len(args) != 1 {
			return 0, fmt.Errorf("ceil() requires 1 argument, got %d", len(args))
		}
		return math.Ceil(args[0]), nil
	case "floor":
		if len(args) != 1 {
			return 0, fmt.Errorf("floor() requires 1 argument, got %d", len(args))
		}
		return math.Floor(args[0]), nil
	case "round":
		if len(args) != 1 {
			return 0, fmt.Errorf("round() requires 1 argument, got %d", len(args))
		}
		return math.Round(args[0]), nil
	case "pow":
		if len(args) != 2 {
			return 0, fmt.Errorf("pow() requires 2 arguments, got %d", len(args))
		}
		return math.Pow(args[0], args[1]), nil
	default:
		return 0, fmt.Errorf("unknown function: %q (supported: sqrt, abs, sin, cos, log, ceil, floor, round, pow)", name)
	}
}
