package ilink

import (
	"bytes"
	"crypto/aes"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// --- AES-128-ECB encrypt/decrypt round-trip tests ---

func TestPKCS7PadUnpad(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"1 byte", []byte{0x01}},
		{"15 bytes", make([]byte, 15)},
		{"16 bytes (full block)", make([]byte, 16)},
		{"17 bytes", make([]byte, 17)},
		{"31 bytes", make([]byte, 31)},
		{"32 bytes (2 blocks)", make([]byte, 32)},
		{"1000 bytes", make([]byte, 1000)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			padded := pkcs7Pad(tt.data, aes.BlockSize)
			if len(padded)%aes.BlockSize != 0 {
				t.Fatalf("padded length %d is not a multiple of block size", len(padded))
			}
			unpadded, err := pkcs7Unpad(padded)
			if err != nil {
				t.Fatalf("unpad error: %v", err)
			}
			if !bytes.Equal(unpadded, tt.data) {
				t.Fatalf("unpadded data doesn't match original")
			}
		})
	}
}

func TestPKCS7PadPaddingValues(t *testing.T) {
	// 15 bytes should get 1 byte of padding (0x01)
	padded := pkcs7Pad(make([]byte, 15), aes.BlockSize)
	if len(padded) != 16 {
		t.Fatalf("expected 16 bytes, got %d", len(padded))
	}
	if padded[15] != 0x01 {
		t.Fatalf("expected padding byte 0x01, got 0x%02x", padded[15])
	}

	// 16 bytes should get 16 bytes of padding (0x10)
	padded = pkcs7Pad(make([]byte, 16), aes.BlockSize)
	if len(padded) != 32 {
		t.Fatalf("expected 32 bytes, got %d", len(padded))
	}
	for i := 16; i < 32; i++ {
		if padded[i] != 0x10 {
			t.Fatalf("expected padding byte 0x10 at position %d, got 0x%02x", i, padded[i])
		}
	}
}

