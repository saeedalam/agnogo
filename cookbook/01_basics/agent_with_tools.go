//go:build ignore

// Agent with typed tools -- clean, type-safe tool definitions.
//
//	OPENAI_API_KEY=sk-... go run ./cookbook/01_basics/agent_with_tools.go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/saeedalam/agnogo"
)

type WeatherInput struct {
	City string `json:"city" desc:"City name" required:"true"`
}

type WeatherOutput struct {
	City      string `json:"city"`
	Temp      int    `json:"temp"`
	Condition string `json:"condition"`
}

type TimeInput struct {
	Timezone string `json:"timezone" desc:"Timezone (e.g. Europe/Stockholm)" required:"true"`
}

func main() {
	weather := agnogo.TypedTool("get_weather", "Get current weather for a city",
		func(ctx context.Context, in WeatherInput) (WeatherOutput, error) {
			return WeatherOutput{City: in.City, Temp: 18, Condition: "Partly cloudy"}, nil
		})

	getTime := agnogo.TypedTool("get_time", "Get current date and time",
		func(ctx context.Context, in TimeInput) (string, error) {
			loc, err := time.LoadLocation(in.Timezone)
			if err != nil {
				return "", fmt.Errorf("unknown timezone: %s", in.Timezone)
			}
			return time.Now().In(loc).Format("2006-01-02 15:04 (Monday)"), nil
		})

	agent := agnogo.Agent("You are a helpful assistant.", agnogo.Tools(weather, getTime), agnogo.Debug)
	agent.CLI()
}
