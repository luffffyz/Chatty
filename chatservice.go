package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"chatty/internal/chat"
	"chatty/internal/config"
	"chatty/internal/llm"
	"chatty/internal/mcp"
)

// ---------- 事件负载（经 application.RegisterEvent 注册，前端可强类型订阅） ----------

// ChatDeltaEvent 在流式输出期间逐段推送。
type ChatDeltaEvent struct {
	SessionID string `json:"sessionId"`
	Delta     string `json:"delta"`
}

// ChatDoneEvent 在整条回复结束时推送。
type ChatDoneEvent struct {
	SessionID string `json:"sessionId"`
	Content   string `json:"content"`
	Thinking  string `json:"thinking"`
}

// ChatErrorEvent 在流式过程出错时推送。
type ChatErrorEvent struct {
	SessionID string `json:"sessionId"`
	Error     string `json:"error"`
}

// ChatThinkingEvent 在模型输出推理/思考文本时按增量推送（如 DeepSeek
// reasoner、OpenAI 推理模型的 reasoning_content）。
type ChatThinkingEvent struct {
	SessionID string `json:"sessionId"`
	Text      string `json:"text"`
}

// ---------- 对外 DTO（避免直接暴露含 time.Time 的内部模型） ----------

// SessionDTO 是对前端暴露的会话摘要。
type SessionDTO struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	CreatedAtMs int64  `json:"createdAtMs"`
	UpdatedAtMs int64  `json:"updatedAtMs"`
}

