package collector

import (
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
)

func sample(at time.Time, cpu, sys uint64, rx uint64, mem uint64, inactive uint64, limit uint64) *container.StatsResponse {
	r := &container.StatsResponse{Read: at}
	r.CPUStats.CPUUsage.TotalUsage = cpu
	r.CPUStats.SystemUsage = sys
	r.CPUStats.OnlineCPUs = 4
	r.Networks = map[string]container.NetworkStats{"eth0": {RxBytes: rx, TxBytes: rx / 2}}
	r.MemoryStats.Usage = mem
	r.MemoryStats.Stats = map[string]uint64{"inactive_file": inactive}
	r.MemoryStats.Limit = limit
	return r
}

func TestCompute(t *testing.T) {
	c := &Collector{capacity: Capacity{CPUs: 4, Mem: 8 << 30}, prev: map[string]rawSample{}}
	t0 := time.Unix(1000, 0)

	st := ContainerStat{ID: "a", MemOfLimit: -1}
	c.compute(&st, sample(t0, 1e9, 100e9, 1000, 600<<20, 100<<20, 8<<30))
	if st.CPUPct != 0 {
		t.Fatalf("first sample must not report CPU, got %v", st.CPUPct)
	}
	if st.MemUsed != 500<<20 {
		t.Fatalf("mem must exclude inactive_file: %d", st.MemUsed)
	}
	if st.MemOfLimit != -1 {
		t.Fatalf("limit == capacity means unlimited, got %v", st.MemOfLimit)
	}

	// +2s: container used 2 cpu-seconds while the 4 cpu host used 8 cpu-seconds => 1 core.
	st = ContainerStat{ID: "a", MemOfLimit: -1}
	c.compute(&st, sample(t0.Add(2*time.Second), 3e9, 108e9, 3000, 300<<20, 0, 1<<30))
	if got := st.CPUPct; got < 99.9 || got > 100.1 {
		t.Fatalf("CPUPct = %v, want 100 (one core)", got)
	}
	if got := st.CPUShare; got < 24.9 || got > 25.1 {
		t.Fatalf("CPUShare = %v, want 25 (1 of 4 docker cpus)", got)
	}
	if st.NetRx != 1000 {
		t.Fatalf("NetRx = %v B/s, want 1000", st.NetRx)
	}
	if got := st.MemOfLimit; got < 29.2 || got > 29.4 {
		t.Fatalf("MemOfLimit = %v, want ~29.3", got)
	}

	// counter reset (container restarted): rates must not go negative/huge
	st = ContainerStat{ID: "a"}
	c.compute(&st, sample(t0.Add(3*time.Second), 1e8, 112e9, 10, 1, 0, 1<<30))
	if st.CPUPct != 0 || st.NetRx != 0 {
		t.Fatalf("reset must yield zero, got cpu=%v rx=%v", st.CPUPct, st.NetRx)
	}
}

func TestPushKeepsSnapshotsImmutable(t *testing.T) {
	var h []float64
	for i := 0; i < HistoryLen+10; i++ {
		h = push(h, float64(i))
	}
	if len(h) != HistoryLen || h[len(h)-1] != float64(HistoryLen+9) {
		t.Fatalf("bad ring: len=%d last=%v", len(h), h[len(h)-1])
	}
	old := h
	h = push(h, -1)
	if old[len(old)-1] == -1 {
		t.Fatal("push mutated a slice already handed out")
	}
}

func TestUsageLabel(t *testing.T) {
	if UsageDangling.Label("containers") != "stopped" || UsageActive.Label("images") != "active" {
		t.Fatal("labels changed")
	}
}
