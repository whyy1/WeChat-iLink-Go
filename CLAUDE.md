# CLAUDE.md


This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build all packages
go build ./...

# Run tests
go test ./...

# Run a single test
go test -run TestName .

# Run the example bot
go run ./example/main.go

# Vet and lint
go vet ./...
```

No external dependencies — the module uses only the Go standard library.

## Architecture

This is a flat Go package (`package ilink`) providing a client for the WeChat iLink Bot API at `https://ilinkai.weixin.qq.com`. All source files except `example/main.go` are in the root and share the `ilink` package.

**File responsibilities:**

- `client.go` — `Client` struct, `NewClient`, and the private `do()` HTTP helper that attaches auth headers to every request. All other files call `c.do()` to make requests. Also defines `baseResponse` (embedded in every response struct to surface `ret`/`errmsg` API errors).
- `types.go` — All shared data types: `Message`, `Item`, `TextItem`, `ImageItem`, `VoiceItem`, `FileItem`, `VideoItem`, `BaseInfo`, and all constants (`ItemType*`, `MessageType*`, `QRCodeStatus*`, `TypingStatus*`).
- `login.go` — QR-code login flow: `GetBotQRCode` → `GetQRCodeStatus` → `WaitForLogin` (blocking poll).
- `updates.go` — `GetUpdates` (single long-poll call) and `Poll` (infinite loop with automatic cursor management).
- `send.go` — `SendMessage` (low-level) plus typed helpers `SendText`, `SendImage`, `SendFile`, `SendVideo`.
- `media.go` — AES-128-ECB encrypt/decrypt (PKCS7 padded, stdlib only), `GetUploadURL`, and `UploadMedia` (encrypt → get presigned URL → PUT to CDN).
- `typing.go` — `GetConfig` (retrieves `typing_ticket`) and `SendTyping`.

## Key Protocol Constraints

- **`ContextToken`** from every inbound message must be echoed verbatim in replies — it is the conversation threading handle.
- **`get_updates_buf`** is a cursor returned by `getupdates`; it must be passed back on the next call or messages will be duplicated. `Poll()` handles this automatically.
- **Media encryption**: files are AES-128-ECB encrypted with a per-file random 16-byte key before CDN upload; the base64 key travels in the `sendmessage` payload alongside the CDN URL.
- The HTTP client timeout is set to 60 s (server holds long-poll connections for up to 35 s).
- `X-WECHAT-UIN` is regenerated on every request (`base64(randomUint32)`); this is replay-attack prevention required by the protocol.
