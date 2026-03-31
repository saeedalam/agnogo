package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/saeedalam/agnogo"
)

// WebBrowserConfig configures the web browser tool.
type WebBrowserConfig struct {
	// MaxSize is the maximum response body size in bytes. Default: 65536.
	MaxSize int64
	// MaxTextLen is the maximum text length after HTML stripping. Default: 8000.
	MaxTextLen int
	// Timeout in seconds. Default: 15.
	Timeout int
}

func (c *WebBrowserConfig) defaults() {
	if c.MaxSize <= 0 {
		c.MaxSize = 65536
	}
	if c.MaxTextLen <= 0 {
		c.MaxTextLen = 8000
	}
	if c.Timeout <= 0 {
		c.Timeout = 15
	}
}

// WebBrowser returns tools for fetching web pages and extracting links.
func WebBrowser(cfgs ...WebBrowserConfig) []agnogo.ToolDef {
	var cfg WebBrowserConfig
	if len(cfgs) > 0 {
		cfg = cfgs[0]
	}
	cfg.defaults()

	fetchPage := func(ctx context.Context, u string) ([]byte, error) {
		if !strings.HasPrefix(u, "http") {
			u = "https://" + u
		}
		client := &http.Client{Timeout: time.Duration(cfg.Timeout) * time.Second}
		req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
		if err != nil {
			return nil, fmt.Errorf("invalid URL: %w", err)
		}
		req.Header.Set("User-Agent", "agnogo/1.0")
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetch failed: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, cfg.MaxSize))
		if err != nil {
			return nil, fmt.Errorf("read error: %w", err)
		}
		return body, nil
	}

	return []agnogo.ToolDef{
		{
			Name: "read_url",
			Desc: "Fetch a URL and return its text content (HTML tags stripped)",
			Params: agnogo.Params{
				"url": {Type: "string", Desc: "URL to fetch", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				if err := ctx.Err(); err != nil {
					return "", fmt.Errorf("context cancelled: %w", err)
				}
				u := strings.TrimSpace(args["url"])
				if u == "" {
					return "", fmt.Errorf("missing required parameter: url")
				}
				body, err := fetchPage(ctx, u)
				if err != nil {
					return "", err
				}
				text := stripHTMLStateMachine(string(body))
				if len(text) > cfg.MaxTextLen {
					text = text[:cfg.MaxTextLen] + "\n... (truncated)"
				}
				return text, nil
			},
		},
		{
			Name: "web_extract_links",
			Desc: "Extract all links (<a href>) from a web page",
			Params: agnogo.Params{
				"url": {Type: "string", Desc: "URL to fetch and extract links from", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				if err := ctx.Err(); err != nil {
					return "", fmt.Errorf("context cancelled: %w", err)
				}
				u := strings.TrimSpace(args["url"])
				if u == "" {
					return "", fmt.Errorf("missing required parameter: url")
				}
				body, err := fetchPage(ctx, u)
				if err != nil {
					return "", err
				}
				links := extractLinks(string(body))
				out, _ := json.Marshal(links)
				return string(out), nil
			},
		},
	}
}

// stripHTMLStateMachine removes HTML tags using a state machine approach.
// It properly removes <script>, <style>, and <noscript> content entirely.
func stripHTMLStateMachine(s string) string {
	type state int
	const (
		stText state = iota
		stTag
		stSkipContent // inside <script>, <style>, <noscript>
	)

	var (
		out        strings.Builder
		st         = stText
		tagBuf     strings.Builder
		skipTag    string
		lastWasWS  bool
	)

	out.Grow(len(s) / 2)

	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch st {
		case stText:
			if ch == '<' {
				st = stTag
				tagBuf.Reset()
			} else if ch == '&' {
				// Decode common HTML entities
				entity, advance := decodeEntity(s[i:])
				if advance > 0 {
					if entity == " " || entity == "\n" || entity == "\t" {
						if !lastWasWS {
							out.WriteByte(' ')
							lastWasWS = true
						}
					} else {
						out.WriteString(entity)
						lastWasWS = false
					}
					i += advance - 1
				} else {
					out.WriteByte(ch)
					lastWasWS = false
				}
			} else if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
				if !lastWasWS {
					out.WriteByte(' ')
					lastWasWS = true
				}
			} else {
				out.WriteByte(ch)
				lastWasWS = false
			}

		case stTag:
			if ch == '>' {
				tag := strings.ToLower(tagBuf.String())
				tagName := extractTagName(tag)
				if tagName == "script" || tagName == "style" || tagName == "noscript" {
					if len(tag) == 0 || tag[0] != '/' {
						// Opening tag: skip content until closing tag
						st = stSkipContent
						skipTag = tagName
						continue
					}
				}
				// Block-level tags get a space
				if isBlockTag(tagName) && !lastWasWS {
					out.WriteByte(' ')
					lastWasWS = true
				}
				st = stText
			} else {
				tagBuf.WriteByte(ch)
			}

		case stSkipContent:
			// Look for closing tag: </script>, </style>, </noscript>
			if ch == '<' && i+2 < len(s) && s[i+1] == '/' {
				end := strings.Index(s[i:], ">")
				if end > 0 {
					closingTag := strings.ToLower(strings.TrimSpace(s[i+2 : i+end]))
					if closingTag == skipTag {
						i += end
						st = stText
						// Insert a space separator after skipped content
						if !lastWasWS && out.Len() > 0 {
							out.WriteByte(' ')
							lastWasWS = true
						}
						continue
					}
				}
			}
		}
	}

	return strings.TrimSpace(out.String())
}

