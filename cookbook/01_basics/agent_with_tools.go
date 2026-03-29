//go:build ignore

// Agent with tools — the model decides when to call your functions.
//
//	OPENAI_API_KEY=sk-... go run ./cookbook/01_basics/agent_with_tools.go
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/saeedalam/agnogo"
	"github.com/saeedalam/agnogo/providers/openai"
)

func main() {
	model := openai.New(os.Getenv("OPENAI_API_KEY"), "gpt-4.1-mini")
	debug := agnogo.VerboseDebug()

	agent := agnogo.New(agnogo.Config{
		Model:        model,
		Instructions: "You are a helpful assistant with access to tools. Use them when needed.",
		Debug:        &debug,
	})

	// Weather tool
	agent.Tool("get_weather", "Get current weather for a city", agnogo.Params{
		"city": {Type: "string", Desc: "City name", Required: true},
	}, func(ctx context.Context, args map[string]string) (string, error) {
		// Simulated weather data
		return fmt.Sprintf(`{"city": "%s", "temp": 18, "condition": "Partly cloudy", "wind": "12 km/h"}`, args["city"]), nil
	})

	// Time tool
	agent.Tool("get_time", "Get current time in a timezone", agnogo.Params{
		"timezone": {Type: "string", Desc: "Timezone (e.g. Europe/Stockholm)", Required: true},
	}, func(ctx context.Context, args map[string]string) (string, error) {
		loc, err := time.LoadLocation(args["timezone"])
		if err != nil {
			return "", fmt.Errorf("unknown timezone: %s", args["timezone"])
		}
		return time.Now().In(loc).Format("15:04 Monday, January 2"), nil
	})

	session := agnogo.NewSession("demo")
	resp, err := agent.Run(context.Background(), session, "What's the weather in Stockholm and what time is it there?")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n" + resp.Text)
	fmt.Printf("Tools called: %v\n", resp.ToolsCalled)
}
