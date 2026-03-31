package agnogo

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// ServeOption configures the HTTP server created by Agent.Serve.
type ServeOption func(*serverConfig)

type serverConfig struct {
	corsOrigins    []string
	authToken      string
	middleware     []func(http.Handler) http.Handler
	readTimeout    time.Duration
	writeTimeout   time.Duration
	maxConcurrent  int
	maxBodySize    int64
}

// WithCORS enables CORS for the given origins (e.g. "*", "https://example.com").
func WithCORS(origins ...string) ServeOption {
	return func(c *serverConfig) {
		c.corsOrigins = origins
	}
}

// WithAuth requires a Bearer token on every request.
func WithAuth(token string) ServeOption {
	return func(c *serverConfig) {
		c.authToken = token
	}
}

// WithMiddleware adds an http.Handler middleware that wraps all routes.
func WithMiddleware(mw func(http.Handler) http.Handler) ServeOption {
	return func(c *serverConfig) {
		c.middleware = append(c.middleware, mw)
	}
}

// WithTimeouts sets read and write timeouts for the HTTP server.
func WithTimeouts(read, write time.Duration) ServeOption {
	return func(c *serverConfig) {
		c.readTimeout = read
		c.writeTimeout = write
	}
}

// WithMaxConcurrent limits the number of concurrent requests the server will
// handle. When at capacity, new requests receive 503 Service Unavailable.
func WithMaxConcurrent(n int) ServeOption {
	return func(c *serverConfig) {
		c.maxConcurrent = n
	}
}

// WithMaxBodySize limits the maximum request body size in bytes.
// If not set, the default is 1 MB (1<<20).
func WithMaxBodySize(bytes int64) ServeOption {
	return func(c *serverConfig) {
		c.maxBodySize = bytes
	}
}

// askRequest is the JSON body for /ask and /stream endpoints.
type askRequest struct {
	Message   string `json:"message"`
	SessionID string `json:"session_id,omitempty"`
}

// askResponse is the JSON body returned by /ask.
type askResponse struct {
	Text        string      `json:"text"`
	ToolsCalled []string    `json:"tools_called,omitempty"`
	Metrics     *RunMetrics `json:"metrics,omitempty"`
}

// healthResponse is the JSON body returned by /health.
type healthResponse struct {
	Status     string `json:"status"`
	Tools      int    `json:"tools"`
	ActiveRuns int    `json:"active_runs"`
}

// toolInfo is one entry in the /tools response.
type toolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Handler returns an http.Handler for embedding in existing servers.
//
//	mux := http.NewServeMux()
//	mux.Handle("/agent/", http.StripPrefix("/agent", agent.Handler()))
func (a *Core) Handler(opts ...ServeOption) http.Handler {
	cfg := &serverConfig{}
	for _, o := range opts {
		o(cfg)
	}

	// Default max body size: 1 MB.
	bodyLimit := cfg.maxBodySize
	if bodyLimit == 0 {
		bodyLimit = 1 << 20
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /ask", a.handleAskWithLimit(bodyLimit))
	mux.HandleFunc("POST /stream", a.handleStreamWithLimit(bodyLimit))
	mux.HandleFunc("GET /health", a.handleHealth)
	mux.HandleFunc("GET /tools", a.handleTools)

	var handler http.Handler = mux

	// Apply concurrency limiter (semaphore) middleware.
	if cfg.maxConcurrent > 0 {
		handler = concurrencyMiddleware(cfg.maxConcurrent, handler)
	}

	// Apply CORS middleware.
	if len(cfg.corsOrigins) > 0 {
		handler = corsMiddleware(cfg.corsOrigins, handler)
	}

	// Apply auth middleware.
	if cfg.authToken != "" {
		handler = authMiddleware(cfg.authToken, handler)
	}

	// Apply user-supplied middleware (outermost wraps first).
	for i := len(cfg.middleware) - 1; i >= 0; i-- {
		handler = cfg.middleware[i](handler)
	}

	return handler
}

// Serve starts an HTTP server on addr with the agent's endpoints.
// It blocks until the server returns an error.
//
//	err := agent.Serve(":8080", agnogo.WithCORS("*"), agnogo.WithAuth("secret"))
func (a *Core) Serve(addr string, opts ...ServeOption) error {
	cfg := &serverConfig{}
	for _, o := range opts {
		o(cfg)
	}

	handler := a.Handler(opts...)

	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  cfg.readTimeout,
		WriteTimeout: cfg.writeTimeout,
	}

	slog.Info("agnogo: serving", "addr", addr, "tools", a.tools.Count())
	return srv.ListenAndServe()
}

