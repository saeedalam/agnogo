package agnogo

import (
	"context"
	"encoding/json"
	"net/http"
)

// agentContextKey is the unexported key used to store an *Core in context.
type agentContextKey struct{}

// AgentMiddleware returns HTTP middleware that injects the agent into every
// request's context. Downstream handlers can retrieve it with AgentFromContext.
//
//	mux := http.NewServeMux()
//	mux.Handle("/chat", AgentMiddleware(agent)(chatHandler))
func AgentMiddleware(agent *Core) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), agentContextKey{}, agent)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AgentFromContext retrieves the *Core stored in ctx by AgentMiddleware.
// Returns nil if no agent is present.
func AgentFromContext(ctx context.Context) *Core {
	a, _ := ctx.Value(agentContextKey{}).(*Core)
	return a
}

// agentRequest is the JSON body accepted by AgentHandler.
type agentRequest struct {
	Message   string `json:"message"`
	SessionID string `json:"session_id,omitempty"`
}

// agentResponse is the JSON body returned by AgentHandler.
type agentResponse struct {
	Text        string   `json:"text"`
	ToolsCalled []string `json:"tools_called,omitempty"`
	SessionID   string   `json:"session_id,omitempty"`
}

// AgentHandler creates an http.HandlerFunc that handles POST requests.
// It reads a JSON body with "message" and optional "session_id", runs the agent,
// and returns a JSON response with the agent's reply.
//
//	mux.HandleFunc("POST /chat", agnogo.AgentHandler(agent))
func AgentHandler(agent *Core) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
			return
		}

		var req agentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON: " + err.Error()})
			return
		}
		if req.Message == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "message is required"})
			return
		}

		var (
			resp      *Response
			err       error
			sessionID string
		)

		if req.SessionID != "" && agent.storage != nil {
			sessionID = req.SessionID
			resp, err = agent.RunWithStorage(r.Context(), req.SessionID, req.Message)
		} else {
			sessionID = generateRunID()
			session := NewSession(sessionID)
			resp, err = agent.Run(r.Context(), session, req.Message)
		}

		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(agentResponse{
			Text:        resp.Text,
			ToolsCalled: resp.ToolsCalled,
			SessionID:   sessionID,
		})
	}
}
