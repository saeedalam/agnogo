//go:build ignore

// Knowledge RAG — inject domain knowledge when the user asks questions.
//
//	OPENAI_API_KEY=sk-... go run ./cookbook/02_memory_knowledge/knowledge_rag.go
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/saeedalam/agnogo"
	"github.com/saeedalam/agnogo/providers/openai"
)

// Simulated knowledge base (in production: pgvector, Qdrant, etc.)
var knowledgeBase = map[string]string{
	"hours":    "Opening hours: Monday-Friday 09:00-18:00, Saturday 10:00-15:00, Sunday closed.",
	"pricing":  "Haircut: 350 SEK, Color: 800 SEK, Beard trim: 200 SEK, Full package: 1200 SEK.",
	"location": "Address: Storgatan 15, 114 55 Stockholm. Nearest metro: Östermalmstorg.",
	"parking":  "Free street parking available on Storgatan. Garage parking at Q-Park Stureplan (5 min walk).",
	"booking":  "Book online at salon.se/book or call +46 8 123 4567. Walk-ins welcome when available.",
}

func searchKnowledge(query string) string {
	query = strings.ToLower(query)
	var results []string
	for key, content := range knowledgeBase {
		if strings.Contains(query, key) ||
			strings.Contains(query, "hour") || strings.Contains(query, "time") && key == "hours" ||
			strings.Contains(query, "price") || strings.Contains(query, "cost") && key == "pricing" ||
			strings.Contains(query, "where") && key == "location" ||
			strings.Contains(query, "park") && key == "parking" ||
			strings.Contains(query, "book") && key == "booking" {
			results = append(results, content)
		}
	}
	if len(results) == 0 {
		// Return all knowledge as fallback
		for _, v := range knowledgeBase {
			results = append(results, v)
		}
	}
	return strings.Join(results, "\n")
}

func main() {
	model := openai.New(os.Getenv("OPENAI_API_KEY"), "gpt-4.1-mini")
	debug := agnogo.DefaultDebug()

	agent := agnogo.New(agnogo.Config{
		Model:        model,
		Instructions: "You are a helpful salon receptionist. Answer questions using the provided knowledge. Be friendly and concise.",
		Knowledge: agnogo.KnowledgeFunc(func(ctx context.Context, query string, limit int) (string, error) {
			return searchKnowledge(query), nil
		}),
		Debug: &debug,
	})

	session := agnogo.NewSession("customer-1")

	questions := []string{
		"What are your opening hours?",
		"How much does a haircut cost?",
		"Where are you located and is there parking?",
	}

	for _, q := range questions {
		fmt.Printf("\n--- Q: %s ---\n", q)
		resp, err := agent.Run(context.Background(), session, q)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			continue
		}
		fmt.Println(resp.Text)
	}
}
