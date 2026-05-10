package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	ilink "github.com/whyy1/WeChat-iLink-Go"
	"github.com/whyy1/WeChat-iLink-Go/internal/config"
)

func newStdinReader() *bufio.Reader {
	return bufio.NewReader(os.Stdin)
}

const maxMessageLen = 2048

func imageDir(cfg *config.Config) string {
	return filepath.Join(config.DownloadDir(cfg), "images")
}

func fileDir(cfg *config.Config) string {
	return filepath.Join(config.DownloadDir(cfg), "files")
}

func videoDir(cfg *config.Config) string {
	return filepath.Join(config.DownloadDir(cfg), "videos")
}

func saveDownloadedData(dir, name string, data []byte) (string, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create download dir: %w", err)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = fmt.Sprintf("received-%d.bin", time.Now().Unix())
	}
	savePath := filepath.Join(dir, filepath.Base(name))
	if err := os.WriteFile(savePath, data, 0644); err != nil {
		return "", fmt.Errorf("save downloaded data: %w", err)
	}
	return savePath, nil
}

func withTyping(bot *ilink.Client, msg ilink.Message, fn func() error) error {
	cfg, err := bot.GetConfigSimple(msg.FromUserID, msg.ContextToken)
	if err != nil {
		return err
	}
	if err := bot.SendTypingSimple(msg.FromUserID, cfg.TypingTicket, ilink.TypingStatusOn); err != nil {
		log.Printf("send typing on: %v", err)
	}
	defer func() {
		if err := bot.SendTypingSimple(msg.FromUserID, cfg.TypingTicket, ilink.TypingStatusOff); err != nil {
			log.Printf("send typing off: %v", err)
		}
	}()
	return fn()
}

func echoImage(bot *ilink.Client, cfg *config.Config, msg ilink.Message, item ilink.Item) error {
	if item.ImageItem == nil || item.ImageItem.Media == nil {
		return errors.New("image item media is incomplete")
	}

	data, err := bot.DownloadReceivedMediaSimple(item)
	if err != nil {
		return fmt.Errorf("download image: %w", err)
	}
	localPath, err := saveDownloadedData(imageDir(cfg), fmt.Sprintf("received-image-%d.png", time.Now().Unix()), data)
	if err != nil {
		return err
	}
	log.Printf("image saved: %s", localPath)

	return withTyping(bot, msg, func() error {
		uploaded, err := bot.UploadMediaForUserSimple(msg.FromUserID, data, ilink.ItemTypeImage)
		if err != nil {
			return fmt.Errorf("upload image: %w", err)
		}
		return bot.SendImageSimple(msg.FromUserID, msg.ContextToken, uploaded.DownloadEncryptedQueryParam, uploaded.AesKey)
	})
}

func echoFile(bot *ilink.Client, cfg *config.Config, msg ilink.Message, item ilink.Item) error {
	if item.FileItem == nil || item.FileItem.Media == nil {
		return errors.New("file item media is incomplete")
	}

	data, err := bot.DownloadReceivedMediaSimple(item)
	if err != nil {
		return fmt.Errorf("download file: %w", err)
	}
	fileName := strings.TrimSpace(item.FileItem.FileName)
	localPath, err := saveDownloadedData(fileDir(cfg), fileName, data)
	if err != nil {
		return err
	}
	log.Printf("file saved: %s", localPath)

	fileSize := int64(len(data))
	if fileSize == 0 && strings.TrimSpace(item.FileItem.Len) != "" {
		if n, parseErr := strconv.ParseInt(strings.TrimSpace(item.FileItem.Len), 10, 64); parseErr == nil {
			fileSize = n
		}
	}
	if fileName == "" {
		fileName = filepath.Base(localPath)
	}

	return withTyping(bot, msg, func() error {
		uploaded, err := bot.UploadMediaForUserSimple(msg.FromUserID, data, ilink.ItemTypeFile)
		if err != nil {
			return fmt.Errorf("upload file: %w", err)
		}
		return bot.SendFileSimple(msg.FromUserID, msg.ContextToken, uploaded.DownloadEncryptedQueryParam, uploaded.AesKey, fileName, fileSize)
	})
}

func echoVideo(bot *ilink.Client, cfg *config.Config, msg ilink.Message, item ilink.Item) error {
	if item.VideoItem == nil || item.VideoItem.Media == nil {
		return errors.New("video item media is incomplete")
	}

	data, err := bot.DownloadReceivedMediaSimple(item)
	if err != nil {
		return fmt.Errorf("download video: %w", err)
	}
	localPath, err := saveDownloadedData(videoDir(cfg), fmt.Sprintf("received-video-%d.mp4", time.Now().Unix()), data)
	if err != nil {
		return err
	}
	log.Printf("video saved: %s", localPath)

	return withTyping(bot, msg, func() error {
		uploaded, err := bot.UploadMediaForUserSimple(msg.FromUserID, data, ilink.ItemTypeVideo)
		if err != nil {
			return fmt.Errorf("upload video: %w", err)
		}
		return bot.SendVideoSimple(msg.FromUserID, msg.ContextToken, uploaded.DownloadEncryptedQueryParam, uploaded.AesKey)
	})
}

// ensureToken returns the bot token, triggering QR login if not saved.
func ensureToken(cfg *config.Config, configPath string) (string, error) {
	if cfg.BotToken != "" {
		return cfg.BotToken, nil
	}
	fmt.Println("No saved token found. Starting QR login...")
	client := ilink.NewClient("", ilink.WithDebug(true))
	qr, err := client.GetBotQRCodeSimple()
	if err != nil {
		return "", fmt.Errorf("get qr code: %w", err)
	}
	fmt.Println("Scan with WeChat to log in:")
	_ = printQRCode(qr.QRCodeURL)
	token, err := client.WaitForLoginSimple(qr.QRCode, 2*time.Second)
	if err != nil {
		return "", fmt.Errorf("login: %w", err)
	}
	cfg.BotToken = token
	if err := config.Save(cfg, configPath); err != nil {
		log.Printf("warning: save token: %v", err)
	}
	fmt.Printf("Login succeeded, token saved to %s\n", config.FilePath(configPath))
	return token, nil
}
