package contrib

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/saeedalam/agnogo"
)

// Base64 returns tools for base64 encoding and decoding.
func Base64() []agnogo.ToolDef {
	return []agnogo.ToolDef{
		{
			Name: "base64_encode", Desc: "Encode text to base64",
			Params: agnogo.Params{
				"data":     {Type: "string", Desc: "Data to encode", Required: true},
				"encoding": {Type: "string", Desc: "Encoding type: standard (default) or url", Enum: []string{"standard", "url"}},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				data := args["data"]
				if data == "" {
					return "", fmt.Errorf("data is required")
				}
				enc := base64.StdEncoding
				if args["encoding"] == "url" {
					enc = base64.URLEncoding
				}
				return enc.EncodeToString([]byte(data)), nil
			},
		},
		{
			Name: "base64_decode", Desc: "Decode base64 text",
			Params: agnogo.Params{
				"data":     {Type: "string", Desc: "Base64 encoded data to decode", Required: true},
				"encoding": {Type: "string", Desc: "Encoding type: standard (default) or url", Enum: []string{"standard", "url"}},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				data := args["data"]
				if data == "" {
					return "", fmt.Errorf("data is required")
				}
				enc := base64.StdEncoding
				if args["encoding"] == "url" {
					enc = base64.URLEncoding
				}
				decoded, err := enc.DecodeString(data)
				if err != nil {
					return "", fmt.Errorf("invalid base64: %w", err)
				}
				return string(decoded), nil
			},
		},
	}
}
