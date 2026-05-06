package anthropic

import (
	"os"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/whyy1/WeChat-iLink-Go/agent"
)

func TestNewBackendDefaults(t *testing.T) {
	// Save and clear env vars to test defaults.
	oldAPIKey := os.Getenv("ANTHROPIC_API_KEY")
	oldAuthToken := os.Getenv("ANTHROPIC_AUTH_TOKEN")
	oldModel := os.Getenv("ANTHROPIC_MODEL")
	oldBaseURL := os.Getenv("ANTHROPIC_BASE_URL")
	os.Unsetenv("ANTHROPIC_API_KEY")
	os.Unsetenv("ANTHROPIC_AUTH_TOKEN")
	os.Unsetenv("ANTHROPIC_MODEL")
	os.Unsetenv("ANTHROPIC_BASE_URL")
	defer func() {
		os.Setenv("ANTHROPIC_API_KEY", oldAPIKey)
		os.Setenv("ANTHROPIC_AUTH_TOKEN", oldAuthToken)
		os.Setenv("ANTHROPIC_MODEL", oldModel)
		os.Setenv("ANTHROPIC_BASE_URL", oldBaseURL)
	}()

	b := NewBackend(Config{APIKey: "test-key"})
	if b == nil {
		t.Fatal("NewBackend should not return nil")
	}
	if b.Model() != string(DefaultModel) {
		t.Fatalf("Model: got %q, want %q", b.Model(), DefaultModel)
	}
	if b.MaxTokens() != DefaultMaxTokens {
		t.Fatalf("MaxTokens: got %d, want %d", b.MaxTokens(), DefaultMaxTokens)
	}
	if b.SystemPrompt() != DefaultSystemPrompt {
		t.Fatalf("SystemPrompt: got %q, want %q", b.SystemPrompt(), DefaultSystemPrompt)
	}
}

func TestNewBackendCustomConfig(t *testing.T) {
	// Save and clear env vars to test config values.
	oldAPIKey := os.Getenv("ANTHROPIC_API_KEY")
	oldModel := os.Getenv("ANTHROPIC_MODEL")
	oldBaseURL := os.Getenv("ANTHROPIC_BASE_URL")
	os.Unsetenv("ANTHROPIC_API_KEY")
	os.Unsetenv("ANTHROPIC_MODEL")
	os.Unsetenv("ANTHROPIC_BASE_URL")
	defer func() {
		os.Setenv("ANTHROPIC_API_KEY", oldAPIKey)
		os.Setenv("ANTHROPIC_MODEL", oldModel)
		os.Setenv("ANTHROPIC_BASE_URL", oldBaseURL)
	}()

	b := NewBackend(Config{
		APIKey:       "custom-key",
		Model:        "claude-3-haiku-20240307",
		MaxTokens:    1024,
		SystemPrompt: "custom prompt",
	})
	if b.Model() != "claude-3-haiku-20240307" {
		t.Fatalf("Model: got %q", b.Model())
	}
	if b.MaxTokens() != 1024 {
		t.Fatalf("MaxTokens: got %d", b.MaxTokens())
	}
	if b.SystemPrompt() != "custom prompt" {
		t.Fatalf("SystemPrompt: got %q", b.SystemPrompt())
	}
}

func TestNewBackendEnvVars(t *testing.T) {
	// Save and clear env vars first.
	oldAPIKey := os.Getenv("ANTHROPIC_API_KEY")
	oldAuthToken := os.Getenv("ANTHROPIC_AUTH_TOKEN")
	oldModel := os.Getenv("ANTHROPIC_MODEL")
	oldBaseURL := os.Getenv("ANTHROPIC_BASE_URL")
	os.Unsetenv("ANTHROPIC_API_KEY")
	os.Unsetenv("ANTHROPIC_AUTH_TOKEN")
	os.Unsetenv("ANTHROPIC_MODEL")
	os.Unsetenv("ANTHROPIC_BASE_URL")
	defer func() {
		os.Setenv("ANTHROPIC_API_KEY", oldAPIKey)
		os.Setenv("ANTHROPIC_AUTH_TOKEN", oldAuthToken)
		os.Setenv("ANTHROPIC_MODEL", oldModel)
		os.Setenv("ANTHROPIC_BASE_URL", oldBaseURL)
	}()

	os.Setenv("ANTHROPIC_API_KEY", "env-key")
	os.Setenv("ANTHROPIC_MODEL", "env-model")
	os.Setenv("ANTHROPIC_BASE_URL", "https://env.example.com")

	b := NewBackend(Config{})
	if b.Model() != "env-model" {
		t.Fatalf("Model: got %q, want %q", b.Model(), "env-model")
	}
}

func TestNewBackendAuthTokenFallback(t *testing.T) {
	// Save and clear env vars first.
	oldAPIKey := os.Getenv("ANTHROPIC_API_KEY")
	oldAuthToken := os.Getenv("ANTHROPIC_AUTH_TOKEN")
	os.Unsetenv("ANTHROPIC_API_KEY")
	os.Unsetenv("ANTHROPIC_AUTH_TOKEN")
	defer func() {
		os.Setenv("ANTHROPIC_API_KEY", oldAPIKey)
		os.Setenv("ANTHROPIC_AUTH_TOKEN", oldAuthToken)
	}()

	os.Setenv("ANTHROPIC_AUTH_TOKEN", "auth-token-fallback")

	b := NewBackend(Config{})
	if b == nil {
		t.Fatal("NewBackend should not return nil")
	}
}

