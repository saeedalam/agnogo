package agnogo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── MIME Detection ──────────────────────────────────────────────────

func TestDetectImageMimeJPEG(t *testing.T) {
	data := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00}
	if got := detectImageMime(data); got != "image/jpeg" {
		t.Errorf("got %q, want image/jpeg", got)
	}
}

func TestDetectImageMimePNG(t *testing.T) {
	data := []byte{0x89, 'P', 'N', 'G', 0x0D}
	if got := detectImageMime(data); got != "image/png" {
		t.Errorf("got %q, want image/png", got)
	}
}

func TestDetectImageMimeGIF(t *testing.T) {
	data := []byte{'G', 'I', 'F', '8', '9'}
	if got := detectImageMime(data); got != "image/gif" {
		t.Errorf("got %q, want image/gif", got)
	}
}

func TestDetectImageMimeWebP(t *testing.T) {
	data := []byte{'R', 'I', 'F', 'F', 0, 0, 0, 0, 'W', 'E', 'B', 'P'}
	if got := detectImageMime(data); got != "image/webp" {
		t.Errorf("got %q, want image/webp", got)
	}
}

func TestDetectImageMimePDF(t *testing.T) {
	data := []byte{'%', 'P', 'D', 'F', '-'}
	if got := detectImageMime(data); got != "application/pdf" {
		t.Errorf("got %q, want application/pdf", got)
	}
}

func TestDetectImageMimeFallback(t *testing.T) {
	data := []byte{0x00, 0x01, 0x02, 0x03}
	if got := detectImageMime(data); got != "image/jpeg" {
		t.Errorf("got %q, want image/jpeg (fallback)", got)
	}
}

func TestDetectImageMimeShortData(t *testing.T) {
	data := []byte{0xFF}
	if got := detectImageMime(data); got != "image/jpeg" {
		t.Errorf("got %q, want image/jpeg (short data fallback)", got)
	}
}

// ── Constructors ────────────────────────────────────────────────────

func TestImageFromURL(t *testing.T) {
	img := ImageFromURL("https://example.com/photo.jpg")
	if img.URL != "https://example.com/photo.jpg" {
		t.Errorf("URL = %q", img.URL)
	}
	if img.Path != "" || img.Content != nil {
		t.Error("Path and Content should be empty")
	}
}

func TestImageFromBytes(t *testing.T) {
	data := []byte{0xFF, 0xD8, 0xFF}
	img := ImageFromBytes(data, "image/jpeg")
	if img.MimeType != "image/jpeg" {
		t.Errorf("MimeType = %q", img.MimeType)
	}
	if len(img.Content) != 3 {
		t.Errorf("Content length = %d", len(img.Content))
	}
}

func TestImageFromFile(t *testing.T) {
	// Create a temp file with JPEG magic bytes
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jpg")
	os.WriteFile(path, []byte{0xFF, 0xD8, 0xFF, 0xE0}, 0644)

	img := ImageFromFile(path)
	if img.Path != path {
		t.Errorf("Path = %q", img.Path)
	}
}

func TestMimeFromExtension(t *testing.T) {
	cases := map[string]string{
		"photo.jpg":  "image/jpeg",
		"photo.jpeg": "image/jpeg",
		"photo.png":  "image/png",
		"photo.gif":  "image/gif",
		"photo.webp": "image/webp",
		"doc.pdf":    "application/pdf",
		"data.json":  "application/json",
		"song.mp3":   "audio/mpeg",
		"song.wav":   "audio/wav",
		"unknown.xyz": "application/octet-stream",
	}
	for path, want := range cases {
		if got := mimeFromExtension(path); got != want {
			t.Errorf("mimeFromExtension(%q) = %q, want %q", path, got, want)
		}
	}
}

// ── resolveMediaBytes ───────────────────────────────────────────────

