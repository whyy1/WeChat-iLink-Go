package ilink

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
)

const (
	UploadMediaTypeImage = 1
	UploadMediaTypeVideo = 2
	UploadMediaTypeFile  = 3
	UploadMediaTypeVoice = 4
)

var hexKeyPattern = regexp.MustCompile("^[0-9a-fA-F]{32}$")

type DownloadMediaRequest struct {
	CDNURL string
	AesKey string
}

type UploadMediaRequest struct {
	ToUserID string
	Data     []byte
	ItemType int
}

type GetUploadURLRequest struct {
	ToUserID     string
	MediaType    int
	FileKey      string
	RawSize      int64
	RawFileMD5   string
	FileSize     int64
	ThumbRawSize int64
	ThumbRawMD5  string
	ThumbSize    int64
	NoNeedThumb  bool
	AesKeyHex    string
}

type receivedMedia struct {
	media  *MediaContent
	hexKey string
}

type uploadMetadata struct {
	fileKey    string
	aesKeyRaw  []byte
	rawSize    int64
	rawFileMD5 string
	cipherSize int64
}

type uploadURLEnvelope struct {
	FileKey         string   `json:"filekey"`
	MediaType       int      `json:"media_type"`
	ToUserID        string   `json:"to_user_id,omitempty"`
	RawSize         int64    `json:"rawsize"`
	RawFileMD5      string   `json:"rawfilemd5"`
	FileSize        int64    `json:"filesize"`
	ThumbRawSize    int64    `json:"thumb_rawsize,omitempty"`
	ThumbRawFileMD5 string   `json:"thumb_rawfilemd5,omitempty"`
	ThumbFileSize   int64    `json:"thumb_filesize,omitempty"`
	NoNeedThumb     bool     `json:"no_need_thumb,omitempty"`
	AesKey          string   `json:"aeskey,omitempty"`
	BaseInfo        BaseInfo `json:"base_info"`
}

type UploadURLResponse struct {
	baseResponse
	UploadParam      string `json:"upload_param"`
	ThumbUploadParam string `json:"thumb_upload_param,omitempty"`
}

func extractReceivedMedia(item Item) (*receivedMedia, error) {
	switch item.Type {
	case ItemTypeImage:
		if item.ImageItem == nil || item.ImageItem.Media == nil {
			return nil, fmt.Errorf("image item missing media info")
		}
		return &receivedMedia{media: item.ImageItem.Media, hexKey: item.ImageItem.AesKeyHex}, nil
	case ItemTypeVoice:
		if item.VoiceItem == nil || item.VoiceItem.Media == nil {
			return nil, fmt.Errorf("voice item missing media info")
		}
		return &receivedMedia{media: item.VoiceItem.Media, hexKey: item.VoiceItem.AesKeyHex}, nil
	case ItemTypeFile:
		if item.FileItem == nil || item.FileItem.Media == nil {
			return nil, fmt.Errorf("file item missing media info")
		}
		return &receivedMedia{media: item.FileItem.Media, hexKey: item.FileItem.AesKeyHex}, nil
	case ItemTypeVideo:
		if item.VideoItem == nil || item.VideoItem.Media == nil {
			return nil, fmt.Errorf("video item missing media info")
		}
		return &receivedMedia{media: item.VideoItem.Media, hexKey: item.VideoItem.AesKeyHex}, nil
	default:
		return nil, fmt.Errorf("unsupported item type %d", item.Type)
	}
}

