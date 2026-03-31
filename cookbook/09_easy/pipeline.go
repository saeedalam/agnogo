//go:build ignore

// Pipeline — chain agents sequentially with Then.
//
//	OPENAI_API_KEY=sk-... go run ./cookbook/09_easy/pipeline.go
package main

import (
	"context"
	"fmt"

	"github.com/saeedalam/agnogo"
)

func main() {
	summarizer := agnogo.Agent("Summarize the text in one sentence.")
	translator := agnogo.Agent("Translate the text to French.")

	pipeline := summarizer.Then(translator)
	session := agnogo.NewSession("demo")
	resp, err := pipeline.Run(context.Background(), session, "Go is a statically typed language designed at Google.")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println(resp.Text)
}
