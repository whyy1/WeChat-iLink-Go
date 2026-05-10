package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFile(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nonexistent.json"))
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.BotToken != "" {
		t.Fatalf("expected empty BotToken, got %q", cfg.BotToken)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := &Config{
		BotToken:          "test:token",
		ANTHROPIC_API_KEY: "sk-test123",
		ANTHROPIC_BASE_URL: "https://api.example.com",
		ANTHROPIC_MODEL:   "claude-sonnet-4-6",
		Backend:           "anthropic",
		EnableCommands:    true,
		SystemPrompt:      "test prompt",
		WorkingDir:        "/tmp/work",
	}

	if err := Save(cfg, path); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if loaded.BotToken != cfg.BotToken {
		t.Errorf("BotToken: got %q, want %q", loaded.BotToken, cfg.BotToken)
	}
	if loaded.ANTHROPIC_API_KEY != cfg.ANTHROPIC_API_KEY {
		t.Errorf("API_KEY: got %q, want %q", loaded.ANTHROPIC_API_KEY, cfg.ANTHROPIC_API_KEY)
	}
	if loaded.ANTHROPIC_MODEL != cfg.ANTHROPIC_MODEL {
		t.Errorf("Model: got %q, want %q", loaded.ANTHROPIC_MODEL, cfg.ANTHROPIC_MODEL)
	}
	if loaded.EnableCommands != cfg.EnableCommands {
		t.Errorf("EnableCommands: got %v, want %v", loaded.EnableCommands, cfg.EnableCommands)
	}
}

func TestValidateForAgentMode(t *testing.T) {
	cfg := &Config{ANTHROPIC_API_KEY: ""}
	if err := ValidateForAgentMode(cfg); err == nil {
		t.Fatal("expected error for empty API key")
	}

	cfg.ANTHROPIC_API_KEY = "sk-test"
	if err := ValidateForAgentMode(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateForCmdMode(t *testing.T) {
	cfg := &Config{BotToken: ""}
	if err := ValidateForCmdMode(cfg); err == nil {
		t.Fatal("expected error for empty bot token")
	}

	cfg.BotToken = "test:token"
	if err := ValidateForCmdMode(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestShowMasksAPIKey(t *testing.T) {
	cfg := &Config{
		BotToken:          "abc:xyz",
		ANTHROPIC_API_KEY: "sk-ant-1234567890abcdef",
		ANTHROPIC_MODEL:   "claude-sonnet-4-6",
	}
	output := Show(cfg, "")
	if !containsStr(output, "sk-ant-1") {
		t.Errorf("Show should contain first 8 chars of API key, got: %s", output)
	}
	if containsStr(output, "sk-ant-1234567890abcdef") {
		t.Error("Show should mask the full API key")
	}
}

func TestSetValidKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")

	if err := Set("ANTHROPIC_MODEL", "claude-opus-4-6", path); err != nil {
		t.Fatalf("set: %v", err)
	}

	cfg, _ := Load(path)
	if cfg.ANTHROPIC_MODEL != "claude-opus-4-6" {
		t.Errorf("model: got %q, want %q", cfg.ANTHROPIC_MODEL, "claude-opus-4-6")
	}
}

func TestSetInvalidKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := Set("nonexistent_key", "value", path); err == nil {
		t.Fatal("expected error for unknown key")
	}
}

func TestSetEnableCommands(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")

	if err := Set("enable_commands", "true", path); err != nil {
		t.Fatalf("set true: %v", err)
	}
	cfg, _ := Load(path)
	if !cfg.EnableCommands {
		t.Error("expected EnableCommands=true")
	}

	if err := Set("enable_commands", "false", path); err != nil {
		t.Fatalf("set false: %v", err)
	}
	cfg, _ = Load(path)
	if cfg.EnableCommands {
		t.Error("expected EnableCommands=false")
	}

	if err := Set("enable_commands", "invalid", path); err == nil {
		t.Error("expected error for invalid value")
	}
}

func TestExtractBotID(t *testing.T) {
	tests := []struct {
		token string
		want  string
	}{
		{"abc123:xyz789", "abc123"},
		{"nocolon", ""},
		{"", ""},
		{"id:", "id"},
	}
	for _, tt := range tests {
		got := ExtractBotID(tt.token)
		if got != tt.want {
			t.Errorf("ExtractBotID(%q) = %q, want %q", tt.token, got, tt.want)
		}
	}
}

func TestResolveWithDefaults(t *testing.T) {
	// Use a non-existent path so it starts empty.
	path := filepath.Join(t.TempDir(), "nope.json")
	os.Setenv("ANTHROPIC_API_KEY", "env-key")
	defer os.Unsetenv("ANTHROPIC_API_KEY")
	// Set ANTHROPIC_MODEL to default to override any ~/.claude/settings.json value.
	os.Setenv("ANTHROPIC_MODEL", defaultModel)
	defer os.Unsetenv("ANTHROPIC_MODEL")

	cfg := ResolveWithDefaults(path)
	if cfg.ANTHROPIC_API_KEY != "env-key" {
		t.Errorf("API key: got %q, want %q", cfg.ANTHROPIC_API_KEY, "env-key")
	}
	if cfg.ANTHROPIC_MODEL != defaultModel {
		t.Errorf("model: got %q, want %q", cfg.ANTHROPIC_MODEL, defaultModel)
	}
	if cfg.Backend != defaultBackend {
		t.Errorf("backend: got %q, want %q", cfg.Backend, defaultBackend)
	}
	if !cfg.EnableCommands {
		t.Error("expected EnableCommands=true by default")
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && searchStr(s, substr)
}

func searchStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