// MessageDTO 是对前端暴露的单条消息。
type MessageDTO struct {
	ID        int64  `json:"id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	Thinking  string `json:"thinking"`
	CreatedMs int64  `json:"createdMs"`
}

// EventEmitter 抽象 wails 事件发射，便于测试注入。
type EventEmitter interface {
	Emit(name string, data ...any) bool
}

// ChatService 是暴露给前端的聊天服务。
type ChatService struct {
	store        chat.Store
	settings     *config.Settings
	settingsPath string
	emitter      EventEmitter
	client       func(p *config.Provider) llm.Provider // 可注入，默认 OpenAICompatible

	mu   sync.Mutex
	busy map[string]bool

	// mcpMu 守卫 mcpClients：按 endpoint 复用 MCP 客户端（懒初始化会话）。
	mcpMu      sync.Mutex
	mcpClients map[string]*mcp.Client
}

// NewChatService 构造聊天服务。settings 会被深拷贝保存到 settingsPath。
func NewChatService(store chat.Store, cfg *config.Settings, settingsPath string, emitter EventEmitter) *ChatService {
	return &ChatService{
		store:        store,
		settings:     cfg,
		settingsPath: settingsPath,
		emitter:      emitter,
		client: func(p *config.Provider) llm.Provider {
			return llm.NewOpenAICompatible(p.ID, p.BaseURL, p.APIKey)
		},
		busy:       make(map[string]bool),
		mcpClients: make(map[string]*mcp.Client),
	}
}

// GetSessions 返回全部会话（按最近更新倒序）。
func (s *ChatService) GetSessions() ([]SessionDTO, error) {
	all, err := s.store.ListSessions()
	if err != nil {
		return nil, err
	}
	out := make([]SessionDTO, len(all))
	for i, ss := range all {
		out[i] = sessionToDTO(ss)
	}
	return out, nil
}

// NewSession 新建一个空会话。
func (s *ChatService) NewSession() (*SessionDTO, error) {
	ss, err := s.store.CreateSession("新会话")
	if err != nil {
		return nil, err
	}
	dto := sessionToDTO(*ss)
	return &dto, nil
}

// GetMessages 返回会话内全部消息。
func (s *ChatService) GetMessages(sessionID string) ([]MessageDTO, error) {
	_, msgs, err := s.store.GetSession(sessionID)
	if err != nil {
		return nil, err
	}
	out := make([]MessageDTO, len(msgs))
	for i, m := range msgs {
		out[i] = MessageDTO{ID: m.ID, Role: string(m.Role), Content: m.Content, Thinking: m.Thinking, CreatedMs: m.CreatedAt.UnixMilli()}
	}
	return out, nil
}

// DeleteSession 删除会话。
func (s *ChatService) DeleteSession(sessionID string) error {
	return s.store.DeleteSession(sessionID)
}

// GetSettings 返回当前设置副本。
func (s *ChatService) GetSettings() (*config.Settings, error) {
	cp := *s.settings
	cp.Providers = append([]config.Provider(nil), s.settings.Providers...)
	cp.MCPServers = append([]config.MCPServer(nil), s.settings.MCPServers...)
	return &cp, nil
}

// SaveSettings 校验并保存设置。
func (s *ChatService) SaveSettings(st *config.Settings) error {
	if st == nil {
		return errors.New("设置不能为空")
	}
	if err := validateSettings(st); err != nil {
		return err
	}
	if err := config.Save(s.settingsPath, st); err != nil {
		return err
	}
	s.settings = st
	return nil
}

// ListModels 扫描 OpenAI-compatible 端点的可用模型（用于设置页下拉）。
func (s *ChatService) ListModels(baseURL, apiKey string) ([]string, error) {
	if trimSpace(baseURL) == "" {
		return nil, errors.New("Base URL 不能为空")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	p := llm.NewOpenAICompatible("scan", baseURL, apiKey)
	return p.ListModels(ctx)
}

// SendMessage 把用户消息追加到会话并异步流式请求模型。
// effort 取值 ""|"low"|"medium"|"high"，作为 reasoning_effort 透传。
// 返回错误表示同步失败（会话不存在 / 正忙 / 未配置）；流式期间的
// 增量/结束/错误通过 chat:delta / chat:done / chat:error 事件推送。
func (s *ChatService) SendMessage(sessionID, text, effort string) error {
	switch effort {
	case "", "low", "medium", "high":
	default:
		return errors.New("思考深度取值无效（low/medium/high）")
	}
	text = trimSpace(text)
	if text == "" {
		return errors.New("消息不能为空")
	}
	s.mu.Lock()
	if s.busy[sessionID] {
		s.mu.Unlock()
		return errors.New("该会话正在生成中")
	}
	s.busy[sessionID] = true
	s.mu.Unlock()

	go s.runCompletion(sessionID, text, effort)
	return nil
}

func (s *ChatService) runCompletion(sessionID, text, effort string) {
	defer func() {
		s.mu.Lock()
		delete(s.busy, sessionID)
		s.mu.Unlock()
	}()

	emitErr := func(msg string) {
		s.emitter.Emit("chat:error", ChatErrorEvent{SessionID: sessionID, Error: msg})
	}

	// 历史 + 追加用户消息
	_, msgs, err := s.store.GetSession(sessionID)
	if err != nil {
		emitErr(err.Error())
		return
	}
	if len(msgs) == 0 {
		if err := s.store.RenameSession(sessionID, firstRunes(text, 30)); err != nil {
			emitErr("更新会话标题失败: " + err.Error())
			return
		}
	}
	if _, err := s.store.AppendMessage(sessionID, chat.RoleUser, text, ""); err != nil {
		emitErr(err.Error())
		return
	}

	// 构造 provider 请求
	provCfg := s.activeProvider()
	if provCfg == nil {
		emitErr("尚未配置可用的 provider，请在设置中填写 baseURL 与 API key")
		return
	}
	if trimSpace(s.settings.ActiveModel) == "" {
		emitErr("尚未选择模型")
		return
	}
	msgs = append(msgs, chat.Message{Role: chat.RoleUser, Content: text})

	prov := s.client(provCfg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// 文本/思考全文累积（跨工具轮）：文本推给 chat:delta，思考推给 chat:thinking
	var full, think strings.Builder
	emitDelta := func(delta string) {
		full.WriteString(delta)
		s.emitter.Emit("chat:delta", ChatDeltaEvent{SessionID: sessionID, Delta: delta})
	}
	emitThink := func(t string) {
		think.WriteString(t)
		s.emitter.Emit("chat:thinking", ChatThinkingEvent{SessionID: sessionID, Text: t})
	}

	// MCP 工具集合（此完成周期的固定快照）
	kit := s.mcpKit(ctx)
	for _, w := range kit.warnings {
		emitThink("[MCP] " + w + "\n")
	}
	req := llm.ChatRequest{Model: s.settings.ActiveModel, ReasoningEffort: effort, Tools: kit.tools()}
	if sp := trimSpace(s.settings.SystemPrompt); sp != "" {
		req.Messages = append(req.Messages, llm.Message{Role: llm.RoleSystem, Content: sp})
	}
	for _, m := range msgs {
		req.Messages = append(req.Messages, llm.Message{Role: llm.Role(m.Role), Content: m.Content})
	}

	maxRounds := 8
	round := 0
	for {
		round++
		res, err := prov.StreamChat(ctx, req, emitDelta, emitThink)
		if err != nil {
			emitErr("请求失败: " + err.Error())
			return
		}

		// 记录本轮 assistant 消息（含 tool_calls，回传给兼容端点必须带上）
		hist := llm.Message{Role: llm.RoleAssistant, Content: res.Content}
		if len(res.ToolCalls) > 0 {
			hist.ToolCalls = res.ToolCalls
		}
		req.Messages = append(req.Messages, hist)

		if len(res.ToolCalls) == 0 {
			break // 本轮直接给出最终文本
		}
		if round >= maxRounds {
			emitErr("工具调用轮次过多，已停止（可能陷入循环）")
			return
		}

		// 依次执行工具，结果以 tool 角色回传
		for _, tc := range res.ToolCalls {
			name := tc.Function.Name
			emitThink("调用了工具 " + name + "\n")
			out, callErr := s.execTool(ctx, kit, tc)
			if callErr != nil {
				out = "[工具执行失败] " + callErr.Error()
			} else if out == "" {
				out = "（工具无返回）"
			}
			req.Messages = append(req.Messages, llm.Message{
				Role:       llm.RoleTool,
				ToolCallID: tc.ID,
				Content:    out,
			})
		}
	}

	content := strings.TrimSpace(full.String())
	if content == "" {
		// 无文本（理论少见）：以最后一轮 model 回复兜底
		content = strings.TrimSpace(req.Messages[len(req.Messages)-1].Content)
	}
	thinkingText := strings.TrimSpace(think.String())
	if _, err := s.store.AppendMessage(sessionID, chat.RoleAssistant, content, thinkingText); err != nil {
		emitErr(err.Error())
		return
	}
	s.emitter.Emit("chat:done", ChatDoneEvent{SessionID: sessionID, Content: content, Thinking: thinkingText})
}

// ---------- MCP 工具集成 ----------

// mcpKit 是本轮对话所用 MCP 服务器的快照：按 endpoint 复用客户端，
// 工具统一暴露为 "serverID_toolName"（仅 [a-zA-Z0-9_-]），避免跨服务器同名冲突。
type mcpKit struct {
	clients  map[string]*mcp.Client // serverID -> client
	byName   map[string]mcpToolRef  // 完整工具名 -> 引用
	toolDefs []llm.ToolDef
	warnings []string
}

type mcpToolRef struct {
	serverID string
	toolName string
}

// mcpKit 从当前设置构建工具快照；单台服务器连不上只记 warning，不阻塞聊天。
func (s *ChatService) mcpKit(ctx context.Context) *mcpKit {
	kit := &mcpKit{
		clients: map[string]*mcp.Client{},
		byName:  map[string]mcpToolRef{},
	}
	for i := range s.settings.MCPServers {
		sv := &s.settings.MCPServers[i]
		ep := trimSpace(sv.Endpoint)
		if ep == "" || trimSpace(sv.ID) == "" {
			continue
		}
		cl := s.mcpClientFor(ep, trimSpace(sv.APIKey))
		tools, err := cl.ListTools(ctx)
		if err != nil {
			kit.warnings = append(kit.warnings, fmt.Sprintf("「%s」连接失败: %v", labelOr(sv.Label, sv.ID), err))
			continue
		}
		kit.clients[sv.ID] = cl
		for _, t := range tools {
			// OpenAI 兼容端点要求 tools[].function.name 匹配 ^[a-zA-Z0-9_-]+$，
			// 用 "_" 连接 serverID 与转义后的工具名（serverID 恒为字母数字）。
			name := sv.ID + "_" + safeToolName(t.Name)
			params := t.InputSchema
			if len(params) == 0 {
				params = json.RawMessage(`{"type":"object","properties":{}}`)
			}
			kit.byName[name] = mcpToolRef{serverID: sv.ID, toolName: t.Name}
			kit.toolDefs = append(kit.toolDefs, llm.ToolDef{
				Type: "function",
				Function: llm.ToolFunctionDef{
					Name:        name,
					Description: t.Description,
					Parameters:  params,
				},
			})
		}
	}
	return kit
}

// tools 返回 llm 层工具定义（无工具时返回 nil）。
func (k *mcpKit) tools() []llm.ToolDef {
	return k.toolDefs
}

// execTool 在对应 MCP 服务器上执行一次工具调用。
func (s *ChatService) execTool(ctx context.Context, kit *mcpKit, tc llm.ToolCall) (string, error) {
	ref, ok := kit.byName[tc.Function.Name]
	if !ok {
		return "", fmt.Errorf("未知工具 %q（可能已被移除）", tc.Function.Name)
	}
	cl := kit.clients[ref.serverID]
	if cl == nil {
		return "", fmt.Errorf("工具 %q 所属 MCP 服务器未连接", tc.Function.Name)
	}
	callCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	res, err := cl.CallTool(callCtx, ref.toolName, tc.Function.Arguments)
	if err != nil {
		return "", err
	}
	if res.IsError {
		return "", fmt.Errorf("工具返回错误: %s", res.Content)
	}
	return res.Content, nil
}

// mcpClientFor 按 endpoint+apiKey 返回缓存的 MCP 客户端（key 变化即重建）。
func (s *ChatService) mcpClientFor(endpoint, apiKey string) *mcp.Client {
	key := endpoint + "\x00" + apiKey
	s.mcpMu.Lock()
	defer s.mcpMu.Unlock()
	if cl, ok := s.mcpClients[key]; ok {
		return cl
	}
	cl := mcp.New(endpoint, apiKey)
	s.mcpClients[key] = cl
	return cl
}

func labelOr(label, id string) string {
	if trimSpace(label) != "" {
		return trimSpace(label)
	}
	return id
}

// safeToolName 把 MCP 工具名转成 OpenAI 兼容端点接受的名字
// （仅 [a-zA-Z0-9_-]；非法字符替换为 _）。
func safeToolName(name string) string {
	if name == "" {
		return "tool"
	}
	var b strings.Builder
	for _, r := range name {
		ok := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-'
		if ok {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

func (s *ChatService) activeProvider() *config.Provider {
	for i := range s.settings.Providers {
		if s.settings.Providers[i].ID == s.settings.ActiveProviderID {
			return &s.settings.Providers[i]
		}
	}
	return nil
}

func validateSettings(st *config.Settings) error {
	// providers 允许为空：外观等设置允许在配置 provider 之前保存；
	// 仅当声明了 activeProviderId 时要求其指向已配置的 provider。
	if st.ActiveProviderID != "" {
		found := false
		for _, p := range st.Providers {
			if trimSpace(p.ID) == "" || trimSpace(p.BaseURL) == "" {
				return fmt.Errorf("provider %q 缺少 id 或 baseURL", p.Label)
			}
			if p.ID == st.ActiveProviderID {
				found = true
			}
		}
		if !found {
			return errors.New("activeProviderId 未指向任何已配置 provider")
		}
	}
	// MCP 服务器基本校验：填写了就必须有 ID 与合法的 http(s) 端点
	for i := range st.MCPServers {
		m := &st.MCPServers[i]
		if trimSpace(m.ID) == "" || trimSpace(m.Endpoint) == "" {
			return fmt.Errorf("MCP 服务器 %q 缺少 id 或 endpoint", m.Label)
		}
		if !strings.HasPrefix(m.Endpoint, "http://") && !strings.HasPrefix(m.Endpoint, "https://") {
			return fmt.Errorf("MCP 服务器 %q 的 endpoint 必须是 http(s) URL", m.Label)
		}
	}
	return nil
}

func sessionToDTO(ss chat.Session) SessionDTO {
	return SessionDTO{
		ID:          ss.ID,
		Title:       ss.Title,
		CreatedAtMs: ss.CreatedAt.UnixMilli(),
		UpdatedAtMs: ss.UpdatedAt.UnixMilli(),
	}
}

// 小工具

func firstRunes(s string, n int) string {
	r := []rune(trimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n])
}

func trimSpace(v string) string { return strings.TrimSpace(v) }
