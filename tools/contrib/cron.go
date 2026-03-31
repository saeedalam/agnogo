package contrib

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/saeedalam/agnogo"
)

// CronTool returns a tool for parsing cron expressions and calculating next run times.
func CronTool() []agnogo.ToolDef {
	return []agnogo.ToolDef{
		{
			Name: "cron_next", Desc: "Calculate next N run times for a cron expression",
			Params: agnogo.Params{
				"expression": {Type: "string", Desc: "Standard 5-field cron expression (min hour dom month dow)", Required: true},
				"count":      {Type: "string", Desc: "Number of next run times to return (default 5)"},
				"from":       {Type: "string", Desc: "Start time in RFC3339 (default: now)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				expr := args["expression"]
				if expr == "" {
					return "", fmt.Errorf("expression is required")
				}
				count := 5
				if args["count"] != "" {
					if c, err := strconv.Atoi(args["count"]); err == nil && c > 0 && c <= 100 {
						count = c
					}
				}
				from := time.Now().UTC()
				if args["from"] != "" {
					var err error
					from, err = time.Parse(time.RFC3339, args["from"])
					if err != nil {
						return "", fmt.Errorf("invalid from time: %w", err)
					}
				}

				cron, err := parseCron(expr)
				if err != nil {
					return "", fmt.Errorf("invalid cron expression: %w", err)
				}

				var times []string
				t := from.Truncate(time.Minute).Add(time.Minute)
				maxIter := 525960 // ~1 year in minutes
				for len(times) < count && maxIter > 0 {
					if cron.matches(t) {
						times = append(times, t.Format(time.RFC3339))
					}
					t = t.Add(time.Minute)
					maxIter--
				}

				result := map[string]any{
					"expression": expr,
					"next_times": times,
					"count":      len(times),
				}
				out, _ := json.Marshal(result)
				return string(out), nil
			},
		},
	}
}

type cronExpr struct {
	minutes  []int
	hours    []int
	doms     []int
	months   []int
	weekdays []int
	domWild  bool
	dowWild  bool
}

func (c *cronExpr) matches(t time.Time) bool {
	if !cronContains(c.minutes, t.Minute()) {
		return false
	}
	if !cronContains(c.hours, t.Hour()) {
		return false
	}
	if !cronContains(c.months, int(t.Month())) {
		return false
	}

	domMatch := cronContains(c.doms, t.Day())
	dowMatch := cronContains(c.weekdays, int(t.Weekday()))

	if c.domWild && c.dowWild {
		return true
	}
	if c.domWild {
		return dowMatch
	}
	if c.dowWild {
		return domMatch
	}
	// Both restricted: OR them (POSIX behavior)
	return domMatch || dowMatch
}

func cronContains(vals []int, v int) bool {
	for _, x := range vals {
		if x == v {
			return true
		}
	}
	return false
}

func parseCron(expr string) (*cronExpr, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return nil, fmt.Errorf("expected 5 fields, got %d", len(fields))
	}
	minutes, err := parseCronField(fields[0], 0, 59)
	if err != nil {
		return nil, fmt.Errorf("minute field: %w", err)
	}
	hours, err := parseCronField(fields[1], 0, 23)
	if err != nil {
		return nil, fmt.Errorf("hour field: %w", err)
	}
	doms, err := parseCronField(fields[2], 1, 31)
	if err != nil {
		return nil, fmt.Errorf("day-of-month field: %w", err)
	}
	months, err := parseCronField(fields[3], 1, 12)
	if err != nil {
		return nil, fmt.Errorf("month field: %w", err)
	}
	weekdays, err := parseCronField(fields[4], 0, 6)
	if err != nil {
		return nil, fmt.Errorf("weekday field: %w", err)
	}
	return &cronExpr{
		minutes: minutes, hours: hours, doms: doms, months: months, weekdays: weekdays,
		domWild: fields[2] == "*", dowWild: fields[4] == "*",
	}, nil
}

func parseCronField(field string, min, max int) ([]int, error) {
	var result []int
	parts := strings.Split(field, ",")
	for _, part := range parts {
		vals, err := parseCronPart(part, min, max)
		if err != nil {
			return nil, err
		}
		result = append(result, vals...)
	}
	return result, nil
}

func parseCronPart(part string, min, max int) ([]int, error) {
	// Handle step: */5 or 1-10/2
	step := 1
	if idx := strings.Index(part, "/"); idx >= 0 {
		var err error
		step, err = strconv.Atoi(part[idx+1:])
		if err != nil || step < 1 {
			return nil, fmt.Errorf("invalid step: %s", part)
		}
		part = part[:idx]
	}

	var start, end int
	if part == "*" {
		start, end = min, max
	} else if idx := strings.Index(part, "-"); idx >= 0 {
		var err error
		start, err = strconv.Atoi(part[:idx])
		if err != nil {
			return nil, fmt.Errorf("invalid range start: %s", part)
		}
		end, err = strconv.Atoi(part[idx+1:])
		if err != nil {
			return nil, fmt.Errorf("invalid range end: %s", part)
		}
	} else {
		val, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid value: %s", part)
		}
		if val < min || val > max {
			return nil, fmt.Errorf("value %d out of range [%d, %d]", val, min, max)
		}
		return []int{val}, nil
	}

	if start < min || end > max || start > end {
		return nil, fmt.Errorf("range %d-%d out of bounds [%d, %d]", start, end, min, max)
	}

	var result []int
	for i := start; i <= end; i += step {
		result = append(result, i)
	}
	return result, nil
}
