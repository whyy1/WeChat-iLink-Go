package ilink

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

type sendMessageRequest struct {
	Msg      Message  `json:"msg"`
	BaseInfo BaseInfo `json:"base_info"`
}

type sendMessageResponse struct {
	baseResponse
}

// SendMessage sends a fully constructed Message to a user.
// Always set ContextToken from the inbound message to maintain conversation threading.
// If FromUserID is empty, it is filled automatically from the client's bot ID.
func (c *Client) SendMessage(msg Message) error {
	if msg.FromUserID == "" && c.botID != "" {
		msg.FromUserID = c.botID
	}
	msg.FromUserID = ""
	//if msg.ClientID == "" {
	msg.ClientID = generateClientID()
	//}
	data, err := c.do(http.MethodPost, "/ilink/bot/sendmessage", sendMessageRequest{Msg: msg, BaseInfo: buildBaseInfo()})
	if err != nil {
		return err
	}
	var resp sendMessageResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}
	return resp.err()
}

// SendText sends a plain text reply.
func (c *Client) SendText(toUserID, contextToken, text string) error {
	return c.SendMessage(Message{
		ToUserID:     toUserID,
		MessageType:  MessageTypeBot,
		MessageState: MessageStateNormal,
		ContextToken: contextToken,
		ItemList:     []Item{{Type: ItemTypeText, TextItem: &TextItem{Text: text}}},
	})
}

// SendImage sends an image that was previously uploaded via UploadMedia.
func (c *Client) SendImage(toUserID, contextToken, cdnURL, aesKey string) error {
	return c.SendMessage(Message{
		ToUserID:     toUserID,
		MessageType:  MessageTypeBot,
		MessageState: MessageStateNormal,
		ContextToken: contextToken,
		ItemList:     []Item{{Type: ItemTypeImage, ImageItem: &ImageItem{CDNUrl: cdnURL, AesKey: aesKey}}},
	})
}

// SendImageRef sends an image using the protocol-native media reference.
func (c *Client) SendImageRef(toUserID, contextToken, encryptQueryParam, aesKey string, midSize int) error {
	return c.SendMessage(Message{
		ToUserID:     toUserID,
		MessageType:  MessageTypeBot,
		MessageState: MessageStateNormal,
		ContextToken: contextToken,
		ItemList: []Item{{
			Type: ItemTypeImage,
			ImageItem: &ImageItem{
				Media:   &MediaContent{EncryptQueryParam: encryptQueryParam, AesKey: aesKey, EncryptType: 1},
				MidSize: midSize,
			},
		}},
	})
}

// SendFile sends a file attachment that was previously uploaded via UploadMedia.
func (c *Client) SendFile(toUserID, contextToken, cdnURL, aesKey, fileName string, fileSize int64) error {
	return c.SendMessage(Message{
		ToUserID:     toUserID,
		MessageType:  MessageTypeBot,
		MessageState: MessageStateNormal,
		ContextToken: contextToken,
		ItemList: []Item{{Type: ItemTypeFile, FileItem: &FileItem{
			CDNUrl:   cdnURL,
			AesKey:   aesKey,
			FileName: fileName,
			FileSize: fileSize,
		}}},
	})
}

// SendFileRef sends a file using the protocol-native media reference.
func (c *Client) SendFileRef(toUserID, contextToken, encryptQueryParam, aesKey, fileName string, fileSize int64) error {
	return c.SendMessage(Message{
		ToUserID:     toUserID,
		MessageType:  MessageTypeBot,
		MessageState: MessageStateNormal,
		ContextToken: contextToken,
		ItemList: []Item{{
			Type: ItemTypeFile,
			FileItem: &FileItem{
				Media:    &MediaContent{EncryptQueryParam: encryptQueryParam, AesKey: aesKey, EncryptType: 1},
				FileName: fileName,
				Len:      strconv.FormatInt(fileSize, 10),
			},
		}},
	})
}

// SendVideo sends a video that was previously uploaded via UploadMedia.
func (c *Client) SendVideo(toUserID, contextToken, cdnURL, aesKey string) error {
	return c.SendMessage(Message{
		ToUserID:     toUserID,
		MessageType:  MessageTypeBot,
		MessageState: MessageStateNormal,
		ContextToken: contextToken,
		ItemList:     []Item{{Type: ItemTypeVideo, VideoItem: &VideoItem{CDNUrl: cdnURL, AesKey: aesKey}}},
	})
}
