//go:build ignore

// Loop workflow — iterative refinement until a condition is met.
//
//	OPENAI_API_KEY=sk-... go run ./cookbook/04_workflows/loop.go
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/saeedalam/agnogo"
	"github.com/saeedalam/agnogo/providers/openai"
)

func main() {
	model := openai.New(os.Getenv("OPENAI_API_KEY"), "gpt-4.1-mini")
	debug := agnogo.DefaultDebug()

	refiner := agnogo.New(agnogo.Config{
		Model: model,
		Instructions: `You are an iterative text refiner. Each time you receive text:
1. Improve the clarity and conciseness
2. If you think the text is good enough, start your response with "FINAL:"
3. If it needs more work, just output the improved version

Focus on making the text shorter while keeping the meaning.`,
		Debug: &debug,
	})

	wf := agnogo.Loop(refiner, func(resp *agnogo.Response, i int) bool {
		fmt.Printf("  [Iteration %d] Length: %d chars\n", i+1, len(resp.Text))
		return strings.HasPrefix(resp.Text, "FINAL:") || i >= 3
	}).WithMaxIterations(5)

	session := agnogo.NewSession("demo")
	resp, err := wf.Run(context.Background(), session, `
		In the event that you find yourself in a situation where you need to
		communicate with another individual regarding the scheduling of a meeting
		or appointment, it would be advisable to utilize our online booking system
		which is available for your convenience at the following URL which you can
		access through any web browser on your computer or mobile device.
	`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n--- Final refined text ---")
	fmt.Println(resp.Text)
}
