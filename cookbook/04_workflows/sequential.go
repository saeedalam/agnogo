//go:build ignore

// Sequential workflow — pipeline of agents processing data in order.
//
//	OPENAI_API_KEY=sk-... go run ./cookbook/04_workflows/sequential.go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/saeedalam/agnogo"
	"github.com/saeedalam/agnogo/providers/openai"
)

func main() {
	model := openai.New(os.Getenv("OPENAI_API_KEY"), "gpt-4.1-mini")
	debug := agnogo.DefaultDebug()

	// Step 1: Extract key info
	extractor := agnogo.New(agnogo.Config{
		Model:        model,
		Instructions: "Extract the key facts from the user's message. Output a clean bullet list of facts only.",
		Debug:        &debug,
	})

	// Step 2: Translate to Swedish
	translator := agnogo.New(agnogo.Config{
		Model:        model,
		Instructions: "Translate the following text to Swedish. Keep the same format.",
		Debug:        &debug,
	})

	// Step 3: Summarize
	summarizer := agnogo.New(agnogo.Config{
		Model:        model,
		Instructions: "Write a one-paragraph summary of the provided information. Be concise.",
		Debug:        &debug,
	})

	wf := agnogo.Sequential(
		agnogo.Step("extract", extractor),
		agnogo.Step("translate", translator),
		agnogo.Step("summarize", summarizer),
	)

	session := agnogo.NewSession("demo")
	resp, err := wf.Run(context.Background(), session, `
		Stockholm is the capital of Sweden with a population of about 1 million in the city
		and 2.4 million in the metro area. It's built on 14 islands connected by 57 bridges.
		The city was founded in 1252 and is known for its old town Gamla Stan, the Nobel Prize,
		and being home to companies like Spotify, IKEA's headquarters, and Ericsson.
	`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n--- Final output (extracted → translated → summarized) ---")
	fmt.Println(resp.Text)
}
