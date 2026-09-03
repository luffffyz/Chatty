package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// chunkBody 构造一个 OpenAI 流式 chunk 的 JSON 文本。
func chunkBody(model, content string, finish *string) string {
	m := map[string]any{
		"id":      "chatcmpl-test",
		"object":  "chat.completion.chunk",
		"model":   model,
		"choices": []map[string]any{{"index": 0, "delta": map[string]any{"content": content}, "finish_reason": finish}},
	}
	b, _ := json.Marshal(m)
	return string(b)
}

func usageChunk(prompt, completion int) string {
	m := map[string]any{
		"choices": []map[string]any{{"index": 0, "delta": map[string]any{}, "finish_reason": nil}},
		"usage":   map[string]int{"prompt_tokens": prompt, "completion_tokens": completion},
	}
	b, _ := json.Marshal(m)
	return string(b)
}

func sseEvent(data string) string { return "data: " + data + "\n\n" }

func TestStreamChat_CollectsDeltasAndResult(t *testing.T) {
	var mu sync.Mutex
	var gotPath, gotAuth, gotBody string
	stop := "stop"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		gotBody = string(buf)

		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w,
			sseEvent(chunkBody("demo-model", "你好", nil))+
				sseEvent(chunkBody("demo-model", "，世界", nil))+
				sseEvent(chunkBody("demo-model", "", &stop))+
				sseEvent(usageChunk(12, 7))+
				sseEvent("[DONE]"),
		)
	}))
	defer srv.Close()

	p := NewOpenAICompatible("test", srv.URL+"/v1", "sk-test")
	var mu2 sync.Mutex
	var deltas []string
	res, err := p.StreamChat(context.Background(), ChatRequest{
		Model: "demo-model",
		Messages: []Message{
			{Role: RoleSystem, Content: "say hi"},
			{Role: RoleUser, Content: "hi"},
		},
	}, func(d string) {
		mu2.Lock()
		defer mu2.Unlock()
		deltas = append(deltas, d)
	})
	if err != nil {
		t.Fatalf("StreamChat error: %v", err)
	}

	if gotPath != "/v1/chat/completions" {
		t.Errorf("path = %q, want /v1/chat/completions", gotPath)
	}
	if gotAuth != "Bearer sk-test" {
		t.Errorf("auth = %q", gotAuth)
	}
	for _, want := range []string{`"stream":true`, `"role":"system"`, `"role":"user"`, `"model":"demo-model"`} {
		if !strings.Contains(gotBody, want) {
			t.Errorf("body missing %s; body=%s", want, gotBody)
		}
	}
	if res.Content != "你好，世界" {
		t.Errorf("content = %q", res.Content)
	}
	if len(deltas) != 2 || deltas[0] != "你好" || deltas[1] != "，世界" {
		t.Errorf("deltas = %#v", deltas)
	}
	if res.StopReason != "stop" {
		t.Errorf("stopReason = %q", res.StopReason)
	}
	if res.InputTokens != 12 || res.OutputTokens != 7 {
		t.Errorf("usage = %d/%d", res.InputTokens, res.OutputTokens)
	}
	if res.Model != "demo-model" {
		t.Errorf("model = %q", res.Model)
	}
}

func TestStreamChat_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"Invalid API key","type":"invalid_request_error"}}`))
	}))
	defer srv.Close()

	p := NewOpenAICompatible("test", srv.URL+"/v1", "bad-key")
	_, err := p.StreamChat(context.Background(), ChatRequest{Model: "m", Messages: []Message{{Role: RoleUser, Content: "x"}}}, nil)
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "Invalid API key") {
		t.Errorf("error = %v", err)
	}
}

func TestStreamChat_StreamErrorChunk(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseEvent(`{"error":{"message":"upstream boom"}}`))
	}))
	defer srv.Close()

	p := NewOpenAICompatible("test", srv.URL+"/v1", "k")
	_, err := p.StreamChat(context.Background(), ChatRequest{Model: "m", Messages: []Message{{Role: RoleUser, Content: "x"}}}, nil)
	if err == nil || !strings.Contains(err.Error(), "upstream boom") {
		t.Fatalf("error = %v", err)
	}
}

func TestChat_CollectsFullContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseEvent(chunkBody("m", "a", nil))+sseEvent(chunkBody("m", "b", nil))+sseEvent("[DONE]"))
	}))
	defer srv.Close()

	p := NewOpenAICompatible("test", srv.URL+"/v1", "k")
	res, err := Chat(context.Background(), p, ChatRequest{Model: "m", Messages: []Message{{Role: RoleUser, Content: "hi"}}})
	if err != nil {
		t.Fatalf("Chat error: %v", err)
	}
	if res.Content != "ab" {
		t.Errorf("content = %q, want ab", res.Content)
	}
}

func TestStreamChat_MissingModel(t *testing.T) {
	p := NewOpenAICompatible("test", "", "")
	_, err := p.StreamChat(context.Background(), ChatRequest{}, nil)
	if err == nil {
		t.Fatal("want error for empty model")
	}
}
