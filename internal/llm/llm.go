// Package llm 提供多 provider 的 LLM 流式对话客户端。
//
// MVP 阶段实现 OpenAI Chat Completions 协议（OpenRouter / DeepSeek /
// Ollama /v1 等端点均兼容），通过 Provider 接口为后续 Anthropic 等
// 后端预留扩展位。
package llm

import (
	"context"
	"encoding/json"
)

// Role 表示对话消息的角色。
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	// RoleTool 是工具调用结果回传给模型的消息角色。
	RoleTool Role = "tool"
)

// FunctionCall 是一次函数调用请求的参数部分。
type FunctionCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// ToolCall 描述模型发起的一次工具调用（OpenAI tool_calls 片段）。
type ToolCall struct {
	ID       string       `json:"id,omitempty"`
	Type     string       `json:"type,omitempty"` // 恒为 "function"
	Function FunctionCall `json:"function"`
}

// ToolFunctionDef 描述一个可调用函数的元数据（给模型的 schema）。
type ToolFunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// ToolDef 是 ChatRequest.Tools 中单个工具条目。
type ToolDef struct {
	Type     string          `json:"type"` // 恒为 "function"
	Function ToolFunctionDef `json:"function"`
}

// Message 是单条对话消息。
type Message struct {
	Role Role `json:"role"`
	// Content 为文本内容；tool 结果消息里可为空串（结果经 ToolCallID 关联）。
	Content string `json:"content"`
	// ToolCallID 仅 tool 角色使用，指向被回应的那次函数调用。
	ToolCallID string `json:"tool_call_id,omitempty"`
	// ToolCalls 仅 assistant 角色携带；当模型决定调用工具时设置。
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// ChatRequest 描述一轮对话请求。Messages 为完整历史（含本轮最新 user
// 消息）；system 指令（如输出格式约定）作为 RoleSystem 消息置于最前。
type ChatRequest struct {
	Model    string    // 模型名，例如 openai/gpt-4o-mini、deepseek-chat
	Messages []Message // 完整多轮历史
	// ReasoningEffort 透传给兼容端点的 reasoning_effort（如 OpenAI o 系、
	// 部分兼容服务）。取值 "low"|"medium"|"high"；空表示不发送该字段。
	ReasoningEffort string
	// Tools 为可选函数工具（MCP 工具映射而来）；为空则请求不携带 tools。
	Tools []ToolDef
}

// ChatResult 描述一次完成的模型回复。
type ChatResult struct {
	Content      string // 完整回复文本
	Model        string // 实际响应的模型名（若端点回传）
	StopReason   string // finish_reason，如 "stop"
	ToolCalls    []ToolCall
	InputTokens  int // 若端点回传 usage
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
	// onThinking 收到模型的推理/思考增量（如 DeepSeek 的 reasoning_content、
	// OpenAI 推理模型的 reasoning_content）；无思考输出的模型不会调用它。
	// 若模型本轮决定调用工具，调用方从 ChatResult.ToolCalls 读取。
	// ctx 被取消或超时会中止底层请求。
	StreamChat(ctx context.Context, req ChatRequest, onDelta, onThinking DeltaFunc) (*ChatResult, error)
}

// Chat 是流式接口的便捷封装：收集完整回复一次返回。
func Chat(ctx context.Context, p Provider, req ChatRequest) (*ChatResult, error) {
	var collected string
	res, err := p.StreamChat(ctx, req, func(delta string) { collected += delta }, nil)
	if err != nil {
		return nil, err
	}
	if res.Content == "" {
		res.Content = collected
	}
	return res, nil
}