func resolveReceivedMedia(media *MediaContent, hexKey string, cdnBaseURL string) (cdnURLs []string, aesKey string, err error) {
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
		addCandidate(cdnBaseURL + "/download?encrypted_query_param=" + encryptQueryParam)
		addCandidate(cdnBaseURL + "/download?encrypted_query_param=" + url.QueryEscape(encryptQueryParam))
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

func (c *Client) DownloadMedia(ctx context.Context, req DownloadMediaRequest) ([]byte, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, req.CDNURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	resp, err := c.httpClient.Do(httpReq)
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
	return DecryptMedia(encrypted, req.AesKey)
}

func (c *Client) DownloadReceivedMedia(ctx context.Context, item Item) ([]byte, error) {
	rm, err := extractReceivedMedia(item)
	if err != nil {
		return nil, err
	}
	cdnURLs, aesKey, err := resolveReceivedMedia(rm.media, rm.hexKey, c.cdnBaseURL)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for _, cdnURL := range cdnURLs {
		if c.Debug {
			fmt.Fprintf(os.Stderr, "[ilink] received media type=%d cdn_url=%s\n", item.Type, cdnURL)
		}
		data, err := c.DownloadMedia(ctx, DownloadMediaRequest{CDNURL: cdnURL, AesKey: aesKey})
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

func (c *Client) GetUploadURL(ctx context.Context, req GetUploadURLRequest) (*UploadURLResponse, error) {
	var resp UploadURLResponse
	body := uploadURLEnvelope{
		FileKey:         req.FileKey,
		MediaType:       req.MediaType,
		ToUserID:        req.ToUserID,
		RawSize:         req.RawSize,
		RawFileMD5:      req.RawFileMD5,
		FileSize:        req.FileSize,
		ThumbRawSize:    req.ThumbRawSize,
		ThumbRawFileMD5: req.ThumbRawMD5,
		ThumbFileSize:   req.ThumbSize,
		NoNeedThumb:     req.NoNeedThumb,
		AesKey:          req.AesKeyHex,
		BaseInfo:        buildBaseInfo(),
	}
	if err := c.doJSON(ctx, http.MethodPost, "/ilink/bot/getuploadurl", body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func uploadBufferToCDN(ctx context.Context, httpClient *http.Client, cdnBaseURL string, plaintext []byte, uploadParam, fileKey string, aesKeyRaw []byte) (string, error) {
	ciphertext, err := encryptAES128ECB(plaintext, aesKeyRaw)
	if err != nil {
		return "", err
	}
	cdnURL := cdnBaseURL + "/upload?encrypted_query_param=" + url.QueryEscape(uploadParam) + "&filekey=" + url.QueryEscape(fileKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cdnURL, bytes.NewReader(ciphertext))
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

func buildUploadMetadata(data []byte) (*uploadMetadata, error) {
	fileKey, err := randomHex(16)
	if err != nil {
		return nil, err
	}
	aesKeyRaw := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, aesKeyRaw); err != nil {
		return nil, fmt.Errorf("generate aes key: %w", err)
	}
	rawMD5 := md5.Sum(data)
	return &uploadMetadata{
		fileKey:    fileKey,
		aesKeyRaw:  aesKeyRaw,
		rawSize:    int64(len(data)),
		rawFileMD5: hex.EncodeToString(rawMD5[:]),
		cipherSize: int64(len(pkcs7Pad(data, aes.BlockSize))),
	}, nil
}

func (c *Client) UploadMedia(ctx context.Context, req UploadMediaRequest) (*UploadedMedia, error) {
	mediaType, err := mapItemTypeToUploadMediaType(req.ItemType)
	if err != nil {
		return nil, err
	}
	meta, err := buildUploadMetadata(req.Data)
	if err != nil {
		return nil, err
	}

	urlResp, err := c.GetUploadURL(ctx, GetUploadURLRequest{
		ToUserID:    req.ToUserID,
		MediaType:   mediaType,
		FileKey:     meta.fileKey,
		RawSize:     meta.rawSize,
		RawFileMD5:  meta.rawFileMD5,
		FileSize:    meta.cipherSize,
		NoNeedThumb: true,
		AesKeyHex:   hex.EncodeToString(meta.aesKeyRaw),
	})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(urlResp.UploadParam) == "" {
		return nil, fmt.Errorf("getuploadurl returned empty upload_param")
	}

	downloadParam, err := uploadBufferToCDN(ctx, c.httpClient, c.cdnBaseURL, req.Data, urlResp.UploadParam, meta.fileKey, meta.aesKeyRaw)
	if err != nil {
		return nil, err
	}

	return &UploadedMedia{
		FileKey:           meta.fileKey,
		EncryptQueryParam: downloadParam,
		AesKey:            base64.StdEncoding.EncodeToString([]byte(hex.EncodeToString(meta.aesKeyRaw))),
		PlainSize:         meta.rawSize,
		CipherSize:        meta.cipherSize,
	}, nil
}

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

func DecryptMedia(data []byte, aesKey string) ([]byte, error) {
	key, err := parseAesKey(aesKey)
	if err != nil {
		return nil, err
	}
	return decryptAES128ECB(data, key)
}

// UploadMedia uploads media without specifying a target user.
// The server assigns a default recipient context. For user-scoped uploads, use UploadMediaForUser.
func (c *Client) UploadMediaSimple(data []byte, itemType int) (*MediaInfo, error) {
	return c.UploadMediaForUserSimple("", data, itemType)
}

// UploadMediaForUser uploads media on behalf of a specific target user.
// It performs the full upload flow: compute metadata → getUploadUrl → encrypt → PUT to CDN.
func (c *Client) UploadMediaForUserSimple(toUserID string, data []byte, itemType int) (*MediaInfo, error) {
	uploaded, err := c.UploadMedia(context.Background(), UploadMediaRequest{
		ToUserID: toUserID,
		Data:     data,
		ItemType: itemType,
	})
	if err != nil {
		return nil, err
	}
	return &MediaInfo{
		FileKey:                     uploaded.FileKey,
		DownloadEncryptedQueryParam: uploaded.EncryptQueryParam,
		AesKey:                      uploaded.AesKey,
		FileSize:                    uploaded.PlainSize,
		FileSizeCiphertext:          uploaded.CipherSize,
	}, nil
}

// Convenience wrappers matching the README public API (no context.Context).

func (c *Client) GetUploadURLSimple(req GetUploadURLRequest) (*UploadURLResponse, error) {
	return c.GetUploadURL(context.Background(), req)
}

func (c *Client) DownloadMediaSimple(cdnURL, aesKey string) ([]byte, error) {
	return c.DownloadMedia(context.Background(), DownloadMediaRequest{CDNURL: cdnURL, AesKey: aesKey})
}

func (c *Client) DownloadReceivedMediaSimple(item Item) ([]byte, error) {
	return c.DownloadReceivedMedia(context.Background(), item)
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
