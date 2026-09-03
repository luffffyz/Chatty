package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"chatty/internal/chat"
	"chatty/internal/config"
	"chatty/internal/llm"
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
		busy: make(map[string]bool),
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
		out[i] = MessageDTO{ID: m.ID, Role: string(m.Role), Content: m.Content, CreatedMs: m.CreatedAt.UnixMilli()}
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

// SendMessage 把用户消息追加到会话并异步流式请求模型。
// 返回错误表示同步失败（会话不存在 / 正忙 / 未配置）；流式期间的
// 增量/结束/错误通过 chat:delta / chat:done / chat:error 事件推送。
func (s *ChatService) SendMessage(sessionID, text string) error {
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

	go s.runCompletion(sessionID, text)
	return nil
}

func (s *ChatService) runCompletion(sessionID, text string) {
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
	if _, err := s.store.AppendMessage(sessionID, chat.RoleUser, text); err != nil {
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

	req := llm.ChatRequest{Model: s.settings.ActiveModel}
	if sp := trimSpace(s.settings.SystemPrompt); sp != "" {
		req.Messages = append(req.Messages, llm.Message{Role: llm.RoleSystem, Content: sp})
	}
	for _, m := range msgs {
		req.Messages = append(req.Messages, llm.Message{Role: llm.Role(m.Role), Content: m.Content})
	}

	prov := s.client(provCfg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var full strings.Builder
	res, err := prov.StreamChat(ctx, req, func(delta string) {
		full.WriteString(delta)
		s.emitter.Emit("chat:delta", ChatDeltaEvent{SessionID: sessionID, Delta: delta})
	}, func(think string) {
		// 推理文本逐段透传，前端据此展示"思考中"状态
		s.emitter.Emit("chat:thinking", ChatThinkingEvent{SessionID: sessionID, Text: think})
	})
	if err != nil {
		emitErr("请求失败: " + err.Error())
		return
	}
	if trimSpace(full.String()) == "" {
		full.Reset()
		full.WriteString(res.Content)
	}
	if _, err := s.store.AppendMessage(sessionID, chat.RoleAssistant, full.String()); err != nil {
		emitErr(err.Error())
		return
	}
	s.emitter.Emit("chat:done", ChatDoneEvent{SessionID: sessionID, Content: full.String()})
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