func TestPKCS7UnpadInvalid(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"zero padding", []byte{0x00}},
		{"padding too large", []byte{0x11}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := pkcs7Unpad(tt.data)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestEncryptDecryptAES128ECB(t *testing.T) {
	key := []byte("0123456789abcdef") // 16 bytes
	plaintext := []byte("Hello, WeChat iLink Bot!")

	ciphertext, err := encryptAES128ECB(plaintext, key)
	if err != nil {
		t.Fatalf("encrypt error: %v", err)
	}
	if len(ciphertext)%aes.BlockSize != 0 {
		t.Fatalf("ciphertext length %d not a multiple of block size", len(ciphertext))
	}
	if len(ciphertext) <= len(plaintext) {
		t.Fatalf("ciphertext should be larger than plaintext due to padding")
	}

	decrypted, err := decryptAES128ECB(ciphertext, key)
	if err != nil {
		t.Fatalf("decrypt error: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("decrypted data doesn't match original\ngot:  %x\nwant: %x", decrypted, plaintext)
	}
}

func TestEncryptDecryptAES128ECBVariousSizes(t *testing.T) {
	key := []byte("abcdefghijklmnop") // 16 bytes

	for size := 0; size <= 256; size++ {
		data := make([]byte, size)
		for i := range data {
			data[i] = byte(i)
		}

		enc, err := encryptAES128ECB(data, key)
		if err != nil {
			t.Fatalf("encrypt size=%d: %v", size, err)
		}
		dec, err := decryptAES128ECB(enc, key)
		if err != nil {
			t.Fatalf("decrypt size=%d: %v", size, err)
		}
		if !bytes.Equal(dec, data) {
			t.Fatalf("round-trip failed for size=%d", size)
		}
	}
}

// --- EncryptMedia / DecryptMedia public API tests ---

func TestEncryptDecryptMedia(t *testing.T) {
	data := []byte("test media content for round-trip")

	encrypted, aesKey, err := EncryptMedia(data)
	if err != nil {
		t.Fatalf("EncryptMedia error: %v", err)
	}
	if len(encrypted) == 0 {
		t.Fatal("encrypted data is empty")
	}
	if aesKey == "" {
		t.Fatal("aes key is empty")
	}
	if bytes.Equal(encrypted, data) {
		t.Fatal("encrypted data should differ from plaintext")
	}

	decrypted, err := DecryptMedia(encrypted, aesKey)
	if err != nil {
		t.Fatalf("DecryptMedia error: %v", err)
	}
	if !bytes.Equal(decrypted, data) {
		t.Fatalf("decrypted data doesn't match original")
	}
}

func TestDecryptMediaWithRawKey(t *testing.T) {
	// Test that DecryptMedia works with a raw 16-byte key (base64 encoded)
	key := []byte("0123456789abcdef")
	data := []byte("test with raw key")

	encrypted, err := encryptAES128ECB(data, key)
	if err != nil {
		t.Fatalf("encrypt error: %v", err)
	}

	aesKeyBase64 := base64.StdEncoding.EncodeToString(key)
	decrypted, err := DecryptMedia(encrypted, aesKeyBase64)
	if err != nil {
		t.Fatalf("DecryptMedia with raw key: %v", err)
	}
	if !bytes.Equal(decrypted, data) {
		t.Fatal("decrypted data doesn't match original")
	}
}

func TestDecryptMediaWithHexKey(t *testing.T) {
	// Test that DecryptMedia works with a hex-encoded key (base64 of hex string)
	key := []byte("0123456789abcdef")
	hexKey := hex.EncodeToString(key) // "30313233343536373839616263646566"
	data := []byte("test with hex key")

	encrypted, err := encryptAES128ECB(data, key)
	if err != nil {
		t.Fatalf("encrypt error: %v", err)
	}

	aesKeyBase64 := base64.StdEncoding.EncodeToString([]byte(hexKey))
	decrypted, err := DecryptMedia(encrypted, aesKeyBase64)
	if err != nil {
		t.Fatalf("DecryptMedia with hex key: %v", err)
	}
	if !bytes.Equal(decrypted, data) {
		t.Fatal("decrypted data doesn't match original")
	}
}

// --- parseAesKey tests ---

func TestParseAesKey(t *testing.T) {
	t.Run("raw 16-byte key", func(t *testing.T) {
		raw := []byte("0123456789abcdef")
		b64 := base64.StdEncoding.EncodeToString(raw)
		parsed, err := parseAesKey(b64)
		if err != nil {
			t.Fatalf("parseAesKey error: %v", err)
		}
		if !bytes.Equal(parsed, raw) {
			t.Fatal("parsed key doesn't match original")
		}
	})

	t.Run("hex-encoded key", func(t *testing.T) {
		raw := []byte("0123456789abcdef")
		hexStr := hex.EncodeToString(raw)
		b64 := base64.StdEncoding.EncodeToString([]byte(hexStr))
		parsed, err := parseAesKey(b64)
		if err != nil {
			t.Fatalf("parseAesKey error: %v", err)
		}
		if !bytes.Equal(parsed, raw) {
			t.Fatal("parsed key doesn't match original")
		}
	})

	t.Run("invalid length", func(t *testing.T) {
		raw := []byte("too-short")
		b64 := base64.StdEncoding.EncodeToString(raw)
		_, err := parseAesKey(b64)
		if err == nil {
			t.Fatal("expected error for invalid key length")
		}
	})
}

// --- JSON serialization tests ---

func TestMessageJSONMarshal(t *testing.T) {
	msg := Message{
		ToUserID:     "user123",
		MessageType:  MessageTypeBot,
		MessageState: MessageStateNormal,
		ContextToken: "ctx_token_abc",
		ItemList: []Item{
			{
				Type:     ItemTypeText,
				TextItem: &TextItem{Text: "Hello!"},
			},
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.ToUserID != msg.ToUserID {
		t.Fatalf("to_user_id mismatch: got %q, want %q", decoded.ToUserID, msg.ToUserID)
	}
	if decoded.ContextToken != msg.ContextToken {
		t.Fatalf("context_token mismatch: got %q, want %q", decoded.ContextToken, msg.ContextToken)
	}
	if len(decoded.ItemList) != 1 {
		t.Fatalf("item_list length: got %d, want 1", len(decoded.ItemList))
	}
	if decoded.ItemList[0].Type != ItemTypeText {
		t.Fatalf("item type: got %d, want %d", decoded.ItemList[0].Type, ItemTypeText)
	}
	if decoded.ItemList[0].TextItem == nil || decoded.ItemList[0].TextItem.Text != "Hello!" {
		t.Fatal("text_item mismatch")
	}
}

func TestImageItemJSONRoundTrip(t *testing.T) {
	item := ImageItem{
		URL:       "https://example.com/img.png",
		AesKeyHex: "30313233343536373839616263646566",
		Media: &MediaContent{
			EncryptQueryParam: "eqp_value",
			AesKey:            "aes_key_value",
			EncryptType:       1,
		},
		MidSize: 1024,
	}

	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded ImageItem
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.URL != item.URL {
		t.Fatalf("url mismatch")
	}
	if decoded.AesKeyHex != item.AesKeyHex {
		t.Fatalf("aeskey mismatch")
	}
	if decoded.Media == nil || decoded.Media.EncryptQueryParam != "eqp_value" {
		t.Fatalf("media mismatch")
	}
	if decoded.MidSize != 1024 {
		t.Fatalf("mid_size mismatch: got %d, want 1024", decoded.MidSize)
	}
}

func TestUpdatesResponseJSON(t *testing.T) {
	jsonStr := `{
		"ret": 0,
		"msgs": [
			{
				"from_user_id": "user1",
				"to_user_id": "bot1",
				"message_type": 1,
				"context_token": "token1",
				"item_list": [
					{"type": 1, "text_item": {"text": "hi"}}
				]
			}
		],
		"get_updates_buf": "cursor123",
		"longpolling_timeout_ms": 35000
	}`

	var resp UpdatesResponse
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.Ret != 0 {
		t.Fatalf("ret: got %d, want 0", resp.Ret)
	}
	if len(resp.Msgs) != 1 {
		t.Fatalf("msgs length: got %d, want 1", len(resp.Msgs))
	}
	if resp.Msgs[0].FromUserID != "user1" {
		t.Fatalf("from_user_id: got %q, want %q", resp.Msgs[0].FromUserID, "user1")
	}
	if resp.GetUpdatesBuf != "cursor123" {
		t.Fatalf("get_updates_buf: got %q, want %q", resp.GetUpdatesBuf, "cursor123")
	}
}

// --- Client construction tests ---

func TestNewClient(t *testing.T) {
	t.Run("with bot token containing botID", func(t *testing.T) {
		c := NewClient("12345:abcdef")
		if c.botID != "12345" {
			t.Fatalf("botID: got %q, want %q", c.botID, "12345")
		}
		if c.botToken != "12345:abcdef" {
			t.Fatalf("botToken: got %q, want %q", c.botToken, "12345:abcdef")
		}
	})

	t.Run("with empty token", func(t *testing.T) {
		c := NewClient("")
		if c.botID != "" {
			t.Fatalf("botID should be empty, got %q", c.botID)
		}
	})

	t.Run("with options", func(t *testing.T) {
		c := NewClient("token", WithDebug(true), WithBaseURL("https://custom.url/"))
		if !c.Debug {
			t.Fatal("debug should be true")
		}
		if c.baseURL != "https://custom.url" {
			t.Fatalf("baseURL: got %q, want %q", c.baseURL, "https://custom.url")
		}
	})
}

func TestBuildHeaders(t *testing.T) {
	c := NewClient("test_token")
	h := c.buildHeaders()

	if h.Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type: got %q", h.Get("Content-Type"))
	}
	if h.Get("AuthorizationType") != "ilink_bot_token" {
		t.Fatalf("AuthorizationType: got %q", h.Get("AuthorizationType"))
	}
	if h.Get("Authorization") != "Bearer test_token" {
		t.Fatalf("Authorization: got %q", h.Get("Authorization"))
	}
	if h.Get("X-WECHAT-UIN") == "" {
		t.Fatal("X-WECHAT-UIN should not be empty")
	}

	// X-WECHAT-UIN should change on each call
	h2 := c.buildHeaders()
	if h.Get("X-WECHAT-UIN") == h2.Get("X-WECHAT-UIN") {
		// This is probabilistic; extremely unlikely to fail if random is working
		t.Log("Warning: X-WECHAT-UIN was same on two calls (extremely unlikely)")
	}
}

func TestGenerateWechatUIN(t *testing.T) {
	uin := generateWechatUIN()
	if uin == "" {
		t.Fatal("UIN should not be empty")
	}
	// Should be valid base64
	decoded, err := base64.StdEncoding.DecodeString(uin)
	if err != nil {
		t.Fatalf("UIN should be valid base64: %v", err)
	}
	// Should decode to a decimal string of a uint32
	var n uint32
	_, err = fmt.Sscanf(string(decoded), "%d", &n)
	if err != nil {
		t.Fatalf("UIN should decode to a uint32 decimal string: %v", err)
	}
}

// --- buildUploadMetadata tests ---

func TestBuildUploadMetadata(t *testing.T) {
	data := []byte("test file content")
	meta, err := buildUploadMetadata(data)
	if err != nil {
		t.Fatalf("buildUploadMetadata error: %v", err)
	}

	if meta.fileKey == "" {
		t.Fatal("fileKey should not be empty")
	}
	if len(meta.aesKeyRaw) != 16 {
		t.Fatalf("aesKeyRaw length: got %d, want 16", len(meta.aesKeyRaw))
	}
	if meta.rawSize != int64(len(data)) {
		t.Fatalf("rawSize: got %d, want %d", meta.rawSize, len(data))
	}

	// Verify MD5
	expectedMD5 := md5.Sum(data)
	if meta.rawFileMD5 != hex.EncodeToString(expectedMD5[:]) {
		t.Fatalf("rawFileMD5 mismatch: got %q", meta.rawFileMD5)
	}

	// Verify cipher size (should be padded to next block boundary)
	expectedCipherSize := int64(len(pkcs7Pad(data, aes.BlockSize)))
	if meta.cipherSize != expectedCipherSize {
		t.Fatalf("cipherSize: got %d, want %d", meta.cipherSize, expectedCipherSize)
	}
}

// --- MediaInfo type test ---

func TestMediaInfoFields(t *testing.T) {
	info := MediaInfo{
		FileKey:                     "abc123",
		DownloadEncryptedQueryParam: "eqp_value",
		AesKey:                      "aes_key_b64",
		FileSize:                    1024,
		FileSizeCiphertext:          1040,
	}
	if info.FileKey != "abc123" {
		t.Fatal("FileKey mismatch")
	}
	if info.DownloadEncryptedQueryParam != "eqp_value" {
		t.Fatal("DownloadEncryptedQueryParam mismatch")
	}
}

// --- Constants tests ---

func TestConstants(t *testing.T) {
	if ItemTypeText != 1 {
		t.Fatalf("ItemTypeText: got %d, want 1", ItemTypeText)
	}
	if ItemTypeImage != 2 {
		t.Fatalf("ItemTypeImage: got %d, want 2", ItemTypeImage)
	}
	if ItemTypeVoice != 3 {
		t.Fatalf("ItemTypeVoice: got %d, want 3", ItemTypeVoice)
	}
	if ItemTypeFile != 4 {
		t.Fatalf("ItemTypeFile: got %d, want 4", ItemTypeFile)
	}
	if ItemTypeVideo != 5 {
		t.Fatalf("ItemTypeVideo: got %d, want 5", ItemTypeVideo)
	}
	if MessageTypeUser != 1 {
		t.Fatalf("MessageTypeUser: got %d, want 1", MessageTypeUser)
	}
	if MessageTypeBot != 2 {
		t.Fatalf("MessageTypeBot: got %d, want 2", MessageTypeBot)
	}
	if TypingStatusOn != 1 {
		t.Fatalf("TypingStatusOn: got %d, want 1", TypingStatusOn)
	}
	if TypingStatusOff != 2 {
		t.Fatalf("TypingStatusOff: got %d, want 2", TypingStatusOff)
	}
	if ChannelVersion != "1.0.2" {
		t.Fatalf("ChannelVersion: got %q, want %q", ChannelVersion, "1.0.2")
	}
}

// --- Helper function tests ---

func TestMapItemTypeToUploadMediaType(t *testing.T) {
	tests := []struct {
		itemType   int
		mediaType  int
		shouldFail bool
	}{
		{ItemTypeImage, UploadMediaTypeImage, false},
		{ItemTypeVideo, UploadMediaTypeVideo, false},
		{ItemTypeFile, UploadMediaTypeFile, false},
		{ItemTypeVoice, UploadMediaTypeVoice, false},
		{99, 0, true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("itemType_%d", tt.itemType), func(t *testing.T) {
			result, err := mapItemTypeToUploadMediaType(tt.itemType)
			if tt.shouldFail {
				if err == nil {
					t.Fatal("expected error")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if result != tt.mediaType {
					t.Fatalf("media type: got %d, want %d", result, tt.mediaType)
				}
			}
		})
	}
}

func TestRandomHex(t *testing.T) {
	h, err := randomHex(16)
	if err != nil {
		t.Fatalf("randomHex error: %v", err)
	}
	if len(h) != 32 { // 16 bytes = 32 hex chars
		t.Fatalf("hex length: got %d, want 32", len(h))
	}
	// Should be different on each call
	h2, _ := randomHex(16)
	if h == h2 {
		t.Log("Warning: two random hex values were equal (extremely unlikely)")
	}
}

// --- Fuzz-style test for AES round-trip ---

func TestAESRoundTripFuzz(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 100; i++ {
		size := rng.Intn(4096)
		data := make([]byte, size)
		rng.Read(data)

		key := make([]byte, 16)
		rng.Read(key)

		enc, err := encryptAES128ECB(data, key)
		if err != nil {
			t.Fatalf("encrypt i=%d size=%d: %v", i, size, err)
		}
		dec, err := decryptAES128ECB(enc, key)
		if err != nil {
			t.Fatalf("decrypt i=%d size=%d: %v", i, size, err)
		}
		if !bytes.Equal(dec, data) {
			t.Fatalf("round-trip failed at i=%d size=%d", i, size)
		}
	}
}

// --- JSON edge case tests ---

func TestBaseResponseError(t *testing.T) {
	tests := []struct {
		name    string
		resp    baseResponse
		wantErr bool
	}{
		{"success", baseResponse{Ret: 0, ErrCode: 0}, false},
		{"ret error", baseResponse{Ret: -1, ErrCode: 0}, true},
		{"errcode error", baseResponse{Ret: 0, ErrCode: -14}, true},
		{"errcode takes priority", baseResponse{Ret: 0, ErrCode: -14, ErrMsg: "session timeout"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.resp.err()
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantErr && tt.resp.ErrMsg != "" {
				if !strings.Contains(err.Error(), tt.resp.ErrMsg) {
					t.Fatalf("error should contain %q: %v", tt.resp.ErrMsg, err)
				}
			}
		})
	}
}

func TestNewOutboundMessage(t *testing.T) {
	msg := newOutboundMessage("user1", "token1", Item{
		Type:     ItemTypeText,
		TextItem: &TextItem{Text: "hello"},
	})
	if msg.ToUserID != "user1" {
		t.Fatalf("to_user_id: got %q, want %q", msg.ToUserID, "user1")
	}
	if msg.ContextToken != "token1" {
		t.Fatalf("context_token: got %q, want %q", msg.ContextToken, "token1")
	}
	if msg.MessageType != MessageTypeBot {
		t.Fatalf("message_type: got %d, want %d", msg.MessageType, MessageTypeBot)
	}
	if msg.MessageState != MessageStateNormal {
		t.Fatalf("message_state: got %d, want %d", msg.MessageState, MessageStateNormal)
	}
}

func TestNewMediaRef(t *testing.T) {
	media := UploadedMedia{
		EncryptQueryParam: "eqp_test",
		AesKey:            "aes_test",
	}
	ref := newMediaRef(media)
	if ref.EncryptQueryParam != "eqp_test" {
		t.Fatalf("encrypt_query_param: got %q", ref.EncryptQueryParam)
	}
	if ref.AesKey != "aes_test" {
		t.Fatalf("aes_key: got %q", ref.AesKey)
	}
	if ref.EncryptType != 1 {
		t.Fatalf("encrypt_type: got %d, want 1", ref.EncryptType)
	}
}

func TestGenerateClientID(t *testing.T) {
	id1 := generateClientID()
	id2 := generateClientID()
	if id1 == id2 {
		t.Fatal("client IDs should be unique")
	}
	if !strings.HasPrefix(id1, "ilink-go:") {
		t.Fatalf("client ID should start with 'ilink-go:': got %q", id1)
	}
}

// --- extractReceivedMedia tests ---

func TestExtractReceivedMedia(t *testing.T) {
	t.Run("image with media", func(t *testing.T) {
		item := Item{
			Type: ItemTypeImage,
			ImageItem: &ImageItem{
				AesKeyHex: "30313233343536373839616263646566",
				Media:     &MediaContent{EncryptQueryParam: "eqp1"},
			},
		}
		rm, err := extractReceivedMedia(item)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rm.hexKey != "30313233343536373839616263646566" {
			t.Fatalf("hexKey: got %q", rm.hexKey)
		}
		if rm.media.EncryptQueryParam != "eqp1" {
			t.Fatalf("encrypt_query_param: got %q", rm.media.EncryptQueryParam)
		}
	})

	t.Run("image without media", func(t *testing.T) {
		item := Item{Type: ItemTypeImage, ImageItem: &ImageItem{}}
		_, err := extractReceivedMedia(item)
		if err == nil {
			t.Fatal("expected error for nil media")
		}
	})

	t.Run("voice with media", func(t *testing.T) {
		item := Item{
			Type: ItemTypeVoice,
			VoiceItem: &VoiceItem{
				AesKeyHex: "aabbccdd",
				Media:     &MediaContent{EncryptQueryParam: "eqp_voice"},
			},
		}
		rm, err := extractReceivedMedia(item)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rm.hexKey != "aabbccdd" {
			t.Fatalf("hexKey: got %q", rm.hexKey)
		}
	})

	t.Run("file with media", func(t *testing.T) {
		item := Item{
			Type: ItemTypeFile,
			FileItem: &FileItem{
				AesKeyHex: "ff001122",
				Media:     &MediaContent{EncryptQueryParam: "eqp_file"},
			},
		}
		rm, err := extractReceivedMedia(item)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rm.media.EncryptQueryParam != "eqp_file" {
			t.Fatalf("encrypt_query_param: got %q", rm.media.EncryptQueryParam)
		}
	})

	t.Run("video with media", func(t *testing.T) {
		item := Item{
			Type: ItemTypeVideo,
			VideoItem: &VideoItem{
				AesKeyHex: "11223344",
				Media:     &MediaContent{EncryptQueryParam: "eqp_video"},
			},
		}
		rm, err := extractReceivedMedia(item)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rm.hexKey != "11223344" {
			t.Fatalf("hexKey: got %q", rm.hexKey)
		}
	})

	t.Run("unsupported type", func(t *testing.T) {
		item := Item{Type: 99}
		_, err := extractReceivedMedia(item)
		if err == nil {
			t.Fatal("expected error for unsupported type")
		}
	})

	t.Run("nil item field", func(t *testing.T) {
		item := Item{Type: ItemTypeImage, ImageItem: nil}
		_, err := extractReceivedMedia(item)
		if err == nil {
			t.Fatal("expected error for nil image item")
		}
	})
}

// --- resolveReceivedMedia tests ---

func TestResolveReceivedMedia(t *testing.T) {
	cdnBase := "https://novac2c.cdn.weixin.qq.com/c2c"

	t.Run("with full_url only", func(t *testing.T) {
		media := &MediaContent{FullURL: "https://cdn.example.com/file"}
		hexKey := "30313233343536373839616263646566"
		urls, key, err := resolveReceivedMedia(media, hexKey, cdnBase)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(urls) != 1 || urls[0] != "https://cdn.example.com/file" {
			t.Fatalf("urls: got %v", urls)
		}
		if key == "" {
			t.Fatal("expected non-empty key when hex key provided")
		}
	})

	t.Run("with encrypt_query_param", func(t *testing.T) {
		media := &MediaContent{EncryptQueryParam: "abc123"}
		hexKey := "30313233343536373839616263646566"
		urls, _, err := resolveReceivedMedia(media, hexKey, cdnBase)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(urls) < 1 {
			t.Fatal("expected at least one URL")
		}
		found := false
		for _, u := range urls {
			if strings.Contains(u, "abc123") {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected URL containing 'abc123', got %v", urls)
		}
	})

	t.Run("with hex key returns base64", func(t *testing.T) {
		media := &MediaContent{EncryptQueryParam: "eqp1"}
		hexKey := "30313233343536373839616263646566" // hex of "0123456789abcdef"
		_, key, err := resolveReceivedMedia(media, hexKey, cdnBase)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		decoded, err := base64.StdEncoding.DecodeString(key)
		if err != nil {
			t.Fatalf("key not valid base64: %v", err)
		}
		if !bytes.Equal(decoded, []byte("0123456789abcdef")) {
			t.Fatalf("decoded key mismatch: got %x", decoded)
		}
	})

	t.Run("with aes_key fallback", func(t *testing.T) {
		media := &MediaContent{
			EncryptQueryParam: "eqp1",
			AesKey:            "c2FtcGxlX2tleQ==",
		}
		_, key, err := resolveReceivedMedia(media, "", cdnBase)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if key != "c2FtcGxlX2tleQ==" {
			t.Fatalf("expected fallback to media.AesKey, got %q", key)
		}
	})

	t.Run("nil media", func(t *testing.T) {
		_, _, err := resolveReceivedMedia(nil, "", cdnBase)
		if err == nil {
			t.Fatal("expected error for nil media")
		}
	})

	t.Run("no urls available", func(t *testing.T) {
		media := &MediaContent{}
		_, _, err := resolveReceivedMedia(media, "", cdnBase)
		if err == nil {
			t.Fatal("expected error when no download URLs available")
		}
	})

	t.Run("no key available", func(t *testing.T) {
		media := &MediaContent{FullURL: "https://cdn.example.com/f"}
		_, _, err := resolveReceivedMedia(media, "", cdnBase)
		if err == nil {
			t.Fatal("expected error when no AES key available")
		}
	})

	t.Run("deduplicates URLs", func(t *testing.T) {
		media := &MediaContent{
			FullURL:           "https://cdn.example.com/same",
			EncryptQueryParam: "eqp1",
		}
		urls, _, _ := resolveReceivedMedia(media, "00112233445566778899aabbccddeeff", cdnBase)
		seen := map[string]bool{}
		for _, u := range urls {
			if seen[u] {
				t.Fatalf("duplicate URL found: %s", u)
			}
			seen[u] = true
		}
	})
}

// --- decodeJSON / decodeAPIResponse tests ---

func TestDecodeJSON(t *testing.T) {
	t.Run("valid JSON", func(t *testing.T) {
		var result baseResponse
		err := decodeJSON([]byte(`{"ret":0,"errmsg":"ok"}`), &result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Ret != 0 {
			t.Fatalf("ret: got %d, want 0", result.Ret)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		var result baseResponse
		err := decodeJSON([]byte(`{invalid`), &result)
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})
}

func TestDecodeAPIResponse(t *testing.T) {
	t.Run("successful response", func(t *testing.T) {
		var result baseResponse
		err := decodeAPIResponse([]byte(`{"ret":0,"errcode":0}`), &result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("error response", func(t *testing.T) {
		var result baseResponse
		err := decodeAPIResponse([]byte(`{"ret":-1,"errcode":-14,"errmsg":"timeout"}`), &result)
		if err == nil {
			t.Fatal("expected error for error response")
		}
		if !strings.Contains(err.Error(), "timeout") {
			t.Fatalf("error should contain 'timeout': %v", err)
		}
	})
}

// --- JSON serialization tests for more types ---

func TestVoiceItemJSONRoundTrip(t *testing.T) {
	item := VoiceItem{
		URL:           "https://example.com/voice.amr",
		AesKeyHex:     "aabbccdd",
		Media:         &MediaContent{EncryptQueryParam: "eqp_v", AesKey: "aes_v"},
		Transcription: "hello world",
		CDNUrl:        "https://cdn.example.com/v",
		AesKey:        "key_v",
	}
	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded VoiceItem
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.Transcription != "hello world" {
		t.Fatalf("transcription: got %q", decoded.Transcription)
	}
	if decoded.Media == nil || decoded.Media.EncryptQueryParam != "eqp_v" {
		t.Fatal("media mismatch")
	}
}

func TestFileItemJSONRoundTrip(t *testing.T) {
	item := FileItem{
		URL:       "https://example.com/file.pdf",
		AesKeyHex: "11223344",
		Media:     &MediaContent{EncryptQueryParam: "eqp_f"},
		FileName:  "report.pdf",
		FileSize:  2048,
		Len:       "2048",
		CDNUrl:    "https://cdn.example.com/f",
		AesKey:    "key_f",
	}
	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded FileItem
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.FileName != "report.pdf" {
		t.Fatalf("file_name: got %q", decoded.FileName)
	}
	if decoded.FileSize != 2048 {
		t.Fatalf("file_size: got %d", decoded.FileSize)
	}
}

func TestVideoItemJSONRoundTrip(t *testing.T) {
	item := VideoItem{
		URL:       "https://example.com/video.mp4",
		AesKeyHex: "55667788",
		Media:     &MediaContent{EncryptQueryParam: "eqp_vid"},
		CDNUrl:    "https://cdn.example.com/vid",
		AesKey:    "key_vid",
	}
	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded VideoItem
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.URL != "https://example.com/video.mp4" {
		t.Fatalf("url: got %q", decoded.URL)
	}
	if decoded.Media == nil {
		t.Fatal("media should not be nil")
	}
}

func TestQRCodeStatusJSON(t *testing.T) {
	jsonStr := `{
		"ret": 0,
		"status": "confirmed",
		"bot_token": "12345:abcdef",
		"baseurl": "https://ilinkai.weixin.qq.com",
		"ilink_bot_id": "bot_123",
		"ilink_user_id": "user_456"
	}`
	var resp QRCodeStatus
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.Status != QRCodeStatusConfirmed {
		t.Fatalf("status: got %q, want %q", resp.Status, QRCodeStatusConfirmed)
	}
	if resp.BotToken != "12345:abcdef" {
		t.Fatalf("bot_token: got %q", resp.BotToken)
	}
	if resp.BaseURL != "https://ilinkai.weixin.qq.com" {
		t.Fatalf("baseurl: got %q", resp.BaseURL)
	}
}

func TestQRCodeResponseJSON(t *testing.T) {
	jsonStr := `{
		"ret": 0,
		"qrcode": "qr_value",
		"qrcode_img_content": "https://example.com/qr.png"
	}`
	var resp QRCodeResponse
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.QRCode != "qr_value" {
		t.Fatalf("qrcode: got %q", resp.QRCode)
	}
	if resp.QRCodeURL != "https://example.com/qr.png" {
		t.Fatalf("qrcode_img_content: got %q", resp.QRCodeURL)
	}
}

func TestSendMessageEnvelopeJSON(t *testing.T) {
	env := sendMessageEnvelope{
		Msg: Message{
			ToUserID:     "user1",
			MessageType:  MessageTypeBot,
			MessageState: MessageStateNormal,
			ContextToken: "ctx1",
			ItemList: []Item{
				{Type: ItemTypeText, TextItem: &TextItem{Text: "hello"}},
			},
		},
		BaseInfo: buildBaseInfo(),
	}
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded struct {
		Msg      Message `json:"msg"`
		BaseInfo BaseInfo `json:"base_info"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.Msg.ToUserID != "user1" {
		t.Fatalf("to_user_id: got %q", decoded.Msg.ToUserID)
	}
	if decoded.BaseInfo.ChannelVersion != ChannelVersion {
		t.Fatalf("channel_version: got %q", decoded.BaseInfo.ChannelVersion)
	}
}

func TestSendImageRefMessageJSON(t *testing.T) {
	msg := newOutboundMessage("user1", "ctx1", Item{
		Type: ItemTypeImage,
		ImageItem: &ImageItem{
			Media: &MediaContent{
				EncryptQueryParam: "eqp_img",
				AesKey:            "aes_img",
				EncryptType:       1,
			},
			MidSize: 2048,
		},
	})
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.ItemList[0].ImageItem.Media.EncryptQueryParam != "eqp_img" {
		t.Fatal("encrypt_query_param mismatch")
	}
	if decoded.ItemList[0].ImageItem.MidSize != 2048 {
		t.Fatal("mid_size mismatch")
	}
}

func TestSendFileRefMessageJSON(t *testing.T) {
	msg := newOutboundMessage("user1", "ctx1", Item{
		Type: ItemTypeFile,
		FileItem: &FileItem{
			Media: &MediaContent{
				EncryptQueryParam: "eqp_file",
				AesKey:            "aes_file",
				EncryptType:       1,
			},
			FileName: "doc.pdf",
			Len:      "1024",
		},
	})
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.ItemList[0].FileItem.FileName != "doc.pdf" {
		t.Fatal("file_name mismatch")
	}
	if decoded.ItemList[0].FileItem.Len != "1024" {
		t.Fatal("len mismatch")
	}
}

func TestGetUpdatesEnvelopeJSON(t *testing.T) {
	env := getUpdatesEnvelope{
		GetUpdatesBuf: "cursor123",
		BaseInfo:      buildBaseInfo(),
	}
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded getUpdatesEnvelope
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.GetUpdatesBuf != "cursor123" {
		t.Fatalf("get_updates_buf: got %q", decoded.GetUpdatesBuf)
	}
	if decoded.BaseInfo.ChannelVersion != ChannelVersion {
		t.Fatalf("channel_version: got %q", decoded.BaseInfo.ChannelVersion)
	}
}

func TestUploadURLEnvelopeJSON(t *testing.T) {
	env := uploadURLEnvelope{
		FileKey:     "abc123",
		MediaType:   1,
		RawSize:     1024,
		RawFileMD5:  "d41d8cd98f00b204e9800998ecf8427e",
		FileSize:    1040,
		NoNeedThumb: true,
		AesKey:      "30313233343536373839616263646566",
		BaseInfo:    buildBaseInfo(),
	}
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded uploadURLEnvelope
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.FileKey != "abc123" {
		t.Fatalf("filekey: got %q", decoded.FileKey)
	}
	if decoded.MediaType != 1 {
		t.Fatalf("media_type: got %d", decoded.MediaType)
	}
	if !decoded.NoNeedThumb {
		t.Fatal("no_need_thumb should be true")
	}
}

func TestUploadedMediaFields(t *testing.T) {
	um := UploadedMedia{
		FileKey:           "fk123",
		EncryptQueryParam: "eqp123",
		AesKey:            "aes123",
		PlainSize:         100,
		CipherSize:        112,
	}
	if um.FileKey != "fk123" {
		t.Fatal("FileKey mismatch")
	}
	if um.PlainSize != 100 {
		t.Fatal("PlainSize mismatch")
	}
	if um.CipherSize != 112 {
		t.Fatal("CipherSize mismatch")
	}
}

func TestEncryptMediaKeyConsistency(t *testing.T) {
	data := []byte("consistency check: key from EncryptMedia must work with DecryptMedia")
	encrypted, aesKey, err := EncryptMedia(data)
	if err != nil {
		t.Fatalf("EncryptMedia error: %v", err)
	}
	decrypted, err := DecryptMedia(encrypted, aesKey)
	if err != nil {
		t.Fatalf("DecryptMedia error: %v", err)
	}
	if !bytes.Equal(decrypted, data) {
		t.Fatal("round-trip failed: EncryptMedia key not compatible with DecryptMedia")
	}
}

func TestConfigResponseJSON(t *testing.T) {
	jsonStr := `{"ret":0,"typing_ticket":"ticket_abc"}`
	var resp ConfigResponse
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.TypingTicket != "ticket_abc" {
		t.Fatalf("typing_ticket: got %q", resp.TypingTicket)
	}
}

func TestMessageOmitEmptyFields(t *testing.T) {
	msg := Message{
		ToUserID:     "user1",
		MessageType:  MessageTypeBot,
		MessageState: MessageStateNormal,
		ItemList:     []Item{},
	}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	// omitempty fields should not appear
	if strings.Contains(string(data), `"from_user_id"`) {
		t.Fatal("from_user_id should be omitted when empty")
	}
	if strings.Contains(string(data), `"context_token"`) {
		t.Fatal("context_token should be omitted when empty")
	}
}
