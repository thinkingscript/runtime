package approval

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpToModeChar(t *testing.T) {
	tests := []struct {
		op   string
		want string
	}{
		{"read", "r"},
		{"list", "r"},
		{"write", "w"},
		{"delete", "d"},
		{"unknown", "r"},
	}

	for _, tc := range tests {
		got := opToModeChar(tc.op)
		if got != tc.want {
			t.Errorf("opToModeChar(%q) = %q, want %q", tc.op, got, tc.want)
		}
	}
}

func TestHasMode(t *testing.T) {
	tests := []struct {
		mode string
		char string
		want bool
	}{
		{"rwd", "r", true},
		{"rwd", "w", true},
		{"rwd", "d", true},
		{"r", "w", false},
		{"rw", "d", false},
		{"", "r", false},
	}

	for _, tc := range tests {
		got := hasMode(tc.mode, tc.char)
		if got != tc.want {
			t.Errorf("hasMode(%q, %q) = %v, want %v", tc.mode, tc.char, got, tc.want)
		}
	}
}

func TestBootstrapDefaults(t *testing.T) {
	dir := t.TempDir()
	thoughtDir := filepath.Join(dir, "thought")
	libDir := filepath.Join(dir, "lib")
	tmpDir := filepath.Join(dir, "tmp")
	memoriesDir := filepath.Join(dir, "memories")
	workDir := filepath.Join(dir, "cwd")

	os.MkdirAll(thoughtDir, 0700)
	os.MkdirAll(libDir, 0700)
	os.MkdirAll(tmpDir, 0700)
	os.MkdirAll(memoriesDir, 0700)
	os.MkdirAll(workDir, 0700)

	// Create approver without any existing policy
	approver := NewApprover(thoughtDir, "")
	defer approver.Close()

	// Bootstrap defaults
	approver.BootstrapDefaults(libDir, tmpDir, memoriesDir, workDir)

	// Verify policy was created
	policyPath := filepath.Join(thoughtDir, "policy.json")
	if _, err := os.Stat(policyPath); os.IsNotExist(err) {
		t.Fatal("expected policy file to be created")
	}

	// Load and verify entries
	policy, err := LoadPolicy(policyPath)
	if err != nil {
		t.Fatalf("failed to load policy: %v", err)
	}

	// Should have 5 entries: lib, tmp, memories, cwd, policy.json (denied)
	if len(policy.Paths.Entries) != 5 {
		t.Errorf("expected 5 path entries, got %d", len(policy.Paths.Entries))
	}

	// Check lib entry
	found := false
	for _, entry := range policy.Paths.Entries {
		if entry.Path == libDir {
			found = true
			if entry.Mode != "rwd" {
				t.Errorf("lib mode = %q, want rwd", entry.Mode)
			}
			if entry.Approval != ApprovalAllow {
				t.Errorf("lib approval = %q, want allow", entry.Approval)
			}
			if entry.Source != SourceDefault {
				t.Errorf("lib source = %q, want default", entry.Source)
			}
		}
	}
	if !found {
		t.Error("lib entry not found")
	}

	// Check CWD entry (should be read-only)
	found = false
	for _, entry := range policy.Paths.Entries {
		if entry.Path == workDir {
			found = true
			if entry.Mode != "r" {
				t.Errorf("workDir mode = %q, want r", entry.Mode)
			}
		}
	}
	if !found {
		t.Error("workDir entry not found")
	}

	// Check policy.json entry (should be denied)
	found = false
	for _, entry := range policy.Paths.Entries {
		if entry.Path == policyPath {
			found = true
			if entry.Approval != ApprovalDeny {
				t.Errorf("policy.json approval = %q, want deny", entry.Approval)
			}
		}
	}
	if !found {
		t.Error("policy.json deny entry not found")
	}
}

func TestBootstrapDefaultsSkipsIfExists(t *testing.T) {
	dir := t.TempDir()
	thoughtDir := filepath.Join(dir, "thought")
	libDir := filepath.Join(dir, "lib")
	tmpDir := filepath.Join(dir, "tmp")
	memoriesDir := filepath.Join(dir, "memories")
	workDir := filepath.Join(dir, "cwd")

	os.MkdirAll(thoughtDir, 0700)

	// Create approver and bootstrap once
	approver := NewApprover(thoughtDir, "")
	approver.BootstrapDefaults(libDir, tmpDir, memoriesDir, workDir)
	approver.Close()

	// Add a custom entry
	policy, _ := LoadPolicy(filepath.Join(thoughtDir, "policy.json"))
	policy.AddPathEntry("/custom/path", "rw", ApprovalAllow, SourceConfig)
	policy.Save(filepath.Join(thoughtDir, "policy.json"))

	// Create new approver and try to bootstrap again
	approver2 := NewApprover(thoughtDir, "")
	defer approver2.Close()
	approver2.BootstrapDefaults("/other/lib", "/other/tmp", "/other/memories", "/other/cwd")

	// Verify custom entry is still there (bootstrap was skipped)
	policy2, _ := LoadPolicy(filepath.Join(thoughtDir, "policy.json"))
	found := false
	for _, entry := range policy2.Paths.Entries {
		if entry.Path == "/custom/path" {
			found = true
		}
	}
	if !found {
		t.Error("custom entry was lost - bootstrap should have been skipped")
	}
}

