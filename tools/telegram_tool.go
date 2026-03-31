package tools

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

// Telegram returns tools for interacting with the Telegram Bot API.
// Clone of agno's TelegramTools.
// If token or chatID are empty, falls back to TELEGRAM_TOKEN and TELEGRAM_CHAT_ID env vars.
func Telegram(token string, chatID string) []agnogo.ToolDef {
	if token == "" {
		token = os.Getenv("TELEGRAM_TOKEN")
	}
	if chatID == "" {
		chatID = os.Getenv("TELEGRAM_CHAT_ID")
	}

	client := &http.Client{Timeout: 15 * time.Second}
	baseURL := fmt.Sprintf("https://api.telegram.org/bot%s", token)

	postJSON := func(ctx context.Context, endpoint string, payload interface{}) (string, error) {
		if token == "" {
			return "", fmt.Errorf("TELEGRAM_TOKEN not set")
		}
		body, err := json.Marshal(payload)
		if err != nil {
			return "", fmt.Errorf("marshal error: %w", err)
		}
		req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/"+endpoint, bytes.NewReader(body))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "agnogo/1.0")
		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			return "", fmt.Errorf("read error: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("Telegram API HTTP %d: %s", resp.StatusCode, string(data))
		}
		return string(data), nil
	}

	resolveChatID := func(args map[string]string) string {
		if cid := strings.TrimSpace(args["chat_id"]); cid != "" {
			return cid
		}
		return chatID
	}

	return []agnogo.ToolDef{
		{
			Name: "send_message",
			Desc: "Send a message to a Telegram chat using the bot",
			Params: agnogo.Params{
				"message": {Type: "string", Desc: "Message text to send (supports Markdown)", Required: true},
				"chat_id": {Type: "string", Desc: "Telegram chat ID (uses default if not provided)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				message := strings.TrimSpace(args["message"])
				if message == "" {
					return "", fmt.Errorf("missing required parameter: message")
				}
				cid := resolveChatID(args)
				if cid == "" {
					return "", fmt.Errorf("chat_id is required (no default set)")
				}
				payload := map[string]string{
					"chat_id":    cid,
					"text":       message,
					"parse_mode": "Markdown",
				}
				return postJSON(ctx, "sendMessage", payload)
			},
		},
		{
			Name: "edit_message",
			Desc: "Edit an existing message in a Telegram chat",
			Params: agnogo.Params{
				"message":    {Type: "string", Desc: "New message text", Required: true},
				"message_id": {Type: "string", Desc: "ID of the message to edit", Required: true},
				"chat_id":   {Type: "string", Desc: "Telegram chat ID (uses default if not provided)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				message := strings.TrimSpace(args["message"])
				if message == "" {
					return "", fmt.Errorf("missing required parameter: message")
				}
				messageID := strings.TrimSpace(args["message_id"])
				if messageID == "" {
					return "", fmt.Errorf("missing required parameter: message_id")
				}
				cid := resolveChatID(args)
				if cid == "" {
					return "", fmt.Errorf("chat_id is required (no default set)")
				}
				payload := map[string]string{
					"chat_id":    cid,
					"message_id": messageID,
					"text":       message,
					"parse_mode": "Markdown",
				}
				return postJSON(ctx, "editMessageText", payload)
			},
		},
		{
			Name: "delete_message",
			Desc: "Delete a message from a Telegram chat",
			Params: agnogo.Params{
				"message_id": {Type: "string", Desc: "ID of the message to delete", Required: true},
				"chat_id":   {Type: "string", Desc: "Telegram chat ID (uses default if not provided)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				messageID := strings.TrimSpace(args["message_id"])
				if messageID == "" {
					return "", fmt.Errorf("missing required parameter: message_id")
				}
				cid := resolveChatID(args)
				if cid == "" {
					return "", fmt.Errorf("chat_id is required (no default set)")
				}
				payload := map[string]string{
					"chat_id":    cid,
					"message_id": messageID,
				}
				return postJSON(ctx, "deleteMessage", payload)
			},
		},
	}
}
