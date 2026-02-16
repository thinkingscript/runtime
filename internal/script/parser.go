package script

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/thinkingscript/cli/internal/config"
	"gopkg.in/yaml.v3"
)

type ParsedScript struct {
	Prompt      string
	Config      *config.ScriptConfig
	Fingerprint string
	Path        string
	IsURL       bool
}

func Parse(path string) (*ParsedScript, error) {
	isURL := strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://")

	var data []byte
	var err error
	if isURL {
		data, err = fetchURL(path)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, fmt.Errorf("reading script %s: %w", path, err)
	}

	content := string(data)
	fingerprint := config.Fingerprint(data)

	// Strip shebang line
	if strings.HasPrefix(content, "#!") {
		if idx := strings.Index(content, "\n"); idx != -1 {
			content = content[idx+1:]
		} else {
			content = ""
		}
	}

	// Parse optional frontmatter
	var scriptCfg *config.ScriptConfig
	content = strings.TrimLeft(content, "\n")

	if strings.HasPrefix(content, "---") {
		// Find closing ---
		rest := content[3:]
		if idx := strings.Index(rest, "\n"); idx != -1 {
			rest = rest[idx+1:]
		}
		endIdx := strings.Index(rest, "---")
		if endIdx != -1 {
			frontmatter := rest[:endIdx]
			scriptCfg = &config.ScriptConfig{}
			if err := yaml.Unmarshal([]byte(frontmatter), scriptCfg); err != nil {
				return nil, fmt.Errorf("parsing frontmatter: %w", err)
			}
			// Skip past closing --- and newline
			rest = rest[endIdx+3:]
			if len(rest) > 0 && rest[0] == '\n' {
				rest = rest[1:]
			}
			content = rest
		}
	}

	prompt := strings.TrimSpace(content)
	if prompt == "" {
		return nil, fmt.Errorf("script %s has no prompt content", path)
	}

	return &ParsedScript{
		Prompt:      prompt,
		Config:      scriptCfg,
		Fingerprint: fingerprint,
		Path:        path,
		IsURL:       isURL,
	}, nil
}

// maxScriptSize is the maximum size of a remotely-fetched thought file (1 MB).
const maxScriptSize = 1 << 20

func fetchURL(url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching %s: HTTP %d", url, resp.StatusCode)
	}
	limited := io.LimitReader(resp.Body, maxScriptSize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("reading response from %s: %w", url, err)
	}
	if len(data) > maxScriptSize {
		return nil, fmt.Errorf("script from %s exceeds maximum size (%d bytes)", url, maxScriptSize)
	}
	return data, nil
}
