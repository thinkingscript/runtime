package config

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DefaultAgent         = "anthropic"
	DefaultModel         = "claude-sonnet-4-5-20250929"
	DefaultMaxTokens     = 4096
	DefaultMaxIterations = 50
)

type Config struct {
	Version       int    `yaml:"version"`
	Agent         string `yaml:"agent"`
	Wreckless     bool   `yaml:"wreckless"`
	MaxTokens     int    `yaml:"max_tokens"`
	MaxIterations int    `yaml:"max_iterations"`
}

type AgentConfig struct {
	Version  int    `yaml:"version"`
	Provider string `yaml:"provider"`
	APIKey   string `yaml:"api_key"`
	APIBase  string `yaml:"api_base"`
	Model    string `yaml:"model"`
}

type ScriptConfig struct {
	Agent     string `yaml:"agent"`
	Model     string `yaml:"model"`
	MaxTokens *int   `yaml:"max_tokens"`
}

// ResolvedConfig holds the final merged configuration.
type ResolvedConfig struct {
	Provider      string
	APIKey        string
	APIBase       string
	Model         string
	Wreckless     bool
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
		filepath.Join(home, "cache"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", d, err)
		}
	}
	return nil
}

func LoadConfig() *Config {
	cfg := &Config{
		Version:       1,
		Agent:         DefaultAgent,
		MaxTokens:     DefaultMaxTokens,
		MaxIterations: DefaultMaxIterations,
	}

	path := filepath.Join(HomeDir(), "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	_ = yaml.Unmarshal(data, cfg)

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
	path := filepath.Join(HomeDir(), "agents", name+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return agent
	}
	_ = yaml.Unmarshal(data, agent)
	return agent
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
		Wreckless:     cfg.Wreckless,
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
	if v := getEnv("WRECKLESS"); v == "true" || v == "1" {
		resolved.Wreckless = true
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

// CacheDir returns the cache directory path for a given script path.
func CacheDir(scriptPath string) (string, error) {
	absPath, err := filepath.Abs(scriptPath)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256([]byte(absPath))
	shortHash := fmt.Sprintf("%x", hash[:16])
	return filepath.Join(HomeDir(), "cache", shortHash), nil
}

// Fingerprint computes a SHA-256 hash of file contents.
func Fingerprint(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
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

// WriteMeta stores script metadata in the cache directory.
func WriteMeta(cacheDir, scriptPath string) error {
	meta := map[string]string{
		"script_path": scriptPath,
	}
	data, err := yaml.Marshal(meta)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cacheDir, "meta.json"), data, 0644)
}
