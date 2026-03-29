package agnogo

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
)

const defaultMaxLoops = 8

// Agent is a stateful AI agent that uses tools to accomplish tasks.
//
//	a := agnogo.New(agnogo.Config{
//	    Model:        agnogo.OpenAI(key, "gpt-4.1-mini"),
//	    Instructions: "You are a helpful assistant.",
//	    Knowledge:    myKB,
//	    AutoMemory:   true,
//	    Storage:      myDB,
//	    Trace:        agnogo.DefaultTrace(),
//	})
//	a.Tool("search", "Search the web", params, searchFn)
//	resp, _ := a.Run(ctx, session, "What's the weather?")
type Agent struct {
	model        ModelProvider
	tools        *ToolRegistry
	instructions string
	promptFunc   func(session *Session) string
	knowledge    Knowledge
	knowledgeN   int
	reasoning    *ReasoningConfig
	memory       MemoryExtractor
	storage      Storage
	trace        *Trace
	retry        *RetryConfig
	history      *HistoryConfig
	debug        DebugConfig
	inputGuards  []Guardrail
	outputGuards []Guardrail
	maxLoops     int
	fallbackText string
}

// Config configures a new Agent.
type Config struct {
	Model        ModelProvider
	Instructions string                        // static system prompt
	PromptFunc   func(session *Session) string // dynamic prompt (overrides Instructions)
	Knowledge    Knowledge                     // auto-search for questions (optional)
	Reasoning    *ReasoningConfig              // chain-of-thought before responding (optional)
	KnowledgeN   int                           // max knowledge results (default 3)
	AutoMemory   bool                          // pattern-based memory extraction
	Memory       MemoryExtractor               // custom memory extractor (overrides AutoMemory)
	Storage      Storage                       // auto-save sessions (optional)
	Trace        *Trace                        // observability hooks (optional)
	Retry        *RetryConfig                  // retry failed model calls (optional)
	History      *HistoryConfig                // trim long histories (optional)
	Debug        *DebugConfig                  // debug output (optional)
	MaxLoops     int                           // max tool loops per Run (default 8)
	FallbackText string                        // text when max loops reached
}

// New creates an Agent.
func New(cfg Config) *Agent {
	maxLoops := cfg.MaxLoops
	if maxLoops == 0 {
		maxLoops = defaultMaxLoops
	}
	knowledgeN := cfg.KnowledgeN
	if knowledgeN == 0 {
		knowledgeN = 3
	}
	fallback := cfg.FallbackText
	if fallback == "" {
		fallback = "I couldn't complete your request. Would you like me to try differently?"
	}

	var mem MemoryExtractor
	if cfg.Memory != nil {
		mem = cfg.Memory
	} else if cfg.AutoMemory {
		mem = DefaultPatternMemory()
	}

	var dbg DebugConfig
	if cfg.Debug != nil {
		dbg = *cfg.Debug
	}

	return &Agent{
		model:        cfg.Model,
		tools:        NewToolRegistry(),
		instructions: cfg.Instructions,
		promptFunc:   cfg.PromptFunc,
		knowledge:    cfg.Knowledge,
		knowledgeN:   knowledgeN,
		reasoning:    cfg.Reasoning,
		memory:       mem,
		storage:      cfg.Storage,
		trace:        cfg.Trace,
		retry:        cfg.Retry,
		history:      cfg.History,
		debug:        dbg,
		inputGuards:  nil,
		outputGuards: nil,
		maxLoops:     maxLoops,
		fallbackText: fallback,
	}
}

// Tool registers a tool the agent can use. Chainable.
//
//	a.Tool("book", "Book appointment", agnogo.Params{
//	    "date": {Type: "string", Desc: "Date", Required: true},
//	}, bookFn)
func (a *Agent) Tool(name, desc string, params Params, fn ToolFunc) *Agent {
	a.tools.Add(name, desc, params, fn)
	return a
}

// AddTools registers multiple tools from built-in tool packages.
//
//	a.AddTools(tools.Calculator()...)
//	a.AddTools(tools.WebSearch()...)
func (a *Agent) AddTools(defs ...ToolDef) *Agent {
	for _, d := range defs {
		a.tools.Add(d.Name, d.Desc, d.Params, d.Fn)
	}
	return a
}

// ToolWithApproval registers a tool that requires human approval before execution.
//
//	a.ToolWithApproval("transfer", "Transfer money", params, transferFn, "Amounts over 1000 need approval")
func (a *Agent) ToolWithApproval(name, desc string, params Params, fn ToolFunc, reason string) *Agent {
	t := a.tools.Add(name, desc, params, fn)
	t.RequireApproval = true
	t.ApprovalReason = reason
	return a
}

