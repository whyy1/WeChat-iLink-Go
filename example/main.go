package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	ilink "github.com/whyy1/WeChat-iLink-Go"
)

const tokenFile = "F:\\code\\WeChat-iLink-Go\\example\\bot_token.txt"

func loadToken() string {
	data, err := os.ReadFile(tokenFile)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func saveToken(token string) {
	if err := os.WriteFile(tokenFile, []byte(token), 0600); err != nil {
		log.Printf("save token: %v", err)
	}
}

func login() string {
	c := ilink.NewClient("")
	c.Debug = true
	qr, err := c.GetBotQRCode()
	if err != nil {
		log.Fatalf("get qr code: %v", err)
	}
	fmt.Printf("请用微信扫码登录:\n%s\n", qr.QRCodeURL)

	token, err := c.WaitForLogin(qr.QRCode, 2*time.Second)
	if err != nil {
		log.Fatalf("login: %v", err)
	}
	saveToken(token)
	fmt.Printf("登录成功! bot_token 已保存到 %s\n", tokenFile)
	return token
}

func main() {
	token := loadToken()
	if token == "" {
		fmt.Println("未找到已保存的 token，开始扫码登录...")
		token = login()
	} else {
		fmt.Printf("使用已保存的 token: %s\n", token)
	}

	bot := ilink.NewClient(token)
	bot.Debug = true
	fmt.Println("开始轮询消息 (Ctrl+C 退出)...")

	err := bot.Poll(func(msg ilink.Message) error {
		// 只处理用户发来的消息，跳过 bot 自身的回显消息（MessageType=2）
		if msg.MessageType != ilink.MessageTypeUser {
			return nil
		}
		for _, item := range msg.ItemList {
			if item.Type == ilink.ItemTypeText && item.TextItem != nil {
				fmt.Printf("[%s] 文本: %s\n", msg.FromUserID, item.TextItem.Text)
				cfg, err := bot.GetConfig(msg.FromUserID, msg.ContextToken)
				if err != nil {
					log.Printf("getconfig 失败: %v", err)
					continue
				}
				if err = bot.SendTyping(msg.FromUserID, cfg.TypingTicket, ilink.TypingStatusOn); err != nil {
					fmt.Println("sendtyping on err=", err)
				}
				time.Sleep(2 * time.Second)
				if err := bot.SendText(msg.FromUserID, msg.ContextToken, "Echo: "+item.TextItem.Text); err != nil {
					log.Printf("发送失败: %v", err)
				}
				_ = bot.SendTyping(msg.FromUserID, cfg.TypingTicket, ilink.TypingStatusOff)
			}
		}
		return nil
	})

	if err != nil {
		log.Printf("poll 错误: %v", err)
		os.Remove(tokenFile)
		log.Fatal("token 可能已失效，已删除缓存，请重新运行")
	}
}