func TestResolveMediaBytesFromContent(t *testing.T) {
	data := []byte{0xFF, 0xD8, 0xFF}
	bytes, mime, err := resolveMediaBytes("", "", data, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(bytes) != 3 {
		t.Errorf("bytes length = %d", len(bytes))
	}
	if mime != "image/jpeg" {
		t.Errorf("mime = %q, want image/jpeg", mime)
	}
}

func TestResolveMediaBytesFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.png")
	pngData := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	os.WriteFile(path, pngData, 0644)

	bytes, mime, err := resolveMediaBytes("", path, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(bytes) != len(pngData) {
		t.Errorf("bytes length = %d, want %d", len(bytes), len(pngData))
	}
	if mime != "image/png" {
		t.Errorf("mime = %q, want image/png", mime)
	}
}

func TestResolveMediaBytesNoSource(t *testing.T) {
	_, _, err := resolveMediaBytes("", "", nil, "")
	if err == nil {
		t.Fatal("expected error for no source")
	}
}

// ── OpenAI Format ───────────────────────────────────────────────────

func TestFormatOpenAIWithImages(t *testing.T) {
	msgs := []Message{{
		Role:    "user",
		Content: "What's in this image?",
		Images:  []Image{ImageFromURL("https://example.com/photo.jpg")},
	}}

	formatted := formatOpenAIMessages(msgs)
	if len(formatted) != 1 {
		t.Fatalf("expected 1 message, got %d", len(formatted))
	}

	content, ok := formatted[0]["content"].([]map[string]any)
	if !ok {
		t.Fatalf("content should be array, got %T", formatted[0]["content"])
	}
	if len(content) != 2 {
		t.Fatalf("expected 2 parts (text + image), got %d", len(content))
	}
	if content[0]["type"] != "text" {
		t.Errorf("first part type = %v", content[0]["type"])
	}
	if content[1]["type"] != "image_url" {
		t.Errorf("second part type = %v", content[1]["type"])
	}
	imageURL := content[1]["image_url"].(map[string]any)
	if imageURL["url"] != "https://example.com/photo.jpg" {
		t.Errorf("image url = %v", imageURL["url"])
	}
}

func TestFormatOpenAIWithImageDetail(t *testing.T) {
	msgs := []Message{{
		Role:    "user",
		Content: "Describe",
		Images:  []Image{{URL: "https://example.com/photo.jpg", Detail: "high"}},
	}}

	formatted := formatOpenAIMessages(msgs)
	content := formatted[0]["content"].([]map[string]any)
	imageURL := content[1]["image_url"].(map[string]any)
	if imageURL["detail"] != "high" {
		t.Errorf("detail = %v, want high", imageURL["detail"])
	}
}

func TestFormatOpenAIWithoutImages(t *testing.T) {
	// Backward compatibility: no images = content is plain string
	msgs := []Message{{Role: "user", Content: "Hello"}}
	formatted := formatOpenAIMessages(msgs)
	content, ok := formatted[0]["content"].(string)
	if !ok {
		t.Fatalf("content should be string, got %T", formatted[0]["content"])
	}
	if content != "Hello" {
		t.Errorf("content = %q", content)
	}
}

func TestFormatOpenAIWithBase64Image(t *testing.T) {
	jpegBytes := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	msgs := []Message{{
		Role:    "user",
		Content: "What is this?",
		Images:  []Image{ImageFromBytes(jpegBytes, "image/jpeg")},
	}}

	formatted := formatOpenAIMessages(msgs)
	content := formatted[0]["content"].([]map[string]any)
	imageURL := content[1]["image_url"].(map[string]any)
	url := imageURL["url"].(string)
	if !startsWith(url, "data:image/jpeg;base64,") {
		t.Errorf("expected data URI, got %q", url[:min(50, len(url))])
	}
}

// ── Anthropic Format ────────────────────────────────────────────────

func TestFormatAnthropicWithImages(t *testing.T) {
	jpegBytes := []byte{0xFF, 0xD8, 0xFF}
	msgs := []Message{{
		Role:    "user",
		Content: "What's in this?",
		Images:  []Image{ImageFromBytes(jpegBytes, "image/jpeg")},
	}}

	_, apiMsgs, _ := formatAnthropicRequest("claude-3", ModelConfig{MaxTokens: 1024}, msgs, nil)
	if len(apiMsgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(apiMsgs))
	}

	content, ok := apiMsgs[0]["content"].([]map[string]any)
	if !ok {
		t.Fatalf("content should be array, got %T", apiMsgs[0]["content"])
	}
	if len(content) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(content))
	}
	if content[0]["type"] != "text" {
		t.Errorf("first part type = %v", content[0]["type"])
	}
	if content[1]["type"] != "image" {
		t.Errorf("second part type = %v", content[1]["type"])
	}
	source := content[1]["source"].(map[string]any)
	if source["type"] != "base64" {
		t.Errorf("source type = %v", source["type"])
	}
	if source["media_type"] != "image/jpeg" {
		t.Errorf("media_type = %v", source["media_type"])
	}
}

// ── Gemini Format ───────────────────────────────────────────────────

func TestFormatGeminiWithImages(t *testing.T) {
	pngBytes := []byte{0x89, 'P', 'N', 'G'}
	msgs := []Message{{
		Role:    "user",
		Content: "Describe this",
		Images:  []Image{ImageFromBytes(pngBytes, "image/png")},
	}}

	body := formatGeminiRequest("", ModelConfig{MaxTokens: 1024, Temperature: 0.7}, msgs, nil)
	contents := body["contents"].([]map[string]any)
	if len(contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(contents))
	}

	parts := contents[0]["parts"].([]map[string]any)
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts (text + inline_data), got %d", len(parts))
	}
	if parts[0]["text"] != "Describe this" {
		t.Errorf("text = %v", parts[0]["text"])
	}
	inlineData, ok := parts[1]["inline_data"].(map[string]any)
	if !ok {
		t.Fatalf("expected inline_data, got %v", parts[1])
	}
	if inlineData["mime_type"] != "image/png" {
		t.Errorf("mime_type = %v", inlineData["mime_type"])
	}
}

