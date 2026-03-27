# WeChat-iLink-Go

`WeChat-iLink-Go` 是一个面向 Go 的微信 iLink Bot API 客户端，封装了扫码登录、长轮询收消息、发送文本/图片/文件/视频、媒体加解密与上传、输入中状态等能力。

项目当前基于仓库内已实现的协议适配代码，而不是泛化的微信 SDK。

## 功能概览

- 扫码登录并获取 `bot_token`
- 长轮询接收消息
- 发送文本消息
- 上传并发送图片、文件、视频
- 下载并解密收到的媒体消息
- 发送“正在输入”状态
- 纯标准库实现 AES-128-ECB + PKCS7 媒体加解密

## 安装

```bash
go get github.com/whyy1/WeChat-iLink-Go
```

要求：

- Go `1.21+`

## 快速开始

### 1. 扫码登录

```go
package main

import (
	"fmt"
	"log"
	"time"

	ilink "github.com/whyy1/WeChat-iLink-Go"
)

func main() {
	c := ilink.NewClient("")

	qr, err := c.GetBotQRCode()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("请使用微信扫码登录：")
	fmt.Println(qr.QRCodeURL)

	token, err := c.WaitForLogin(qr.QRCode, 2*time.Second)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("bot_token:", token)
}
```

### 2. 接收并回复文本消息

```go
package main

import (
	"log"

	ilink "github.com/whyy1/WeChat-iLink-Go"
)

func main() {
	bot := ilink.NewClient("<your-bot-token>")

	err := bot.Poll(func(msg ilink.Message) error {
		if msg.MessageType != ilink.MessageTypeUser {
			return nil
		}

		for _, item := range msg.ItemList {
			if item.Type == ilink.ItemTypeText && item.TextItem != nil {
				return bot.SendText(
					msg.FromUserID,
					msg.ContextToken,
					"Echo: "+item.TextItem.Text,
				)
			}
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
}
```

## 核心 API

### Client

```go
func NewClient(botToken string) *Client
```

- 登录前传空字符串 `""`
- 登录成功后传 `bot_token`
- 可选开启调试日志：

```go
c := ilink.NewClient(token)
c.Debug = true
```

### 登录

```go
func (c *Client) GetBotQRCode() (*QRCodeResponse, error)
func (c *Client) GetQRCodeStatus(qrcode string) (*QRCodeStatus, error)
func (c *Client) WaitForLogin(qrcode string, interval time.Duration) (string, error)
```

`QRCodeStatus.Status` 的可能值：

- `wait`
- `confirmed`
- `expired`

### 收消息

```go
func (c *Client) GetUpdates(cursor string) (*UpdatesResponse, error)
func (c *Client) Poll(handler func(msg Message) error) error
```

说明：

- `GetUpdates("")` 可用于首次拉取
- 后续请求必须使用服务端返回的 `GetUpdatesBuf`
- `Poll` 已在内部自动维护 cursor
- 单次长轮询超时约为 `35s`

### 发消息

```go
func (c *Client) SendMessage(msg Message) error
func (c *Client) SendText(toUserID, contextToken, text string) error
func (c *Client) SendImage(toUserID, contextToken, cdnURL, aesKey string) error
func (c *Client) SendFile(toUserID, contextToken, cdnURL, aesKey, fileName string, fileSize int64) error
func (c *Client) SendVideo(toUserID, contextToken, cdnURL, aesKey string) error
```

注意：

- 回复用户时必须带回原消息中的 `ContextToken`
- `toUserID` 一般使用收到消息里的 `msg.FromUserID`

### 输入中状态

```go
func (c *Client) GetConfig(ilinkUserID, contextToken string) (*ConfigResponse, error)
func (c *Client) SendTyping(ilinkUserID, typingTicket string, status int) error
```

示例：

```go
cfg, err := bot.GetConfig(msg.FromUserID, msg.ContextToken)
if err != nil {
	return err
}

_ = bot.SendTyping(msg.FromUserID, cfg.TypingTicket, ilink.TypingStatusOn)
defer bot.SendTyping(msg.FromUserID, cfg.TypingTicket, ilink.TypingStatusOff)
```

状态常量：

- `ilink.TypingStatusOn`
- `ilink.TypingStatusOff`

## 媒体上传与下载

### 上传媒体

```go
func (c *Client) GetUploadURL(fileType int, fileSize int64) (*UploadURLResponse, error)
func (c *Client) UploadMedia(data []byte, fileType int) (*MediaInfo, error)
```

`UploadMedia` 会自动完成：

1. AES-128-ECB 加密原始内容
2. 获取预签名上传地址
3. PUT 上传到 CDN
4. 返回 `cdn_url` 与 `aes_key`

示例：

```go
media, err := bot.UploadMedia(fileBytes, ilink.ItemTypeImage)
if err != nil {
	return err
}

err = bot.SendImage(msg.FromUserID, msg.ContextToken, media.CDNUrl, media.AesKey)
```

### 下载媒体

```go
func (c *Client) DownloadMedia(cdnURL, aesKey string) ([]byte, error)
func (c *Client) DownloadReceivedMedia(item Item) ([]byte, error)
func EncryptMedia(data []byte) (encrypted []byte, aesKey string, err error)
func DecryptMedia(data []byte, aesKey string) ([]byte, error)
```

使用建议：

- 下载自己上传过的媒体，用 `DownloadMedia`
- 下载收到的图片/语音/文件/视频，用 `DownloadReceivedMedia`

## 主要类型与常量

### ItemType

- `ItemTypeText = 1`
- `ItemTypeImage = 2`
- `ItemTypeVoice = 3`
- `ItemTypeFile = 4`
- `ItemTypeVideo = 5`

### MessageType

- `MessageTypeUser = 1`
- `MessageTypeBot = 2`

### 其他常量

- `MessageStateNormal = 2`
- `ChannelVersion = "1.0.2"`

## 请求头与协议细节

客户端会自动附带以下请求头：

- `Content-Type: application/json`
- `AuthorizationType: ilink_bot_token`
- `X-WECHAT-UIN: base64(randomUint32)`
- `Authorization: Bearer {bot_token}`（登录后）

协议相关常量：

- Base URL: `https://ilinkai.weixin.qq.com`
- CDN: `https://novac2c.cdn.weixin.qq.com/c2c`

## 目录说明

- [`client.go`](/E:/y1_code/WeChat-iLink-Go/client.go): 基础客户端与请求封装
- [`login.go`](/E:/y1_code/WeChat-iLink-Go/login.go): 扫码登录
- [`updates.go`](/E:/y1_code/WeChat-iLink-Go/updates.go): 长轮询收消息
- [`send.go`](/E:/y1_code/WeChat-iLink-Go/send.go): 发消息
- [`media.go`](/E:/y1_code/WeChat-iLink-Go/media.go): 媒体上传、下载、加解密
- [`typing.go`](/E:/y1_code/WeChat-iLink-Go/typing.go): 输入中状态
- [`types.go`](/E:/y1_code/WeChat-iLink-Go/types.go): 协议类型定义
- [`example/main.go`](/E:/y1_code/WeChat-iLink-Go/example/main.go): 示例程序

## 当前限制

- 目前未提供会话状态持久化封装，`Poll` 重启后需要重新建立消费状态
- 仓库内示例程序带有本地硬编码 token 文件路径，更适合作者本地调试，不建议直接照搬
- API 与协议字段仍应以当前代码实现为准，旧文档中的部分字段和命名可能已经过时

## License

Apache-2.0
