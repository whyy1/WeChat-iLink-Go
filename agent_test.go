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
		if a.model != "claude- 3-haiku-20240307" {
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
	t.Run("without commands", func(t *testing.T) {
		a := NewAgent(AgentConfig{APIKey: "test"})
		tools := a.buildTools()
		if len(tools) != 1 {
			t.Fatalf("expected 1 tool, got %d", len(tools))
		}
		if tools[0].OfTool == nil {
			t.Fatal("expected OfTool to be set")
		}
		if tools[0].OfTool.Name != "get_current_time" {
			t.Fatalf("tool name: got %q", tools[0].OfTool.Name)
		}
	})

	t.Run("with commands", func(t *testing.T) {
		a := NewAgent(AgentConfig{APIKey: "test", EnableCommands: true})
		tools := a.buildTools()
		if len(tools) != 2 {
			t.Fatalf("expected 2 tools, got %d", len(tools))
		}
		names := map[string]bool{}
		for _, tool := range tools {
			if tool.OfTool != nil {
				names[tool.OfTool.Name] = true
			}
		}
		if !names["get_current_time"] {
			t.Fatal("missing get_current_time tool")
		}
		if !names["execute_command"] {
			t.Fatal("missing execute_command tool")
		}
	})
}

func TestExecuteTool(t *testing.T) {
	a := NewAgent(AgentConfig{APIKey: "test"})

	t.Run("get_current_time", func(t *testing.T) {
		result, isErr := a.executeTool("get_current_time", json.RawMessage(`{}`))
		if isErr {
			t.Fatalf("unexpected error: %s", result)
		}
		if result == "" {
			t.Fatal("result should not be empty")
		}
		// Should contain year
		if !strings.Contains(result, "20") {
			t.Fatalf("result should look like a datetime: %q", result)
		}
	})

	t.Run("execute_command echo", func(t *testing.T) {
		a2 := NewAgent(AgentConfig{APIKey: "test", EnableCommands: true})
		result, isErr := a2.executeTool("execute_command", json.RawMessage(`{"command":"echo hello"}`))
		if isErr {
			t.Fatalf("unexpected error: %s", result)
		}
		if !strings.Contains(result, "hello") {
			t.Fatalf("result should contain 'hello': %q", result)
		}
	})

	t.Run("execute_command blocked", func(t *testing.T) {
		a2 := NewAgent(AgentConfig{APIKey: "test", EnableCommands: true})
		result, isErr := a2.executeTool("execute_command", json.RawMessage(`{"command":"rm -rf /tmp/test"}`))
		if !isErr {
			t.Fatal("expected error for blocked command")
		}
		if !strings.Contains(result, "安全策略") {
			t.Fatalf("result should mention safety: %q", result)
		}
	})

	t.Run("unknown tool", func(t *testing.T) {
		result, isErr := a.executeTool("nonexistent", json.RawMessage(`{}`))
		if !isErr {
			t.Fatal("expected error for unknown tool")
		}
		if !strings.Contains(result, "unknown tool") {
			t.Fatalf("result should mention unknown tool: %q", result)
		}
	})

	t.Run("invalid input json", func(t *testing.T) {
		a2 := NewAgent(AgentConfig{APIKey: "test", EnableCommands: true})
		_, isErr := a2.executeTool("execute_command", json.RawMessage(`not json`))
		if !isErr {
			t.Fatal("expected error for invalid json")
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