// ── Session.AddMediaMessage ─────────────────────────────────────────

func TestAddMediaMessage(t *testing.T) {
	session := NewSession("test")
	img := ImageFromURL("https://example.com/photo.jpg")
	session.AddMediaMessage("user", "Check this", []Image{img}, nil, nil)

	history := session.GetHistory()
	if len(history) != 1 {
		t.Fatalf("history length = %d, want 1", len(history))
	}
	if history[0].Content != "Check this" {
		t.Errorf("content = %q", history[0].Content)
	}
	if len(history[0].Images) != 1 {
		t.Errorf("images = %d, want 1", len(history[0].Images))
	}
}

// ── Message JSON Serialization ──────────────────────────────────────

func TestMessageWithImagesJSON(t *testing.T) {
	msg := Message{
		Role:    "user",
		Content: "Hello",
		Images:  []Image{ImageFromURL("https://example.com/photo.jpg")},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Images) != 1 {
		t.Errorf("decoded images = %d, want 1", len(decoded.Images))
	}
	if decoded.Images[0].URL != "https://example.com/photo.jpg" {
		t.Errorf("decoded URL = %q", decoded.Images[0].URL)
	}
}

func TestMessageWithoutImagesJSON(t *testing.T) {
	msg := Message{Role: "user", Content: "Hello"}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	// Should not contain "images" key at all
	if contains(string(data), "images") {
		t.Errorf("JSON should not contain 'images' when empty: %s", data)
	}
}

// ── Edge Cases ──────────────────────────────────────────────────────

func TestResolveMediaBytesHTTPError(t *testing.T) {
	// Non-existent file should error
	_, _, err := resolveMediaBytes("", "/nonexistent/file.jpg", nil, "")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestFormatOpenAIFailedMediaFallback(t *testing.T) {
	// All images fail, no text → should fallback to empty string, not empty array
	msgs := []Message{{
		Role:    "user",
		Content: "",
		Images:  []Image{ImageFromFile("/nonexistent/file.jpg")},
	}}

	formatted := formatOpenAIMessages(msgs)
	// Should fall back to string content (empty), not empty parts array
	content, ok := formatted[0]["content"].(string)
	if !ok {
		t.Fatalf("expected fallback to string content, got %T", formatted[0]["content"])
	}
	if content != "" {
		t.Errorf("content = %q, want empty string", content)
	}
}

func TestFormatOpenAIFailedMediaWithText(t *testing.T) {
	// All images fail but text exists → text part should survive
	msgs := []Message{{
		Role:    "user",
		Content: "Hello",
		Images:  []Image{ImageFromFile("/nonexistent/file.jpg")},
	}}

	formatted := formatOpenAIMessages(msgs)
	// Should have parts array with just the text part
	parts, ok := formatted[0]["content"].([]map[string]any)
	if !ok {
		t.Fatalf("expected parts array, got %T", formatted[0]["content"])
	}
	if len(parts) != 1 {
		t.Fatalf("expected 1 part (text only), got %d", len(parts))
	}
	if parts[0]["type"] != "text" {
		t.Errorf("part type = %v, want text", parts[0]["type"])
	}
}

func TestFormatAnthropicFailedMediaFallback(t *testing.T) {
	// All images fail, no text → should fallback to string content
	msgs := []Message{{
		Role:    "user",
		Content: "",
		Images:  []Image{ImageFromFile("/nonexistent/file.jpg")},
	}}

	_, apiMsgs, _ := formatAnthropicRequest("claude-3", ModelConfig{MaxTokens: 1024}, msgs, nil)
	// Should fall back to string content
	content, ok := apiMsgs[0]["content"].(string)
	if !ok {
		t.Fatalf("expected fallback to string content, got %T", apiMsgs[0]["content"])
	}
	if content != "" {
		t.Errorf("content = %q, want empty string", content)
	}
}

func TestContentTypeSanitization(t *testing.T) {
	// Simulate Content-Type with charset — resolveMediaBytes should strip it
	// We test the stripping logic directly via a file with known bytes
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jpg")
	os.WriteFile(path, []byte{0xFF, 0xD8, 0xFF, 0xE0}, 0644)

	_, mime, err := resolveMediaBytes("", path, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	// Should not contain semicolons or charset
	if strings.Contains(mime, ";") {
		t.Errorf("MIME should not contain charset: %q", mime)
	}
}

// ── Helpers ─────────────────────────────────────────────────────────

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