func TestBackendSetTools(t *testing.T) {
	b := NewBackend(Config{APIKey: "test"})
	tools := []agent.Tool{
		{Name: "tool1", Description: "Tool 1"},
		{Name: "tool2", Description: "Tool 2"},
	}
	b.SetTools(tools)

	got := b.Tools()
	if len(got) != 2 {
		t.Fatalf("Tools: got %d, want 2", len(got))
	}
	if got[0].Name != "tool1" {
		t.Fatalf("tool[0].Name: got %q, want %q", got[0].Name, "tool1")
	}
	if got[1].Name != "tool2" {
		t.Fatalf("tool[1].Name: got %q, want %q", got[1].Name, "tool2")
	}
}

func TestBackendToolsReturnsCopy(t *testing.T) {
	b := NewBackend(Config{APIKey: "test"})
	b.SetTools([]agent.Tool{{Name: "tool1"}})

	got := b.Tools()
	got[0].Name = "modified"

	// Original should not be modified.
	original := b.Tools()
	if original[0].Name != "tool1" {
		t.Fatalf("Tools should return a copy, but original was modified")
	}
}

func TestBackendResetConversation(t *testing.T) {
	b := NewBackend(Config{APIKey: "test"})
	// Should not panic on reset of non-existent user.
	b.ResetConversation("user1")
	if b.GetConversationLength("user1") != 0 {
		t.Fatal("conversation length should be 0")
	}
}

func TestBuildTools(t *testing.T) {
	tools := []agent.Tool{
		{
			Name:        "test_tool",
			Description: "A test tool",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"param1": map[string]any{
						"type":        "string",
						"description": "A parameter",
					},
				},
				"required": []string{"param1"},
			},
		},
	}

	result := buildTools(tools)
	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}

	tool := result[0].OfTool
	if tool == nil {
		t.Fatal("OfTool should not be nil")
	}
	if tool.Name != "test_tool" {
		t.Fatalf("Name: got %q, want %q", tool.Name, "test_tool")
	}
	if !tool.Description.Valid() {
		t.Fatal("Description should be valid")
	}
	if tool.Description.Value != "A test tool" {
		t.Fatalf("Description: got %q, want %q", tool.Description.Value, "A test tool")
	}
}

func TestBuildToolsEmptySchema(t *testing.T) {
	tools := []agent.Tool{
		{
			Name:        "simple_tool",
			Description: "Simple tool",
			InputSchema: map[string]any{},
		},
	}

	result := buildTools(tools)
	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}
}

func TestSchemaProperties(t *testing.T) {
	t.Run("with properties", func(t *testing.T) {
		schema := map[string]any{
			"properties": map[string]any{
				"key": map[string]any{"type": "string"},
			},
		}
		props := schemaProperties(schema)
		if props == nil {
			t.Fatal("expected non-nil properties")
		}
		if _, ok := props["key"]; !ok {
			t.Fatal("expected 'key' property")
		}
	})

	t.Run("without properties", func(t *testing.T) {
		schema := map[string]any{}
		props := schemaProperties(schema)
		if props != nil {
			t.Fatal("expected nil properties")
		}
	})
}

func TestSchemaRequired(t *testing.T) {
	t.Run("string slice", func(t *testing.T) {
		schema := map[string]any{
			"required": []string{"a", "b"},
		}
		required := schemaRequired(schema)
		if len(required) != 2 {
			t.Fatalf("expected 2, got %d", len(required))
		}
	})

	t.Run("any slice", func(t *testing.T) {
		schema := map[string]any{
			"required": []any{"a", "b"},
		}
		required := schemaRequired(schema)
		if len(required) != 2 {
			t.Fatalf("expected 2, got %d", len(required))
		}
	})

	t.Run("nil", func(t *testing.T) {
		schema := map[string]any{}
		required := schemaRequired(schema)
		if required != nil {
			t.Fatal("expected nil")
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
	text := ExtractText(msg)
	if text != "Hello World" {
		t.Fatalf("got %q, want %q", text, "Hello World")
	}
}

func TestExtractTextEmpty(t *testing.T) {
	msg := &anthropic.Message{
		Content: []anthropic.ContentBlockUnion{},
	}
	text := ExtractText(msg)
	if text != "" {
		t.Fatalf("got %q, want empty", text)
	}
}

func TestDefaultConstants(t *testing.T) {
	if DefaultModel != anthropic.ModelClaudeSonnet4_6 {
		t.Fatalf("DefaultModel: got %q, want %q", DefaultModel, anthropic.ModelClaudeSonnet4_6)
	}
	if DefaultMaxTokens != 4096 {
		t.Fatalf("DefaultMaxTokens: got %d, want 4096", DefaultMaxTokens)
	}
	if DefaultSystemPrompt == "" {
		t.Fatal("DefaultSystemPrompt should not be empty")
	}
}
