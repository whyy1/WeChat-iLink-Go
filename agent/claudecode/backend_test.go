package claudecode

import (
	"context"
	"testing"
	"time"
)

func TestNewBackendDefaults(t *testing.T) {
	b := NewBackend(Config{})
	if b == nil {
		t.Fatal("NewBackend should not return nil")
	}
	if b.command != DefaultCommand {
		t.Fatalf("command: got %q, want %q", b.command, DefaultCommand)
	}
	if b.timeout != DefaultTimeout {
		t.Fatalf("timeout: got %v, want %v", b.timeout, DefaultTimeout)
	}
}

func TestNewBackendCustomConfig(t *testing.T) {
	b := NewBackend(Config{
		Command:      "custom-claude",
		Model:        "claude-3-haiku",
		SystemPrompt: "custom prompt",
		Timeout:      5 * time.Minute,
		WorkingDir:   "/tmp",
	})
	if b.command != "custom-claude" {
		t.Fatalf("command: got %q", b.command)
	}
	if b.model != "claude-3-haiku" {
		t.Fatalf("model: got %q", b.model)
	}
	if b.systemPrompt != "custom prompt" {
		t.Fatalf("systemPrompt: got %q", b.systemPrompt)
	}
	if b.timeout != 5*time.Minute {
		t.Fatalf("timeout: got %v", b.timeout)
	}
	if b.workingDir != "/tmp" {
		t.Fatalf("workingDir: got %q", b.workingDir)
	}
}

func TestBackendResetConversation(t *testing.T) {
	b := NewBackend(Config{})
	// Should not panic.
	b.ResetConversation("user1")
}

func TestBackendGetConversationLength(t *testing.T) {
	b := NewBackend(Config{})
	if b.GetConversationLength("user1") != 0 {
		t.Fatal("conversation length should be 0")
	}
}

func TestBackendChatWithCtxEmptyText(t *testing.T) {
	b := NewBackend(Config{Command: "echo"})
	_, err := b.ChatWithCtx(context.Background(), "user1", "ctx1", "")
	if err == nil {
		t.Fatal("expected error for empty text")
	}
}

func TestBackendChatWithCtxWhitespaceText(t *testing.T) {
	b := NewBackend(Config{Command: "echo"})
	_, err := b.ChatWithCtx(context.Background(), "user1", "ctx1", "   ")
	if err == nil {
		t.Fatal("expected error for whitespace text")
	}
}

func TestBackendChatWithCtxNonexistentCommand(t *testing.T) {
	b := NewBackend(Config{Command: "nonexistent_command_12345"})
	_, err := b.ChatWithCtx(context.Background(), "user1", "ctx1", "hello")
	if err == nil {
		t.Fatal("expected error for nonexistent command")
	}
}

func TestBackendChatWithCtxTimeout(t *testing.T) {
	b := NewBackend(Config{
		Command: "sleep",
		Timeout: 1 * time.Millisecond,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	_, err := b.ChatWithCtx(ctx, "user1", "ctx1", "hello")
	if err == nil {
		t.Fatal("expected error for timeout")
	}
}

func TestDefaultConstants(t *testing.T) {
	if DefaultCommand != "claude" {
		t.Fatalf("DefaultCommand: got %q, want %q", DefaultCommand, "claude")
	}
	if DefaultTimeout != 2*time.Minute {
		t.Fatalf("DefaultTimeout: got %v, want %v", DefaultTimeout, 2*time.Minute)
	}
}
