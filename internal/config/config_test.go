package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestThoughtName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// Local paths
		{"weather.md", "weather"},
		{"weather.thought", "weather"},
		{"examples/weather.md", "weather"},
		{"foo/bar/baz/script.md", "script"},
		{"/absolute/path/to/hello.md", "hello"},

		// URLs
		{"https://example.com/weather.md", "weather"},
		{"https://example.com/path/to/script.thought", "script"},
		{"https://example.com/weather.md?v=2", "weather"},
		{"https://example.com/weather.md?foo=bar&baz=qux", "weather"},
		{"http://localhost:8080/test.md", "test"},

		// Edge cases
		{"noextension", "noextension"},
		{".hidden.md", ".hidden"},
		{"multiple.dots.in.name.md", "multiple.dots.in.name"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ThoughtName(tt.input)
			if got != tt.want {
				t.Errorf("ThoughtName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestThoughtDir(t *testing.T) {
	// Set a known home dir for testing
	tmpHome := t.TempDir()
	t.Setenv("THINKINGSCRIPT_HOME", tmpHome)

	got := ThoughtDir("examples/weather.md")
	want := filepath.Join(tmpHome, "thoughts", "weather")

	if got != want {
		t.Errorf("ThoughtDir() = %q, want %q", got, want)
	}
}

func TestWorkspaceDir(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("THINKINGSCRIPT_HOME", tmpHome)

	got := WorkspaceDir("weather.md")
	want := filepath.Join(tmpHome, "thoughts", "weather", "workspace")

	if got != want {
		t.Errorf("WorkspaceDir() = %q, want %q", got, want)
	}
}

func TestMemoryJSPath(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("THINKINGSCRIPT_HOME", tmpHome)

	got := MemoryJSPath("weather.md")
	want := filepath.Join(tmpHome, "thoughts", "weather", "memory.js")

	if got != want {
		t.Errorf("MemoryJSPath() = %q, want %q", got, want)
	}
}

func TestMemoriesDir(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("THINKINGSCRIPT_HOME", tmpHome)

	got := MemoriesDir("weather.md")
	want := filepath.Join(tmpHome, "thoughts", "weather", "memories")

	if got != want {
		t.Errorf("MemoriesDir() = %q, want %q", got, want)
	}
}

func TestBinDir(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("THINKINGSCRIPT_HOME", tmpHome)

	got := BinDir()
	want := filepath.Join(tmpHome, "bin")

	if got != want {
		t.Errorf("BinDir() = %q, want %q", got, want)
	}
}

func TestHomeDir(t *testing.T) {
	t.Run("with THINKINGSCRIPT_HOME set", func(t *testing.T) {
		tmpHome := t.TempDir()
		t.Setenv("THINKINGSCRIPT_HOME", tmpHome)

		got := HomeDir()
		if got != tmpHome {
			t.Errorf("HomeDir() = %q, want %q", got, tmpHome)
		}
	})

	t.Run("without THINKINGSCRIPT_HOME", func(t *testing.T) {
		t.Setenv("THINKINGSCRIPT_HOME", "")

		got := HomeDir()
		if !strings.HasSuffix(got, ".thinkingscript") {
			t.Errorf("HomeDir() = %q, want suffix %q", got, ".thinkingscript")
		}
	})
}

func TestEnsureHomeDir(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("THINKINGSCRIPT_HOME", tmpHome)

	err := EnsureHomeDir()
	if err != nil {
		t.Fatalf("EnsureHomeDir() error: %v", err)
	}

	// Check all expected directories exist
	expectedDirs := []string{
		tmpHome,
		filepath.Join(tmpHome, "agents"),
		filepath.Join(tmpHome, "bin"),
		filepath.Join(tmpHome, "cache"),
		filepath.Join(tmpHome, "thoughts"),
	}

	for _, dir := range expectedDirs {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("expected directory %q to exist: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected %q to be a directory", dir)
		}
	}
}

func TestCacheDir(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("THINKINGSCRIPT_HOME", tmpHome)

	tests := []struct {
		fingerprint string
		wantSuffix  string
	}{
		{"abcd1234", "abcd1234"},
		{"abcdef1234567890abcdef1234567890abcdef12", "abcdef1234567890abcdef1234567890"}, // truncated to 32
		{"short", "short"},
	}

	for _, tt := range tests {
		t.Run(tt.fingerprint, func(t *testing.T) {
			got := CacheDir(tt.fingerprint)
			wantPrefix := filepath.Join(tmpHome, "cache")

			if !strings.HasPrefix(got, wantPrefix) {
				t.Errorf("CacheDir(%q) = %q, want prefix %q", tt.fingerprint, got, wantPrefix)
			}
			if !strings.HasSuffix(got, tt.wantSuffix) {
				t.Errorf("CacheDir(%q) = %q, want suffix %q", tt.fingerprint, got, tt.wantSuffix)
			}
		})
	}
}

func TestFingerprint(t *testing.T) {
	// Same content should produce same fingerprint
	data := []byte("hello world")
	fp1 := Fingerprint(data)
	fp2 := Fingerprint(data)

	if fp1 != fp2 {
		t.Errorf("same content produced different fingerprints: %q vs %q", fp1, fp2)
	}

	// Different content should produce different fingerprint
	fp3 := Fingerprint([]byte("different content"))
	if fp1 == fp3 {
		t.Errorf("different content produced same fingerprint")
	}

	// Fingerprint should be hex string
	for _, c := range fp1 {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("fingerprint contains non-hex character: %c", c)
		}
	}
}

