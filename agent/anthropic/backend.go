package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/whyy1/WeChat-iLink-Go/agent"
)

const (
	DefaultModel        = anthropic.ModelClaudeSonnet4_6
	DefaultMaxTokens    = 4096
	DefaultSystemPrompt = "你是一个有用的微信助手。请用简洁的中文回复。"
	maxHistoryMessages  = 40
	maxAgentIterations  = 20
)

type Config struct {
	APIKey       string
	BaseURL      string
	Model        string
	MaxTokens    int64
	SystemPrompt string
}

type Backend struct {
	client        *anthropic.Client
	model         string
	maxTokens     int64
	systemPrompt  string
	tools         []agent.Tool
	conversations map[string][]anthropic.MessageParam
	mu            sync.RWMutex
}

func NewBackend(cfg Config) *Backend {
	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_AUTH_TOKEN")
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
		model = string(DefaultModel)
	}

	maxTokens := cfg.MaxTokens
	if maxTokens == 0 {
		maxTokens = DefaultMaxTokens
	}

	systemPrompt := cfg.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = DefaultSystemPrompt
	}

	return &Backend{
		client:        &client,
		model:         model,
		maxTokens:     maxTokens,
		systemPrompt:  systemPrompt,
		conversations: make(map[string][]anthropic.MessageParam),
	}
}

func (b *Backend) SetTools(tools []agent.Tool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tools = append([]agent.Tool(nil), tools...)
}

func (b *Backend) ChatWithCtx(ctx context.Context, userID, contextToken, text string) (string, error) {
	b.mu.RLock()
	history := b.conversations[userID]
	historyCopy := make([]anthropic.MessageParam, len(history))
	copy(historyCopy, history)
	tools := append([]agent.Tool(nil), b.tools...)
	b.mu.RUnlock()

	working := append(historyCopy, anthropic.NewUserMessage(anthropic.NewTextBlock(text)))
	anthropicTools := buildTools(tools)

	for i := 0; i < maxAgentIterations; i++ {
		params := anthropic.MessageNewParams{
			Model:     b.model,
			MaxTokens: b.maxTokens,
			System: []anthropic.TextBlockParam{{
				Text:         b.systemPrompt,
				CacheControl: anthropic.NewCacheControlEphemeralParam(),
			}},
			Messages: working,
		}
		if len(anthropicTools) > 0 {
			params.Tools = anthropicTools
		}

		msg, err := b.client.Messages.New(ctx, params)
		if err != nil {
			return "", fmt.Errorf("claude api: %w", err)
		}

		if msg.StopReason != anthropic.StopReasonToolUse {
			working = append(working, msg.ToParam())
			b.mu.Lock()
			b.conversations[userID] = trimHistory(working, maxHistoryMessages)
			b.mu.Unlock()
			return ExtractText(msg), nil
		}

		toolResults := make([]anthropic.ContentBlockParamUnion, 0)
		for _, block := range msg.Content {
			if block.Type != "tool_use" {
				continue
			}
			toolUse := block.AsToolUse()
			result := executeTool(ctx, tools, agent.ToolCall{
				UserID:       userID,
				ContextToken: contextToken,
				Name:         toolUse.Name,
				Input:        toolUse.Input,
			})
			toolResults = append(toolResults, anthropic.NewToolResultBlock(toolUse.ID, result.Content, result.IsError))
		}

		working = append(working, msg.ToParam())
		working = append(working, anthropic.NewUserMessage(toolResults...))
	}

	return "", fmt.Errorf("agent loop exceeded max iterations")
}

func (b *Backend) ResetConversation(userID string) {
	b.mu.Lock()
	delete(b.conversations, userID)
	b.mu.Unlock()
}

func (b *Backend) GetConversationLength(userID string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.conversations[userID])
}

func (b *Backend) Model() string {
	return b.model
}

func (b *Backend) MaxTokens() int64 {
	return b.maxTokens
}

func (b *Backend) SystemPrompt() string {
	return b.systemPrompt
}

func (b *Backend) Tools() []agent.Tool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return append([]agent.Tool(nil), b.tools...)
}

func buildTools(tools []agent.Tool) []anthropic.ToolUnionParam {
	result := make([]anthropic.ToolUnionParam, 0, len(tools))
	for _, tool := range tools {
		result = append(result, anthropic.ToolUnionParam{OfTool: &anthropic.ToolParam{
			Name:        tool.Name,
			Description: anthropic.String(tool.Description),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: schemaProperties(tool.InputSchema),
				Required:   schemaRequired(tool.InputSchema),
			},
		}})
	}
	return result
}

func schemaProperties(schema map[string]any) map[string]any {
	if value, ok := schema["properties"].(map[string]any); ok {
		return value
	}
	return nil
}

func schemaRequired(schema map[string]any) []string {
	switch value := schema["required"].(type) {
	case []string:
		return append([]string(nil), value...)
	case []any:
		required := make([]string, 0, len(value))
		for _, item := range value {
			if text, ok := item.(string); ok {
				required = append(required, text)
			}
		}
		return required
	default:
		return nil
	}
}

func executeTool(ctx context.Context, tools []agent.Tool, call agent.ToolCall) agent.ToolResult {
	for _, tool := range tools {
		if tool.Name == call.Name {
			return tool.Handler(ctx, call)
		}
	}
	return agent.ToolResult{Content: fmt.Sprintf("unknown tool: %s", call.Name), IsError: true}
}

func trimHistory(history []anthropic.MessageParam, max int) []anthropic.MessageParam {
	if len(history) <= max {
		return history
	}
	trimmed := make([]anthropic.MessageParam, len(history)-max)
	copy(trimmed, history[max:])
	return trimmed
}

func ExtractText(msg *anthropic.Message) string {
	var text string
	for _, block := range msg.Content {
		if block.Type == "text" {
			text += block.Text
		}
	}
	return text
}

func RawJSON(input string) json.RawMessage {
	return json.RawMessage(input)
}
