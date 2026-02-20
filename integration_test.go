package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thinkingscript/cli/internal/boot"
	"github.com/thinkingscript/cli/internal/config"
	"github.com/thinkingscript/cli/internal/sandbox"
	"github.com/thinkingscript/cli/internal/script"
)

// Integration tests for the full think execution flow

func TestScriptParsingEndToEnd(t *testing.T) {
	// Parse actual example files
	examples := []string{
		"examples/hello.md",
		"examples/weather.md",
	}

	for _, path := range examples {
		t.Run(path, func(t *testing.T) {
			parsed, err := script.Parse(path)
			if err != nil {
				t.Fatalf("failed to parse %s: %v", path, err)
			}

			if parsed.Prompt == "" {
				t.Errorf("prompt should not be empty")
			}
			if parsed.Fingerprint == "" {
				t.Errorf("fingerprint should not be empty")
			}
		})
	}
}

func TestBootFlowMemoryJSSuccess(t *testing.T) {
	// Set up a temp directory for thought data
	tmpHome := t.TempDir()
	t.Setenv("THINKINGSCRIPT_HOME", tmpHome)

	// Create thought directory structure
	thoughtDir := filepath.Join(tmpHome, "thoughts", "test")
	workspaceDir := filepath.Join(thoughtDir, "workspace")
	memoriesDir := filepath.Join(thoughtDir, "memories")
	memoryJSPath := filepath.Join(thoughtDir, "memory.js")

	os.MkdirAll(workspaceDir, 0700)
	os.MkdirAll(memoriesDir, 0700)

	// Create a memory.js that succeeds
	memoryJS := `
		// This memory.js just outputs "hello world" without needing the agent
		process.stdout.write("hello world");
		"done"
	`
	os.WriteFile(memoryJSPath, []byte(memoryJS), 0644)

	// Run boot
	result := boot.TryMemoryJS(context.Background(), boot.Config{
		MemoryJSPath: memoryJSPath,
		WorkDir:      thoughtDir,
		ThoughtDir:   thoughtDir,
		WorkspaceDir: workspaceDir,
		MemoriesDir:  memoriesDir,
	})

	if !result.Success {
		t.Errorf("expected success, got ResumeContext=%q", result.ResumeContext)
	}
	if result.Output != "done" {
		t.Errorf("output = %q, want %q", result.Output, "done")
	}
}

func TestBootFlowAgentResume(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("THINKINGSCRIPT_HOME", tmpHome)

	thoughtDir := filepath.Join(tmpHome, "thoughts", "test")
	workspaceDir := filepath.Join(thoughtDir, "workspace")
	memoriesDir := filepath.Join(thoughtDir, "memories")
	memoryJSPath := filepath.Join(thoughtDir, "memory.js")

	os.MkdirAll(workspaceDir, 0700)
	os.MkdirAll(memoriesDir, 0700)

	// Create a memory.js that calls agent.resume()
	memoryJS := `
		// Check arguments
		if (process.args.length === 0) {
			agent.resume("no arguments provided, need help");
		}
		process.stdout.write("Got " + process.args.length + " args");
		"done"
	`
	os.WriteFile(memoryJSPath, []byte(memoryJS), 0644)

	// Run without arguments - should trigger resume
	result := boot.TryMemoryJS(context.Background(), boot.Config{
		MemoryJSPath: memoryJSPath,
		WorkDir:      thoughtDir,
		ThoughtDir:   thoughtDir,
		WorkspaceDir: workspaceDir,
		MemoriesDir:  memoriesDir,
		Args:         []string{},
	})

	if result.Success {
		t.Error("expected failure (resume), got success")
	}
	if result.ResumeContext != "no arguments provided, need help" {
		t.Errorf("ResumeContext = %q, want 'no arguments provided, need help'", result.ResumeContext)
	}

	// Run with arguments - should succeed
	result2 := boot.TryMemoryJS(context.Background(), boot.Config{
		MemoryJSPath: memoryJSPath,
		WorkDir:      thoughtDir,
		ThoughtDir:   thoughtDir,
		WorkspaceDir: workspaceDir,
		MemoriesDir:  memoriesDir,
		Args:         []string{"NYC"},
	})

	if !result2.Success {
		t.Errorf("expected success with args, got ResumeContext=%q", result2.ResumeContext)
	}
}

