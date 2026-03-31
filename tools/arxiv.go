package tools

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/saeedalam/agnogo"
)

// Arxiv returns tools for searching the arXiv preprint repository.
// Clone of agno's ArxivTools. Returns: search_arxiv.
// No authentication required — arXiv uses an Atom/XML feed.
func Arxiv() []agnogo.ToolDef {
	client := &http.Client{Timeout: 15 * time.Second}

	// Atom XML structures for parsing arXiv API responses.
	type atomLink struct {
		Href string `xml:"href,attr"`
		Rel  string `xml:"rel,attr"`
		Type string `xml:"type,attr"`
	}
	type atomAuthor struct {
		Name string `xml:"name"`
	}
	type atomCategory struct {
		Term string `xml:"term,attr"`
	}
	type atomEntry struct {
		Title           string        `xml:"title"`
		ID              string        `xml:"id"`
		Summary         string        `xml:"summary"`
		Published       string        `xml:"published"`
		Updated         string        `xml:"updated"`
		Authors         []atomAuthor  `xml:"author"`
		Links           []atomLink    `xml:"link"`
		PrimaryCategory atomCategory  `xml:"primary_category"`
	}
	type atomFeed struct {
		Entries []atomEntry `xml:"entry"`
	}

	return []agnogo.ToolDef{
		{
			Name: "search_arxiv",
			Desc: "Search arXiv for academic papers and preprints. Returns titles, authors, summaries, and PDF links.",
			Params: agnogo.Params{
				"query":       {Type: "string", Desc: "Search query for arXiv papers", Required: true},
				"max_results": {Type: "string", Desc: "Maximum number of results to return (default 10)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				query := strings.TrimSpace(args["query"])
				if query == "" {
					return "", fmt.Errorf("missing required parameter: query")
				}
				maxResults := 10
				if mr := strings.TrimSpace(args["max_results"]); mr != "" {
					if n, err := strconv.Atoi(mr); err == nil && n > 0 {
						maxResults = n
					}
				}

				u := fmt.Sprintf("http://export.arxiv.org/api/query?search_query=all:%s&max_results=%d&sortBy=relevance&sortOrder=descending",
					url.QueryEscape(query), maxResults)

				req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
				if err != nil {
					return "", fmt.Errorf("failed to create request: %w", err)
				}
				req.Header.Set("User-Agent", "agnogo/1.0")

				resp, err := client.Do(req)
				if err != nil {
					return "", fmt.Errorf("arXiv request failed: %w", err)
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					return "", fmt.Errorf("arXiv API returned HTTP %d", resp.StatusCode)
				}

				data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
				if err != nil {
					return "", fmt.Errorf("read error: %w", err)
				}

				var feed atomFeed
				if err := xml.Unmarshal(data, &feed); err != nil {
					return "", fmt.Errorf("XML parse error: %w", err)
				}

				var results []map[string]interface{}
				for _, entry := range feed.Entries {
					// Extract arxiv ID from the full URL (e.g., http://arxiv.org/abs/1234.5678v1)
					arxivID := entry.ID
					if idx := strings.LastIndex(arxivID, "/abs/"); idx >= 0 {
						arxivID = arxivID[idx+5:]
					}

					// Collect author names
					authors := make([]string, 0, len(entry.Authors))
					for _, a := range entry.Authors {
						authors = append(authors, strings.TrimSpace(a.Name))
					}

					// Find PDF URL from links
					pdfURL := ""
					for _, link := range entry.Links {
						if link.Type == "application/pdf" || link.Rel == "related" && strings.Contains(link.Href, "pdf") {
							pdfURL = link.Href
							break
						}
					}
					if pdfURL == "" {
						// Construct from ID as fallback
						pdfURL = fmt.Sprintf("http://arxiv.org/pdf/%s", arxivID)
					}

					results = append(results, map[string]interface{}{
						"title":            strings.TrimSpace(entry.Title),
						"arxiv_id":         arxivID,
						"authors":          authors,
						"summary":          strings.TrimSpace(entry.Summary),
						"published":        entry.Published,
						"updated":          entry.Updated,
						"primary_category": entry.PrimaryCategory.Term,
						"pdf_url":          pdfURL,
					})
				}

				if len(results) == 0 {
					return `{"results":[],"message":"No papers found"}`, nil
				}

				out, _ := json.Marshal(map[string]interface{}{
					"results": results,
					"query":   query,
				})
				return string(out), nil
			},
		},
	}
}
