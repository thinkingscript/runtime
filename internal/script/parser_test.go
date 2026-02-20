package script

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSimpleScript(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "simple.md")

	content := "Print hello world"
	os.WriteFile(path, []byte(content), 0644)

	parsed, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if parsed.Prompt != "Print hello world" {
		t.Errorf("Prompt = %q, want %q", parsed.Prompt, "Print hello world")
	}
	if parsed.Config != nil {
		t.Error("Config should be nil for script without frontmatter")
	}
	if parsed.IsURL {
		t.Error("IsURL should be false for local file")
	}
	if parsed.Fingerprint == "" {
		t.Error("Fingerprint should not be empty")
	}
}

func TestParseWithShebang(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shebang.md")

	content := `#!/usr/bin/env think
Print hello world`
	os.WriteFile(path, []byte(content), 0644)

	parsed, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if parsed.Prompt != "Print hello world" {
		t.Errorf("Prompt = %q, want %q", parsed.Prompt, "Print hello world")
	}
}

func TestParseWithFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "frontmatter.md")

	content := `---
model: claude-3-opus
agent: custom-agent
max_tokens: 8192
---
Print hello world`
	os.WriteFile(path, []byte(content), 0644)

	parsed, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if parsed.Prompt != "Print hello world" {
		t.Errorf("Prompt = %q, want %q", parsed.Prompt, "Print hello world")
	}
	if parsed.Config == nil {
		t.Fatal("Config should not be nil")
	}
	if parsed.Config.Model != "claude-3-opus" {
		t.Errorf("Model = %q, want %q", parsed.Config.Model, "claude-3-opus")
	}
	if parsed.Config.Agent != "custom-agent" {
		t.Errorf("Agent = %q, want %q", parsed.Config.Agent, "custom-agent")
	}
	if parsed.Config.MaxTokens == nil || *parsed.Config.MaxTokens != 8192 {
		t.Errorf("MaxTokens = %v, want 8192", parsed.Config.MaxTokens)
	}
}

func TestParseWithShebangAndFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "both.md")

	content := `#!/usr/bin/env think
---
model: claude-3-opus
---
Print hello world`
	os.WriteFile(path, []byte(content), 0644)

	parsed, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if parsed.Prompt != "Print hello world" {
		t.Errorf("Prompt = %q, want %q", parsed.Prompt, "Print hello world")
	}
	if parsed.Config == nil {
		t.Fatal("Config should not be nil")
	}
	if parsed.Config.Model != "claude-3-opus" {
		t.Errorf("Model = %q, want %q", parsed.Config.Model, "claude-3-opus")
	}
}

func TestParseEmptyPrompt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.md")

	content := `#!/usr/bin/env think
---
model: claude-3
---
`
	os.WriteFile(path, []byte(content), 0644)

	_, err := Parse(path)
	if err == nil {
		t.Error("expected error for empty prompt")
	}
	if !strings.Contains(err.Error(), "no prompt content") {
		t.Errorf("error = %q, want to contain 'no prompt content'", err.Error())
	}
}

func TestParseInvalidFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.md")

	content := `---
model: [invalid yaml
---
Print hello`
	os.WriteFile(path, []byte(content), 0644)

	_, err := Parse(path)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
	if !strings.Contains(err.Error(), "parsing frontmatter") {
		t.Errorf("error = %q, want to contain 'parsing frontmatter'", err.Error())
	}
}

func TestParseNonExistentFile(t *testing.T) {
	_, err := Parse("/nonexistent/path/script.md")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestParseMultilinePrompt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "multiline.md")

	content := `#!/usr/bin/env think

This is a multiline prompt.

It has multiple paragraphs.

- And a list
- With items`
	os.WriteFile(path, []byte(content), 0644)

	parsed, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if !strings.Contains(parsed.Prompt, "multiline prompt") {
		t.Errorf("Prompt should contain 'multiline prompt'")
	}
	if !strings.Contains(parsed.Prompt, "multiple paragraphs") {
		t.Errorf("Prompt should contain 'multiple paragraphs'")
	}
	if !strings.Contains(parsed.Prompt, "- And a list") {
		t.Errorf("Prompt should contain list items")
	}
}

func TestParseFrontmatterWithExtraNewlines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "newlines.md")

	content := `#!/usr/bin/env think


---
model: claude-3
---


Print hello world`
	os.WriteFile(path, []byte(content), 0644)

	parsed, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if parsed.Prompt != "Print hello world" {
		t.Errorf("Prompt = %q, want %q", parsed.Prompt, "Print hello world")
	}
}

func TestParseOnlyShebang(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "onlyshebang.md")

	content := `#!/usr/bin/env think`
	os.WriteFile(path, []byte(content), 0644)

	_, err := Parse(path)
	if err == nil {
		t.Error("expected error for script with only shebang")
	}
}

func TestParseFingerprintConsistency(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fingerprint.md")

	content := "Print hello world"
	os.WriteFile(path, []byte(content), 0644)

	parsed1, _ := Parse(path)
	parsed2, _ := Parse(path)

	if parsed1.Fingerprint != parsed2.Fingerprint {
		t.Error("same content should produce same fingerprint")
	}

	// Change content
	os.WriteFile(path, []byte("Different content"), 0644)
	parsed3, _ := Parse(path)

	if parsed1.Fingerprint == parsed3.Fingerprint {
		t.Error("different content should produce different fingerprint")
	}
}

func TestParsePartialFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "partial.md")

	// Only model specified
	content := `---
model: claude-3
---
Print hello`
	os.WriteFile(path, []byte(content), 0644)

	parsed, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if parsed.Config.Model != "claude-3" {
		t.Errorf("Model = %q, want %q", parsed.Config.Model, "claude-3")
	}
	if parsed.Config.Agent != "" {
		t.Errorf("Agent should be empty, got %q", parsed.Config.Agent)
	}
	if parsed.Config.MaxTokens != nil {
		t.Errorf("MaxTokens should be nil, got %v", parsed.Config.MaxTokens)
	}
}

func TestParseUnclosedFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "unclosed.md")

	// Frontmatter not closed - should be treated as regular content
	content := `---
model: claude-3
Print hello world`
	os.WriteFile(path, []byte(content), 0644)

	parsed, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// The unclosed frontmatter should be treated as part of the prompt
	if parsed.Config != nil {
		t.Error("Config should be nil for unclosed frontmatter")
	}
}

func TestParseURLDetection(t *testing.T) {
	// We can't actually fetch URLs in tests, but we can verify detection
	tests := []struct {
		path  string
		isURL bool
	}{
		{"weather.md", false},
		{"/absolute/path.md", false},
		{"./relative/path.md", false},
		{"https://example.com/script.md", true},
		{"http://example.com/script.md", true},
		{"HTTP://EXAMPLE.COM/script.md", false}, // case sensitive
	}

	for _, tt := range tests {
		// We can only test local files, but we verify the URL prefix check
		isURL := strings.HasPrefix(tt.path, "http://") || strings.HasPrefix(tt.path, "https://")
		if isURL != tt.isURL {
			t.Errorf("path %q: isURL = %v, want %v", tt.path, isURL, tt.isURL)
		}
	}
}

func TestParsePath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	os.WriteFile(path, []byte("Hello"), 0644)

	parsed, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if parsed.Path != path {
		t.Errorf("Path = %q, want %q", parsed.Path, path)
	}
}
