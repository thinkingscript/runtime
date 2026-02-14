package sandbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thinkingscript/cli/internal/approval"
	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
)

// Config holds everything needed to create a sandbox.
type Config struct {
	AllowedPaths []string      // Resolved absolute paths the sandbox may access freely (CWD, workspace)
	WorkDir      string        // CWD for relative path resolution
	Stderr       io.Writer     // Where console.log goes
	Args         []string      // Script arguments
	Timeout      time.Duration // Max execution time (default 30s)
	ApprovePath func(op, path string) (bool, error) // Called for paths outside AllowedPaths; nil = deny all
	ApproveEnv  func(name string) (bool, error) // Called before reading env vars; nil = allow all
	OnWrite     func(path, content string)      // Called after successful file writes; nil = no-op
}

// Sandbox executes JavaScript code with restricted filesystem access.
type Sandbox struct {
	cfg          Config
	allowedPaths []string // resolved + cleaned allowed paths
	ctx          context.Context
	interrupted  bool // set when a user prompt is interrupted (Ctrl+C)
}

// New creates a Sandbox. AllowedPaths are resolved via EvalSymlinks at
// creation time so that runtime path checks can't be tricked by symlinks.
func New(cfg Config) (*Sandbox, error) {
	resolved := make([]string, 0, len(cfg.AllowedPaths))
	for _, p := range cfg.AllowedPaths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, fmt.Errorf("resolving allowed path %q: %w", p, err)
		}
		// Try to resolve symlinks; if the path doesn't exist yet that's OK —
		// use the absolute path as-is (e.g., workspace dir created on first use).
		real, err := filepath.EvalSymlinks(abs)
		if err != nil {
			real = abs
		}
		resolved = append(resolved, real)
	}

	if cfg.Stderr == nil {
		cfg.Stderr = os.Stderr
	}

	// Resolve WorkDir symlinks so it matches the resolved AllowedPaths.
	if cfg.WorkDir != "" {
		if real, err := filepath.EvalSymlinks(cfg.WorkDir); err == nil {
			cfg.WorkDir = real
		}
	}

	return &Sandbox{cfg: cfg, allowedPaths: resolved}, nil
}

// Run executes JavaScript code and returns the last expression value as a string.
func (s *Sandbox) Run(ctx context.Context, code string) (result string, err error) {
	s.ctx = ctx
	vm := goja.New()

	// Wire bridges
	s.registerConsole(vm)
	s.registerFS(vm)
	s.registerNet(vm)
	s.registerEnv(vm)
	s.registerProcess(vm)
	s.registerSys(vm)

	// Enable require() with sandbox-aware source loading
	registry := require.NewRegistry(
		require.WithLoader(func(path string) ([]byte, error) {
			resolved, err := s.resolvePath("read", path)
			if err != nil {
				return nil, require.ModuleFileDoesNotExistError
			}
			return os.ReadFile(resolved)
		}),
	)
	registry.Enable(vm)

	// Context cancellation via interrupt
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			vm.Interrupt("execution cancelled")
		case <-done:
		}
	}()

	// Timeout (only if configured)
	if s.cfg.Timeout > 0 {
		timer := time.AfterFunc(s.cfg.Timeout, func() {
			vm.Interrupt("execution timed out")
		})
		defer timer.Stop()
	}

	// Catch panics from goja (e.g., process.exit)
	defer func() {
		if r := recover(); r != nil {
			if exitErr, ok := r.(*exitError); ok {
				if exitErr.code == 0 {
					err = nil
				} else {
					err = fmt.Errorf("script exited with code %d", exitErr.code)
				}
			} else {
				err = fmt.Errorf("script panic: %v", r)
			}
		}
	}()

	v, runErr := vm.RunString(code)
	if s.interrupted {
		return "", approval.ErrInterrupted
	}
	if runErr != nil {
		// Extract just the error message, not Go internals
		if ex, ok := runErr.(*goja.Exception); ok {
			return "", fmt.Errorf("%s", ex.Value().String())
		}
		return "", fmt.Errorf("%s", runErr.Error())
	}

	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return "", nil
	}
	return stringify(vm, v), nil
}

// resolvePath takes a user-supplied path (possibly relative), resolves it
// against WorkDir, evaluates symlinks, and checks that the result falls
// within one of the allowed paths. The op parameter describes the operation
// (e.g. "read", "write", "delete") and is shown in approval prompts.
func (s *Sandbox) resolvePath(op, userPath string) (string, error) {
	var abs string
	if filepath.IsAbs(userPath) {
		abs = filepath.Clean(userPath)
	} else {
		abs = filepath.Join(s.cfg.WorkDir, userPath)
	}

	// For files that don't exist yet (write), resolve the parent and append the base.
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		// Path doesn't exist — resolve parent instead
		parent := filepath.Dir(abs)
		realParent, err2 := filepath.EvalSymlinks(parent)
		if err2 != nil {
			return "", fmt.Errorf("path not accessible: %s", userPath)
		}
		real = filepath.Join(realParent, filepath.Base(abs))
	}

	for _, allowed := range s.allowedPaths {
		if real == allowed || strings.HasPrefix(real, allowed+string(filepath.Separator)) {
			return real, nil
		}
	}

	// Path is outside the sandbox — ask for approval if a callback is set.
	if s.cfg.ApprovePath != nil {
		approved, err := s.cfg.ApprovePath(op, real)
		if err != nil {
			if errors.Is(err, approval.ErrInterrupted) {
				s.interrupted = true
			}
			return "", err
		}
		if approved {
			return real, nil
		}
	}

	return "", fmt.Errorf("access denied: path %q is outside the sandbox", userPath)
}

// checkInterrupted sets the interrupted flag if err is ErrInterrupted.
func (s *Sandbox) checkInterrupted(err error) {
	if errors.Is(err, approval.ErrInterrupted) {
		s.interrupted = true
	}
}

// throwError panics with a clean JS Error (no Go stack traces).
func throwError(vm *goja.Runtime, msg string) {
	panic(vm.ToValue(msg))
}

// exitError is used with panic to implement process.exit().
type exitError struct {
	code int
}
