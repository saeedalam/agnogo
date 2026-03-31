//go:build ignore

// Event bus -- subscribe to agent events for observability.
//
//	OPENAI_API_KEY=sk-... go run ./cookbook/10_production/event_bus.go
package main

import (
	"context"
	"fmt"

	"github.com/saeedalam/agnogo"
)

func main() {
	bus := agnogo.NewEventBus()

	// Subscribe to events
	bus.On(agnogo.EventRunStart, func(e agnogo.Event) {
		fmt.Println("[event] Run started")
	})
	bus.On(agnogo.EventModelDone, func(e agnogo.Event) {
		fmt.Printf("[event] Model responded in %v\n", e.Data["duration"])
	})
	bus.On(agnogo.EventToolCall, func(e agnogo.Event) {
		fmt.Printf("[event] Tool called: %s\n", e.Data["name"])
	})
	bus.On(agnogo.EventRunEnd, func(e agnogo.Event) {
		fmt.Println("[event] Run complete")
	})

	agent := agnogo.Agent("You are a helpful assistant.", agnogo.WithEvents(bus))

	answer, err := agent.Ask(context.Background(), "What is 2 + 2?")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("\nAnswer:", answer)
}
