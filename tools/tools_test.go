package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Calculator Tests ---

func TestCalculatorLegacy(t *testing.T) {
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
			if !strings.Contains(result, tt.want) {
				t.Errorf("result = %q, want to contain %q", result, tt.want)
			}
		})
	}

	// Division by zero returns error
	t.Run("divide_by_zero", func(t *testing.T) {
		_, err := tool.Fn(ctx, map[string]string{"operation": "divide", "a": "5", "b": "0"})
		if err == nil {
			t.Error("expected error for div/0")
		}
	})

	// Unknown operation returns error
	t.Run("unknown_op", func(t *testing.T) {
		_, err := tool.Fn(ctx, map[string]string{"operation": "unknown", "a": "1"})
		if err == nil {
			t.Error("expected error for unknown op")
		}
	})
}

func TestCalculatorExpressionParser(t *testing.T) {
	calc := Calculator()
	ctx := context.Background()
	tool := calc[0]

	tests := []struct {
		name string
		expr string
		want float64
	}{
		{"simple_add", "2 + 3", 5},
		{"simple_sub", "10 - 3", 7},
		{"simple_mul", "5 * 6", 30},
		{"simple_div", "15 / 3", 5},
		{"modulo", "10 % 3", 1},
		{"power", "2 ^ 10", 1024},
		{"precedence_mul_add", "2 + 3 * 4", 14},
		{"precedence_parens", "(2 + 3) * 4", 20},
		{"nested_parens", "((2 + 3) * (4 - 1))", 15},
		{"unary_minus", "-5 + 3", -2},
		{"unary_double_minus", "--5", 5},
		{"unary_plus", "+5", 5},
		{"power_precedence", "2 ^ 3 ^ 2", 512}, // right-associative: 2^(3^2) = 2^9 = 512
		{"float", "1.5 * 2", 3},
		{"scientific", "1e2 + 1", 101},
		{"complex_expr", "2 + 3 * (4 - 1) / 3", 5},
		{"sqrt_func", "sqrt(16)", 4},
		{"abs_func", "abs(-5)", 5},
		{"pow_func", "pow(2, 10)", 1024},
		{"ceil_func", "ceil(1.1)", 2},
		{"floor_func", "floor(1.9)", 1},
		{"round_func", "round(1.5)", 2},
		{"log_func", "log(1)", 0},
		{"nested_func", "sqrt(pow(3, 2) + pow(4, 2))", 5},
		{"constant_pi", "pi", 3.141592653589793},
		{"constant_e", "e", 2.718281828459045},
		{"func_in_expr", "1 + sqrt(9) * 2", 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Fn(ctx, map[string]string{"expression": tt.expr})
			if err != nil {
				t.Fatalf("expression %q: %v", tt.expr, err)
			}
			var parsed struct {
				Result float64 `json:"result"`
			}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Fatalf("cannot parse result JSON %q: %v", result, err)
			}
			// Allow small floating point tolerance
			diff := parsed.Result - tt.want
			if diff < -1e-9 || diff > 1e-9 {
				t.Errorf("expression %q = %g, want %g", tt.expr, parsed.Result, tt.want)
			}
		})
	}
}

func TestCalculatorExpressionErrors(t *testing.T) {
	calc := Calculator()
	ctx := context.Background()
	tool := calc[0]

	tests := []struct {
		name string
		expr string
	}{
		{"empty", ""},
		{"div_by_zero", "1 / 0"},
		{"mod_by_zero", "5 % 0"},
		{"sqrt_negative", "sqrt(-1)"},
		{"log_negative", "log(-1)"},
		{"unknown_func", "foo(1)"},
		{"unknown_ident", "xyz"},
		{"unmatched_paren", "(1 + 2"},
		{"extra_close_paren", "1 + 2)"},
		{"wrong_arg_count", "sqrt(1, 2)"},
		{"pow_one_arg", "pow(2)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tool.Fn(ctx, map[string]string{"expression": tt.expr})
			if err == nil {
				t.Errorf("expression %q: expected error", tt.expr)
			}
		})
	}
}

func TestCalculatorMaxDepth(t *testing.T) {
	calc := Calculator(CalculatorConfig{MaxDepth: 5})
	ctx := context.Background()
	tool := calc[0]

	// Deeply nested expression should fail
	_, err := tool.Fn(ctx, map[string]string{"expression": "((((((1))))))"})
	if err == nil {
		t.Error("expected depth error for deeply nested expression")
	}
}

