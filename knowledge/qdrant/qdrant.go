// Package qdrant provides a Qdrant vector database Knowledge backend for agnogo.
//
//	import "github.com/saeedalam/agnogo/knowledge/qdrant"
//	kb := qdrant.New("http://localhost:6333", "my_collection", embedFn)
package qdrant

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

// EmbedFunc generates a vector embedding.
type EmbedFunc func(ctx context.Context, text string) ([]float32, error)

// Knowledge implements agnogo.Knowledge using Qdrant.
type Knowledge struct {
	baseURL    string
	collection string
	embedFunc  EmbedFunc
	client     *http.Client
}

// New creates a Qdrant knowledge backend.
func New(baseURL, collection string, embedFunc EmbedFunc) *Knowledge {
	return &Knowledge{
		baseURL: strings.TrimRight(baseURL, "/"),
		collection: collection,
		embedFunc: embedFunc,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (k *Knowledge) Search(ctx context.Context, query string, limit int) (string, error) {
	embedding, err := k.embedFunc(ctx, query)
	if err != nil {
		return "", fmt.Errorf("embed: %w", err)
	}

	body, _ := json.Marshal(map[string]any{
		"vector":       embedding,
		"limit":        limit,
		"with_payload": true,
	})

	url := fmt.Sprintf("%s/collections/%s/points/search", k.baseURL, k.collection)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := k.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("qdrant: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)

	var result struct {
		Result []struct {
			Payload map[string]any `json:"payload"`
			Score   float64        `json:"score"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", nil
	}

	var sb strings.Builder
	for _, r := range result.Result {
		if content, ok := r.Payload["content"].(string); ok {
			sb.WriteString(content + "\n")
		}
	}
	return sb.String(), nil
}
