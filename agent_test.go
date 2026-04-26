package ilink

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
)

func TestNewAgent(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		a := NewAgent(AgentConfig{APIKey: "test-key"})
		if a.model != string(defaultModel) {
			t.Fatalf("model: got %q, want %q", a.model, defaultModel)
		}
		if a.maxTokens != 4096 {
			t.Fatalf("maxTokens: got %d, want 4096", a.maxTokens)
		}
		if a.systemPrompt == "" {
			t.Fatal("systemPrompt should not be empty")
		}
		if a.enableCommands {
			t.Fatal("enableCommands should be false by default")
		}
	})

	t.Run("custom config", func(t *testing.T) {
		a := NewAgent(AgentConfig{
			APIKey:         "my-key",
			Model:          "claude-3-haiku-20240307",
			MaxTokens:      1024,
			SystemPrompt:   "custom prompt",
			EnableCommands: true,
		})
		if a.model != "claude-3-haiku-20240307" {
			t.Fatalf("model: got %q", a.model)
		}
		if a.maxTokens != 1024 {
			t.Fatalf("maxTokens: got %d", a.maxTokens)
		}
		if a.systemPrompt != "custom prompt" {
			t.Fatalf("systemPrompt: got %q", a.systemPrompt)
		}
		if !a.enableCommands {
			t.Fatal("enableCommands should be true")
		}
	})

	t.Run("reads env var", func(t *testing.T) {
		os.Setenv("ANTHROPIC_API_KEY", "env-key-123")
		defer os.Unsetenv("ANTHROPIC_API_KEY")
		a := NewAgent(AgentConfig{})
		if a.client == nil {
			t.Fatal("client should not be nil")
		}
	})
}

func TestAgentResetConversation(t *testing.T) {
	a := NewAgent(AgentConfig{APIKey: "test"})
	a.conversations["user1"] = []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("hello")),
	}
	a.ResetConversation("user1")
	if _, ok := a.conversations["user1"]; ok {
		t.Fatal("conversation should be deleted after reset")
	}
}

func TestBuildTools(t *testing.T) {
	t.Run("without commands or reminders", func(t *testing.T) {
		a := NewAgent(AgentConfig{APIKey: "test"})
		tools := a.buildTools()
		if len(tools) != 1 {
			t.Fatalf("expected 1 tool, got %d", len(tools))
		}
		if tools[0].OfTool.Name != "get_current_time" {
			t.Fatalf("tool name: got %q", tools[0].OfTool.Name)
		}
	})

	t.Run("with reminder store", func(t *testing.T) {
		a := NewAgent(AgentConfig{APIKey: "test"})
		a.SetReminderStore(NewReminderStore())
		tools := a.buildTools()
		if len(tools) != 4 {
			t.Fatalf("expected 4 tools (get_current_time + 3 reminder tools), got %d", len(tools))
		}
		names := toolNames(tools)
		if !names["get_current_time"] {
			t.Fatal("missing get_current_time tool")
		}
		if !names["set_reminder"] {
			t.Fatal("missing set_reminder tool")
		}
		if !names["list_reminders"] {
			t.Fatal("missing list_reminders tool")
		}
		if !names["cancel_reminder"] {
			t.Fatal("missing cancel_reminder tool")
		}
	})

	t.Run("with commands and reminders", func(t *testing.T) {
		a := NewAgent(AgentConfig{APIKey: "test", EnableCommands: true})
		a.SetReminderStore(NewReminderStore())
		tools := a.buildTools()
		if len(tools) != 5 {
			t.Fatalf("expected 5 tools, got %d", len(tools))
		}
		names := toolNames(tools)
		if !names["execute_command"] {
			t.Fatal("missing execute_command tool")
		}
		if !names["set_reminder"] {
			t.Fatal("missing set_reminder tool")
		}
		if !names["list_reminders"] {
			t.Fatal("missing list_reminders tool")
		}
		if !names["cancel_reminder"] {
			t.Fatal("missing cancel_reminder tool")
		}
		if !names["get_current_time"] {
			t.Fatal("missing get_current_time tool")
		}
	})
}

func toolNames(tools []anthropic.ToolUnionParam) map[string]bool {
	names := map[string]bool{}
	for _, tool := range tools {
		if tool.OfTool != nil {
			names[tool.OfTool.Name] = true
		}
	}
	return names
}

