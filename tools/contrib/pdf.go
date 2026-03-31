package contrib

import (
	"bytes"
	"compress/flate"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/saeedalam/agnogo"
)

// PDFTool returns tools for PDF metadata extraction and basic text extraction.
//
// Limitations of pdf_extract_text (stdlib-only, no external deps):
//   - Works with common text PDFs using standard encodings (WinAnsiEncoding,
//     StandardEncoding, MacRomanEncoding).
//   - Handles FlateDecode compressed streams (the most common compression).
//   - Does NOT handle: scanned/image-only PDFs, CIDFont/Type0 fonts with
//     custom ToUnicode maps, encrypted PDFs, JBIG2/JPX compression.
//   - Coverage: approximately 60-70% of typical text-based PDFs.
func PDFTool() []agnogo.ToolDef {
	return []agnogo.ToolDef{
		{
			Name: "pdf_info",
			Desc: "Extract PDF metadata: page count, title, author, creator, producer, version, and file size.",
			Params: agnogo.Params{
				"path": {Type: "string", Desc: "Path to PDF file", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				path := args["path"]
				if path == "" {
					return "", fmt.Errorf("path is required")
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return "", fmt.Errorf("failed to read file: %w", err)
				}
				if len(data) < 5 || string(data[:5]) != "%PDF-" {
					return "", fmt.Errorf("not a valid PDF file")
				}
				info := extractPDFInfo(data)
				out, _ := json.MarshalIndent(info, "", "  ")
				return string(out), nil
			},
		},
		{
			Name: "pdf_extract_text",
			Desc: "Extract text from a PDF file. Returns text per page. Works with common text PDFs; does not handle scanned/image PDFs or encrypted files.",
			Params: agnogo.Params{
				"path":  {Type: "string", Desc: "Path to PDF file", Required: true},
				"pages": {Type: "string", Desc: "Page range to extract (e.g. '1-5', '3', 'all'). Default: all"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				path := args["path"]
				if path == "" {
					return "", fmt.Errorf("path is required")
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return "", fmt.Errorf("failed to read file: %w", err)
				}
				if len(data) < 5 || string(data[:5]) != "%PDF-" {
					return "", fmt.Errorf("not a valid PDF file")
				}

				pages, err := extractPDFText(data)
				if err != nil {
					return "", fmt.Errorf("text extraction failed: %w", err)
				}

				// Filter pages if requested
				pageRange := strings.TrimSpace(args["pages"])
				if pageRange != "" && pageRange != "all" {
					filtered, err := filterPages(pages, pageRange)
					if err != nil {
						return "", err
					}
					pages = filtered
				}

				result := map[string]any{
					"page_count": len(pages),
					"pages":      pages,
				}
				out, _ := json.MarshalIndent(result, "", "  ")
				return string(out), nil
			},
		},
	}
}

// ---------------------------------------------------------------------------
// PDF info extraction
// ---------------------------------------------------------------------------

func extractPDFInfo(data []byte) map[string]any {
	info := map[string]any{
		"file_size": len(data),
	}

	content := string(data)

	// Version
	if len(data) >= 10 {
		end := bytes.IndexByte(data[:20], '\n')
		if end < 0 {
			end = 10
		}
		info["pdf_version"] = strings.TrimSpace(string(data[5:end]))
	}

	// Page count from /Pages /Count
	countRe := regexp.MustCompile(`/Type\s*/Pages[^>]*?/Count\s+(\d+)`)
	if m := countRe.FindStringSubmatch(content); len(m) > 1 {
		if c, err := strconv.Atoi(m[1]); err == nil {
			info["page_count"] = c
		}
	} else {
		// Fallback: count /Type /Page occurrences
		pageRe := regexp.MustCompile(`/Type\s*/Page\b[^s]`)
		matches := pageRe.FindAllStringIndex(content, -1)
		info["page_count"] = len(matches)
	}

	// Info dictionary fields
	for _, field := range []struct{ key, pdfKey string }{
		{"title", "Title"},
		{"author", "Author"},
		{"creator", "Creator"},
		{"producer", "Producer"},
		{"subject", "Subject"},
		{"creation_date", "CreationDate"},
		{"mod_date", "ModDate"},
	} {
		re := regexp.MustCompile(`/` + field.pdfKey + `\s*\(([^)]*)\)`)
		if m := re.FindStringSubmatch(content); len(m) > 1 {
			info[field.key] = m[1]
		}
	}

	return info
}

// ---------------------------------------------------------------------------
// PDF text extraction
// ---------------------------------------------------------------------------

func extractPDFText(data []byte) ([]map[string]any, error) {
	content := string(data)

	// Find all content streams from page objects.
	// Strategy: find /Type /Page objects, extract their /Contents references,
	// then find and decompress those streams.
	streams := findContentStreams(data, content)
	if len(streams) == 0 {
		// Fallback: try to find all streams and extract text
		streams = findAllStreams(data)
	}

	var pages []map[string]any
	for i, stream := range streams {
		text := extractTextFromStream(stream)
		text = strings.TrimSpace(text)
		pages = append(pages, map[string]any{
			"page":   i + 1,
			"text":   text,
			"length": len(text),
		})
	}

	if len(pages) == 0 {
		return []map[string]any{{
			"page":   1,
			"text":   "",
			"length": 0,
			"note":   "No extractable text found. The PDF may be image-based, encrypted, or use unsupported font encodings.",
		}}, nil
	}

	return pages, nil
}

// findContentStreams locates page content streams by parsing /Type /Page objects.
func findContentStreams(data []byte, content string) [][]byte {
	// Find page objects: /Type /Page (not /Pages)
	pageRe := regexp.MustCompile(`(?s)\d+\s+\d+\s+obj\s*(<<[^>]*?/Type\s*/Page\b[^s][^>]*?>>)`)
	pageMatches := pageRe.FindAllStringSubmatchIndex(content, -1)

	var streams [][]byte
	for _, match := range pageMatches {
		if len(match) < 4 {
			continue
		}
		pageDict := content[match[2]:match[3]]

		// Find /Contents reference (could be indirect ref like "5 0 R" or array)
		contentsRe := regexp.MustCompile(`/Contents\s+(\d+)\s+\d+\s+R`)
		if m := contentsRe.FindStringSubmatch(pageDict); len(m) > 1 {
			objNum, _ := strconv.Atoi(m[1])
			stream := findAndDecompressObject(data, content, objNum)
			if stream != nil {
				streams = append(streams, stream)
			}
		} else {
			// Try array of references: /Contents [5 0 R 6 0 R]
			arrRe := regexp.MustCompile(`/Contents\s*\[([^\]]+)\]`)
			if m := arrRe.FindStringSubmatch(pageDict); len(m) > 1 {
				refRe := regexp.MustCompile(`(\d+)\s+\d+\s+R`)
				refs := refRe.FindAllStringSubmatch(m[1], -1)
				var combined []byte
				for _, ref := range refs {
					objNum, _ := strconv.Atoi(ref[1])
					stream := findAndDecompressObject(data, content, objNum)
					if stream != nil {
						combined = append(combined, stream...)
						combined = append(combined, '\n')
					}
				}
				if len(combined) > 0 {
					streams = append(streams, combined)
				}
			}
		}
	}
	return streams
}

// findAndDecompressObject finds a PDF object by number and decompresses its stream.
func findAndDecompressObject(data []byte, content string, objNum int) []byte {
	// Find the object: "objNum gen obj ... stream ... endstream"
	pattern := fmt.Sprintf(`%d\s+\d+\s+obj`, objNum)
	re := regexp.MustCompile(pattern)
	loc := re.FindStringIndex(content)
	if loc == nil {
		return nil
	}

	// Find stream/endstream within reasonable range
	objStart := loc[0]
	searchEnd := objStart + 1024*1024 // Search up to 1MB ahead
	if searchEnd > len(content) {
		searchEnd = len(content)
	}
	sub := content[objStart:searchEnd]

	streamIdx := strings.Index(sub, "stream")
	if streamIdx < 0 {
		return nil
	}
	endstreamIdx := strings.Index(sub[streamIdx:], "endstream")
	if endstreamIdx < 0 {
		return nil
	}

	// Stream data starts after "stream\r\n" or "stream\n"
	dataStart := objStart + streamIdx + 6
	if dataStart < len(data) && data[dataStart] == '\r' {
		dataStart++
	}
	if dataStart < len(data) && data[dataStart] == '\n' {
		dataStart++
	}
	dataEnd := objStart + streamIdx + endstreamIdx
	// Trim trailing whitespace before endstream
	for dataEnd > dataStart && (data[dataEnd-1] == '\n' || data[dataEnd-1] == '\r') {
		dataEnd--
	}

	if dataStart >= dataEnd || dataStart >= len(data) || dataEnd > len(data) {
		return nil
	}

	streamData := data[dataStart:dataEnd]

	// Check if FlateDecode
	dictEnd := streamIdx
	dict := sub[:dictEnd]
	if strings.Contains(dict, "/FlateDecode") {
		decoded, err := flateDecompress(streamData)
		if err != nil {
			return nil
		}
		return decoded
	}

	// Uncompressed stream
	return streamData
}

// findAllStreams is a fallback that finds all stream objects in the PDF.
func findAllStreams(data []byte) [][]byte {
	content := string(data)
	var streams [][]byte

	// Find all stream/endstream pairs that look like content streams
	re := regexp.MustCompile(`(?s)(\d+\s+\d+\s+obj\s*<<[^>]*?>>)\s*stream`)
	matches := re.FindAllStringSubmatchIndex(content, 100) // Limit to 100

	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		dict := content[match[2]:match[3]]
		// Only process streams that look like page content (have text operators)
		streamStart := match[1]
		if streamStart < len(data) && data[streamStart] == '\r' {
			streamStart++
		}
		if streamStart < len(data) && data[streamStart] == '\n' {
			streamStart++
		}

		endIdx := strings.Index(content[streamStart:], "endstream")
		if endIdx < 0 {
			continue
		}
		streamEnd := streamStart + endIdx
		for streamEnd > streamStart && (data[streamEnd-1] == '\n' || data[streamEnd-1] == '\r') {
			streamEnd--
		}

		if streamStart >= streamEnd || streamEnd > len(data) {
			continue
		}

		streamData := data[streamStart:streamEnd]
		if strings.Contains(dict, "/FlateDecode") {
			decoded, err := flateDecompress(streamData)
			if err != nil {
				continue
			}
			streamData = decoded
		}

		// Only include if it contains text operators
		s := string(streamData)
		if strings.Contains(s, "Tj") || strings.Contains(s, "TJ") ||
			strings.Contains(s, "'") || strings.Contains(s, `"`) {
			streams = append(streams, streamData)
		}
	}
	return streams
}

func flateDecompress(data []byte) ([]byte, error) {
	r := flate.NewReader(bytes.NewReader(data))
	defer r.Close()
	out, err := io.ReadAll(io.LimitReader(r, 10*1024*1024)) // 10MB limit
	if err != nil {
		return nil, err
	}
	return out, nil
}

// extractTextFromStream parses PDF content stream operators to extract text.
func extractTextFromStream(stream []byte) string {
	s := string(stream)
	var result strings.Builder

	i := 0
	inText := false // Between BT and ET
	for i < len(s) {
		// Skip whitespace
		for i < len(s) && (s[i] == ' ' || s[i] == '\n' || s[i] == '\r' || s[i] == '\t') {
			i++
		}
		if i >= len(s) {
			break
		}

		// BT - begin text
		if i+2 <= len(s) && s[i:i+2] == "BT" && (i+2 >= len(s) || !isAlpha(s[i+2])) {
			inText = true
			i += 2
			continue
		}

		// ET - end text
		if i+2 <= len(s) && s[i:i+2] == "ET" && (i+2 >= len(s) || !isAlpha(s[i+2])) {
			inText = false
			result.WriteByte(' ')
			i += 2
			continue
		}

		if !inText {
			i++
			continue
		}

		// Tj operator: (string) Tj
		if s[i] == '(' {
			str, end := parsePDFString(s, i)
			i = end
			// Skip whitespace then check for Tj or ' or "
			for i < len(s) && (s[i] == ' ' || s[i] == '\n' || s[i] == '\r') {
				i++
			}
			if i < len(s) && (s[i] == 'T' || s[i] == '\'' || s[i] == '"') {
				result.WriteString(decodePDFText(str))
				// Skip operator
				for i < len(s) && s[i] != ' ' && s[i] != '\n' && s[i] != '\r' {
					i++
				}
			}
			continue
		}

		// TJ operator: [(string) num (string) ...] TJ
		if s[i] == '[' {
			i++
			var parts []string
			for i < len(s) && s[i] != ']' {
				if s[i] == '(' {
					str, end := parsePDFString(s, i)
					parts = append(parts, decodePDFText(str))
					i = end
				} else if s[i] == '-' || s[i] == '.' || (s[i] >= '0' && s[i] <= '9') {
					// Numeric spacing: large negative values indicate word space
					numStart := i
					if s[i] == '-' {
						i++
					}
					for i < len(s) && ((s[i] >= '0' && s[i] <= '9') || s[i] == '.') {
						i++
					}
					numStr := s[numStart:i]
					if n, err := strconv.ParseFloat(numStr, 64); err == nil {
						// Values less than -100 typically indicate word spacing
						if n < -100 {
							parts = append(parts, " ")
						}
					}
				} else {
					i++
				}
			}
			if i < len(s) {
				i++ // skip ']'
			}
			// Skip whitespace and TJ
			for i < len(s) && (s[i] == ' ' || s[i] == '\n' || s[i] == '\r') {
				i++
			}
			if i+1 < len(s) && s[i:i+2] == "TJ" {
				i += 2
			}
			result.WriteString(strings.Join(parts, ""))
			continue
		}

		// Td/TD operators with large Y offset may indicate new line
		// T* operator: new line
		if i+2 <= len(s) && s[i:i+2] == "T*" {
			result.WriteByte('\n')
			i += 2
			continue
		}

		i++
	}

	return cleanExtractedText(result.String())
}

func isAlpha(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// parsePDFString parses a parenthesized PDF string, handling escapes and nesting.
func parsePDFString(s string, start int) (string, int) {
	if start >= len(s) || s[start] != '(' {
		return "", start
	}
	var buf strings.Builder
	depth := 1
	i := start + 1
	for i < len(s) && depth > 0 {
		ch := s[i]
		if ch == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case 'n':
				buf.WriteByte('\n')
			case 'r':
				buf.WriteByte('\r')
			case 't':
				buf.WriteByte('\t')
			case 'b':
				buf.WriteByte('\b')
			case 'f':
				buf.WriteByte('\f')
			case '(':
				buf.WriteByte('(')
			case ')':
				buf.WriteByte(')')
			case '\\':
				buf.WriteByte('\\')
			default:
				// Octal escape
				if s[i] >= '0' && s[i] <= '7' {
					oct := string(s[i])
					for j := 0; j < 2 && i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '7'; j++ {
						i++
						oct += string(s[i])
					}
					if v, err := strconv.ParseUint(oct, 8, 8); err == nil {
						buf.WriteByte(byte(v))
					}
				} else {
					buf.WriteByte(s[i])
				}
			}
		} else if ch == '(' {
			depth++
			buf.WriteByte(ch)
		} else if ch == ')' {
			depth--
			if depth > 0 {
				buf.WriteByte(ch)
			}
		} else {
			buf.WriteByte(ch)
		}
		i++
	}
	return buf.String(), i
}

