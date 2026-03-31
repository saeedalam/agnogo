package contrib

import (
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

// GoogleMaps returns tools for interacting with the Google Maps Platform APIs.
// Clone of agno's GoogleMapTools. Returns: search_places, geocode_address, get_directions, get_distance_matrix.
// If apiKey is empty, falls back to GOOGLE_MAPS_API_KEY env var.
func GoogleMaps(apiKey string) []agnogo.ToolDef {
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_MAPS_API_KEY")
	}

	client := &http.Client{Timeout: 15 * time.Second}

	doGet := func(ctx context.Context, u string) ([]byte, error) {
		if apiKey == "" {
			return nil, fmt.Errorf("Google Maps API key not configured: pass apiKey or set GOOGLE_MAPS_API_KEY")
		}
		req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("Google Maps request failed: %w", err)
		}
		defer resp.Body.Close()

		data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			return nil, fmt.Errorf("read error: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("Google Maps API returned HTTP %d: %s", resp.StatusCode, string(data[:min(len(data), 200)]))
		}
		return data, nil
	}

	return []agnogo.ToolDef{
		{
			Name: "search_places",
			Desc: "Search for places using Google Maps. Returns names, addresses, ratings, and locations.",
			Params: agnogo.Params{
				"query": {Type: "string", Desc: "Search query for places (e.g. 'restaurants in Sydney')", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				query := strings.TrimSpace(args["query"])
				if query == "" {
					return "", fmt.Errorf("missing required parameter: query")
				}

				u := fmt.Sprintf("https://maps.googleapis.com/maps/api/place/textsearch/json?query=%s&key=%s",
					url.QueryEscape(query), url.QueryEscape(apiKey))

				data, err := doGet(ctx, u)
				if err != nil {
					return "", err
				}

				var apiResp struct {
					Results []struct {
						Name             string   `json:"name"`
						FormattedAddress string   `json:"formatted_address"`
						Rating           float64  `json:"rating"`
						PlaceID          string   `json:"place_id"`
						Types            []string `json:"types"`
						Geometry         struct {
							Location struct {
								Lat float64 `json:"lat"`
								Lng float64 `json:"lng"`
							} `json:"location"`
						} `json:"geometry"`
					} `json:"results"`
					Status string `json:"status"`
				}
				if err := json.Unmarshal(data, &apiResp); err != nil {
					return "", fmt.Errorf("parse error: %w", err)
				}

				if apiResp.Status != "OK" && apiResp.Status != "ZERO_RESULTS" {
					return "", fmt.Errorf("Google Maps API error: %s", apiResp.Status)
				}

				var results []map[string]interface{}
				for _, r := range apiResp.Results {
					results = append(results, map[string]interface{}{
						"name":     r.Name,
						"address":  r.FormattedAddress,
						"rating":   r.Rating,
						"place_id": r.PlaceID,
						"types":    r.Types,
						"location": map[string]float64{
							"lat": r.Geometry.Location.Lat,
							"lng": r.Geometry.Location.Lng,
						},
					})
				}

				out, _ := json.Marshal(results)
				return string(out), nil
			},
		},
		{
			Name: "geocode_address",
			Desc: "Geocode an address to get its coordinates and formatted address using Google Maps.",
			Params: agnogo.Params{
				"address": {Type: "string", Desc: "Address to geocode", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				address := strings.TrimSpace(args["address"])
				if address == "" {
					return "", fmt.Errorf("missing required parameter: address")
				}

				u := fmt.Sprintf("https://maps.googleapis.com/maps/api/geocode/json?address=%s&key=%s",
					url.QueryEscape(address), url.QueryEscape(apiKey))

				data, err := doGet(ctx, u)
				if err != nil {
					return "", err
				}

				var apiResp struct {
					Results []struct {
						FormattedAddress string `json:"formatted_address"`
						PlaceID          string `json:"place_id"`
						Geometry         struct {
							Location struct {
								Lat float64 `json:"lat"`
								Lng float64 `json:"lng"`
							} `json:"location"`
						} `json:"geometry"`
					} `json:"results"`
					Status string `json:"status"`
				}
				if err := json.Unmarshal(data, &apiResp); err != nil {
					return "", fmt.Errorf("parse error: %w", err)
				}

				if apiResp.Status != "OK" || len(apiResp.Results) == 0 {
					return fmt.Sprintf(`{"error":"geocoding failed","status":"%s"}`, apiResp.Status), nil
				}

				r := apiResp.Results[0]
				out, _ := json.Marshal(map[string]interface{}{
					"formatted_address": r.FormattedAddress,
					"location": map[string]float64{
						"lat": r.Geometry.Location.Lat,
						"lng": r.Geometry.Location.Lng,
					},
					"place_id": r.PlaceID,
				})
				return string(out), nil
			},
		},
		{
			Name: "get_directions",
			Desc: "Get directions between two locations using Google Maps. Returns distance, duration, and step-by-step instructions.",
			Params: agnogo.Params{
				"origin":      {Type: "string", Desc: "Starting address or coordinates", Required: true},
				"destination": {Type: "string", Desc: "Destination address or coordinates", Required: true},
				"mode":        {Type: "string", Desc: "Travel mode: driving (default), walking, bicycling, or transit"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				origin := strings.TrimSpace(args["origin"])
				if origin == "" {
					return "", fmt.Errorf("missing required parameter: origin")
				}
				destination := strings.TrimSpace(args["destination"])
				if destination == "" {
					return "", fmt.Errorf("missing required parameter: destination")
				}
				mode := "driving"
				if m := strings.TrimSpace(args["mode"]); m != "" {
					mode = m
				}

				u := fmt.Sprintf("https://maps.googleapis.com/maps/api/directions/json?origin=%s&destination=%s&mode=%s&key=%s",
					url.QueryEscape(origin), url.QueryEscape(destination), url.QueryEscape(mode), url.QueryEscape(apiKey))

				data, err := doGet(ctx, u)
				if err != nil {
					return "", err
				}

				var apiResp struct {
					Routes []struct {
						Legs []struct {
							Distance struct {
								Text string `json:"text"`
							} `json:"distance"`
							Duration struct {
								Text string `json:"text"`
							} `json:"duration"`
							Steps []struct {
								HTMLInstructions string `json:"html_instructions"`
								Distance         struct {
									Text string `json:"text"`
								} `json:"distance"`
								Duration struct {
									Text string `json:"text"`
								} `json:"duration"`
							} `json:"steps"`
						} `json:"legs"`
					} `json:"routes"`
					Status string `json:"status"`
				}
				if err := json.Unmarshal(data, &apiResp); err != nil {
					return "", fmt.Errorf("parse error: %w", err)
				}

				if apiResp.Status != "OK" || len(apiResp.Routes) == 0 || len(apiResp.Routes[0].Legs) == 0 {
					return fmt.Sprintf(`{"error":"no route found","status":"%s"}`, apiResp.Status), nil
				}

				leg := apiResp.Routes[0].Legs[0]
				var steps []map[string]string
				for _, s := range leg.Steps {
					steps = append(steps, map[string]string{
						"instruction": s.HTMLInstructions,
						"distance":    s.Distance.Text,
						"duration":    s.Duration.Text,
					})
				}

				out, _ := json.Marshal(map[string]interface{}{
					"distance": leg.Distance.Text,
					"duration": leg.Duration.Text,
					"steps":    steps,
				})
				return string(out), nil
			},
		},
		{
			Name: "get_distance_matrix",
			Desc: "Get travel distance and time for a matrix of origins and destinations using Google Maps.",
			Params: agnogo.Params{
				"origins":      {Type: "string", Desc: "Pipe-separated origins (e.g. 'New York|Boston')", Required: true},
				"destinations": {Type: "string", Desc: "Pipe-separated destinations (e.g. 'Philadelphia|Washington DC')", Required: true},
				"mode":         {Type: "string", Desc: "Travel mode: driving (default), walking, bicycling, or transit"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				origins := strings.TrimSpace(args["origins"])
				if origins == "" {
					return "", fmt.Errorf("missing required parameter: origins")
				}
				destinations := strings.TrimSpace(args["destinations"])
				if destinations == "" {
					return "", fmt.Errorf("missing required parameter: destinations")
				}
				mode := "driving"
				if m := strings.TrimSpace(args["mode"]); m != "" {
					mode = m
				}

				u := fmt.Sprintf("https://maps.googleapis.com/maps/api/distancematrix/json?origins=%s&destinations=%s&mode=%s&key=%s",
					url.QueryEscape(origins), url.QueryEscape(destinations), url.QueryEscape(mode), url.QueryEscape(apiKey))

				data, err := doGet(ctx, u)
				if err != nil {
					return "", err
				}

				// Return the raw API response — it's already well-structured JSON
				var raw json.RawMessage
				if err := json.Unmarshal(data, &raw); err != nil {
					return "", fmt.Errorf("parse error: %w", err)
				}
				return string(raw), nil
			},
		},
	}
}
