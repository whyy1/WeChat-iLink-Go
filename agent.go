package ilink

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const defaultModel = anthropic.ModelClaudeSonnet4_6

// DefaultAgentModel is the default model used by Agent.
const DefaultAgentModel = defaultModel

const maxHistoryMessages = 40

// AgentConfig holds configuration for creating a Claude Agent.
type AgentConfig struct {
	APIKey         string
	BaseURL        string
	Model          string
	MaxTokens      int64
	SystemPrompt   string
	EnableCommands bool
}

// chatContext carries per-call state through the tool execution chain.
type chatContext struct {
	userID       string
	contextToken string
}

// Agent wraps the Anthropic Claude API client with an agentic tool-use loop.
type Agent struct {
	client         *anthropic.Client
	model          string
	maxTokens      int64
	systemPrompt   string
	enableCommands bool
	reminderStore  *ReminderStore
	conversations  map[string][]anthropic.MessageParam
	mu             sync.RWMutex
}

// NewAgent creates a new Claude Agent from the given config.
func NewAgent(cfg AgentConfig) *Agent {
	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}

	opts := []option.RequestOption{option.WithAPIKey(apiKey)}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = os.Getenv("ANTHROPIC_BASE_URL")
	}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}

	client := anthropic.NewClient(opts...)

	model := cfg.Model
	if model == "" {
		model = os.Getenv("ANTHROPIC_MODEL")
	}
	if model == "" {
		model = string(defaultModel)
	}

	maxTokens := cfg.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	systemPrompt := cfg.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = "你是一个有用的微信助手。请用简洁的中文回复。"
	}

	return &Agent{
		client:         &client,
		model:          model,
		maxTokens:      maxTokens,
		systemPrompt:   systemPrompt,
		enableCommands: cfg.EnableCommands,
		conversations:  make(map[string][]anthropic.MessageParam),
	}
}

// SetReminderStore attaches a reminder store so the set_reminder tool can schedule reminders.
func (a *Agent) SetReminderStore(store *ReminderStore) {
	a.reminderStore = store
}

// Chat sends a user message and runs the agentic loop until a final text response is produced.
func (a *Agent) Chat(userID, contextToken, text string) (string, error) {
	return a.ChatWithCtx(context.Background(), userID, contextToken, text)
}

// ChatWithCtx is like Chat but accepts a context for timeout/cancellation control.
func (a *Agent) ChatWithCtx(ctx context.Context, userID, contextToken, text string) (string, error) {
	cc := &chatContext{userID: userID, contextToken: contextToken}

	a.mu.Lock()
	history := a.conversations[userID]
	historyCopy := make([]anthropic.MessageParam, len(history))
	copy(historyCopy, history)
	a.mu.Unlock()

	working := append(historyCopy, anthropic.NewUserMessage(
		anthropic.NewTextBlock(text),
	))

	tools := a.buildTools()

	for i := 0; i < 20; i++ {
		params := anthropic.MessageNewParams{
			Model:     a.model,
			MaxTokens: a.maxTokens,
			System: []anthropic.TextBlockParam{
				{Text: a.systemPrompt},
			},
			Messages: working,
		}
		if len(tools) > 0 {
			params.Tools = tools
		}

		msg, err := a.client.Messages.New(ctx, params)
		if err != nil {
			return "", fmt.Errorf("claude api: %w", err)
		}

		if msg.StopReason != anthropic.StopReasonToolUse {
			working = append(working, msg.ToParam())
			trimmed := trimHistory(working, maxHistoryMessages)
			a.mu.Lock()
			a.conversations[userID] = trimmed
			a.mu.Unlock()
			return extractText(msg), nil
		}

		var toolResults []anthropic.ContentBlockParamUnion
		for _, block := range msg.Content {
			if block.Type != "tool_use" {
				continue
			}
			toolUse := block.AsToolUse()
			result, isErr := a.executeTool(cc, toolUse.Name, toolUse.Input)
			toolResults = append(toolResults, anthropic.NewToolResultBlock(toolUse.ID, result, isErr))
		}

		working = append(working, msg.ToParam())
		working = append(working, anthropic.NewUserMessage(toolResults...))
	}

	return "", fmt.Errorf("agent loop exceeded max iterations")
}

// ResetConversation clears the conversation history for a user.
func (a *Agent) ResetConversation(userID string) {
	a.mu.Lock()
	delete(a.conversations, userID)
	a.mu.Unlock()
}

// GetConversationLength returns the number of messages in a user's conversation history.
func (a *Agent) GetConversationLength(userID string) int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.conversations[userID])
}

func trimHistory(history []anthropic.MessageParam, max int) []anthropic.MessageParam {
	if len(history) <= max {
		return history
	}
	trimmed := make([]anthropic.MessageParam, len(history)-max)
	copy(trimmed, history[max:])
	return trimmed
}

