package agnogo

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// EventType identifies a category of agent event.
type EventType string

const (
	EventModelCall EventType = "model.call"
	EventModelDone EventType = "model.done"
	EventToolCall  EventType = "tool.call"
	EventToolDone  EventType = "tool.done"
	EventToolError EventType = "tool.error"
	EventGuardrail EventType = "guardrail"
	EventRunStart  EventType = "run.start"
	EventRunEnd    EventType = "run.end"
	EventRetry     EventType = "retry"
	EventMemory    EventType = "memory"
	EventKnowledge EventType = "knowledge"
)

// Event is a single occurrence emitted during an agent run.
type Event struct {
	Type      EventType
	Timestamp time.Time
	Data      map[string]any
}

// EventHandler is a callback for a specific event type.
type EventHandler func(Event)

// EventBus provides publish/subscribe for agent events. Multiple subscribers
// can listen to the same event type.
//
//	bus := agnogo.NewEventBus()
//	bus.On(agnogo.EventModelCall, func(e agnogo.Event) {
//	    fmt.Printf("Model called: %s\n", e.Data["duration"])
//	})
//	bus.On(agnogo.EventToolCall, func(e agnogo.Event) {
//	    metrics.RecordToolCall(e.Data["name"].(string))
//	})
//	agent := agnogo.Agent("...", agnogo.WithEvents(bus))
type EventBus struct {
	mu          sync.RWMutex
	handlers    map[EventType][]EventHandler
	allHandlers []EventHandler
	eventCount  atomic.Int64
}

// NewEventBus creates a new event bus with no subscribers.
func NewEventBus() *EventBus {
	return &EventBus{
		handlers: make(map[EventType][]EventHandler),
	}
}

// On registers a handler for a specific event type.
func (eb *EventBus) On(eventType EventType, handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.handlers[eventType] = append(eb.handlers[eventType], handler)
}

// OnAll registers a handler that receives ALL event types.
func (eb *EventBus) OnAll(handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.allHandlers = append(eb.allHandlers, handler)
}

// Filter returns a new EventBus that only forwards matching events.
// It subscribes to the parent bus for the specified types and re-emits
// each matching event to the child bus.
func (eb *EventBus) Filter(types ...EventType) *EventBus {
	child := NewEventBus()
	for _, t := range types {
		t := t // capture loop var
		eb.On(t, func(e Event) {
			child.Emit(e)
		})
	}
	return child
}

// EventCount returns the number of events emitted (atomic counter).
func (eb *EventBus) EventCount() int64 {
	return eb.eventCount.Load()
}

// Emit publishes an event to all registered handlers for that event type,
// plus any handlers registered via OnAll. Increments the atomic event counter.
// If event.Timestamp is zero, it is set to time.Now().
func (eb *EventBus) Emit(event Event) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	eb.eventCount.Add(1)
	eb.mu.RLock()
	handlers := eb.handlers[event.Type]
	all := eb.allHandlers
	eb.mu.RUnlock()
	for _, h := range handlers {
		h(event)
	}
	for _, h := range all {
		h(event)
	}
}

// Trace returns a *Trace with all hooks wired to emit corresponding events
// to this EventBus.
func (eb *EventBus) Trace() *Trace {
	return &Trace{
		OnModelCall: func(messages []Message, resp *ModelResponse, dur time.Duration) {
			eb.Emit(Event{
				Type: EventModelCall,
				Data: map[string]any{
					"messages": len(messages),
					"duration": dur,
				},
			})
			text := ""
			toolCalls := 0
			if resp != nil {
				text = resp.Text
				toolCalls = len(resp.ToolCalls)
			}
			eb.Emit(Event{
				Type: EventModelDone,
				Data: map[string]any{
					"text":       text,
					"tool_calls": toolCalls,
					"duration":   dur,
				},
			})
		},
		OnToolCall: func(name string, args map[string]string, result string, dur time.Duration, err error) {
			eb.Emit(Event{
				Type: EventToolCall,
				Data: map[string]any{
					"name": name,
					"args": args,
				},
			})
			if err != nil {
				eb.Emit(Event{
					Type: EventToolError,
					Data: map[string]any{
						"name":     name,
						"error":    err.Error(),
						"duration": dur,
					},
				})
			} else {
				eb.Emit(Event{
					Type: EventToolDone,
					Data: map[string]any{
						"name":     name,
						"result":   result,
						"duration": dur,
					},
				})
			}
		},
		OnKnowledge: func(query string, result string, dur time.Duration) {
			eb.Emit(Event{
				Type: EventKnowledge,
				Data: map[string]any{
					"query":    query,
					"result":   result,
					"duration": dur,
				},
			})
		},
		OnMemory: func(key, value string) {
			eb.Emit(Event{
				Type: EventMemory,
				Data: map[string]any{
					"key":   key,
					"value": value,
				},
			})
		},
		OnGuardrail: func(name, direction string, blocked bool) {
			eb.Emit(Event{
				Type: EventGuardrail,
				Data: map[string]any{
					"name":      name,
					"direction": direction,
					"blocked":   blocked,
				},
			})
		},
	}
}

// WithEvents returns an Option that connects an EventBus to the agent.
// Uses Trace hooks for model/tool/knowledge events, and a Hook for run start/end.
func WithEvents(bus *EventBus) Option {
	traceOpt := WithTrace(bus.Trace())
	hookOpt := WithHooks(func(ctx context.Context, a *Core, s *Session, msg string, next NextFunc) (*Response, error) {
		bus.Emit(Event{Type: EventRunStart, Data: map[string]any{"message": msg, "session": s.ID}})
		resp, err := next(ctx, a, s, msg)
		data := map[string]any{"session": s.ID}
		if err != nil {
			data["error"] = err.Error()
		} else if resp != nil {
			data["text_len"] = len(resp.Text)
			data["tools_called"] = len(resp.ToolsCalled)
		}
		bus.Emit(Event{Type: EventRunEnd, Data: data})
		return resp, err
	})
	return optionFunc(func(sc *smartConfig) {
		traceOpt.applyOption(sc)
		hookOpt.applyOption(sc)
	})
}
