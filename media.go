package ilink

import (
	"bytes"
	"crypto/aes"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
)

var hexKeyPattern = regexp.MustCompile("^[0-9a-fA-F]{32}$")

// DownloadReceivedMedia downloads and decrypts a received media item (image/voice/file/video).
// Follows the OpenClaw implementation: use encrypt_query_param + parsed media.aes_key.
func (c *Client) DownloadReceivedMedia(item Item) ([]byte, error) {
	var media *MediaContent
	var hexKey string
	switch item.Type {
	case ItemTypeImage:
		if item.ImageItem == nil || item.ImageItem.Media == nil {
			return nil, fmt.Errorf("image item missing media info")
		}
		media = item.ImageItem.Media
		hexKey = item.ImageItem.AesKeyHex
	case ItemTypeVoice:
		if item.VoiceItem == nil || item.VoiceItem.Media == nil {
			return nil, fmt.Errorf("voice item missing media info")
		}
		media = item.VoiceItem.Media
		hexKey = item.VoiceItem.AesKeyHex
	case ItemTypeFile:
		if item.FileItem == nil || item.FileItem.Media == nil {
			return nil, fmt.Errorf("file item missing media info")
		}
		media = item.FileItem.Media
		hexKey = item.FileItem.AesKeyHex
	case ItemTypeVideo:
		if item.VideoItem == nil || item.VideoItem.Media == nil {
			return nil, fmt.Errorf("video item missing media info")
		}
		media = item.VideoItem.Media
		hexKey = item.VideoItem.AesKeyHex
	default:
		return nil, fmt.Errorf("unsupported item type %d", item.Type)
	}

	cdnURLs, aesKey, err := resolveReceivedMedia(media, hexKey)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for _, cdnURL := range cdnURLs {
		if c.Debug {
			fmt.Fprintf(os.Stderr, "[ilink] received media type=%d cdn_url=%s\n", item.Type, cdnURL)
		}
		data, err := c.DownloadMedia(cdnURL, aesKey)
		if err == nil {
			return data, nil
		}
		lastErr = err
		if c.Debug {
			fmt.Fprintf(os.Stderr, "[ilink] received media fallback failed type=%d err=%v\n", item.Type, err)
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no download candidates for received media")
	}
	return nil, lastErr
}

func resolveReceivedMedia(media *MediaContent, hexKey string) (cdnURLs []string, aesKey string, err error) {
	if media == nil {
		return nil, "", fmt.Errorf("media content is nil")
	}

	addCandidate := func(u string) {
		u = strings.TrimSpace(u)
		if u == "" {
			return
		}
		for _, existing := range cdnURLs {
			if existing == u {
				return
			}
		}
		cdnURLs = append(cdnURLs, u)
	}

	addCandidate(media.FullURL)
	encryptQueryParam := strings.TrimSpace(media.EncryptQueryParam)
	if encryptQueryParam != "" {
		addCandidate(CDNBaseURL + "/download?encrypted_query_param=" + encryptQueryParam)
		addCandidate(CDNBaseURL + "/download?encrypted_query_param=" + url.QueryEscape(encryptQueryParam))
	}
	if len(cdnURLs) == 0 {
		return nil, "", fmt.Errorf("missing media download url")
	}

	if strings.TrimSpace(hexKey) != "" {
		key, err := hex.DecodeString(strings.TrimSpace(hexKey))
		if err != nil {
			return nil, "", fmt.Errorf("decode hex key: %w", err)
		}
		return cdnURLs, base64.StdEncoding.EncodeToString(key), nil
	}

	aesKey = strings.TrimSpace(media.AesKey)
	if aesKey == "" {
		return nil, "", fmt.Errorf("missing aes key for received media")
	}
	return cdnURLs, aesKey, nil
}

// DownloadMedia downloads and decrypts media using a base64-encoded AES key.
// Use this for media you uploaded yourself (outbound CDN URL + your base64 key).
// For received messages, use DownloadReceivedMedia instead.
func (c *Client) DownloadMedia(cdnURL, aesKey string) ([]byte, error) {
	resp, err := c.httpClient.Get(cdnURL)
	if err != nil {
		return nil, fmt.Errorf("cdn download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cdn download status %d", resp.StatusCode)
	}
	encrypted, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cdn read: %w", err)
	}
	return DecryptMedia(encrypted, aesKey)
}