// InputGuardrail adds a pre-execution check.
func (a *Agent) InputGuardrail(name string, fn func(ctx context.Context, session *Session, msg string) error) *Agent {
	a.inputGuards = append(a.inputGuards, Guardrail{Name: name, Check: fn})
	return a
}

// OutputGuardrail adds a post-execution check.
func (a *Agent) OutputGuardrail(name string, fn func(ctx context.Context, session *Session, msg string) error) *Agent {
	a.outputGuards = append(a.outputGuards, Guardrail{Name: name, Check: fn})
	return a
}

// Tools returns the registry for inspection.
func (a *Agent) Tools() *ToolRegistry { return a.tools }

// Response is the result of Run.
type Response struct {
	Text          string          `json:"text"`
	ToolsCalled   []string        `json:"tools_called,omitempty"`
	NeedsApproval bool            `json:"needs_approval,omitempty"`
	Approval      *HumanApproval  `json:"approval,omitempty"`
	Metrics       *RunMetrics     `json:"metrics,omitempty"`
}

// Run processes one user message. The main method.
//
//	resp, _ := a.Run(ctx, session, "Hello!")
func (a *Agent) Run(ctx context.Context, session *Session, userMessage string) (*Response, error) {
	runStart := time.Now()
	runID := generateRunID()
	metrics := &RunMetrics{RunID: runID}

	// Check env var for debug override
	dbg := a.debug
	if !dbg.Enabled {
		if v := os.Getenv("AGNOGO_DEBUG"); v == "true" || v == "1" {
			dbg = DefaultDebug()
			if os.Getenv("AGNOGO_DEBUG_LEVEL") == "2" {
				dbg.Level = 2
			}
		}
	}

	dbg.printRunStart(runID, session.ID)

	// Input guardrails
	if err := runGuardrails(ctx, a.inputGuards, session, userMessage); err != nil {
		if a.trace != nil && a.trace.OnGuardrail != nil {
			a.trace.OnGuardrail("input", "input", true)
		}
		dbg.printGuardrail("input", "input", true)
		metrics.Duration = time.Since(runStart)
		dbg.printRunEnd(runID, metrics)
		return &Response{Text: err.Error(), Metrics: metrics}, nil
	}

	// Build messages
	systemPrompt := a.instructions
	if a.promptFunc != nil {
		systemPrompt = a.promptFunc(session)
	}

	messages := []Message{{Role: "system", Content: systemPrompt}}
	messages = append(messages, session.History...)
	session.AddMessage("user", userMessage)
	messages = append(messages, Message{Role: "user", Content: userMessage})

	// Reasoning (chain-of-thought before responding)
	if a.reasoning != nil && a.reasoning.Enabled {
		_, reasoningContext := runReasoning(ctx, a.reasoning, a.model, userMessage, session)
		if reasoningContext != "" {
			messages = append(messages, Message{Role: "system", Content: reasoningContext})
		}
	}

	// Knowledge injection
	if a.knowledge != nil {
		start := time.Now()
		messages = injectKnowledge(ctx, a.knowledge, userMessage, messages, a.knowledgeN)
		if a.trace != nil && a.trace.OnKnowledge != nil {
			a.trace.OnKnowledge(userMessage, "", time.Since(start))
		}
	}

	// History trimming
	if a.history != nil {
		before := len(messages)
		messages = trimHistory(messages, *a.history)
		messages = trimToolMessages(messages, a.history.MaxToolMessages)
		dbg.printHistory(before, len(messages))
	}

	dbg.printMessages(messages)

	toolDefs := a.tools.FunctionDefs()
	var toolsCalled []string
	dupes := map[string]int{}

	// Agent loop
	for loop := 0; loop < a.maxLoops; loop++ {
		// Model call with optional retry
		modelStart := time.Now()
		var resp *ModelResponse
		var err error
		if a.retry != nil {
			resp, err = retryModelCall(ctx, *a.retry, func() (*ModelResponse, error) {
				return a.model.ChatCompletion(ctx, messages, toolDefs)
			})
		} else {
			resp, err = a.model.ChatCompletion(ctx, messages, toolDefs)
		}
		modelDur := time.Since(modelStart)

		metrics.ModelCalls++
		if resp != nil {
			metrics.addUsage(resp.Usage)
		}

		dbg.printModelCall(len(messages), lenToolCalls(resp), modelDur)
		if a.trace != nil && a.trace.OnModelCall != nil {
			a.trace.OnModelCall(messages, resp, modelDur)
		}
		if err != nil {
			slog.Error("agnogo: model error", "error", err, "loop", loop)
			metrics.Duration = time.Since(runStart)
			dbg.printRunEnd(runID, metrics)
			return &Response{Text: a.fallbackText, Metrics: metrics}, nil
		}

		// Text response — done
		if len(resp.ToolCalls) == 0 {
			text := resp.Text
			if text == "" {
				text = "..."
			}
			if err := runGuardrails(ctx, a.outputGuards, session, text); err != nil {
				if a.trace != nil && a.trace.OnGuardrail != nil {
					a.trace.OnGuardrail("output", "output", true)
				}
				text = err.Error()
			}
			dbg.printResponse(text)
			session.AddMessage("assistant", text)

			// Memory extraction
			if a.memory != nil {
				a.memory.Extract(ctx, session, userMessage, text)
			}

			// Auto-save
			if a.storage != nil {
				saveErr := a.storage.Save(ctx, session)
				if a.trace != nil && a.trace.OnSessionSave != nil {
					a.trace.OnSessionSave(session, saveErr)
				}
			}

			metrics.ToolCalls = len(toolsCalled)
			metrics.Duration = time.Since(runStart)
			dbg.printRunEnd(runID, metrics)
			return &Response{Text: text, ToolsCalled: toolsCalled, Metrics: metrics}, nil
		}

		// Tool calls
		assistantMsg := Message{Role: "assistant"}
		for _, tc := range resp.ToolCalls {
			assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, tc)
		}
		messages = append(messages, assistantMsg)

		for _, tc := range resp.ToolCalls {
			args := ParseArgs(tc.Arguments)

			// Duplicate detection
			key := tc.Name + ":" + tc.Arguments
			dupes[key]++
			if dupes[key] > 2 {
				result := fmt.Sprintf("ERROR: '%s' called %d times with same args. Try a different approach.", tc.Name, dupes[key])
				messages = append(messages, Message{Role: "tool", Content: result, Name: tc.ID})
				slog.Warn("agnogo: duplicate tool call blocked", "tool", tc.Name)
				continue
			}

			// Human approval check
			tool := a.tools.Get(tc.Name)
			if tool != nil && tool.RequireApproval {
				approval := HumanApproval{
					ToolName:  tc.Name,
					Arguments: args,
					Reason:    tool.ApprovalReason,
					SessionID: session.ID,
				}
				if a.trace != nil && a.trace.OnApproval != nil {
					a.trace.OnApproval(approval)
				}
				dbg.printApproval(tc.Name, tool.ApprovalReason)
				// Save state for resume
				session.Set("_pending_tool", tc.Name)
				session.Set("_pending_args", tc.Arguments)
				session.Set("_pending_call_id", tc.ID)
				if a.storage != nil {
					a.storage.Save(ctx, session)
				}
				metrics.ToolCalls = len(toolsCalled)
				metrics.Duration = time.Since(runStart)
				dbg.printRunEnd(runID, metrics)
				return &Response{
					Text:          fmt.Sprintf("This action requires approval: %s", tool.ApprovalReason),
					ToolsCalled:   toolsCalled,
					NeedsApproval: true,
					Approval:      &approval,
					Metrics:       metrics,
				}, nil
			}

			// Execute tool
			toolStart := time.Now()
			result, err := a.tools.Invoke(ctx, tc.Name, args)
			toolDur := time.Since(toolStart)

			if err != nil {
				result = fmt.Sprintf("Tool '%s' failed: %s. Try a different approach.", tc.Name, err.Error())
			}

			dbg.printToolCall(tc.Name, args, result, toolDur, err)
			if a.trace != nil && a.trace.OnToolCall != nil {
				a.trace.OnToolCall(tc.Name, args, result, toolDur, err)
			}

			toolsCalled = append(toolsCalled, tc.Name)
			messages = append(messages, Message{Role: "tool", Content: result, Name: tc.ID})
			session.AddToolResult(tc.ID, result)
		}
	}

	// Max loops
	slog.Warn("agnogo: max loops reached", "max", a.maxLoops, "session", session.ID)
	session.AddMessage("assistant", a.fallbackText)
	metrics.ToolCalls = len(toolsCalled)
	metrics.Duration = time.Since(runStart)
	dbg.printRunEnd(runID, metrics)
	return &Response{Text: a.fallbackText, ToolsCalled: toolsCalled, Metrics: metrics}, nil
}