func TestExecuteTool(t *testing.T) {
	a := NewAgent(AgentConfig{APIKey: "test"})

	t.Run("get_current_time", func(t *testing.T) {
		cc := &chatContext{userID: "user1", contextToken: "ctx1"}
		result, isErr := a.executeTool(cc, "get_current_time", json.RawMessage(`{}`))
		if isErr {
			t.Fatalf("unexpected error: %s", result)
		}
		if result == "" {
			t.Fatal("result should not be empty")
		}
		if !strings.Contains(result, "20") {
			t.Fatalf("result should look like a datetime: %q", result)
		}
	})

	t.Run("execute_command echo", func(t *testing.T) {
		a2 := NewAgent(AgentConfig{APIKey: "test", EnableCommands: true})
		cc := &chatContext{userID: "user1", contextToken: "ctx1"}
		result, isErr := a2.executeTool(cc, "execute_command", json.RawMessage(`{"command":"echo hello"}`))
		if isErr {
			t.Fatalf("unexpected error: %s", result)
		}
		if !strings.Contains(result, "hello") {
			t.Fatalf("result should contain 'hello': %q", result)
		}
	})

	t.Run("execute_command blocked", func(t *testing.T) {
		a2 := NewAgent(AgentConfig{APIKey: "test", EnableCommands: true})
		cc := &chatContext{userID: "user1", contextToken: "ctx1"}
		result, isErr := a2.executeTool(cc, "execute_command", json.RawMessage(`{"command":"rm -rf /tmp/test"}`))
		if !isErr {
			t.Fatal("expected error for blocked command")
		}
		if !strings.Contains(result, "允许列表") {
			t.Fatalf("result should mention allowlist: %q", result)
		}
	})

	t.Run("unknown tool", func(t *testing.T) {
		cc := &chatContext{userID: "user1", contextToken: "ctx1"}
		result, isErr := a.executeTool(cc, "nonexistent", json.RawMessage(`{}`))
		if !isErr {
			t.Fatal("expected error for unknown tool")
		}
		if !strings.Contains(result, "unknown tool") {
			t.Fatalf("result should mention unknown tool: %q", result)
		}
	})

	t.Run("invalid input json", func(t *testing.T) {
		a2 := NewAgent(AgentConfig{APIKey: "test", EnableCommands: true})
		cc := &chatContext{userID: "user1", contextToken: "ctx1"}
		_, isErr := a2.executeTool(cc, "execute_command", json.RawMessage(`not json`))
		if !isErr {
			t.Fatal("expected error for invalid json")
		}
	})

	t.Run("set_reminder without store", func(t *testing.T) {
		a2 := NewAgent(AgentConfig{APIKey: "test"})
		cc := &chatContext{userID: "user1", contextToken: "ctx1"}
		_, isErr := a2.executeTool(cc, "set_reminder", json.RawMessage(`{"message":"test","minutes":5}`))
		if !isErr {
			t.Fatal("expected error when no reminder store")
		}
	})

	t.Run("set_reminder success", func(t *testing.T) {
		a2 := NewAgent(AgentConfig{APIKey: "test"})
		store := NewReminderStore()
		a2.SetReminderStore(store)
		cc := &chatContext{userID: "user1", contextToken: "ctx_tok"}
		result, isErr := a2.executeTool(cc, "set_reminder", json.RawMessage(`{"message":"开会","minutes":5}`))
		if isErr {
			t.Fatalf("unexpected error: %s", result)
		}
		if !strings.Contains(result, "开会") {
			t.Fatalf("result should contain reminder message: %q", result)
		}
		reminders := store.ListReminders("user1")
		if len(reminders) != 1 {
			t.Fatalf("expected 1 reminder, got %d", len(reminders))
		}
		if reminders[0].Message != "开会" {
			t.Fatalf("reminder message: got %q", reminders[0].Message)
		}
		if reminders[0].ContextToken != "ctx_tok" {
			t.Fatalf("reminder context_token: got %q", reminders[0].ContextToken)
		}
	})

	t.Run("set_reminder invalid minutes", func(t *testing.T) {
		a2 := NewAgent(AgentConfig{APIKey: "test"})
		a2.SetReminderStore(NewReminderStore())
		cc := &chatContext{userID: "user1", contextToken: "ctx1"}
		_, isErr := a2.executeTool(cc, "set_reminder", json.RawMessage(`{"message":"test","minutes":0}`))
		if !isErr {
			t.Fatal("expected error for zero minutes")
		}
	})

	t.Run("list_reminders empty", func(t *testing.T) {
		a2 := NewAgent(AgentConfig{APIKey: "test"})
		a2.SetReminderStore(NewReminderStore())
		cc := &chatContext{userID: "user1", contextToken: "ctx1"}
		result, isErr := a2.executeTool(cc, "list_reminders", json.RawMessage(`{}`))
		if isErr {
			t.Fatalf("unexpected error: %s", result)
		}
		if !strings.Contains(result, "没有") {
			t.Fatalf("result should mention no reminders: %q", result)
		}
	})

	t.Run("list_reminders with items", func(t *testing.T) {
		a2 := NewAgent(AgentConfig{APIKey: "test"})
		store := NewReminderStore()
		a2.SetReminderStore(store)
		cc := &chatContext{userID: "user1", contextToken: "ctx1"}
		a2.executeTool(cc, "set_reminder", json.RawMessage(`{"message":"测试","minutes":10}`))
		result, isErr := a2.executeTool(cc, "list_reminders", json.RawMessage(`{}`))
		if isErr {
			t.Fatalf("unexpected error: %s", result)
		}
		if !strings.Contains(result, "测试") {
			t.Fatalf("result should contain reminder: %q", result)
		}
	})

	t.Run("cancel_reminder", func(t *testing.T) {
		a2 := NewAgent(AgentConfig{APIKey: "test"})
		store := NewReminderStore()
		a2.SetReminderStore(store)
		cc := &chatContext{userID: "user1", contextToken: "ctx1"}
		setResult, _ := a2.executeTool(cc, "set_reminder", json.RawMessage(`{"message":"取消测试","minutes":5}`))
		// Extract reminder ID from result
		idStart := strings.Index(setResult, "ID: ")
		if idStart == -1 {
			t.Fatal("set_reminder result should contain 'ID: '")
		}
		reminderID := setResult[idStart+4:]
		// Trim trailing "）" or ")"
		for _, sep := range []string{"）", ")"} {
			if idx := strings.Index(reminderID, sep); idx != -1 {
				reminderID = reminderID[:idx]
			}
		}
		reminderID = strings.TrimSpace(reminderID)
		result, isErr := a2.executeTool(cc, "cancel_reminder", json.RawMessage(`{"reminder_id":"`+reminderID+`"}`))
		if isErr {
			t.Fatalf("unexpected error: %s", result)
		}
		if !strings.Contains(result, "已取消") {
			t.Fatalf("result should confirm cancellation: %q", result)
		}
	})

	t.Run("cancel_reminder not found", func(t *testing.T) {
		a2 := NewAgent(AgentConfig{APIKey: "test"})
		a2.SetReminderStore(NewReminderStore())
		cc := &chatContext{userID: "user1", contextToken: "ctx1"}
		result, isErr := a2.executeTool(cc, "cancel_reminder", json.RawMessage(`{"reminder_id":"nonexistent"}`))
		if !isErr {
			t.Fatal("expected error for nonexistent reminder")
		}
		if !strings.Contains(result, "未找到") {
			t.Fatalf("result should mention not found: %q", result)
		}
	})
}

