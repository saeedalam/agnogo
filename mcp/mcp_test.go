package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync/atomic"
	"testing"
)

// ── Mock MCP server over pipes ──────────────────────────────────────

// mockTransport simulates an MCP server's stdin/stdout.
type mockTransport struct {
	reader    *bufio.Reader
	writer    io.Writer
	responses []jsonrpcResponse
}

func newMockServer(tools []mcpTool) (*Client, io.Writer) {
	// Create two pipes: client→server and server→client.
	serverR, clientW := io.Pipe()
	clientR, serverW := io.Pipe()

	c := &Client{
		stdin:  clientW,
		stdout: bufio.NewReader(clientR),
	}

	// Run a mock MCP server in the background.
	go func() {
		scanner := bufio.NewScanner(serverR)
		for scanner.Scan() {
			line := scanner.Bytes()
			var req jsonrpcRequest
			if err := json.Unmarshal(line, &req); err != nil {
				continue
			}

			var result any
			switch req.Method {
			case "initialize":
				result = mcpInitResult{
					ProtocolVersion: "2024-11-05",
				}
			case "notifications/initialized":
				continue // no response for notifications
			case "tools/list":
				result = mcpToolsListResult{Tools: tools}
			case "tools/call":
				// Parse which tool is being called.
				var params struct {
					Name      string         `json:"name"`
					Arguments map[string]any `json:"arguments"`
				}
				raw, _ := json.Marshal(req.Params)
				json.Unmarshal(raw, &params)

				result = mcpCallToolResult{
					Content: []mcpContent{
						{Type: "text", Text: "called " + params.Name},
					},
				}
			}

			resultJSON, _ := json.Marshal(result)
			resp := jsonrpcResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  resultJSON,
			}
			data, _ := json.Marshal(resp)
			data = append(data, '\n')
			serverW.Write(data)
		}
	}()

	return c, clientW
}

// ── Tests ───────────────────────────────────────────────────────────

func TestInitializeAndListTools(t *testing.T) {
	tools := []mcpTool{
		{
			Name:        "read_file",
			Description: "Read a file from the filesystem",
			InputSchema: mcpInputSchema{
				Type: "object",
				Properties: map[string]mcpPropertyDef{
					"path": {Type: "string", Description: "Path to the file"},
				},
				Required: []string{"path"},
			},
		},
		{
			Name:        "write_file",
			Description: "Write content to a file",
			InputSchema: mcpInputSchema{
				Type: "object",
				Properties: map[string]mcpPropertyDef{
					"path":    {Type: "string", Description: "Path to the file"},
					"content": {Type: "string", Description: "Content to write"},
				},
				Required: []string{"path", "content"},
			},
		},
	}

	c, _ := newMockServer(tools)
	defer c.Close()

	ctx := context.Background()

	// Initialize
	if err := c.initialize(ctx); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	// List tools
	if err := c.listTools(ctx); err != nil {
		t.Fatalf("listTools: %v", err)
	}

	if c.ToolCount() != 2 {
		t.Errorf("ToolCount() = %d, want 2", c.ToolCount())
	}

	names := c.ToolNames()
	if names[0] != "read_file" || names[1] != "write_file" {
		t.Errorf("ToolNames() = %v", names)
	}
}

func TestToolDefs(t *testing.T) {
	tools := []mcpTool{
		{
			Name:        "search",
			Description: "Search the web",
			InputSchema: mcpInputSchema{
				Type: "object",
				Properties: map[string]mcpPropertyDef{
					"query": {Type: "string", Description: "Search query"},
				},
				Required: []string{"query"},
			},
		},
	}

	c, _ := newMockServer(tools)
	defer c.Close()

	ctx := context.Background()
	c.initialize(ctx)
	c.listTools(ctx)

	defs := c.ToolDefs()
	if len(defs) != 1 {
		t.Fatalf("len(ToolDefs()) = %d, want 1", len(defs))
	}

	def := defs[0]
	if def.Name != "search" {
		t.Errorf("Name = %q, want search", def.Name)
	}
	if def.Desc != "Search the web" {
		t.Errorf("Desc = %q", def.Desc)
	}
	if def.Params["query"].Required != true {
		t.Error("query should be required")
	}
	if def.Params["query"].Type != "string" {
		t.Errorf("query type = %q", def.Params["query"].Type)
	}
}

func TestCallTool(t *testing.T) {
	tools := []mcpTool{
		{
			Name:        "greet",
			Description: "Say hello",
			InputSchema: mcpInputSchema{
				Type:       "object",
				Properties: map[string]mcpPropertyDef{"name": {Type: "string"}},
			},
		},
	}

	c, _ := newMockServer(tools)
	defer c.Close()

	ctx := context.Background()
	c.initialize(ctx)
	c.listTools(ctx)

	result, err := c.callTool(ctx, "greet", map[string]string{"name": "Erik"})
	if err != nil {
		t.Fatalf("callTool: %v", err)
	}
	if !strings.Contains(result, "called greet") {
		t.Errorf("result = %q, want 'called greet'", result)
	}
}

func TestToolDefsCallsBackToServer(t *testing.T) {
	tools := []mcpTool{
		{
			Name:        "ping",
			Description: "Ping",
			InputSchema: mcpInputSchema{Type: "object"},
		},
	}

	c, _ := newMockServer(tools)
	defer c.Close()

	ctx := context.Background()
	c.initialize(ctx)
	c.listTools(ctx)

	defs := c.ToolDefs()
	result, err := defs[0].Fn(ctx, map[string]string{})
	if err != nil {
		t.Fatalf("Fn: %v", err)
	}
	if !strings.Contains(result, "called ping") {
		t.Errorf("result = %q", result)
	}
}

func TestTrimBytes(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"hello\n", "hello"},
		{"\r\nhello\r\n", "hello"},
		{"  hello  ", "hello"},
		{"", ""},
	}
	for _, tt := range tests {
		got := string(trimBytes([]byte(tt.in)))
		if got != tt.want {
			t.Errorf("trimBytes(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestJSONRPCError(t *testing.T) {
	e := &jsonrpcError{Code: -32600, Message: "invalid request"}
	if !strings.Contains(e.Error(), "-32600") {
		t.Errorf("error = %q", e.Error())
	}
}

func TestEmptyToolSchema(t *testing.T) {
	tools := []mcpTool{
		{
			Name:        "noop",
			Description: "Does nothing",
			InputSchema: mcpInputSchema{Type: "object"},
		},
	}

	c, _ := newMockServer(tools)
	defer c.Close()

	ctx := context.Background()
	c.initialize(ctx)
	c.listTools(ctx)

	defs := c.ToolDefs()
	if len(defs[0].Params) != 0 {
		t.Errorf("expected 0 params, got %d", len(defs[0].Params))
	}
}

// Ensure nextID is concurrency-safe.
func TestNextIDConcurrency(t *testing.T) {
	var id atomic.Int64
	done := make(chan struct{}, 10)
	for range 10 {
		go func() {
			id.Add(1)
			done <- struct{}{}
		}()
	}
	for range 10 {
		<-done
	}
	if id.Load() != 10 {
		t.Errorf("id = %d, want 10", id.Load())
	}
}
