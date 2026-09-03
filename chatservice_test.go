package main

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"chatty/internal/chat"
	"chatty/internal/config"
	"chatty/internal/llm"
)

// ---------- fakes ----------

type fakeProvider struct {
	name string
	mu   sync.Mutex
	reqs []llm.ChatRequest
}

func (f *fakeProvider) Name() string { return f.name }

func (f *fakeProvider) StreamChat(_ context.Context, req llm.ChatRequest, onDelta llm.DeltaFunc) (*llm.ChatResult, error) {
	f.mu.Lock()
	f.reqs = append(f.reqs, req)
	f.mu.Unlock()
	for _, d := range []string{"a", "b", "c"} {
		if onDelta != nil {
			onDelta(d)
		}
	}
	return &llm.ChatResult{Content: "abc", Model: "fake"}, nil
}

type fakeEmitter struct {
	mu   sync.Mutex
	recs []emittedEvent
	done chan struct{}
}

type emittedEvent struct {
	name string
	data any
}

func (e *fakeEmitter) Emit(name string, data ...any) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, d := range data {
		e.recs = append(e.recs, emittedEvent{name: name, data: d})
		if name == "chat:done" {
			select {
			case <-e.done:
			default:
				close(e.done)
			}
		}
	}
	return true
}

func (e *fakeEmitter) eventsNamed(name string) []emittedEvent {
	e.mu.Lock()
	defer e.mu.Unlock()
	var out []emittedEvent
	for _, r := range e.recs {
		if r.name == name {
			out = append(out, r)
		}
	}
	return out
}

func waitDone(t *testing.T, em *fakeEmitter) {
	t.Helper()
	select {
	case <-em.done:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for chat:done")
	}
}

func newTestService(t *testing.T) (*ChatService, *fakeEmitter, *fakeProvider, *chat.SQLiteStore) {
	t.Helper()
	st, err := chat.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	cfg := config.Default()
	cfg.Providers = []config.Provider{
		{ID: "fake", Label: "Fake", BaseURL: "http://localhost/v1"},
	}
	cfg.ActiveProviderID = "fake"
	cfg.ActiveModel = "fake-model"

	em := &fakeEmitter{done: make(chan struct{})}
	fp := &fakeProvider{name: "fake"}
	svc := NewChatService(st, cfg, filepath.Join(t.TempDir(), "settings.json"), em)
	svc.client = func(p *config.Provider) llm.Provider { return fp }
	return svc, em, fp, st
}

// ---------- tests ----------

func TestSendMessageStreamsAndPersists(t *testing.T) {
	svc, em, fp, st := newTestService(t)

	s, err := svc.NewSession()
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if err := svc.SendMessage(s.ID, "解释欧拉恒等式"); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	waitDone(t, em)

	// 会话被首条消息改名
	got, _, _ := st.GetSession(s.ID)
	if got.Title != "解释欧拉恒等式" {
		t.Errorf("title = %q", got.Title)
	}

	// 消息持久化：user + assistant
	_, msgs, err := st.GetSession(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 || msgs[0].Role != chat.RoleUser || msgs[1].Role != chat.RoleAssistant {
		t.Fatalf("msgs = %+v", msgs)
	}
	if msgs[1].Content != "abc" {
		t.Errorf("assistant content = %q", msgs[1].Content)
	}

	// 事件：3 个 delta + 1 个 done
	deltas := em.eventsNamed("chat:delta")
	if len(deltas) != 3 {
		t.Errorf("delta events = %d, want 3", len(deltas))
	}
	if len(em.eventsNamed("chat:done")) != 1 {
		t.Error("missing chat:done")
	}

	// 发给模型的请求：system 提示在前，历史含 1 条 user
	fp.mu.Lock()
	defer fp.mu.Unlock()
	if len(fp.reqs) != 1 {
		t.Fatalf("requests = %d", len(fp.reqs))
	}
	req := fp.reqs[0]
	if req.Model != "fake-model" {
		t.Errorf("model = %q", req.Model)
	}
	if len(req.Messages) != 2 || req.Messages[0].Role != llm.RoleSystem {
		t.Errorf("messages = %+v", req.Messages)
	}
}

func TestSendMessageWithoutProviderEmitsError(t *testing.T) {
	st, err := chat.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cfg := config.Default() // 无 provider
	em := &fakeEmitter{done: make(chan struct{})}
	svc := NewChatService(st, cfg, filepath.Join(t.TempDir(), "s.json"), em)

	s, _ := st.CreateSession("")
	if err := svc.SendMessage(s.ID, "hi"); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	// 该路径没有 chat:done，轮询 chat:error
	deadline := time.Now().Add(2 * time.Second)
	for {
		if len(em.eventsNamed("chat:error")) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("expected chat:error event, got none")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestSendMessageConcurrentSendRejected(t *testing.T) {
	svc, _, _, st := newTestService(t)
	s, _ := st.CreateSession("")
	if err := svc.SendMessage(s.ID, "first"); err != nil {
		t.Fatal(err)
	}
	if err := svc.SendMessage(s.ID, "second"); err == nil {
		t.Error("second concurrent SendMessage should be rejected")
	}
}

func TestSaveSettingsValidation(t *testing.T) {
	svc, _, _, st := newTestService(t)
	_ = st
	// 空 providers 且未选 active → 允许（外观页先行保存场景）
	if err := svc.SaveSettings(config.Default()); err != nil {
		t.Errorf("empty providers with no active selection should pass: %v", err)
	}
	// active 指向不存在的 provider → 拒绝
	bad := config.Default()
	bad.ActiveProviderID = "ghost"
	if err := svc.SaveSettings(bad); err == nil {
		t.Error("dangling activeProviderId should be rejected")
	}
	good := config.Default()
	good.Providers = []config.Provider{{ID: "p", BaseURL: "http://x/v1"}}
	good.ActiveProviderID = "p"
	good.ActiveModel = "m"
	if err := svc.SaveSettings(good); err != nil {
		t.Errorf("valid settings rejected: %v", err)
	}
}
