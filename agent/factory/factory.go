package factory

import (
	"fmt"

	"github.com/whyy1/WeChat-iLink-Go/agent"
	"github.com/whyy1/WeChat-iLink-Go/agent/anthropic"
	"github.com/whyy1/WeChat-iLink-Go/agent/claudecode"
)

const (
	BackendAnthropic  = agent.BackendAnthropic
	BackendClaudeCode = agent.BackendClaudeCode
)

type BackendConfig struct {
	Backend    string
	Anthropic  anthropic.Config
	ClaudeCode claudecode.Config
	Tools      []agent.Tool
}

func NewBackend(cfg BackendConfig) (agent.Backend, error) {
	backend := cfg.Backend
	if backend == "" {
		backend = BackendAnthropic
	}
	switch backend {
	case BackendAnthropic:
		b := anthropic.NewBackend(cfg.Anthropic)
		b.SetTools(cfg.Tools)
		return b, nil
	case BackendClaudeCode:
		return claudecode.NewBackend(cfg.ClaudeCode), nil
	default:
		return nil, fmt.Errorf("unsupported agent backend: %s", backend)
	}
}

func NewFromConfig(cfg BackendConfig) (*agent.Agent, error) {
	backend, err := NewBackend(cfg)
	if err != nil {
		return nil, err
	}
	return agent.New(backend), nil
}
