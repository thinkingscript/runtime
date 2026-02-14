//go:build linux

package sandbox

import (
	"golang.org/x/sys/unix"
)

func totalMemory() (uint64, error) {
	var info unix.Sysinfo_t
	if err := unix.Sysinfo(&info); err != nil {
		return 0, err
	}
	return info.Totalram * uint64(info.Unit), nil
}

func freeMemory() (uint64, error) {
	var info unix.Sysinfo_t
	if err := unix.Sysinfo(&info); err != nil {
		return 0, err
	}
	return info.Freeram * uint64(info.Unit), nil
}

func systemUptime() (int64, error) {
	var info unix.Sysinfo_t
	if err := unix.Sysinfo(&info); err != nil {
		return 0, err
	}
	return info.Uptime, nil
}

func systemLoadavg() ([]float64, error) {
	var info unix.Sysinfo_t
	if err := unix.Sysinfo(&info); err != nil {
		return nil, err
	}
	scale := float64(1 << 16)
	return []float64{
		float64(info.Loads[0]) / scale,
		float64(info.Loads[1]) / scale,
		float64(info.Loads[2]) / scale,
	}, nil
}
