package factory

import (
	"testing"

	"github.com/whyy1/WeChat-iLink-Go/agent"
	"github.com/whyy1/WeChat-iLink-Go/agent/anthropic"
	"github.com/whyy1/WeChat-iLink-Go/agent/claudecode"
)

func TestNewBackendAnthropic(t *testing.T) {
	b, err := NewBackend(BackendConfig{
		Backend: BackendAnthropic,
		Anthropic: anthropic.Config{
			APIKey: "test-key",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b == nil {
		t.Fatal("backend should not be nil")
	}
}

func TestNewBackendClaudeCode(t *testing.T) {
	b, err := NewBackend(BackendConfig{
		Backend: BackendClaudeCode,
		ClaudeCode: claudecode.Config{
			Command: "echo",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b == nil {
		t.Fatal("backend should not be nil")
	}
}

func TestNewBackendDefaultIsAnthropic(t *testing.T) {
	b, err := NewBackend(BackendConfig{
		Anthropic: anthropic.Config{
			APIKey: "test-key",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b == nil {
		t.Fatal("backend should not be nil")
	}
	// Should be an Anthropic backend.
	if _, ok := b.(*anthropic.Backend); !ok {
		t.Fatal("expected *anthropic.Backend")
	}
}

func TestNewBackendUnsupported(t *testing.T) {
	_, err := NewBackend(BackendConfig{
		Backend: "unsupported",
	})
	if err == nil {
		t.Fatal("expected error for unsupported backend")
	}
}

func TestNewFromConfig(t *testing.T) {
	a, err := NewFromConfig(BackendConfig{
		Backend: BackendAnthropic,
		Anthropic: anthropic.Config{
			APIKey: "test-key",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a == nil {
		t.Fatal("agent should not be nil")
	}
}

func TestBackendConstants(t *testing.T) {
	if BackendAnthropic != agent.BackendAnthropic {
		t.Fatalf("BackendAnthropic: got %q, want %q", BackendAnthropic, agent.BackendAnthropic)
	}
	if BackendClaudeCode != agent.BackendClaudeCode {
		t.Fatalf("BackendClaudeCode: got %q, want %q", BackendClaudeCode, agent.BackendClaudeCode)
	}
}
