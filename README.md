# WeChat-iLink-Go

`WeChat-iLink-Go` 是一个 Go 版微信 iLink Bot API 客户端，支持扫码登录、长轮询收消息、文本/图片/文件/视频发送、媒体下载解密、输入中状态等能力。

当前实现已对齐仓库内 `weixin.md` 与 `openclaw-weixin-2.0.1` 中实际使用的媒体流程，媒体发送优先使用协议原生的 `encrypt_query_param + aes_key` 结构。

## 功能

- 扫码登录，获取并复用 `bot_token`
- 长轮询接收消息
- 发送文本消息
- 发送图片、文件、视频
- 下载并解密收到的图片、文件、语音、视频
- 发送 typing 状态
- 使用标准库实现 AES-128-ECB + PKCS7

## 安装

```bash
go get github.com/whyy1/WeChat-iLink-Go
```

要求：

- Go `1.21+`

## 快速开始

### 登录

```go
package main

import (
	"fmt"
	"log"
	"time"

	ilink "github.com/whyy1/WeChat-iLink-Go"
)

func main() {
	client := ilink.NewClient("")

	qr, err := client.GetBotQRCode()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(qr.QRCodeURL)

	token, err := client.WaitForLogin(qr.QRCode, 2*time.Second)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("bot_token:", token)
}
```

### 文本回显

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
				return bot.SendText(msg.FromUserID, msg.ContextToken, item.TextItem.Text)
			}
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
}
```

## 示例程序

示例见 [`example/main.go`](/E:/y1_code/WeChat-iLink-Go/example/main.go)。

当前示例行为：

- 文本消息：直接回显文本
- 图片消息：先下载到 `example/downloads/images/`，再重新上传并发回
- 文件消息：先下载到 `example/downloads/files/`，再重新上传并发回
- token 缓存在 `example/bot_token.txt`

## 核心 API

### Client

```go
func NewClient(botToken string) *Client
```

- 登录前传空字符串
- 登录成功后传 `bot_token`
- 可开启调试日志：

```go
bot := ilink.NewClient(token)
bot.Debug = true
```

### 登录

```go
func (c *Client) GetBotQRCode() (*QRCodeResponse, error)
func (c *Client) GetQRCodeStatus(qrcode string) (*QRCodeStatus, error)
func (c *Client) WaitForLogin(qrcode string, interval time.Duration) (string, error)
```

二维码状态：

- `wait`
- `confirmed`
- `expired`

### 收消息

```go
func (c *Client) GetUpdates(cursor string) (*UpdatesResponse, error)
func (c *Client) Poll(handler func(msg Message) error) error
```

说明：

- 首次调用可传 `""`
- 后续应持续使用返回的 `GetUpdatesBuf`
- `Poll` 内部已经处理了 cursor

### 发送消息

```go
func (c *Client) SendMessage(msg Message) error
func (c *Client) SendText(toUserID, contextToken, text string) error
func (c *Client) SendImage(toUserID, contextToken, cdnURL, aesKey string) error
func (c *Client) SendFile(toUserID, contextToken, cdnURL, aesKey, fileName string, fileSize int64) error
func (c *Client) SendVideo(toUserID, contextToken, cdnURL, aesKey string) error
func (c *Client) SendImageRef(toUserID, contextToken, encryptQueryParam, aesKey string, midSize int) error
func (c *Client) SendFileRef(toUserID, contextToken, encryptQueryParam, aesKey, fileName string, fileSize int64) error
```

注意：

- 回复时必须带回原消息的 `ContextToken`
- 接收媒体再回发时，优先使用 `SendImageRef` / `SendFileRef`

### 输入中状态

```go
func (c *Client) GetConfig(ilinkUserID, contextToken string) (*ConfigResponse, error)
func (c *Client) SendTyping(ilinkUserID, typingTicket string, status int) error
```

常量：

- `ilink.TypingStatusOn`
- `ilink.TypingStatusOff`

## 媒体上传与下载

### 上传

```go
func (c *Client) GetUploadURL(req UploadURLRequest) (*UploadURLResponse, error)
func (c *Client) UploadMedia(data []byte, itemType int) (*MediaInfo, error)
func (c *Client) UploadMediaForUser(toUserID string, data []byte, itemType int) (*MediaInfo, error)
```

`UploadMediaForUser` 会执行以下流程：

1. 计算原文件大小与 MD5
2. 生成 AES key
3. 调用 `getuploadurl`
4. 使用 AES-128-ECB 加密内容
5. 上传到 CDN
6. 返回 `downloadEncryptedQueryParam` 与 `aes_key`

`MediaInfo` 当前包含：

- `FileKey`
- `DownloadEncryptedQueryParam`
- `AesKey`
- `FileSize`
- `FileSizeCiphertext`

### 下载

```go
func (c *Client) DownloadMedia(cdnURL, aesKey string) ([]byte, error)
func (c *Client) DownloadReceivedMedia(item Item) ([]byte, error)
func EncryptMedia(data []byte) (encrypted []byte, aesKey string, err error)
func DecryptMedia(data []byte, aesKey string) ([]byte, error)
```

说明：

- `DownloadReceivedMedia` 会根据消息中的媒体字段解析下载地址和密钥
- 当前实现会尝试多种下载 URL 形式，以兼容不同消息形态

## 常量

### ItemType

- `ItemTypeText = 1`
- `ItemTypeImage = 2`
- `ItemTypeVoice = 3`
- `ItemTypeFile = 4`
- `ItemTypeVideo = 5`

### MessageType

- `MessageTypeUser = 1`
- `MessageTypeBot = 2`

### 其他

- `MessageStateNormal = 2`
- `ChannelVersion = "1.0.2"`

## 请求头与协议

客户端会自动附带：

- `Content-Type: application/json`
- `AuthorizationType: ilink_bot_token`
- `X-WECHAT-UIN: base64(randomUint32)`
- `Authorization: Bearer {bot_token}`

协议常量：

- Base URL: `https://ilinkai.weixin.qq.com`
- CDN: `https://novac2c.cdn.weixin.qq.com/c2c`

## 文件结构

- [`client.go`](/E:/y1_code/WeChat-iLink-Go/client.go): 客户端与请求封装
- [`login.go`](/E:/y1_code/WeChat-iLink-Go/login.go): 扫码登录
- [`updates.go`](/E:/y1_code/WeChat-iLink-Go/updates.go): 长轮询
- [`send.go`](/E:/y1_code/WeChat-iLink-Go/send.go): 消息发送
- [`media.go`](/E:/y1_code/WeChat-iLink-Go/media.go): 媒体上传、下载、加解密
- [`typing.go`](/E:/y1_code/WeChat-iLink-Go/typing.go): typing 状态
- [`types.go`](/E:/y1_code/WeChat-iLink-Go/types.go): 协议类型
- [`example/main.go`](/E:/y1_code/WeChat-iLink-Go/example/main.go): 回显示例
- [`weixin.md`](/E:/y1_code/WeChat-iLink-Go/weixin.md): 协议整理说明

## 当前限制

- 图片/文件下载仍受运行环境网络与 DNS 影响
- 首次接收媒体时，部分环境可能需要依赖多种 URL 回退逻辑
- 示例程序仅展示最小可运行回显流程，不包含完整生产级状态管理

## License

Apache-2.0
