// Package collector turns raw Docker API data into the metrics dtop displays.
// It is UI agnostic: the UI calls Sample()/Disk() from tea.Cmds and renders the
// returned snapshots. Formulas are documented in .knowledge/metrics.md.
package collector

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"

	"github.com/albedev/dtop/internal/docker"
	"github.com/albedev/dtop/internal/host"
)

// HistoryLen is the number of samples kept for graphs (enough for a wide terminal).
const HistoryLen = 300

// Capacity is what docker can use at most.
type Capacity struct {
	CPUs     int   // NCPU reported by the daemon (VM cpus on Docker Desktop)
	Mem      int64 // MemTotal reported by the daemon
	HostCPUs int   // physical host running dtop
	HostMem  int64
}

// ContainerStat is one row of the resources view.
type ContainerStat struct {
	ID, Name, Image, Project string
	State                    container.ContainerState
	Status                   string // "Up 2 hours (healthy)"
	Health                   string
	Created                  time.Time
	StartedAt, FinishedAt    time.Time
	RestartCount             int
	OOMKilled                bool
	Tty                      bool

	CPUPct     float64 // docker stats style: 100% = one core
	CPUShare   float64 // % of docker total CPU capacity
	CPULimit   float64 // cores allowed by --cpus / quota (0 = unlimited)
	MemUsed    int64   // usage minus inactive page cache, like `docker stats`
	MemLimit   int64   // cgroup limit (== capacity when unlimited)
	MemShare   float64 // % of docker total memory
	MemOfLimit float64 // % of own limit (-1 when unlimited)
	NetRx      float64 // bytes/s
	NetTx      float64
	BlkRead    float64 // bytes/s
	BlkWrite   float64
	PIDs       uint64

	CPUHist []float64 // CPUShare history
	MemHist []float64 // MemShare history
}

// Running reports whether the container consumes resources.
func (c ContainerStat) Running() bool {
	return c.State == container.StateRunning || c.State == container.StateRestarting
}

// Totals across all containers.
type Totals struct {
	CPUCores float64 // sum of CPUPct/100
	CPUShare float64 // % of docker capacity
	MemUsed  int64
	MemShare float64
	NetRx    float64
	NetTx    float64
	BlkRead  float64
	BlkWrite float64
	Running  int
	Paused   int
	Stopped  int

	CPUHist, MemHist, NetRxHist, NetTxHist []float64
}

// Snapshot is everything the resources view needs for one frame.
type Snapshot struct {
	At         time.Time
	Capacity   Capacity
	Totals     Totals
	Containers []ContainerStat
	Err        error
}

type rawSample struct {
	at                 time.Time
	cpuTotal, sysTotal uint64
	online             uint32
	netRx, netTx       uint64
	blkR, blkW         uint64
}

type inspectCache struct {
	key  string // state+health: re-inspect only on transitions (events also invalidate)
	resp container.InspectResponse
}

type series struct{ cpu, mem []float64 }

// Collector keeps previous samples to compute rates and histories.
type Collector struct {
	cli      *docker.Client
	capacity Capacity
	info     InfoSummary

	mu       sync.Mutex
	prev     map[string]rawSample
	inspects map[string]inspectCache
	hist     map[string]*series
	totals   Totals
}

// InfoSummary is the subset of /info shown in the header.
type InfoSummary struct {
	Name, OS, Kernel, Version, Driver, Cgroup, RootDir, Arch, Endpoint string
}

// New queries /info once to learn the capacity docker can use.
func New(ctx context.Context, cli *docker.Client) (*Collector, error) {
	res, err := cli.Info(ctx, client.InfoOptions{})
	if err != nil {
		return nil, err
	}
	i := res.Info
	h := host.Detect()
	return &Collector{
		cli: cli,
		capacity: Capacity{
			CPUs: i.NCPU, Mem: i.MemTotal,
			HostCPUs: h.CPUs, HostMem: h.Mem,
		},
		info: InfoSummary{
			Name: i.Name, OS: i.OperatingSystem, Kernel: i.KernelVersion, Version: i.ServerVersion,
			Driver: i.Driver, Cgroup: i.CgroupVersion, RootDir: i.DockerRootDir, Arch: i.Architecture,
			Endpoint: cli.Endpoint,
		},
		prev:     map[string]rawSample{},
		inspects: map[string]inspectCache{},
		hist:     map[string]*series{},
	}, nil
}