func TestCheckFingerprint(t *testing.T) {
	dir := t.TempDir()

	t.Run("matching fingerprint", func(t *testing.T) {
		fingerprint := "abc123"
		os.WriteFile(filepath.Join(dir, "fingerprint"), []byte(fingerprint), 0644)

		if !CheckFingerprint(dir, fingerprint) {
			t.Error("CheckFingerprint should return true for matching fingerprint")
		}
	})

	t.Run("non-matching fingerprint", func(t *testing.T) {
		os.WriteFile(filepath.Join(dir, "fingerprint"), []byte("abc123"), 0644)

		if CheckFingerprint(dir, "different") {
			t.Error("CheckFingerprint should return false for non-matching fingerprint")
		}
	})

	t.Run("missing fingerprint file", func(t *testing.T) {
		emptyDir := t.TempDir()

		if CheckFingerprint(emptyDir, "anything") {
			t.Error("CheckFingerprint should return false when file doesn't exist")
		}
	})

	t.Run("fingerprint with whitespace", func(t *testing.T) {
		fingerprint := "abc123"
		os.WriteFile(filepath.Join(dir, "fingerprint"), []byte(fingerprint+"\n"), 0644)

		if !CheckFingerprint(dir, fingerprint) {
			t.Error("CheckFingerprint should trim whitespace")
		}
	})
}

func TestWriteFingerprint(t *testing.T) {
	dir := t.TempDir()
	fingerprint := "test-fingerprint-123"

	err := WriteFingerprint(dir, fingerprint)
	if err != nil {
		t.Fatalf("WriteFingerprint error: %v", err)
	}

	// Read it back
	data, err := os.ReadFile(filepath.Join(dir, "fingerprint"))
	if err != nil {
		t.Fatalf("reading fingerprint file: %v", err)
	}

	if string(data) != fingerprint {
		t.Errorf("fingerprint = %q, want %q", string(data), fingerprint)
	}
}

func TestLoadConfig(t *testing.T) {
	t.Run("default values when no config file", func(t *testing.T) {
		tmpHome := t.TempDir()
		t.Setenv("THINKINGSCRIPT_HOME", tmpHome)

		cfg := LoadConfig()

		if cfg.Agent != DefaultAgent {
			t.Errorf("Agent = %q, want %q", cfg.Agent, DefaultAgent)
		}
		if cfg.MaxTokens != DefaultMaxTokens {
			t.Errorf("MaxTokens = %d, want %d", cfg.MaxTokens, DefaultMaxTokens)
		}
		if cfg.MaxIterations != DefaultMaxIterations {
			t.Errorf("MaxIterations = %d, want %d", cfg.MaxIterations, DefaultMaxIterations)
		}
	})

	t.Run("loads from config file", func(t *testing.T) {
		tmpHome := t.TempDir()
		t.Setenv("THINKINGSCRIPT_HOME", tmpHome)

		configJSON := `{"version": 1, "agent": "custom", "max_tokens": 8192, "max_iterations": 100}`
		os.WriteFile(filepath.Join(tmpHome, "config.json"), []byte(configJSON), 0644)

		cfg := LoadConfig()

		if cfg.Agent != "custom" {
			t.Errorf("Agent = %q, want %q", cfg.Agent, "custom")
		}
		if cfg.MaxTokens != 8192 {
			t.Errorf("MaxTokens = %d, want %d", cfg.MaxTokens, 8192)
		}
		if cfg.MaxIterations != 100 {
			t.Errorf("MaxIterations = %d, want %d", cfg.MaxIterations, 100)
		}
	})
}

