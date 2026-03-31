package contrib

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/saeedalam/agnogo"
)

// YFinance returns tools for fetching stock data from Yahoo Finance.
// Clone of agno's YFinanceTools. No authentication required.
func YFinance() []agnogo.ToolDef {
	client := &http.Client{Timeout: 15 * time.Second}

	fetchYahoo := func(ctx context.Context, u string) ([]byte, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; agnogo/1.0)")
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
		}
		return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	}

	// Helper to safely extract nested values from parsed JSON
	getNestedMap := func(data map[string]interface{}, keys ...string) map[string]interface{} {
		current := data
		for _, key := range keys {
			if v, ok := current[key]; ok {
				if m, ok := v.(map[string]interface{}); ok {
					current = m
				} else {
					return nil
				}
			} else {
				return nil
			}
		}
		return current
	}

	getRawValue := func(m map[string]interface{}, key string) interface{} {
		if m == nil {
			return nil
		}
		if v, ok := m[key]; ok {
			if vm, ok := v.(map[string]interface{}); ok {
				if raw, ok := vm["raw"]; ok {
					return raw
				}
				if fmt, ok := vm["fmt"]; ok {
					return fmt
				}
			}
			return v
		}
		return nil
	}

	return []agnogo.ToolDef{
		{
			Name: "get_stock_price",
			Desc: "Get the current stock price for a given ticker symbol",
			Params: agnogo.Params{
				"symbol": {Type: "string", Desc: "Stock ticker symbol (e.g. AAPL, GOOGL, MSFT)", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				symbol := strings.TrimSpace(strings.ToUpper(args["symbol"]))
				if symbol == "" {
					return "", fmt.Errorf("missing required parameter: symbol")
				}

				u := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?interval=1d&range=1d", symbol)
				data, err := fetchYahoo(ctx, u)
				if err != nil {
					return "", fmt.Errorf("failed to fetch stock price: %w", err)
				}

				var resp map[string]interface{}
				if err := json.Unmarshal(data, &resp); err != nil {
					return "", fmt.Errorf("parse error: %w", err)
				}

				chart := getNestedMap(resp, "chart")
				if chart == nil {
					return "", fmt.Errorf("unexpected response format")
				}
				resultsRaw, ok := chart["result"]
				if !ok {
					return "", fmt.Errorf("no chart results")
				}
				results, ok := resultsRaw.([]interface{})
				if !ok || len(results) == 0 {
					return "", fmt.Errorf("no chart results")
				}
				first, ok := results[0].(map[string]interface{})
				if !ok {
					return "", fmt.Errorf("unexpected result format")
				}
				meta, ok := first["meta"].(map[string]interface{})
				if !ok {
					return "", fmt.Errorf("no meta data")
				}

				result := map[string]interface{}{
					"symbol":              symbol,
					"regular_market_price": meta["regularMarketPrice"],
					"previous_close":      meta["previousClose"],
					"currency":            meta["currency"],
					"exchange":            meta["exchangeName"],
				}
				out, _ := json.Marshal(result)
				return string(out), nil
			},
		},
		{
			Name: "get_company_info",
			Desc: "Get detailed company information including sector, industry, website, and financial summary",
			Params: agnogo.Params{
				"symbol": {Type: "string", Desc: "Stock ticker symbol (e.g. AAPL, GOOGL, MSFT)", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				symbol := strings.TrimSpace(strings.ToUpper(args["symbol"]))
				if symbol == "" {
					return "", fmt.Errorf("missing required parameter: symbol")
				}

				u := fmt.Sprintf("https://query1.finance.yahoo.com/v10/finance/quoteSummary/%s?modules=assetProfile,price,summaryDetail,defaultKeyStatistics,financialData", symbol)
				data, err := fetchYahoo(ctx, u)
				if err != nil {
					return "", fmt.Errorf("failed to fetch company info: %w", err)
				}

				var resp map[string]interface{}
				if err := json.Unmarshal(data, &resp); err != nil {
					return "", fmt.Errorf("parse error: %w", err)
				}

				qs := getNestedMap(resp, "quoteSummary")
				if qs == nil {
					return "", fmt.Errorf("unexpected response format")
				}
				resultsRaw, ok := qs["result"]
				if !ok {
					return "", fmt.Errorf("no results")
				}
				results, ok := resultsRaw.([]interface{})
				if !ok || len(results) == 0 {
					return "", fmt.Errorf("no results")
				}
				first, ok := results[0].(map[string]interface{})
				if !ok {
					return "", fmt.Errorf("unexpected result format")
				}

				profile, _ := first["assetProfile"].(map[string]interface{})
				price, _ := first["price"].(map[string]interface{})
				summary, _ := first["summaryDetail"].(map[string]interface{})
				keyStats, _ := first["defaultKeyStatistics"].(map[string]interface{})
				financial, _ := first["financialData"].(map[string]interface{})

				result := map[string]interface{}{
					"symbol":         symbol,
					"name":           getRawValue(price, "shortName"),
					"price":          getRawValue(price, "regularMarketPrice"),
					"market_cap":     getRawValue(price, "marketCap"),
					"currency":       getRawValue(price, "currency"),
					"sector":         nil,
					"industry":       nil,
					"website":        nil,
					"summary":        nil,
					"eps":            getRawValue(keyStats, "trailingEps"),
					"pe_ratio":       getRawValue(summary, "trailingPE"),
					"52w_high":       getRawValue(summary, "fiftyTwoWeekHigh"),
					"52w_low":        getRawValue(summary, "fiftyTwoWeekLow"),
					"recommendation": getRawValue(financial, "recommendationKey"),
				}
				if profile != nil {
					if v, ok := profile["sector"]; ok {
						result["sector"] = v
					}
					if v, ok := profile["industry"]; ok {
						result["industry"] = v
					}
					if v, ok := profile["website"]; ok {
						result["website"] = v
					}
					if v, ok := profile["longBusinessSummary"]; ok {
						summary := fmt.Sprintf("%v", v)
						if len(summary) > 500 {
							summary = summary[:500] + "..."
						}
						result["summary"] = summary
					}
				}

				out, _ := json.Marshal(result)
				return string(out), nil
			},
		},
		{
			Name: "get_stock_fundamentals",
			Desc: "Get stock fundamental data including PE ratio, EPS, dividend yield, and beta",
			Params: agnogo.Params{
				"symbol": {Type: "string", Desc: "Stock ticker symbol (e.g. AAPL, GOOGL, MSFT)", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				symbol := strings.TrimSpace(strings.ToUpper(args["symbol"]))
				if symbol == "" {
					return "", fmt.Errorf("missing required parameter: symbol")
				}

				u := fmt.Sprintf("https://query1.finance.yahoo.com/v10/finance/quoteSummary/%s?modules=assetProfile,price,summaryDetail,defaultKeyStatistics,financialData", symbol)
				data, err := fetchYahoo(ctx, u)
				if err != nil {
					return "", fmt.Errorf("failed to fetch fundamentals: %w", err)
				}

				var resp map[string]interface{}
				if err := json.Unmarshal(data, &resp); err != nil {
					return "", fmt.Errorf("parse error: %w", err)
				}

				qs := getNestedMap(resp, "quoteSummary")
				if qs == nil {
					return "", fmt.Errorf("unexpected response format")
				}
				resultsRaw, ok := qs["result"]
				if !ok {
					return "", fmt.Errorf("no results")
				}
				results, ok := resultsRaw.([]interface{})
				if !ok || len(results) == 0 {
					return "", fmt.Errorf("no results")
				}
				first, ok := results[0].(map[string]interface{})
				if !ok {
					return "", fmt.Errorf("unexpected result format")
				}

				profile, _ := first["assetProfile"].(map[string]interface{})
				price, _ := first["price"].(map[string]interface{})
				summary, _ := first["summaryDetail"].(map[string]interface{})
				keyStats, _ := first["defaultKeyStatistics"].(map[string]interface{})

				result := map[string]interface{}{
					"symbol":         symbol,
					"company_name":   getRawValue(price, "shortName"),
					"market_cap":     getRawValue(price, "marketCap"),
					"pe_ratio":       getRawValue(summary, "trailingPE"),
					"pb_ratio":       getRawValue(keyStats, "priceToBook"),
					"dividend_yield": getRawValue(summary, "dividendYield"),
					"eps":            getRawValue(keyStats, "trailingEps"),
					"beta":           getRawValue(keyStats, "beta"),
					"52w_high":       getRawValue(summary, "fiftyTwoWeekHigh"),
					"52w_low":        getRawValue(summary, "fiftyTwoWeekLow"),
					"sector":         nil,
					"industry":       nil,
				}
				if profile != nil {
					if v, ok := profile["sector"]; ok {
						result["sector"] = v
					}
					if v, ok := profile["industry"]; ok {
						result["industry"] = v
					}
				}

				out, _ := json.Marshal(result)
				return string(out), nil
			},
		},
	}
}
