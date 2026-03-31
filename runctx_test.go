package agnogo

import (
	"context"
	"testing"
)

func TestRunContextSetGet(t *testing.T) {
	rc := NewRunContext()
	rc.Set("key", "hello")
	rc.Set("num", 42)

	if rc.Get("key") != "hello" {
		t.Errorf("Get(key) = %v", rc.Get("key"))
	}
	if rc.Get("num") != 42 {
		t.Errorf("Get(num) = %v", rc.Get("num"))
	}
	if rc.Get("missing") != nil {
		t.Errorf("Get(missing) = %v, want nil", rc.Get("missing"))
	}
}

func TestRunContextGetStr(t *testing.T) {
	rc := NewRunContext()
	rc.Set("name", "alice")
	rc.Set("count", 10)

	if rc.GetStr("name") != "alice" {
		t.Errorf("GetStr(name) = %q", rc.GetStr("name"))
	}
	// Non-string returns ""
	if rc.GetStr("count") != "" {
		t.Errorf("GetStr(count) = %q, want empty", rc.GetStr("count"))
	}
	// Missing returns ""
	if rc.GetStr("missing") != "" {
		t.Errorf("GetStr(missing) = %q, want empty", rc.GetStr("missing"))
	}
}

func TestRunContextGetInt(t *testing.T) {
	rc := NewRunContext()
	rc.Set("count", 7)
	rc.Set("price", 3.14)
	rc.Set("name", "bob")

	if rc.GetInt("count") != 7 {
		t.Errorf("GetInt(count) = %d", rc.GetInt("count"))
	}
	// float64 should convert to int
	if rc.GetInt("price") != 3 {
		t.Errorf("GetInt(price) = %d, want 3", rc.GetInt("price"))
	}
	// Non-numeric returns 0
	if rc.GetInt("name") != 0 {
		t.Errorf("GetInt(name) = %d, want 0", rc.GetInt("name"))
	}
	// Missing returns 0
	if rc.GetInt("missing") != 0 {
		t.Errorf("GetInt(missing) = %d, want 0", rc.GetInt("missing"))
	}
}

func TestRunContextWithContext(t *testing.T) {
	rc := NewRunContext()
	rc.Set("user_id", "u-123")

	ctx := rc.WithContext(context.Background())

	extracted := RunCtx(ctx)
	if extracted == nil {
		t.Fatal("RunCtx returned nil")
	}
	if extracted.GetStr("user_id") != "u-123" {
		t.Errorf("user_id = %q", extracted.GetStr("user_id"))
	}
}

func TestRunCtxNil(t *testing.T) {
	// RunCtx on a plain context without RunContext should return nil
	ctx := context.Background()
	if RunCtx(ctx) != nil {
		t.Error("expected nil RunContext from plain context")
	}
}
