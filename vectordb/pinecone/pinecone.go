// Package pinecone provides a Pinecone vector database Knowledge backend for agnogo.
//
//	import "github.com/saeedalam/agnogo/vectordb/pinecone"
//	kb := pinecone.New("https://xxx.svc.pinecone.io", "api-key", embedFn)
package pinecone

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// EmbedFunc generates a vector embedding for text.
type EmbedFunc func(ctx context.Context, text string) ([]float32, error)

// Knowledge implements agnogo.Knowledge using Pinecone.
type Knowledge struct {
	host      string
	apiKey    string
	namespace string
	embedFunc EmbedFunc
	client    *http.Client
}

// New creates a Pinecone knowledge backend.
func New(host, apiKey string, embedFunc EmbedFunc, namespace ...string) *Knowledge {
	ns := ""
	if len(namespace) > 0 {
		ns = namespace[0]
	}
	return &Knowledge{
		host:      strings.TrimRight(host, "/"),
		apiKey:    apiKey,
		namespace: ns,
		embedFunc: embedFunc,
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (k *Knowledge) Search(ctx context.Context, query string, limit int) (string, error) {
	embedding, err := k.embedFunc(ctx, query)
	if err != nil {
		return "", fmt.Errorf("embed: %w", err)
	}

	body := map[string]any{
		"vector":          embedding,
		"topK":            limit,
		"includeMetadata": true,
	}
	if k.namespace != "" {
		body["namespace"] = k.namespace
	}

	bodyJSON, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, "POST", k.host+"/query", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Api-Key", k.apiKey)

	resp, err := k.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("pinecone: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)

	var result struct {
		Matches []struct {
			ID       string         `json:"id"`
			Score    float64        `json:"score"`
			Metadata map[string]any `json:"metadata"`
		} `json:"matches"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", nil
	}

	var sb strings.Builder
	for _, m := range result.Matches {
		if content, ok := m.Metadata["content"].(string); ok {
			sb.WriteString(content + "\n")
		} else if text, ok := m.Metadata["text"].(string); ok {
			sb.WriteString(text + "\n")
		}
	}
	return sb.String(), nil
}
