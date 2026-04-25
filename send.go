package ilink

import (
	"context"
	"net/http"
	"strconv"
)

type SendMessageRequest struct {
	Message Message
}

type SendTextRequest struct {
	ToUserID     string
	ContextToken string
	Text         string
}

type SendImageRequest struct {
	ToUserID     string
	ContextToken string
	Media        UploadedMedia
}

type SendFileRequest struct {
	ToUserID     string
	ContextToken string
	FileName     string
	FileSize     int64
	Media        UploadedMedia
}

type SendVideoRequest struct {
	ToUserID     string
	ContextToken string
	Media        UploadedMedia
}

type sendMessageEnvelope struct {
	Msg      Message  `json:"msg"`
	BaseInfo BaseInfo `json:"base_info"`
}

type sendMessageResponse struct{ baseResponse }

func newOutboundMessage(toUserID, contextToken string, item Item) Message {
	return Message{
		ToUserID:     toUserID,
		MessageType:  MessageTypeBot,
		MessageState: MessageStateNormal,
		ContextToken: contextToken,
		ItemList:     []Item{item},
	}
}

func newMediaRef(media UploadedMedia) *MediaContent {
	return &MediaContent{
		EncryptQueryParam: media.EncryptQueryParam,
		AesKey:            media.AesKey,
		EncryptType:       1,
	}
}

func (c *Client) SendMessage(ctx context.Context, req SendMessageRequest) error {
	msg := req.Message
	// Server identifies the sender by bot_token; FromUserID must be empty on outbound.
	msg.FromUserID = ""
	msg.ClientID = generateClientID()

	var resp sendMessageResponse
	body := sendMessageEnvelope{Msg: msg, BaseInfo: buildBaseInfo()}
	return c.doJSON(ctx, http.MethodPost, "/ilink/bot/sendmessage", body, &resp)
}

func (c *Client) SendText(ctx context.Context, req SendTextRequest) error {
	msg := newOutboundMessage(req.ToUserID, req.ContextToken, Item{
		Type:     ItemTypeText,
		TextItem: &TextItem{Text: req.Text},
	})
	return c.SendMessage(ctx, SendMessageRequest{Message: msg})
}

func (c *Client) SendImage(ctx context.Context, req SendImageRequest) error {
	msg := newOutboundMessage(req.ToUserID, req.ContextToken, Item{
		Type: ItemTypeImage,
		ImageItem: &ImageItem{
			Media:   newMediaRef(req.Media),
			MidSize: int(req.Media.CipherSize),
		},
	})
	return c.SendMessage(ctx, SendMessageRequest{Message: msg})
}

func (c *Client) SendFile(ctx context.Context, req SendFileRequest) error {
	fileSize := req.FileSize
	if fileSize <= 0 {
		fileSize = req.Media.PlainSize
	}
	msg := newOutboundMessage(req.ToUserID, req.ContextToken, Item{
		Type: ItemTypeFile,
		FileItem: &FileItem{
			Media:    newMediaRef(req.Media),
			FileName: req.FileName,
			Len:      strconv.FormatInt(fileSize, 10),
		},
	})
	return c.SendMessage(ctx, SendMessageRequest{Message: msg})
}

func (c *Client) SendVideo(ctx context.Context, req SendVideoRequest) error {
	msg := newOutboundMessage(req.ToUserID, req.ContextToken, Item{
		Type: ItemTypeVideo,
		VideoItem: &VideoItem{
			Media: newMediaRef(req.Media),
		},
	})
	return c.SendMessage(ctx, SendMessageRequest{Message: msg})
}

// SendImageRef sends an image using the protocol-native encrypt_query_param + aes_key
// structure, which is preferred when re-sending a previously received image.
func (c *Client) SendImageRef(ctx context.Context, toUserID, contextToken, encryptQueryParam, aesKey string, midSize int) error {
	msg := newOutboundMessage(toUserID, contextToken, Item{
		Type: ItemTypeImage,
		ImageItem: &ImageItem{
			Media: &MediaContent{
				EncryptQueryParam: encryptQueryParam,
				AesKey:            aesKey,
				EncryptType:       1,
			},
			MidSize: midSize,
		},
	})
	return c.SendMessage(ctx, SendMessageRequest{Message: msg})
}

// SendFileRef sends a file using the protocol-native encrypt_query_param + aes_key
// structure, which is preferred when re-sending a previously received file.
func (c *Client) SendFileRef(ctx context.Context, toUserID, contextToken, encryptQueryParam, aesKey, fileName string, fileSize int64) error {
	msg := newOutboundMessage(toUserID, contextToken, Item{
		Type: ItemTypeFile,
		FileItem: &FileItem{
			Media: &MediaContent{
				EncryptQueryParam: encryptQueryParam,
				AesKey:            aesKey,
				EncryptType:       1,
			},
			FileName: fileName,
			Len:      strconv.FormatInt(fileSize, 10),
		},
	})
	return c.SendMessage(ctx, SendMessageRequest{Message: msg})
}

// Convenience wrappers matching the README public API (no context.Context).

func (c *Client) SendMessageSimple(msg Message) error {
	return c.SendMessage(context.Background(), SendMessageRequest{Message: msg})
}

func (c *Client) SendTextSimple(toUserID, contextToken, text string) error {
	return c.SendText(context.Background(), SendTextRequest{
		ToUserID:     toUserID,
		ContextToken: contextToken,
		Text:         text,
	})
}

func (c *Client) SendImageSimple(toUserID, contextToken, cdnURL, aesKey string) error {
	return c.SendImage(context.Background(), SendImageRequest{
		ToUserID:     toUserID,
		ContextToken: contextToken,
		Media: UploadedMedia{
			EncryptQueryParam: cdnURL,
			AesKey:            aesKey,
		},
	})
}

func (c *Client) SendFileSimple(toUserID, contextToken, cdnURL, aesKey, fileName string, fileSize int64) error {
	return c.SendFile(context.Background(), SendFileRequest{
		ToUserID:     toUserID,
		ContextToken: contextToken,
		FileName:     fileName,
		FileSize:     fileSize,
		Media: UploadedMedia{
			EncryptQueryParam: cdnURL,
			AesKey:            aesKey,
		},
	})
}

func (c *Client) SendVideoSimple(toUserID, contextToken, cdnURL, aesKey string) error {
	return c.SendVideo(context.Background(), SendVideoRequest{
		ToUserID:     toUserID,
		ContextToken: contextToken,
		Media: UploadedMedia{
			EncryptQueryParam: cdnURL,
			AesKey:            aesKey,
		},
	})
}

func (c *Client) SendImageRefSimple(toUserID, contextToken, encryptQueryParam, aesKey string, midSize int) error {
	return c.SendImageRef(context.Background(), toUserID, contextToken, encryptQueryParam, aesKey, midSize)
}

func (c *Client) SendFileRefSimple(toUserID, contextToken, encryptQueryParam, aesKey, fileName string, fileSize int64) error {
	return c.SendFileRef(context.Background(), toUserID, contextToken, encryptQueryParam, aesKey, fileName, fileSize)
}