func TestPolicyFileProtection(t *testing.T) {
	dir := t.TempDir()
	thoughtDir := filepath.Join(dir, "thought")
	os.MkdirAll(thoughtDir, 0700)

	approver := NewApprover(thoughtDir, "")
	defer approver.Close()

	// Try to approve writing to the policy file itself
	policyPath := filepath.Join(thoughtDir, "policy.json")
	approved, err := approver.ApprovePath("write", policyPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if approved {
		t.Error("expected policy file write to be denied")
	}

	// Try to approve deleting the policy file
	approved, err = approver.ApprovePath("delete", policyPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if approved {
		t.Error("expected policy file delete to be denied")
	}
}

func TestApprovePathWithPolicy(t *testing.T) {
	dir := t.TempDir()
	thoughtDir := filepath.Join(dir, "thought")
	os.MkdirAll(thoughtDir, 0700)

	// Create policy with some entries
	policy := NewPolicy()
	policy.AddPathEntry("/allowed/read", "r", ApprovalAllow, SourceConfig)
	policy.AddPathEntry("/allowed/write", "w", ApprovalAllow, SourceConfig)
	policy.AddPathEntry("/denied/path", "rwd", ApprovalDeny, SourceConfig)
	policy.Save(filepath.Join(thoughtDir, "policy.json"))

	approver := NewApprover(thoughtDir, "")
	defer approver.Close()

	// Test allowed read
	approved, _ := approver.ApprovePath("read", "/allowed/read/file.txt")
	if !approved {
		t.Error("expected read to be approved")
	}

	// Test allowed write
	approved, _ = approver.ApprovePath("write", "/allowed/write/file.txt")
	if !approved {
		t.Error("expected write to be approved")
	}

	// Test write on read-only path (should not be approved without TTY)
	approved, _ = approver.ApprovePath("write", "/allowed/read/file.txt")
	if approved {
		t.Error("expected write to read-only path to be denied")
	}

	// Test denied path
	approved, _ = approver.ApprovePath("read", "/denied/path/file.txt")
	if approved {
		t.Error("expected read on denied path to be denied")
	}
}

func TestApproveEnvWithPolicy(t *testing.T) {
	dir := t.TempDir()
	thoughtDir := filepath.Join(dir, "thought")
	os.MkdirAll(thoughtDir, 0700)

	// Create policy with env entries
	policy := NewPolicy()
	policy.AddEnvEntry("HOME", ApprovalAllow, SourceConfig)
	policy.AddEnvEntry("AWS_*", ApprovalDeny, SourceConfig)
	policy.Save(filepath.Join(thoughtDir, "policy.json"))

	approver := NewApprover(thoughtDir, "")
	defer approver.Close()

	// Test allowed env
	approved, _ := approver.ApproveEnvRead("HOME")
	if !approved {
		t.Error("expected HOME to be approved")
	}

	// Test denied env (wildcard)
	approved, _ = approver.ApproveEnvRead("AWS_SECRET_KEY")
	if approved {
		t.Error("expected AWS_SECRET_KEY to be denied")
	}
}

func TestApproveNetWithPolicy(t *testing.T) {
	dir := t.TempDir()
	thoughtDir := filepath.Join(dir, "thought")
	os.MkdirAll(thoughtDir, 0700)

	// Create policy with host entries
	policy := NewPolicy()
	policy.AddHostEntry("*.github.com", ApprovalAllow, SourceConfig)
	policy.AddHostEntry("evil.com", ApprovalDeny, SourceConfig)
	policy.Save(filepath.Join(thoughtDir, "policy.json"))

	approver := NewApprover(thoughtDir, "")
	defer approver.Close()

	// Test allowed host (wildcard)
	approved, _ := approver.ApproveNet("api.github.com")
	if !approved {
		t.Error("expected api.github.com to be approved")
	}

	// Test denied host
	approved, _ = approver.ApproveNet("evil.com")
	if approved {
		t.Error("expected evil.com to be denied")
	}
}

func TestGlobalPolicyProtected(t *testing.T) {
	dir := t.TempDir()
	thoughtDir := filepath.Join(dir, "thought")
	globalPolicyPath := filepath.Join(dir, "global_policy.json")
	os.MkdirAll(thoughtDir, 0700)

	// Create global policy with protected entry
	globalPolicy := NewPolicy()
	globalPolicy.Paths.Protected = []PathEntry{
		{Path: "/etc/shadow", Mode: "rwd", Approval: ApprovalDeny},
	}
	globalPolicy.Save(globalPolicyPath)

	// Create thought policy that tries to override
	thoughtPolicy := NewPolicy()
	thoughtPolicy.AddPathEntry("/etc/shadow", "r", ApprovalAllow, SourceConfig)
	thoughtPolicy.Save(filepath.Join(thoughtDir, "policy.json"))

	approver := NewApprover(thoughtDir, globalPolicyPath)
	defer approver.Close()

	// Protected deny should override thought policy
	approved, _ := approver.ApprovePath("read", "/etc/shadow")
	if approved {
		t.Error("expected /etc/shadow to be denied (protected)")
	}
}
