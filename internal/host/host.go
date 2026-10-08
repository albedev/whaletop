//go:build unix

// Package host reads resources of the machine dtop runs on, and figures out how
// much disk the Docker daemon can use. Docker's API does not expose the size
// of its storage, so this is platform specific (see .knowledge/metrics.md).
package host

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	"golang.org/x/sys/unix"
)

// Resources of the physical host running dtop.
type Resources struct {
	CPUs int
	Mem  int64
}

// DiskCapacity describes the storage backing the Docker data root.
type DiskCapacity struct {
	Known     bool
	Total     int64  // max size of docker storage (VM disk size or filesystem size)
	Available int64  // bytes docker can still write (bounded by host free space too)
	Allocated int64  // bytes currently allocated on the backing storage (sparse VM disk or fs used)
	Source    string // human description of where the numbers come from
	VM        bool   // storage is a virtual disk of a VM (Docker Desktop, colima...)
}

// Options influence capacity detection.
type Options struct {
	Local       bool   // daemon reached through a local unix socket
	RootDir     string // DockerRootDir from /info
	OperatingOS string // OperatingSystem from /info ("Docker Desktop", "Ubuntu 24.04"...)
	Override    int64  // --disk-limit flag, bytes (0 = none)
}

// DetectDisk returns the best known capacity for the docker storage.
func DetectDisk(o Options) DiskCapacity {
	if o.Override > 0 {
		return DiskCapacity{Known: true, Total: o.Override, Available: -1, Allocated: -1, Source: "--disk-limit flag"}
	}
	if !o.Local {
		return DiskCapacity{Source: "remote daemon: capacity unknown (use --disk-limit)"}
	}
	if c, ok := detectVMDisk(o); ok {
		return c
	}
	// Daemon runs natively on this host: the data root is a real directory.
	if o.RootDir != "" {
		if c, ok := statfsCapacity(o.RootDir); ok {
			c.Source = "statfs " + o.RootDir
			return c
		}
	}
	return DiskCapacity{Source: "capacity unknown (use --disk-limit)"}
}

// Detect reads host CPU count and RAM (gopsutil).
func Detect() Resources {
	r := Resources{CPUs: runtime.NumCPU()}
	if vm, err := mem.VirtualMemory(); err == nil {
		r.Mem = int64(vm.Total)
	}
	return r
}

func statfsCapacity(path string) (DiskCapacity, bool) {
	u, err := disk.Usage(path)
	if err != nil {
		return DiskCapacity{}, false
	}
	return DiskCapacity{Known: true, Total: int64(u.Total), Available: int64(u.Free), Allocated: int64(u.Used)}, true
}

// vmDisk builds a capacity from a sparse virtual disk image: its logical size
// is the configured max, its allocated blocks are what the VM really used.
// gopsutil has no notion of sparse allocation, hence the raw stat(2).
func vmDisk(path, source string, configured int64) (DiskCapacity, bool) {
	var st unix.Stat_t
	if err := unix.Stat(path, &st); err != nil {
		return DiskCapacity{}, false
	}
	total := st.Size
	if configured > 0 {
		total = configured
	}
	alloc := st.Blocks * 512
	avail := total - alloc
	if hs, ok := statfsCapacity(filepath.Dir(path)); ok && hs.Available < avail {
		// The sparse image can't grow past the free space of the host disk.
		avail = hs.Available
	}
	if avail < 0 {
		avail = 0
	}
	return DiskCapacity{Known: true, Total: total, Available: avail, Allocated: alloc, Source: source, VM: true}, true
}

func homeDir() string {
	h, _ := os.UserHomeDir()
	return h
}

func isDockerDesktop(o Options) bool {
	return strings.Contains(strings.ToLower(o.OperatingOS), "docker desktop")
}
