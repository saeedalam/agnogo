package agnogo

import (
	"context"
	"testing"
)

func TestDetectPIIEmail(t *testing.T) {
	matches := DetectPII("Contact me at john@example.com please")
	found := false
	for _, m := range matches {
		if m.Type == PIIEmail && m.Match == "john@example.com" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected email detection, got matches: %v", matches)
	}
}

func TestDetectPIIPhone(t *testing.T) {
	matches := DetectPII("Call me at 555-123-4567")
	found := false
	for _, m := range matches {
		if m.Type == PIIPhone {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected phone detection, got matches: %v", matches)
	}
}

func TestDetectPIICreditCard(t *testing.T) {
	// 4111 1111 1111 1111 is a valid Luhn number.
	matches := DetectPII("My card is 4111 1111 1111 1111")
	found := false
	for _, m := range matches {
		if m.Type == PIICreditCard {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected credit card detection, got matches: %v", matches)
	}
}

func TestDetectPIISSN(t *testing.T) {
	matches := DetectPII("SSN: 123-45-6789")
	found := false
	for _, m := range matches {
		if m.Type == PIISSN {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected SSN detection, got matches: %v", matches)
	}
}

func TestDetectPIIIP(t *testing.T) {
	matches := DetectPII("Server at 192.168.1.100")
	found := false
	for _, m := range matches {
		if m.Type == PIIIPAddress {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IP detection, got matches: %v", matches)
	}
}

func TestRedactPII(t *testing.T) {
	input := "Email me at test@example.com"
	result := RedactPII(input)
	if result == input {
		t.Fatal("expected redaction but text unchanged")
	}
	if result != "Email me at [EMAIL REDACTED]" {
		t.Fatalf("unexpected redaction result: %s", result)
	}
}

func TestRedactPIIExcept(t *testing.T) {
	input := "Email test@example.com and SSN 123-45-6789"
	result := RedactPIIExcept(input, []PIIType{PIIEmail})
	// Email should remain, SSN should be redacted.
	if result != "Email test@example.com and SSN [SSN REDACTED]" {
		t.Fatalf("unexpected result: %s", result)
	}
}

func TestPIIGuardBlocksOutput(t *testing.T) {
	config := &PIIConfig{BlockOutput: true}
	guard := piiOutputGuardrail(config)

	err := guard.Check(context.Background(), NewSession("test"), "Hello there")
	if err != nil {
		t.Fatalf("expected no error for clean text, got: %v", err)
	}

	err = guard.Check(context.Background(), NewSession("test"), "Email me at user@example.com")
	if err == nil {
		t.Fatal("expected error for PII output, got nil")
	}
}

func TestPIIGuardRedactsInput(t *testing.T) {
	config := &PIIConfig{RedactInput: true}
	guard := piiInputGuardrail(config)

	session := NewSession("test")
	msg := "My email is user@example.com"
	session.AddMessage("user", msg)

	err := guard.Check(context.Background(), session, msg)
	if err != nil {
		t.Fatalf("input guard should not return error, got: %v", err)
	}

	// Check that history was redacted.
	history := session.GetHistory()
	lastUser := ""
	for _, h := range history {
		if h.Role == "user" {
			lastUser = h.Content
		}
	}
	if lastUser == msg {
		t.Fatal("expected message to be redacted in history")
	}
	if lastUser != "My email is [EMAIL REDACTED]" {
		t.Fatalf("unexpected redacted message: %s", lastUser)
	}
}

func TestLuhnValidation(t *testing.T) {
	tests := []struct {
		number string
		valid  bool
	}{
		{"4111111111111111", true},
		{"4111 1111 1111 1111", true},
		{"4111-1111-1111-1111", true},
		{"1234567890123456", false},
		{"0000000000000000", true}, // Luhn valid (all zeros sum to 0)
		{"12345", false},          // too short
	}
	for _, tt := range tests {
		got := luhnValid(tt.number)
		if got != tt.valid {
			t.Errorf("luhnValid(%q) = %v, want %v", tt.number, got, tt.valid)
		}
	}
}

func TestSessionConsent(t *testing.T) {
	s := NewSession("test")

	if s.HasConsent("analytics") {
		t.Fatal("expected no consent initially")
	}

	s.SetConsent("analytics", true)
	if !s.HasConsent("analytics") {
		t.Fatal("expected consent after SetConsent(true)")
	}

	s.SetConsent("analytics", false)
	if s.HasConsent("analytics") {
		t.Fatal("expected no consent after SetConsent(false)")
	}
}

func TestDetectPIINoPII(t *testing.T) {
	matches := DetectPII("This is a clean message with no personal data.")
	if len(matches) != 0 {
		t.Fatalf("expected no matches, got %d: %v", len(matches), matches)
	}
}