func lenToolCalls(r *ModelResponse) int {
	if r == nil {
		return 0
	}
	return len(r.ToolCalls)
}

func generateRunID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("run_%x", b)
}

// Resume continues after a human approval.
// Call with approved=true to execute the pending tool, or false to skip it.
func (a *Agent) Resume(ctx context.Context, session *Session, approved bool) (*Response, error) {
	toolName := session.GetStr("_pending_tool")
	argsJSON := session.GetStr("_pending_args")
	callID := session.GetStr("_pending_call_id")

	if toolName == "" {
		return &Response{Text: "No pending approval."}, nil
	}

	// Clear pending state
	session.Set("_pending_tool", nil)
	session.Set("_pending_args", nil)
	session.Set("_pending_call_id", nil)

	if !approved {
		session.AddMessage("assistant", "The action was not approved.")
		return &Response{Text: "The action was not approved."}, nil
	}

	// Execute the approved tool
	args := ParseArgs(argsJSON)
	result, err := a.tools.Invoke(ctx, toolName, args)
	if err != nil {
		result = fmt.Sprintf("Tool failed: %s", err.Error())
	}
	session.AddToolResult(callID, result)

	// Continue the conversation with the tool result
	return a.Run(ctx, session, fmt.Sprintf("[Approved: %s executed successfully]", toolName))
}

