package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net"

	"github.com/saeedalam/agnogo"
)

// DNS returns tools for DNS lookups.
func DNS() []agnogo.ToolDef {
	return []agnogo.ToolDef{
		{
			Name: "dns_lookup", Desc: "Lookup IP addresses for a hostname",
			Params: agnogo.Params{
				"hostname": {Type: "string", Desc: "Hostname to resolve", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				hostname := args["hostname"]
				if hostname == "" {
					return "", fmt.Errorf("hostname is required")
				}
				addrs, err := net.DefaultResolver.LookupHost(ctx, hostname)
				if err != nil {
					return "", fmt.Errorf("DNS lookup failed: %w", err)
				}
				result := map[string]any{"hostname": hostname, "addresses": addrs}
				out, _ := json.Marshal(result)
				return string(out), nil
			},
		},
		{
			Name: "dns_mx", Desc: "Lookup MX records for a domain",
			Params: agnogo.Params{
				"domain": {Type: "string", Desc: "Domain to lookup MX records for", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				domain := args["domain"]
				if domain == "" {
					return "", fmt.Errorf("domain is required")
				}
				mxs, err := net.DefaultResolver.LookupMX(ctx, domain)
				if err != nil {
					return "", fmt.Errorf("MX lookup failed: %w", err)
				}
				var records []map[string]any
				for _, mx := range mxs {
					records = append(records, map[string]any{
						"host": mx.Host,
						"pref": mx.Pref,
					})
				}
				if records == nil {
					records = []map[string]any{}
				}
				result := map[string]any{"domain": domain, "mx_records": records}
				out, _ := json.Marshal(result)
				return string(out), nil
			},
		},
		{
			Name: "dns_txt", Desc: "Lookup TXT records for a domain",
			Params: agnogo.Params{
				"domain": {Type: "string", Desc: "Domain to lookup TXT records for", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				domain := args["domain"]
				if domain == "" {
					return "", fmt.Errorf("domain is required")
				}
				txts, err := net.DefaultResolver.LookupTXT(ctx, domain)
				if err != nil {
					return "", fmt.Errorf("TXT lookup failed: %w", err)
				}
				result := map[string]any{"domain": domain, "txt_records": txts}
				out, _ := json.Marshal(result)
				return string(out), nil
			},
		},
	}
}
