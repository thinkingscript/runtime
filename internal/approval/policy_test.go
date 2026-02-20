package approval

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewPolicy(t *testing.T) {
	p := NewPolicy()

	if p.Version != PolicyVersion {
		t.Errorf("expected version %d, got %d", PolicyVersion, p.Version)
	}
	if p.Paths.Default != ApprovalPrompt {
		t.Errorf("expected paths default to be prompt, got %s", p.Paths.Default)
	}
	if p.Env.Default != ApprovalPrompt {
		t.Errorf("expected env default to be prompt, got %s", p.Env.Default)
	}
	if p.Net.Hosts.Default != ApprovalPrompt {
		t.Errorf("expected net hosts default to be prompt, got %s", p.Net.Hosts.Default)
	}
	if p.Net.Listen.Default != ApprovalDeny {
		t.Errorf("expected net listen default to be deny, got %s", p.Net.Listen.Default)
	}
}

func TestPolicySaveLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.json")

	// Create and save policy
	p := NewPolicy()
	p.AddPathEntry("/home/user/projects", "rwd", ApprovalAllow, SourcePrompt)
	p.AddEnvEntry("HOME", ApprovalAllow, SourceConfig)
	p.AddHostEntry("*.github.com", ApprovalAllow, SourceCLI)

	if err := p.Save(path); err != nil {
		t.Fatalf("failed to save policy: %v", err)
	}

	// Load and verify
	loaded, err := LoadPolicy(path)
	if err != nil {
		t.Fatalf("failed to load policy: %v", err)
	}

	if len(loaded.Paths.Entries) != 1 {
		t.Errorf("expected 1 path entry, got %d", len(loaded.Paths.Entries))
	}
	if loaded.Paths.Entries[0].Path != "/home/user/projects" {
		t.Errorf("expected path /home/user/projects, got %s", loaded.Paths.Entries[0].Path)
	}
	if loaded.Paths.Entries[0].Mode != "rwd" {
		t.Errorf("expected mode rwd, got %s", loaded.Paths.Entries[0].Mode)
	}

	if len(loaded.Env.Entries) != 1 {
		t.Errorf("expected 1 env entry, got %d", len(loaded.Env.Entries))
	}
	if loaded.Env.Entries[0].Name != "HOME" {
		t.Errorf("expected env name HOME, got %s", loaded.Env.Entries[0].Name)
	}

	if len(loaded.Net.Hosts.Entries) != 1 {
		t.Errorf("expected 1 host entry, got %d", len(loaded.Net.Hosts.Entries))
	}
	if loaded.Net.Hosts.Entries[0].Host != "*.github.com" {
		t.Errorf("expected host *.github.com, got %s", loaded.Net.Hosts.Entries[0].Host)
	}
}

func TestLoadPolicyNonExistent(t *testing.T) {
	p, err := LoadPolicy("/nonexistent/path/policy.json")
	if err != nil {
		t.Fatalf("expected no error for non-existent file, got %v", err)
	}
	if p.Version != PolicyVersion {
		t.Errorf("expected default version, got %d", p.Version)
	}
}

func TestPathMatching(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"/home/user", "/home/user", true},
		{"/home/user", "/home/user/file.txt", true},
		{"/home/user", "/home/user/subdir/file.txt", true},
		{"/home/user", "/home/another", false},
		{"/home/user", "/home/username", false}, // not a prefix match
	}

	for _, tc := range tests {
		got := pathMatches(tc.pattern, tc.path)
		if got != tc.want {
			t.Errorf("pathMatches(%q, %q) = %v, want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}

func TestEnvMatching(t *testing.T) {
	tests := []struct {
		pattern string
		name    string
		want    bool
	}{
		{"HOME", "HOME", true},
		{"HOME", "PATH", false},
		{"AWS_*", "AWS_SECRET_KEY", true},
		{"AWS_*", "AWS_ACCESS_KEY", true},
		{"AWS_*", "HOME", false},
		{"AWS_*", "AWSSOMETHING", false}, // must have underscore
	}

	for _, tc := range tests {
		got := envMatches(tc.pattern, tc.name)
		if got != tc.want {
			t.Errorf("envMatches(%q, %q) = %v, want %v", tc.pattern, tc.name, got, tc.want)
		}
	}
}

func TestHostMatching(t *testing.T) {
	tests := []struct {
		pattern string
		host    string
		want    bool
	}{
		{"github.com", "github.com", true},
		{"github.com", "api.github.com", false},
		{"*.github.com", "api.github.com", true},
		{"*.github.com", "raw.githubusercontent.com", false},
		{"*.github.com", "github.com", false}, // wildcard requires subdomain
	}

	for _, tc := range tests {
		got := hostMatches(tc.pattern, tc.host)
		if got != tc.want {
			t.Errorf("hostMatches(%q, %q) = %v, want %v", tc.pattern, tc.host, got, tc.want)
		}
	}
}

func TestPathPolicyMatch(t *testing.T) {
	p := &PathPolicy{
		Entries: []PathEntry{
			{Path: "/home/user/projects", Mode: "rwd", Approval: ApprovalAllow},
			{Path: "/home/user/projects/secret", Mode: "r", Approval: ApprovalDeny},
			{Path: "/etc", Mode: "r", Approval: ApprovalAllow},
		},
	}

	tests := []struct {
		path     string
		wantPath string
	}{
		{"/home/user/projects/foo.txt", "/home/user/projects"},
		{"/home/user/projects/secret/key", "/home/user/projects/secret"}, // more specific match wins
		{"/etc/passwd", "/etc"},
		{"/var/log", ""},
	}

	for _, tc := range tests {
		entry := p.MatchPath(tc.path)
		if tc.wantPath == "" {
			if entry != nil {
				t.Errorf("MatchPath(%q) = %v, want nil", tc.path, entry.Path)
			}
		} else {
			if entry == nil {
				t.Errorf("MatchPath(%q) = nil, want %q", tc.path, tc.wantPath)
			} else if entry.Path != tc.wantPath {
				t.Errorf("MatchPath(%q) = %q, want %q", tc.path, entry.Path, tc.wantPath)
			}
		}
	}
}

func TestPathEntryModes(t *testing.T) {
	entry := PathEntry{Mode: "rwd"}

	if !entry.HasRead() {
		t.Error("expected HasRead() to be true")
	}
	if !entry.HasWrite() {
		t.Error("expected HasWrite() to be true")
	}
	if !entry.HasDelete() {
		t.Error("expected HasDelete() to be true")
	}

	entry2 := PathEntry{Mode: "r"}
	if !entry2.HasRead() {
		t.Error("expected HasRead() to be true")
	}
	if entry2.HasWrite() {
		t.Error("expected HasWrite() to be false")
	}
	if entry2.HasDelete() {
		t.Error("expected HasDelete() to be false")
	}
}

func TestPolicySaveCreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "policy.json")

	p := NewPolicy()
	if err := p.Save(path); err != nil {
		t.Fatalf("failed to save policy: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected policy file to be created")
	}
}