// RunWithStorage loads session, runs, and saves. Convenience for production use.
func (a *Agent) RunWithStorage(ctx context.Context, sessionID, userMessage string) (*Response, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("storage not configured")
	}

	session, err := a.storage.Load(ctx, sessionID)
	if err != nil {
		session = NewSession(sessionID)
	}

	resp, err := a.Run(ctx, session, userMessage)
	if err != nil {
		return nil, err
	}

	if saveErr := a.storage.Save(ctx, session); saveErr != nil {
		slog.Error("agnogo: save failed", "session", sessionID, "error", saveErr)
	}

	return resp, nil
}

// RunStream streams the response word-by-word. For WebSocket/SSE.
func (a *Agent) RunStream(ctx context.Context, session *Session, userMessage string) <-chan StreamChunk {
	ch := make(chan StreamChunk, 20)
	go func() {
		defer close(ch)
		resp, err := a.Run(ctx, session, userMessage)
		if err != nil {
			ch <- StreamChunk{Error: err, Done: true}
			return
		}
		for i, word := range strings.Fields(resp.Text) {
			if i > 0 {
				ch <- StreamChunk{Text: " "}
			}
			ch <- StreamChunk{Text: word}
		}
		ch <- StreamChunk{Done: true}
	}()
	return ch
}

// StreamChunk is one piece of a streaming response.
type StreamChunk struct {
	Text  string
	Done  bool
	Error error
}

// ── Agno-equivalent Agent methods ────────────────────────

// SetTools replaces all tools. Agno: agent.set_tools()
func (a *Agent) SetTools(defs ...ToolDef) *Agent {
	a.tools = NewToolRegistry()
	return a.AddTools(defs...)
}

// ClearTools removes all tools.
func (a *Agent) ClearTools() *Agent {
	a.tools = NewToolRegistry()
	return a
}

// GetSession loads a session from storage. Agno: agent.get_session()
func (a *Agent) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("storage not configured")
	}
	return a.storage.Load(ctx, sessionID)
}

// SaveSession persists a session. Agno: agent.save_session()
func (a *Agent) SaveSession(ctx context.Context, session *Session) error {
	if a.storage == nil {
		return fmt.Errorf("storage not configured")
	}
	return a.storage.Save(ctx, session)
}

// DeleteSession removes a session. Agno: agent.delete_session()
func (a *Agent) DeleteSession(ctx context.Context, sessionID string) error {
	if a.storage == nil {
		return fmt.Errorf("storage not configured")
	}
	return a.storage.Delete(ctx, sessionID)
}

// ListSessions returns recent sessions. Agno: agent.get_sessions()
func (a *Agent) ListSessions(ctx context.Context, limit int) ([]*Session, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("storage not configured")
	}
	return a.storage.List(ctx, limit)
}

// AddKnowledge adds content to the knowledge store. Agno: agent.add_to_knowledge()
func (a *Agent) AddKnowledge(ctx context.Context, key, content string) error {
	if ks, ok := a.storage.(KnowledgeStore); ok {
		return ks.AddKnowledge(ctx, key, content)
	}
	return fmt.Errorf("storage does not support knowledge management")
}

// GetChatHistory returns conversation messages. Agno: agent.get_chat_history()
func (a *Agent) GetChatHistory(ctx context.Context, sessionID string) ([]Message, error) {
	session, err := a.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return session.History, nil
}

// GetMemories returns learned facts. Agno: agent.get_user_memories()
func (a *Agent) GetMemories(ctx context.Context, sessionID string) (map[string]string, error) {
	session, err := a.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return session.Memory, nil
}

// PrintResponse runs the agent and prints the response. Agno: agent.print_response()
func (a *Agent) PrintResponse(ctx context.Context, session *Session, message string) {
	resp, err := a.Run(ctx, session, message)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Println(resp.Text)
}
