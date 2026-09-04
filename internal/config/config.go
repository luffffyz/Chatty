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
	Model   string   // 该提供商常用的模型（切换为该提供商时自动恢复）
	Models  []string // 扫描得到的可用模型列表（可选）
}

// Settings 是应用设置。
type Settings struct {
	Providers        []Provider  `json:"providers"`
	ActiveProviderID string      `json:"activeProviderId"`
	ActiveModel      string      `json:"activeModel"`
	SystemPrompt     string      `json:"systemPrompt"`
	Appearance       Appearance  `json:"appearance"`
	MCPServers       []MCPServer `json:"mcpServers,omitempty"`
}

// MCPServer 描述一个 MCP（Model Context Protocol）服务器。
// 传输固定为 Streamable HTTP（JSON-RPC 2.0 over POST）。
type MCPServer struct {
	ID       string `json:"id"`       // 唯一标识，同时用作工具名前缀 "serverID::tool"
	Label    string `json:"label"`    // 展示名，如 "DeepWiki"
	Endpoint string `json:"endpoint"` // 如 "https://mcp.deepwiki.com/mcp"
	// APIKey 可选：非空时以 "Authorization: Bearer <key>" 发送（无鉴权留空）。
	APIKey string `json:"apiKey,omitempty"`
}

// Appearance 是界面外观设置。
type Appearance struct {
	Theme    string `json:"theme"`    // light | dark | system
	FontSize int    `json:"fontSize"` // 基础字号 px（0 表示默认 14）
	ChartBg  string `json:"chartBg"`  // 暗色主题下 mermaid 图背景色(hex)，空=默认
}

// Theme 取值。
const (
	ThemeLight  = "light"
	ThemeDark   = "dark"
	ThemeSystem = "system"
)

// DefaultFontSize 是界面默认基础字号。
const DefaultFontSize = 14

// v6Marker 是当前默认提示词的独有标记，用于识别“尚未升级的旧默认”。
const v6Marker = "#sym.num"

// promptV6Raw 是当前默认系统提示词。
// 规则核心：整条正文 = Typst 文档；# 是 Typst 语法前缀，绝不当 Markdown 用。
const promptV6Raw = "" +
	"你是 Chatty，一个用 Typst 排版聊天的桌面助手。你的每条回复正文都是一份连续排版的 " +
	"Typst 文档，客户端会直接排版渲染（就像别的助手用 Markdown 一样）。\n" +
	"\n" +
	"输出规则（严格）：\n" +
	"- 整条正文直接写 Typst；不要用 Markdown 语法，不要输出 ```typst 围栏；唯一允许的围栏是 ```mermaid（需要图表时）。\n" +
	"- 井号 `#` 在 Typst 中是语法前缀，不能像 Markdown 那样用在行首或 `##`：标题一律用行首的 `=`（= 一级 / == 二级 / === 三级）。" +
	"若内容真的需要显示 # 符号，写作 `#sym.num`，绝不要裸写 #。\n" +
	"- 斜体用 `*文字*`（西文、学名等拉丁文本）；粗体用 `_文字_`（与 Markdown 相反；中文里没有斜体变体，中文强调也用粗体）。" +
	"也可用 #emph[...] 与 #strong[...]。#u[下划线]、#strike[删除线]。\n" +
	"- 列表：`- 项` 无序、`1. 项` 有序。行内代码 #raw(\"x\")；展示整段代码用三反引号 raw 块。\n" +
	"- 数学：行内 `$x^2$`，独立公式行 `$ a >= b $`（两侧带空格）。不要写 LaTeX 的 \\geq、\\leq、\\frac 等命令。" +
	"符号拿不准时直接输出 Unicode（≥ ≤ ≠ ≈ ± × ÷ → ∞ π α β γ θ ∑ ∫），或用 #sym.名称（如 #sym.alpha、#sym.infinity）。\n" +
	"- 不要编写 #set page、#set text、#import 等影响页面与字体的指令，排版由客户端统一控制。\n" +
	"- mermaid 图外观约定：图形使用白色字体、白色边框；背景色可选 #6e2b2b、#6e3a2b、#75561c、#69622b、" +
	"#46692b、#255947、#254d59、#403159、#593148（暗色主题下画布为黑色）；不要在 mermaid 代码里写 %%{init: {...}}%% 指令。\n" +
	"- 中文回复；版面紧凑、易读。"

// defaultSystemPrompt 返回当前默认系统提示词。
func defaultSystemPrompt() string {
	return strings.TrimSpace(promptV6Raw)
}

// isDefaultLikePrompt 判断一段文本是否是“我们历史上某一版默认提示词”
// （无论哪一版），以便统一升级到当前版本。以默认提示词的固定开头识别，
// 并排除已含当前版本标记的自定义文本。
func isDefaultLikePrompt(sp string) bool {
	if strings.Contains(sp, v6Marker) {
		return false // 已经是（或基于）当前版本，不覆盖用户改动
	}
	heads := []string{
		"你是 Chatty，一个用 Typst 排版聊天的桌面助手。你的每条回复正文都是一份连续排版的 Typst 文档", // v3+ / flow 系
		"你是 Chatty，一个以 Typst 为主要排版格式的桌面聊天助手。",                      // v1 围栏系
	}
	for _, h := range heads {
		if strings.HasPrefix(sp, h) {
			return true
		}
	}
	return false
}

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

// ensureDefaults 补齐旧版配置缺少的字段（兼容升级），并迁移旧版默认提示词
// 到当前版本。用户自定文本（非默认样式开头）保持不动。
func ensureDefaults(s *Settings) {
	if s.Appearance.Theme != ThemeLight && s.Appearance.Theme != ThemeDark {
		s.Appearance.Theme = ThemeSystem
	}
	if s.Appearance.FontSize <= 0 {
		s.Appearance.FontSize = DefaultFontSize
	}
	sp := strings.TrimSpace(s.SystemPrompt)
	if isDefaultLikePrompt(sp) {
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
