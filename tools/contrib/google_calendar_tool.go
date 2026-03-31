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

// GoogleCalendar returns tools for interacting with Google Calendar.
// Clone of agno's GoogleCalendarTools. Returns: list_events, get_event, create_event, delete_event, search_events.
// Accepts a pre-obtained OAuth2 access token. If accessToken is empty, falls back to GOOGLE_CALENDAR_TOKEN env var.
func GoogleCalendar(accessToken string) []agnogo.ToolDef {
	if accessToken == "" {
		accessToken = os.Getenv("GOOGLE_CALENDAR_TOKEN")
	}

	client := &http.Client{Timeout: 15 * time.Second}
	baseURL := "https://www.googleapis.com/calendar/v3"

	doReq := func(ctx context.Context, method, path string, body []byte) ([]byte, error) {
		if accessToken == "" {
			return nil, fmt.Errorf("Google Calendar access token not configured: pass accessToken or set GOOGLE_CALENDAR_TOKEN")
		}

		var bodyReader io.Reader
		if body != nil {
			bodyReader = bytes.NewReader(body)
		}

		req, err := http.NewRequestWithContext(ctx, method, baseURL+path, bodyReader)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("Google Calendar request failed: %w", err)
		}
		defer resp.Body.Close()

		data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			return nil, fmt.Errorf("read error: %w", err)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("Google Calendar API returned HTTP %d: %s", resp.StatusCode, string(data[:min(len(data), 300)]))
		}
		return data, nil
	}

	calendarID := func(args map[string]string) string {
		if id := strings.TrimSpace(args["calendar_id"]); id != "" {
			return id
		}
		return "primary"
	}

	return []agnogo.ToolDef{
		{
			Name: "list_events",
			Desc: "List upcoming events from a Google Calendar.",
			Params: agnogo.Params{
				"calendar_id": {Type: "string", Desc: "Calendar ID (default 'primary')"},
				"max_results": {Type: "string", Desc: "Maximum number of events to return (default 10)"},
				"time_min":    {Type: "string", Desc: "Minimum start time in RFC3339 format (e.g. 2024-01-01T00:00:00Z)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				cID := calendarID(args)
				maxResults := "10"
				if mr := strings.TrimSpace(args["max_results"]); mr != "" {
					maxResults = mr
				}

				path := fmt.Sprintf("/calendars/%s/events?maxResults=%s&singleEvents=true&orderBy=startTime",
					url.PathEscape(cID), url.QueryEscape(maxResults))
				if tMin := strings.TrimSpace(args["time_min"]); tMin != "" {
					path += "&timeMin=" + url.QueryEscape(tMin)
				}

				data, err := doReq(ctx, "GET", path, nil)
				if err != nil {
					return "", err
				}

				// Return the raw response which includes items array
				var raw json.RawMessage
				if err := json.Unmarshal(data, &raw); err != nil {
					return "", fmt.Errorf("parse error: %w", err)
				}
				return string(raw), nil
			},
		},
		{
			Name: "get_event",
			Desc: "Get details of a specific Google Calendar event.",
			Params: agnogo.Params{
				"calendar_id": {Type: "string", Desc: "Calendar ID (default 'primary')"},
				"event_id":    {Type: "string", Desc: "Event ID to retrieve", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				cID := calendarID(args)
				eventID := strings.TrimSpace(args["event_id"])
				if eventID == "" {
					return "", fmt.Errorf("missing required parameter: event_id")
				}

				path := fmt.Sprintf("/calendars/%s/events/%s",
					url.PathEscape(cID), url.PathEscape(eventID))

				data, err := doReq(ctx, "GET", path, nil)
				if err != nil {
					return "", err
				}
				return string(data), nil
			},
		},
		{
			Name: "create_event",
			Desc: "Create a new event in Google Calendar.",
			Params: agnogo.Params{
				"calendar_id": {Type: "string", Desc: "Calendar ID (default 'primary')"},
				"summary":     {Type: "string", Desc: "Event title/summary", Required: true},
				"start":       {Type: "string", Desc: "Start time in RFC3339 format (e.g. 2024-01-15T09:00:00-05:00)", Required: true},
				"end":         {Type: "string", Desc: "End time in RFC3339 format (e.g. 2024-01-15T10:00:00-05:00)", Required: true},
				"description": {Type: "string", Desc: "Event description"},
				"location":    {Type: "string", Desc: "Event location"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				cID := calendarID(args)
				summary := strings.TrimSpace(args["summary"])
				if summary == "" {
					return "", fmt.Errorf("missing required parameter: summary")
				}
				start := strings.TrimSpace(args["start"])
				if start == "" {
					return "", fmt.Errorf("missing required parameter: start")
				}
				end := strings.TrimSpace(args["end"])
				if end == "" {
					return "", fmt.Errorf("missing required parameter: end")
				}

				event := map[string]interface{}{
					"summary": summary,
					"start":   map[string]string{"dateTime": start},
					"end":     map[string]string{"dateTime": end},
				}
				if desc := strings.TrimSpace(args["description"]); desc != "" {
					event["description"] = desc
				}
				if loc := strings.TrimSpace(args["location"]); loc != "" {
					event["location"] = loc
				}

				body, _ := json.Marshal(event)
				path := fmt.Sprintf("/calendars/%s/events", url.PathEscape(cID))

				data, err := doReq(ctx, "POST", path, body)
				if err != nil {
					return "", err
				}
				return string(data), nil
			},
		},
		{
			Name: "delete_event",
			Desc: "Delete an event from Google Calendar.",
			Params: agnogo.Params{
				"calendar_id": {Type: "string", Desc: "Calendar ID (default 'primary')"},
				"event_id":    {Type: "string", Desc: "Event ID to delete", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				cID := calendarID(args)
				eventID := strings.TrimSpace(args["event_id"])
				if eventID == "" {
					return "", fmt.Errorf("missing required parameter: event_id")
				}

				path := fmt.Sprintf("/calendars/%s/events/%s",
					url.PathEscape(cID), url.PathEscape(eventID))

				_, err := doReq(ctx, "DELETE", path, nil)
				if err != nil {
					return "", err
				}
				return fmt.Sprintf(`{"status":"deleted","event_id":"%s"}`, eventID), nil
			},
		},
		{
			Name: "search_events",
			Desc: "Search for events in Google Calendar by text query.",
			Params: agnogo.Params{
				"calendar_id": {Type: "string", Desc: "Calendar ID (default 'primary')"},
				"query":       {Type: "string", Desc: "Free-text search query", Required: true},
				"max_results": {Type: "string", Desc: "Maximum number of events to return (default 10)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				cID := calendarID(args)
				query := strings.TrimSpace(args["query"])
				if query == "" {
					return "", fmt.Errorf("missing required parameter: query")
				}
				maxResults := "10"
				if mr := strings.TrimSpace(args["max_results"]); mr != "" {
					maxResults = mr
				}

				path := fmt.Sprintf("/calendars/%s/events?q=%s&maxResults=%s",
					url.PathEscape(cID), url.QueryEscape(query), url.QueryEscape(maxResults))

				data, err := doReq(ctx, "GET", path, nil)
				if err != nil {
					return "", err
				}
				return string(data), nil
			},
		},
	}
}
