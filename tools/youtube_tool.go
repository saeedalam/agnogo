package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/saeedalam/agnogo"
)

// YouTube returns tools for fetching YouTube video data and captions.
// Clone of agno's YouTubeTools.
func YouTube() []agnogo.ToolDef {
	client := &http.Client{Timeout: 15 * time.Second}

	// parseVideoID extracts the video ID from various YouTube URL formats.
	parseVideoID := func(rawURL string) (string, error) {
		rawURL = strings.TrimSpace(rawURL)

		// Try youtu.be/ID
		if strings.Contains(rawURL, "youtu.be/") {
			u, err := url.Parse(rawURL)
			if err == nil {
				parts := strings.SplitN(strings.TrimPrefix(u.Path, "/"), "/", 2)
				if len(parts) > 0 && parts[0] != "" {
					return parts[0], nil
				}
			}
		}

		// Try youtube.com formats
		if strings.Contains(rawURL, "youtube.com") {
			u, err := url.Parse(rawURL)
			if err == nil {
				// /watch?v=ID
				if v := u.Query().Get("v"); v != "" {
					return v, nil
				}
				// /embed/ID or /v/ID
				path := u.Path
				for _, prefix := range []string{"/embed/", "/v/"} {
					if strings.HasPrefix(path, prefix) {
						id := strings.TrimPrefix(path, prefix)
						id = strings.SplitN(id, "/", 2)[0]
						if id != "" {
							return id, nil
						}
					}
				}
			}
		}

		// Try bare ID (11 characters, alphanumeric + _ -)
		re := regexp.MustCompile(`^[a-zA-Z0-9_-]{11}$`)
		if re.MatchString(rawURL) {
			return rawURL, nil
		}

		return "", fmt.Errorf("could not parse video ID from URL: %s", rawURL)
	}

	httpGet := func(ctx context.Context, u string) ([]byte, int, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
		if err != nil {
			return nil, 0, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
		return data, resp.StatusCode, nil
	}

	return []agnogo.ToolDef{
		{
			Name: "get_video_data",
			Desc: "Get metadata for a YouTube video (title, author, thumbnail, etc.)",
			Params: agnogo.Params{
				"url": {Type: "string", Desc: "YouTube video URL", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				rawURL := strings.TrimSpace(args["url"])
				if rawURL == "" {
					return "", fmt.Errorf("missing required parameter: url")
				}
				videoID, err := parseVideoID(rawURL)
				if err != nil {
					return "", err
				}
				oembedURL := fmt.Sprintf("https://www.youtube.com/oembed?format=json&url=%s",
					url.QueryEscape("https://www.youtube.com/watch?v="+videoID))
				data, status, err := httpGet(ctx, oembedURL)
				if err != nil {
					return "", fmt.Errorf("failed to fetch video data: %w", err)
				}
				if status >= 400 {
					return "", fmt.Errorf("YouTube oembed error %d: %s", status, string(data))
				}
				// Pretty-print JSON
				var parsed map[string]any
				if json.Unmarshal(data, &parsed) == nil {
					b, _ := json.MarshalIndent(parsed, "", "  ")
					return string(b), nil
				}
				return string(data), nil
			},
		},
		{
			Name: "get_video_captions",
			Desc: "Get captions/transcript for a YouTube video",
			Params: agnogo.Params{
				"url": {Type: "string", Desc: "YouTube video URL", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				rawURL := strings.TrimSpace(args["url"])
				if rawURL == "" {
					return "", fmt.Errorf("missing required parameter: url")
				}
				videoID, err := parseVideoID(rawURL)
				if err != nil {
					return "", err
				}

				// Attempt 1: timedtext API
				timedtextURL := fmt.Sprintf("https://www.youtube.com/api/timedtext?v=%s&lang=en&fmt=json3", url.QueryEscape(videoID))
				data, status, err := httpGet(ctx, timedtextURL)
				if err == nil && status == 200 && len(data) > 10 {
					var tt struct {
						Events []struct {
							Segs []struct {
								UTF8 string `json:"utf8"`
							} `json:"segs"`
						} `json:"events"`
					}
					if json.Unmarshal(data, &tt) == nil && len(tt.Events) > 0 {
						var lines []string
						for _, ev := range tt.Events {
							for _, seg := range ev.Segs {
								text := strings.TrimSpace(seg.UTF8)
								if text != "" && text != "\n" {
									lines = append(lines, text)
								}
							}
						}
						if len(lines) > 0 {
							return strings.Join(lines, " "), nil
						}
					}
				}

				// Attempt 2: fetch page HTML and look for caption tracks
				pageURL := "https://www.youtube.com/watch?v=" + videoID
				pageData, pageStatus, pageErr := httpGet(ctx, pageURL)
				if pageErr == nil && pageStatus == 200 {
					pageStr := string(pageData)
					// Look for "captionTracks" in the page JSON
					re := regexp.MustCompile(`"captionTracks"\s*:\s*\[(.*?)\]`)
					if match := re.FindStringSubmatch(pageStr); len(match) > 1 {
						// Extract first baseUrl
						urlRe := regexp.MustCompile(`"baseUrl"\s*:\s*"(.*?)"`)
						if urlMatch := urlRe.FindStringSubmatch(match[1]); len(urlMatch) > 1 {
							captionURL := strings.ReplaceAll(urlMatch[1], `\u0026`, "&")
							// Fetch the caption track
							capData, capStatus, capErr := httpGet(ctx, captionURL+"&fmt=json3")
							if capErr == nil && capStatus == 200 {
								var tt struct {
									Events []struct {
										Segs []struct {
											UTF8 string `json:"utf8"`
										} `json:"segs"`
									} `json:"events"`
								}
								if json.Unmarshal(capData, &tt) == nil && len(tt.Events) > 0 {
									var lines []string
									for _, ev := range tt.Events {
										for _, seg := range ev.Segs {
											text := strings.TrimSpace(seg.UTF8)
											if text != "" && text != "\n" {
												lines = append(lines, text)
											}
										}
									}
									if len(lines) > 0 {
										return strings.Join(lines, " "), nil
									}
								}
							}
						}
					}
				}

				return "Captions not available for this video", nil
			},
		},
	}
}
