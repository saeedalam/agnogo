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
)

type MovieReview struct {
	Title   string  `json:"title"`
	Year    int     `json:"year"`
	Rating  float64 `json:"rating"`
	Summary string  `json:"summary"`
	Genre   string  `json:"genre"`
}

func main() {
	agent := agnogo.Agent("You review movies. Always respond with valid JSON matching the requested schema.")

	var review MovieReview
	err := agnogo.AskStructured(context.Background(), agent, "Review the movie Interstellar", &review)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nMovie: %s (%d)\n", review.Title, review.Year)
	fmt.Printf("Genre: %s\n", review.Genre)
	fmt.Printf("Rating: %.1f/10\n", review.Rating)
	fmt.Printf("Summary: %s\n", review.Summary)
}
