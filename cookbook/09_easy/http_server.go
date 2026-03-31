//go:build ignore

// HTTP server — serve an agent over HTTP with CORS and concurrency limits.
//
//	OPENAI_API_KEY=sk-... go run ./cookbook/09_easy/http_server.go
//	curl -X POST localhost:8080/ask -d '{"message":"Hello!"}'
package main

import (
	"fmt"

	"github.com/saeedalam/agnogo"
)

func main() {
	agent := agnogo.Agent("You are a helpful API assistant.")

	fmt.Println("Server starting on :8080...")
	if err := agent.Serve(":8080", agnogo.WithCORS("*"), agnogo.WithMaxConcurrent(10)); err != nil {
		fmt.Println("Error:", err)
	}
}