func TestBootFlowMemoryJSError(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("THINKINGSCRIPT_HOME", tmpHome)

	thoughtDir := filepath.Join(tmpHome, "thoughts", "test")
	workspaceDir := filepath.Join(thoughtDir, "workspace")
	memoriesDir := filepath.Join(thoughtDir, "memories")
	memoryJSPath := filepath.Join(thoughtDir, "memory.js")

	os.MkdirAll(workspaceDir, 0700)
	os.MkdirAll(memoriesDir, 0700)

	// Create a memory.js that has an error
	memoryJS := `
		throw new Error("something is broken");
	`
	os.WriteFile(memoryJSPath, []byte(memoryJS), 0644)

	result := boot.TryMemoryJS(context.Background(), boot.Config{
		MemoryJSPath: memoryJSPath,
		WorkDir:      thoughtDir,
		ThoughtDir:   thoughtDir,
		WorkspaceDir: workspaceDir,
		MemoriesDir:  memoriesDir,
	})

	if result.Success {
		t.Error("expected failure, got success")
	}
	if !strings.Contains(result.ResumeContext, "memory.js error:") {
		t.Errorf("ResumeContext should mention error: %q", result.ResumeContext)
	}
	if !strings.Contains(result.ResumeContext, "something is broken") {
		t.Errorf("ResumeContext should contain error message: %q", result.ResumeContext)
	}
}

func TestSandboxWithRealFileSystem(t *testing.T) {
	tmpHome := t.TempDir()
	tmpHome, _ = filepath.EvalSymlinks(tmpHome)
	t.Setenv("THINKINGSCRIPT_HOME", tmpHome)

	workDir := filepath.Join(tmpHome, "work")
	workspaceDir := filepath.Join(tmpHome, "workspace")
	os.MkdirAll(workDir, 0700)
	os.MkdirAll(workspaceDir, 0700)

	// Create a test file
	testFile := filepath.Join(workDir, "data.txt")
	os.WriteFile(testFile, []byte("test data"), 0644)

	sb, err := sandbox.New(sandbox.Config{
		AllowedPaths:  []string{workDir, workspaceDir},
		WritablePaths: []string{workspaceDir},
		WorkDir:       workDir,
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	// Test reading
	result, err := sb.Run(context.Background(), `fs.readFile("`+testFile+`")`)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if result != "test data" {
		t.Errorf("result = %q, want %q", result, "test data")
	}

	// Test writing to workspace
	wsFile := filepath.Join(workspaceDir, "output.txt")
	_, err = sb.Run(context.Background(), `fs.writeFile("`+wsFile+`", "output data")`)
	if err != nil {
		t.Fatalf("write error: %v", err)
	}

	content, _ := os.ReadFile(wsFile)
	if string(content) != "output data" {
		t.Errorf("file content = %q, want %q", string(content), "output data")
	}
}

func TestMemoryJSWithWorkspaceModule(t *testing.T) {
	tmpHome := t.TempDir()
	tmpHome, _ = filepath.EvalSymlinks(tmpHome)
	t.Setenv("THINKINGSCRIPT_HOME", tmpHome)

	thoughtDir := filepath.Join(tmpHome, "thoughts", "test")
	workspaceDir := filepath.Join(thoughtDir, "workspace")
	memoriesDir := filepath.Join(thoughtDir, "memories")
	memoryJSPath := filepath.Join(thoughtDir, "memory.js")

	os.MkdirAll(workspaceDir, 0700)
	os.MkdirAll(memoriesDir, 0700)

	// Create a helper module in workspace
	helperPath := filepath.Join(workspaceDir, "helper.js")
	helper := `
		module.exports = {
			greet: function(name) {
				return "Hello, " + name + "!";
			}
		};
	`
	os.WriteFile(helperPath, []byte(helper), 0644)

	// Create memory.js that uses the helper
	memoryJS := `
		var helper = require("` + helperPath + `");
		helper.greet("World")
	`
	os.WriteFile(memoryJSPath, []byte(memoryJS), 0644)

	result := boot.TryMemoryJS(context.Background(), boot.Config{
		MemoryJSPath: memoryJSPath,
		WorkDir:      thoughtDir,
		ThoughtDir:   thoughtDir,
		WorkspaceDir: workspaceDir,
		MemoriesDir:  memoriesDir,
	})

	if !result.Success {
		t.Errorf("expected success, got ResumeContext=%q", result.ResumeContext)
	}
	if result.Output != "Hello, World!" {
		t.Errorf("output = %q, want %q", result.Output, "Hello, World!")
	}
}

func TestConfigPathsConsistency(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("THINKINGSCRIPT_HOME", tmpHome)

	scriptPath := "examples/weather.md"

	// All these paths should be under the same thought directory
	thoughtDir := config.ThoughtDir(scriptPath)
	workspaceDir := config.WorkspaceDir(scriptPath)
	memoriesDir := config.MemoriesDir(scriptPath)
	memoryJSPath := config.MemoryJSPath(scriptPath)

	if !strings.HasPrefix(workspaceDir, thoughtDir) {
		t.Errorf("workspaceDir should be under thoughtDir")
	}
	if !strings.HasPrefix(memoriesDir, thoughtDir) {
		t.Errorf("memoriesDir should be under thoughtDir")
	}
	if !strings.HasPrefix(memoryJSPath, thoughtDir) {
		t.Errorf("memoryJSPath should be under thoughtDir")
	}
}