func TestRunCommand(t *testing.T) {
	t.Run("blocked commands", func(t *testing.T) {
		blocked := []string{
			"rm -rf /",
			"del /f file",
			"format C:",
			"mkfs.ext4 /dev/sda",
			"dd if=/dev/zero of=/dev/sda",
			"shutdown -h now",
			"reboot",
			"rmdir /s /q dir",
		}
		for _, cmd := range blocked {
			_, isErr := runCommand(cmd)
			if !isErr {
				t.Fatalf("command %q should be blocked", cmd)
			}
		}
	})

	t.Run("safe command", func(t *testing.T) {
		result, isErr := runCommand("echo ok")
		if isErr {
			t.Fatalf("unexpected error: %s", result)
		}
		if !strings.Contains(result, "ok") {
			t.Fatalf("result should contain 'ok': %q", result)
		}
	})
}

func TestExtractText(t *testing.T) {
	msg := &anthropic.Message{
		Content: []anthropic.ContentBlockUnion{
			{Type: "text", Text: "Hello "},
			{Type: "text", Text: "World"},
			{Type: "tool_use"},
		},
	}
	text := extractText(msg)
	if text != "Hello World" {
		t.Fatalf("got %q, want %q", text, "Hello World")
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

func TestAgentConfigDefaults(t *testing.T) {
	cfg := AgentConfig{}
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
