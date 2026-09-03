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
	Appearance       Appearance `json:"appearance"`
}

// Appearance 是界面外观设置。
type Appearance struct {
	Theme    string `json:"theme"`    // light | dark | system
	FontSize int    `json:"fontSize"` // 基础字号 px（0 表示默认 14）
}

// Theme 取值。
const (
	ThemeLight  = "light"
	ThemeDark   = "dark"
	ThemeSystem = "system"
)

// DefaultFontSize 是界面默认基础字号。
const DefaultFontSize = 14

// defaultSystemPrompt 是面向“整条消息 = Typst 文档”渲染模式的提示词。
func defaultSystemPrompt() string {
	p := "" +
		"你是 Chatty，一个用 Typst 排版聊天的桌面助手。你的每条回复正文都是一份连续排版的 " +
		"Typst 文档，客户端会直接把它排版渲染出来（就像别的助手用 Markdown 一样）。\n" +
		"\n" +
		"回复规则：\n" +
		"- 正文直接写 Typst 源码，不要再包 ```typst 围栏，不要输出 Markdown 语法。\n" +
		"- 强调用 #emph[...]，粗体用 #strong[...]，行内代码用 #raw(\"...\")，列表/表格/引用都用 Typst 原生语法。\n" +
		"- 公式直接写 Typst 数学：行内用 $x^2$，独立公式行用 $ x^2 $（两侧带空格）。\n" +
		"- 需要图表（流程图、时序图等）时，单独用 ```mermaid 围栏包住 Mermaid 源码——这是唯一允许的围栏。\n" +
		"- 不要编写 #set page、#set text、#import 等影响页面与字体的指令，排版由客户端统一控制。\n" +
		"- 保持版面紧凑、易读；中文回复。"
	return strings.TrimSpace(p)
}

// legacyDefaultPrompt 是上一个版本（围栏式）的默认提示词；命中时自动迁移。
const legacyDefaultPrompt = "" +
	"你是 Chatty，一个以 Typst 为主要排版格式的桌面聊天助手。\n" +
	"\n" +
	"回复规则：\n" +
	"- 涉及公式、严谨排版的片段放在 ```typst 围栏内，写完整 Typst 源码，" +
	"推荐以 #set page(width: auto, height: auto, margin: 10pt, fill: white) 开头；" +
	"数学公式用 Typst 的 $...$ 语法。\n" +
	"- 图表（流程图、时序图等）放在 ```mermaid 围栏内。\n" +
	"- 普通叙述直接写纯文本段落，不要包 Typst 围栏。\n" +
	"- 默认使用简体中文回复。"

// Default 返回出厂设置：无 provider、带默认系统提示（Typst 输出约定）。
func Default() *Settings {
	return &Settings{
		SystemPrompt: defaultSystemPrompt(),
		Appearance: Appearance{
			Theme:    ThemeSystem,
			FontSize: DefaultFontSize,
		},
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
	ensureDefaults(s)
	return s, nil
}

// ensureDefaults 补齐旧版配置缺少的字段（兼容升级），并迁移旧版默认提示词。
func ensureDefaults(s *Settings) {
	if s.Appearance.Theme != ThemeLight && s.Appearance.Theme != ThemeDark {
		s.Appearance.Theme = ThemeSystem
	}
	if s.Appearance.FontSize <= 0 {
		s.Appearance.FontSize = DefaultFontSize
	}
	if strings.TrimSpace(s.SystemPrompt) == legacyDefaultPrompt {
		s.SystemPrompt = defaultSystemPrompt()
	}
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
