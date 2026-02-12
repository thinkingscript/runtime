package script

import (
	"fmt"
	"os"
	"strings"

	"github.com/bradgessler/agent-exec/internal/config"
	"gopkg.in/yaml.v3"
)

type ParsedScript struct {
	Prompt      string
	Config      *config.ScriptConfig
	Fingerprint string
	Path        string
}

func Parse(path string) (*ParsedScript, error) {
	data, err := os.ReadFile(path)
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
	}, nil
}
