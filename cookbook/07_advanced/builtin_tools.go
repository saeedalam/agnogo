//go:build ignore

// Built-in tools — interactive agent with calculator and JSON tools. Try:
//   "What is the square root of 144?"
//   "Calculate 15 factorial"
//   "Parse this JSON: {\"name\": \"Erik\", \"age\": 30}"
//   Type "tools" to see available tools.
//
//	source .env && go run ./cookbook/07_advanced/builtin_tools.go
package main

import (
	"os"

	"github.com/saeedalam/agnogo"
	"github.com/saeedalam/agnogo/providers/openai"
	"github.com/saeedalam/agnogo/tools"
)

func main() {
	model := openai.New(os.Getenv("OPENAI_API_KEY"), "gpt-4.1-mini")
	debug := agnogo.VerboseDebug()

	agent := agnogo.New(agnogo.Config{
		Model:        model,
		Instructions: "You are a helpful assistant with access to a calculator and JSON tools. Use them when appropriate.",
		Debug:        &debug,
	})

	agent.AddTools(tools.Calculator()...)
	agent.AddTools(tools.JSON()...)

	agent.CLI()
}
