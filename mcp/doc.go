// Package mcp provides Model Context Protocol (MCP) integration for agnogo.
//
// MCP is the standard protocol for LLM tool interoperability. This package
// implements an MCP client that discovers tools from MCP servers and bridges
// them to agnogo ToolDef format.
//
// Zero external dependencies — uses JSON-RPC 2.0 over stdio (subprocess)
// or SSE (HTTP), implemented with Go stdlib only.
//
// Usage with agnogo:
//
//	tools, _ := mcp.Connect(ctx, "npx", "-y", "@modelcontextprotocol/server-filesystem", "/tmp")
//	defer tools.Close()
//	agent := agnogo.Agent("...", agnogo.Tools(tools.ToolDefs()...))
//
// Or with an HTTP server:
//
//	tools, _ := mcp.ConnectSSE(ctx, "http://localhost:3001/sse")
//	defer tools.Close()
//	agent := agnogo.Agent("...", agnogo.Tools(tools.ToolDefs()...))
package mcp
