# WeChat-iLink-Go

Go 语言实现的微信 iLink Bot API 客户端库，支持扫码登录、长轮询收发消息、媒体加解密、Claude Agent 集成与定时提醒。同时提供可编译运行的 CLI 程序，开箱即用。

## 功能概览

**核心库**
- 扫码登录，本地持久化 `bot_token`
- 长轮询接收消息（指数退避自动重试）
- 发送文本、图片、文件、视频
- 下载并解密收到的媒体文件
- 输入中状态（typing indicator）
- AES-128-ECB + PKCS7（纯标准库实现）

**CLI 程序 (`cmd/ilink/`)**
- Agent 模式：文本消息自动调用 Claude，支持工具调用、定时提醒
- Cmd 模式：简单回显机器人，无需 API Key
- 四层配置解析：配置文件 → 环境变量 → Claude 设置文件 → 交互式提示
- 多实例隔离：通过 `--config` 指定不同配置文件
- QR 扫码登录，token 自动保存

## 安装

```bash
# 作为库使用
go get github.com/whyy1/WeChat-iLink-Go

# 编译 CLI
go build -o ilink ./cmd/ilink/
```

要求 Go 1.21+。

## CLI 使用

### 命令一览

```bash
ilink                      # 默认启动 Agent 模式
ilink --cmd / -c           # 启动 Cmd 模式（简单回显，无需 API Key）
ilink --agent / -a         # 显式启动 Agent 模式
ilink --config <path>      # 指定配置文件路径（默认 ~/.ilink/config.json）

ilink config               # 交互式配置向导（扫码登录 + 配置各项参数）
ilink config setup         # 同上
ilink config show          # 显示当前配置（API Key 自动脱敏）
ilink config set <k> <v>   # 设置单个配置项
```

### 快速开始

```bash
# 1. 编译
go build -o ilink ./cmd/ilink/

# 2. 交互式配置（会引导扫码登录 + 配置 API Key 等）
./ilink config

# 3. 启动 Agent 模式
./ilink

# 或启动 Cmd 模式（仅回显，不需要 API Key）
./ilink --cmd
```

### 配置系统

配置文件位于 `~/.ilink/config.json`，JSON 格式，字段名与 Claude 的 `settings.json` 保持一致。

四层配置解析，优先级从高到低：

| 优先级 | 来源 | 说明 |
|--------|------|------|
| 1 | `~/.ilink/config.json` | 用户显式配置 |
| 2 | 环境变量 | `ANTHROPIC_API_KEY`、`ANTHROPIC_BASE_URL`、`ANTHROPIC_MODEL`、`ILINK_BOT_TOKEN` |
| 3 | `~/.claude/settings.json` | 读取 `env` 字段中的值（兼容 Claude Code 配置） |
| 4 | 交互式提示 | 前三层均无值时触发 |

配置字段：

| 字段 | 说明 | 默认值 |
|------|------|--------|
| `bot_token` | 微信 Bot Token（扫码登录自动保存） | - |
| `ANTHROPIC_API_KEY` | Anthropic API 密钥 | - |
| `ANTHROPIC_BASE_URL` | API 基础地址（自定义代理） | - |
| `ANTHROPIC_MODEL` | 模型名称 | `claude-sonnet-4-6` |
| `backend` | 后端类型：`anthropic` 或 `claude-code` | `anthropic` |
| `enable_commands` | 是否启用 execute_command 工具 | `true` |
| `system_prompt` | 自定义系统提示词 | - |
| `working_dir` | Agent 工作目录 | `~/.ilink/workspace/` |

### 多实例隔离

通过 `--config` 指定不同配置文件，实现多 Bot 实例隔离：

```bash
./ilink --config ~/.ilink/bot-a.json    # Bot A
./ilink --config ~/.ilink/bot-b.json    # Bot B
```

## 库 API

### 两套 API 风格

- **Context API**：首参数为 `context.Context`，支持超时和取消
- **Simple API**：后缀 `Simple`，内部使用 `context.Background()`

### Client

