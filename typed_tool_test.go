package agnogo

import (
	"context"
	"encoding/json"
	"testing"
)

// ── TypedTool Tests ─────────────────────────────────────

func TestTypedToolBasic(t *testing.T) {
	type In struct {
		Query string `json:"query" desc:"Search query" required:"true"`
	}
	type Out struct {
		Result string `json:"result"`
	}

	tool := TypedTool[In, Out]("search", "Search things", func(ctx context.Context, in In) (Out, error) {
		return Out{Result: "found"}, nil
	})

	if tool.Name != "search" {
		t.Errorf("name = %q", tool.Name)
	}
	if tool.Desc != "Search things" {
		t.Errorf("desc = %q", tool.Desc)
	}
	if tool.Fn == nil {
		t.Error("Fn should not be nil")
	}
}

func TestTypedToolParams(t *testing.T) {
	type In struct {
		City string `json:"city" desc:"City name" required:"true"`
		Unit string `json:"unit" desc:"Temperature unit" enum:"C,F"`
	}
	type Out struct {
		Temp float64 `json:"temperature"`
	}

	tool := TypedTool[In, Out]("weather", "Get weather", func(ctx context.Context, in In) (Out, error) {
		return Out{}, nil
	})

	p, ok := tool.Params["city"]
	if !ok {
		t.Fatal("city param not found")
	}
	if p.Type != "string" {
		t.Errorf("city type = %q", p.Type)
	}
	if p.Desc != "City name" {
		t.Errorf("city desc = %q", p.Desc)
	}
	if !p.Required {
		t.Error("city should be required")
	}

	u, ok := tool.Params["unit"]
	if !ok {
		t.Fatal("unit param not found")
	}
	if u.Required {
		t.Error("unit should not be required")
	}
}

func TestTypedToolExecution(t *testing.T) {
	type In struct {
		Name string `json:"name"`
	}
	type Out struct {
		Greeting string `json:"greeting"`
	}

	tool := TypedTool[In, Out]("greet", "Greet someone", func(ctx context.Context, in In) (Out, error) {
		return Out{Greeting: "Hello, " + in.Name + "!"}, nil
	})

	result, err := tool.Fn(context.Background(), map[string]string{"name": "Alice"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out Out
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("unmarshal error: %v (raw: %s)", err, result)
	}
	if out.Greeting != "Hello, Alice!" {
		t.Errorf("greeting = %q", out.Greeting)
	}
}

func TestTypedToolNumericArgs(t *testing.T) {
	type In struct {
		Count int     `json:"count"`
		Rate  float64 `json:"rate"`
	}
	type Out struct {
		Total float64 `json:"total"`
	}

	tool := TypedTool[In, Out]("calc", "Calculate", func(ctx context.Context, in In) (Out, error) {
		return Out{Total: float64(in.Count) * in.Rate}, nil
	})

	result, err := tool.Fn(context.Background(), map[string]string{
		"count": "5",
		"rate":  "2.5",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out Out
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if out.Total != 12.5 {
		t.Errorf("total = %f, want 12.5", out.Total)
	}
}

func TestTypedToolBoolArgs(t *testing.T) {
	type In struct {
		Verbose bool `json:"verbose"`
	}
	type Out struct {
		Mode string `json:"mode"`
	}

	tool := TypedTool[In, Out]("run", "Run command", func(ctx context.Context, in In) (Out, error) {
		mode := "quiet"
		if in.Verbose {
			mode = "verbose"
		}
		return Out{Mode: mode}, nil
	})

	result, err := tool.Fn(context.Background(), map[string]string{"verbose": "true"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out Out
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if out.Mode != "verbose" {
		t.Errorf("mode = %q, want verbose", out.Mode)
	}
}

func TestTypedToolStringOutput(t *testing.T) {
	type In struct {
		Msg string `json:"msg"`
	}

	tool := TypedTool[In, string]("echo", "Echo message", func(ctx context.Context, in In) (string, error) {
		return "echoed: " + in.Msg, nil
	})

	result, err := tool.Fn(context.Background(), map[string]string{"msg": "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// String output should be returned directly, not JSON-wrapped.
	if result != "echoed: hello" {
		t.Errorf("result = %q, want %q", result, "echoed: hello")
	}
}

func TestTypedToolPanicsOnNonStruct(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for non-struct input type")
		}
	}()

	TypedTool[string, string]("bad", "Bad tool", func(ctx context.Context, in string) (string, error) {
		return in, nil
	})
}

func TestTypedToolSkipsUnexportedFields(t *testing.T) {
	type In struct {
		Public  string `json:"public"`
		private string //nolint:unused
	}
	type Out struct {
		OK bool `json:"ok"`
	}

	tool := TypedTool[In, Out]("test", "Test unexported", func(ctx context.Context, in In) (Out, error) {
		return Out{OK: true}, nil
	})

	if _, exists := tool.Params["private"]; exists {
		t.Error("unexported field should not be in Params")
	}
	if _, exists := tool.Params["public"]; !exists {
		t.Error("exported field should be in Params")
	}
}

func TestTypedToolEnumTag(t *testing.T) {
	type In struct {
		Format string `json:"format" enum:"json,xml,csv"`
	}
	type Out struct {
		OK bool `json:"ok"`
	}

	tool := TypedTool[In, Out]("export", "Export data", func(ctx context.Context, in In) (Out, error) {
		return Out{OK: true}, nil
	})

	p, ok := tool.Params["format"]
	if !ok {
		t.Fatal("format param not found")
	}
	if len(p.Enum) != 3 {
		t.Fatalf("enum len = %d, want 3", len(p.Enum))
	}
	expected := []string{"json", "xml", "csv"}
	for i, v := range expected {
		if p.Enum[i] != v {
			t.Errorf("enum[%d] = %q, want %q", i, p.Enum[i], v)
		}
	}
}
