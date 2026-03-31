package agnogo

import (
	"context"
	"strings"
	"testing"
)

// ── Mock Provider for smart.go tests ────────────────────

type mockProvider struct{}

func (p *mockProvider) ChatCompletion(_ context.Context, messages []Message, tools []map[string]any) (*ModelResponse, error) {
	return &ModelResponse{Text: "mock response"}, nil
}

// ── RegisterProvider / DetectProvider ────────────────────

func TestRegisterAndDetectProvider(t *testing.T) {
	// Save and restore the global registry.
	saved := providerRegistry
	defer func() { providerRegistry = saved }()
	providerRegistry = nil

	RegisterProvider("TEST_AGNOGO_KEY_1", "test-model", func(apiKey string) ModelProvider {
		return &mockProvider{}
	})

	t.Setenv("TEST_AGNOGO_KEY_1", "sk-test-123")

	p, err := DetectProvider()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestDetectProviderNoRegistry(t *testing.T) {
	saved := providerRegistry
	defer func() { providerRegistry = saved }()
	providerRegistry = nil

	_, err := DetectProvider()
	if err == nil {
		t.Fatal("expected error when registry is empty")
	}
	if !strings.Contains(err.Error(), "no providers registered") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDetectProviderNoEnvVar(t *testing.T) {
	saved := providerRegistry
	defer func() { providerRegistry = saved }()
	providerRegistry = nil

	RegisterProvider("TEST_AGNOGO_MISSING_KEY", "test-model", func(apiKey string) ModelProvider {
		return &mockProvider{}
	})

	// Do NOT set the env var — it should fail.
	_, err := DetectProvider()
	if err == nil {
		t.Fatal("expected error when env var is not set")
	}
	if !strings.Contains(err.Error(), "no API key found") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ── Agent() Tests ───────────────────────────────────────

func TestQuickWithModel(t *testing.T) {
	a := Agent("Test instructions", WithModel(&mockProvider{}))

	if a.instructions != "Test instructions" {
		t.Errorf("instructions = %q", a.instructions)
	}
	if a.model == nil {
		t.Error("model should not be nil")
	}
}

func TestQuickSafeDefaults(t *testing.T) {
	a := Agent("safe agent", WithModel(&mockProvider{}))

	if a.retry == nil {
		t.Error("retry should be set by default")
	}
	if a.history == nil {
		t.Error("history should be set by default")
	}
	// Hallucination guard is added as an output guardrail.
	if len(a.outputGuards) == 0 {
		t.Error("expected hallucination guard as output guardrail")
	}
}

func TestQuickUnsafe(t *testing.T) {
	a := Agent("unsafe agent", WithModel(&mockProvider{}), Unsafe())

	if a.retry != nil {
		t.Error("retry should be nil with Unsafe()")
	}
	if a.history != nil {
		t.Error("history should be nil with Unsafe()")
	}
	if len(a.outputGuards) != 0 {
		t.Errorf("expected no output guards with Unsafe(), got %d", len(a.outputGuards))
	}
}

func TestQuickWithTools(t *testing.T) {
	tool := ToolDef{
		Name: "test_tool",
		Desc: "A test tool",
		Fn: func(ctx context.Context, args map[string]string) (string, error) {
			return "ok", nil
		},
	}

	a := Agent("tool agent", WithModel(&mockProvider{}), WithTools(tool))

	if a.tools.Count() == 0 {
		t.Error("expected at least one tool registered")
	}
	if got := a.tools.Get("test_tool"); got == nil {
		t.Error("test_tool not found in registry")
	}
}

func TestQuickWithMemory(t *testing.T) {
	a := Agent("memory agent", WithModel(&mockProvider{}), WithMemory())

	if a.memory == nil {
		t.Error("expected memory extractor to be set")
	}
}

func TestQuickWithMaxLoops(t *testing.T) {
	a := Agent("loop agent", WithModel(&mockProvider{}), WithMaxLoops(5))

	if a.maxLoops != 5 {
		t.Errorf("maxLoops = %d, want 5", a.maxLoops)
	}
}

func TestQuickWithStorage(t *testing.T) {
	store := NewMemoryStorage()
	a := Agent("storage agent", WithModel(&mockProvider{}), WithStorage(store))

	if a.storage == nil {
		t.Error("expected storage to be set")
	}
}
