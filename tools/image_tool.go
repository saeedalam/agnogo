package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/saeedalam/agnogo"
)

// ImageTool returns tools for image inspection and manipulation.
// Supports PNG and JPEG. Uses bilinear interpolation for resize.
func ImageTool() []agnogo.ToolDef {
	return []agnogo.ToolDef{
		{
			Name: "image_info",
			Desc: "Get image dimensions, format, file size, color model, and bit depth.",
			Params: agnogo.Params{
				"path": {Type: "string", Desc: "Path to image file (PNG or JPEG)", Required: true},
			},
			Fn: imageInfoFn,
		},
		{
			Name: "image_resize",
			Desc: "Resize an image using bilinear interpolation. Specify width, height, or both. Maintains aspect ratio if only one dimension is given.",
			Params: agnogo.Params{
				"path":   {Type: "string", Desc: "Path to source image file", Required: true},
				"output": {Type: "string", Desc: "Path to output image file", Required: true},
				"width":  {Type: "string", Desc: "Target width in pixels"},
				"height": {Type: "string", Desc: "Target height in pixels"},
			},
			Fn: imageResizeFn,
		},
		{
			Name: "image_crop",
			Desc: "Crop an image to a rectangular region.",
			Params: agnogo.Params{
				"path":   {Type: "string", Desc: "Path to source image file", Required: true},
				"output": {Type: "string", Desc: "Path to output image file", Required: true},
				"x":      {Type: "string", Desc: "Left edge X coordinate", Required: true},
				"y":      {Type: "string", Desc: "Top edge Y coordinate", Required: true},
				"width":  {Type: "string", Desc: "Crop width in pixels", Required: true},
				"height": {Type: "string", Desc: "Crop height in pixels", Required: true},
			},
			Fn: imageCropFn,
		},
		{
			Name: "image_convert",
			Desc: "Convert an image between PNG and JPEG formats.",
			Params: agnogo.Params{
				"path":    {Type: "string", Desc: "Path to source image file", Required: true},
				"output":  {Type: "string", Desc: "Path to output image file (extension determines format)", Required: true},
				"quality": {Type: "string", Desc: "JPEG quality 1-100 (default 90, only for JPEG output)"},
			},
			Fn: imageConvertFn,
		},
	}
}

func imageInfoFn(ctx context.Context, args map[string]string) (string, error) {
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

	colorModel, bitDepth := describeColorModel(cfg.ColorModel)

	result := map[string]any{
		"width":       cfg.Width,
		"height":      cfg.Height,
		"format":      format,
		"file_size":   fi.Size(),
		"color_model": colorModel,
		"bit_depth":   bitDepth,
	}
	out, _ := json.MarshalIndent(result, "", "  ")
	return string(out), nil
}

func describeColorModel(m color.Model) (string, int) {
	switch m {
	case color.RGBAModel, color.NRGBAModel:
		return "RGBA", 32
	case color.RGBA64Model, color.NRGBA64Model:
		return "RGBA64", 64
	case color.GrayModel:
		return "Grayscale", 8
	case color.Gray16Model:
		return "Grayscale16", 16
	case color.AlphaModel:
		return "Alpha", 8
	case color.Alpha16Model:
		return "Alpha16", 16
	case color.YCbCrModel:
		return "YCbCr", 24
	case color.CMYKModel:
		return "CMYK", 32
	default:
		return "unknown", 0
	}
}

func imageResizeFn(ctx context.Context, args map[string]string) (string, error) {
	path := args["path"]
	output := args["output"]
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	if output == "" {
		return "", fmt.Errorf("output is required")
	}

	src, format, err := loadImage(path)
	if err != nil {
		return "", err
	}

	srcBounds := src.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()

	dstW, dstH, err := computeResizeDimensions(srcW, srcH, args["width"], args["height"])
	if err != nil {
		return "", err
	}

	dst := bilinearResize(src, dstW, dstH)

	outFormat := detectFormat(output, format)
	if err := saveImage(dst, output, outFormat, 90); err != nil {
		return "", err
	}

	result := map[string]any{
		"output":         output,
		"original_width": srcW, "original_height": srcH,
		"new_width": dstW, "new_height": dstH,
		"format": outFormat,
	}
	out, _ := json.MarshalIndent(result, "", "  ")
	return string(out), nil
}

