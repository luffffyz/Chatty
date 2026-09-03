package config

import (
	"os"
	"path/filepath"
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