func extractTagName(tag string) string {
	tag = strings.TrimLeft(tag, "/")
	for i, ch := range tag {
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' || ch == '/' || ch == '>' {
			return tag[:i]
		}
	}
	return tag
}

func isBlockTag(tag string) bool {
	switch tag {
	case "div", "p", "br", "hr", "h1", "h2", "h3", "h4", "h5", "h6",
		"li", "ul", "ol", "table", "tr", "td", "th", "thead", "tbody",
		"section", "article", "header", "footer", "nav", "main",
		"blockquote", "pre", "form", "fieldset", "dl", "dt", "dd":
		return true
	}
	return false
}

func decodeEntity(s string) (string, int) {
	end := strings.IndexByte(s, ';')
	if end < 0 || end > 10 {
		return "", 0
	}
	entity := s[:end+1]
	switch entity {
	case "&amp;":
		return "&", end + 1
	case "&lt;":
		return "<", end + 1
	case "&gt;":
		return ">", end + 1
	case "&quot;":
		return "\"", end + 1
	case "&apos;":
		return "'", end + 1
	case "&nbsp;":
		return " ", end + 1
	}
	return "", 0
}

// extractLinks finds all <a href="..."> links in HTML.
func extractLinks(html string) []map[string]string {
	var links []map[string]string
	lower := strings.ToLower(html)
	pos := 0
	for {
		idx := strings.Index(lower[pos:], "<a ")
		if idx < 0 {
			idx = strings.Index(lower[pos:], "<a\t")
			if idx < 0 {
				idx = strings.Index(lower[pos:], "<a\n")
			}
		}
		if idx < 0 {
			break
		}
		pos += idx
		// Find the end of the tag
		tagEnd := strings.IndexByte(html[pos:], '>')
		if tagEnd < 0 {
			break
		}
		tag := html[pos : pos+tagEnd+1]
		pos += tagEnd + 1

		href := extractAttr(tag, "href")
		if href == "" {
			continue
		}

		// Extract text content until </a>
		closeIdx := strings.Index(lower[pos:], "</a>")
		text := ""
		if closeIdx >= 0 {
			text = stripHTMLStateMachine(html[pos : pos+closeIdx])
			pos += closeIdx + 4
		}

		links = append(links, map[string]string{
			"href": href,
			"text": strings.TrimSpace(text),
		})
	}
	return links
}

func extractAttr(tag, attr string) string {
	lower := strings.ToLower(tag)
	idx := strings.Index(lower, attr+"=")
	if idx < 0 {
		return ""
	}
	rest := tag[idx+len(attr)+1:]
	if len(rest) == 0 {
		return ""
	}
	if rest[0] == '"' {
		end := strings.IndexByte(rest[1:], '"')
		if end < 0 {
			return ""
		}
		return rest[1 : 1+end]
	}
	if rest[0] == '\'' {
		end := strings.IndexByte(rest[1:], '\'')
		if end < 0 {
			return ""
		}
		return rest[1 : 1+end]
	}
	// Unquoted
	end := strings.IndexAny(rest, " \t\n\r>")
	if end < 0 {
		return rest
	}
	return rest[:end]
}

// DuckDuckGoConfig configures the DuckDuckGo search tool.
type DuckDuckGoConfig struct {
	// MaxResults limits the number of related topics returned. Default: 5.
	MaxResults int
	// Timeout in seconds. Default: 10.
	Timeout int
}

func (c *DuckDuckGoConfig) defaults() {
	if c.MaxResults <= 0 {
		c.MaxResults = 5
	}
	if c.Timeout <= 0 {
		c.Timeout = 10
	}
}

