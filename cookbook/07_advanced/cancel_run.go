//go:build ignore

// Cancel run — cancel agent execution mid-flight.
//
//	go run ./cookbook/07_advanced/cancel_run.go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/saeedalam/agnogo"
)

func main() {
	fmt.Println("--- Cancel Run Demo ---")

	// Register a run
	ctx, runID := agnogo.RegisterRun(context.Background(), "demo-run-1")
	fmt.Printf("Registered run: %s\n", runID)
	fmt.Printf("Active runs: %d\n", agnogo.ActiveRunCount())

	// Simulate work in a goroutine
	done := make(chan struct{})
	go func() {
		defer close(done)
		select {
		case <-ctx.Done():
			fmt.Println("Run was cancelled!")
		case <-time.After(10 * time.Second):
			fmt.Println("Run completed normally")
		}
	}()

	// Cancel after a short delay
	time.Sleep(100 * time.Millisecond)
	fmt.Println("\nCancelling run...")
	agnogo.CancelRun(runID)
	fmt.Printf("Active runs after cancel: %d\n", agnogo.ActiveRunCount())

	<-done
	fmt.Println("\nDone.")
}
