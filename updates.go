package ilink

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"
)

type GetUpdatesRequest struct {
	Cursor string
}

type UpdatesResponse struct {
	baseResponse
	Msgs               []Message `json:"msgs"`
	GetUpdatesBuf      string    `json:"get_updates_buf"`
	LongpollingTimeout int       `json:"longpolling_timeout_ms"`
}

type PollHandler func(context.Context, Message) error

type getUpdatesEnvelope struct {
	GetUpdatesBuf string   `json:"get_updates_buf"`
	BaseInfo      BaseInfo `json:"base_info"`
}

func (c *Client) GetUpdates(ctx context.Context, req GetUpdatesRequest) (*UpdatesResponse, error) {
	var resp UpdatesResponse
	body := getUpdatesEnvelope{
		GetUpdatesBuf: req.Cursor,
		BaseInfo:      buildBaseInfo(),
	}
	if err := c.doJSON(ctx, http.MethodPost, "/ilink/bot/getupdates", body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

const maxConsecutiveErrors = 3

func (c *Client) Poll(ctx context.Context, handler PollHandler) error {
	cursor := ""
	consecutiveErrors := 0

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		resp, err := c.GetUpdates(ctx, GetUpdatesRequest{Cursor: cursor})
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			consecutiveErrors++
			if consecutiveErrors >= maxConsecutiveErrors {
				return fmt.Errorf("poll failed after %d consecutive errors: %w", maxConsecutiveErrors, err)
			}
			backoff := time.Duration(consecutiveErrors) * time.Second
			if c.Debug {
				fmt.Fprintf(os.Stderr, "[ilink] poll error (%d/%d), retrying in %v: %v\n", consecutiveErrors, maxConsecutiveErrors, backoff, err)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				continue
			}
		}
		consecutiveErrors = 0
		if resp.GetUpdatesBuf != "" {
			cursor = resp.GetUpdatesBuf
		}
		for _, msg := range resp.Msgs {
			if err := handler(ctx, msg); err != nil {
				return err
			}
		}
	}
}

// SimplePollHandler is the callback signature for PollSimple (no context).
type SimplePollHandler func(msg Message) error

// Convenience wrappers matching the README public API (no context.Context).

func (c *Client) GetUpdatesSimple(cursor string) (*UpdatesResponse, error) {
	return c.GetUpdates(context.Background(), GetUpdatesRequest{Cursor: cursor})
}

// PollSimple polls for messages using a simplified handler without context.
func (c *Client) PollSimple(handler SimplePollHandler) error {
	return c.Poll(context.Background(), func(_ context.Context, msg Message) error {
		return handler(msg)
	})
}
