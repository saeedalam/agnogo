package contrib

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/saeedalam/agnogo"
)

// Notion returns tools for interacting with the Notion API.
// Clone of agno's NotionTools.
func Notion(apiKey, databaseID string) []agnogo.ToolDef {
	if apiKey == "" {
		apiKey = os.Getenv("NOTION_API_KEY")
	}
	if databaseID == "" {
		databaseID = os.Getenv("NOTION_DATABASE_ID")
	}

	client := &http.Client{Timeout: 15 * time.Second}

	notionAPI := func(ctx context.Context, method, urlPath string, body map[string]any) (string, error) {
		if err := ctx.Err(); err != nil {
			return "", fmt.Errorf("context cancelled: %w", err)
		}
		var bodyReader io.Reader
		if body != nil {
			data, _ := json.Marshal(body)
			bodyReader = bytes.NewReader(data)
		}
		req, err := http.NewRequestWithContext(ctx, method, urlPath, bodyReader)
		if err != nil {
			return "", fmt.Errorf("invalid request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Notion-Version", "2022-06-28")
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("Notion API request failed: %w", err)
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 65536))

		if resp.StatusCode >= 400 {
			return "", fmt.Errorf("Notion API error %d: %s", resp.StatusCode, string(data))
		}
		return string(data), nil
	}

	return []agnogo.ToolDef{
		{
			Name: "create_page",
			Desc: "Create a new page in the Notion database",
			Params: agnogo.Params{
				"title":   {Type: "string", Desc: "Page title", Required: true},
				"tag":     {Type: "string", Desc: "Tag for the page"},
				"content": {Type: "string", Desc: "Page content"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				title := strings.TrimSpace(args["title"])
				if title == "" {
					return "", fmt.Errorf("missing required parameter: title")
				}
				body := map[string]any{
					"parent": map[string]any{
						"database_id": databaseID,
					},
					"properties": map[string]any{
						"Name": map[string]any{
							"title": []map[string]any{
								{"text": map[string]any{"content": title}},
							},
						},
					},
				}
				if tag := strings.TrimSpace(args["tag"]); tag != "" {
					props := body["properties"].(map[string]any)
					props["Tag"] = map[string]any{
						"select": map[string]any{"name": tag},
					}
				}
				if content := args["content"]; content != "" {
					body["children"] = []map[string]any{
						{
							"object": "block",
							"type":   "paragraph",
							"paragraph": map[string]any{
								"rich_text": []map[string]any{
									{"type": "text", "text": map[string]any{"content": content}},
								},
							},
						},
					}
				}
				return notionAPI(ctx, "POST", "https://api.notion.com/v1/pages", body)
			},
		},
		{
			Name: "update_page",
			Desc: "Append content to an existing Notion page",
			Params: agnogo.Params{
				"page_id": {Type: "string", Desc: "Notion page ID", Required: true},
				"content": {Type: "string", Desc: "Content to append", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				pageID := strings.TrimSpace(args["page_id"])
				if pageID == "" {
					return "", fmt.Errorf("missing required parameter: page_id")
				}
				content := args["content"]
				if content == "" {
					return "", fmt.Errorf("missing required parameter: content")
				}
				apiURL := fmt.Sprintf("https://api.notion.com/v1/blocks/%s/children", pageID)
				return notionAPI(ctx, "PATCH", apiURL, map[string]any{
					"children": []map[string]any{
						{
							"object": "block",
							"type":   "paragraph",
							"paragraph": map[string]any{
								"rich_text": []map[string]any{
									{"type": "text", "text": map[string]any{"content": content}},
								},
							},
						},
					},
				})
			},
		},
		{
			Name: "search_pages",
			Desc: "Search pages in the Notion database by tag",
			Params: agnogo.Params{
				"tag": {Type: "string", Desc: "Tag to filter by", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				tag := strings.TrimSpace(args["tag"])
				if tag == "" {
					return "", fmt.Errorf("missing required parameter: tag")
				}
				apiURL := fmt.Sprintf("https://api.notion.com/v1/databases/%s/query", databaseID)
				result, err := notionAPI(ctx, "POST", apiURL, map[string]any{
					"filter": map[string]any{
						"property": "Tag",
						"select":   map[string]any{"equals": tag},
					},
				})
				if err != nil {
					return "", err
				}
				var raw struct {
					Results []struct {
						ID         string `json:"id"`
						URL        string `json:"url"`
						Properties struct {
							Name struct {
								Title []struct {
									Text struct {
										Content string `json:"content"`
									} `json:"text"`
								} `json:"title"`
							} `json:"Name"`
							Tag struct {
								Select *struct {
									Name string `json:"name"`
								} `json:"select"`
							} `json:"Tag"`
						} `json:"properties"`
					} `json:"results"`
				}
				if err := json.Unmarshal([]byte(result), &raw); err != nil {
					return result, nil
				}
				type pageEntry struct {
					PageID string `json:"page_id"`
					Title  string `json:"title"`
					Tag    string `json:"tag"`
					URL    string `json:"url"`
				}
				entries := make([]pageEntry, 0, len(raw.Results))
				for _, r := range raw.Results {
					title := ""
					if len(r.Properties.Name.Title) > 0 {
						title = r.Properties.Name.Title[0].Text.Content
					}
					tagName := ""
					if r.Properties.Tag.Select != nil {
						tagName = r.Properties.Tag.Select.Name
					}
					entries = append(entries, pageEntry{
						PageID: r.ID,
						Title:  title,
						Tag:    tagName,
						URL:    r.URL,
					})
				}
				b, _ := json.MarshalIndent(entries, "", "  ")
				return string(b), nil
			},
		},
	}
}
