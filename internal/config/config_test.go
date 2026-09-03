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
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	in := Default()
	in.Providers = []Provider{
		{ID: "openrouter", Label: "OpenRouter", BaseURL: "https://openrouter.ai/api/v1", APIKey: "sk-x"},
	}
	in.ActiveProviderID = "openrouter"
	in.ActiveModel = "openai/gpt-4o-mini"

	if err := Save(path, in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	out, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(out.Providers) != 1 || out.Providers[0].APIKey != "sk-x" || out.ActiveModel != "openai/gpt-4o-mini" {
		t.Errorf("round trip mismatch: %+v", out)
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

func TestLegacyPromptMigratedOnLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-prompt.json")
	in := Default()
	in.SystemPrompt = legacyDefaultPrompt
	raw, _ := json.Marshal(in)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if out.SystemPrompt != defaultSystemPrompt() {
		t.Error("legacy default prompt should be migrated to the new default")
	}
	if strings.Contains(out.SystemPrompt, "围栏内，写完整 Typst") {
		t.Errorf("still contains legacy wording")
	}
}

func TestFlowPromptMigratedOnLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flow-prompt.json")
	in := Default()
	in.SystemPrompt = flowDefaultPrompt
	raw, _ := json.Marshal(in)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if out.SystemPrompt != defaultSystemPrompt() {
		t.Error("flow default prompt should be migrated to the latest default")
	}
	if !strings.Contains(out.SystemPrompt, ">=") {
		t.Errorf("new prompt should teach typst >= symbol")
	}
}

func TestPromptV4MigratedOnLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v4-prompt.json")
	in := Default()
	in.SystemPrompt = promptV4Raw
	raw, _ := json.Marshal(in)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if out.SystemPrompt != defaultSystemPrompt() {
		t.Error("v4 default prompt should be migrated to v5")
	}
	if !strings.Contains(out.SystemPrompt, "6e2b2b") {
		t.Errorf("v5 prompt should mention mermaid palette")
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