func imageCropFn(ctx context.Context, args map[string]string) (string, error) {
	path := args["path"]
	output := args["output"]
	if path == "" || output == "" {
		return "", fmt.Errorf("path and output are required")
	}

	src, format, err := loadImage(path)
	if err != nil {
		return "", err
	}

	x, err := strconv.Atoi(args["x"])
	if err != nil {
		return "", fmt.Errorf("invalid x: %w", err)
	}
	y, err := strconv.Atoi(args["y"])
	if err != nil {
		return "", fmt.Errorf("invalid y: %w", err)
	}
	w, err := strconv.Atoi(args["width"])
	if err != nil || w <= 0 {
		return "", fmt.Errorf("invalid width: must be positive integer")
	}
	h, err := strconv.Atoi(args["height"])
	if err != nil || h <= 0 {
		return "", fmt.Errorf("invalid height: must be positive integer")
	}

	srcBounds := src.Bounds()
	if x < srcBounds.Min.X || y < srcBounds.Min.Y ||
		x+w > srcBounds.Max.X || y+h > srcBounds.Max.Y {
		return "", fmt.Errorf("crop region (%d,%d %dx%d) exceeds image bounds (%dx%d)",
			x, y, w, h, srcBounds.Dx(), srcBounds.Dy())
	}

	cropRect := image.Rect(x, y, x+w, y+h)
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			dst.Set(dx, dy, src.At(cropRect.Min.X+dx, cropRect.Min.Y+dy))
		}
	}

	outFormat := detectFormat(output, format)
	if err := saveImage(dst, output, outFormat, 90); err != nil {
		return "", err
	}

	result := map[string]any{
		"output": output,
		"x": x, "y": y, "width": w, "height": h,
		"format": outFormat,
	}
	out, _ := json.MarshalIndent(result, "", "  ")
	return string(out), nil
}

func imageConvertFn(ctx context.Context, args map[string]string) (string, error) {
	path := args["path"]
	output := args["output"]
	if path == "" || output == "" {
		return "", fmt.Errorf("path and output are required")
	}

	src, _, err := loadImage(path)
	if err != nil {
		return "", err
	}

	quality := 90
	if q := args["quality"]; q != "" {
		quality, err = strconv.Atoi(q)
		if err != nil || quality < 1 || quality > 100 {
			return "", fmt.Errorf("quality must be 1-100")
		}
	}

	outFormat := detectFormat(output, "")
	if outFormat == "" {
		return "", fmt.Errorf("cannot determine output format from extension; use .png or .jpg/.jpeg")
	}

	if err := saveImage(src, output, outFormat, quality); err != nil {
		return "", err
	}

	bounds := src.Bounds()
	result := map[string]any{
		"output": output,
		"format": outFormat,
		"width":  bounds.Dx(),
		"height": bounds.Dy(),
	}
	out, _ := json.MarshalIndent(result, "", "  ")
	return string(out), nil
}

// ---------------------------------------------------------------------------
// Image helpers
// ---------------------------------------------------------------------------

func loadImage(path string) (image.Image, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open image: %w", err)
	}
	defer f.Close()

	img, format, err := image.Decode(f)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode image: %w", err)
	}
	return img, format, nil
}

func saveImage(img image.Image, path, format string, quality int) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer f.Close()

	switch format {
	case "png":
		return png.Encode(f, img)
	case "jpeg", "jpg":
		return jpeg.Encode(f, img, &jpeg.Options{Quality: quality})
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func detectFormat(path, fallback string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png":
		return "png"
	case ".jpg", ".jpeg":
		return "jpeg"
	default:
		return fallback
	}
}

