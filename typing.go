package ilink

import (
	"context"
	"net/http"
)

type GetConfigRequest struct {
	IlinkUserID  string
	ContextToken string
}

type SendTypingRequest struct {
	IlinkUserID  string
	TypingTicket string
	Status       int
}

type getConfigEnvelope struct {
	IlinkUserID  string   `json:"ilink_user_id"`
	ContextToken string   `json:"context_token,omitempty"`
	BaseInfo     BaseInfo `json:"base_info"`
}

type sendTypingEnvelope struct {
	IlinkUserID  string   `json:"ilink_user_id"`
	TypingTicket string   `json:"typing_ticket"`
	Status       int      `json:"status"`
	BaseInfo     BaseInfo `json:"base_info"`
}

type ConfigResponse struct {
	baseResponse
	TypingTicket string `json:"typing_ticket"`
}

type typingResponse struct{ baseResponse }

func (c *Client) GetConfig(ctx context.Context, req GetConfigRequest) (*ConfigResponse, error) {
	var resp ConfigResponse
	body := getConfigEnvelope{
		IlinkUserID:  req.IlinkUserID,
		ContextToken: req.ContextToken,
		BaseInfo:     buildBaseInfo(),
	}
	if err := c.doJSON(ctx, http.MethodPost, "/ilink/bot/getconfig", body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) SendTyping(ctx context.Context, req SendTypingRequest) error {
	var resp typingResponse
	body := sendTypingEnvelope{
		IlinkUserID:  req.IlinkUserID,
		TypingTicket: req.TypingTicket,
		Status:       req.Status,
		BaseInfo:     buildBaseInfo(),
	}
	return c.doJSON(ctx, http.MethodPost, "/ilink/bot/sendtyping", body, &resp)
}

// Convenience wrappers matching the README public API (no context.Context).

func (c *Client) GetConfigSimple(ilinkUserID, contextToken string) (*ConfigResponse, error) {
	return c.GetConfig(context.Background(), GetConfigRequest{
		IlinkUserID:  ilinkUserID,
		ContextToken: contextToken,
	})
}

func (c *Client) SendTypingSimple(ilinkUserID, typingTicket string, status int) error {
	return c.SendTyping(context.Background(), SendTypingRequest{
		IlinkUserID:  ilinkUserID,
		TypingTicket: typingTicket,
		Status:       status,
	})
}