// decodePDFText applies basic PDF text decoding (WinAnsiEncoding compatible).
func decodePDFText(s string) string {
	// For WinAnsiEncoding (most common), bytes 0x20-0x7E are ASCII.
	// Higher bytes map to Windows-1252.
	var buf strings.Builder
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b >= 0x20 && b <= 0x7E {
			buf.WriteByte(b)
		} else {
			// Common Windows-1252 mappings
			switch b {
			case 0x91:
				buf.WriteRune('\u2018') // left single quote
			case 0x92:
				buf.WriteRune('\u2019') // right single quote
			case 0x93:
				buf.WriteRune('\u201C') // left double quote
			case 0x94:
				buf.WriteRune('\u201D') // right double quote
			case 0x95:
				buf.WriteRune('\u2022') // bullet
			case 0x96:
				buf.WriteRune('\u2013') // en dash
			case 0x97:
				buf.WriteRune('\u2014') // em dash
			case 0x85:
				buf.WriteRune('\u2026') // ellipsis
			case 0xA0:
				buf.WriteByte(' ') // non-breaking space
			case 0x0A, 0x0D:
				buf.WriteByte('\n')
			case 0x09:
				buf.WriteByte('\t')
			default:
				if b > 0x7E {
					// Best effort: treat as Latin-1
					buf.WriteRune(rune(b))
				}
				// Skip control characters
			}
		}
	}
	return buf.String()
}

// cleanExtractedText normalizes whitespace in extracted PDF text.
func cleanExtractedText(s string) string {
	// Collapse multiple spaces, preserve intentional newlines
	lines := strings.Split(s, "\n")
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Collapse multiple spaces within a line
		for strings.Contains(line, "  ") {
			line = strings.ReplaceAll(line, "  ", " ")
		}
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	return strings.Join(cleaned, "\n")
}

// filterPages filters page results by a range string like "1-5" or "3".
func filterPages(pages []map[string]any, rangeStr string) ([]map[string]any, error) {
	parts := strings.SplitN(rangeStr, "-", 2)
	start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return nil, fmt.Errorf("invalid page range: %s", rangeStr)
	}
	end := start
	if len(parts) == 2 {
		end, err = strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return nil, fmt.Errorf("invalid page range: %s", rangeStr)
		}
	}
	if start < 1 {
		start = 1
	}
	if end > len(pages) {
		end = len(pages)
	}
	if start > len(pages) {
		return []map[string]any{}, nil
	}
	return pages[start-1 : end], nil
}
