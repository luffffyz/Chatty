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
		Index        int     `json:"index"`
		Delta        Message `json:"delta"`
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
func (p *OpenAICompatible) StreamChat(ctx context.Context, req ChatRequest, onDelta DeltaFunc) (*ChatResult, error) {
	if req.Model == "" {
		return nil, errors.New("llm: model is required")
	}
	payload, err := json.Marshal(map[string]any{
		"model":    req.Model,
		"messages": req.Messages,
		"stream":   true,
	})
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
	return readEventStream(resp.Body, onDelta)
}

// readEventStream 逐行消费 SSE 响应，把文本增量交给 onDelta 并累积。
func readEventStream(r io.Reader, onDelta DeltaFunc) (*ChatResult, error) {
	res := &ChatResult{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)

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
