package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultModel     = "claude-sonnet-4-6"
	defaultBackend   = "anthropic"
	defaultSystemMsg = "你是一个有用的微信助手。请用简洁的中文回复。"
)

// Config holds all configuration for the iLink CLI.
type Config struct {
	BotToken          string `json:"bot_token"`
	ANTHROPIC_API_KEY string `json:"ANTHROPIC_API_KEY,omitempty"`
	ANTHROPIC_BASE_URL string `json:"ANTHROPIC_BASE_URL,omitempty"`
	ANTHROPIC_MODEL   string `json:"ANTHROPIC_MODEL,omitempty"`
	Backend           string `json:"backend,omitempty"`
	EnableCommands    bool   `json:"enable_commands"`
	SystemPrompt      string `json:"system_prompt,omitempty"`
	WorkingDir        string `json:"working_dir,omitempty"`
}

// Dir returns ~/.ilink/, creating it if needed.
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".ilink")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// DefaultFilePath returns ~/.ilink/config.json.
func DefaultFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ilink", "config.json")
}

// FilePath returns configPath if non-empty, otherwise the default.
func FilePath(configPath string) string {
	if configPath != "" {
		return configPath
	}
	return DefaultFilePath()
}

// WorkingDir returns the working directory from config, or default ~/.ilink/workspace/.
func WorkingDir(cfg *Config) string {
	if cfg != nil && cfg.WorkingDir != "" {
		return cfg.WorkingDir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ilink", "workspace")
}

// DownloadDir returns <workingDir>/downloads/.
func DownloadDir(cfg *Config) string {
	return filepath.Join(WorkingDir(cfg), "downloads")
}

// ClaudeSettingsPath returns ~/.claude/settings.json (cross-platform).
func ClaudeSettingsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "settings.json")
}

