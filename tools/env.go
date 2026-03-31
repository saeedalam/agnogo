package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/saeedalam/agnogo"
)

// Env returns tools for reading environment variables, restricted to an allowlist.
// If no allowlist patterns are given, no variables are returned (secure by default).
func Env(allowlist ...string) []agnogo.ToolDef {
	matchesAllowlist := func(name string) bool {
		for _, pattern := range allowlist {
			if matched, _ := filepath.Match(pattern, name); matched {
				return true
			}
		}
		return false
	}

	return []agnogo.ToolDef{
		{
			Name: "env_get", Desc: "Get the value of an environment variable (restricted to allowlist)",
			Params: agnogo.Params{
				"name": {Type: "string", Desc: "Environment variable name", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				name := args["name"]
				if name == "" {
					return "", fmt.Errorf("name is required")
				}
				if !matchesAllowlist(name) {
					return "", fmt.Errorf("variable %q is not in the allowlist", name)
				}
				val, ok := os.LookupEnv(name)
				if !ok {
					return fmt.Sprintf(`{"name":"%s","found":false,"value":""}`, name), nil
				}
				result := map[string]any{"name": name, "found": true, "value": val}
				out, _ := json.Marshal(result)
				return string(out), nil
			},
		},
		{
			Name: "env_list", Desc: "List environment variables matching the allowlist",
			Params: agnogo.Params{},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				if len(allowlist) == 0 {
					return `{"variables":{}}`, nil
				}
				vars := map[string]string{}
				for _, env := range os.Environ() {
					parts := strings.SplitN(env, "=", 2)
					if len(parts) != 2 {
						continue
					}
					if matchesAllowlist(parts[0]) {
						vars[parts[0]] = parts[1]
					}
				}
				result := map[string]any{"variables": vars}
				out, _ := json.Marshal(result)
				return string(out), nil
			},
		},
	}
}
