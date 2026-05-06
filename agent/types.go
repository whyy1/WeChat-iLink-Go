package agent

import (
	"context"
	"encoding/json"
	"time"
)

const (
	BackendAnthropic  = "anthropic"
	BackendClaudeCode = "claude-code"
)

type Backend interface {
	ChatWithCtx(ctx context.Context, userID, contextToken, text string) (string, error)
	ResetConversation(userID string)
	GetConversationLength(userID string) int
}

type ToolRegistrar interface {
	SetTools(tools []Tool)
}

type Agent struct {
	backend Backend
}

func New(backend Backend) *Agent {
	return &Agent{backend: backend}
}

func (a *Agent) Chat(userID, contextToken, text string) (string, error) {
	return a.ChatWithCtx(context.Background(), userID, contextToken, text)
}

func (a *Agent) ChatWithCtx(ctx context.Context, userID, contextToken, text string) (string, error) {
	return a.backend.ChatWithCtx(ctx, userID, contextToken, text)
}

func (a *Agent) ResetConversation(userID string) {
	a.backend.ResetConversation(userID)
}

func (a *Agent) GetConversationLength(userID string) int {
	return a.backend.GetConversationLength(userID)
}

func (a *Agent) SetTools(tools []Tool) {
	if registrar, ok := a.backend.(ToolRegistrar); ok {
		registrar.SetTools(tools)
	}
}

type Tool struct {
	Name        string
	Description string
	InputSchema map[string]any
	Handler     ToolHandler
}

type ToolHandler func(ctx context.Context, call ToolCall) ToolResult

type ToolCall struct {
	UserID       string
	ContextToken string
	Name         string
	Input        json.RawMessage
}

type ToolResult struct {
	Content string
	IsError bool
}

type Reminder struct {
	ID           string
	UserID       string
	ContextToken string
	Message      string
	TriggerAt    time.Time
}

type ReminderStore interface {
	AddReminder(userID, contextToken, message string, triggerAt time.Time) string
	ListReminders(userID string) []Reminder
	RemoveReminder(userID, id string) bool
}
