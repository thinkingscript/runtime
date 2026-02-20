// Package boot handles the memory.js execution flow.
// It tries to run memory.js first, and returns whether the agent should take over.
package boot

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/thinkingscript/cli/internal/sandbox"
)

// Result represents the outcome of trying to run memory.js.
type Result struct {
	// Success is true if memory.js ran without errors.
	Success bool
	// Output is the result from memory.js (if successful).
	Output string
	// ResumeContext is the context string for the agent (if resuming).
	ResumeContext string
}

// Config holds the configuration for running memory.js.
type Config struct {
	MemoryJSPath string
	WorkDir      string
	ThoughtDir   string // readable but NOT writable (protects policy.json)
	WorkspaceDir string
	MemoriesDir  string
	Args         []string
	ApprovePath  func(op, path string) (bool, error)
	ApproveEnv   func(name string) (bool, error)
	ApproveNet   func(host string) (bool, error)
}

// TryMemoryJS attempts to run memory.js if it exists.
// Returns a Result indicating whether execution succeeded or the agent should resume.
func TryMemoryJS(ctx context.Context, cfg Config) Result {
	// Check if memory.js exists
	if _, err := os.Stat(cfg.MemoryJSPath); os.IsNotExist(err) {
		return Result{
			Success:       false,
			ResumeContext: "no memory.js exists, first run",
		}
	}

	// Read memory.js
	code, err := os.ReadFile(cfg.MemoryJSPath)
	if err != nil {
		return Result{
			Success:       false,
			ResumeContext: fmt.Sprintf("failed to read memory.js: %s", err),
		}
	}

	// Create sandbox
	// SECURITY: ThoughtDir is readable but NOT writable (protects policy.json)
	// Only memory.js, workspace, and memories are writable
	sb, err := sandbox.New(sandbox.Config{
		AllowedPaths:  []string{cfg.WorkDir, cfg.ThoughtDir, cfg.WorkspaceDir, cfg.MemoriesDir},
		WritablePaths: []string{cfg.WorkspaceDir, cfg.MemoriesDir, cfg.MemoryJSPath},
		WorkDir:       cfg.WorkDir,
		Stderr:        os.Stderr,
		Args:          cfg.Args,
		ApprovePath:   cfg.ApprovePath,
		ApproveEnv:    cfg.ApproveEnv,
		ApproveNet:    cfg.ApproveNet,
	})
	if err != nil {
		return Result{
			Success:       false,
			ResumeContext: fmt.Sprintf("failed to create sandbox: %s", err),
		}
	}

	// Run memory.js
	result, err := sb.Run(ctx, string(code))
	if err == nil {
		// Success! memory.js handled everything
		return Result{
			Success: true,
			Output:  result,
		}
	}

	// Check if it's a resume request or an error
	var resumeErr *sandbox.ResumeError
	if errors.As(err, &resumeErr) {
		return Result{
			Success:       false,
			ResumeContext: resumeErr.Context,
		}
	}

	// Runtime error
	return Result{
		Success:       false,
		ResumeContext: fmt.Sprintf("memory.js error: %s", err),
	}
}
