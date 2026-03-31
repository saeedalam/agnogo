# agnogo in 5 Minutes

Build your first AI agent in Go. By the end you'll have a working agent with tools, streaming, and an HTTP API.

## Install

```bash
go get github.com/saeedalam/agnogo@latest
```

Set any provider API key:

```bash
export OPENAI_API_KEY=sk-...
# or ANTHROPIC_API_KEY, GEMINI_API_KEY, GROQ_API_KEY, etc.
```

## Step 1: Hello World (30 seconds)

```go
package main

import (
    "context"
    "fmt"
    "github.com/saeedalam/agnogo"
)

func main() {
    agent := agnogo.Agent("You are a helpful assistant.")
    answer, _ := agent.Ask(context.Background(), "What is Go?")
    fmt.Println(answer)
}
```

```bash
go run main.go
# Go is a statically typed, compiled programming language designed at Google...
```

That's it. `Agent()` auto-detects your provider from env vars. `Ask()` handles sessions automatically.

## Step 2: Add Tools (2 minutes)

Tools let the agent call your Go functions. Define input/output as structs:

```go
package main

import (
    "context"
    "fmt"
    "github.com/saeedalam/agnogo"
)

type WeatherInput struct {
    City string `json:"city" desc:"City name" required:"true"`
}

type WeatherOutput struct {
    City string  `json:"city"`
    Temp float64 `json:"temp_celsius"`
    Desc string  `json:"description"`
}

func getWeather(ctx context.Context, in WeatherInput) (WeatherOutput, error) {
    // In production, call a real weather API here
    return WeatherOutput{City: in.City, Temp: 18.5, Desc: "Partly cloudy"}, nil
}

func main() {
    weather := agnogo.TypedTool("get_weather", "Get current weather", getWeather)

    agent := agnogo.Agent("You are a weather assistant.", agnogo.Tools(weather))

    answer, _ := agent.Ask(context.Background(), "What's the weather in Stockholm?")
    fmt.Println(answer)
}
```

```bash
go run main.go
# The weather in Stockholm is 18.5°C and partly cloudy.
```

The agent sees the tool, decides to call it, gets the result, and responds naturally.

## Step 3: Interactive CLI (30 seconds)

Replace `Ask()` with `CLI()` for a terminal chat:

```go
func main() {
    weather := agnogo.TypedTool("get_weather", "Get current weather", getWeather)
    agent := agnogo.Agent("You are a weather assistant.", agnogo.Tools(weather), agnogo.Debug)
    agent.CLI()
}
```

Commands: `exit`, `clear`, `memory`, `history`, `tools`.

## Step 4: Serve as HTTP API (1 minute)

Replace `CLI()` with `Serve()`:

```go
func main() {
    weather := agnogo.TypedTool("get_weather", "Get current weather", getWeather)
    agent := agnogo.Agent("You are a weather assistant.", agnogo.Tools(weather))

    fmt.Println("Listening on :8080")
    agent.Serve(":8080", agnogo.WithCORS("*"))
}
```

Endpoints:
- `POST /ask` -- `{"message": "weather in London?"}` returns `{"text": "..."}`
- `POST /stream` -- same input, returns SSE stream
- `GET /health` -- `{"status": "ok"}`
- `GET /tools` -- list registered tools

Test it:

```bash
curl -X POST http://localhost:8080/ask \
  -H "Content-Type: application/json" \
  -d '{"message": "weather in Paris?"}'
```

## Step 5: Structured Output (1 minute)

Parse the LLM response directly into a Go struct:

```go
type CityInfo struct {
    Name       string `json:"name"`
    Country    string `json:"country"`
    Population int    `json:"population"`
}

func main() {
    agent := agnogo.Agent("You provide city data. Always respond with valid JSON.")

    var info CityInfo
    agnogo.AskStructured(context.Background(), agent, "Tell me about Tokyo", &info)

    fmt.Printf("%s, %s -- population %d\n", info.Name, info.Country, info.Population)
}
```

## What's Next?

You now know the core API. Here's where to go deeper:

| Want to... | Read |
|------------|------|
| Chain agents together | [README: Pipelines](README.md#pipelines) |
| Handle provider failures | [README: Resilience](README.md#resilience) |
| Use graph workflows | [README: Graph Workflows](README.md#graph-workflows) |
| Add middleware/hooks | [README: Middleware Hooks](README.md#middleware-hooks) |
| Full API reference | [GUIDE.md](GUIDE.md) |
| Run examples | [cookbook/](cookbook/) |
| Pick a provider | [README: Provider Selection](README.md#provider-selection) |

## Cheat Sheet

```go
// Create agent (auto-detect provider)
agent := agnogo.Agent("instructions")
agent := agnogo.Agent("instructions", agnogo.WithOpenAI())
agent := agnogo.Agent("instructions", agnogo.Tools(t1, t2), agnogo.Debug)

// One-shot
answer, err := agent.Ask(ctx, "question")

// Streaming
for chunk := range agent.AskStream(ctx, "question") {
    fmt.Print(chunk.Text)
}

// Structured output
var result MyStruct
agnogo.AskStructured(ctx, agent, "question", &result)

// Interactive CLI
agent.CLI()

// HTTP server
agent.Serve(":8080", agnogo.WithCORS("*"), agnogo.WithAuth("secret"))

// Chain agents
resp, _ := agent1.Then(agent2).Run(ctx, session, input)

// Parallel
resp, _ := agnogo.All(a1, a2, a3).Run(ctx, session, input)

// Resilient provider
model := agnogo.Fallback(primary, backup)
agent := agnogo.Agent("...", agnogo.WithModel(model))
```
