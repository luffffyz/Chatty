// Package mcp 实现 MCP（Model Context Protocol）客户端，传输为
// Streamable HTTP（2025-03-26 spec）：JSON-RPC 2.0 over POST，
// 支持 JSON 或 SSE 编码响应，并跟踪 mcp-session-id 会话。
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// 支持的协议版本（客户端声明自身版本）。
const protocolVersion = "2025-03-26"

// Tool 是服务器暴露的一个工具。
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

// CallResult 是一次 tools/call 的文本化结果。
type CallResult struct {
	Content string
	IsError bool
}

// rpcRequest 是 JSON-RPC 请求。
type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int            `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResult struct {
	Result json.RawMessage `json:"result,omitempty"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Client 是一个 Streamable HTTP MCP 客户端。
type Client struct {
	Endpoint string // MCP 端点 URL（形如 https://host/mcp）
	http     *http.Client

	initMu      chan struct{} // 串行化 initialize（并发安全用轻量方式见下）
	sessionID   string
	initialized bool
}

// New 构造客户端；endpoint 为 MCP 服务器根 URL（不含 method）。
// 超时由调用方的 context 控制。
func New(endpoint string) *Client {
	return &Client{
		Endpoint: strings.TrimRight(endpoint, "/"),
		http:     http.DefaultClient,
		initMu:   make(chan struct{}, 1),
	}
}

// initMu 守卫：initialize 只做一次。
func (c *Client) ensureInit(ctx context.Context) error {
	c.initMu <- struct{}{}
	defer func() { <-c.initMu }()
	if c.initialized {
		return nil
	}
	id := 1
	params, _ := json.Marshal(map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]string{"name": "chatty", "version": "0.1.0"},
	})
	var res rpcResult
	if err := c.post(ctx, &rpcRequest{JSONRPC: "2.0", ID: &id, Method: "initialize", Params: params}, &res); err != nil {
		return err
	}
	if res.Error != nil {
		return fmt.Errorf("mcp: initialize error %d: %s", res.Error.Code, res.Error.Message)
	}
	// 服务器可能返回 capabilities 中的 protocolVersion 等；不强制解析。
	c.initialized = true
	// 客户端侧 initialize 完成后发送 notifications/initialized。
	if err := c.post(ctx, &rpcRequest{JSONRPC: "2.0", Method: "notifications/initialized"}, nil); err != nil {
		return fmt.Errorf("mcp: initialized notice: %w", err)
	}
	return nil
}

// post 发送一个 JSON-RPC 请求并解码响应（JSON 或 SSE 编码均可）。
func (c *Client) post(ctx context.Context, req *rpcRequest, out *rpcResult) error {
	payload, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("mcp: marshal: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("mcp: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	if c.sessionID != "" {
		httpReq.Header.Set("Mcp-Session-Id", c.sessionID)
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return fmt.Errorf("mcp: post %s: %w", c.Endpoint, err)
	}
	defer resp.Body.Close()

	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		c.sessionID = sid
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return fmt.Errorf("mcp: read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("mcp: status %d: %s", resp.StatusCode, truncate(string(raw), 300))
	}
	if out == nil {
		return nil // 通知类：无响应体
	}
	body := decodeBody(raw)
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("mcp: decode response: %w (%s)", err, truncate(string(body), 200))
	}
	return nil
}

// decodeBody 从原始响应体提取 JSON：SSE 流取最后一个 data: 行，否则原样。
func decodeBody(raw []byte) []byte {
	s := strings.TrimSpace(string(raw))
	if strings.Contains(s, "data:") {
		last := ""
		for _, line := range strings.Split(s, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "data:") {
				last = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			}
		}
		return []byte(last)
	}
	return raw
}

// ListTools 返回服务器声明的全部工具。
func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	if err := c.ensureInit(ctx); err != nil {
		return nil, err
	}
	id := 2
	var res rpcResult
	if err := c.post(ctx, &rpcRequest{JSONRPC: "2.0", ID: &id, Method: "tools/list"}, &res); err != nil {
		return nil, err
	}
	if res.Error != nil {
		return nil, fmt.Errorf("mcp: tools/list error %d: %s", res.Error.Code, res.Error.Message)
	}
	var body struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(res.Result, &body); err != nil {
		return nil, fmt.Errorf("mcp: decode tools: %w", err)
	}
	return body.Tools, nil
}

// CallTool 调用服务器工具并返回文本化结果。
func (c *Client) CallTool(ctx context.Context, name string, args json.RawMessage) (*CallResult, error) {
	if err := c.ensureInit(ctx); err != nil {
		return nil, err
	}
	id := 3
	if len(args) == 0 {
		args = json.RawMessage(`{}`) // 无参数工具也要传空对象
	}
	params, _ := json.Marshal(map[string]any{"name": name, "arguments": json.RawMessage(args)})
	var res rpcResult
	if err := c.post(ctx, &rpcRequest{JSONRPC: "2.0", ID: &id, Method: "tools/call", Params: params}, &res); err != nil {
		return nil, err
	}
	if res.Error != nil {
		return nil, fmt.Errorf("mcp: tools/call %q error %d: %s", name, res.Error.Code, res.Error.Message)
	}
	var body struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(res.Result, &body); err != nil {
		return nil, fmt.Errorf("mcp: decode call result: %w", err)
	}
	var sb strings.Builder
	for _, c := range body.Content {
		if c.Type == "text" {
			sb.WriteString(c.Text)
			sb.WriteString("\n")
		}
	}
	out := &CallResult{IsError: body.IsError}
	if sb.Len() > 0 {
		out.Content = strings.TrimRight(sb.String(), "\n")
	} else if len(body.Content) == 0 {
		out.Content = "（工具无文本返回）"
	}
	return out, nil
}

// Ping 用于健康检查/连通性测试（设置页“测试”用）。
func (c *Client) Ping(ctx context.Context) error {
	return c.ensureInit(ctx)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
