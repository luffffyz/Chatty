// Package llm 提供多 provider 的 LLM 流式对话客户端。
//
// MVP 阶段实现 OpenAI Chat Completions 协议（OpenRouter / DeepSeek /
// Ollama /v1 等端点均兼容），通过 Provider 接口为后续 Anthropic 等
// 后端预留扩展位。
package llm

import "context"

// Role 表示对话消息的角色。
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message 是单条对话消息。
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

// ChatRequest 描述一轮对话请求。Messages 为完整历史（含本轮最新 user
// 消息）；system 指令（如输出格式约定）作为 RoleSystem 消息置于最前。
type ChatRequest struct {
	Model    string    // 模型名，例如 openai/gpt-4o-mini、deepseek-chat
	Messages []Message // 完整多轮历史
}

// ChatResult 描述一次完成的模型回复。
type ChatResult struct {
	Content      string // 完整回复文本
	Model        string // 实际响应的模型名（若端点回传）
	StopReason   string // finish_reason，如 "stop"
	InputTokens  int    // 若端点回传 usage
	OutputTokens int
}

// DeltaFunc 在流式输出期间被逐增量调用。
type DeltaFunc func(delta string)

// Provider 是模型后端的最小抽象；一个实例对应一个可配置端点。
type Provider interface {
	// Name 返回 provider 标识（用于配置展示与日志）。
	Name() string
	// StreamChat 流式执行一轮对话：onDelta 在每段增量文本到达时被同步
	// 调用（可能为空实现），返回的 ChatResult.Content 为完整文本。
	// ctx 被取消或超时会中止底层请求。
	StreamChat(ctx context.Context, req ChatRequest, onDelta DeltaFunc) (*ChatResult, error)
}

// Chat 是流式接口的便捷封装：收集完整回复一次返回。
func Chat(ctx context.Context, p Provider, req ChatRequest) (*ChatResult, error) {
	var collected string
	res, err := p.StreamChat(ctx, req, func(delta string) { collected += delta })
	if err != nil {
		return nil, err
	}
	if res.Content == "" {
		res.Content = collected
	}
	return res, nil
}
