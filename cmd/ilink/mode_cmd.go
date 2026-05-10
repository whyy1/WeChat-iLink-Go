package main

import (
	"fmt"
	"log"
	"strings"

	ilink "github.com/whyy1/WeChat-iLink-Go"
	"github.com/whyy1/WeChat-iLink-Go/agent"
	"github.com/whyy1/WeChat-iLink-Go/internal/config"
)

func runCmdMode(configPath string) {
	cfg := config.ResolveWithDefaults(configPath)

	token, err := ensureToken(cfg, configPath)
	if err != nil {
		log.Fatalf("failed to get bot token: %v", err)
	}

	bot := ilink.NewClient(token, ilink.WithDebug(true))

	botID := config.ExtractBotID(token)
	fmt.Println("=== iLink Bot (Cmd Mode) ===")
	fmt.Printf("Bot ID: %s\n", botID)
	fmt.Println("Text -> Echo | Image/File/Video -> pass-through")
	fmt.Println("Waiting for messages...")

	err = bot.PollSimple(func(msg ilink.Message) error {
		if msg.MessageType != ilink.MessageTypeUser {
			return nil
		}

		for _, item := range msg.ItemList {
			var handlerErr error
			switch item.Type {
			case ilink.ItemTypeText:
				handlerErr = handleCmdText(bot, msg, item)
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

func handleCmdText(bot *ilink.Client, msg ilink.Message, item ilink.Item) error {
	if item.TextItem == nil {
		return nil
	}
	text := item.TextItem.Text

	if strings.TrimSpace(text) == "/reset" {
		return bot.SendTextSimple(msg.FromUserID, msg.ContextToken, "Cmd mode: no conversation to reset.")
	}

	reply := "[Echo] " + text
	return bot.SendTextSimple(msg.FromUserID, msg.ContextToken, agent.TruncateText(reply, maxMessageLen))
}
