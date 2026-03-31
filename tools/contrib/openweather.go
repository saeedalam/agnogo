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

// OpenWeather returns tools for interacting with the OpenWeatherMap API.
// Clone of agno's OpenWeatherTools.
// If apiKey is empty, falls back to OPENWEATHER_API_KEY env var.
func OpenWeather(apiKey string) []agnogo.ToolDef {
	if apiKey == "" {
		apiKey = os.Getenv("OPENWEATHER_API_KEY")
	}

	client := &http.Client{Timeout: 15 * time.Second}

	fetchJSON := func(ctx context.Context, u string, dest interface{}) error {
		req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", "agnogo/1.0")
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			return fmt.Errorf("read error: %w", err)
		}
		return json.Unmarshal(data, dest)
	}

	type geoResult struct {
		Name    string  `json:"name"`
		Lat     float64 `json:"lat"`
		Lon     float64 `json:"lon"`
		Country string  `json:"country"`
		State   string  `json:"state"`
	}

	geocode := func(ctx context.Context, location string, limit int) ([]geoResult, error) {
		if apiKey == "" {
			return nil, fmt.Errorf("OPENWEATHER_API_KEY not set")
		}
		u := fmt.Sprintf("https://api.openweathermap.org/geo/1.0/direct?q=%s&limit=%d&appid=%s",
			url.QueryEscape(location), limit, apiKey)
		var results []geoResult
		if err := fetchJSON(ctx, u, &results); err != nil {
			return nil, fmt.Errorf("geocode failed: %w", err)
		}
		if len(results) == 0 {
			return nil, fmt.Errorf("location not found: %s", location)
		}
		return results, nil
	}

	return []agnogo.ToolDef{
		{
			Name: "geocode_location",
			Desc: "Geocode a location name to latitude and longitude coordinates",
			Params: agnogo.Params{
				"location": {Type: "string", Desc: "Location name (e.g. 'London', 'New York, US')", Required: true},
				"limit":    {Type: "string", Desc: "Maximum number of results (default 1)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				location := strings.TrimSpace(args["location"])
				if location == "" {
					return "", fmt.Errorf("missing required parameter: location")
				}
				limit := 1
				if l := strings.TrimSpace(args["limit"]); l != "" {
					if n, err := strconv.Atoi(l); err == nil && n > 0 {
						limit = n
					}
				}
				results, err := geocode(ctx, location, limit)
				if err != nil {
					return "", err
				}
				var out []map[string]interface{}
				for _, r := range results {
					m := map[string]interface{}{
						"name":    r.Name,
						"lat":     r.Lat,
						"lon":     r.Lon,
						"country": r.Country,
					}
					if r.State != "" {
						m["state"] = r.State
					}
					out = append(out, m)
				}
				data, _ := json.Marshal(out)
				return string(data), nil
			},
		},
		{
			Name: "get_current_weather",
			Desc: "Get current weather for a location including temperature, humidity, wind, and conditions",
			Params: agnogo.Params{
				"location": {Type: "string", Desc: "Location name (e.g. 'London', 'New York, US')", Required: true},
				"units":    {Type: "string", Desc: "Units: metric (Celsius), imperial (Fahrenheit), standard (Kelvin). Default: metric"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				location := strings.TrimSpace(args["location"])
				if location == "" {
					return "", fmt.Errorf("missing required parameter: location")
				}
				units := "metric"
				if u := strings.TrimSpace(args["units"]); u != "" {
					units = u
				}

				geo, err := geocode(ctx, location, 1)
				if err != nil {
					return "", err
				}

				u := fmt.Sprintf("https://api.openweathermap.org/data/2.5/weather?lat=%f&lon=%f&units=%s&appid=%s",
					geo[0].Lat, geo[0].Lon, units, apiKey)

				var weather map[string]interface{}
				if err := fetchJSON(ctx, u, &weather); err != nil {
					return "", fmt.Errorf("weather fetch failed: %w", err)
				}

				data, _ := json.Marshal(weather)
				return string(data), nil
			},
		},
		{
			Name: "get_forecast",
			Desc: "Get weather forecast for a location for up to 5 days",
			Params: agnogo.Params{
				"location": {Type: "string", Desc: "Location name (e.g. 'London', 'New York, US')", Required: true},
				"days":     {Type: "string", Desc: "Number of days to forecast (1-5, default 5)"},
				"units":    {Type: "string", Desc: "Units: metric (Celsius), imperial (Fahrenheit), standard (Kelvin). Default: metric"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				location := strings.TrimSpace(args["location"])
				if location == "" {
					return "", fmt.Errorf("missing required parameter: location")
				}
				units := "metric"
				if u := strings.TrimSpace(args["units"]); u != "" {
					units = u
				}
				days := 5
				if d := strings.TrimSpace(args["days"]); d != "" {
					if n, err := strconv.Atoi(d); err == nil && n > 0 && n <= 5 {
						days = n
					}
				}

				geo, err := geocode(ctx, location, 1)
				if err != nil {
					return "", err
				}

				cnt := days * 8 // 3-hour intervals, 8 per day
				u := fmt.Sprintf("https://api.openweathermap.org/data/2.5/forecast?lat=%f&lon=%f&units=%s&cnt=%d&appid=%s",
					geo[0].Lat, geo[0].Lon, units, cnt, apiKey)

				var forecast map[string]interface{}
				if err := fetchJSON(ctx, u, &forecast); err != nil {
					return "", fmt.Errorf("forecast fetch failed: %w", err)
				}

				data, _ := json.Marshal(forecast)
				return string(data), nil
			},
		},
	}
}
