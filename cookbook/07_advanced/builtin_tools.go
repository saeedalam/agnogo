//go:build ignore

// Built-in tools -- interactive agent with calculator and JSON tools. Try:
//
//	"What is sqrt(144) + 3^2?"
//	"Parse this JSON: {\"name\": \"Erik\", \"age\": 30}"
//	Type "tools" to see available tools.
//
//	OPENAI_API_KEY=sk-... go run ./cookbook/07_advanced/builtin_tools.go
package main

import (
	"github.com/saeedalam/agnogo"
	"github.com/saeedalam/agnogo/tools"
)

func main() {
	allTools := append(tools.Calculator(), tools.JSON()...)
	agent := agnogo.Agent("You are a helpful assistant with calculator and JSON tools.",
		agnogo.Tools(allTools...), agnogo.Debug,
	)
	agent.CLI()
}
