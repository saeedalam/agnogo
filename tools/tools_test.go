package tools

import (
	"context"
	"testing"
)

func TestCalculator(t *testing.T) {
	calc := Calculator()
	if len(calc) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(calc))
	}

	ctx := context.Background()
	tool := calc[0]

	tests := []struct {
		name string
		args map[string]string
		want string
	}{
		{"add", map[string]string{"operation": "add", "a": "3", "b": "4"}, "7"},
		{"subtract", map[string]string{"operation": "subtract", "a": "10", "b": "3"}, "7"},
		{"multiply", map[string]string{"operation": "multiply", "a": "5", "b": "6"}, "30"},
		{"divide", map[string]string{"operation": "divide", "a": "15", "b": "3"}, "5"},
		{"sqrt", map[string]string{"operation": "sqrt", "a": "16"}, "4"},
		{"power", map[string]string{"operation": "power", "a": "2", "b": "10"}, "1024"},
		{"factorial", map[string]string{"operation": "factorial", "a": "5"}, "120"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Fn(ctx, tt.args)
			if err != nil {
				t.Fatal(err)
			}
			if !contains(result, tt.want) {
				t.Errorf("result = %q, want to contain %q", result, tt.want)
			}
		})
	}

	// Division by zero
	t.Run("divide_by_zero", func(t *testing.T) {
		result, _ := tool.Fn(ctx, map[string]string{"operation": "divide", "a": "5", "b": "0"})
		if !contains(result, "error") {
			t.Errorf("expected error for div/0, got %q", result)
		}
	})

	// Unknown operation
	t.Run("unknown_op", func(t *testing.T) {
		result, _ := tool.Fn(ctx, map[string]string{"operation": "unknown", "a": "1"})
		if !contains(result, "error") {
			t.Errorf("expected error for unknown op, got %q", result)
		}
	})
}

func TestJSON(t *testing.T) {
	jsonTools := JSON()
	if len(jsonTools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(jsonTools))
	}

	ctx := context.Background()

	t.Run("parse_with_path", func(t *testing.T) {
		result, _ := jsonTools[0].Fn(ctx, map[string]string{
			"json_str": `{"user": {"name": "Erik", "age": 30}}`,
			"path":     "user.name",
		})
		if result != `"Erik"` {
			t.Errorf("result = %q, want '\"Erik\"'", result)
		}
	})

	t.Run("parse_no_path", func(t *testing.T) {
		result, _ := jsonTools[0].Fn(ctx, map[string]string{
			"json_str": `{"a": 1}`,
		})
		if !contains(result, `"a"`) {
			t.Errorf("result = %q", result)
		}
	})

	t.Run("invalid_json", func(t *testing.T) {
		result, _ := jsonTools[0].Fn(ctx, map[string]string{
			"json_str": "not json",
		})
		if !contains(result, "Invalid") {
			t.Errorf("expected error, got %q", result)
		}
	})

	t.Run("format", func(t *testing.T) {
		result, _ := jsonTools[1].Fn(ctx, map[string]string{
			"json_str": `{"a":1,"b":2}`,
		})
		if !contains(result, "  ") { // pretty printed has indentation
			t.Errorf("result not pretty: %q", result)
		}
	})
}

func TestHTTP(t *testing.T) {
	httpTools := HTTP()
	if len(httpTools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(httpTools))
	}
	if httpTools[0].Name != "http_request" {
		t.Errorf("name = %q", httpTools[0].Name)
	}
}

func TestWebBrowser(t *testing.T) {
	tools := WebBrowser()
	if len(tools) != 1 || tools[0].Name != "read_url" {
		t.Error("WebBrowser tool not configured correctly")
	}
}

func TestDuckDuckGo(t *testing.T) {
	tools := DuckDuckGo()
	if len(tools) != 1 || tools[0].Name != "web_search" {
		t.Error("DuckDuckGo tool not configured correctly")
	}
}

func TestWikipedia(t *testing.T) {
	tools := Wikipedia()
	if len(tools) != 1 || tools[0].Name != "wikipedia" {
		t.Error("Wikipedia tool not configured correctly")
	}
}

func TestFile(t *testing.T) {
	tools := File("/tmp")
	if len(tools) != 3 {
		t.Fatalf("expected 3 file tools, got %d", len(tools))
	}
	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.Name] = true
	}
	for _, expected := range []string{"read_file", "write_file", "list_files"} {
		if !names[expected] {
			t.Errorf("missing tool: %s", expected)
		}
	}

	// Test write + read
	ctx := context.Background()
	writeResult, _ := tools[1].Fn(ctx, map[string]string{"path": "agnogo_test_file.txt", "content": "hello world"})
	if !contains(writeResult, "Written") {
		t.Errorf("write result = %q", writeResult)
	}
	readResult, _ := tools[0].Fn(ctx, map[string]string{"path": "agnogo_test_file.txt"})
	if readResult != "hello world" {
		t.Errorf("read result = %q", readResult)
	}
}

func TestCSV(t *testing.T) {
	tools := CSV()
	if len(tools) != 1 || tools[0].Name != "read_csv" {
		t.Error("CSV tool not configured correctly")
	}
}

func TestShell(t *testing.T) {
	// With allowlist
	tools := Shell("echo", "ls")
	result, _ := tools[0].Fn(context.Background(), map[string]string{"command": "echo hello"})
	if !contains(result, "hello") {
		t.Errorf("shell result = %q", result)
	}

	// Blocked command
	result, _ = tools[0].Fn(context.Background(), map[string]string{"command": "rm -rf /"})
	if !contains(result, "not allowed") {
		t.Errorf("expected blocked, got %q", result)
	}
}

func contains(s, sub string) bool {
	return len(s) > 0 && len(sub) > 0 && containsStr(s, sub)
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
