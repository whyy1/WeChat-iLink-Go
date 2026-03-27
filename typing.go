package ilink

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type configRequest struct {
	IlinkUserID  string `json:"ilink_user_id"`
	ContextToken string `json:"context_token,omitempty"`
	BaseInfo     BaseInfo `json:"base_info"`
}

// ConfigResponse is returned by GetConfig
type ConfigResponse struct {
	baseResponse
	TypingTicket string `json:"typing_ticket"`
}

// GetConfig retrieves the typing_ticket for the given conversation context.
// The ticket is required by SendTyping.
func (c *Client) GetConfig(ilinkUserID, contextToken string) (*ConfigResponse, error) {
	data, err := c.do(http.MethodPost, "/ilink/bot/getconfig", configRequest{IlinkUserID: ilinkUserID, ContextToken: contextToken, BaseInfo: buildBaseInfo()})
	if err != nil {
		return nil, err
	}
	var resp ConfigResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	if err := resp.err(); err != nil {
		return nil, err
	}
	return &resp, nil
}

type typingRequest struct {
	IlinkUserID  string `json:"ilink_user_id"`
	TypingTicket string `json:"typing_ticket"`
	Status       int    `json:"status"`
	BaseInfo     BaseInfo `json:"base_info"`
}

type typingResponse struct {
	baseResponse
}

// SendTyping transmits the typing indicator status for a conversation.
// Use TypingStatusOn before processing and TypingStatusOff after sending the reply.
func (c *Client) SendTyping(ilinkUserID, typingTicket string, status int) error {
	data, err := c.do(http.MethodPost, "/ilink/bot/sendtyping", typingRequest{
		IlinkUserID:  ilinkUserID,
		TypingTicket: typingTicket,
		Status:       status,
		BaseInfo:     buildBaseInfo(),
	})
	if err != nil {
		return err
	}
	var resp typingResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}
	return resp.err()
}
