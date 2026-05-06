package ilink

import (
	"strings"
	"testing"
)

func TestNewAgent(t *testing.T) {
	t.Run("defaults to anthropic backend", func(t *testing.T) {
		a := NewAgent(AgentConfig{APIKey: "test-key"})
		if a == nil {
			t.Fatal("NewAgent should not return nil")
		}
		if a.agent == nil {
			t.Fatal("internal agent should not be nil")
		}
	})

	t.Run("with anthropic backend", func(t *testing.T) {
		a := NewAgent(AgentConfig{
			Backend:        BackendAnthropic,
			APIKey:         "test-key",
			Model:          "claude-3-haiku-20240307",
			MaxTokens:      1024,
			SystemPrompt:   "custom prompt",
			EnableCommands: true,
		})
		if a == nil {
			t.Fatal("NewAgent should not return nil")
		}
	})

	t.Run("with claude-code backend", func(t *testing.T) {
		a := NewAgent(AgentConfig{
			Backend:           BackendClaudeCode,
			ClaudeCodeCommand: "echo",
		})
		if a == nil {
			t.Fatal("NewAgent should not return nil")
		}
	})
}

func TestAgentResetConversation(t *testing.T) {
	a := NewAgent(AgentConfig{APIKey: "test"})
	// Should not panic on reset.
	a.ResetConversation("user1")
	if a.GetConversationLength("user1") != 0 {
		t.Fatal("conversation length should be 0 after reset")
	}
}

func TestAgentSetReminderStore(t *testing.T) {
	a := NewAgent(AgentConfig{APIKey: "test"})
	store := NewReminderStore()
	// Should not panic.
	a.SetReminderStore(store)
}

func TestAgentConfigDefaults(t *testing.T) {
	cfg := AgentConfig{}
	if cfg.Backend != "" {
		t.Fatal("default Backend should be empty")
	}
	if cfg.APIKey != "" {
		t.Fatal("default APIKey should be empty")
	}
	if cfg.Model != "" {
		t.Fatal("default Model should be empty")
	}
	if cfg.MaxTokens != 0 {
		t.Fatal("default MaxTokens should be 0")
	}
	if cfg.SystemPrompt != "" {
		t.Fatal("default SystemPrompt should be empty")
	}
	if cfg.EnableCommands {
		t.Fatal("default EnableCommands should be false")
	}
}

func TestDefaultAgentModel(t *testing.T) {
	if DefaultAgentModel == "" {
		t.Fatal("DefaultAgentModel should not be empty")
	}
	if !strings.Contains(DefaultAgentModel, "claude") {
		t.Fatalf("DefaultAgentModel should contain 'claude', got %q", DefaultAgentModel)
	}
}

func TestBackendConstants(t *testing.T) {
	if BackendAnthropic != "anthropic" {
		t.Fatalf("BackendAnthropic: got %q, want %q", BackendAnthropic, "anthropic")
	}
	if BackendClaudeCode != "claude-code" {
		t.Fatalf("BackendClaudeCode: got %q, want %q", BackendClaudeCode, "claude-code")
	}
}

func TestTruncateText(t *testing.T) {
	t.Run("short text", func(t *testing.T) {
		got := TruncateText("hello", 10)
		if got != "hello" {
			t.Fatalf("got %q, want %q", got, "hello")
		}
	})
	t.Run("exact length", func(t *testing.T) {
		got := TruncateText("hello", 5)
		if got != "hello" {
			t.Fatalf("got %q, want %q", got, "hello")
		}
	})
	t.Run("truncated", func(t *testing.T) {
		got := TruncateText("hello world", 5)
		if got != "hello..." {
			t.Fatalf("got %q, want %q", got, "hello...")
		}
	})
}