func TestResolve(t *testing.T) {
	t.Run("defaults with no config", func(t *testing.T) {
		tmpHome := t.TempDir()
		t.Setenv("THINKINGSCRIPT_HOME", tmpHome)
		t.Setenv("THINKINGSCRIPT__AGENT", "")
		t.Setenv("THINKINGSCRIPT__MODEL", "")
		t.Setenv("THINKINGSCRIPT__ANTHROPIC__API_KEY", "")

		resolved := Resolve(nil)

		if resolved.Provider != "anthropic" {
			t.Errorf("Provider = %q, want %q", resolved.Provider, "anthropic")
		}
		if resolved.Model != DefaultModel {
			t.Errorf("Model = %q, want %q", resolved.Model, DefaultModel)
		}
	})

	t.Run("frontmatter overrides defaults", func(t *testing.T) {
		tmpHome := t.TempDir()
		t.Setenv("THINKINGSCRIPT_HOME", tmpHome)
		t.Setenv("THINKINGSCRIPT__MODEL", "")

		maxTokens := 2048
		scriptCfg := &ScriptConfig{
			Model:     "custom-model",
			MaxTokens: &maxTokens,
		}

		resolved := Resolve(scriptCfg)

		if resolved.Model != "custom-model" {
			t.Errorf("Model = %q, want %q", resolved.Model, "custom-model")
		}
		if resolved.MaxTokens != 2048 {
			t.Errorf("MaxTokens = %d, want %d", resolved.MaxTokens, 2048)
		}
	})

	t.Run("env vars override frontmatter", func(t *testing.T) {
		tmpHome := t.TempDir()
		t.Setenv("THINKINGSCRIPT_HOME", tmpHome)
		t.Setenv("THINKINGSCRIPT__MODEL", "env-model")
		t.Setenv("THINKINGSCRIPT__ANTHROPIC__API_KEY", "sk-test-key")

		scriptCfg := &ScriptConfig{
			Model: "frontmatter-model",
		}

		resolved := Resolve(scriptCfg)

		if resolved.Model != "env-model" {
			t.Errorf("Model = %q, want %q", resolved.Model, "env-model")
		}
		if resolved.APIKey != "sk-test-key" {
			t.Errorf("APIKey = %q, want %q", resolved.APIKey, "sk-test-key")
		}
	})
}

func TestSaveAgent(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("THINKINGSCRIPT_HOME", tmpHome)

	agent := &AgentConfig{
		Version:  1,
		Provider: "anthropic",
		APIKey:   "sk-test",
		Model:    "claude-3",
	}

	err := SaveAgent("test-agent", agent)
	if err != nil {
		t.Fatalf("SaveAgent error: %v", err)
	}

	// Verify file exists with correct permissions
	path := filepath.Join(tmpHome, "agents", "test-agent.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("agent file not created: %v", err)
	}

	// Check permissions (0600)
	if info.Mode().Perm() != 0600 {
		t.Errorf("permissions = %o, want %o", info.Mode().Perm(), 0600)
	}

	// Load it back
	loaded := LoadAgent("test-agent")
	if loaded.Provider != "anthropic" {
		t.Errorf("Provider = %q, want %q", loaded.Provider, "anthropic")
	}
	if loaded.APIKey != "sk-test" {
		t.Errorf("APIKey = %q, want %q", loaded.APIKey, "sk-test")
	}
}
