package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/saeedalam/agnogo"
)

// SlackConfig configures Slack tools.
type SlackConfig struct {
	// Timeout in seconds. Default: 10.
	Timeout int
	// DefaultLimit for message history. Default: 10.
	DefaultLimit int
}

func (c *SlackConfig) defaults() {
	if c.Timeout <= 0 {
		c.Timeout = 10
	}
	if c.DefaultLimit <= 0 {
		c.DefaultLimit = 10
	}
}

// Slack returns tools for interacting with Slack.
// Uses Slack Web API directly (no external SDK).
func Slack(token string, cfgs ...SlackConfig) []agnogo.ToolDef {
	var cfg SlackConfig
	if len(cfgs) > 0 {
		cfg = cfgs[0]
	}
	cfg.defaults()

	client := &http.Client{Timeout: time.Duration(cfg.Timeout) * time.Second}

	// slackAPI calls a Slack API method and checks the response for errors.
	slackAPI := func(ctx context.Context, method string, body map[string]any) (json.RawMessage, error) {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("context cancelled: %w", err)
		}

		bodyJSON, _ := json.Marshal(body)
		req, err := http.NewRequestWithContext(ctx, "POST", "https://slack.com/api/"+method, bytes.NewReader(bodyJSON))
		if err != nil {
			return nil, fmt.Errorf("invalid request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("Slack API request failed: %w", err)
		}
		defer resp.Body.Close()

		data, err := io.ReadAll(io.LimitReader(resp.Body, 65536))
		if err != nil {
			return nil, fmt.Errorf("error reading Slack response: %w", err)
		}

		// Check the Slack API response envelope
		var envelope struct {
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		}
		if err := json.Unmarshal(data, &envelope); err != nil {
			return nil, fmt.Errorf("error parsing Slack response: %w", err)
		}
		if !envelope.OK {
			return nil, fmt.Errorf("Slack API error: %s", envelope.Error)
		}

		return json.RawMessage(data), nil
	}

	return []agnogo.ToolDef{
		{
			Name: "slack_send_message",
			Desc: "Send a message to a Slack channel",
			Params: agnogo.Params{
				"channel": {Type: "string", Desc: "Channel ID or name (e.g. #general or C1234)", Required: true},
				"text":    {Type: "string", Desc: "Message text", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				channel := strings.TrimSpace(args["channel"])
				if channel == "" {
					return "", fmt.Errorf("missing required parameter: channel")
				}
				text := args["text"]
				if text == "" {
					return "", fmt.Errorf("missing required parameter: text")
				}
				result, err := slackAPI(ctx, "chat.postMessage", map[string]any{
					"channel": channel,
					"text":    text,
				})
				if err != nil {
					return "", err
				}
				return string(result), nil
			},
		},
		{
			Name: "slack_reply",
			Desc: "Reply to a message thread in a Slack channel",
			Params: agnogo.Params{
				"channel":   {Type: "string", Desc: "Channel ID", Required: true},
				"thread_ts": {Type: "string", Desc: "Thread timestamp (ts) of the parent message", Required: true},
				"text":      {Type: "string", Desc: "Reply text", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				channel := strings.TrimSpace(args["channel"])
				if channel == "" {
					return "", fmt.Errorf("missing required parameter: channel")
				}
				threadTS := strings.TrimSpace(args["thread_ts"])
				if threadTS == "" {
					return "", fmt.Errorf("missing required parameter: thread_ts")
				}
				text := args["text"]
				if text == "" {
					return "", fmt.Errorf("missing required parameter: text")
				}
				result, err := slackAPI(ctx, "chat.postMessage", map[string]any{
					"channel":   channel,
					"thread_ts": threadTS,
					"text":      text,
				})
				if err != nil {
					return "", err
				}
				return string(result), nil
			},
		},
		{
			Name: "slack_react",
			Desc: "Add a reaction (emoji) to a message",
			Params: agnogo.Params{
				"channel":   {Type: "string", Desc: "Channel ID", Required: true},
				"timestamp": {Type: "string", Desc: "Message timestamp (ts)", Required: true},
				"name":      {Type: "string", Desc: "Emoji name without colons (e.g. thumbsup)", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				channel := strings.TrimSpace(args["channel"])
				if channel == "" {
					return "", fmt.Errorf("missing required parameter: channel")
				}
				timestamp := strings.TrimSpace(args["timestamp"])
				if timestamp == "" {
					return "", fmt.Errorf("missing required parameter: timestamp")
				}
				name := strings.TrimSpace(args["name"])
				if name == "" {
					return "", fmt.Errorf("missing required parameter: name")
				}
				// Strip colons if user included them
				name = strings.Trim(name, ":")
				result, err := slackAPI(ctx, "reactions.add", map[string]any{
					"channel":   channel,
					"timestamp": timestamp,
					"name":      name,
				})
				if err != nil {
					return "", err
				}
				return string(result), nil
			},
		},
		{
			Name: "slack_list_channels",
			Desc: "List Slack channels with cursor-based pagination",
			Params: agnogo.Params{
				"limit":  {Type: "string", Desc: "Max channels per page (default 20)"},
				"cursor": {Type: "string", Desc: "Pagination cursor for next page"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				limit := 20
				if l := strings.TrimSpace(args["limit"]); l != "" {
					if n, err := strconv.Atoi(l); err == nil && n > 0 {
						limit = n
					}
				}
				body := map[string]any{"limit": limit}
				if cursor := strings.TrimSpace(args["cursor"]); cursor != "" {
					body["cursor"] = cursor
				}
				result, err := slackAPI(ctx, "conversations.list", body)
				if err != nil {
					return "", err
				}
				var resp struct {
					Channels []struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"channels"`
					ResponseMetadata struct {
						NextCursor string `json:"next_cursor"`
					} `json:"response_metadata"`
				}
				if err := json.Unmarshal(result, &resp); err != nil {
					return "", fmt.Errorf("error parsing channels: %w", err)
				}
				var out strings.Builder
				for _, ch := range resp.Channels {
					out.WriteString(fmt.Sprintf("- #%s (%s)\n", ch.Name, ch.ID))
				}
				if resp.ResponseMetadata.NextCursor != "" {
					out.WriteString(fmt.Sprintf("\n[next_cursor: %s]\n", resp.ResponseMetadata.NextCursor))
				}
				return out.String(), nil
			},
		},
		{
			Name: "slack_read_messages",
			Desc: "Read recent messages from a Slack channel with cursor-based pagination",
			Params: agnogo.Params{
				"channel": {Type: "string", Desc: "Channel ID", Required: true},
				"limit":   {Type: "string", Desc: fmt.Sprintf("Max messages (default %d)", cfg.DefaultLimit)},
				"cursor":  {Type: "string", Desc: "Pagination cursor for next page"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				channel := strings.TrimSpace(args["channel"])
				if channel == "" {
					return "", fmt.Errorf("missing required parameter: channel")
				}
				limit := cfg.DefaultLimit
				if l := strings.TrimSpace(args["limit"]); l != "" {
					if n, err := strconv.Atoi(l); err == nil && n > 0 {
						limit = n
					}
				}
				body := map[string]any{
					"channel": channel,
					"limit":   limit,
				}
				if cursor := strings.TrimSpace(args["cursor"]); cursor != "" {
					body["cursor"] = cursor
				}
				result, err := slackAPI(ctx, "conversations.history", body)
				if err != nil {
					return "", err
				}
				return string(result), nil
			},
		},
	}
}
