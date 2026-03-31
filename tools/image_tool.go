package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"

	"github.com/saeedalam/agnogo"
)

// ImageTool returns a tool for reading image metadata.
func ImageTool() []agnogo.ToolDef {
	return []agnogo.ToolDef{
		{
			Name: "image_info", Desc: "Get image dimensions, format, and file size",
			Params: agnogo.Params{
				"path": {Type: "string", Desc: "Path to image file (PNG or JPEG)", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				path := args["path"]
				if path == "" {
					return "", fmt.Errorf("path is required")
				}

				fi, err := os.Stat(path)
				if err != nil {
					return "", fmt.Errorf("failed to stat file: %w", err)
				}

				f, err := os.Open(path)
				if err != nil {
					return "", fmt.Errorf("failed to open file: %w", err)
				}
				defer f.Close()

				cfg, format, err := image.DecodeConfig(f)
				if err != nil {
					return "", fmt.Errorf("failed to decode image: %w", err)
				}

				result := map[string]any{
					"width":      cfg.Width,
					"height":     cfg.Height,
					"format":     format,
					"file_size":  fi.Size(),
					"color_model": fmt.Sprintf("%v", cfg.ColorModel),
				}
				out, _ := json.Marshal(result)
				return string(out), nil
			},
		},
	}
}
