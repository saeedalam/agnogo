//go:build ignore

// CLI agent -- interactive terminal with memory and tools.
//
// Commands: exit, clear, memory, history, tools
//
//	OPENAI_API_KEY=sk-... go run ./cookbook/07_advanced/cli_agent.go
package main

import (
	"github.com/saeedalam/agnogo"
	"github.com/saeedalam/agnogo/tools"
)

func main() {
	agent := agnogo.Agent("You are a helpful assistant. Be concise.",
		agnogo.Tools(tools.Calculator()...), agnogo.Memory, agnogo.Debug,
	)
	agent.CLI()
}