```go
client := ilink.NewClient(botToken,
    ilink.WithDebug(true),
    ilink.WithBaseURL(url),
)

// 登录
qr, _ := client.GetBotQRCodeSimple()
token, _ := client.WaitForLoginSimple(qr.QRCode, 2*time.Second)

// 收消息
client.PollSimple(func(msg ilink.Message) error {
    // msg.ItemList 包含文本、图片、文件、视频等
    return nil
})

// 发消息
client.SendTextSimple(toUserID, contextToken, "hello")
client.SendImageSimple(toUserID, contextToken, cdnURL, aesKey)
```

### Agent

```go
agent := ilink.NewAgent(ilink.AgentConfig{
    APIKey:         "sk-ant-...",
    Model:          "claude-sonnet-4-6",
    SystemPrompt:   "你是一个有用的微信助手。",
    EnableCommands: true,
    WorkDir:        "/path/to/workspace",
})
agent.SetReminderStore(ilink.NewReminderStore())

// 对话（自动管理会话历史，支持工具调用）
reply, _ := agent.Chat(userID, contextToken, "你好")
agent.ResetConversation(userID)
```

Agent 内置工具：

| 工具 | 说明 |
|------|------|
| `get_current_time` | 获取当前日期时间 |
| `set_reminder` | 设置定时提醒 |
| `list_reminders` | 列出待执行提醒 |
| `cancel_reminder` | 取消提醒 |
| `execute_command` | 执行白名单中的 shell 命令 |

### 媒体

```go
// 上传（自动加密）
info, _ := client.UploadMediaForUserSimple(toUserID, data, ilink.ItemTypeImage)

// 下载（自动解密）
data, _ := client.DownloadReceivedMediaSimple(item)

// 独立加解密
encrypted, key, _ := ilink.EncryptMedia(data)
decrypted, _ := ilink.DecryptMedia(encrypted, key)
```

### 常量

| 常量 | 值 |
|------|-----|
| `ItemTypeText` | 1 |
| `ItemTypeImage` | 2 |
| `ItemTypeVoice` | 3 |
| `ItemTypeFile` | 4 |
| `ItemTypeVideo` | 5 |
| `MessageTypeUser` | 1 |
| `MessageTypeBot` | 2 |

## 文件结构

```
.
├── client.go              # Client 结构体与 HTTP 请求封装
├── types.go               # 协议类型与常量
├── login.go               # 扫码登录流程
├── updates.go             # 长轮询收消息（含指数退避）
├── send.go                # 消息发送
├── media.go               # 媒体上传、下载、AES 加解密
├── typing.go              # 输入中状态
├── agent.go               # Agent 封装（多后端、会话管理）
├── remind.go              # 定时提醒调度器
├── agent/
│   ├── agent.go           # provider-neutral Agent 核心
│   ├── tools.go           # 内置工具实现
│   ├── anthropic/         # Anthropic API 后端
│   ├── claudecode/        # Claude Code CLI 后端
│   └── factory/           # 后端工厂
├── internal/config/       # CLI 配置包（四层解析）
├── cmd/ilink/             # CLI 入口
│   ├── main.go            # flag 解析、子命令分发
│   ├── handlers.go        # 共享消息处理器
│   ├── mode_agent.go      # Agent 模式
│   └── mode_cmd.go        # Cmd 模式
├── example/main.go        # 完整示例
└── cmd/agent_demo/        # Agent 集成测试
```

## 协议说明

- Base URL: `https://ilinkai.weixin.qq.com`
- CDN: `https://novac2c.cdn.weixin.qq.com/c2c`
- 认证: `AuthorizationType: ilink_bot_token` + `Authorization: Bearer {token}`
- 防重放: `X-WECHAT-UIN: base64(randomUint32)` 每次请求重新生成
- `ContextToken` 必须原样回传，用于会话线程关联

## 当前限制

- ReminderStore 为纯内存存储，进程重启后提醒丢失
- 媒体下载受运行环境网络影响
- 首次接收媒体时可能需要多种 URL 回退逻辑

## License

Apache-2.0
