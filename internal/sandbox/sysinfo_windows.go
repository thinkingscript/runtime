//go:build windows

package sandbox

import "fmt"

func totalMemory() (uint64, error) {
	return 0, fmt.Errorf("not supported on windows")
}

func freeMemory() (uint64, error) {
	return 0, fmt.Errorf("not supported on windows")
}

func systemUptime() (int64, error) {
	return 0, fmt.Errorf("not supported on windows")
}

func systemLoadavg() ([]float64, error) {
	return nil, fmt.Errorf("not supported on windows")
}