// --- JSON Tests ---

func TestJSON(t *testing.T) {
	jsonTools := JSON()
	ctx := context.Background()

	// Find tools by name
	toolMap := map[string]int{}
	for i, tool := range jsonTools {
		toolMap[tool.Name] = i
	}

	t.Run("parse_with_path", func(t *testing.T) {
		result, err := jsonTools[toolMap["json_parse"]].Fn(ctx, map[string]string{
			"json_str": `{"user": {"name": "Erik", "age": 30}}`,
			"path":     "user.name",
		})
		if err != nil {
			t.Fatal(err)
		}
		if result != `"Erik"` {
			t.Errorf("result = %q, want '\"Erik\"'", result)
		}
	})

	t.Run("parse_array_index", func(t *testing.T) {
		result, err := jsonTools[toolMap["json_parse"]].Fn(ctx, map[string]string{
			"json_str": `{"items": [{"name": "first"}, {"name": "second"}]}`,
			"path":     "items[1].name",
		})
		if err != nil {
			t.Fatal(err)
		}
		if result != `"second"` {
			t.Errorf("result = %q, want '\"second\"'", result)
		}
	})

	t.Run("parse_nested_array", func(t *testing.T) {
		result, err := jsonTools[toolMap["json_parse"]].Fn(ctx, map[string]string{
			"json_str": `{"data": {"items": [10, 20, 30]}}`,
			"path":     "data.items[2]",
		})
		if err != nil {
			t.Fatal(err)
		}
		if result != "30" {
			t.Errorf("result = %q, want '30'", result)
		}
	})

	t.Run("parse_no_path", func(t *testing.T) {
		result, err := jsonTools[toolMap["json_parse"]].Fn(ctx, map[string]string{
			"json_str": `{"a": 1}`,
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(result, `"a"`) {
			t.Errorf("result = %q", result)
		}
	})

	t.Run("invalid_json", func(t *testing.T) {
		_, err := jsonTools[toolMap["json_parse"]].Fn(ctx, map[string]string{
			"json_str": "not json",
		})
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})

	t.Run("format", func(t *testing.T) {
		result, err := jsonTools[toolMap["json_format"]].Fn(ctx, map[string]string{
			"json_str": `{"a":1,"b":2}`,
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(result, "  ") {
			t.Errorf("result not pretty: %q", result)
		}
	})

	t.Run("validate_valid", func(t *testing.T) {
		result, err := jsonTools[toolMap["json_validate"]].Fn(ctx, map[string]string{
			"json_str": `{"a": 1}`,
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(result, `"valid":true`) {
			t.Errorf("expected valid=true, got %q", result)
		}
	})

	t.Run("validate_invalid", func(t *testing.T) {
		result, err := jsonTools[toolMap["json_validate"]].Fn(ctx, map[string]string{
			"json_str": `{bad}`,
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(result, `"valid":false`) {
			t.Errorf("expected valid=false, got %q", result)
		}
	})

	t.Run("merge", func(t *testing.T) {
		result, err := jsonTools[toolMap["json_merge"]].Fn(ctx, map[string]string{
			"base":    `{"a": 1, "b": {"c": 2, "d": 3}}`,
			"overlay": `{"b": {"c": 99, "e": 4}, "f": 5}`,
		})
		if err != nil {
			t.Fatal(err)
		}
		var merged map[string]any
		if err := json.Unmarshal([]byte(result), &merged); err != nil {
			t.Fatal(err)
		}
		b := merged["b"].(map[string]any)
		if b["c"].(float64) != 99 {
			t.Errorf("b.c should be 99, got %v", b["c"])
		}
		if b["d"].(float64) != 3 {
			t.Errorf("b.d should be 3, got %v", b["d"])
		}
		if b["e"].(float64) != 4 {
			t.Errorf("b.e should be 4, got %v", b["e"])
		}
		if merged["f"].(float64) != 5 {
			t.Errorf("f should be 5, got %v", merged["f"])
		}
	})
}

// --- HTTP Tests ---

func TestHTTP(t *testing.T) {
	httpTools := HTTP()
	if len(httpTools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(httpTools))
	}
	if httpTools[0].Name != "http_request" {
		t.Errorf("name = %q", httpTools[0].Name)
	}

	// Test invalid method
	t.Run("invalid_method", func(t *testing.T) {
		_, err := httpTools[0].Fn(context.Background(), map[string]string{
			"url":    "https://example.com",
			"method": "INVALID",
		})
		if err == nil {
			t.Error("expected error for invalid method")
		}
	})

	// Test invalid timeout
	t.Run("invalid_timeout", func(t *testing.T) {
		_, err := httpTools[0].Fn(context.Background(), map[string]string{
			"url":     "https://example.com",
			"method":  "GET",
			"timeout": "abc",
		})
		if err == nil {
			t.Error("expected error for invalid timeout")
		}
	})

	// Test invalid headers JSON
	t.Run("invalid_headers", func(t *testing.T) {
		_, err := httpTools[0].Fn(context.Background(), map[string]string{
			"url":     "https://example.com",
			"method":  "GET",
			"headers": "not json",
		})
		if err == nil {
			t.Error("expected error for invalid headers JSON")
		}
	})
}

func TestHTTPConfig(t *testing.T) {
	httpTools := HTTP(HTTPConfig{
		DefaultTimeout:  5,
		MaxResponseSize: 100,
		UserAgent:       "test/1.0",
	})
	if len(httpTools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(httpTools))
	}
}

// --- Web Browser Tests ---

func TestWebBrowser(t *testing.T) {
	tools := WebBrowser()
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.Name] = true
	}
	if !names["read_url"] {
		t.Error("missing read_url tool")
	}
	if !names["web_extract_links"] {
		t.Error("missing web_extract_links tool")
	}
}

func TestStripHTMLStateMachine(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain_text", "hello world", "hello world"},
		{"simple_tags", "<p>hello</p>", "hello"},
		{"script_removal", "before<script>var x=1;</script>after", "before after"},
		{"style_removal", "before<style>.x{color:red}</style>after", "before after"},
		{"noscript_removal", "before<noscript>enable js</noscript>after", "before after"},
		{"nested_tags", "<div><p>hello</p></div>", "hello"},
		{"entities", "&amp; &lt; &gt;", "& < >"},
		{"whitespace_collapse", "hello   \n\t  world", "hello world"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripHTMLStateMachine(tt.input)
			if got != tt.want {
				t.Errorf("stripHTMLStateMachine(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractLinks(t *testing.T) {
	html := `<html><body>
		<a href="https://example.com">Example</a>
		<a href="/about">About Us</a>
		<a href="https://test.com">Test <b>Bold</b></a>
	</body></html>`

	links := extractLinks(html)
	if len(links) != 3 {
		t.Fatalf("expected 3 links, got %d", len(links))
	}
	if links[0]["href"] != "https://example.com" {
		t.Errorf("link 0 href = %q", links[0]["href"])
	}
	if links[0]["text"] != "Example" {
		t.Errorf("link 0 text = %q", links[0]["text"])
	}
	if links[2]["text"] != "Test Bold" {
		t.Errorf("link 2 text = %q (expected nested tag stripped)", links[2]["text"])
	}
}

func TestDuckDuckGo(t *testing.T) {
	tools := DuckDuckGo()
	if len(tools) != 1 || tools[0].Name != "web_search" {
		t.Error("DuckDuckGo tool not configured correctly")
	}

	// Test empty query
	t.Run("empty_query", func(t *testing.T) {
		_, err := tools[0].Fn(context.Background(), map[string]string{"query": ""})
		if err == nil {
			t.Error("expected error for empty query")
		}
	})
}

func TestWikipedia(t *testing.T) {
	tools := Wikipedia()
	if len(tools) != 1 || tools[0].Name != "wikipedia" {
		t.Error("Wikipedia tool not configured correctly")
	}

	// Test empty query
	t.Run("empty_query", func(t *testing.T) {
		_, err := tools[0].Fn(context.Background(), map[string]string{"query": ""})
		if err == nil {
			t.Error("expected error for empty query")
		}
	})
}

// --- File Tests ---

func TestFile(t *testing.T) {
	tmpDir := t.TempDir()
	tools := File(tmpDir)
	if len(tools) != 5 {
		t.Fatalf("expected 5 file tools, got %d", len(tools))
	}
	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.Name] = true
	}
	for _, expected := range []string{"read_file", "write_file", "file_append", "file_info", "list_files"} {
		if !names[expected] {
			t.Errorf("missing tool: %s", expected)
		}
	}

	ctx := context.Background()

	// Find tools by name
	toolMap := map[string]int{}
	for i, tool := range tools {
		toolMap[tool.Name] = i
	}

	// Test write + read
	t.Run("write_and_read", func(t *testing.T) {
		writeResult, err := tools[toolMap["write_file"]].Fn(ctx, map[string]string{
			"path": "test_file.txt", "content": "hello world",
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(writeResult, "Written") {
			t.Errorf("write result = %q", writeResult)
		}
		readResult, err := tools[toolMap["read_file"]].Fn(ctx, map[string]string{"path": "test_file.txt"})
		if err != nil {
			t.Fatal(err)
		}
		if readResult != "hello world" {
			t.Errorf("read result = %q", readResult)
		}
	})

	// Test atomic write (file should exist after write)
	t.Run("atomic_write", func(t *testing.T) {
		_, err := tools[toolMap["write_file"]].Fn(ctx, map[string]string{
			"path": "atomic_test.txt", "content": "atomic content",
		})
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(tmpDir, "atomic_test.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "atomic content" {
			t.Errorf("file content = %q", string(data))
		}
	})

	// Test append
	t.Run("append", func(t *testing.T) {
		tools[toolMap["write_file"]].Fn(ctx, map[string]string{
			"path": "append_test.txt", "content": "line1\n",
		})
		appendResult, err := tools[toolMap["file_append"]].Fn(ctx, map[string]string{
			"path": "append_test.txt", "content": "line2\n",
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(appendResult, "Appended") {
			t.Errorf("append result = %q", appendResult)
		}
		readResult, _ := tools[toolMap["read_file"]].Fn(ctx, map[string]string{"path": "append_test.txt"})
		if readResult != "line1\nline2\n" {
			t.Errorf("after append, content = %q", readResult)
		}
	})

	// Test file_info
	t.Run("file_info", func(t *testing.T) {
		tools[toolMap["write_file"]].Fn(ctx, map[string]string{
			"path": "info_test.txt", "content": "test content",
		})
		infoResult, err := tools[toolMap["file_info"]].Fn(ctx, map[string]string{"path": "info_test.txt"})
		if err != nil {
			t.Fatal(err)
		}
		var info map[string]any
		if err := json.Unmarshal([]byte(infoResult), &info); err != nil {
			t.Fatal(err)
		}
		if info["name"] != "info_test.txt" {
			t.Errorf("name = %v", info["name"])
		}
		if info["size"].(float64) != 12 {
			t.Errorf("size = %v", info["size"])
		}
	})

	// Test path traversal
	t.Run("path_traversal", func(t *testing.T) {
		_, err := tools[toolMap["read_file"]].Fn(ctx, map[string]string{"path": "../../etc/passwd"})
		if err == nil {
			t.Error("expected error for path traversal")
		}
	})

	// Test empty path
	t.Run("empty_path", func(t *testing.T) {
		_, err := tools[toolMap["read_file"]].Fn(ctx, map[string]string{"path": ""})
		if err == nil {
			t.Error("expected error for empty path")
		}
	})
}

func TestFileMaxReadSize(t *testing.T) {
	tmpDir := t.TempDir()
	tools := File(tmpDir, FileConfig{MaxReadSize: 10})
	ctx := context.Background()

	toolMap := map[string]int{}
	for i, tool := range tools {
		toolMap[tool.Name] = i
	}

	// Write a large file
	os.WriteFile(filepath.Join(tmpDir, "large.txt"), []byte("this is more than 10 bytes"), 0o644)
	_, err := tools[toolMap["read_file"]].Fn(ctx, map[string]string{"path": "large.txt"})
	if err == nil {
		t.Error("expected error for file exceeding max read size")
	}
}

func TestCSV(t *testing.T) {
	tools := CSV()
	if len(tools) != 1 || tools[0].Name != "read_csv" {
		t.Error("CSV tool not configured correctly")
	}
}

// --- Shell Tests ---

func TestShell(t *testing.T) {
	// With allowlist
	tools := Shell("echo", "ls")
	ctx := context.Background()

	t.Run("allowed_command", func(t *testing.T) {
		result, err := tools[0].Fn(ctx, map[string]string{"command": "echo hello"})
		if err != nil {
			t.Fatal(err)
		}
		var parsed map[string]any
		if err := json.Unmarshal([]byte(result), &parsed); err != nil {
			t.Fatalf("cannot parse result JSON: %v", err)
		}
		stdout := parsed["stdout"].(string)
		if !strings.Contains(stdout, "hello") {
			t.Errorf("stdout = %q", stdout)
		}
		if parsed["exit_code"].(float64) != 0 {
			t.Errorf("exit_code = %v", parsed["exit_code"])
		}
	})

	// Blocked command
	t.Run("blocked_command", func(t *testing.T) {
		_, err := tools[0].Fn(ctx, map[string]string{"command": "rm -rf /"})
		if err == nil {
			t.Error("expected error for blocked command")
		}
		if !strings.Contains(err.Error(), "not in allowlist") {
			t.Errorf("expected 'not in allowlist' error, got %q", err.Error())
		}
	})

	// Shell metacharacter injection blocked
	t.Run("metacharacter_blocked", func(t *testing.T) {
		_, err := tools[0].Fn(ctx, map[string]string{"command": "echo hello; rm -rf /"})
		if err == nil {
			t.Error("expected error for metacharacter")
		}
		if !strings.Contains(err.Error(), "metacharacter") {
			t.Errorf("expected 'metacharacter' error, got %q", err.Error())
		}
	})

	// Empty command
	t.Run("empty_command", func(t *testing.T) {
		_, err := tools[0].Fn(ctx, map[string]string{"command": ""})
		if err == nil {
			t.Error("expected error for empty command")
		}
	})
}

func TestShellWithConfig(t *testing.T) {
	tools := ShellWithConfig(ShellConfig{DefaultTimeout: 5})
	ctx := context.Background()

	result, err := tools[0].Fn(ctx, map[string]string{"command": "echo test"})
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("cannot parse result JSON: %v", err)
	}
	if !strings.Contains(parsed["stdout"].(string), "test") {
		t.Errorf("stdout = %q", parsed["stdout"])
	}
}

func TestShellExitCode(t *testing.T) {
	tools := ShellWithConfig(ShellConfig{})
	ctx := context.Background()

	result, err := tools[0].Fn(ctx, map[string]string{"command": "exit 42"})
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("cannot parse result JSON: %v", err)
	}
	if parsed["exit_code"].(float64) != 42 {
		t.Errorf("exit_code = %v, want 42", parsed["exit_code"])
	}
}

func TestShellWorkingDir(t *testing.T) {
	tools := ShellWithConfig(ShellConfig{})
	ctx := context.Background()
	tmpDir := t.TempDir()

	result, err := tools[0].Fn(ctx, map[string]string{
		"command":     "pwd",
		"working_dir": tmpDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	json.Unmarshal([]byte(result), &parsed)
	// The output should contain the tmpDir path (resolving symlinks)
	stdout := strings.TrimSpace(parsed["stdout"].(string))
	// On macOS, /tmp may be a symlink to /private/tmp
	realTmp, _ := filepath.EvalSymlinks(tmpDir)
	if stdout != realTmp && stdout != tmpDir {
		t.Errorf("working dir stdout = %q, want %q or %q", stdout, tmpDir, realTmp)
	}
}

// --- Truncation Tests ---

func TestTruncateHeadTail(t *testing.T) {
	short := "hello"
	if got := truncateHeadTail(short, 100, 50, 30); got != short {
		t.Errorf("short string should not be truncated, got %q", got)
	}

	long := strings.Repeat("a", 5000)
	result := truncateHeadTail(long, 4000, 1000, 500)
	if !strings.HasPrefix(result, strings.Repeat("a", 1000)) {
		t.Error("truncated string should start with 1000 a's")
	}
	if !strings.HasSuffix(result, strings.Repeat("a", 500)) {
		t.Error("truncated string should end with 500 a's")
	}
	if !strings.Contains(result, "truncated") {
		t.Error("truncated string should contain truncation marker")
	}
}

// --- SQL Validation Tests ---

func TestValidateReadOnly(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{"select", "SELECT * FROM users", false},
		{"with", "WITH cte AS (SELECT 1) SELECT * FROM cte", false},
		{"insert", "INSERT INTO users VALUES (1)", true},
		{"update", "UPDATE users SET name='x'", true},
		{"delete", "DELETE FROM users", true},
		{"drop", "DROP TABLE users", true},
		{"alter", "ALTER TABLE users ADD col INT", true},
		{"create", "CREATE TABLE t (id INT)", true},
		{"truncate", "TRUNCATE TABLE users", true},
		{"semicolon", "SELECT 1; DROP TABLE users", true},
		{"select_into", "SELECT * INTO backup FROM users", true},
		{"case_insensitive", "select * from users", false},
		{"whitespace", "  SELECT 1  ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateReadOnly(tt.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateReadOnly(%q) error = %v, wantErr %v", tt.query, err, tt.wantErr)
			}
		})
	}
}

// --- GitHub Tests ---

func TestGitHubConfig(t *testing.T) {
	tools := GitHub("test-token", GitHubConfig{
		BaseURL:        "https://github.example.com/api/v3",
		DefaultPerPage: 20,
	})
	if len(tools) != 7 {
		t.Fatalf("expected 7 tools, got %d", len(tools))
	}
	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.Name] = true
	}
	for _, expected := range []string{"github_search_repos", "github_get_repo", "github_list_issues", "github_create_issue", "github_get_file", "github_list_pulls", "github_get_pull"} {
		if !names[expected] {
			t.Errorf("missing tool: %s", expected)
		}
	}
}

// --- Slack Tests ---

func TestSlackToolCount(t *testing.T) {
	tools := Slack("test-token")
	expected := []string{"slack_send_message", "slack_reply", "slack_react", "slack_list_channels", "slack_read_messages"}
	if len(tools) != len(expected) {
		t.Fatalf("expected %d tools, got %d", len(expected), len(tools))
	}
	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.Name] = true
	}
	for _, e := range expected {
		if !names[e] {
			t.Errorf("missing tool: %s", e)
		}
	}
}

// --- Docker Tests ---

func TestDockerToolCount(t *testing.T) {
	tools := Docker()
	expected := []string{"docker_ps", "docker_run", "docker_stop", "docker_logs", "docker_images", "docker_exec", "docker_inspect", "docker_build"}
	if len(tools) != len(expected) {
		t.Fatalf("expected %d tools, got %d", len(expected), len(tools))
	}
	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.Name] = true
	}
	for _, e := range expected {
		if !names[e] {
			t.Errorf("missing tool: %s", e)
		}
	}
}

