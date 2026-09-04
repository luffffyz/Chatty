package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMissingReturnsDefault(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s == nil || len(s.Providers) != 0 {
		t.Fatalf("expected default settings with no providers, got %+v", s)
	}
	if !containsFold(s.SystemPrompt, "typst") {
		t.Errorf("default system prompt should mention typst")
	}
	if s.Appearance.Theme != ThemeSystem || s.Appearance.FontSize != DefaultFontSize {
		t.Errorf("default appearance = %+v", s.Appearance)
	}
	if !strings.Contains(s.SystemPrompt, v6Marker) {
		t.Errorf("default prompt should contain v6 marker #sym.num")
	}
}

func TestLoadLegacyFileGetsAppearanceDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.json")
	// 旧版设置没有 appearance 字段
	raw := `{"providers":[],"activeProviderId":"","activeModel":"","systemPrompt":""}`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Appearance.Theme != ThemeSystem || s.Appearance.FontSize != DefaultFontSize {
		t.Errorf("legacy upgrade appearance = %+v", s.Appearance)
	}
	// 空 systemPrompt 不是“默认样式的旧文本”，不应被改写后迁移
	if s.SystemPrompt != "" {
		t.Errorf("empty system prompt should stay empty, got %q", s.SystemPrompt)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	in := Default()
	in.Providers = []Provider{
		{ID: "openrouter", Label: "OpenRouter", BaseURL: "https://openrouter.ai/api/v1", APIKey: "sk-x", Model: "m1", Models: []string{"m1", "m2"}},
	}
	in.ActiveProviderID = "openrouter"
	in.ActiveModel = "m1"

	if err := Save(path, in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	out, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(out.Providers) != 1 || out.Providers[0].APIKey != "sk-x" || out.Providers[0].Model != "m1" {
		t.Errorf("round trip providers mismatch: %+v", out.Providers)
	}
	if len(out.Providers[0].Models) != 2 {
		t.Errorf("models lost: %+v", out.Providers[0].Models)
	}
	if out.SystemPrompt != in.SystemPrompt {
		t.Errorf("system prompt lost in round trip")
	}
}

func TestLoadCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("{oops"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for corrupt settings file")
	}
}

// migrateToJSON 写入一个带给定 systemPrompt 的设置文件并重新 Load。
func migrateToJSON(t *testing.T, sp string) *Settings {
	t.Helper()
	in := Default()
	in.SystemPrompt = sp
	raw, _ := json.Marshal(in)
	path := filepath.Join(t.TempDir(), "migrate.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return out
}

func TestOldDefaultPromptsMigrated(t *testing.T) {
	// 各历史版默认提示词的开头（未含 v6 标记）
	oldV1 := "你是 Chatty，一个以 Typst 为主要排版格式的桌面聊天助手。\n\n回复规则：\n- 涉及公式的片段放在 ```typst 围栏内…"
	oldFlow := "你是 Chatty，一个用 Typst 排版聊天的桌面助手。你的每条回复正文都是一份连续排版的 Typst 文档，客户端会直接把它排版渲染出来…"
	oldV3 := "你是 Chatty，一个用 Typst 排版聊天的桌面助手。你的每条回复正文都是一份连续排版的 Typst 文档，客户端会直接排版渲染…"

	for _, old := range []string{oldV1, oldFlow, oldV3} {
		out := migrateToJSON(t, old)
		if out.SystemPrompt != defaultSystemPrompt() {
			t.Errorf("old default prompt should be migrated to v6; got %q", out.SystemPrompt[:40])
		}
		if !strings.Contains(out.SystemPrompt, v6Marker) {
			t.Error("migrated prompt should contain v6 marker")
		}
	}
}

func TestCustomPromptNotMigrated(t *testing.T) {
	custom := "你是我自定义的系统提示，没有任何默认字样。"
	out := migrateToJSON(t, custom)
	if out.SystemPrompt != custom {
		t.Errorf("custom prompt should stay untouched, got %q", out.SystemPrompt)
	}
}

func containsFold(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if equalFoldASCII(s[i:i+len(sub)], sub) {
			return true
		}
	}
	return false
}

func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
