// Package tools provides built-in tools for agnogo agents.
// Each tool is a standalone function that can be registered with agent.Tool().
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	"github.com/saeedalam/agnogo"
)

// Calculator returns tools for basic math operations.
func Calculator() []agnogo.ToolDef {
	return []agnogo.ToolDef{
		{
			Name: "calculator", Desc: "Perform math: add, subtract, multiply, divide, sqrt, power",
			Params: agnogo.Params{
				"operation": {Type: "string", Desc: "Operation: add, subtract, multiply, divide, sqrt, power, factorial", Required: true},
				"a":         {Type: "number", Desc: "First number", Required: true},
				"b":         {Type: "number", Desc: "Second number (not needed for sqrt/factorial)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				a, _ := strconv.ParseFloat(args["a"], 64)
				b, _ := strconv.ParseFloat(args["b"], 64)
				var result float64
				op := args["operation"]
				switch op {
				case "add":
					result = a + b
				case "subtract":
					result = a - b
				case "multiply":
					result = a * b
				case "divide":
					if b == 0 {
						return `{"error": "division by zero"}`, nil
					}
					result = a / b
				case "sqrt":
					if a < 0 {
						return `{"error": "cannot sqrt negative"}`, nil
					}
					result = math.Sqrt(a)
				case "power":
					result = math.Pow(a, b)
				case "factorial":
					if a < 0 || a > 20 {
						return `{"error": "factorial only for 0-20"}`, nil
					}
					result = 1
					for i := 2; i <= int(a); i++ {
						result *= float64(i)
					}
				default:
					return fmt.Sprintf(`{"error": "unknown operation: %s"}`, op), nil
				}
				r, _ := json.Marshal(map[string]any{"operation": op, "result": result})
				return string(r), nil
			},
		},
	}
}
