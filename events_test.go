package agnogo

import (
	"testing"
	"time"
)

func TestEventBusEmit(t *testing.T) {
	bus := NewEventBus()

	var received Event
	bus.On(EventToolCall, func(e Event) {
		received = e
	})

	bus.Emit(Event{
		Type: EventToolCall,
		Data: map[string]any{"name": "search"},
	})

	if received.Type != EventToolCall {
		t.Errorf("type = %q, want %q", received.Type, EventToolCall)
	}
	if received.Data["name"] != "search" {
		t.Errorf("data[name] = %v", received.Data["name"])
	}
	if received.Timestamp.IsZero() {
		t.Error("timestamp should be set automatically")
	}
}

func TestEventBusMultipleHandlers(t *testing.T) {
	bus := NewEventBus()

	count := 0
	bus.On(EventModelCall, func(e Event) { count++ })
	bus.On(EventModelCall, func(e Event) { count++ })
	bus.On(EventModelCall, func(e Event) { count++ })

	bus.Emit(Event{Type: EventModelCall})

	if count != 3 {
		t.Errorf("handler count = %d, want 3", count)
	}
}

func TestEventBusNoHandler(t *testing.T) {
	bus := NewEventBus()
	// Should not panic
	bus.Emit(Event{Type: EventToolDone, Data: map[string]any{"result": "ok"}})
}

func TestEventBusTrace(t *testing.T) {
	bus := NewEventBus()

	var events []EventType
	bus.On(EventModelCall, func(e Event) { events = append(events, e.Type) })
	bus.On(EventModelDone, func(e Event) { events = append(events, e.Type) })
	bus.On(EventToolCall, func(e Event) { events = append(events, e.Type) })
	bus.On(EventToolDone, func(e Event) { events = append(events, e.Type) })

	tr := bus.Trace()
	if tr == nil {
		t.Fatal("Trace() returned nil")
	}

	// Trigger model call hook
	tr.OnModelCall(
		[]Message{{Role: "user", Content: "hi"}},
		&ModelResponse{Text: "hello"},
		100*time.Millisecond,
	)

	// Trigger tool call hook (success)
	tr.OnToolCall("search", map[string]string{"q": "test"}, "found", 50*time.Millisecond, nil)

	// Should have received: model.call, model.done, tool.call, tool.done
	if len(events) != 4 {
		t.Fatalf("events = %v, want 4 events", events)
	}
	expected := []EventType{EventModelCall, EventModelDone, EventToolCall, EventToolDone}
	for i, e := range expected {
		if events[i] != e {
			t.Errorf("events[%d] = %q, want %q", i, events[i], e)
		}
	}
}

func TestEventBusOnAll(t *testing.T) {
	bus := NewEventBus()

	var received []EventType
	bus.OnAll(func(e Event) {
		received = append(received, e.Type)
	})

	bus.Emit(Event{Type: EventModelCall})
	bus.Emit(Event{Type: EventToolCall})
	bus.Emit(Event{Type: EventRunStart})

	if len(received) != 3 {
		t.Fatalf("got %d events, want 3", len(received))
	}
	want := []EventType{EventModelCall, EventToolCall, EventRunStart}
	for i, w := range want {
		if received[i] != w {
			t.Errorf("received[%d] = %q, want %q", i, received[i], w)
		}
	}
}

func TestEventBusFilter(t *testing.T) {
	bus := NewEventBus()
	filtered := bus.Filter(EventModelCall, EventModelDone)

	var received []EventType
	filtered.On(EventModelCall, func(e Event) { received = append(received, e.Type) })
	filtered.On(EventModelDone, func(e Event) { received = append(received, e.Type) })
	filtered.On(EventToolCall, func(e Event) { received = append(received, e.Type) })

	bus.Emit(Event{Type: EventModelCall})
	bus.Emit(Event{Type: EventToolCall})  // should NOT reach filtered bus
	bus.Emit(Event{Type: EventModelDone})

	if len(received) != 2 {
		t.Fatalf("got %d events, want 2: %v", len(received), received)
	}
	if received[0] != EventModelCall {
		t.Errorf("received[0] = %q, want %q", received[0], EventModelCall)
	}
	if received[1] != EventModelDone {
		t.Errorf("received[1] = %q, want %q", received[1], EventModelDone)
	}
}

func TestEventBusEventCount(t *testing.T) {
	bus := NewEventBus()

	if bus.EventCount() != 0 {
		t.Fatalf("initial count = %d, want 0", bus.EventCount())
	}

	bus.Emit(Event{Type: EventModelCall})
	bus.Emit(Event{Type: EventToolCall})
	bus.Emit(Event{Type: EventRunEnd})

	if bus.EventCount() != 3 {
		t.Errorf("count = %d, want 3", bus.EventCount())
	}
}
