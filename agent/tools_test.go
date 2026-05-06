package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

type fakeReminderStore struct {
	reminders map[string][]Reminder
	counter   int64
}

func newFakeReminderStore() *fakeReminderStore {
	return &fakeReminderStore{
		reminders: make(map[string][]Reminder),
	}
}

func (s *fakeReminderStore) AddReminder(userID, contextToken, message string, triggerAt time.Time) string {
	s.counter++
	id := fmt.Sprintf("r_%d", s.counter)
	s.reminders[userID] = append(s.reminders[userID], Reminder{
		ID:           id,
		UserID:       userID,
		ContextToken: contextToken,
		Message:      message,
		TriggerAt:    triggerAt,
	})
	return id
}

func (s *fakeReminderStore) ListReminders(userID string) []Reminder {
	result := make([]Reminder, 0)
	for _, r := range s.reminders[userID] {
		if r.TriggerAt.After(time.Now()) {
			result = append(result, r)
		}
	}
	return result
}

func (s *fakeReminderStore) RemoveReminder(userID, id string) bool {
	list := s.reminders[userID]
	for i, r := range list {
		if r.ID == id {
			s.reminders[userID] = append(list[:i], list[i+1:]...)
			return true
		}
	}
	return false
}

func TestBuiltinToolsNoRemindersNoCommands(t *testing.T) {
	tools := BuiltinTools(nil, false)
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Name != "get_current_time" {
		t.Fatalf("tool name: got %q, want %q", tools[0].Name, "get_current_time")
	}
}

func TestBuiltinToolsWithReminders(t *testing.T) {
	store := newFakeReminderStore()
	tools := BuiltinTools(store, false)
	if len(tools) != 4 {
		t.Fatalf("expected 4 tools, got %d", len(tools))
	}
	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Name] = true
	}
	expected := []string{"get_current_time", "set_reminder", "list_reminders", "cancel_reminder"}
	for _, name := range expected {
		if !names[name] {
			t.Fatalf("missing tool: %s", name)
		}
	}
}

func TestBuiltinToolsWithRemindersAndCommands(t *testing.T) {
	store := newFakeReminderStore()
	tools := BuiltinTools(store, true)
	if len(tools) != 5 {
		t.Fatalf("expected 5 tools, got %d", len(tools))
	}
	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Name] = true
	}
	if !names["execute_command"] {
		t.Fatal("missing execute_command tool")
	}
}

func TestToolGetCurrentTime(t *testing.T) {
	tools := BuiltinTools(nil, false)
	handler := tools[0].Handler
	result := handler(context.Background(), ToolCall{})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "20") {
		t.Fatalf("result should look like a datetime: %q", result.Content)
	}
}

func TestToolSetReminderSuccess(t *testing.T) {
	store := newFakeReminderStore()
	tools := BuiltinTools(store, false)
	var setReminderHandler ToolHandler
	for _, tool := range tools {
		if tool.Name == "set_reminder" {
			setReminderHandler = tool.Handler
			break
		}
	}
	if setReminderHandler == nil {
		t.Fatal("set_reminder tool not found")
	}

	call := ToolCall{
		UserID:       "user1",
		ContextToken: "ctx1",
		Input:        json.RawMessage(`{"message":"开会","minutes":5}`),
	}
	result := setReminderHandler(context.Background(), call)
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "开会") {
		t.Fatalf("result should contain reminder message: %q", result.Content)
	}
}

func TestToolSetReminderInvalidMinutes(t *testing.T) {
	store := newFakeReminderStore()
	tools := BuiltinTools(store, false)
	var setReminderHandler ToolHandler
	for _, tool := range tools {
		if tool.Name == "set_reminder" {
			setReminderHandler = tool.Handler
			break
		}
	}

	call := ToolCall{
		UserID:       "user1",
		ContextToken: "ctx1",
		Input:        json.RawMessage(`{"message":"test","minutes":0}`),
	}
	result := setReminderHandler(context.Background(), call)
	if !result.IsError {
		t.Fatal("expected error for zero minutes")
	}
}

func TestToolSetReminderInvalidJSON(t *testing.T) {
	store := newFakeReminderStore()
	tools := BuiltinTools(store, false)
	var setReminderHandler ToolHandler
	for _, tool := range tools {
		if tool.Name == "set_reminder" {
			setReminderHandler = tool.Handler
			break
		}
	}

	call := ToolCall{
		UserID:       "user1",
		ContextToken: "ctx1",
		Input:        json.RawMessage(`not json`),
	}
	result := setReminderHandler(context.Background(), call)
	if !result.IsError {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestToolListRemindersEmpty(t *testing.T) {
	store := newFakeReminderStore()
	tools := BuiltinTools(store, false)
	var listHandler ToolHandler
	for _, tool := range tools {
		if tool.Name == "list_reminders" {
			listHandler = tool.Handler
			break
		}
	}

	result := listHandler(context.Background(), ToolCall{UserID: "user1"})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "没有") {
		t.Fatalf("result should mention no reminders: %q", result.Content)
	}
}

func TestToolCancelReminderNotFound(t *testing.T) {
	store := newFakeReminderStore()
	tools := BuiltinTools(store, false)
	var cancelHandler ToolHandler
	for _, tool := range tools {
		if tool.Name == "cancel_reminder" {
			cancelHandler = tool.Handler
			break
		}
	}

	call := ToolCall{
		UserID: "user1",
		Input:  json.RawMessage(`{"reminder_id":"nonexistent"}`),
	}
	result := cancelHandler(context.Background(), call)
	if !result.IsError {
		t.Fatal("expected error for nonexistent reminder")
	}
}

func TestRunCommandSafe(t *testing.T) {
	result, isErr := RunCommand(context.Background(), "echo ok", DefaultAllowedCommands)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}
	if !strings.Contains(result, "ok") {
		t.Fatalf("result should contain 'ok': %q", result)
	}
}

func TestRunCommandBlocked(t *testing.T) {
	_, isErr := RunCommand(context.Background(), "rm -rf /tmp/test", DefaultAllowedCommands)
	if !isErr {
		t.Fatal("expected error for blocked command")
	}
}

func TestRunCommandBlockedVariations(t *testing.T) {
	blocked := []string{
		"rm -rf /",
		"del /f file",
		"format C:",
		"mkfs.ext4 /dev/sda",
	}
	for _, cmd := range blocked {
		_, isErr := RunCommand(context.Background(), cmd, DefaultAllowedCommands)
		if !isErr {
			t.Fatalf("command %q should be blocked", cmd)
		}
	}
}

func TestRunCommandContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	// Sleep longer than timeout.
	_, isErr := RunCommand(ctx, "sleep 10", []string{"sleep"})
	if !isErr {
		t.Fatal("expected error for timed out command")
	}
}
