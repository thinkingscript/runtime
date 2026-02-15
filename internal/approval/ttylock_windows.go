//go:build windows

package approval

import "os"

func ttyID() string {
	return "default"
}

func acquirePromptLock() (*os.File, error) {
	return nil, nil
}

func releasePromptLock(f *os.File) {}

func openTTY() *os.File {
	return nil
}
