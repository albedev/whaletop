//go:build linux

package host

import (
	"path/filepath"
)

func detectVMDisk(o Options) (DiskCapacity, bool) {
	if !isDockerDesktop(o) {
		return DiskCapacity{}, false
	}
	p := filepath.Join(homeDir(), ".docker/desktop/vms/0/data/Docker.raw")
	return vmDisk(p, "Docker Desktop VM disk (Docker.raw)", 0)
}
