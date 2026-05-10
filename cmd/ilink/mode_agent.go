package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	ilink "github.com/whyy1/WeChat-iLink-Go"
	"github.com/whyy1/WeChat-iLink-Go/agent"
	"github.com/whyy1/WeChat-iLink-Go/internal/config"
)

func runAgentMode(configPath string) {
	cfg := config.ResolveWithDefaults(configPath)

	// Prompt for API key if missing.
	if cfg.ANTHROPIC_API_KEY == "" {
		reader := newStdinReader()
		cfg.ANTHROPIC_API_KEY = config.PromptInput(reader, "ANTHROPIC_API_KEY (required for agent mode)", "")
		if strings.TrimSpace(cfg.ANTHROPIC_API_KEY) == "" {
			log.Fatal("API key is required for agent mode. Set via config, env var, ~/.claude/settings.json, or 'ilink config setup'")
		}
		if err := config.Save(cfg, configPath); err != nil {
			log.Printf("warning: failed to save config: %v", err)
		}
	}

	if err := config.ValidateForAgentMode(cfg); err != nil {
		log.Fatalf("invalid agent config: %v", err)
	}

	token, err := ensureToken(cfg, configPath)
	if err != nil {
		log.Fatalf("failed to get bot token: %v", err)
	}

	workDir := config.WorkingDir(cfg)
	if err := os.MkdirAll(workDir, 0755); err != nil {
		log.Fatalf("failed to create working directory %s: %v", workDir, err)
	}

	bot := ilink.NewClient(token, ilink.WithDebug(true))

	agentCfg := ilink.AgentConfig{
		Backend:        cfg.Backend,
		APIKey:         cfg.ANTHROPIC_API_KEY,
		BaseURL:        cfg.ANTHROPIC_BASE_URL,
		Model:          cfg.ANTHROPIC_MODEL,
		EnableCommands: cfg.EnableCommands,
		SystemPrompt:   cfg.SystemPrompt,
		WorkDir:        workDir,
	}
	a := ilink.NewAgent(agentCfg)

	// Set up reminder store.
	store := ilink.NewReminderStore()
	a.SetReminderStore(store)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go store.Start(ctx, bot)

	botID := config.ExtractBotID(token)
	modelName := cfg.ANTHROPIC_MODEL
	if modelName == "" {
		modelName = ilink.DefaultAgentModel
	}

	fmt.Println("=== iLink Agent Mode ===")
	fmt.Printf("Bot ID:      %s\n", botID)
	fmt.Printf("Model:       %s\n", modelName)
	fmt.Printf("Backend:     %s\n", cfg.Backend)
	fmt.Printf("Working Dir: %s\n", workDir)
	fmt.Println("Text -> Claude Agent | Image/File/Video -> echo | /reset -> clear history")
	fmt.Println("Waiting for messages...")

	err = bot.PollSimple(func(msg ilink.Message) error {
		if msg.MessageType != ilink.MessageTypeUser {
			return nil
		}

		for _, item := range msg.ItemList {
			var handlerErr error
			switch item.Type {
			case ilink.ItemTypeText:
				handlerErr = handleAgentText(bot, a, cfg, msg, item)
			case ilink.ItemTypeImage:
				handlerErr = echoImage(bot, cfg, msg, item)
			case ilink.ItemTypeFile:
				handlerErr = echoFile(bot, cfg, msg, item)
			case ilink.ItemTypeVideo:
				handlerErr = echoVideo(bot, cfg, msg, item)
			}

			if handlerErr != nil {
				log.Printf("handle item type=%d: %v", item.Type, handlerErr)
				errMsg := agent.TruncateText(fmt.Sprintf("处理失败: %v", handlerErr), maxMessageLen)
				if sendErr := bot.SendTextSimple(msg.FromUserID, msg.ContextToken, errMsg); sendErr != nil {
					log.Printf("send error notice: %v", sendErr)
				}
			}
		}
		return nil
	})

	if err != nil {
		log.Printf("poll error: %v", err)
		cfg.BotToken = ""
		_ = config.Save(cfg, configPath)
		log.Fatal("Token may be invalid. Config cache cleared; please run again.")
	}
}

func handleAgentText(bot *ilink.Client, a *ilink.Agent, cfg *config.Config, msg ilink.Message, item ilink.Item) error {
	if item.TextItem == nil {
		return errors.New("text item is nil")
	}
	text := item.TextItem.Text

	if strings.TrimSpace(text) == "/reset" {
		a.ResetConversation(msg.FromUserID)
		return bot.SendTextSimple(msg.FromUserID, msg.ContextToken, "对话已重置。")
	}

	return withTyping(bot, msg, func() error {
		reply, err := a.Chat(msg.FromUserID, msg.ContextToken, text)
		if err != nil {
			return fmt.Errorf("agent chat: %w", err)
		}
		return bot.SendTextSimple(msg.FromUserID, msg.ContextToken, agent.TruncateText(reply, maxMessageLen))
	})
}
