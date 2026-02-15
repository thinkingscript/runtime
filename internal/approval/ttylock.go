//go:build !windows

package approval

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/thinkingscript/cli/internal/config"
)

// ttyID returns a unique identifier for the current controlling terminal,
// derived from the stderr file descriptor's device number.
func ttyID() string {
	var stat syscall.Stat_t
	if err := syscall.Fstat(int(os.Stderr.Fd()), &stat); err != nil {
		return "default"
	}
	return fmt.Sprintf("%d", stat.Rdev)
}

// acquirePromptLock acquires an exclusive file lock scoped to the current TTY.
// This ensures only one think process prompts at a time in a given terminal.
// Different terminals get different locks and never block each other.
func acquirePromptLock() (*os.File, error) {
	dir := filepath.Join(config.HomeDir(), "locks")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	path := filepath.Join(dir, fmt.Sprintf("prompt-%s.lock", ttyID()))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}

	return f, nil
}

// releasePromptLock releases the TTY prompt lock.
func releasePromptLock(f *os.File) {
	if f == nil {
		return
	}
	syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	f.Close()
}

// openTTY opens /dev/tty for direct keyboard input, bypassing stdin.
func openTTY() *os.File {
	f, err := os.Open("/dev/tty")
	if err != nil {
		return nil
	}
	return f
}