// Invalidate drops cached inspect data, e.g. after a docker event for that container.
func (c *Collector) Invalidate(id string) {
	c.mu.Lock()
	delete(c.inspects, id)
	c.mu.Unlock()
}

func (c *Collector) Capacity() Capacity { return c.capacity }
func (c *Collector) Info() InfoSummary  { return c.info }

// Sample lists every container and fetches one stats sample for running ones.
func (c *Collector) Sample(ctx context.Context) Snapshot {
	now := time.Now()
	snap := Snapshot{At: now, Capacity: c.capacity}

	list, err := c.cli.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		snap.Err = err
		c.mu.Lock()
		snap.Totals = c.totals
		c.mu.Unlock()
		return snap
	}

	stats := make([]ContainerStat, len(list.Items))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 16) // bound concurrent stats calls
	for i, s := range list.Items {
		stats[i] = ContainerStat{
			ID: s.ID, Name: docker.ContainerName(s.Names, s.ID), Image: s.Image,
			Project: s.Labels["com.docker.compose.project"],
			State:   s.State, Status: s.Status, Created: time.Unix(s.Created, 0),
			MemOfLimit: -1,
		}
		if s.Health != nil {
			stats[i].Health = string(s.Health.Status)
		}
		wg.Add(1)
		go func(i int, s container.Summary) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			c.enrich(ctx, &stats[i], s)
		}(i, s)
	}
	wg.Wait()

	c.mu.Lock()
	defer c.mu.Unlock()

	var t Totals
	seen := make(map[string]bool, len(stats))
	for i := range stats {
		st := &stats[i]
		seen[st.ID] = true
		switch st.State {
		case container.StateRunning, container.StateRestarting:
			t.Running++
		case container.StatePaused:
			t.Paused++
		default:
			t.Stopped++
		}
		t.CPUCores += st.CPUPct / 100
		t.MemUsed += st.MemUsed
		t.NetRx += st.NetRx
		t.NetTx += st.NetTx
		t.BlkRead += st.BlkRead
		t.BlkWrite += st.BlkWrite

		h := c.hist[st.ID]
		if h == nil {
			h = &series{}
			c.hist[st.ID] = h
		}
		h.cpu = push(h.cpu, st.CPUShare)
		h.mem = push(h.mem, st.MemShare)
		st.CPUHist, st.MemHist = h.cpu, h.mem
	}
	for id := range c.hist {
		if !seen[id] {
			delete(c.hist, id)
			delete(c.prev, id)
			delete(c.inspects, id)
		}
	}
	if c.capacity.CPUs > 0 {
		t.CPUShare = t.CPUCores / float64(c.capacity.CPUs) * 100
	}
	if c.capacity.Mem > 0 {
		t.MemShare = float64(t.MemUsed) / float64(c.capacity.Mem) * 100
	}
	t.CPUHist = push(c.totals.CPUHist, t.CPUShare)
	t.MemHist = push(c.totals.MemHist, t.MemShare)
	t.NetRxHist = push(c.totals.NetRxHist, t.NetRx)
	t.NetTxHist = push(c.totals.NetTxHist, t.NetTx)
	c.totals = t

	sort.SliceStable(stats, func(a, b int) bool { return stats[a].Name < stats[b].Name })
	snap.Totals = t
	snap.Containers = stats
	return snap
}

// enrich fills inspect data (cached until the status string changes) and stats.
func (c *Collector) enrich(ctx context.Context, st *ContainerStat, s container.Summary) {
	key := string(s.State)
	if s.Health != nil {
		key += "/" + string(s.Health.Status)
	}
	c.mu.Lock()
	ic, ok := c.inspects[s.ID]
	c.mu.Unlock()
	if !ok || ic.key != key {
		if r, err := c.cli.ContainerInspect(ctx, s.ID, client.ContainerInspectOptions{}); err == nil {
			ic = inspectCache{key: key, resp: r.Container}
			c.mu.Lock()
			c.inspects[s.ID] = ic
			c.mu.Unlock()
		}
	}
	if in := ic.resp; in.State != nil {
		st.StartedAt = parseTime(in.State.StartedAt)
		st.FinishedAt = parseTime(in.State.FinishedAt)
		st.OOMKilled = in.State.OOMKilled
		st.RestartCount = in.RestartCount
		if in.Config != nil {
			st.Tty = in.Config.Tty
		}
		if hc := in.HostConfig; hc != nil {
			switch {
			case hc.NanoCPUs > 0:
				st.CPULimit = float64(hc.NanoCPUs) / 1e9
			case hc.CPUQuota > 0 && hc.CPUPeriod > 0:
				st.CPULimit = float64(hc.CPUQuota) / float64(hc.CPUPeriod)
			}
		}
	}

	if s.State != container.StateRunning {
		return
	}
	res, err := c.cli.ContainerStats(ctx, s.ID, client.ContainerStatsOptions{Stream: false})
	if err != nil {
		return
	}
	var r container.StatsResponse
	err = json.NewDecoder(res.Body).Decode(&r)
	res.Body.Close()
	if err != nil {
		return
	}
	c.compute(st, &r)
}