// DuckDuckGo returns a web search tool using DuckDuckGo instant answers.
func DuckDuckGo(cfgs ...DuckDuckGoConfig) []agnogo.ToolDef {
	var cfg DuckDuckGoConfig
	if len(cfgs) > 0 {
		cfg = cfgs[0]
	}
	cfg.defaults()

	return []agnogo.ToolDef{{
		Name: "web_search",
		Desc: "Search the web using DuckDuckGo",
		Params: agnogo.Params{
			"query":       {Type: "string", Desc: "Search query", Required: true},
			"max_results": {Type: "string", Desc: fmt.Sprintf("Max related topics to return (default %d)", cfg.MaxResults)},
		},
		Fn: func(ctx context.Context, args map[string]string) (string, error) {
			if err := ctx.Err(); err != nil {
				return "", fmt.Errorf("context cancelled: %w", err)
			}
			query := strings.TrimSpace(args["query"])
			if query == "" {
				return "", fmt.Errorf("missing required parameter: query")
			}

			maxResults := cfg.MaxResults
			if mr := strings.TrimSpace(args["max_results"]); mr != "" {
				if n, err := strconv.Atoi(mr); err == nil && n > 0 {
					maxResults = n
				}
			}

			q := url.QueryEscape(query)
			u := fmt.Sprintf("https://api.duckduckgo.com/?q=%s&format=json&no_html=1&skip_disambig=1", q)

			client := &http.Client{Timeout: time.Duration(cfg.Timeout) * time.Second}
			req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
			if err != nil {
				return "", fmt.Errorf("invalid request: %w", err)
			}
			req.Header.Set("User-Agent", "agnogo/1.0")

			resp, err := client.Do(req)
			if err != nil {
				return "", fmt.Errorf("search request failed: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return "", fmt.Errorf("DuckDuckGo API returned HTTP %d", resp.StatusCode)
			}

			data, err := io.ReadAll(io.LimitReader(resp.Body, 65536))
			if err != nil {
				return "", fmt.Errorf("error reading response: %w", err)
			}

			var result struct {
				Abstract     string `json:"Abstract"`
				AbstractText string `json:"AbstractText"`
				Answer       string `json:"Answer"`
				RelatedTopics []struct {
					Text string `json:"Text"`
				} `json:"RelatedTopics"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				return "", fmt.Errorf("error parsing DuckDuckGo response: %w", err)
			}

			var sb strings.Builder
			if result.Answer != "" {
				sb.WriteString("Answer: " + result.Answer + "\n")
			}
			if result.AbstractText != "" {
				sb.WriteString(result.AbstractText + "\n")
			}
			for i, t := range result.RelatedTopics {
				if i >= maxResults {
					break
				}
				if t.Text != "" {
					sb.WriteString("- " + t.Text + "\n")
				}
			}
			if sb.Len() == 0 {
				return "No results found.", nil
			}
			return sb.String(), nil
		},
	}}
}

// WikipediaConfig configures the Wikipedia tool.
type WikipediaConfig struct {
	// Language code (e.g. "en", "de", "fr"). Default: "en".
	Language string
	// Timeout in seconds. Default: 10.
	Timeout int
}

func (c *WikipediaConfig) defaults() {
	if c.Language == "" {
		c.Language = "en"
	}
	if c.Timeout <= 0 {
		c.Timeout = 10
	}
}

// Wikipedia returns a tool for searching Wikipedia.
func Wikipedia(cfgs ...WikipediaConfig) []agnogo.ToolDef {
	var cfg WikipediaConfig
	if len(cfgs) > 0 {
		cfg = cfgs[0]
	}
	cfg.defaults()

	return []agnogo.ToolDef{{
		Name: "wikipedia",
		Desc: "Search Wikipedia for information",
		Params: agnogo.Params{
			"query":    {Type: "string", Desc: "Topic to search", Required: true},
			"language": {Type: "string", Desc: fmt.Sprintf("Wikipedia language code (default %q)", cfg.Language)},
		},
		Fn: func(ctx context.Context, args map[string]string) (string, error) {
			if err := ctx.Err(); err != nil {
				return "", fmt.Errorf("context cancelled: %w", err)
			}
			query := strings.TrimSpace(args["query"])
			if query == "" {
				return "", fmt.Errorf("missing required parameter: query")
			}

			lang := cfg.Language
			if l := strings.TrimSpace(args["language"]); l != "" {
				lang = l
			}

			q := url.QueryEscape(query)
			u := fmt.Sprintf("https://%s.wikipedia.org/api/rest_v1/page/summary/%s", lang, q)

			client := &http.Client{Timeout: time.Duration(cfg.Timeout) * time.Second}
			req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
			if err != nil {
				return "", fmt.Errorf("invalid request: %w", err)
			}
			req.Header.Set("User-Agent", "agnogo/1.0")

			resp, err := client.Do(req)
			if err != nil {
				return "", fmt.Errorf("Wikipedia request failed: %w", err)
			}
			defer resp.Body.Close()

			data, err := io.ReadAll(io.LimitReader(resp.Body, 65536))
			if err != nil {
				return "", fmt.Errorf("error reading response: %w", err)
			}

			if resp.StatusCode == http.StatusNotFound {
				return "No Wikipedia article found for that topic.", nil
			}
			if resp.StatusCode != http.StatusOK {
				return "", fmt.Errorf("Wikipedia API returned HTTP %d: %s", resp.StatusCode, string(data[:min(len(data), 200)]))
			}

			var result struct {
				Title   string `json:"title"`
				Extract string `json:"extract"`
				Type    string `json:"type"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				return "", fmt.Errorf("error parsing Wikipedia response: %w", err)
			}
			if result.Extract == "" {
				return "No Wikipedia article found.", nil
			}
			return fmt.Sprintf("# %s\n%s", result.Title, result.Extract), nil
		},
	}}
}
