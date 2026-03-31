package contrib

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/saeedalam/agnogo"
)

// Unsplash returns tools for searching and retrieving photos from Unsplash.
// Clone of agno's UnsplashTools. Returns: search_photos, get_photo, get_random_photo.
// If accessKey is empty, falls back to UNSPLASH_ACCESS_KEY env var.
func Unsplash(accessKey string) []agnogo.ToolDef {
	if accessKey == "" {
		accessKey = os.Getenv("UNSPLASH_ACCESS_KEY")
	}

	client := &http.Client{Timeout: 15 * time.Second}
	baseURL := "https://api.unsplash.com"

	doReq := func(ctx context.Context, method, path string) ([]byte, error) {
		if accessKey == "" {
			return nil, fmt.Errorf("Unsplash access key not configured: pass accessKey or set UNSPLASH_ACCESS_KEY")
		}
		req, err := http.NewRequestWithContext(ctx, method, baseURL+path, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Authorization", "Client-ID "+accessKey)
		req.Header.Set("Accept-Version", "v1")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("Unsplash request failed: %w", err)
		}
		defer resp.Body.Close()

		data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			return nil, fmt.Errorf("read error: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("Unsplash API returned HTTP %d: %s", resp.StatusCode, string(data[:min(len(data), 200)]))
		}
		return data, nil
	}

	// parsePhoto extracts a consistent photo object from raw JSON.
	parsePhoto := func(raw json.RawMessage) map[string]interface{} {
		var photo struct {
			ID          string `json:"id"`
			Description string `json:"description"`
			Width       int    `json:"width"`
			Height      int    `json:"height"`
			Likes       int    `json:"likes"`
			Views       int    `json:"views"`
			Downloads   int    `json:"downloads"`
			URLs        struct {
				Raw     string `json:"raw"`
				Full    string `json:"full"`
				Regular string `json:"regular"`
				Small   string `json:"small"`
				Thumb   string `json:"thumb"`
			} `json:"urls"`
			User struct {
				Name     string `json:"name"`
				Username string `json:"username"`
			} `json:"user"`
			Exif *struct {
				Make         string `json:"make"`
				Model        string `json:"model"`
				ExposureTime string `json:"exposure_time"`
				Aperture     string `json:"aperture"`
				FocalLength  string `json:"focal_length"`
				ISO          int    `json:"iso"`
			} `json:"exif"`
			Location *struct {
				Name    string `json:"name"`
				City    string `json:"city"`
				Country string `json:"country"`
			} `json:"location"`
		}
		_ = json.Unmarshal(raw, &photo)

		result := map[string]interface{}{
			"id":          photo.ID,
			"description": photo.Description,
			"width":       photo.Width,
			"height":      photo.Height,
			"urls": map[string]string{
				"raw":     photo.URLs.Raw,
				"full":    photo.URLs.Full,
				"regular": photo.URLs.Regular,
				"small":   photo.URLs.Small,
				"thumb":   photo.URLs.Thumb,
			},
			"author": map[string]string{
				"name":     photo.User.Name,
				"username": photo.User.Username,
			},
			"likes": photo.Likes,
		}
		if photo.Views > 0 {
			result["views"] = photo.Views
		}
		if photo.Downloads > 0 {
			result["downloads"] = photo.Downloads
		}
		if photo.Exif != nil {
			result["exif"] = photo.Exif
		}
		if photo.Location != nil && photo.Location.Name != "" {
			result["location"] = photo.Location
		}
		return result
	}

	return []agnogo.ToolDef{
		{
			Name: "search_photos",
			Desc: "Search for photos on Unsplash by keyword. Returns photo URLs, dimensions, and author info.",
			Params: agnogo.Params{
				"query":       {Type: "string", Desc: "Search query for photos", Required: true},
				"per_page":    {Type: "string", Desc: "Results per page (default 10)"},
				"page":        {Type: "string", Desc: "Page number (default 1)"},
				"orientation": {Type: "string", Desc: "Photo orientation: landscape, portrait, or squarish"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				query := strings.TrimSpace(args["query"])
				if query == "" {
					return "", fmt.Errorf("missing required parameter: query")
				}
				perPage := "10"
				if pp := strings.TrimSpace(args["per_page"]); pp != "" {
					perPage = pp
				}
				page := "1"
				if p := strings.TrimSpace(args["page"]); p != "" {
					page = p
				}

				path := fmt.Sprintf("/search/photos?query=%s&per_page=%s&page=%s",
					url.QueryEscape(query), url.QueryEscape(perPage), url.QueryEscape(page))
				if orientation := strings.TrimSpace(args["orientation"]); orientation != "" {
					path += "&orientation=" + url.QueryEscape(orientation)
				}

				data, err := doReq(ctx, "GET", path)
				if err != nil {
					return "", err
				}

				var searchResp struct {
					Total      int               `json:"total"`
					TotalPages int               `json:"total_pages"`
					Results    []json.RawMessage  `json:"results"`
				}
				if err := json.Unmarshal(data, &searchResp); err != nil {
					return "", fmt.Errorf("parse error: %w", err)
				}

				photos := make([]map[string]interface{}, 0, len(searchResp.Results))
				for _, raw := range searchResp.Results {
					photos = append(photos, parsePhoto(raw))
				}

				out, _ := json.Marshal(map[string]interface{}{
					"total":       searchResp.Total,
					"total_pages": searchResp.TotalPages,
					"photos":      photos,
				})
				return string(out), nil
			},
		},
		{
			Name: "get_photo",
			Desc: "Get detailed information about a specific Unsplash photo by ID.",
			Params: agnogo.Params{
				"photo_id": {Type: "string", Desc: "The Unsplash photo ID", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				photoID := strings.TrimSpace(args["photo_id"])
				if photoID == "" {
					return "", fmt.Errorf("missing required parameter: photo_id")
				}

				data, err := doReq(ctx, "GET", "/photos/"+url.PathEscape(photoID))
				if err != nil {
					return "", err
				}

				photo := parsePhoto(json.RawMessage(data))
				out, _ := json.Marshal(photo)
				return string(out), nil
			},
		},
		{
			Name: "get_random_photo",
			Desc: "Get random photo(s) from Unsplash, optionally filtered by query and orientation.",
			Params: agnogo.Params{
				"query":       {Type: "string", Desc: "Optional search query to filter random photos"},
				"orientation": {Type: "string", Desc: "Photo orientation: landscape, portrait, or squarish"},
				"count":       {Type: "string", Desc: "Number of random photos to return (default 1)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				count := "1"
				if c := strings.TrimSpace(args["count"]); c != "" {
					count = c
				}

				path := "/photos/random?count=" + url.QueryEscape(count)
				if query := strings.TrimSpace(args["query"]); query != "" {
					path += "&query=" + url.QueryEscape(query)
				}
				if orientation := strings.TrimSpace(args["orientation"]); orientation != "" {
					path += "&orientation=" + url.QueryEscape(orientation)
				}

				data, err := doReq(ctx, "GET", path)
				if err != nil {
					return "", err
				}

				// The API returns an array when count >= 1
				var rawPhotos []json.RawMessage
				if err := json.Unmarshal(data, &rawPhotos); err != nil {
					// Single photo case (count=1 may return object)
					photo := parsePhoto(json.RawMessage(data))
					out, _ := json.Marshal(photo)
					return string(out), nil
				}

				photos := make([]map[string]interface{}, 0, len(rawPhotos))
				for _, raw := range rawPhotos {
					photos = append(photos, parsePhoto(raw))
				}

				// Return single object if count was 1
				if n, _ := strconv.Atoi(count); n == 1 && len(photos) == 1 {
					out, _ := json.Marshal(photos[0])
					return string(out), nil
				}

				out, _ := json.Marshal(photos)
				return string(out), nil
			},
		},
	}
}