func (c *Collector) compute(st *ContainerStat, r *container.StatsResponse) {
	cur := rawSample{
		at:       r.Read,
		cpuTotal: r.CPUStats.CPUUsage.TotalUsage,
		sysTotal: r.CPUStats.SystemUsage,
		online:   r.CPUStats.OnlineCPUs,
	}
	if cur.at.IsZero() {
		cur.at = time.Now()
	}
	if cur.online == 0 {
		cur.online = uint32(len(r.CPUStats.CPUUsage.PercpuUsage))
	}
	for _, n := range r.Networks {
		cur.netRx += n.RxBytes
		cur.netTx += n.TxBytes
	}
	for _, e := range r.BlkioStats.IoServiceBytesRecursive {
		switch strings.ToLower(e.Op) {
		case "read":
			cur.blkR += e.Value
		case "write":
			cur.blkW += e.Value
		}
	}

	c.mu.Lock()
	prev, hasPrev := c.prev[st.ID]
	c.prev[st.ID] = cur
	c.mu.Unlock()

	if hasPrev && cur.sysTotal > prev.sysTotal && cur.cpuTotal >= prev.cpuTotal {
		cpuDelta := float64(cur.cpuTotal - prev.cpuTotal)
		sysDelta := float64(cur.sysTotal - prev.sysTotal)
		st.CPUPct = cpuDelta / sysDelta * float64(cur.online) * 100
	}
	if hasPrev {
		if dt := cur.at.Sub(prev.at).Seconds(); dt > 0 {
			st.NetRx = rate(cur.netRx, prev.netRx, dt)
			st.NetTx = rate(cur.netTx, prev.netTx, dt)
			st.BlkRead = rate(cur.blkR, prev.blkR, dt)
			st.BlkWrite = rate(cur.blkW, prev.blkW, dt)
		}
	}
	if c.capacity.CPUs > 0 {
		st.CPUShare = st.CPUPct / float64(c.capacity.CPUs)
	}

	m := r.MemoryStats
	used := m.Usage
	// Same as the docker CLI: subtract inactive page cache (cgroup v1 / v2 key names).
	if v, ok := m.Stats["total_inactive_file"]; ok && v < used {
		used -= v
	} else if v, ok := m.Stats["inactive_file"]; ok && v < used {
		used -= v
	}
	st.MemUsed = int64(used)
	st.MemLimit = int64(m.Limit)
	if c.capacity.Mem > 0 {
		st.MemShare = float64(used) / float64(c.capacity.Mem) * 100
	}
	// A limit >= daemon memory means "no limit configured".
	if m.Limit > 0 && int64(m.Limit) < c.capacity.Mem {
		st.MemOfLimit = float64(used) / float64(m.Limit) * 100
	}
	st.PIDs = r.PidsStats.Current
}

func rate(cur, prev uint64, dt float64) float64 {
	if cur < prev { // counter reset (container restarted)
		return 0
	}
	return float64(cur-prev) / dt
}

func push(h []float64, v float64) []float64 {
	if len(h) >= HistoryLen {
		// copy into a new slice: snapshots handed to the UI must stay immutable
		n := make([]float64, HistoryLen-1, HistoryLen)
		copy(n, h[len(h)-HistoryLen+1:])
		return append(n, v)
	}
	n := make([]float64, len(h), len(h)+1)
	copy(n, h)
	return append(n, v)
}

func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil || t.Year() <= 1 {
		return time.Time{}
	}
	return t
}
