package ilink

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// API endpoints
const (
	BaseURL    = "https://ilinkai.weixin.qq.com"
	CDNBaseURL = "https://novac2c.cdn.weixin.qq.com/c2c"
)

// Client is the WeChat iLink bot API client
type Client struct {
	botToken   string
	botID      string // parsed from token prefix, e.g. "e9546fe14322@im.bot"
	httpClient *http.Client
	// Debug enables printing raw request/response bodies to stderr for troubleshooting
	Debug bool
}

func buildBaseInfo() BaseInfo {
	return BaseInfo{ChannelVersion: ChannelVersion}
}

// NewClient creates a new iLink client.
// Pass an empty string for botToken before login; use the token returned by WaitForLogin afterwards.
// The bot ID (from_user_id for outbound messages) is parsed automatically from the token prefix.
func NewClient(botToken string) *Client {
	botID := ""
	if i := strings.Index(botToken, ":"); i > 0 {
		botID = botToken[:i]
	}
	return &Client{
		botToken: botToken,
		botID:    botID,
		httpClient: &http.Client{
			// Longer than the 35-second long-poll timeout
			Timeout: 60 * time.Second,
		},
	}
}

// generateClientID creates a unique message identifier required by the sendmessage API.
// Format mirrors the reference implementation: "ilink-go:{timestamp_ms}-{random_hex}".
func generateClientID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		b = []byte{byte(time.Now().UnixNano())}
	}
	return fmt.Sprintf("ilink-go:%d-%x", time.Now().UnixMilli(), b)
}

// generateWechatUIN builds the X-WECHAT-UIN header value.
// The spec requires base64(String(randomUint32())) per request to prevent replay attacks.
func generateWechatUIN() string {
	var n uint32
	if err := binary.Read(rand.Reader, binary.BigEndian, &n); err != nil {
		n = uint32(time.Now().UnixNano())
	}
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d", n)))
}

// buildHeaders returns the authentication headers required by every API request
func (c *Client) buildHeaders() http.Header {
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	h.Set("AuthorizationType", "ilink_bot_token")
	h.Set("X-WECHAT-UIN", generateWechatUIN())
	if c.botToken != "" {
		h.Set("Authorization", "Bearer "+c.botToken)
	}
	return h
}

// do executes an HTTP request against the iLink API and returns the raw response body
func (c *Client) do(method, path string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		if c.Debug {
			fmt.Fprintf(os.Stderr, "[ilink] --> %s %s body=%s\n", method, path, data)
		}
		reqBody = bytes.NewReader(data)
	} else if c.Debug {
		fmt.Fprintf(os.Stderr, "[ilink] --> %s %s\n", method, path)
	}

	req, err := http.NewRequest(method, BaseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	for k, vals := range c.buildHeaders() {
		for _, v := range vals {
			req.Header.Set(k, v)
		}
	}
	//fmt.Fprintf(os.Stderr, "[Header]=%s\n", req.Header)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if c.Debug {
		fmt.Fprintf(os.Stderr, "[ilink] <-- %d %s\n", resp.StatusCode, data)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, data)
	}
	return data, nil
}

// baseResponse is embedded in all API response structs to surface API-level errors.
// Different endpoints use either "ret" or "errcode" as the error field.
type baseResponse struct {
	Ret     int    `json:"ret"`
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg,omitempty"`
}

func (r baseResponse) err() error {
	code := r.ErrCode
	if code == 0 {
		code = r.Ret
	}
	if code != 0 {
		if r.ErrMsg != "" {
			return fmt.Errorf("api error %d: %s", code, r.ErrMsg)
		}
		return fmt.Errorf("api error %d", code)
	}
	return nil
}