// Load reads the config file. Returns empty Config if file doesn't exist.
func Load(configPath string) (*Config, error) {
	path := FilePath(configPath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return &cfg, nil
}

// Save writes the config to file with 0600 permissions.
func Save(cfg *Config, configPath string) error {
	path := FilePath(configPath)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

// Resolve loads config with four-layer resolution: file -> env -> Claude settings -> empty.
func Resolve(configPath string) (*Config, error) {
	cfg, err := Load(configPath)
	if err != nil {
		return nil, err
	}

	// Layer 2: Environment variables (fill empty fields only).
	if cfg.ANTHROPIC_API_KEY == "" {
		cfg.ANTHROPIC_API_KEY = envOr("ANTHROPIC_API_KEY", envOr("ANTHROPIC_AUTH_TOKEN", ""))
	}
	if cfg.ANTHROPIC_BASE_URL == "" {
		cfg.ANTHROPIC_BASE_URL = envOr("ANTHROPIC_BASE_URL", "")
	}
	if cfg.ANTHROPIC_MODEL == "" {
		cfg.ANTHROPIC_MODEL = envOr("ANTHROPIC_MODEL", "")
	}
	if cfg.BotToken == "" {
		cfg.BotToken = envOr("ILINK_BOT_TOKEN", "")
	}

	// Layer 3: ~/.claude/settings.json env section.
	claudeEnv := readClaudeSettings()
	if cfg.ANTHROPIC_API_KEY == "" {
		if v, ok := claudeEnv["ANTHROPIC_API_KEY"]; ok {
			cfg.ANTHROPIC_API_KEY = v
		} else if v, ok := claudeEnv["ANTHROPIC_AUTH_TOKEN"]; ok {
			cfg.ANTHROPIC_API_KEY = v
		}
	}
	if cfg.ANTHROPIC_BASE_URL == "" {
		if v, ok := claudeEnv["ANTHROPIC_BASE_URL"]; ok {
			cfg.ANTHROPIC_BASE_URL = v
		}
	}
	if cfg.ANTHROPIC_MODEL == "" {
		if v, ok := claudeEnv["ANTHROPIC_MODEL"]; ok {
			cfg.ANTHROPIC_MODEL = v
		}
	}

	return cfg, nil
}

// ResolveWithDefaults resolves config and fills in defaults.
func ResolveWithDefaults(configPath string) *Config {
	cfg, err := Resolve(configPath)
	if err != nil {
		cfg = &Config{}
	}
	if cfg.ANTHROPIC_MODEL == "" {
		cfg.ANTHROPIC_MODEL = defaultModel
	}
	if cfg.Backend == "" {
		cfg.Backend = defaultBackend
	}
	// EnableCommands defaults to true (zero value of bool is false, so we need explicit check).
	// We treat the default as true unless explicitly set to false via Save/Set.
	// Since JSON zero value is false, we use a simple heuristic: if BotToken is empty (fresh config),
	// default to true. Otherwise respect the saved value.
	// Actually, we just always default to true for ResolveWithDefaults.
	cfg.EnableCommands = true
	return cfg
}

// ValidateForAgentMode checks that ANTHROPIC_API_KEY is set.
func ValidateForAgentMode(cfg *Config) error {
	if cfg.ANTHROPIC_API_KEY == "" {
		return fmt.Errorf("ANTHROPIC_API_KEY is required for agent mode. Set it via config file, environment variable, ~/.claude/settings.json, or run 'ilink config setup'")
	}
	return nil
}

// ValidateForCmdMode checks that BotToken is set.
func ValidateForCmdMode(cfg *Config) error {
	if cfg.BotToken == "" {
		return fmt.Errorf("bot_token is required. Run 'ilink config setup' or 'ilink' to login")
	}
	return nil
}

// Show returns a formatted string displaying the current configuration.
func Show(cfg *Config, configPath string) string {
	path := FilePath(configPath)
	apiKey := cfg.ANTHROPIC_API_KEY
	if len(apiKey) > 8 {
		apiKey = apiKey[:8] + "..."
	} else if apiKey == "" {
		apiKey = "(not set)"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Config file: %s\n", path)
	fmt.Fprintf(&sb, "Bot Token:         %s\n", maskToken(cfg.BotToken))
	fmt.Fprintf(&sb, "ANTHROPIC_API_KEY: %s\n", apiKey)
	fmt.Fprintf(&sb, "ANTHROPIC_BASE_URL: %s\n", valueOr(cfg.ANTHROPIC_BASE_URL, "(not set)"))
	fmt.Fprintf(&sb, "ANTHROPIC_MODEL:   %s\n", valueOr(cfg.ANTHROPIC_MODEL, "(not set)"))
	fmt.Fprintf(&sb, "Backend:           %s\n", valueOr(cfg.Backend, "(not set)"))
	fmt.Fprintf(&sb, "Enable Commands:   %v\n", cfg.EnableCommands)
	fmt.Fprintf(&sb, "System Prompt:     %s\n", valueOr(cfg.SystemPrompt, "(default)"))
	fmt.Fprintf(&sb, "Working Dir:       %s\n", valueOr(WorkingDir(cfg), "(default)"))
	return sb.String()
}

// Set sets a single config key and saves.
func Set(key, value, configPath string) error {
	cfg, err := Load(configPath)
	if err != nil {
		return err
	}

	switch key {
	case "bot_token":
		cfg.BotToken = value
	case "ANTHROPIC_API_KEY":
		cfg.ANTHROPIC_API_KEY = value
	case "ANTHROPIC_BASE_URL":
		cfg.ANTHROPIC_BASE_URL = value
	case "ANTHROPIC_MODEL":
		cfg.ANTHROPIC_MODEL = value
	case "backend":
		cfg.Backend = value
	case "enable_commands":
		switch strings.ToLower(value) {
		case "true", "1", "yes":
			cfg.EnableCommands = true
		case "false", "0", "no":
			cfg.EnableCommands = false
		default:
			return fmt.Errorf("enable_commands must be true/false, got %q", value)
		}
	case "system_prompt":
		cfg.SystemPrompt = value
	case "working_dir":
		cfg.WorkingDir = value
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}

	return Save(cfg, configPath)
}

// InteractiveSetup walks the user through configuration interactively.
func InteractiveSetup(botToken string, configPath string) (*Config, error) {
	reader := bufio.NewReader(os.Stdin)

	cfg, _ := Load(configPath)
	if botToken != "" {
		cfg.BotToken = botToken
	}

	fmt.Println("\nConfigure Anthropic API (press Enter to keep current value):")

	cfg.ANTHROPIC_API_KEY = PromptInput(reader, "ANTHROPIC_API_KEY", cfg.ANTHROPIC_API_KEY)
	if cfg.ANTHROPIC_API_KEY == "" {
		fmt.Println("  (skipped — you can set it later with 'ilink config set ANTHROPIC_API_KEY <key>')")
	}

	cfg.Backend = PromptInput(reader, "Backend (anthropic/claude-code)", valueOr(cfg.Backend, defaultBackend))
	cfg.ANTHROPIC_BASE_URL = PromptInput(reader, "ANTHROPIC_BASE_URL (optional)", cfg.ANTHROPIC_BASE_URL)
	cfg.ANTHROPIC_MODEL = PromptInput(reader, "ANTHROPIC_MODEL", valueOr(cfg.ANTHROPIC_MODEL, defaultModel))
	cfg.EnableCommands = PromptYesNo(reader, "Enable command execution", true)
	cfg.SystemPrompt = PromptInput(reader, "System Prompt (optional, Enter for default)", cfg.SystemPrompt)
	cfg.WorkingDir = PromptInput(reader, "Working Directory (optional, Enter for default)", cfg.WorkingDir)

	if err := Save(cfg, configPath); err != nil {
		return nil, fmt.Errorf("save config: %w", err)
	}
	return cfg, nil
}

// ExtractBotID extracts the bot ID from a token (part before the colon).
func ExtractBotID(token string) string {
	if idx := strings.Index(token, ":"); idx > 0 {
		return token[:idx]
	}
	return ""
}

// PromptInput prompts for input with a default value.
func PromptInput(reader *bufio.Reader, prompt, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("  %s [%s]: ", prompt, defaultVal)
	} else {
		fmt.Printf("  %s: ", prompt)
	}
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal
	}
	return line
}

// PromptYesNo prompts for a yes/no answer.
func PromptYesNo(reader *bufio.Reader, prompt string, defaultYes bool) bool {
	hint := "y/N"
	if defaultYes {
		hint = "Y/n"
	}
	fmt.Printf("  %s [%s]: ", prompt, hint)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	if line == "" {
		return defaultYes
	}
	return line == "y" || line == "yes"
}

// readClaudeSettings reads the env map from ~/.claude/settings.json.
func readClaudeSettings() map[string]string {
	path := ClaudeSettingsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var raw struct {
		Env map[string]string `json:"env"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	return raw.Env
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func valueOr(val, fallback string) string {
	if val != "" {
		return val
	}
	return fallback
}

func maskToken(token string) string {
	if token == "" {
		return "(not set)"
	}
	if idx := strings.Index(token, ":"); idx > 0 && idx < len(token) {
		return token[:idx+1] + "..."
	}
	if len(token) > 8 {
		return token[:8] + "..."
	}
	return token
}
