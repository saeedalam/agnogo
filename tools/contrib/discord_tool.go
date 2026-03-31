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

// Discord returns tools for interacting with the Discord API.
// Clone of agno's DiscordTools.
// If token is empty, falls back to DISCORD_BOT_TOKEN env var.
func Discord(token string) []agnogo.ToolDef {
	if token == "" {
		token = os.Getenv("DISCORD_BOT_TOKEN")
	}

	client := &http.Client{Timeout: 15 * time.Second}
	baseURL := "https://discord.com/api/v10"

	doRequest := func(ctx context.Context, method, path string, body interface{}) (string, error) {
		if token == "" {
			return "", fmt.Errorf("DISCORD_BOT_TOKEN not set")
		}

		var reqBody io.Reader
		if body != nil {
			data, err := json.Marshal(body)
			if err != nil {
				return "", fmt.Errorf("marshal error: %w", err)
			}
			reqBody = bytes.NewReader(data)
		}

		req, err := http.NewRequestWithContext(ctx, method, baseURL+path, reqBody)
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bot "+token)
		req.Header.Set("User-Agent", "agnogo/1.0")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()
		respData, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			return "", fmt.Errorf("read error: %w", err)
		}
		if resp.StatusCode >= 400 {
			return "", fmt.Errorf("Discord API HTTP %d: %s", resp.StatusCode, string(respData))
		}
		if len(respData) == 0 {
			return `{"status":"ok"}`, nil
		}
		return string(respData), nil
	}

	return []agnogo.ToolDef{
		{
			Name: "discord_send_message",
			Desc: "Send a message to a Discord channel",
			Params: agnogo.Params{
				"channel_id": {Type: "string", Desc: "Discord channel ID", Required: true},
				"message":    {Type: "string", Desc: "Message content to send", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				channelID := strings.TrimSpace(args["channel_id"])
				if channelID == "" {
					return "", fmt.Errorf("missing required parameter: channel_id")
				}
				message := strings.TrimSpace(args["message"])
				if message == "" {
					return "", fmt.Errorf("missing required parameter: message")
				}
				payload := map[string]string{"content": message}
				return doRequest(ctx, "POST", fmt.Sprintf("/channels/%s/messages", channelID), payload)
			},
		},
		{
			Name: "discord_get_channel_messages",
			Desc: "Get recent messages from a Discord channel",
			Params: agnogo.Params{
				"channel_id": {Type: "string", Desc: "Discord channel ID", Required: true},
				"limit":      {Type: "string", Desc: "Number of messages to fetch (default 50, max 100)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				channelID := strings.TrimSpace(args["channel_id"])
				if channelID == "" {
					return "", fmt.Errorf("missing required parameter: channel_id")
				}
				limit := "50"
				if l := strings.TrimSpace(args["limit"]); l != "" {
					limit = l
				}
				return doRequest(ctx, "GET", fmt.Sprintf("/channels/%s/messages?limit=%s", channelID, limit), nil)
			},
		},
		{
			Name: "discord_get_channel_info",
			Desc: "Get information about a Discord channel",
			Params: agnogo.Params{
				"channel_id": {Type: "string", Desc: "Discord channel ID", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				channelID := strings.TrimSpace(args["channel_id"])
				if channelID == "" {
					return "", fmt.Errorf("missing required parameter: channel_id")
				}
				return doRequest(ctx, "GET", fmt.Sprintf("/channels/%s", channelID), nil)
			},
		},
		{
			Name: "discord_list_channels",
			Desc: "List all channels in a Discord guild (server)",
			Params: agnogo.Params{
				"guild_id": {Type: "string", Desc: "Discord guild (server) ID", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				guildID := strings.TrimSpace(args["guild_id"])
				if guildID == "" {
					return "", fmt.Errorf("missing required parameter: guild_id")
				}
				return doRequest(ctx, "GET", fmt.Sprintf("/guilds/%s/channels", guildID), nil)
			},
		},
		{
			Name: "discord_delete_message",
			Desc: "Delete a message from a Discord channel",
			Params: agnogo.Params{
				"channel_id": {Type: "string", Desc: "Discord channel ID", Required: true},
				"message_id": {Type: "string", Desc: "ID of the message to delete", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				channelID := strings.TrimSpace(args["channel_id"])
				if channelID == "" {
					return "", fmt.Errorf("missing required parameter: channel_id")
				}
				messageID := strings.TrimSpace(args["message_id"])
				if messageID == "" {
					return "", fmt.Errorf("missing required parameter: message_id")
				}
				return doRequest(ctx, "DELETE", fmt.Sprintf("/channels/%s/messages/%s", channelID, messageID), nil)
			},
		},
	}
}
