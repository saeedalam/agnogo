//go:build ignore

// Agent with tools — interactive chat with weather and time tools.
//
//	source .env && go run ./cookbook/01_basics/agent_with_tools.go
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
	debug := agnogo.DefaultDebug()

	agent := agnogo.New(agnogo.Config{
		Model:        model,
		Instructions: "You are a helpful assistant with access to tools.",
		Debug:        &debug,
	})

	agent.Tool("get_weather", "Get current weather for a city", agnogo.Params{
		"city": {Type: "string", Desc: "City name", Required: true},
	}, func(ctx context.Context, args map[string]string) (string, error) {
		return fmt.Sprintf(`{"city": "%s", "temp": 18, "condition": "Partly cloudy", "wind": "12 km/h"}`, args["city"]), nil
	})

	agent.Tool("get_time", "Get current date and time in a timezone", agnogo.Params{
		"timezone": {Type: "string", Desc: "Timezone (e.g. Europe/Stockholm)", Required: true},
	}, func(ctx context.Context, args map[string]string) (string, error) {
		loc, err := time.LoadLocation(args["timezone"])
		if err != nil {
			return "", fmt.Errorf("unknown timezone: %s", args["timezone"])
		}
		return time.Now().In(loc).Format("2006-01-02 15:04 (Monday)"), nil
	})

	agent.CLI()
}
