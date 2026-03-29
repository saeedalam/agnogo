// Package chromadb provides a ChromaDB Knowledge backend for agnogo.
//
//	import "github.com/saeedalam/agnogo/knowledge/chromadb"
//	kb := chromadb.New("http://localhost:8000", "my_collection")
package chromadb

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

// Knowledge implements agnogo.Knowledge using ChromaDB.
type Knowledge struct {
	baseURL    string
	collection string
	client     *http.Client
}

// New creates a ChromaDB knowledge backend.
// ChromaDB handles embedding internally if configured with an embedding function.
func New(baseURL, collection string) *Knowledge {
	return &Knowledge{
		baseURL:    strings.TrimRight(baseURL, "/"),
		collection: collection,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (k *Knowledge) Search(ctx context.Context, query string, limit int) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"query_texts": []string{query},
		"n_results":   limit,
	})

	url := fmt.Sprintf("%s/api/v1/collections/%s/query", k.baseURL, k.collection)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := k.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("chromadb: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)

	var result struct {
		Documents [][]string `json:"documents"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", nil
	}

	var sb strings.Builder
	for _, docs := range result.Documents {
		for _, doc := range docs {
			sb.WriteString(doc + "\n")
		}
	}
	return sb.String(), nil
}
