package tools

import (
	"context"
	"crypto/rand"
	"fmt"

	"github.com/saeedalam/agnogo"
)

// UUID returns a tool for generating V4 UUIDs.
func UUID() []agnogo.ToolDef {
	return []agnogo.ToolDef{
		{
			Name: "uuid_generate", Desc: "Generate a random UUID v4",
			Params: agnogo.Params{},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				var b [16]byte
				if _, err := rand.Read(b[:]); err != nil {
					return "", fmt.Errorf("failed to generate UUID: %w", err)
				}
				b[6] = (b[6] & 0x0f) | 0x40 // version 4
				b[8] = (b[8] & 0x3f) | 0x80 // variant 10
				uuid := fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
					b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
				return fmt.Sprintf(`{"uuid":"%s"}`, uuid), nil
			},
		},
	}
}
