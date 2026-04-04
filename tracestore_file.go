package agnogo

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ── File-based Trace Store ───────────────────────────────────────────
//
// Persists traces as JSON files in a directory. Survives restarts.
// Good for development, testing, and small-scale production.
// Zero external dependencies — uses only os + encoding/json.
//
// Usage:
//
//	store, _ := agnogo.NewFileTraceStore("./traces")
//	sc := agnogo.NewSpanCollector().WithTraceStore(store)

// FileTraceStore persists traces as individual JSON files in a directory.
type FileTraceStore struct {
	dir string
}

// NewFileTraceStore creates a file-based trace store.
// Creates the directory if it doesn't exist.
func NewFileTraceStore(dir string) (*FileTraceStore, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("agnogo: create trace dir %q: %w", dir, err)
	}
	return &FileTraceStore{dir: dir}, nil
}

func (s *FileTraceStore) tracePath(runID string) string {
	// Sanitize runID to prevent path traversal
	safe := strings.ReplaceAll(runID, "/", "_")
	safe = strings.ReplaceAll(safe, "..", "_")
	return filepath.Join(s.dir, safe+".json")
}

func (s *FileTraceStore) SaveTrace(_ context.Context, trace *RunTrace) error {
	if trace == nil || trace.RunID == "" {
		return fmt.Errorf("agnogo: trace has no RunID")
	}
	data, err := json.MarshalIndent(trace, "", "  ")
	if err != nil {
		return fmt.Errorf("agnogo: marshal trace: %w", err)
	}
	path := s.tracePath(trace.RunID)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("agnogo: write trace file: %w", err)
	}
	return nil
}

func (s *FileTraceStore) LoadTrace(_ context.Context, runID string) (*RunTrace, error) {
	data, err := os.ReadFile(s.tracePath(runID))
	if err != nil {
		return nil, fmt.Errorf("agnogo: read trace %q: %w", runID, err)
	}
	var trace RunTrace
	if err := json.Unmarshal(data, &trace); err != nil {
		return nil, fmt.Errorf("agnogo: parse trace %q: %w", runID, err)
	}
	return &trace, nil
}

func (s *FileTraceStore) DeleteTrace(_ context.Context, runID string) error {
	return os.Remove(s.tracePath(runID))
}

func (s *FileTraceStore) QueryTraces(_ context.Context, q TraceQuery) ([]*RunTrace, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("agnogo: read trace dir: %w", err)
	}

	var results []*RunTrace
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, entry.Name()))
		if err != nil {
			continue
		}
		var trace RunTrace
		if err := json.Unmarshal(data, &trace); err != nil {
			continue
		}
		if !matchesQuery(&trace, q) {
			continue
		}
		results = append(results, &trace)
		if q.Limit > 0 && len(results) >= q.Limit {
			break
		}
	}
	return results, nil
}