// handleAskWithLimit returns an /ask handler that enforces a body size limit.
func (a *Core) handleAskWithLimit(maxBody int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		a.handleAsk(w, r)
	}
}

// handleStreamWithLimit returns a /stream handler that enforces a body size limit.
func (a *Core) handleStreamWithLimit(maxBody int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		a.handleStream(w, r)
	}
}

// concurrencyMiddleware limits concurrent requests using a buffered channel as semaphore.
// When at capacity, new requests receive 503 Service Unavailable.
func concurrencyMiddleware(n int, next http.Handler) http.Handler {
	sem := make(chan struct{}, n)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
			next.ServeHTTP(w, r)
		default:
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "server at capacity"})
		}
	})
}

// handleAsk handles POST /ask.
func (a *Core) handleAsk(w http.ResponseWriter, r *http.Request) {
	var req askRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	if req.Message == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "message is required"})
		return
	}

	var (
		resp *Response
		err  error
	)

	if req.SessionID != "" && a.storage != nil {
		resp, err = a.RunWithStorage(r.Context(), req.SessionID, req.Message)
	} else {
		session := NewSession(generateRunID())
		resp, err = a.Run(r.Context(), session, req.Message)
	}

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, askResponse{
		Text:        resp.Text,
		ToolsCalled: resp.ToolsCalled,
		Metrics:     resp.Metrics,
	})
}

// handleStream handles POST /stream with SSE.
func (a *Core) handleStream(w http.ResponseWriter, r *http.Request) {
	var req askRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	if req.Message == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "message is required"})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	rc := http.NewResponseController(w)

	// Determine session.
	var session *Session
	if req.SessionID != "" && a.storage != nil {
		session = NewSession(req.SessionID)
		// Load existing session if storage is available; ignore errors (fresh session).
		if loaded, err := a.storage.Load(r.Context(), req.SessionID); err == nil && loaded != nil {
			session = loaded
		}
	} else {
		session = NewSession(generateRunID())
	}

	ch := a.RunStreamReal(r.Context(), session, req.Message)

	for chunk := range ch {
		if chunk.Error != nil {
			writeSSE(w, rc, map[string]any{"error": chunk.Error.Error()})
			return
		}
		if chunk.Done {
			writeSSE(w, rc, map[string]any{"done": true})
			return
		}
		writeSSE(w, rc, map[string]any{"text": chunk.Text})
	}

	// If the channel closed without a Done signal, send one.
	writeSSE(w, rc, map[string]any{"done": true})
}

// handleHealth handles GET /health.
func (a *Core) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		Status:     "ok",
		Tools:      a.tools.Count(),
		ActiveRuns: ActiveRunCount(),
	})
}

// handleTools handles GET /tools.
func (a *Core) handleTools(w http.ResponseWriter, _ *http.Request) {
	list := a.tools.List()
	out := make([]toolInfo, len(list))
	for i, t := range list {
		out[i] = toolInfo{Name: t.Name, Description: t.Description}
	}
	writeJSON(w, http.StatusOK, out)
}

// writeJSON encodes v as JSON and writes it to w with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeSSE writes one SSE data event and flushes.
func writeSSE(w http.ResponseWriter, rc *http.ResponseController, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	w.Write([]byte("data: "))
	w.Write(data)
	w.Write([]byte("\n\n"))
	rc.Flush()
}

// corsMiddleware returns a handler that sets CORS headers and handles preflight.
func corsMiddleware(origins []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := ""
		if len(origins) == 1 && origins[0] == "*" {
			origin = "*"
		} else {
			reqOrigin := r.Header.Get("Origin")
			for _, o := range origins {
				if o == reqOrigin {
					origin = reqOrigin
					break
				}
			}
			w.Header().Set("Vary", "Origin")
		}

		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// authMiddleware returns a handler that checks for a valid Bearer token.
func authMiddleware(token string, next http.Handler) http.Handler {
	expected := "Bearer " + token
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != expected {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

