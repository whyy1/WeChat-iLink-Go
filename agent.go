package ilink

import (
	"context"
	"time"

	"github.com/whyy1/WeChat-iLink-Go/agent"
	"github.com/whyy1/WeChat-iLink-Go/agent/anthropic"
	"github.com/whyy1/WeChat-iLink-Go/agent/claudecode"
	"github.com/whyy1/WeChat-iLink-Go/agent/factory"
)

const (
	// DefaultAgentModel is the default model used by Agent.
	DefaultAgentModel = string(anthropic.DefaultModel)

	// BackendAnthropic selects the Anthropic API backend.
	BackendAnthropic = agent.BackendAnthropic

	// BackendClaudeCode selects the Claude Code CLI backend.
	BackendClaudeCode = agent.BackendClaudeCode
)

// AgentConfig holds configuration for creating an Agent.
type AgentConfig struct {
	// Backend selects which Claude backend to use: "anthropic" or "claude-code".
	// Defaults to "anthropic".
	Backend string

	// Anthropic API configuration.
	APIKey         string
	BaseURL        string
	Model          string
	MaxTokens      int64
	SystemPrompt   string
	EnableCommands bool

	// Claude Code CLI configuration.
	ClaudeCodeCommand      string
	ClaudeCodeModel        string
	ClaudeCodeSystemPrompt string
	ClaudeCodeTimeout      time.Duration
	ClaudeCodeWorkingDir   string
}

// Agent wraps a provider-neutral backend with compatibility for existing callers.
type Agent struct {
	agent        *agent.Agent
	backend      agent.Backend
	enableCommands bool
	reminderStore *ReminderStore
}

// NewAgent creates a new Agent from the given config.
func NewAgent(cfg AgentConfig) *Agent {
	backendCfg := factory.BackendConfig{
		Backend: cfg.Backend,
		Anthropic: anthropic.Config{
			APIKey:       cfg.APIKey,
			BaseURL:      cfg.BaseURL,
			Model:        cfg.Model,
			MaxTokens:    cfg.MaxTokens,
			SystemPrompt: cfg.SystemPrompt,
		},
		ClaudeCode: claudecode.Config{
			Command:      cfg.ClaudeCodeCommand,
			Model:        cfg.ClaudeCodeModel,
			SystemPrompt: cfg.ClaudeCodeSystemPrompt,
			Timeout:      cfg.ClaudeCodeTimeout,
			WorkingDir:   cfg.ClaudeCodeWorkingDir,
		},
	}

	// Build builtin tools based on config.
	// Note: tools are set after backend creation for Anthropic backend.
	tools := builtinTools(cfg.EnableCommands)
	backendCfg.Tools = tools

	backend, err := factory.NewBackend(backendCfg)
	if err != nil {
		// Fallback to Anthropic backend on error.
		b := anthropic.NewBackend(backendCfg.Anthropic)
		b.SetTools(tools)
		backend = b
	}

	return &Agent{
		agent:          agent.New(backend),
		backend:        backend,
		enableCommands: cfg.EnableCommands,
	}
}

// SetReminderStore attaches a reminder store so the set_reminder tool can schedule reminders.
func (a *Agent) SetReminderStore(store *ReminderStore) {
	a.reminderStore = store
	// Rebuild tools with reminder store.
	tools := builtinToolsWithReminders(a.enableCommands, store)
	a.agent.SetTools(tools)
}

// Chat sends a user message and runs the agentic loop until a final text response is produced.
func (a *Agent) Chat(userID, contextToken, text string) (string, error) {
	return a.agent.Chat(userID, contextToken, text)
}

// ChatWithCtx is like Chat but accepts a context for timeout/cancellation control.
func (a *Agent) ChatWithCtx(ctx context.Context, userID, contextToken, text string) (string, error) {
	return a.agent.ChatWithCtx(ctx, userID, contextToken, text)
}

// ResetConversation clears the conversation history for a user.
func (a *Agent) ResetConversation(userID string) {
	a.agent.ResetConversation(userID)
}

// GetConversationLength returns the number of messages in a user's conversation history.
func (a *Agent) GetConversationLength(userID string) int {
	return a.agent.GetConversationLength(userID)
}

// reminderStoreAdapter adapts ilink.ReminderStore to agent.ReminderStore interface.
type reminderStoreAdapter struct {
	store *ReminderStore
}

func (a *reminderStoreAdapter) AddReminder(userID, contextToken, message string, triggerAt time.Time) string {
	return a.store.AddReminder(userID, contextToken, message, triggerAt)
}

func (a *reminderStoreAdapter) ListReminders(userID string) []agent.Reminder {
	ilinkReminders := a.store.ListReminders(userID)
	result := make([]agent.Reminder, len(ilinkReminders))
	for i, r := range ilinkReminders {
		result[i] = agent.Reminder{
			ID:           r.ID,
			UserID:       r.UserID,
			ContextToken: r.ContextToken,
			Message:      r.Message,
			TriggerAt:    r.TriggerAt,
		}
	}
	return result
}

func (a *reminderStoreAdapter) RemoveReminder(userID, id string) bool {
	return a.store.RemoveReminder(userID, id)
}

// TruncateText truncates text to maxLen characters, appending "..." if truncated.
// Deprecated: Use agent.TruncateText instead.
func TruncateText(text string, maxLen int) string {
	return agent.TruncateText(text, maxLen)
}

func builtinTools(enableCommands bool) []agent.Tool {
	return agent.BuiltinTools(nil, enableCommands)
}

func builtinToolsWithReminders(enableCommands bool, store *ReminderStore) []agent.Tool {
	if store == nil {
		return agent.BuiltinTools(nil, enableCommands)
	}
	adapter := &reminderStoreAdapter{store: store}
	return agent.BuiltinTools(adapter, enableCommands)
}
