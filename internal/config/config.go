// Package config 负责 Chatty 应用设置（provider 列表 / 当前模型 / 系统提示）
// 的加载与保存。设置以 JSON 存放于用户配置目录。
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Provider 描述一个 OpenAI-compatible 后端。
type Provider struct {
	ID      string // 唯一标识，如 "openrouter"、"deepseek"、"ollama"
	Label   string // 展示名
	BaseURL string // 如 "https://openrouter.ai/api/v1"
	APIKey  string
}

// Settings 是应用设置。
type Settings struct {
	Providers        []Provider `json:"providers"`
	ActiveProviderID string     `json:"activeProviderId"`
	ActiveModel      string     `json:"activeModel"`
	SystemPrompt     string     `json:"systemPrompt"`
}

// Default 返回出厂设置：无 provider、带默认系统提示（Typst 输出约定）。
func Default() *Settings {
	prompt := "" +
		"你是 Chatty，一个以 Typst 为主要排版格式的桌面聊天助手。\n" +
		"\n" +
		"回复规则：\n" +
		"- 涉及公式、严谨排版的片段放在 ```typst 围栏内，写完整 Typst 源码，" +
		"推荐以 #set page(width: auto, height: auto, margin: 10pt, fill: white) 开头；" +
		"数学公式用 Typst 的 $...$ 语法。\n" +
		"- 图表（流程图、时序图等）放在 ```mermaid 围栏内。\n" +
		"- 普通叙述直接写纯文本段落，不要包 Typst 围栏。\n" +
		"- 默认使用简体中文回复。"
	return &Settings{
		SystemPrompt: strings.TrimSpace(prompt),
	}
}

// Load 从 path 读取设置；文件不存在时返回默认设置（不写盘）。
func Load(path string) (*Settings, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}
	s := &Settings{}
	if err := json.Unmarshal(raw, s); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}
	return s, nil
}

// Save 把设置写入 path（自动创建父目录）。
func Save(path string, s *Settings) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("config: mkdir: %w", err)
	}
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return fmt.Errorf("config: write %s: %w", path, err)
	}
	return nil
}
