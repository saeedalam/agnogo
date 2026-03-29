//go:build ignore

// Structured output — force the model to return typed JSON.
//
//	OPENAI_API_KEY=sk-... go run ./cookbook/01_basics/structured_output.go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/saeedalam/agnogo"
	"github.com/saeedalam/agnogo/providers/openai"
)

type MovieReview struct {
	Title    string  `json:"title"`
	Year     int     `json:"year"`
	Rating   float64 `json:"rating"`
	Summary  string  `json:"summary"`
	Genre    string  `json:"genre"`
}

func main() {
	model := openai.New(os.Getenv("OPENAI_API_KEY"), "gpt-4.1-mini")
	debug := agnogo.DefaultDebug()

	agent := agnogo.New(agnogo.Config{
		Model:        model,
		Instructions: "You review movies. Always respond with valid JSON matching the requested schema.",
		Debug:        &debug,
	})

	session := agnogo.NewSession("demo")

	var review MovieReview
	err := agnogo.RunStructured(context.Background(), agent, session, "Review the movie Interstellar", &review)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nMovie: %s (%d)\n", review.Title, review.Year)
	fmt.Printf("Genre: %s\n", review.Genre)
	fmt.Printf("Rating: %.1f/10\n", review.Rating)
	fmt.Printf("Summary: %s\n", review.Summary)
}