func computeResizeDimensions(srcW, srcH int, widthStr, heightStr string) (int, int, error) {
	var dstW, dstH int
	var err error

	if widthStr != "" {
		dstW, err = strconv.Atoi(widthStr)
		if err != nil || dstW <= 0 {
			return 0, 0, fmt.Errorf("invalid width: must be positive integer")
		}
	}
	if heightStr != "" {
		dstH, err = strconv.Atoi(heightStr)
		if err != nil || dstH <= 0 {
			return 0, 0, fmt.Errorf("invalid height: must be positive integer")
		}
	}

	if dstW == 0 && dstH == 0 {
		return 0, 0, fmt.Errorf("at least one of width or height is required")
	}

	// Maintain aspect ratio
	if dstW == 0 {
		dstW = int(math.Round(float64(srcW) * float64(dstH) / float64(srcH)))
		if dstW < 1 {
			dstW = 1
		}
	}
	if dstH == 0 {
		dstH = int(math.Round(float64(srcH) * float64(dstW) / float64(srcW)))
		if dstH < 1 {
			dstH = 1
		}
	}

	return dstW, dstH, nil
}

// bilinearResize performs bilinear interpolation to resize an image.
func bilinearResize(src image.Image, dstW, dstH int) *image.RGBA {
	srcBounds := src.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))

	xRatio := float64(srcW) / float64(dstW)
	yRatio := float64(srcH) / float64(dstH)

	for dy := 0; dy < dstH; dy++ {
		for dx := 0; dx < dstW; dx++ {
			// Map destination pixel to source coordinates
			sx := (float64(dx)+0.5)*xRatio - 0.5
			sy := (float64(dy)+0.5)*yRatio - 0.5

			// Clamp to valid range
			if sx < 0 {
				sx = 0
			}
			if sy < 0 {
				sy = 0
			}

			x0 := int(sx)
			y0 := int(sy)
			x1 := x0 + 1
			y1 := y0 + 1

			// Clamp to image bounds
			if x0 >= srcW {
				x0 = srcW - 1
			}
			if y0 >= srcH {
				y0 = srcH - 1
			}
			if x1 >= srcW {
				x1 = srcW - 1
			}
			if y1 >= srcH {
				y1 = srcH - 1
			}

			// Fractional parts
			xFrac := sx - float64(x0)
			yFrac := sy - float64(y0)

			// Get the 4 surrounding pixels (as uint32 RGBA)
			r00, g00, b00, a00 := src.At(srcBounds.Min.X+x0, srcBounds.Min.Y+y0).RGBA()
			r10, g10, b10, a10 := src.At(srcBounds.Min.X+x1, srcBounds.Min.Y+y0).RGBA()
			r01, g01, b01, a01 := src.At(srcBounds.Min.X+x0, srcBounds.Min.Y+y1).RGBA()
			r11, g11, b11, a11 := src.At(srcBounds.Min.X+x1, srcBounds.Min.Y+y1).RGBA()

			// Bilinear interpolation
			r := bilinearInterp(r00, r10, r01, r11, xFrac, yFrac)
			g := bilinearInterp(g00, g10, g01, g11, xFrac, yFrac)
			b := bilinearInterp(b00, b10, b01, b11, xFrac, yFrac)
			a := bilinearInterp(a00, a10, a01, a11, xFrac, yFrac)

			dst.SetRGBA(dx, dy, color.RGBA{
				R: uint8(r >> 8),
				G: uint8(g >> 8),
				B: uint8(b >> 8),
				A: uint8(a >> 8),
			})
		}
	}

	return dst
}

// bilinearInterp interpolates between 4 values (16-bit range from RGBA()).
func bilinearInterp(v00, v10, v01, v11 uint32, xFrac, yFrac float64) uint32 {
	top := float64(v00)*(1-xFrac) + float64(v10)*xFrac
	bot := float64(v01)*(1-xFrac) + float64(v11)*xFrac
	val := top*(1-yFrac) + bot*yFrac
	if val < 0 {
		return 0
	}
	if val > 0xFFFF {
		return 0xFFFF
	}
	return uint32(val)
}
