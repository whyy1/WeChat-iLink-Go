package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	ilink "github.com/whyy1/WeChat-iLink-Go"
)

const tokenFileName = "bot_token.txt"

const (
	downloadDirName = "downloads"
	fileDirName     = "files"
	imageDirName    = "images"
	videoDirName    = "videos"
)

func exampleDir() string {
	wd, err := os.Getwd()
	if err == nil {
		if filepath.Base(wd) == "example" {
			return wd
		}
		candidate := filepath.Join(wd, "example")
		if info, statErr := os.Stat(candidate); statErr == nil && info.IsDir() {
			return candidate
		}
	}
	exe, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exe)
		if filepath.Base(exeDir) == "example" {
			return exeDir
		}
	}
	return "."
}

func tokenFilePath() string {
	return filepath.Join(exampleDir(), tokenFileName)
}

func downloadRoot() string {
	return filepath.Join(exampleDir(), downloadDirName)
}

func fileDownloadDir() string {
	return filepath.Join(downloadRoot(), fileDirName)
}

func imageDownloadDir() string {
	return filepath.Join(downloadRoot(), imageDirName)
}

func videoDownloadDir() string {
	return filepath.Join(downloadRoot(), videoDirName)
}

func loadToken() string {
	data, err := os.ReadFile(tokenFilePath())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func saveToken(token string) {
	if err := os.WriteFile(tokenFilePath(), []byte(token), 0600); err != nil {
		log.Printf("save token: %v", err)
	}
}

func login() string {
	client := ilink.NewClient("", ilink.WithDebug(true))

	qr, err := client.GetBotQRCodeSimple()
	if err != nil {
		log.Fatalf("get qr code: %v", err)
	}
	fmt.Printf("Scan with WeChat to log in:\n%s\n", qr.QRCodeURL)

	token, err := client.WaitForLoginSimple(qr.QRCode, 2*time.Second)
	if err != nil {
		log.Fatalf("login: %v", err)
	}
	saveToken(token)
	fmt.Printf("Login succeeded, bot token saved to %s\n", tokenFilePath())
	return token
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

func echoText(bot *ilink.Client, msg ilink.Message, item ilink.Item) error {
	if item.TextItem == nil {
		return errors.New("text item is nil")
	}
	return withTyping(bot, msg, func() error {
		return bot.SendTextSimple(msg.FromUserID, msg.ContextToken, item.TextItem.Text)
	})
}

func echoImage(bot *ilink.Client, msg ilink.Message, item ilink.Item) error {
	if item.ImageItem == nil || item.ImageItem.Media == nil {
		return errors.New("image item media is incomplete")
	}

	data, err := bot.DownloadReceivedMediaSimple(item)
	if err != nil {
		return fmt.Errorf("download image: %w", err)
	}
	localPath, err := saveDownloadedData(imageDownloadDir(), fmt.Sprintf("received-image-%d.png", time.Now().Unix()), data)
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

func echoFile(bot *ilink.Client, msg ilink.Message, item ilink.Item) error {
	if item.FileItem == nil || item.FileItem.Media == nil {
		return errors.New("file item media is incomplete")
	}

	data, err := bot.DownloadReceivedMediaSimple(item)
	if err != nil {
		return fmt.Errorf("download file: %w", err)
	}
	fileName := strings.TrimSpace(item.FileItem.FileName)
	localPath, err := saveDownloadedData(fileDownloadDir(), fileName, data)
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

func echoVideo(bot *ilink.Client, msg ilink.Message, item ilink.Item) error {
	if item.VideoItem == nil || item.VideoItem.Media == nil {
		return errors.New("video item media is incomplete")
	}

	data, err := bot.DownloadReceivedMediaSimple(item)
	if err != nil {
		return fmt.Errorf("download video: %w", err)
	}
	localPath, err := saveDownloadedData(videoDownloadDir(), fmt.Sprintf("received-video-%d.mp4", time.Now().Unix()), data)
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

func main() {
	token := loadToken()
	if token == "" {
		fmt.Println("No saved token found. Starting QR login...")
		token = login()
	} else {
		fmt.Printf("Using saved token: %s\n", token)
	}

	bot := ilink.NewClient(token, ilink.WithDebug(true))

	fmt.Println("Polling messages. Press Ctrl+C to stop.")
	fmt.Println("Echo behavior: text -> text, image -> image, file -> file, video -> video.")

	err := bot.PollSimple(func(msg ilink.Message) error {
		if msg.MessageType != ilink.MessageTypeUser {
			return nil
		}

		for _, item := range msg.ItemList {
			var err error
				switch item.Type {
			case ilink.ItemTypeText:
				err = echoText(bot, msg, item)
			case ilink.ItemTypeImage:
				err = echoImage(bot, msg, item)
			case ilink.ItemTypeFile:
				err = echoFile(bot, msg, item)
			case ilink.ItemTypeVideo:
				err = echoVideo(bot, msg, item)
			}

			if err != nil {
				log.Printf("echo item type=%d: %v", item.Type, err)
				if sendErr := bot.SendTextSimple(msg.FromUserID, msg.ContextToken, "Echo failed. Check logs."); sendErr != nil {
					log.Printf("send error notice: %v", sendErr)
				}
			}
		}
		return nil
	})

	if err != nil {
		log.Printf("poll error: %v", err)
		_ = os.Remove(tokenFilePath())
		log.Fatal("Token may be invalid. Local cache removed; please run again.")
	}
}
