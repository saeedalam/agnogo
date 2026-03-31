package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/saeedalam/agnogo"
)

// TimeTool returns tools for time operations.
func TimeTool() []agnogo.ToolDef {
	return []agnogo.ToolDef{
		{
			Name: "get_time", Desc: "Get the current time in a given timezone",
			Params: agnogo.Params{
				"timezone": {Type: "string", Desc: "IANA timezone (e.g. America/New_York). Default: UTC"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				tz := args["timezone"]
				if tz == "" {
					tz = "UTC"
				}
				loc, err := time.LoadLocation(tz)
				if err != nil {
					return "", fmt.Errorf("invalid timezone %q: %w", tz, err)
				}
				now := time.Now().In(loc)
				result := map[string]string{
					"time":     now.Format(time.RFC3339),
					"timezone": tz,
					"unix":     fmt.Sprintf("%d", now.Unix()),
				}
				out, _ := json.Marshal(result)
				return string(out), nil
			},
		},
		{
			Name: "convert_timezone", Desc: "Convert a time from one timezone to another",
			Params: agnogo.Params{
				"time":    {Type: "string", Desc: "Time in RFC3339 format (e.g. 2024-01-15T10:30:00Z)", Required: true},
				"from_tz": {Type: "string", Desc: "Source IANA timezone", Required: true},
				"to_tz":   {Type: "string", Desc: "Target IANA timezone", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				timeStr := args["time"]
				fromTZ := args["from_tz"]
				toTZ := args["to_tz"]
				if timeStr == "" || fromTZ == "" || toTZ == "" {
					return "", fmt.Errorf("time, from_tz, and to_tz are required")
				}
				fromLoc, err := time.LoadLocation(fromTZ)
				if err != nil {
					return "", fmt.Errorf("invalid from_tz %q: %w", fromTZ, err)
				}
				toLoc, err := time.LoadLocation(toTZ)
				if err != nil {
					return "", fmt.Errorf("invalid to_tz %q: %w", toTZ, err)
				}
				t, err := time.ParseInLocation(time.RFC3339, timeStr, fromLoc)
				if err != nil {
					// Try without timezone info
					t, err = time.ParseInLocation("2006-01-02T15:04:05", timeStr, fromLoc)
					if err != nil {
						return "", fmt.Errorf("invalid time format: %w", err)
					}
				}
				converted := t.In(toLoc)
				result := map[string]string{
					"original":  t.Format(time.RFC3339),
					"converted": converted.Format(time.RFC3339),
					"from_tz":   fromTZ,
					"to_tz":     toTZ,
				}
				out, _ := json.Marshal(result)
				return string(out), nil
			},
		},
		{
			Name: "date_math", Desc: "Add or subtract a duration from a date",
			Params: agnogo.Params{
				"date": {Type: "string", Desc: "Date in RFC3339 format (e.g. 2024-01-15T10:30:00Z)", Required: true},
				"add":  {Type: "string", Desc: "Duration to add (e.g. 2h30m, 24h, -48h, 720h for 30 days)", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				dateStr := args["date"]
				addStr := args["add"]
				if dateStr == "" || addStr == "" {
					return "", fmt.Errorf("date and add are required")
				}
				t, err := time.Parse(time.RFC3339, dateStr)
				if err != nil {
					return "", fmt.Errorf("invalid date format: %w", err)
				}
				dur, err := time.ParseDuration(addStr)
				if err != nil {
					return "", fmt.Errorf("invalid duration: %w", err)
				}
				result := t.Add(dur)
				out := map[string]string{
					"original": t.Format(time.RFC3339),
					"added":    addStr,
					"result":   result.Format(time.RFC3339),
				}
				r, _ := json.Marshal(out)
				return string(r), nil
			},
		},
	}
}