func (a *Agent) buildTools() []anthropic.ToolUnionParam {
	var tools []anthropic.ToolUnionParam

	tools = append(tools, anthropic.ToolUnionParam{
		OfTool: &anthropic.ToolParam{
			Name:        "get_current_time",
			Description: anthropic.String("获取当前日期和时间"),
			InputSchema: anthropic.ToolInputSchemaParam{
				Type: "object",
			},
		},
	})

	if a.reminderStore != nil {
		tools = append(tools, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        "set_reminder",
				Description: anthropic.String("为用户设置定时提醒。在指定分钟后提醒用户某件事。"),
				InputSchema: anthropic.ToolInputSchemaParam{
					Type: "object",
					Properties: map[string]any{
						"message": map[string]any{
							"type":        "string",
							"description": "提醒的内容",
						},
						"minutes": map[string]any{
							"type":        "integer",
							"description": "几分钟后提醒",
						},
					},
					Required: []string{"message", "minutes"},
				},
			},
		})

		tools = append(tools, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        "list_reminders",
				Description: anthropic.String("列出用户当前所有待执行的提醒"),
				InputSchema: anthropic.ToolInputSchemaParam{
					Type: "object",
				},
			},
		})

		tools = append(tools, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        "cancel_reminder",
				Description: anthropic.String("取消一个已设置的提醒"),
				InputSchema: anthropic.ToolInputSchemaParam{
					Type: "object",
					Properties: map[string]any{
						"reminder_id": map[string]any{
							"type":        "string",
							"description": "要取消的提醒ID",
						},
					},
					Required: []string{"reminder_id"},
				},
			},
		})
	}

	if a.enableCommands {
		tools = append(tools, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        "execute_command",
				Description: anthropic.String("执行允许的 shell 命令并返回输出。仅允许安全命令。"),
				InputSchema: anthropic.ToolInputSchemaParam{
					Type: "object",
					Properties: map[string]any{
						"command": map[string]any{
							"type":        "string",
							"description": "要执行的命令",
						},
					},
					Required: []string{"command"},
				},
			},
		})
	}

	return tools
}

func (a *Agent) executeTool(cc *chatContext, name string, input json.RawMessage) (string, bool) {
	switch name {
	case "get_current_time":
		return time.Now().Format("2006-01-02 15:04:05 MST"), false
	case "set_reminder":
		var args struct {
			Message string `json:"message"`
			Minutes int    `json:"minutes"`
		}
		if err := json.Unmarshal(input, &args); err != nil {
			return fmt.Sprintf("parse input: %v", err), true
		}
		if a.reminderStore == nil {
			return "reminder store not configured", true
		}
		if args.Minutes <= 0 {
			return "minutes must be positive", true
		}
		triggerAt := time.Now().Add(time.Duration(args.Minutes) * time.Minute)
		id := a.reminderStore.AddReminder(cc.userID, cc.contextToken, args.Message, triggerAt)
		return fmt.Sprintf("已设置提醒：%s（%d分钟后，ID: %s）", args.Message, args.Minutes, id), false
	case "list_reminders":
		if a.reminderStore == nil {
			return "reminder store not configured", true
		}
		reminders := a.reminderStore.ListReminders(cc.userID)
		if len(reminders) == 0 {
			return "当前没有待执行的提醒", false
		}
		var sb strings.Builder
		for _, r := range reminders {
			fmt.Fprintf(&sb, "- [%s] %s（%s触发）\n", r.ID, r.Message, r.TriggerAt.Format("15:04"))
		}
		return sb.String(), false
	case "cancel_reminder":
		var args struct {
			ReminderID string `json:"reminder_id"`
		}
		if err := json.Unmarshal(input, &args); err != nil {
			return fmt.Sprintf("parse input: %v", err), true
		}
		if a.reminderStore == nil {
			return "reminder store not configured", true
		}
		if a.reminderStore.RemoveReminder(cc.userID, args.ReminderID) {
			return fmt.Sprintf("已取消提醒 %s", args.ReminderID), false
		}
		return fmt.Sprintf("未找到提醒 %s", args.ReminderID), true
	case "execute_command":
		var args struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal(input, &args); err != nil {
			return fmt.Sprintf("parse input: %v", err), true
		}
		return runCommand(args.Command)
	default:
		return fmt.Sprintf("unknown tool: %s", name), true
	}
}

var allowedCommands = []string{
	"echo", "date", "whoami", "hostname", "pwd", "ls", "dir",
	"cat", "head", "tail", "wc", "find", "grep", "sort", "uniq",
	"df", "du", "free", "uptime", "uname", "env", "printenv",
	"curl", "ping", "nslookup", "ipconfig", "ifconfig",
	"python", "python3", "node", "go version",
	"git status", "git log", "git diff", "git branch",
}

func runCommand(cmdStr string) (string, bool) {
	cmdStr = strings.TrimSpace(cmdStr)

	// Extract the base command for allowlist check
	baseCmd := cmdStr
	if idx := strings.IndexAny(cmdStr, " \t"); idx > 0 {
		baseCmd = cmdStr[:idx]
	}
	// Handle paths: extract the binary name
	if strings.Contains(baseCmd, "/") || strings.Contains(baseCmd, "\\") {
		baseCmd = baseCmd[strings.LastIndexAny(baseCmd, "/\\")+1:]
	}

	allowed := false
	for _, ac := range allowedCommands {
		if baseCmd == ac || strings.HasPrefix(cmdStr, ac+" ") {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Sprintf("命令 %q 不在允许列表中。允许的命令: %s", baseCmd, strings.Join(allowedCommands, ", ")), true
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", cmdStr)
	} else {
		cmd = exec.Command("sh", "-c", cmdStr)
	}

	out, err := cmd.CombinedOutput()
	result := string(out)
	// Truncate large output
	if len(result) > 2000 {
		result = result[:2000] + "\n... (输出已截断)"
	}
	if err != nil {
		if result == "" {
			result = err.Error()
		}
		return result, true
	}
	return result, false
}

func extractText(msg *anthropic.Message) string {
	var text string
	for _, block := range msg.Content {
		if block.Type == "text" {
			text += block.Text
		}
	}
	return text
}

// TruncateText truncates text to maxLen characters, appending "..." if truncated.
func TruncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}
