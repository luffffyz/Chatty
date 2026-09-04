package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// OpenAICompatible 实现 OpenAI Chat Completions 协议（含 SSE 流式）。
// 兼容 OpenAI、OpenRouter、DeepSeek、Ollama(/v1) 等 OpenAI-compatible 端点。
type OpenAICompatible struct {
	// ID 是配置中的 provider 名，例如 "openrouter"、"deepseek"、"ollama"。
	ID string
	// BaseURL 形如 "https://openrouter.ai/api/v1"；留空默认官方 OpenAI 端点。
	BaseURL string
	// APIKey 为空时（如本地 Ollama）不发送 Authorization 头。
	APIKey string
	// HTTPClient 可选；默认 http.DefaultClient。
	HTTPClient *http.Client
}

// NewOpenAICompatible 构造一个 OpenAI-compatible provider。
func NewOpenAICompatible(id, baseURL, apiKey string) *OpenAICompatible {
	return &OpenAICompatible{
		ID:      id,
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
	}
}

// Name 实现 Provider 接口。
func (p *OpenAICompatible) Name() string { return p.ID }

func (p *OpenAICompatible) client() *http.Client {
	if p.HTTPClient != nil {
		return p.HTTPClient
	}
	return http.DefaultClient
}

func (p *OpenAICompatible) chatURL() string {
	base := p.BaseURL
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	return base + "/chat/completions"
}

// sseChunk 对应流式响应中的一个 data: JSON 块。
type sseChunk struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role             string `json:"role"`
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
			ToolCalls        []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// StreamChat 见 Provider 接口说明。
func (p *OpenAICompatible) StreamChat(ctx context.Context, req ChatRequest, onDelta, onThinking DeltaFunc) (*ChatResult, error) {
	if req.Model == "" {
		return nil, errors.New("llm: model is required")
	}
	body := map[string]any{
		"model":    req.Model,
		"messages": req.Messages,
		"stream":   true,
	}
	if req.ReasoningEffort != "" {
		body["reasoning_effort"] = req.ReasoningEffort
	}
	if len(req.Tools) > 0 {
		body["tools"] = req.Tools
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("llm: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.chatURL(), bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("llm: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if p.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)
	}

	resp, err := p.client().Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("llm: request %q failed: %w", p.Name(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, p.errorFromResponse(resp)
	}
	return readEventStream(resp.Body, onDelta, onThinking)
}

// readEventStream 逐行消费 SSE 响应，把文本增量交给 onDelta、推理增量
// 交给 onThinking，并累积完整回复（含 tool_calls 分片拼接）。
func readEventStream(r io.Reader, onDelta, onThinking DeltaFunc) (*ChatResult, error) {
	res := &ChatResult{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)

	// tool_calls 以 (index) 为单位分片到达：arguments 跨块累积，id/name 通常首块给全。
	type acc struct {
		id   string
		typ  string
		name string
		args strings.Builder
	}
	order := []int{}
	calls := map[int]*acc{}

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue // 跳过注释行 / keep-alive / 空行
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk sseChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // 非 JSON 的边角行直接忽略
		}
		if chunk.Error != nil && chunk.Error.Message != "" {
			return res, fmt.Errorf("llm: stream error: %s", chunk.Error.Message)
		}
		if chunk.Model != "" {
			res.Model = chunk.Model
		}
		if len(chunk.Choices) > 0 {
			ch := chunk.Choices[0]
			if ch.Delta.Content != "" {
				res.Content += ch.Delta.Content
				if onDelta != nil {
					onDelta(ch.Delta.Content)
				}
			}
			if ch.Delta.ReasoningContent != "" {
				if onThinking != nil {
					onThinking(ch.Delta.ReasoningContent)
				}
			}
			for _, tc := range ch.Delta.ToolCalls {
				a, ok := calls[tc.Index]
				if !ok {
					a = &acc{}
					calls[tc.Index] = a
					order = append(order, tc.Index)
				}
				if tc.ID != "" {
					a.id = tc.ID
				}
				if tc.Type != "" {
					a.typ = tc.Type
				}
				if tc.Function.Name != "" {
					a.name = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					a.args.WriteString(tc.Function.Arguments)
				}
			}
			if ch.FinishReason != nil {
				res.StopReason = *ch.FinishReason
			}
		}
		if chunk.Usage != nil {
			res.InputTokens = chunk.Usage.PromptTokens
			res.OutputTokens = chunk.Usage.CompletionTokens
		}
	}
	if err := sc.Err(); err != nil {
		return res, fmt.Errorf("llm: read stream: %w", err)
	}

	// 按到达顺序固化 tool_calls
	for _, idx := range order {
		a := calls[idx]
		if a == nil || a.name == "" {
			continue
		}
		tc := ToolCall{ID: a.id, Type: a.typ, Function: FunctionCall{Name: a.name}}
		if s := a.args.String(); s != "" {
			tc.Function.Arguments = s
		}
		res.ToolCalls = append(res.ToolCalls, tc)
	}
	return res, nil
}

type apiErrorBody struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// errorFromResponse 从非 2xx 响应中提取人类可读的错误。
func (p *OpenAICompatible) errorFromResponse(resp *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	msg := strings.TrimSpace(string(raw))
	var eb apiErrorBody
	if json.Unmarshal(raw, &eb) == nil && eb.Error.Message != "" {
		msg = eb.Error.Message
	}
	if msg == "" {
		msg = resp.Status
	}
	return fmt.Errorf("llm: %s: status %d: %s", p.Name(), resp.StatusCode, msg)
}

// ListModels 查询 OpenAI-compatible 的 GET /models 端点，返回可用模型 id。
// 便于设置页扫描提供商可用模型。
func (p *OpenAICompatible) ListModels(ctx context.Context) ([]string, error) {
	base := p.BaseURL
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("llm: build list request: %w", err)
	}
	if p.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)
	}
	resp, err := p.client().Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("llm: list models %q failed: %w", p.Name(), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, p.errorFromResponse(resp)
	}
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&out); err != nil {
		return nil, fmt.Errorf("llm: decode models: %w", err)
	}
	ids := make([]string, 0, len(out.Data))
	for _, m := range out.Data {
		if m.ID != "" {
			ids = append(ids, m.ID)
		}
	}
	if len(ids) == 0 {
		return nil, errors.New("llm: endpoint returned no models")
	}
	return ids, nil
}
