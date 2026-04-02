package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"

	"github.com/saeedalam/agnogo"
)

// Client connects to an MCP server, discovers its tools, and bridges them
// to agnogo ToolDef format. Implements JSON-RPC 2.0 over stdio.
type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	mu     sync.Mutex
	nextID atomic.Int64
	tools  []mcpTool
}

// ── JSON-RPC 2.0 types ─────────────────────────────────────────────

type jsonrpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *jsonrpcError) Error() string {
	return fmt.Sprintf("MCP error %d: %s", e.Code, e.Message)
}

// ── MCP protocol types ──────────────────────────────────────────────

type mcpInitResult struct {
	ProtocolVersion string `json:"protocolVersion"`
	ServerInfo      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"serverInfo"`
	Capabilities struct {
		Tools *struct{} `json:"tools,omitempty"`
	} `json:"capabilities"`
}

type mcpTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema mcpInputSchema `json:"inputSchema"`
}

type mcpInputSchema struct {
	Type       string                    `json:"type"`
	Properties map[string]mcpPropertyDef `json:"properties"`
	Required   []string                  `json:"required"`
}

type mcpPropertyDef struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type mcpToolsListResult struct {
	Tools []mcpTool `json:"tools"`
}

type mcpCallToolResult struct {
	Content []mcpContent `json:"content"`
	IsError bool         `json:"isError"`
}

type mcpContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ── Connect ─────────────────────────────────────────────────────────

// Connect starts an MCP server as a subprocess and connects over stdio.
// The first argument is the command, followed by its arguments.
//
//	tools, err := mcp.Connect(ctx, "npx", "-y", "@modelcontextprotocol/server-filesystem", "/tmp")
//	defer tools.Close()
//	agent := agnogo.Agent("...", agnogo.Tools(tools.ToolDefs()...))
func Connect(ctx context.Context, command string, args ...string) (*Client, error) {
	cmd := exec.CommandContext(ctx, command, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp: stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp: stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("mcp: start %q: %w", command, err)
	}

	c := &Client{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewReader(stdout),
	}

	// Initialize the MCP session.
	if err := c.initialize(ctx); err != nil {
		c.Close()
		return nil, err
	}

	// Discover tools.
	if err := c.listTools(ctx); err != nil {
		c.Close()
		return nil, err
	}

	return c, nil
}

// Close shuts down the MCP server subprocess.
func (c *Client) Close() error {
	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
		c.cmd.Wait()
	}
	return nil
}

// ── Public API ──────────────────────────────────────────────────────

// ToolDefs returns the MCP server's tools as agnogo ToolDefs.
// Each tool invocation calls back to the MCP server over stdio.
func (c *Client) ToolDefs() []agnogo.ToolDef {
	defs := make([]agnogo.ToolDef, len(c.tools))
	for i, t := range c.tools {
		tool := t // capture
		params := make(agnogo.Params)
		for name, prop := range tool.InputSchema.Properties {
			required := false
			for _, r := range tool.InputSchema.Required {
				if r == name {
					required = true
					break
				}
			}
			params[name] = agnogo.Param{
				Type:     prop.Type,
				Desc:     prop.Description,
				Required: required,
			}
		}

		defs[i] = agnogo.ToolDef{
			Name:   tool.Name,
			Desc:   tool.Description,
			Params: params,
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				return c.callTool(ctx, tool.Name, args)
			},
		}
	}
	return defs
}

// ToolCount returns the number of discovered tools.
func (c *Client) ToolCount() int {
	return len(c.tools)
}

// ToolNames returns the names of all discovered tools.
func (c *Client) ToolNames() []string {
	names := make([]string, len(c.tools))
	for i, t := range c.tools {
		names[i] = t.Name
	}
	return names
}

// ── JSON-RPC communication ──────────────────────────────────────────

func (c *Client) send(req jsonrpcRequest) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = c.stdin.Write(data)
	return err
}

func (c *Client) receive() (*jsonrpcResponse, error) {
	for {
		line, err := c.stdout.ReadBytes('\n')
		if err != nil {
			return nil, fmt.Errorf("mcp: read: %w", err)
		}
		// Skip empty lines and notifications (no id)
		line = trimBytes(line)
		if len(line) == 0 {
			continue
		}
		var resp jsonrpcResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			continue // skip malformed lines
		}
		if resp.JSONRPC == "" {
			continue // skip non-JSON-RPC lines
		}
		return &resp, nil
	}
}

func (c *Client) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := c.nextID.Add(1)
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
	if err := c.send(req); err != nil {
		return nil, fmt.Errorf("mcp: send %s: %w", method, err)
	}

	// Read responses until we get our ID.
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		resp, err := c.receive()
		if err != nil {
			return nil, err
		}
		if resp.ID != id {
			continue // out-of-order response, skip
		}
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp.Result, nil
	}
}

// ── MCP protocol methods ────────────────────────────────────────────

func (c *Client) initialize(ctx context.Context) error {
	params := map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "agnogo",
			"version": "0.4.0",
		},
	}

	result, err := c.call(ctx, "initialize", params)
	if err != nil {
		return fmt.Errorf("mcp: initialize: %w", err)
	}

	var init mcpInitResult
	if err := json.Unmarshal(result, &init); err != nil {
		return fmt.Errorf("mcp: parse init result: %w", err)
	}

	// Send initialized notification (no response expected).
	notif := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      0,
		Method:  "notifications/initialized",
	}
	return c.send(notif)
}

func (c *Client) listTools(ctx context.Context) error {
	result, err := c.call(ctx, "tools/list", nil)
	if err != nil {
		return fmt.Errorf("mcp: list tools: %w", err)
	}

	var toolsResult mcpToolsListResult
	if err := json.Unmarshal(result, &toolsResult); err != nil {
		return fmt.Errorf("mcp: parse tools: %w", err)
	}

	c.tools = toolsResult.Tools
	return nil
}

func (c *Client) callTool(ctx context.Context, name string, args map[string]string) (string, error) {
	// Convert string args to any for JSON-RPC.
	arguments := make(map[string]any, len(args))
	for k, v := range args {
		arguments[k] = v
	}

	params := map[string]any{
		"name":      name,
		"arguments": arguments,
	}

	result, err := c.call(ctx, "tools/call", params)
	if err != nil {
		return "", fmt.Errorf("mcp: call %s: %w", name, err)
	}

	var callResult mcpCallToolResult
	if err := json.Unmarshal(result, &callResult); err != nil {
		return "", fmt.Errorf("mcp: parse call result: %w", err)
	}

	if callResult.IsError {
		var errText string
		for _, c := range callResult.Content {
			if c.Type == "text" {
				errText += c.Text
			}
		}
		return "", fmt.Errorf("mcp tool %s: %s", name, errText)
	}

	// Concatenate text content.
	var text string
	for _, c := range callResult.Content {
		if c.Type == "text" {
			if text != "" {
				text += "\n"
			}
			text += c.Text
		}
	}
	return text, nil
}

// ── Helpers ─────────────────────────────────────────────────────────

func trimBytes(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r' || b[len(b)-1] == ' ') {
		b = b[:len(b)-1]
	}
	for len(b) > 0 && (b[0] == '\n' || b[0] == '\r' || b[0] == ' ') {
		b = b[1:]
	}
	return b
}
