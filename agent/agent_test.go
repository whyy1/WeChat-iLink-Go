package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type fakeBackend struct {
	chatFn            func(ctx context.Context, userID, contextToken, text string) (string, error)
	resetFn           func(userID string)
	getConversationFn func(userID string) int
	setToolsFn        func(tools []Tool)
}

func (f *fakeBackend) ChatWithCtx(ctx context.Context, userID, contextToken, text string) (string, error) {
	if f.chatFn != nil {
		return f.chatFn(ctx, userID, contextToken, text)
	}
	return "fake reply", nil
}

func (f *fakeBackend) ResetConversation(userID string) {
	if f.resetFn != nil {
		f.resetFn(userID)
	}
}

func (f *fakeBackend) GetConversationLength(userID string) int {
	if f.getConversationFn != nil {
		return f.getConversationFn(userID)
	}
	return 0
}

func (f *fakeBackend) SetTools(tools []Tool) {
	if f.setToolsFn != nil {
		f.setToolsFn(tools)
	}
}

func TestAgentNew(t *testing.T) {
	backend := &fakeBackend{}
	a := New(backend)
	if a == nil {
		t.Fatal("New should not return nil")
	}
}

func TestAgentChat(t *testing.T) {
	var called bool
	backend := &fakeBackend{
		chatFn: func(ctx context.Context, userID, contextToken, text string) (string, error) {
			called = true
			if userID != "user1" {
				t.Fatalf("userID: got %q, want %q", userID, "user1")
			}
			if contextToken != "ctx1" {
				t.Fatalf("contextToken: got %q, want %q", contextToken, "ctx1")
			}
			if text != "hello" {
				t.Fatalf("text: got %q, want %q", text, "hello")
			}
			return "reply", nil
		},
	}
	a := New(backend)
	result, err := a.Chat("user1", "ctx1", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("backend.ChatWithCtx should have been called")
	}
	if result != "reply" {
		t.Fatalf("result: got %q, want %q", result, "reply")
	}
}

func TestAgentChatWithCtx(t *testing.T) {
	backend := &fakeBackend{
		chatFn: func(ctx context.Context, userID, contextToken, text string) (string, error) {
			return "ctx reply", nil
		},
	}
	a := New(backend)
	result, err := a.ChatWithCtx(context.Background(), "user1", "ctx1", "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ctx reply" {
		t.Fatalf("result: got %q, want %q", result, "ctx reply")
	}
}

func TestAgentChatError(t *testing.T) {
	backend := &fakeBackend{
		chatFn: func(ctx context.Context, userID, contextToken, text string) (string, error) {
			return "", errors.New("backend error")
		},
	}
	a := New(backend)
	_, err := a.Chat("user1", "ctx1", "test")
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "backend error" {
		t.Fatalf("error: got %q, want %q", err.Error(), "backend error")
	}
}

func TestAgentResetConversation(t *testing.T) {
	var resetUser string
	backend := &fakeBackend{
		resetFn: func(userID string) {
			resetUser = userID
		},
	}
	a := New(backend)
	a.ResetConversation("user1")
	if resetUser != "user1" {
		t.Fatalf("resetUser: got %q, want %q", resetUser, "user1")
	}
}

func TestAgentGetConversationLength(t *testing.T) {
	backend := &fakeBackend{
		getConversationFn: func(userID string) int {
			if userID == "user1" {
				return 5
			}
			return 0
		},
	}
	a := New(backend)
	if a.GetConversationLength("user1") != 5 {
		t.Fatal("expected 5")
	}
	if a.GetConversationLength("user2") != 0 {
		t.Fatal("expected 0")
	}
}

func TestAgentSetTools(t *testing.T) {
	var setToolsCalled bool
	backend := &fakeBackend{
		setToolsFn: func(tools []Tool) {
			setToolsCalled = true
			if len(tools) != 1 {
				t.Fatalf("expected 1 tool, got %d", len(tools))
			}
			if tools[0].Name != "test_tool" {
				t.Fatalf("tool name: got %q, want %q", tools[0].Name, "test_tool")
			}
		},
	}
	a := New(backend)
	a.SetTools([]Tool{{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: map[string]any{"type": "object"},
		Handler: func(ctx context.Context, call ToolCall) ToolResult {
			return ToolResult{Content: "ok"}
		},
	}})
	if !setToolsCalled {
		t.Fatal("SetTools should have been called on backend")
	}
}

func TestAgentSetToolsNonRegistrar(t *testing.T) {
	// Backend without ToolRegistrar interface should not panic.
	backend := &fakeBackend{}
	a := New(backend)
	a.SetTools([]Tool{{
		Name: "test",
	}})
	// Should not panic.
}

func TestToolTypes(t *testing.T) {
	call := ToolCall{
		UserID:       "user1",
		ContextToken: "ctx1",
		Name:         "test",
		Input:        json.RawMessage(`{"key":"value"}`),
	}
	if call.UserID != "user1" {
		t.Fatal("UserID mismatch")
	}

	result := ToolResult{
		Content: "result",
		IsError: false,
	}
	if result.Content != "result" {
		t.Fatal("Content mismatch")
	}
}

func TestReminderType(t *testing.T) {
	now := time.Now()
	r := Reminder{
		ID:           "r1",
		UserID:       "user1",
		ContextToken: "ctx1",
		Message:      "test",
		TriggerAt:    now,
	}
	if r.ID != "r1" {
		t.Fatal("ID mismatch")
	}
	if r.TriggerAt != now {
		t.Fatal("TriggerAt mismatch")
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
