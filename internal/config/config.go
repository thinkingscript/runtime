package config

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	DefaultAgent         = "anthropic"
	DefaultModel         = "claude-sonnet-4-5-20250929"
	DefaultMaxTokens     = 4096
	DefaultMaxIterations = 50
)

type Config struct {
	Version       int    `json:"version"`
	Agent         string `json:"agent"`
	MaxTokens     int    `json:"max_tokens"`
	MaxIterations int    `json:"max_iterations"`
}

type AgentConfig struct {
	Version  int    `json:"version"`
	Provider string `json:"provider"`
	APIKey   string `json:"api_key"`
	APIBase  string `json:"api_base"`
	Model    string `json:"model"`
}

type ScriptConfig struct {
	Agent     string `json:"agent" yaml:"agent"`
	Model     string `json:"model" yaml:"model"`
	MaxTokens *int   `json:"max_tokens" yaml:"max_tokens"`
}

// ResolvedConfig holds the final merged configuration.
type ResolvedConfig struct {
	Provider      string
	APIKey        string
	APIBase       string
	Model         string
	MaxTokens     int
	MaxIterations int
}

func HomeDir() string {
	if v := os.Getenv("THINKINGSCRIPT_HOME"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".thinkingscript")
	}
	return filepath.Join(home, ".thinkingscript")
}

func EnsureHomeDir() error {
	home := HomeDir()
	dirs := []string{
		home,
		filepath.Join(home, "agents"),
		filepath.Join(home, "bin"),
		filepath.Join(home, "cache"),
		filepath.Join(home, "thoughts"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0700); err != nil {
			return fmt.Errorf("creating directory %s: %w", d, err)
		}
	}
	return nil
}

// ThoughtName derives a human-readable name from a script path or URL.
// "examples/weather.md" → "weather", "https://example.com/weather.md?v=2" → "weather"
func ThoughtName(scriptPath string) string {
	name := scriptPath
	if u, err := url.Parse(scriptPath); err == nil && u.Scheme != "" {
		name = path.Base(u.Path)
	} else {
		name = filepath.Base(scriptPath)
	}
	ext := filepath.Ext(name)
	return strings.TrimSuffix(name, ext)
}

// BinDir returns the directory for installed thought binaries.
func BinDir() string {
	return filepath.Join(HomeDir(), "bin")
}

// ThoughtDir returns the per-thought data directory for a given script.
func ThoughtDir(scriptPath string) string {
	return filepath.Join(HomeDir(), "thoughts", ThoughtName(scriptPath))
}

// WorkspaceDir returns the workspace directory for a given script.
func WorkspaceDir(scriptPath string) string {
	return filepath.Join(ThoughtDir(scriptPath), "workspace")
}

// MemoriesDir returns the memories directory for a given script.
// Memories are stored by thought name, not content hash, so they
// survive script edits and binary rebuilds.
func MemoriesDir(scriptPath string) string {
	return filepath.Join(ThoughtDir(scriptPath), "memories")
}

func LoadConfig() *Config {
	cfg := &Config{
		Version:       1,
		Agent:         DefaultAgent,
		MaxTokens:     DefaultMaxTokens,
		MaxIterations: DefaultMaxIterations,
	}

	path := filepath.Join(HomeDir(), "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(data, cfg)

	if cfg.Agent == "" {
		cfg.Agent = DefaultAgent
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = DefaultMaxTokens
	}
	if cfg.MaxIterations == 0 {
		cfg.MaxIterations = DefaultMaxIterations
	}
	return cfg
}

func LoadAgent(name string) *AgentConfig {
	agent := &AgentConfig{}
	path := filepath.Join(HomeDir(), "agents", name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return agent
	}
	_ = json.Unmarshal(data, agent)
	return agent
}

// SaveAgent writes an agent config to the agents directory with 0600 permissions.
func SaveAgent(name string, agent *AgentConfig) error {
	dir := filepath.Join(HomeDir(), "agents")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating agents directory: %w", err)
	}
	data, err := json.MarshalIndent(agent, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling agent config: %w", err)
	}
	path := filepath.Join(dir, name+".json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing agent config: %w", err)
	}
	return nil
}

// Resolve merges config layers: defaults < config.yaml < agent.yaml < frontmatter < env vars
func Resolve(scriptCfg *ScriptConfig) *ResolvedConfig {
	cfg := LoadConfig()

	// Determine agent name: env > frontmatter > config
	agentName := cfg.Agent
	if scriptCfg != nil && scriptCfg.Agent != "" {
		agentName = scriptCfg.Agent
	}
	if v := getEnv("AGENT"); v != "" {
		agentName = v
	}

	agent := LoadAgent(agentName)

	resolved := &ResolvedConfig{
		Provider:      agent.Provider,
		APIKey:        agent.APIKey,
		APIBase:       agent.APIBase,
		Model:         agent.Model,
		MaxTokens:     cfg.MaxTokens,
		MaxIterations: cfg.MaxIterations,
	}

	// Apply defaults if agent file didn't set them
	if resolved.Provider == "" {
		resolved.Provider = "anthropic"
	}
	if resolved.Model == "" {
		resolved.Model = DefaultModel
	}

	// Apply frontmatter overrides
	if scriptCfg != nil {
		if scriptCfg.Model != "" {
			resolved.Model = scriptCfg.Model
		}
		if scriptCfg.MaxTokens != nil {
			resolved.MaxTokens = *scriptCfg.MaxTokens
		}
	}

	// Apply env var overrides
	if v := getEnv("MODEL"); v != "" {
		resolved.Model = v
	}
	if v := getEnv("MAX_TOKENS"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			resolved.MaxTokens = n
		}
	}
	if v := getEnv("ANTHROPIC__API_KEY"); v != "" {
		resolved.APIKey = v
	}
	if v := getEnv("OPENAI__API_KEY"); v != "" && resolved.Provider == "openai" {
		resolved.APIKey = v
	}
	if v := getEnv("OPENAI__API_BASE"); v != "" && resolved.Provider == "openai" {
		resolved.APIBase = v
	}

	return resolved
}

func getEnv(key string) string {
	return os.Getenv("THINKINGSCRIPT__" + key)
}

// CacheDir returns the cache directory path for a given fingerprint.
// Same content produces the same cache dir regardless of source (file or URL).
// The fingerprint is already a hex-encoded hash, so we truncate rather than re-hash.
func CacheDir(fingerprint string) string {
	short := fingerprint
	if len(short) > 32 {
		short = short[:32]
	}
	return filepath.Join(HomeDir(), "cache", short)
}

// Fingerprint computes a SHA-256 hash of the given data combined with the
// running binary's own content. This ensures caches invalidate when either
// the script or the think binary changes.
func Fingerprint(data []byte) string {
	h := sha256.New()
	h.Write(data)
	if exe, err := os.Executable(); err == nil {
		if bin, err := os.ReadFile(exe); err == nil {
			h.Write(bin)
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// CheckFingerprint verifies if the cached fingerprint matches. Returns true if valid.
func CheckFingerprint(cacheDir, currentFingerprint string) bool {
	data, err := os.ReadFile(filepath.Join(cacheDir, "fingerprint"))
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(data)) == currentFingerprint
}

// WriteFingerprint stores the fingerprint in the cache directory.
func WriteFingerprint(cacheDir, fingerprint string) error {
	return os.WriteFile(filepath.Join(cacheDir, "fingerprint"), []byte(fingerprint), 0644)
}

