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

// WhatsApp returns tools for sending messages via the WhatsApp Business Cloud API.
// Clone of agno's WhatsAppTools.
func WhatsApp(accessToken, phoneNumberID string) []agnogo.ToolDef {
	if accessToken == "" {
		accessToken = os.Getenv("WHATSAPP_ACCESS_TOKEN")
	}
	if phoneNumberID == "" {
		phoneNumberID = os.Getenv("WHATSAPP_PHONE_NUMBER_ID")
	}

	baseURL := fmt.Sprintf("https://graph.facebook.com/v22.0/%s/messages", phoneNumberID)
	client := &http.Client{Timeout: 15 * time.Second}

	waAPI := func(ctx context.Context, body map[string]any) (string, error) {
		if err := ctx.Err(); err != nil {
			return "", fmt.Errorf("context cancelled: %w", err)
		}
		body["messaging_product"] = "whatsapp"
		data, _ := json.Marshal(body)
		req, err := http.NewRequestWithContext(ctx, "POST", baseURL, bytes.NewReader(data))
		if err != nil {
			return "", fmt.Errorf("invalid request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("WhatsApp API request failed: %w", err)
		}
		defer resp.Body.Close()
		respData, _ := io.ReadAll(io.LimitReader(resp.Body, 65536))

		if resp.StatusCode >= 400 {
			return "", fmt.Errorf("WhatsApp API error %d: %s", resp.StatusCode, string(respData))
		}
		return string(respData), nil
	}

	return []agnogo.ToolDef{
		{
			Name: "send_text_message",
			Desc: "Send a text message via WhatsApp",
			Params: agnogo.Params{
				"recipient":   {Type: "string", Desc: "Recipient phone number in international format", Required: true},
				"text":        {Type: "string", Desc: "Message text to send", Required: true},
				"preview_url": {Type: "string", Desc: "Enable URL preview (default false)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				recipient := strings.TrimSpace(args["recipient"])
				if recipient == "" {
					return "", fmt.Errorf("missing required parameter: recipient")
				}
				text := args["text"]
				if text == "" {
					return "", fmt.Errorf("missing required parameter: text")
				}
				previewURL := false
				if strings.TrimSpace(args["preview_url"]) == "true" {
					previewURL = true
				}
				return waAPI(ctx, map[string]any{
					"to":   recipient,
					"type": "text",
					"text": map[string]any{
						"preview_url": previewURL,
						"body":        text,
					},
				})
			},
		},
		{
			Name: "send_template_message",
			Desc: "Send a template message via WhatsApp",
			Params: agnogo.Params{
				"recipient":     {Type: "string", Desc: "Recipient phone number in international format", Required: true},
				"template_name": {Type: "string", Desc: "Name of the approved template", Required: true},
				"language_code": {Type: "string", Desc: "Language code (default en_US)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				recipient := strings.TrimSpace(args["recipient"])
				if recipient == "" {
					return "", fmt.Errorf("missing required parameter: recipient")
				}
				templateName := strings.TrimSpace(args["template_name"])
				if templateName == "" {
					return "", fmt.Errorf("missing required parameter: template_name")
				}
				langCode := "en_US"
				if lc := strings.TrimSpace(args["language_code"]); lc != "" {
					langCode = lc
				}
				return waAPI(ctx, map[string]any{
					"to":   recipient,
					"type": "template",
					"template": map[string]any{
						"name": templateName,
						"language": map[string]any{
							"code": langCode,
						},
					},
				})
			},
		},
		{
			Name: "send_image",
			Desc: "Send an image via WhatsApp",
			Params: agnogo.Params{
				"recipient": {Type: "string", Desc: "Recipient phone number in international format", Required: true},
				"image_url": {Type: "string", Desc: "URL of the image to send", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				recipient := strings.TrimSpace(args["recipient"])
				if recipient == "" {
					return "", fmt.Errorf("missing required parameter: recipient")
				}
				imageURL := strings.TrimSpace(args["image_url"])
				if imageURL == "" {
					return "", fmt.Errorf("missing required parameter: image_url")
				}
				return waAPI(ctx, map[string]any{
					"to":   recipient,
					"type": "image",
					"image": map[string]any{
						"link": imageURL,
					},
				})
			},
		},
		{
			Name: "send_location",
			Desc: "Send a location via WhatsApp",
			Params: agnogo.Params{
				"recipient": {Type: "string", Desc: "Recipient phone number in international format", Required: true},
				"latitude":  {Type: "string", Desc: "Latitude of the location", Required: true},
				"longitude": {Type: "string", Desc: "Longitude of the location", Required: true},
				"name":      {Type: "string", Desc: "Name of the location"},
				"address":   {Type: "string", Desc: "Address of the location"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				recipient := strings.TrimSpace(args["recipient"])
				if recipient == "" {
					return "", fmt.Errorf("missing required parameter: recipient")
				}
				lat := strings.TrimSpace(args["latitude"])
				if lat == "" {
					return "", fmt.Errorf("missing required parameter: latitude")
				}
				lon := strings.TrimSpace(args["longitude"])
				if lon == "" {
					return "", fmt.Errorf("missing required parameter: longitude")
				}
				loc := map[string]any{
					"latitude":  lat,
					"longitude": lon,
				}
				if name := strings.TrimSpace(args["name"]); name != "" {
					loc["name"] = name
				}
				if addr := strings.TrimSpace(args["address"]); addr != "" {
					loc["address"] = addr
				}
				return waAPI(ctx, map[string]any{
					"to":       recipient,
					"type":     "location",
					"location": loc,
				})
			},
		},
		{
			Name: "send_reaction",
			Desc: "Send a reaction emoji to a WhatsApp message",
			Params: agnogo.Params{
				"recipient":  {Type: "string", Desc: "Recipient phone number in international format", Required: true},
				"message_id": {Type: "string", Desc: "ID of the message to react to", Required: true},
				"emoji":      {Type: "string", Desc: "Emoji to react with", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				recipient := strings.TrimSpace(args["recipient"])
				if recipient == "" {
					return "", fmt.Errorf("missing required parameter: recipient")
				}
				messageID := strings.TrimSpace(args["message_id"])
				if messageID == "" {
					return "", fmt.Errorf("missing required parameter: message_id")
				}
				emoji := args["emoji"]
				if emoji == "" {
					return "", fmt.Errorf("missing required parameter: emoji")
				}
				return waAPI(ctx, map[string]any{
					"to":   recipient,
					"type": "reaction",
					"reaction": map[string]any{
						"message_id": messageID,
						"emoji":      emoji,
					},
				})
			},
		},
	}
}
