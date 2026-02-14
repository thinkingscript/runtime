//go:build darwin

package sandbox

import (
	"encoding/binary"
	"fmt"
	"time"

	"golang.org/x/sys/unix"
)

func totalMemory() (uint64, error) {
	return unix.SysctlUint64("hw.memsize")
}

func freeMemory() (uint64, error) {
	pageSize, err := unix.SysctlUint32("hw.pagesize")
	if err != nil {
		return 0, err
	}
	free, err := unix.SysctlUint32("vm.page_free_count")
	if err != nil {
		return 0, err
	}
	return uint64(free) * uint64(pageSize), nil
}

func systemUptime() (int64, error) {
	tv, err := unix.SysctlTimeval("kern.boottime")
	if err != nil {
		return 0, err
	}
	boot := time.Unix(tv.Sec, int64(tv.Usec)*1000)
	return int64(time.Since(boot).Seconds()), nil
}

func systemLoadavg() ([]float64, error) {
	raw, err := unix.SysctlRaw("vm.loadavg")
	if err != nil {
		return nil, err
	}
	// struct loadavg { fixpt_t ldavg[3]; long fscale; }
	// On 64-bit darwin: fixpt_t=uint32(4), padding(4), long=int64(8) → 24 bytes
	if len(raw) < 24 {
		return nil, fmt.Errorf("unexpected loadavg data (got %d bytes)", len(raw))
	}
	l0 := binary.LittleEndian.Uint32(raw[0:4])
	l1 := binary.LittleEndian.Uint32(raw[4:8])
	l2 := binary.LittleEndian.Uint32(raw[8:12])
	// 4 bytes padding at [12:16] for alignment
	fscale := float64(binary.LittleEndian.Uint64(raw[16:24]))
	if fscale == 0 {
		fscale = 1
	}
	return []float64{
		float64(l0) / fscale,
		float64(l1) / fscale,
		float64(l2) / fscale,
	}, nil
}
