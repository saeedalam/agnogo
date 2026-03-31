//go:build ignore

// Typed tools — type-safe tool definitions with generics.
//
//	OPENAI_API_KEY=sk-... go run ./cookbook/09_easy/typed_tools.go
package main

import (
	"context"

	"github.com/saeedalam/agnogo"
)

type AddInput struct {
	A float64 `json:"a" desc:"First number" required:"true"`
	B float64 `json:"b" desc:"Second number" required:"true"`
}

type AddOutput struct {
	Result float64 `json:"result"`
}

func main() {
	tool := agnogo.TypedTool("add", "Add two numbers",
		func(ctx context.Context, in AddInput) (AddOutput, error) {
			return AddOutput{Result: in.A + in.B}, nil
		})

	agent := agnogo.Agent("You are a math assistant.", agnogo.Tools(tool))
	agent.CLI()
}
