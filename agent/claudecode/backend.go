package claudecode

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	DefaultCommand = "claude"
	DefaultTimeout = 2 * time.Minute
)

type Config struct {
	Command      string
	Model        string
	SystemPrompt string
	Timeout      time.Duration
	WorkingDir   string
}

type Backend struct {
	command      string
	model        string
	systemPrompt string
	timeout      time.Duration
	workingDir   string
}

func NewBackend(cfg Config) *Backend {
	command := cfg.Command
	if command == "" {
		command = DefaultCommand
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	return &Backend{
		command:      command,
		model:        cfg.Model,
		systemPrompt: cfg.SystemPrompt,
		timeout:      timeout,
		workingDir:   cfg.WorkingDir,
	}
}

func (b *Backend) ChatWithCtx(ctx context.Context, userID, contextToken, text string) (string, error) {
	if strings.TrimSpace(text) == "" {
		return "", errors.New("claude-code: text is empty")
	}
	commandCtx := ctx
	if _, ok := ctx.Deadline(); !ok && b.timeout > 0 {
		var cancel context.CancelFunc
		commandCtx, cancel = context.WithTimeout(ctx, b.timeout)
		defer cancel()
	}

	args := []string{"--print", "--output-format", "text", "--permission-mode", "dontAsk", "--tools", ""}
	if b.model != "" {
		args = append(args, "--model", b.model)
	}
	if b.systemPrompt != "" {
		args = append(args, "--system-prompt", b.systemPrompt)
	}
	args = append(args, text)

	cmd := exec.CommandContext(commandCtx, b.command, args...)
	if b.workingDir != "" {
		cmd.Dir = b.workingDir
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if commandCtx.Err() != nil {
			return "", fmt.Errorf("claude-code timed out or blocked waiting for interaction: %w", commandCtx.Err())
		}
		return "", fmt.Errorf("claude-code failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return "", fmt.Errorf("claude-code returned empty output: %s", strings.TrimSpace(stderr.String()))
	}
	return output, nil
}

func (b *Backend) ResetConversation(userID string) {}

func (b *Backend) GetConversationLength(userID string) int {
	return 0
}