const (
	UploadMediaTypeImage = 1
	UploadMediaTypeVideo = 2
	UploadMediaTypeFile  = 3
	UploadMediaTypeVoice = 4
)

// UploadURLRequest is the body sent to /ilink/bot/getuploadurl
type UploadURLRequest struct {
	FileKey        string   `json:"filekey"`
	MediaType      int      `json:"media_type"`
	ToUserID       string   `json:"to_user_id,omitempty"`
	RawSize        int64    `json:"rawsize"`
	RawFileMD5     string   `json:"rawfilemd5"`
	FileSize       int64    `json:"filesize"`
	ThumbRawSize   int64    `json:"thumb_rawsize,omitempty"`
	ThumbRawFileMD5 string  `json:"thumb_rawfilemd5,omitempty"`
	ThumbFileSize  int64    `json:"thumb_filesize,omitempty"`
	NoNeedThumb    bool     `json:"no_need_thumb,omitempty"`
	AesKey         string   `json:"aeskey,omitempty"`
	BaseInfo       BaseInfo `json:"base_info"`
}

// UploadURLResponse is returned by GetUploadURL
type UploadURLResponse struct {
	baseResponse
	UploadParam      string `json:"upload_param"`
	ThumbUploadParam string `json:"thumb_upload_param,omitempty"`
}

func mapItemTypeToUploadMediaType(itemType int) (int, error) {
	switch itemType {
	case ItemTypeImage:
		return UploadMediaTypeImage, nil
	case ItemTypeVideo:
		return UploadMediaTypeVideo, nil
	case ItemTypeFile:
		return UploadMediaTypeFile, nil
	case ItemTypeVoice:
		return UploadMediaTypeVoice, nil
	default:
		return 0, fmt.Errorf("unsupported upload item type %d", itemType)
	}
}

// GetUploadURL requests CDN upload parameters using the protocol-native payload.
func (c *Client) GetUploadURL(req UploadURLRequest) (*UploadURLResponse, error) {
	req.BaseInfo = buildBaseInfo()
	data, err := c.do(http.MethodPost, "/ilink/bot/getuploadurl", req)
	if err != nil {
		return nil, err
	}
	var resp UploadURLResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	if err := resp.err(); err != nil {
		return nil, err
	}
	return &resp, nil
}

// MediaInfo holds the protocol-native media reference after a successful upload.
type MediaInfo struct {
	FileKey                 string
	DownloadEncryptedQueryParam string
	AesKey                  string // base64-encoded AES-128 key for sendmessage
	FileSize                int64
	FileSizeCiphertext      int64
}

func uploadBufferToCDN(httpClient *http.Client, plaintext []byte, uploadParam, fileKey string, aesKeyRaw []byte) (string, error) {
	ciphertext, err := encryptAES128ECB(plaintext, aesKeyRaw)
	if err != nil {
		return "", err
	}
	cdnURL := CDNBaseURL + "/upload?encrypted_query_param=" + url.QueryEscape(uploadParam) + "&filekey=" + url.QueryEscape(fileKey)
	req, err := http.NewRequest(http.MethodPost, cdnURL, bytes.NewReader(ciphertext))
	if err != nil {
		return "", fmt.Errorf("create upload request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if len(body) > 0 {
			return "", fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, body)
		}
		return "", fmt.Errorf("upload failed with status %d", resp.StatusCode)
	}
	downloadParam := strings.TrimSpace(resp.Header.Get("x-encrypted-param"))
	if downloadParam == "" {
		return "", fmt.Errorf("upload response missing x-encrypted-param header")
	}
	return downloadParam, nil
}

func randomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", fmt.Errorf("generate random hex: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// UploadMedia uploads media for a specific recipient using the protocol-native flow.
func (c *Client) UploadMediaForUser(toUserID string, data []byte, itemType int) (*MediaInfo, error) {
	mediaType, err := mapItemTypeToUploadMediaType(itemType)
	if err != nil {
		return nil, err
	}
	fileKey, err := randomHex(16)
	if err != nil {
		return nil, err
	}
	aesKeyRaw := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, aesKeyRaw); err != nil {
		return nil, fmt.Errorf("generate aes key: %w", err)
	}
	rawMD5 := md5.Sum(data)
	cipherSize := int64(len(pkcs7Pad(data, aes.BlockSize)))
	req := UploadURLRequest{
		FileKey:     fileKey,
		MediaType:   mediaType,
		ToUserID:    toUserID,
		RawSize:     int64(len(data)),
		RawFileMD5:  hex.EncodeToString(rawMD5[:]),
		FileSize:    cipherSize,
		NoNeedThumb: true,
		AesKey:      hex.EncodeToString(aesKeyRaw),
	}
	urlResp, err := c.GetUploadURL(req)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(urlResp.UploadParam) == "" {
		return nil, fmt.Errorf("getuploadurl returned empty upload_param")
	}
	downloadParam, err := uploadBufferToCDN(c.httpClient, data, urlResp.UploadParam, fileKey, aesKeyRaw)
	if err != nil {
		return nil, err
	}
	return &MediaInfo{
		FileKey:                 fileKey,
		DownloadEncryptedQueryParam: downloadParam,
		AesKey:                  base64.StdEncoding.EncodeToString([]byte(hex.EncodeToString(aesKeyRaw))),
		FileSize:                int64(len(data)),
		FileSizeCiphertext:      cipherSize,
	}, nil
}

// UploadMedia uploads media without specifying a recipient.
// Some servers may require to_user_id; prefer UploadMediaForUser in production code.
func (c *Client) UploadMedia(data []byte, fileType int) (*MediaInfo, error) {
	return c.UploadMediaForUser("", data, fileType)
}

// EncryptMedia encrypts data with AES-128-ECB using a freshly generated random key.
// Returns the encrypted bytes and the base64-encoded key to pass in sendmessage.
func EncryptMedia(data []byte) (encrypted []byte, aesKey string, err error) {
	key := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, "", fmt.Errorf("generate key: %w", err)
	}
	enc, err := encryptAES128ECB(data, key)
	if err != nil {
		return nil, "", err
	}
	return enc, base64.StdEncoding.EncodeToString(key), nil
}

// DecryptMedia decrypts AES-128-ECB encrypted media using a base64-encoded key.
func DecryptMedia(data []byte, aesKey string) ([]byte, error) {
	key, err := parseAesKey(aesKey)
	if err != nil {
		return nil, err
	}
	return decryptAES128ECB(data, key)
}

func parseAesKey(aesKeyBase64 string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(aesKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("decode aes key: %w", err)
	}
	if len(decoded) == 16 {
		return decoded, nil
	}
	if len(decoded) == 32 && hexKeyPattern.Match(decoded) {
		key, err := hex.DecodeString(string(decoded))
		if err != nil {
			return nil, fmt.Errorf("decode hex aes key: %w", err)
		}
		return key, nil
	}
	return nil, fmt.Errorf("aes key must decode to 16 raw bytes or 32-char hex string, got %d bytes", len(decoded))
}

func encryptAES128ECB(data, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}
	padded := pkcs7Pad(data, aes.BlockSize)
	out := make([]byte, len(padded))
	for i := 0; i < len(padded); i += aes.BlockSize {
		block.Encrypt(out[i:i+aes.BlockSize], padded[i:i+aes.BlockSize])
	}
	return out, nil
}

func decryptAES128ECB(data, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}
	if len(data)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext length not a multiple of block size")
	}
	out := make([]byte, len(data))
	for i := 0; i < len(data); i += aes.BlockSize {
		block.Decrypt(out[i:i+aes.BlockSize], data[i:i+aes.BlockSize])
	}
	return pkcs7Unpad(out)
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padded := make([]byte, len(data)+padding)
	copy(padded, data)
	for i := len(data); i < len(padded); i++ {
		padded[i] = byte(padding)
	}
	return padded
}

func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}
	padding := int(data[len(data)-1])
	if padding == 0 || padding > aes.BlockSize {
		return nil, fmt.Errorf("invalid pkcs7 padding byte: %d", padding)
	}
	return data[:len(data)-padding], nil
}
