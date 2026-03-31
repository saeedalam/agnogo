package contrib

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/saeedalam/agnogo"
)

// GoogleSheets returns tools for interacting with Google Sheets.
// Clone of agno's GoogleSheetsTools. Returns: read_sheet, create_sheet, update_sheet.
// Accepts a pre-obtained OAuth2 access token. If accessToken is empty, falls back to GOOGLE_SHEETS_TOKEN env var.
func GoogleSheets(accessToken string) []agnogo.ToolDef {
	if accessToken == "" {
		accessToken = os.Getenv("GOOGLE_SHEETS_TOKEN")
	}

	client := &http.Client{Timeout: 15 * time.Second}
	baseURL := "https://sheets.googleapis.com/v4/spreadsheets"

	doReq := func(ctx context.Context, method, fullURL string, body []byte) ([]byte, error) {
		if accessToken == "" {
			return nil, fmt.Errorf("Google Sheets access token not configured: pass accessToken or set GOOGLE_SHEETS_TOKEN")
		}

		var bodyReader io.Reader
		if body != nil {
			bodyReader = bytes.NewReader(body)
		}

		req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("Google Sheets request failed: %w", err)
		}
		defer resp.Body.Close()

		data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			return nil, fmt.Errorf("read error: %w", err)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("Google Sheets API returned HTTP %d: %s", resp.StatusCode, string(data[:min(len(data), 300)]))
		}
		return data, nil
	}

	return []agnogo.ToolDef{
		{
			Name: "read_sheet",
			Desc: "Read data from a Google Sheets spreadsheet. Returns rows as a JSON array.",
			Params: agnogo.Params{
				"spreadsheet_id": {Type: "string", Desc: "The spreadsheet ID from the Google Sheets URL", Required: true},
				"range":          {Type: "string", Desc: "Cell range in A1 notation (e.g. 'Sheet1!A1:D10')", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				spreadsheetID := strings.TrimSpace(args["spreadsheet_id"])
				if spreadsheetID == "" {
					return "", fmt.Errorf("missing required parameter: spreadsheet_id")
				}
				cellRange := strings.TrimSpace(args["range"])
				if cellRange == "" {
					return "", fmt.Errorf("missing required parameter: range")
				}

				u := fmt.Sprintf("%s/%s/values/%s",
					baseURL, url.PathEscape(spreadsheetID), url.PathEscape(cellRange))

				data, err := doReq(ctx, "GET", u, nil)
				if err != nil {
					return "", err
				}

				var resp struct {
					Range  string          `json:"range"`
					Values [][]interface{} `json:"values"`
				}
				if err := json.Unmarshal(data, &resp); err != nil {
					return "", fmt.Errorf("parse error: %w", err)
				}

				out, _ := json.Marshal(resp.Values)
				return string(out), nil
			},
		},
		{
			Name: "create_sheet",
			Desc: "Create a new Google Sheets spreadsheet.",
			Params: agnogo.Params{
				"title": {Type: "string", Desc: "Title for the new spreadsheet", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				title := strings.TrimSpace(args["title"])
				if title == "" {
					return "", fmt.Errorf("missing required parameter: title")
				}

				body, _ := json.Marshal(map[string]interface{}{
					"properties": map[string]string{
						"title": title,
					},
				})

				data, err := doReq(ctx, "POST", baseURL, body)
				if err != nil {
					return "", err
				}

				var resp struct {
					SpreadsheetID  string `json:"spreadsheetId"`
					SpreadsheetURL string `json:"spreadsheetUrl"`
				}
				if err := json.Unmarshal(data, &resp); err != nil {
					return "", fmt.Errorf("parse error: %w", err)
				}

				out, _ := json.Marshal(map[string]string{
					"spreadsheet_id": resp.SpreadsheetID,
					"url":            resp.SpreadsheetURL,
				})
				return string(out), nil
			},
		},
		{
			Name: "update_sheet",
			Desc: "Update cells in a Google Sheets spreadsheet with new values.",
			Params: agnogo.Params{
				"spreadsheet_id": {Type: "string", Desc: "The spreadsheet ID", Required: true},
				"range":          {Type: "string", Desc: "Cell range in A1 notation (e.g. 'Sheet1!A1:D10')", Required: true},
				"values":         {Type: "string", Desc: "JSON 2D array of values (e.g. '[[\"Name\",\"Age\"],[\"Alice\",30]]')", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				spreadsheetID := strings.TrimSpace(args["spreadsheet_id"])
				if spreadsheetID == "" {
					return "", fmt.Errorf("missing required parameter: spreadsheet_id")
				}
				cellRange := strings.TrimSpace(args["range"])
				if cellRange == "" {
					return "", fmt.Errorf("missing required parameter: range")
				}
				valuesStr := strings.TrimSpace(args["values"])
				if valuesStr == "" {
					return "", fmt.Errorf("missing required parameter: values")
				}

				// Parse the values JSON string
				var values [][]interface{}
				if err := json.Unmarshal([]byte(valuesStr), &values); err != nil {
					return "", fmt.Errorf("invalid values JSON: %w", err)
				}

				body, _ := json.Marshal(map[string]interface{}{
					"values": values,
				})

				u := fmt.Sprintf("%s/%s/values/%s?valueInputOption=RAW",
					baseURL, url.PathEscape(spreadsheetID), url.PathEscape(cellRange))

				data, err := doReq(ctx, "PUT", u, body)
				if err != nil {
					return "", err
				}
				return string(data), nil
			},
		},
	}
}
