package ilink

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// QRCodeResponse is returned by GetBotQRCode
type QRCodeResponse struct {
	baseResponse
	QRCode    string `json:"qrcode"`
	QRCodeURL string `json:"qrcode_img_content"`
}

// QRCodeStatus is returned by GetQRCodeStatus
type QRCodeStatus struct {
	baseResponse
	Status     string `json:"status"`
	BotToken   string `json:"bot_token,omitempty"`
	BaseURL    string `json:"baseurl,omitempty"`
	IlinkBotID string `json:"ilink_bot_id,omitempty"`
	UserID     string `json:"ilink_user_id,omitempty"`
}

// GetBotQRCode requests a login QR code image.
// The returned QRCode string is needed for polling GetQRCodeStatus.
func (c *Client) GetBotQRCode() (*QRCodeResponse, error) {
	data, err := c.do(http.MethodGet, "/ilink/bot/get_bot_qrcode?bot_type=3", nil)
	if err != nil {
		return nil, err
	}
	var resp QRCodeResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	if err := resp.err(); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetQRCodeStatus polls the scan status of the given QR code.
// When Status == QRCodeStatusConfirmed the BotToken field is populated.
func (c *Client) GetQRCodeStatus(qrcode string) (*QRCodeStatus, error) {
	data, err := c.do(http.MethodGet, "/ilink/bot/get_qrcode_status?qrcode="+qrcode, nil)
	if err != nil {
		return nil, err
	}
	var resp QRCodeStatus
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	if err := resp.err(); err != nil {
		return nil, err
	}
	return &resp, nil
}

// WaitForLogin polls the QR code status at interval until the user scans and confirms,
// returning the bot_token on success. Use the token to create an authenticated Client.
func (c *Client) WaitForLogin(qrcode string, interval time.Duration) (string, error) {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		status, err := c.GetQRCodeStatus(qrcode)
		if err != nil {
			return "", err
		}
		switch status.Status {
		case QRCodeStatusConfirmed:
			return status.BotToken, nil
		case QRCodeStatusExpired:
			return "", fmt.Errorf("QR code expired")
			// QRCodeStatusWait: keep polling
		}
	}
	return "", fmt.Errorf("polling stopped")
}
