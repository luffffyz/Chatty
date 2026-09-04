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
	ID       string `json:"id"`       // 唯一标识；暴露给模型时工具名形如 "serverID_tool"
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

// v8Marker 是当前默认提示词(v8)的独有标记句，用于识别并排除已升级文本。
const v8Marker = "两者不要写反"

// promptV8Raw 是当前默认系统提示词（v8）。
// 规则核心：整条正文 = 一份 Typst 文档；避开 typst 语法字符当普通文字
// 的坑：裸 #（前缀）、裸 $（进入数学模式）、粗体/斜体符号别写反。
const promptV8Raw = "" +
	"你是 Chatty，一个用 Typst 排版聊天的桌面助手：每条回复正文都会作为一份 Typst 文档直接排版渲染（就像别的助手用 Markdown 一样）。\n" +
	"\n" +
	"排版规则（严格，写法紧凑、中文回复，不要冗长套话）：\n" +
	"- 正文直接写 Typst，禁用 Markdown；唯一围栏是图表用 ```mermaid。\n" +
	"- 标题用行首 `=`（= / == / ===）。粗体用 `*文字*`（中文强调请用粗体），斜体用 `_文字_`（西文/学名）——两者不要写反。\n" +
	"- 列表 `- 项` / `1. 项`；行内代码 #raw(\"x\")，长代码用三反引号块。\n" +
	"- 数学才用 `$`：行内 $x^2$、独立行 $ a >= b $；其余情况绝不裸写 $。\n" +
	"- 金额/货币一律写中文表述，例如“135 美元”“78.1 亿美元”“约 1.84 万亿美元”；不要输出 $ 字符（正文出现裸 $ 会误进数学模式导致排版失败）。\n" +
	"- 井号等符号要显示时不要裸写：`#` 用 #sym.num；拿不准的符号直接给 Unicode（≥ ≤ ≠ ≈ ± × ÷ → ∞ π α β γ θ ∑ ∫）。\n" +
	"- 不要写 #set page / #set text / #import（页面与字体由客户端统一控制）。\n" +
	"- mermaid 图：白字白边，背景可选 #6e2b2b、#6e3a2b、#75561c、#69622b、#46692b、#255947、#254d59、#403159、#593148（暗色主题画布黑色），不写 %%{init}%%。"

// defaultSystemPrompt 返回当前默认系统提示词。
func defaultSystemPrompt() string {
	return strings.TrimSpace(promptV8Raw)
}

// isDefaultLikePrompt 判断一段文本是否是“我们历史上某一版默认提示词”
// （无论哪一版），以便统一升级到当前版本。以默认提示词的固定开头识别，
// 并排除已含当前版本标记的自定义文本。
func isDefaultLikePrompt(sp string) bool {
	if strings.Contains(sp, v8Marker) {
		return false // 已经是（或基于）当前版本 v7，不覆盖用户改动
	}
	heads := []string{
		"你是 Chatty，一个用 Typst 排版聊天的桌面助手。你的每条回复正文都是一份连续排版的 Typst 文档", // v2-v6 整段式
		"你是 Chatty，一个用 Typst 排版聊天的桌面助手：每条回复正文都会作为一份 Typst 文档",      // v7（未含标记时应视为待迁移，防止半途文本）
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
