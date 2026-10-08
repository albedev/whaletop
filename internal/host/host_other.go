//go:build unix && !darwin && !linux

package host

func detectVMDisk(Options) (DiskCapacity, bool) { return DiskCapacity{}, false }