// --- JSON Path Navigation Tests ---

func TestNavigateJSON(t *testing.T) {
	data := map[string]any{
		"a": map[string]any{
			"b": []any{
				map[string]any{"c": "found"},
				map[string]any{"c": "second"},
			},
		},
		"simple": "value",
	}

	tests := []struct {
		path string
		want any
	}{
		{"simple", "value"},
		{"a.b[0].c", "found"},
		{"a.b[1].c", "second"},
		{"a.b[99].c", nil},
		{"nonexistent", nil},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := navigateJSON(data, tt.path)
			if got != tt.want {
				t.Errorf("navigateJSON(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// --- Deep Merge Tests ---

func TestDeepMerge(t *testing.T) {
	base := map[string]any{
		"a": 1,
		"b": map[string]any{
			"c": 2,
			"d": 3,
		},
	}
	overlay := map[string]any{
		"b": map[string]any{
			"c": 99,
			"e": 4,
		},
		"f": 5,
	}

	result := deepMerge(base, overlay).(map[string]any)
	if result["a"].(int) != 1 {
		t.Errorf("a = %v", result["a"])
	}
	b := result["b"].(map[string]any)
	if b["c"].(int) != 99 {
		t.Errorf("b.c = %v", b["c"])
	}
	if b["d"].(int) != 3 {
		t.Errorf("b.d = %v", b["d"])
	}
	if b["e"].(int) != 4 {
		t.Errorf("b.e = %v", b["e"])
	}
	if result["f"].(int) != 5 {
		t.Errorf("f = %v", result["f"])
	}

	// Non-map overlay replaces base
	got := deepMerge("base", "overlay")
	if got != "overlay" {
		t.Errorf("non-map merge = %v", got)
	}
}
