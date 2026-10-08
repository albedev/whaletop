//go:build darwin

package host

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func detectVMDisk(o Options) (DiskCapacity, bool) {
	home := homeDir()
	if isDockerDesktop(o) {
		configured := dockerDesktopDiskSetting(home)
		for _, p := range []string{
			filepath.Join(home, "Library/Containers/com.docker.docker/Data/vms/0/data/Docker.raw"),
			filepath.Join(home, "Library/Containers/com.docker.docker/Data/vms/0/data/Docker.qcow2"),
		} {
			if c, ok := vmDisk(p, "Docker Desktop VM disk ("+filepath.Base(p)+")", configured); ok {
				return c, true
			}
		}
		if configured > 0 {
			return DiskCapacity{Known: true, Total: configured, Available: -1, Allocated: -1,
				Source: "Docker Desktop settings (diskSizeMiB)", VM: true}, true
		}
	}
	// colima: lima VM with a sparse "diffdisk".
	for _, p := range []string{
		filepath.Join(home, ".colima/_lima/colima/diffdisk"),
		filepath.Join(home, ".colima/_lima/_disks/colima/datadisk"),
	} {
		if c, ok := vmDisk(p, "colima VM disk", 0); ok {
			return c, true
		}
	}
	return DiskCapacity{}, false
}

// dockerDesktopDiskSetting reads the configured VM disk size. Recent versions
// store it in settings-store.json (DiskSizeMiB), older ones in settings.json (diskSizeMiB).
func dockerDesktopDiskSetting(home string) int64 {
	dir := filepath.Join(home, "Library/Group Containers/group.com.docker")
	for _, f := range []string{"settings-store.json", "settings.json"} {
		b, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			continue
		}
		var m map[string]any
		if json.Unmarshal(b, &m) != nil {
			continue
		}
		for _, k := range []string{"DiskSizeMiB", "diskSizeMiB"} {
			if v, ok := m[k].(float64); ok && v > 0 {
				return int64(v) << 20
			}
		}
	}
	return 0
}
