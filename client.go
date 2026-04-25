package ilink

import (
	"bytes"
	"context"
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

const (
	DefaultBaseURL    = "https://ilinkai.weixin.qq.com"
	DefaultCDNBaseURL = "https://novac2c.cdn.weixin.qq.com/c2c"
)

type ClientOption func(*Client)

type Client struct {
	botToken   string
	botID      string
	baseURL    string
	cdnBaseURL string
	httpClient *http.Client
	Debug      bool
}

type apiResponse interface {
	apiErr() error
}

func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) {
		if strings.TrimSpace(baseURL) != "" {
			c.baseURL = strings.TrimRight(baseURL, "/")
		}
	}
}

func WithCDNBaseURL(cdnBaseURL string) ClientOption {
	return func(c *Client) {
		if strings.TrimSpace(cdnBaseURL) != "" {
			c.cdnBaseURL = strings.TrimRight(cdnBaseURL, "/")
		}
	}
}

func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

func WithDebug(debug bool) ClientOption {
	return func(c *Client) {
		c.Debug = debug
	}
}

func NewClient(botToken string, opts ...ClientOption) *Client {
	botID := ""
	if i := strings.Index(botToken, ":"); i > 0 {
		botID = botToken[:i]
	}

	client := &Client{
		botToken:   botToken,
		botID:      botID,
		baseURL:    DefaultBaseURL,
		cdnBaseURL: DefaultCDNBaseURL,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
	for _, opt := range opts {
		opt(client)
	}
	return client
}

func buildBaseInfo() BaseInfo {
	return BaseInfo{ChannelVersion: ChannelVersion}
}

func decodeJSON(data []byte, out interface{}) error {
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}
	return nil
}

func decodeAPIResponse[T apiResponse](data []byte, out T) error {
	if err := decodeJSON(data, out); err != nil {
		return err
	}
	return out.apiErr()
}

func generateClientID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		b = []byte{byte(time.Now().UnixNano())}
	}
	return fmt.Sprintf("ilink-go:%d-%x", time.Now().UnixMilli(), b)
}

func generateWechatUIN() string {
	var n uint32
	if err := binary.Read(rand.Reader, binary.BigEndian, &n); err != nil {
		n = uint32(time.Now().UnixNano())
	}
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d", n)))
}

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

func (c *Client) do(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
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

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	for k, vals := range c.buildHeaders() {
		for _, v := range vals {
			req.Header.Set(k, v)
		}
	}

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

func (c *Client) doJSON(ctx context.Context, method, path string, body interface{}, out apiResponse) error {
	data, err := c.do(ctx, method, path, body)
	if err != nil {
		return err
	}
	return decodeAPIResponse(data, out)
}

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

func (r baseResponse) apiErr() error {
	return r.err()
}
