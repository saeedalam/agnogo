//go:build ignore

// Knowledge RAG — salon receptionist with a knowledge base. Try:
//   "What are your opening hours?"
//   "How much does a haircut cost?"
//   "Where are you located?"
//   "Do you have parking?"
//
//	source .env && go run ./cookbook/02_memory_knowledge/knowledge_rag.go
package main

import (
	"context"
	"os"
	"strings"

	"github.com/saeedalam/agnogo"
	"github.com/saeedalam/agnogo/providers/openai"
)

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
	for _, content := range knowledgeBase {
		results = append(results, content)
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

	agent.CLI()
}
