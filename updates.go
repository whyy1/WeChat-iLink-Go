package ilink

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// UpdatesRequest is the body sent to /ilink/bot/getupdates
type UpdatesRequest struct {
	GetUpdatesBuf string `json:"get_updates_buf"`
	//BaseInfo      BaseInfo `json:"base_info"`
}

// UpdatesResponse is returned by GetUpdates
type UpdatesResponse struct {
	baseResponse
	Msgs               []Message `json:"msgs"`
	GetUpdatesBuf      string    `json:"get_updates_buf"`
	LongpollingTimeout int       `json:"longpolling_timeout_ms"`
}

// GetUpdates long-polls the server for incoming messages.
// Pass an empty cursor on the first call; use the returned GetUpdatesBuf for all subsequent calls.
// The server holds the connection for up to 35 seconds before responding.
func (c *Client) GetUpdates(cursor string) (*UpdatesResponse, error) {
	req := UpdatesRequest{
		GetUpdatesBuf: cursor,
		//BaseInfo:      BaseInfo{ChannelVersion: ChannelVersion},
	}
	data, err := c.do(http.MethodPost, "/ilink/bot/getupdates", req)
	if err != nil {
		return nil, err
	}
	var resp UpdatesResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	if err := resp.err(); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Poll continuously long-polls for incoming messages and calls handler for each one.
// It manages the cursor automatically. Returns when handler returns a non-nil error.
func (c *Client) Poll(handler func(msg Message) error) error {
	cursor := ""
	for {
		resp, err := c.GetUpdates(cursor)
		if err != nil {
			return err
		}
		if resp.GetUpdatesBuf != "" {
			cursor = resp.GetUpdatesBuf
		}
		for _, msg := range resp.Msgs {
			if err := handler(msg); err != nil {
				return err
			}
		}
	}
}
