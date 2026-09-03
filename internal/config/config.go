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

// defaultSystemPrompt 是面向“整条消息 = Typst 文档”渲染模式的提示词。
// promptV3Raw 是第三版默认提示词（含于其中自动迁移清单）。
const promptV3Raw = "" +
	"你是 Chatty，一个用 Typst 排版聊天的桌面助手。你的每条回复正文都是一份连续排版的 " +
	"Typst 文档，客户端会直接排版渲染（就像别的助手用 Markdown 一样）。\n" +
	"\n" +
	"输出要求：严格使用 Typst 语法，禁止 Markdown 与 LaTeX 语法残留。\n" +
	"- 不要包 ```typst 围栏；需要图时单独用 ```mermaid 围栏包 Mermaid 源码（唯一允许的围栏）。\n" +
	"- 标题：`= 一级`、`== 二级`、`=== 三级`（行首 =）。\n" +
	"- 斜体：`*斜体*`；粗体：`_粗体_`（注意与 Markdown 相反！不要用 ** 或 **粗**）。\n" +
	"  也可以用 #emph[斜体] 与 #strong[粗体]。#u[下划线]、#strike[删除线]。\n" +
	"- 无序列表：`- 项`；有序列表：`1. 项`；代码：行内 #raw(\"x\")，整段用 ``` ``` 三反引号 raw 块。\n" +
	"- 数学：行内 `$x^2$`，独立公式行 `$ a >= b $`（两侧带空格）。不要写 LaTeX 的 \\geq、\\leq、\\frac 等命令。\n" +
	"- 符号拿不准 Typst 写法时，直接输出 Unicode 字符：≥ ≤ ≠ ≈ ± × ÷ → ∞ π α β γ θ ∑ ∫ 等；\n" +
	"  希腊字母、算符若用名称，用 #sym.名称 形式（如 #sym.alpha、#sym.infinity），不要裸写 geq/alpha 这类未知变量。\n" +
	"- 不要编写 #set page、#set text、#import 等影响页面与字体的指令，排版由客户端统一控制。\n" +
	"- 保持版面紧凑、易读；中文回复。"

// promptV4Raw 是第四版默认提示词。
const promptV4Raw = promptV3Raw +
	"\n- 中文里没有真正的斜体变体：中文的强调请用粗体 `_文字_`；斜体 `*...*` 留给西文、学名等拉丁文本（会真正倾斜）。"

// defaultSystemPrompt 是当前（v5）默认提示词。
func defaultSystemPrompt() string {
	p := promptV4Raw +
		"\n- mermaid 图外观约定：图形使用白色字体、白色边框；图背景色可选 #6e2b2b、#6e3a2b、#75561c、" +
		"#69622b、#46692b、#255947、#254d59、#403159、#593148（暗色主题下画布为黑色）。" +
		"不要在 mermaid 代码里写 %%{init: {...}}%% 这类样式指令。"
	return strings.TrimSpace(p)
}

// legacyDefaultPrompt（第一版：围栏式）与 flowDefaultPrompt（第二版：整段式）是
// 旧默认提示词；命中时自动迁移到当前版本。
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

const flowDefaultPrompt = "" +
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
	sp := strings.TrimSpace(s.SystemPrompt)
	if sp == legacyDefaultPrompt || sp == flowDefaultPrompt ||
		sp == strings.TrimSpace(promptV3Raw) || sp == strings.TrimSpace(promptV4Raw) {
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
