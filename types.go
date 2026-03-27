package ilink

// Item type constants
const (
	ItemTypeText  = 1
	ItemTypeImage = 2
	ItemTypeVoice = 3
	ItemTypeFile  = 4
	ItemTypeVideo = 5
)

// Message type constants
const (
	MessageTypeUser = 1
	MessageTypeBot  = 2
)

// MessageStateNormal is the standard message state for outbound messages
const MessageStateNormal = 2

// ChannelVersion is the iLink protocol version
const ChannelVersion = "1.0.2"

// Typing status constants
const (
	TypingStatusOn  = 1
	TypingStatusOff = 2
)

// QR code scan status string values returned by the API
const (
	QRCodeStatusWait      = "wait"
	QRCodeStatusConfirmed = "confirmed"
	QRCodeStatusExpired   = "expired"
)

// BaseInfo is included in requests to identify the client version
type BaseInfo struct {
	ChannelVersion string `json:"channel_version"`
}

// TextItem holds the content of a text message
type TextItem struct {
	Text string `json:"text"`
}

// MediaContent holds CDN access credentials embedded in received media items
type MediaContent struct {
	EncryptQueryParam string `json:"encrypt_query_param"`
	AesKey            string `json:"aes_key"` // base64 of hex key text (redundant; use parent AesKeyHex)
	EncryptType       int    `json:"encrypt_type,omitempty"`
	FullURL           string `json:"full_url,omitempty"`
}

// ImageItem holds image data.
// Inbound fields (from getupdates): URL, AesKeyHex, Media, size fields.
// Outbound fields (for sendmessage): CDNUrl, AesKey.
type ImageItem struct {
	// Inbound
	URL         string        `json:"url,omitempty"`
	AesKeyHex   string        `json:"aeskey,omitempty"` // hex-encoded AES-128 key
	Media       *MediaContent `json:"media,omitempty"`
	MidSize     int           `json:"mid_size,omitempty"`
	ThumbSize   int           `json:"thumb_size,omitempty"`
	ThumbHeight int           `json:"thumb_height,omitempty"`
	ThumbWidth  int           `json:"thumb_width,omitempty"`
	HDSize      int           `json:"hd_size,omitempty"`
	// Outbound
	CDNUrl string `json:"cdn_url,omitempty"`
	AesKey string `json:"aes_key,omitempty"` // base64-encoded AES key
}

// VoiceItem holds voice/audio data.
// Inbound fields: URL, AesKeyHex, Media, Transcription.
// Outbound fields: CDNUrl, AesKey.
type VoiceItem struct {
	// Inbound
	URL           string        `json:"url,omitempty"`
	AesKeyHex     string        `json:"aeskey,omitempty"`
	Media         *MediaContent `json:"media,omitempty"`
	Transcription string        `json:"transcription,omitempty"`
	// Outbound
	CDNUrl string `json:"cdn_url,omitempty"`
	AesKey string `json:"aes_key,omitempty"`
}

// FileItem holds file attachment data.
// Inbound fields: URL, AesKeyHex, Media, FileName, FileSize.
// Outbound fields: CDNUrl, AesKey, FileName, FileSize.
type FileItem struct {
	// Inbound
	URL       string        `json:"url,omitempty"`
	AesKeyHex string        `json:"aeskey,omitempty"`
	Media     *MediaContent `json:"media,omitempty"`
	// Shared
	FileName string `json:"file_name,omitempty"`
	FileSize int64  `json:"file_size,omitempty"`
	Len      string `json:"len,omitempty"`
	// Outbound
	CDNUrl string `json:"cdn_url,omitempty"`
	AesKey string `json:"aes_key,omitempty"`
}

// VideoItem holds video data.
// Inbound fields: URL, AesKeyHex, Media.
// Outbound fields: CDNUrl, AesKey.
type VideoItem struct {
	// Inbound
	URL       string        `json:"url,omitempty"`
	AesKeyHex string        `json:"aeskey,omitempty"`
	Media     *MediaContent `json:"media,omitempty"`
	// Outbound
	CDNUrl string `json:"cdn_url,omitempty"`
	AesKey string `json:"aes_key,omitempty"`
}

// Item is a single element within a message's ItemList
type Item struct {
	Type         int        `json:"type"`
	CreateTimeMs int64      `json:"create_time_ms,omitempty"`
	UpdateTimeMs int64      `json:"update_time_ms,omitempty"`
	IsCompleted  bool       `json:"is_completed,omitempty"`
	TextItem     *TextItem  `json:"text_item,omitempty"`
	ImageItem    *ImageItem `json:"image_item,omitempty"`
	VoiceItem    *VoiceItem `json:"voice_item,omitempty"`
	FileItem     *FileItem  `json:"file_item,omitempty"`
	VideoItem    *VideoItem `json:"video_item,omitempty"`
}

// Message represents a WeChat iLink message (inbound or outbound)
type Message struct {
	ClientID     string `json:"client_id,omitempty"`
	FromUserID   string `json:"from_user_id,omitempty"`
	ToUserID     string `json:"to_user_id"`
	MessageType  int    `json:"message_type"`
	MessageState int    `json:"message_state"`
	ContextToken string `json:"context_token,omitempty"`
	ItemList     []Item `json:"item_list"`
}
