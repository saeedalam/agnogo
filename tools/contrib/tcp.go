package contrib

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/saeedalam/agnogo"
)

const maxPortsPerScan = 100

// TCP returns tools for TCP connectivity checks.
func TCP() []agnogo.ToolDef {
	return []agnogo.ToolDef{
		{
			Name: "tcp_check", Desc: "Check if a TCP port is open on a host",
			Params: agnogo.Params{
				"host":    {Type: "string", Desc: "Hostname or IP", Required: true},
				"port":    {Type: "string", Desc: "Port number", Required: true},
				"timeout": {Type: "string", Desc: "Timeout in seconds (default 5)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				host := args["host"]
				port := args["port"]
				if host == "" || port == "" {
					return "", fmt.Errorf("host and port are required")
				}
				timeout := 5 * time.Second
				if args["timeout"] != "" {
					if secs, err := strconv.Atoi(args["timeout"]); err == nil && secs > 0 {
						timeout = time.Duration(secs) * time.Second
					}
				}
				addr := net.JoinHostPort(host, port)
				conn, err := net.DialTimeout("tcp", addr, timeout)
				status := "open"
				if err != nil {
					status = "closed"
				} else {
					conn.Close()
				}
				result := map[string]any{"host": host, "port": port, "status": status}
				out, _ := json.Marshal(result)
				return string(out), nil
			},
		},
		{
			Name: "tcp_scan", Desc: "Scan multiple TCP ports on a host (max 100 ports)",
			Params: agnogo.Params{
				"host":    {Type: "string", Desc: "Hostname or IP", Required: true},
				"ports":   {Type: "string", Desc: "Comma-separated ports or range (e.g. 80,443 or 80-100)", Required: true},
				"timeout": {Type: "string", Desc: "Timeout per port in seconds (default 2)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				host := args["host"]
				portsStr := args["ports"]
				if host == "" || portsStr == "" {
					return "", fmt.Errorf("host and ports are required")
				}
				timeout := 2 * time.Second
				if args["timeout"] != "" {
					if secs, err := strconv.Atoi(args["timeout"]); err == nil && secs > 0 {
						timeout = time.Duration(secs) * time.Second
					}
				}

				ports, err := parsePorts(portsStr)
				if err != nil {
					return "", err
				}
				if len(ports) > maxPortsPerScan {
					return "", fmt.Errorf("max %d ports per scan, got %d", maxPortsPerScan, len(ports))
				}

				var results []map[string]any
				for _, p := range ports {
					portStr := strconv.Itoa(p)
					addr := net.JoinHostPort(host, portStr)
					conn, err := net.DialTimeout("tcp", addr, timeout)
					status := "open"
					if err != nil {
						status = "closed"
					} else {
						conn.Close()
					}
					results = append(results, map[string]any{"port": p, "status": status})
				}

				result := map[string]any{"host": host, "results": results, "scanned": len(ports)}
				out, _ := json.Marshal(result)
				return string(out), nil
			},
		},
	}
}

func parsePorts(s string) ([]int, error) {
	var ports []int
	parts := strings.Split(s, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			rangeParts := strings.SplitN(part, "-", 2)
			start, err := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid port: %s", rangeParts[0])
			}
			end, err := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
			if err != nil {
				return nil, fmt.Errorf("invalid port: %s", rangeParts[1])
			}
			if start < 1 || end > 65535 || start > end {
				return nil, fmt.Errorf("invalid port range: %d-%d", start, end)
			}
			for p := start; p <= end; p++ {
				ports = append(ports, p)
			}
		} else {
			p, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid port: %s", part)
			}
			if p < 1 || p > 65535 {
				return nil, fmt.Errorf("port out of range: %d", p)
			}
			ports = append(ports, p)
		}
	}
	return ports, nil
}
