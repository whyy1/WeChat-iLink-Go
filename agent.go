package ilink

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const defaultModel = anthropic.ModelClaudeSonnet4_6

// DefaultAgentModel is the default model used by Agent.
const DefaultAgentModel = defaultModel

// AgentConfig holds configuration for creating a Claude Agent.
type AgentConfig struct {
	APIKey         string
	BaseURL        string
	Model          string
	MaxTokens      int64
	SystemPrompt   string
	EnableCommands bool
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
	// per-call state set before executeTool
	currentUserID       string
	currentContextToken string
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
	// Store per-call state for tool execution
	a.currentUserID = userID
	a.currentContextToken = contextToken

	history := a.conversations[userID]
	history = append(history, anthropic.NewUserMessage(
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
			Messages: history,
		}
		if len(tools) > 0 {
			params.Tools = tools
		}

		msg, err := a.client.Messages.New(ctx, params)
		if err != nil {
			return "", fmt.Errorf("claude api: %w", err)
		}

		if msg.StopReason != anthropic.StopReasonToolUse {
			history = append(history, msg.ToParam())
			a.conversations[userID] = history
			return extractText(msg), nil
		}

		var toolResults []anthropic.ContentBlockParamUnion
		for _, block := range msg.Content {
			if block.Type != "tool_use" {
				continue
			}
			toolUse := block.AsToolUse()
			result, isErr := a.executeTool(toolUse.Name, toolUse.Input)
			toolResults = append(toolResults, anthropic.NewToolResultBlock(toolUse.ID, result, isErr))
		}

		history = append(history, msg.ToParam())
		history = append(history, anthropic.NewUserMessage(toolResults...))
	}

	return "", fmt.Errorf("agent loop exceeded max iterations")
}

// ResetConversation clears the conversation history for a user.
func (a *Agent) ResetConversation(userID string) {
	delete(a.conversations, userID)
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
	}

	if a.enableCommands {
		tools = append(tools, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        "execute_command",
				Description: anthropic.String("执行 shell 命令并返回输出。仅允许只读/安全命令，禁止删除、修改等危险操作。"),
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

func (a *Agent) executeTool(name string, input json.RawMessage) (string, bool) {
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
		id := a.reminderStore.AddReminder(a.currentUserID, a.currentContextToken, args.Message, triggerAt)
		return fmt.Sprintf("已设置提醒：%s（%d分钟后，ID: %s）", args.Message, args.Minutes, id), false
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

func runCommand(cmdStr string) (string, bool) {
	blocked := []string{"rm ", "del ", "format ", "mkfs.", "dd ", "shutdown", "reboot", "> /", "rmdir "}
	for _, b := range blocked {
		if len(cmdStr) >= len(b) && cmdStr[:len(b)] == b {
			return "该命令被安全策略阻止", true
		}
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", cmdStr)
	} else {
		cmd = exec.Command("sh", "-c", cmdStr)
	}

	out, err := cmd.CombinedOutput()
	result := string(out)
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
