# WeChat-iLink-Go

A Go client library for the WeChat iLink Bot API — Tencent's official WeChat Bot protocol introduced via OpenClaw in 2026.

## Overview

The iLink protocol (`ilinkai.weixin.qq.com`) provides personal WeChat account bot capabilities backed by Tencent's official terms. This library covers the full workflow: QR-code login, long-poll message ingestion, text/media sending, CDN upload with AES-128-ECB encryption, and typing indicators.

> API reverse-engineered from the `@tencent-weixin/openclaw-weixin` npm package (v1.0.2). See [weixin-bot-api.md](https://github.com/hao-ji-xing/openclaw-weixin/blob/main/weixin-bot-api.md) for the original analysis.

## Installation

```bash
go get github.com/whyy1/WeChat-iLink-Go
```

## Quick Start

```go
import (
    "fmt"
    "time"
    ilink "github.com/whyy1/WeChat-iLink-Go"
)

// 1. Login via QR code
c := ilink.NewClient("")
qr, _ := c.GetBotQRCode()
fmt.Println("Scan:", qr.QRCodeURL)
token, _ := c.WaitForLogin(qr.QRCode, 2*time.Second)

// 2. Poll and reply
bot := ilink.NewClient(token)
bot.Poll(func(msg ilink.Message) error {
    for _, item := range msg.ItemList {
        if item.Type == ilink.ItemTypeText {
            return bot.SendText(msg.FromUserID, msg.ContextToken, "Got: "+item.TextItem.Text)
        }
    }
    return nil
})
```

A runnable echo bot is in [`example/main.go`](example/main.go).

## API Reference

### Client

```go
// NewClient creates an iLink client.
// Pass "" before login; use the bot_token returned by WaitForLogin afterwards.
func NewClient(botToken string) *Client
```

All requests automatically include the required headers:

| Header | Value |
|--------|-------|
| `Content-Type` | `application/json` |
| `AuthorizationType` | `ilink_bot_token` |
| `X-WECHAT-UIN` | `base64(randomUint32)` — rotated per request |
| `Authorization` | `Bearer {bot_token}` |

---

### Login

```go
// Get a QR code to display to the user
func (c *Client) GetBotQRCode() (*QRCodeResponse, error)
// qr.QRCode    — pass to GetQRCodeStatus / WaitForLogin
// qr.QRCodeURL — URL of the QR image to display

// Poll scan status once
func (c *Client) GetQRCodeStatus(qrcode string) (*QRCodeStatus, error)
// status.Status: 0=pending, 1=confirmed (BotToken available), 2=expired

// Block until the user scans and confirms; returns bot_token
func (c *Client) WaitForLogin(qrcode string, interval time.Duration) (string, error)
```

---

### Receiving Messages

```go
// Long-poll once. Pass "" as cursor on the first call.
// Always update cursor from resp.GetUpdatesBuf before the next call.
func (c *Client) GetUpdates(cursor string) (*UpdatesResponse, error)

// Convenience loop — manages the cursor internally; returns on handler error.
func (c *Client) Poll(handler func(msg Message) error) error
```

**Message item types:**

| Constant | Value | Description |
|----------|-------|-------------|
| `ItemTypeText` | 1 | Plain text (`item.TextItem.Text`) |
| `ItemTypeImage` | 2 | Image — AES-128-ECB encrypted on CDN |
| `ItemTypeVoice` | 3 | Voice/audio (silk codec, optional `Transcription`) |
| `ItemTypeFile` | 4 | File attachment (`FileName`, `FileSize`) |
| `ItemTypeVideo` | 5 | Video |

> **Critical:** Each inbound message carries a `ContextToken`. You **must** echo it back in your reply — omitting it breaks conversation threading.

---

### Sending Messages

```go
func (c *Client) SendText(toUserID, contextToken, text string) error
func (c *Client) SendImage(toUserID, contextToken, cdnURL, aesKey string) error
func (c *Client) SendFile(toUserID, contextToken, cdnURL, aesKey, fileName string, fileSize int64) error
func (c *Client) SendVideo(toUserID, contextToken, cdnURL, aesKey string) error

// Build an arbitrary message
func (c *Client) SendMessage(msg Message) error
```

---

### Media Upload

All CDN files must be AES-128-ECB encrypted before upload. The base64-encoded key travels inside the `sendmessage` payload.

```go
// High-level: encrypt + get upload URL + PUT to CDN in one call
func (c *Client) UploadMedia(data []byte, fileType int) (*MediaInfo, error)
// media.CDNUrl — pass to SendImage / SendFile / SendVideo
// media.AesKey — pass to SendImage / SendFile / SendVideo

// Low-level encrypt/decrypt (standard library only, no external deps)
func EncryptMedia(data []byte) (encrypted []byte, aesKey string, err error)
func DecryptMedia(data []byte, aesKey string) ([]byte, error)

// Request a CDN pre-signed PUT URL
func (c *Client) GetUploadURL(fileType int, fileSize int64) (*UploadURLResponse, error)
```

---

### Typing Indicator

```go
cfg, err := bot.GetConfig(contextToken)  // fetch typing_ticket
err = bot.SendTyping(cfg.TypingTicket, ilink.TypingStatusOn)
// ... process & send reply ...
err = bot.SendTyping(cfg.TypingTicket, ilink.TypingStatusOff)
```

---

## Protocol Notes

| Detail | Value |
|--------|-------|
| Base URL | `https://ilinkai.weixin.qq.com` |
| CDN | `https://novac2c.cdn.weixin.qq.com/c2c` |
| Long-poll hold | ≤ 35 seconds |
| Media encryption | AES-128-ECB + PKCS7 padding |
| User ID format | `xxx@im.wechat` |
| Bot ID format | `xxx@im.bot` |
| Channel version | `1.0.2` |

The `get_updates_buf` cursor acts as a database cursor — it **must** be updated on every `getupdates` call to prevent duplicate message delivery.

## License

Apache 2.0
