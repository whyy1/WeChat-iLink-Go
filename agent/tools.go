package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const defaultCommandTimeout = 30 * time.Second

var DefaultAllowedCommands = []string{
	"echo", "date", "whoami", "hostname", "pwd", "ls", "dir",
	"cat", "head", "tail", "wc", "find", "grep", "sort", "uniq",
	"df", "du", "free", "uptime", "uname", "env", "printenv",
	"curl", "ping", "nslookup", "ipconfig", "ifconfig",
	"python", "python3", "node", "go version",
	"git status", "git log", "git diff", "git branch",
}

func BuiltinTools(reminders ReminderStore, enableCommands bool) []Tool {
	tools := []Tool{{
		Name:        "get_current_time",
		Description: "获取当前日期和时间",
		InputSchema: map[string]any{"type": "object"},
		Handler: func(ctx context.Context, call ToolCall) ToolResult {
			return ToolResult{Content: time.Now().Format("2006-01-02 15:04:05 MST")}
		},
	}}

	if reminders != nil {
		tools = append(tools, reminderTools(reminders)...)
	}
	if enableCommands {
		tools = append(tools, commandTool())
	}
	return tools
}

func reminderTools(reminders ReminderStore) []Tool {
	return []Tool{
		{
			Name:        "set_reminder",
			Description: "为用户设置定时提醒。在指定分钟后提醒用户某件事。",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"message": map[string]any{"type": "string", "description": "提醒的内容"},
					"minutes": map[string]any{"type": "integer", "description": "几分钟后提醒"},
				},
				"required": []string{"message", "minutes"},
			},
			Handler: func(ctx context.Context, call ToolCall) ToolResult {
				var args struct {
					Message string `json:"message"`
					Minutes int    `json:"minutes"`
				}
				if err := json.Unmarshal(call.Input, &args); err != nil {
					return ToolResult{Content: fmt.Sprintf("parse input: %v", err), IsError: true}
				}
				if args.Minutes <= 0 {
					return ToolResult{Content: "minutes must be positive", IsError: true}
				}
				triggerAt := time.Now().Add(time.Duration(args.Minutes) * time.Minute)
				id := reminders.AddReminder(call.UserID, call.ContextToken, args.Message, triggerAt)
				return ToolResult{Content: fmt.Sprintf("已设置提醒：%s（%d分钟后，ID: %s）", args.Message, args.Minutes, id)}
			},
		},
		{
			Name:        "list_reminders",
			Description: "列出用户当前所有待执行的提醒",
			InputSchema: map[string]any{"type": "object"},
			Handler: func(ctx context.Context, call ToolCall) ToolResult {
				items := reminders.ListReminders(call.UserID)
				if len(items) == 0 {
					return ToolResult{Content: "当前没有待执行的提醒"}
				}
				var sb strings.Builder
				for _, item := range items {
					fmt.Fprintf(&sb, "- [%s] %s（%s触发）\n", item.ID, item.Message, item.TriggerAt.Format("15:04"))
				}
				return ToolResult{Content: sb.String()}
			},
		},
		{
			Name:        "cancel_reminder",
			Description: "取消一个已设置的提醒",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"reminder_id": map[string]any{"type": "string", "description": "要取消的提醒ID"},
				},
				"required": []string{"reminder_id"},
			},
			Handler: func(ctx context.Context, call ToolCall) ToolResult {
				var args struct {
					ReminderID string `json:"reminder_id"`
				}
				if err := json.Unmarshal(call.Input, &args); err != nil {
					return ToolResult{Content: fmt.Sprintf("parse input: %v", err), IsError: true}
				}
				if reminders.RemoveReminder(call.UserID, args.ReminderID) {
					return ToolResult{Content: fmt.Sprintf("已取消提醒 %s", args.ReminderID)}
				}
				return ToolResult{Content: fmt.Sprintf("未找到提醒 %s", args.ReminderID), IsError: true}
			},
		},
	}
}

func commandTool() Tool {
	return Tool{
		Name:        "execute_command",
		Description: "执行允许的 shell 命令并返回输出。仅允许安全命令。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{"type": "string", "description": "要执行的命令"},
			},
			"required": []string{"command"},
		},
		Handler: func(ctx context.Context, call ToolCall) ToolResult {
			var args struct {
				Command string `json:"command"`
			}
			if err := json.Unmarshal(call.Input, &args); err != nil {
				return ToolResult{Content: fmt.Sprintf("parse input: %v", err), IsError: true}
			}
			content, isErr := RunCommand(ctx, args.Command, DefaultAllowedCommands)
			return ToolResult{Content: content, IsError: isErr}
		},
	}
}

func RunCommand(ctx context.Context, cmdStr string, allowedCommands []string) (string, bool) {
	cmdStr = strings.TrimSpace(cmdStr)
	baseCmd := cmdStr
	if idx := strings.IndexAny(cmdStr, " \t"); idx > 0 {
		baseCmd = cmdStr[:idx]
	}
	if strings.Contains(baseCmd, "/") || strings.Contains(baseCmd, "\\") {
		baseCmd = baseCmd[strings.LastIndexAny(baseCmd, "/\\")+1:]
	}

	allowed := false
	for _, allowedCommand := range allowedCommands {
		if baseCmd == allowedCommand || strings.HasPrefix(cmdStr, allowedCommand+" ") {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Sprintf("命令 %q 不在允许列表中。允许的命令: %s", baseCmd, strings.Join(allowedCommands, ", ")), true
	}

	commandCtx := ctx
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		commandCtx, cancel = context.WithTimeout(ctx, defaultCommandTimeout)
		defer cancel()
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(commandCtx, "cmd", "/c", cmdStr)
	} else {
		cmd = exec.CommandContext(commandCtx, "sh", "-c", cmdStr)
	}

	out, err := cmd.CombinedOutput()
	result := string(out)
	if len(result) > 2000 {
		result = result[:2000] + "\n... (输出已截断)"
	}
	if commandCtx.Err() != nil {
		return fmt.Sprintf("command timed out: %v", commandCtx.Err()), true
	}
	if err != nil {
		if result == "" {
			result = err.Error()
		}
		return result, true
	}
	return result, false
}
