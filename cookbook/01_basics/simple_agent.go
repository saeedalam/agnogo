//go:build ignore

// Simple agent — interactive chat.
//
//	source .env && go run ./cookbook/01_basics/simple_agent.go
package main

import (
	"os"

	"github.com/saeedalam/agnogo"
	"github.com/saeedalam/agnogo/providers/openai"
)

func main() {
	model := openai.New(os.Getenv("OPENAI_API_KEY"), "gpt-4.1-mini")
	debug := agnogo.DefaultDebug()

	agent := agnogo.New(agnogo.Config{
		Model:        model,
		Instructions: "You are a helpful assistant. Be concise.",
		Debug:        &debug,
	})

	agent.CLI()
}
