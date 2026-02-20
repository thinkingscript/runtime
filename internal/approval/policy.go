package approval

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Policy version for schema migrations.
const PolicyVersion = 1

// Approval represents an approval decision.
type Approval string

const (
	ApprovalAllow  Approval = "allow"
	ApprovalDeny   Approval = "deny"
	ApprovalPrompt Approval = "prompt"
)

// Source indicates how a policy entry was created.
type Source string

const (
	SourceDefault Source = "default" // auto-generated at first run
	SourcePrompt  Source = "prompt"  // user answered a prompt
	SourceConfig  Source = "config"  // manually edited
	SourceCLI     Source = "cli"     // added via thought policy command
)

// Policy represents the complete policy file.
type Policy struct {
	Version int        `json:"version"`
	Paths   PathPolicy `json:"paths"`
	Env     EnvPolicy  `json:"env"`
	Net     NetPolicy  `json:"net"`
}

// PathPolicy controls filesystem access.
type PathPolicy struct {
	Default   Approval    `json:"default"`
	Entries   []PathEntry `json:"entries"`
	Protected []PathEntry `json:"protected,omitempty"` // can't be overridden by thought policy
}

// PathEntry represents a single path permission.
type PathEntry struct {
	Path     string    `json:"path"`
	Mode     string    `json:"mode"` // combination of r, w, d
	Approval Approval  `json:"approval"`
	Source   Source    `json:"source,omitempty"`
	Created  time.Time `json:"created,omitempty"`
}

// HasRead returns true if mode includes read permission.
func (e PathEntry) HasRead() bool {
	return strings.Contains(e.Mode, "r")
}

// HasWrite returns true if mode includes write permission.
func (e PathEntry) HasWrite() bool {
	return strings.Contains(e.Mode, "w")
}

// HasDelete returns true if mode includes delete permission.
func (e PathEntry) HasDelete() bool {
	return strings.Contains(e.Mode, "d")
}

// EnvPolicy controls environment variable access.
type EnvPolicy struct {
	Default Approval   `json:"default"`
	Entries []EnvEntry `json:"entries"`
}

// EnvEntry represents a single env var permission.
type EnvEntry struct {
	Name     string    `json:"name"` // supports wildcards like AWS_*
	Approval Approval  `json:"approval"`
	Source   Source    `json:"source,omitempty"`
	Created  time.Time `json:"created,omitempty"`
}

// NetPolicy controls network access.
type NetPolicy struct {
	Hosts  HostPolicy   `json:"hosts"`
	Listen ListenPolicy `json:"listen"`
}

// HostPolicy controls outbound connections.
type HostPolicy struct {
	Default Approval    `json:"default"`
	Entries []HostEntry `json:"entries"`
}

// HostEntry represents a single host permission.
type HostEntry struct {
	Host     string    `json:"host"` // supports wildcards like *.github.com
	Approval Approval  `json:"approval"`
	Source   Source    `json:"source,omitempty"`
	Created  time.Time `json:"created,omitempty"`
}

// ListenPolicy controls inbound connections (port binding).
type ListenPolicy struct {
	Default Approval      `json:"default"`
	Entries []ListenEntry `json:"entries"`
}

// ListenEntry represents a single port permission.
type ListenEntry struct {
	Port     string    `json:"port"` // single port or range like "3000-3999"
	Approval Approval  `json:"approval"`
	Source   Source    `json:"source,omitempty"`
	Created  time.Time `json:"created,omitempty"`
}

// NewPolicy creates an empty policy with defaults.
func NewPolicy() *Policy {
	return &Policy{
		Version: PolicyVersion,
		Paths: PathPolicy{
			Default: ApprovalPrompt,
			Entries: []PathEntry{},
		},
		Env: EnvPolicy{
			Default: ApprovalPrompt,
			Entries: []EnvEntry{},
		},
		Net: NetPolicy{
			Hosts: HostPolicy{
				Default: ApprovalPrompt,
				Entries: []HostEntry{},
			},
			Listen: ListenPolicy{
				Default: ApprovalDeny,
				Entries: []ListenEntry{},
			},
		},
	}
}

