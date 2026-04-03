package agnogo

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ── Multi-Modal Media Types ──────────────────────────────────────────
//
// Images, audio, and files can be attached to messages for multi-modal
// LLM processing. Each provider formats them differently:
//   - OpenAI: content array with image_url objects
//   - Anthropic: content array with image/document blocks
//   - Gemini: parts array with inline_data
//
// Usage:
//
//	session.AddMediaMessage("user", "What's in this image?",
//	    []agnogo.Image{agnogo.ImageFromURL("https://example.com/photo.jpg")},
//	    nil, nil,
//	)
//	resp, _ := agent.Run(ctx, session, "")

// Image represents an image for multi-modal LLM input.
type Image struct {
	URL      string `json:"url,omitempty"`       // remote URL
	Path     string `json:"path,omitempty"`      // local file path
	Content  []byte `json:"content,omitempty"`   // raw bytes
	MimeType string `json:"mime_type,omitempty"` // auto-detected if empty
	Detail   string `json:"detail,omitempty"`    // "low", "high", "auto" (OpenAI)
}

// Audio represents audio content for speech-enabled models.
type Audio struct {
	URL      string `json:"url,omitempty"`
	Path     string `json:"path,omitempty"`
	Content  []byte `json:"content,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
	Format   string `json:"format,omitempty"` // "wav", "mp3", "flac" (OpenAI input_audio)
}

// File represents a document (PDF, text, etc.) for document-understanding models.
type File struct {
	URL      string `json:"url,omitempty"`
	Path     string `json:"path,omitempty"`
	Content  []byte `json:"content,omitempty"`
	Name     string `json:"name,omitempty"`      // filename
	MimeType string `json:"mime_type,omitempty"` // "application/pdf", etc.
}

// ── Convenience Constructors ─────────────────────────────────────────

// ImageFromURL creates an Image from a remote URL.
func ImageFromURL(url string) Image {
	return Image{URL: url}
}

// ImageFromFile creates an Image from a local file path.
func ImageFromFile(path string) Image {
	return Image{Path: path}
}

// ImageFromBytes creates an Image from raw bytes with explicit MIME type.
func ImageFromBytes(data []byte, mimeType string) Image {
	return Image{Content: data, MimeType: mimeType}
}

// AudioFromFile creates an Audio from a local file path.
func AudioFromFile(path string) Audio {
	mime := mimeFromExtension(path)
	format := strings.TrimPrefix(filepath.Ext(path), ".")
	return Audio{Path: path, MimeType: mime, Format: format}
}

// FileFromPath creates a File from a local file path.
func FileFromPath(path string) File {
	return File{
		Path:     path,
		Name:     filepath.Base(path),
		MimeType: mimeFromExtension(path),
	}
}

// ── Media Resolution ─────────────────────────────────────────────────

// resolveMediaBytes loads content from URL, Path, or Content.
// Returns raw bytes and resolved MIME type.
func resolveMediaBytes(url, path string, content []byte, mimeType string) ([]byte, string, error) {
	if len(content) > 0 {
		if mimeType == "" {
			mimeType = detectImageMime(content)
		}
		return content, mimeType, nil
	}

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, "", fmt.Errorf("agnogo: read media file %q: %w", path, err)
		}
		if mimeType == "" {
			mimeType = mimeFromExtension(path)
			if mimeType == "" {
				mimeType = detectImageMime(data)
			}
		}
		return data, mimeType, nil
	}

	if url != "" {
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Get(url) //nolint:gosec // user-provided URL
		if err != nil {
			return nil, "", fmt.Errorf("agnogo: fetch media URL %q: %w", url, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, "", fmt.Errorf("agnogo: fetch media URL %q: HTTP %d", url, resp.StatusCode)
		}
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, "", fmt.Errorf("agnogo: read media response: %w", err)
		}
		if mimeType == "" {
			ct := resp.Header.Get("Content-Type")
			// Strip charset and parameters: "image/jpeg; charset=utf-8" → "image/jpeg"
			if idx := strings.Index(ct, ";"); idx >= 0 {
				ct = strings.TrimSpace(ct[:idx])
			}
			if ct != "" {
				mimeType = ct
			} else {
				mimeType = detectImageMime(data)
			}
		}
		return data, mimeType, nil
	}

	return nil, "", fmt.Errorf("agnogo: media has no URL, Path, or Content")
}

// encodeMediaBase64 resolves media bytes and returns a base64-encoded string
// along with the MIME type.
func encodeMediaBase64(url, path string, content []byte, mimeType string) (string, string, error) {
	data, mime, err := resolveMediaBytes(url, path, content, mimeType)
	if err != nil {
		return "", "", err
	}
	return base64.StdEncoding.EncodeToString(data), mime, nil
}

// ── MIME Detection ───────────────────────────────────────────────────

// detectImageMime detects image MIME type from magic bytes.
// Returns "image/jpeg" as fallback for unknown formats.
func detectImageMime(data []byte) string {
	if len(data) < 4 {
		return "image/jpeg"
	}
	// JPEG: FF D8 FF
	if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg"
	}
	// PNG: 89 50 4E 47
	if data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G' {
		return "image/png"
	}
	// GIF: 47 49 46 38
	if data[0] == 'G' && data[1] == 'I' && data[2] == 'F' && data[3] == '8' {
		return "image/gif"
	}
	// WebP: RIFF....WEBP
	if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "image/webp"
	}
	// PDF: %PDF
	if data[0] == '%' && data[1] == 'P' && data[2] == 'D' && data[3] == 'F' {
		return "application/pdf"
	}
	return "image/jpeg"
}

// mimeFromExtension returns MIME type based on file extension.
func mimeFromExtension(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".pdf":
		return "application/pdf"
	case ".txt":
		return "text/plain"
	case ".html", ".htm":
		return "text/html"
	case ".json":
		return "application/json"
	case ".csv":
		return "text/csv"
	case ".md":
		return "text/markdown"
	case ".wav":
		return "audio/wav"
	case ".mp3":
		return "audio/mpeg"
	case ".flac":
		return "audio/flac"
	case ".ogg":
		return "audio/ogg"
	case ".mp4":
		return "video/mp4"
	default:
		return "application/octet-stream"
	}
}
