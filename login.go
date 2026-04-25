package ilink

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type LoginRequest struct {
	PollInterval time.Duration
}

type QRCodeResponse struct {
	baseResponse
	QRCode    string `json:"qrcode"`
	QRCodeURL string `json:"qrcode_img_content"`
}

type QRCodeStatus struct {
	baseResponse
	Status     string `json:"status"`
	BotToken   string `json:"bot_token,omitempty"`
	BaseURL    string `json:"baseurl,omitempty"`
	IlinkBotID string `json:"ilink_bot_id,omitempty"`
	UserID     string `json:"ilink_user_id,omitempty"`
}

func (c *Client) GetBotQRCode(ctx context.Context) (*QRCodeResponse, error) {
	var resp QRCodeResponse
	if err := c.doJSON(ctx, http.MethodGet, "/ilink/bot/get_bot_qrcode?bot_type=3", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetQRCodeStatus(ctx context.Context, qrcode string) (*QRCodeStatus, error) {
	var resp QRCodeStatus
	path := "/ilink/bot/get_qrcode_status?qrcode=" + url.QueryEscape(qrcode)
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) WaitForLogin(ctx context.Context, qrcode string, req LoginRequest) (string, error) {
	interval := req.PollInterval
	if interval <= 0 {
		interval = 2 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
			status, err := c.GetQRCodeStatus(ctx, qrcode)
			if err != nil {
				return "", err
			}
			switch status.Status {
			case QRCodeStatusConfirmed:
				return status.BotToken, nil
			case QRCodeStatusExpired:
				return "", fmt.Errorf("qr code expired")
			}
		}
	}
}

// Convenience wrappers matching the README public API (no context.Context).

func (c *Client) GetBotQRCodeSimple() (*QRCodeResponse, error) {
	return c.GetBotQRCode(context.Background())
}

func (c *Client) GetQRCodeStatusSimple(qrcode string) (*QRCodeStatus, error) {
	return c.GetQRCodeStatus(context.Background(), qrcode)
}

// WaitForLoginSimple polls for QR code scan confirmation.
// interval controls how often to check; pass 0 for the default (2s).
func (c *Client) WaitForLoginSimple(qrcode string, interval time.Duration) (string, error) {
	return c.WaitForLogin(context.Background(), qrcode, LoginRequest{PollInterval: interval})
}