// LoadPolicy reads a policy from a JSON file.
func LoadPolicy(path string) (*Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewPolicy(), nil
		}
		return nil, err
	}

	var policy Policy
	if err := json.Unmarshal(data, &policy); err != nil {
		return nil, err
	}

	// Ensure slices are initialized
	if policy.Paths.Entries == nil {
		policy.Paths.Entries = []PathEntry{}
	}
	if policy.Env.Entries == nil {
		policy.Env.Entries = []EnvEntry{}
	}
	if policy.Net.Hosts.Entries == nil {
		policy.Net.Hosts.Entries = []HostEntry{}
	}
	if policy.Net.Listen.Entries == nil {
		policy.Net.Listen.Entries = []ListenEntry{}
	}

	return &policy, nil
}

// Save writes the policy to a JSON file.
func (p *Policy) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// MatchPath finds the best matching path entry for the given path.
// Returns nil if no entry matches.
func (p *PathPolicy) MatchPath(targetPath string) *PathEntry {
	var bestMatch *PathEntry
	var bestLen int

	for i := range p.Entries {
		entry := &p.Entries[i]
		if pathMatches(entry.Path, targetPath) {
			// Prefer more specific (longer) matches
			if len(entry.Path) > bestLen {
				bestMatch = entry
				bestLen = len(entry.Path)
			}
		}
	}

	return bestMatch
}

// pathMatches checks if a pattern matches a path.
// Supports exact matches and prefix matches (directory contains file).
func pathMatches(pattern, path string) bool {
	// Exact match
	if pattern == path {
		return true
	}

	// Pattern is a directory that contains the path
	if strings.HasPrefix(path, pattern+string(filepath.Separator)) {
		return true
	}

	// TODO: support glob patterns like /Users/*/projects

	return false
}

// MatchEnv finds the best matching env entry for the given variable name.
// Returns nil if no entry matches.
func (p *EnvPolicy) MatchEnv(name string) *EnvEntry {
	for i := range p.Entries {
		entry := &p.Entries[i]
		if envMatches(entry.Name, name) {
			return entry
		}
	}
	return nil
}

// envMatches checks if a pattern matches an env var name.
// Supports exact matches and wildcards like AWS_*.
func envMatches(pattern, name string) bool {
	if pattern == name {
		return true
	}

	// Wildcard suffix: AWS_* matches AWS_SECRET_KEY
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(name, prefix)
	}

	return false
}

// MatchHost finds the best matching host entry for the given hostname.
// Returns nil if no entry matches.
func (p *HostPolicy) MatchHost(host string) *HostEntry {
	for i := range p.Entries {
		entry := &p.Entries[i]
		if hostMatches(entry.Host, host) {
			return entry
		}
	}
	return nil
}

// hostMatches checks if a pattern matches a hostname.
// Supports exact matches and wildcards like *.github.com.
func hostMatches(pattern, host string) bool {
	if pattern == host {
		return true
	}

	// Wildcard prefix: *.github.com matches api.github.com
	if strings.HasPrefix(pattern, "*.") {
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(host, suffix)
	}

	return false
}

// AddPathEntry adds a new path entry to the policy.
func (p *Policy) AddPathEntry(path, mode string, approval Approval, source Source) {
	p.Paths.Entries = append(p.Paths.Entries, PathEntry{
		Path:     path,
		Mode:     mode,
		Approval: approval,
		Source:   source,
		Created:  time.Now(),
	})
}

// AddEnvEntry adds a new env entry to the policy.
func (p *Policy) AddEnvEntry(name string, approval Approval, source Source) {
	p.Env.Entries = append(p.Env.Entries, EnvEntry{
		Name:     name,
		Approval: approval,
		Source:   source,
		Created:  time.Now(),
	})
}

// AddHostEntry adds a new host entry to the policy.
func (p *Policy) AddHostEntry(host string, approval Approval, source Source) {
	p.Net.Hosts.Entries = append(p.Net.Hosts.Entries, HostEntry{
		Host:     host,
		Approval: approval,
		Source:   source,
		Created:  time.Now(),
	})
}
