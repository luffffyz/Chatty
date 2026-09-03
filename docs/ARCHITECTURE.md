# Chatty 架构

experimental Typst chatbot —— 以 **Typst 为默认文本格式**的自由聊天桌面应用。

## 决策记录（2026，已确认）

| 决策点 | 结论 | 理由 |
|---|---|---|
| 交互形态 | 先做**自由多轮对话** MVP；agent/任务模式**暂不做**（架构上预留 `TaskRunner` 概念，不实现） | 范围收敛，快速验证渲染与聊天体验 |
| 聊天引擎 | **自写薄对话层**（不依赖 axe `pkg/runner`） | 自由聊 100% 贴合、无上游 API 演进风险；参照 wailschat 形态 |
| LLM Provider | **可插拔多 provider**：OpenAI-compatible 系（OpenRouter / DeepSeek / Ollama / 自建端点），接口预留 Anthropic | 一个兼容客户端 + 可配 `base_url` 即可覆盖多数场景 |
| 桌面框架 | **Wails v3**（3.0.0-beta.x，固定版本号） | Windows 免 cgo；需求面（窗口/服务绑定/事件）全在官方稳定区；服务化 API + 自动 TS 绑定 |
| 前端 | **Vue3 + Vite + TypeScript** | 与 wailschat 对齐，可借鉴其结构 |
| 会话存储 | SQLite（`modernc.org/sqlite`，纯 Go 无 CGO） | 本地优先、单文件 |
| 渲染 | 前端 `@myriaddreamin/typst.ts`（wasm，Typst→SVG）+ `mermaid` 分块渲染 | 浏览器内离线编译，公式/排版统一走 Typst 引擎 |
| 代码蓝本 | wailschat（MIT, Copyright 2026 RedJACK）—— 借鉴 Go 侧结构与前端形态，**自己实现**代码 | MIT 允许借鉴，仍避免无关代码搬运 |

## 模块划分

```
Chatty (Wails v3, Go + Vue3)
│
├── internal/
│   ├── llm/       Provider 接口 + OpenAICompatible 实现
│   │              (base_url / api_key / model 可配多实例, SSE 流式)
│   ├── chat/      会话与消息模型 + SQLite 持久化
│   ├── config/    应用设置(providers 列表、当前 model、system prompt)
│   └── app/       Wails binding 层:SendMessage / NewSession / ListSessions …
│
└── frontend/      Vue3 + Vite
    ├── 视图:会话列表 + 消息流 + 输入框 + 设置
    └── renderer/  消息分块渲染:
        文本        → 纯文本
        ```typst   → typst.ts 编译为 SVG
        ```mermaid → mermaid 渲染
```

### internal/llm

```go
type Provider interface {
    Name() string
    // StreamChat 以流式方式完成一轮对话,每增量文本回调 onDelta。
    StreamChat(ctx context.Context, req ChatRequest, onDelta func(string)) (*ChatResult, error)
}
type ChatRequest struct {
    Model    string
    Messages []Message      // role + content
    System   string
}
type OpenAICompatible struct { Name_, BaseURL, APIKey string } // SSE 解析
```

- MVP 只实现 `OpenAICompatible`；`Anthropic` 等通过同一 `Provider` 接口后续添加。
- 多个 provider 实例从设置加载（`OpenRouter`、`DeepSeek`、`Ollama` 都是 OpenAI-compatible 端点）。

### internal/chat

- `Session{ID, Title, CreatedAt, Messages []Message}`
- `Message{Role, Content}`（user / assistant）；首条用户消息自动生成标题。
- SQLite 表：`sessions`、`messages`。

### 输出格式约定（进 system prompt）

自由聊天中模型应输出**普通文本 + 围栏块**：

- 需要排版的公式/文本用 ` ```typst ` 围栏，内部直接写 Typst（数学用 Typst 的 `$…$`）；
- 图表用 ` ```mermaid ` 围栏；
- 纯文本/对话内容用普通段落，不要包 Typst 围栏。

UI 按围栏切块渲染，保证流式过程中逐块出现。

## 非目标（本阶段明确不做）

- agent / 任务模式、工具调用循环、MCP、子代理、内置文件工具（预留接口不实现）
- 多窗口、托盘、自动更新
- PDF 导出（Typst 已能出，后续可加后端编译）
