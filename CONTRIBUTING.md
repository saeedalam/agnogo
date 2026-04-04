# Contributing to agnogo

## Quick Start

```bash
git clone https://github.com/saeedalam/agnogo
cd agnogo
go test ./...           # run tests
go test -race ./...     # run with race detector
```

## Rules

1. **Zero external dependencies.** Everything uses Go stdlib only. No exceptions.
2. **Every feature has tests.** No PR without tests. Run `go test -race ./...` before submitting.
3. **Backward compatible.** Don't break existing APIs. Add, don't change.

## Adding a Tool

1. Create `tools/your_tool.go` (core) or `tools/contrib/your_tool.go` (community)
2. Return `[]ToolDef` from a constructor function
3. Add tests in `tools/your_tool_test.go`
4. Add to the tool list in GUIDE.md

```go
// tools/your_tool.go
func YourTool() []agnogo.ToolDef {
    return []agnogo.ToolDef{{
        Name: "your_tool",
        Desc: "What it does",
        Params: agnogo.Params{"input": {Type: "string", Required: true}},
        Fn: func(ctx context.Context, args map[string]string) (string, error) {
            return doSomething(args["input"])
        },
    }}
}
```

## Adding a Provider

1. Create `providers/your_provider/your_provider.go`
2. Implement `ModelProvider` interface
3. Register with `RegisterProvider()` for auto-detection
4. Add tests

## Code Style

- `go vet ./...` must pass
- `go test -race ./...` must pass
- Public functions need godoc comments
- Error messages start with `agnogo:`

## Reporting Issues

Open an issue at [github.com/saeedalam/agnogo/issues](https://github.com/saeedalam/agnogo/issues).

For security vulnerabilities, email directly — do not open a public issue.
