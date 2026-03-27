package ilink

import (
	"bytes"
	"crypto/aes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// DownloadReceivedMedia downloads and decrypts a received media item (image/voice/file/video).
// It constructs the CDN URL from media.encrypt_query_param and hex-decodes the aeskey field.
func (c *Client) DownloadReceivedMedia(item Item) ([]byte, error) {
	var encryptQueryParam, hexKey string
	switch item.Type {
	case ItemTypeImage:
		if item.ImageItem == nil || item.ImageItem.Media == nil {
			return nil, fmt.Errorf("image item missing media info")
		}
		encryptQueryParam = item.ImageItem.Media.EncryptQueryParam
		hexKey = item.ImageItem.AesKeyHex
	case ItemTypeVoice:
		if item.VoiceItem == nil || item.VoiceItem.Media == nil {
			return nil, fmt.Errorf("voice item missing media info")
		}
		encryptQueryParam = item.VoiceItem.Media.EncryptQueryParam
		hexKey = item.VoiceItem.AesKeyHex
	case ItemTypeFile:
		if item.FileItem == nil || item.FileItem.Media == nil {
			return nil, fmt.Errorf("file item missing media info")
		}
		encryptQueryParam = item.FileItem.Media.EncryptQueryParam
		hexKey = item.FileItem.AesKeyHex
	case ItemTypeVideo:
		if item.VideoItem == nil || item.VideoItem.Media == nil {
			return nil, fmt.Errorf("video item missing media info")
		}
		encryptQueryParam = item.VideoItem.Media.EncryptQueryParam
		hexKey = item.VideoItem.AesKeyHex
	default:
		return nil, fmt.Errorf("unsupported item type %d", item.Type)
	}

	return c.downloadAndDecryptHex(encryptQueryParam, hexKey)
}

// downloadAndDecryptHex recovers a received media file in two steps:
//  1. Base64 (URL-safe) decode + AES-128-ECB decrypt encrypt_query_param → CDN download URL
//  2. GET the CDN URL → AES-128-ECB decrypt the response body → plaintext file
func (c *Client) downloadAndDecryptHex(encryptQueryParam, hexKey string) ([]byte, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("decode hex key: %w", err)
	}

	// Step 1: decrypt encrypt_query_param to get the CDN URL
	encryptedURL, err := base64.URLEncoding.DecodeString(encryptQueryParam)
	if err != nil {
		return nil, fmt.Errorf("base64 decode encrypt_query_param: %w", err)
	}
	cdnURLBytes, err := decryptAES128ECB(encryptedURL, key)
	if err != nil {
		return nil, fmt.Errorf("decrypt cdn url: %w", err)
	}
	cdnURL := string(cdnURLBytes)
	if !strings.HasPrefix(cdnURL, "http") {
		cdnURL = CDNBaseURL + "?" + cdnURL
	}
	if c.Debug {
		fmt.Fprintf(os.Stderr, "[ilink] cdn URL (decrypted): %s\n", cdnURL)
	}

	// Step 2: download the AES-encrypted file content
	resp, err := c.httpClient.Get(cdnURL)
	if err != nil {
		return nil, fmt.Errorf("cdn download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cdn download status %d", resp.StatusCode)
	}
	encryptedFile, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cdn read: %w", err)
	}

	// Step 3: decrypt the file content with the same key
	return decryptAES128ECB(encryptedFile, key)
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

// UploadURLRequest is the body sent to /ilink/bot/getuploadurl
type UploadURLRequest struct {
	FileType int   `json:"file_type"`
	FileSize int64 `json:"file_size"`
}

// UploadURLResponse is returned by GetUploadURL
type UploadURLResponse struct {
	baseResponse
	UploadURL string `json:"upload_url"`
	CDNUrl    string `json:"cdn_url"`
	FileKey   string `json:"file_key,omitempty"`
}

// GetUploadURL requests a CDN pre-signed upload URL for a file of the given type and encrypted size.
func (c *Client) GetUploadURL(fileType int, fileSize int64) (*UploadURLResponse, error) {
	data, err := c.do(http.MethodPost, "/ilink/bot/getuploadurl", UploadURLRequest{
		FileType: fileType,
		FileSize: fileSize,
	})
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

// MediaInfo holds the CDN URL and base64-encoded AES key after a successful upload
type MediaInfo struct {
	CDNUrl string
	AesKey string // base64-encoded AES-128 key; include verbatim in sendmessage
}

// UploadMedia encrypts data with AES-128-ECB, obtains a pre-signed upload URL,
// PUTs the encrypted bytes to the CDN, and returns the CDN URL and AES key.
func (c *Client) UploadMedia(data []byte, fileType int) (*MediaInfo, error) {
	encrypted, aesKey, err := EncryptMedia(data)
	if err != nil {
		return nil, err
	}
	urlResp, err := c.GetUploadURL(fileType, int64(len(encrypted)))
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPut, urlResp.UploadURL, bytes.NewReader(encrypted))
	if err != nil {
		return nil, fmt.Errorf("create upload request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return nil, fmt.Errorf("upload failed with status %d", resp.StatusCode)
	}
	return &MediaInfo{CDNUrl: urlResp.CDNUrl, AesKey: aesKey}, nil
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
	key, err := base64.StdEncoding.DecodeString(aesKey)
	if err != nil {
		return nil, fmt.Errorf("decode aes key: %w", err)
	}
	return decryptAES128ECB(data, key)
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
