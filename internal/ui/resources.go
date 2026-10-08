package ui

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dustin/go-humanize"
	"github.com/moby/moby/api/types/container"

	"github.com/albedev/dtop/internal/collector"
	"github.com/albedev/dtop/internal/docker"
)

type resSort int

const (
	sortName resSort = iota
	sortState
	sortCPU
	sortMem
	sortNet
	sortIO
	sortUptime
	nResSorts
)

var resSortCol = [...]string{"name", "state", "cpu", "mem", "net", "io", "up"}

type resState struct {
	selector
	rows        []collector.ContainerStat
	sort        resSort
	desc        bool
	showStopped bool
	group       bool // group by compose project
}

func (m *Model) rebuildRes() {
	q := strings.ToLower(strings.TrimSpace(m.filter.Value()))
	rows := make([]collector.ContainerStat, 0, len(m.snap.Containers))
	for _, c := range m.snap.Containers {
		if !m.res.showStopped && !c.Running() && c.State != container.StatePaused {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(c.Name+" "+c.Image+" "+c.Project+" "+c.ID), q) {
			continue
		}
		rows = append(rows, c)
	}
	less := m.resLess()
	sort.SliceStable(rows, func(a, b int) bool {
		if m.res.group && rows[a].Project != rows[b].Project {
			return rows[a].Project < rows[b].Project
		}
		return less(rows[a], rows[b])
	})
	m.res.rows = rows
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	m.res.setIDs(ids, m.resLayout().rows)
}

func (m *Model) resLess() func(a, b collector.ContainerStat) bool {
	desc := m.res.desc
	cmp := func(x, y float64, a, b collector.ContainerStat) bool {
		if x == y {
			return a.Name < b.Name
		}
		if desc {
			return x > y
		}
		return x < y
	}
	switch m.res.sort {
	case sortName:
		return func(a, b collector.ContainerStat) bool {
			if desc {
				return a.Name > b.Name
			}
			return a.Name < b.Name
		}
	case sortState:
		return func(a, b collector.ContainerStat) bool {
			return cmp(stateRank(a), stateRank(b), a, b)
		}
	case sortMem:
		return func(a, b collector.ContainerStat) bool { return cmp(float64(a.MemUsed), float64(b.MemUsed), a, b) }
	case sortNet:
		return func(a, b collector.ContainerStat) bool { return cmp(a.NetRx+a.NetTx, b.NetRx+b.NetTx, a, b) }
	case sortIO:
		return func(a, b collector.ContainerStat) bool {
			return cmp(a.BlkRead+a.BlkWrite, b.BlkRead+b.BlkWrite, a, b)
		}
	case sortUptime:
		return func(a, b collector.ContainerStat) bool {
			return cmp(float64(uptime(a)), float64(uptime(b)), a, b)
		}
	}
	return func(a, b collector.ContainerStat) bool {
		// running containers first, then CPU
		if a.Running() != b.Running() {
			return a.Running()
		}
		return cmp(a.CPUPct, b.CPUPct, a, b)
	}
}

func stateRank(c collector.ContainerStat) float64 {
	switch c.State {
	case container.StateRunning:
		return 5
	case container.StateRestarting:
		return 4
	case container.StatePaused:
		return 3
	case container.StateCreated:
		return 2
	case container.StateExited:
		return 1
	}
	return 0
}

func uptime(c collector.ContainerStat) time.Duration {
	if c.Running() && !c.StartedAt.IsZero() {
		return time.Since(c.StartedAt)
	}
	return 0
}

func (m *Model) selectedContainer() (collector.ContainerStat, bool) {
	if len(m.res.rows) == 0 {
		return collector.ContainerStat{}, false
	}
	return m.res.rows[m.res.index()], true
}

// targetContainers are the multi-selected rows, or the cursor row when nothing is marked.
func (m *Model) targetContainers() []collector.ContainerStat {
	if m.res.hasMarks() {
		var out []collector.ContainerStat
		for _, c := range m.res.rows {
			if m.res.isMarked(c.ID) {
				out = append(out, c)
			}
		}
		return out
	}
	if c, ok := m.selectedContainer(); ok {
		return []collector.ContainerStat{c}
	}
	return nil
}

func (m Model) resKey(key string) (tea.Model, tea.Cmd) {
	visible := m.resLayout().rows
	sel := &m.res.selector
	switch key {
	case "up", "down", "pgup", "pgdown", "home", "end":
		sel.clearMarks()
		sel.move(navDelta(key, visible), visible)
	case "ctrl+up", "shift+up", "ctrl+down", "shift+down":
		sel.extend(navDelta(key, visible), visible)
	case "left":
		m.res.sort = (m.res.sort + nResSorts - 1) % nResSorts
		m.rebuildRes()
	case "right":
		m.res.sort = (m.res.sort + 1) % nResSorts
		m.rebuildRes()
	case "S":
		m.res.desc = !m.res.desc
		m.rebuildRes()
	case "a":
		m.res.showStopped = !m.res.showStopped
		m.rebuildRes()
	case "g":
		m.res.group = !m.res.group
		m.rebuildRes()
	}

	cs := m.targetContainers()
	if len(cs) == 0 {
		return m, nil
	}
	c := cs[0]
	switch key {
	case "enter", " ":
		m.menu = m.containerMenu(cs)
	case "t":
		return m, m.containersAction(cs, func(c collector.ContainerStat) (docker.Action, bool) {
			if c.Running() || c.State == container.StatePaused {
				return docker.ActStop, true
			}
			return docker.ActStart, true
		})
	case "r":
		return m, m.containersAction(cs, always(docker.ActRestart))
	case "p":
		return m, m.containersAction(cs, func(c collector.ContainerStat) (docker.Action, bool) {
			if c.State == container.StatePaused {
				return docker.ActUnpause, true
			}
			return docker.ActPause, c.State == container.StateRunning
		})
	case "k":
		return m, m.containersAction(cs, onlyRunning(docker.ActKill))
	case "d", "delete":
		return m, m.containersAction(cs, always(docker.ActRemove))
	case "D":
		return m, m.containersAction(cs, always(docker.ActForce))
	case "l":
		return m, m.openLogs(c.ID, c.Name, c.Tty)
	case "e":
		if c.State != container.StateRunning {
			m.setStatus(c.Name+" is not running", true)
			return m, nil
		}
		return m, m.execShell(c.ID, c.Name)
	}
	return m, nil
}

// navDelta maps navigation keys (plain or with ctrl/shift) to a cursor delta.
func navDelta(key string, visible int) int {
	switch strings.TrimPrefix(strings.TrimPrefix(key, "ctrl+"), "shift+") {
	case "up":
		return -1
	case "down":
		return 1
	case "pgup":
		return -visible
	case "pgdown":
		return visible
	case "home":
		return -1 << 30
	case "end":
		return 1 << 30
	}
	return 0
}

type actionPicker func(collector.ContainerStat) (docker.Action, bool)

func always(a docker.Action) actionPicker {
	return func(collector.ContainerStat) (docker.Action, bool) { return a, true }
}

func onlyRunning(a docker.Action) actionPicker {
	return func(c collector.ContainerStat) (docker.Action, bool) { return a, c.State == container.StateRunning }
}

func onlyStopped(a docker.Action) actionPicker {
	return func(c collector.ContainerStat) (docker.Action, bool) {
		return a, !c.Running() && c.State != container.StatePaused
	}
}

type job struct {
	id, name string
	a        docker.Action
}

// containersAction runs pick(c) on every target, asking for confirmation first when
// any resulting action is destructive. Targets for which pick says false are skipped.
func (m *Model) containersAction(cs []collector.ContainerStat, pick actionPicker) tea.Cmd {
	var jobs []job
	destructive := false
	for _, c := range cs {
		if a, ok := pick(c); ok {
			jobs = append(jobs, job{c.ID, c.Name, a})
			destructive = destructive || a.Destructive()
		}
	}
	if len(jobs) == 0 {
		m.setStatus("nothing to do for the selected containers", true)
		return nil
	}
	label := jobsLabel(jobs)
	cli := m.cli
	run := m.run(func(ctx context.Context) (string, error) {
		return runJobs(ctx, label, jobs, func(ctx context.Context, j job) error {
			return cli.ContainerAction(ctx, j.id, j.a)
		})
	})
	if !destructive {
		m.res.clearMarks()
		m.setStatus(label+"…", false)
		return run
	}
	body := map[docker.Action]string{
		docker.ActKill:   "Send SIGKILL. Unsaved state is lost.",
		docker.ActRemove: "Remove the container and its writable layer.",
		docker.ActForce:  "Kill (if running) and remove the container and its writable layer.",
	}[jobs[0].a]
	if len(jobs) > 1 {
		names := make([]string, len(jobs))
		for i, j := range jobs {
			names[i] = j.name
		}
		body += " Targets: " + listNames(names, 8) + "."
	}
	if skipped := len(cs) - len(jobs); skipped > 0 {
		body += fmt.Sprintf(" %d selected container(s) skipped (not applicable).", skipped)
	}
	m.confirm = &confirmState{title: label + "?", body: body, run: run,
		onYes: func(m *Model) { m.res.clearMarks() }}
	return nil
}

// jobsLabel: "kill web-1", "restart 3 containers", "stop/start 4 containers".
func jobsLabel(jobs []job) string {
	var acts []string
	seen := map[docker.Action]bool{}
	for _, j := range jobs {
		if !seen[j.a] {
			seen[j.a] = true
			acts = append(acts, string(j.a))
		}
	}
	if len(jobs) == 1 {
		return acts[0] + " " + jobs[0].name
	}
	return fmt.Sprintf("%s %d containers", strings.Join(acts, "/"), len(jobs))
}

// runJobs executes jobs with bounded parallelism and summarizes the outcome.
func runJobs(ctx context.Context, label string, jobs []job, do func(context.Context, job) error) (string, error) {
	var (
		mu     sync.Mutex
		failed []string
		wg     sync.WaitGroup
		sem    = make(chan struct{}, 8)
	)
	for _, j := range jobs {
		wg.Add(1)
		go func(j job) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if err := do(ctx, j); err != nil {
				mu.Lock()
				failed = append(failed, fmt.Sprintf("%s: %v", j.name, err))
				mu.Unlock()
			}
		}(j)
	}
	wg.Wait()
	return summarize(label, len(jobs), failed)
}

func summarize(label string, total int, failed []string) (string, error) {
	switch {
	case len(failed) == 0:
		return label + ": done", nil
	case total == 1:
		return "", fmt.Errorf("%s: %s", label, failed[0])
	}
	sort.Strings(failed)
	return "", fmt.Errorf("%s: %d ok, %d failed (%s)", label, total-len(failed), len(failed), strings.Join(failed, "; "))
}

func listNames(names []string, n int) string {
	if len(names) <= n {
		return strings.Join(names, ", ")
	}
	return strings.Join(names[:n], ", ") + fmt.Sprintf(" and %d more", len(names)-n)
}

func (m *Model) containerMenu(cs []collector.ContainerStat) *menuState {
	act := func(p actionPicker) func(*Model) tea.Cmd {
		return func(m *Model) tea.Cmd { return m.containersAction(cs, p) }
	}
	if len(cs) > 1 {
		return &menuState{title: fmt.Sprintf("%d containers", len(cs)), items: []menuItem{
			{"s", "start", act(onlyStopped(docker.ActStart))},
			{"t", "stop", act(func(c collector.ContainerStat) (docker.Action, bool) {
				return docker.ActStop, c.Running() || c.State == container.StatePaused
			})},
			{"r", "restart", act(always(docker.ActRestart))},
			{"p", "pause", act(onlyRunning(docker.ActPause))},
			{"u", "unpause", act(func(c collector.ContainerStat) (docker.Action, bool) {
				return docker.ActUnpause, c.State == container.StatePaused
			})},
			{"k", "kill (SIGKILL)", act(onlyRunning(docker.ActKill))},
			{"d", "remove", act(always(docker.ActRemove))},
			{"D", "force remove", act(always(docker.ActForce))},
		}}
	}
	c := cs[0]
	var items []menuItem
	if c.Running() || c.State == container.StatePaused {
		items = append(items, menuItem{"t", "stop", act(always(docker.ActStop))})
	} else {
		items = append(items, menuItem{"t", "start", act(always(docker.ActStart))})
	}
	items = append(items, menuItem{"r", "restart", act(always(docker.ActRestart))})
	if c.State == container.StatePaused {
		items = append(items, menuItem{"p", "unpause", act(always(docker.ActUnpause))})
	} else if c.Running() {
		items = append(items, menuItem{"p", "pause", act(always(docker.ActPause))})
	}
	if c.Running() {
		items = append(items, menuItem{"k", "kill (SIGKILL)", act(always(docker.ActKill))})
	}
	items = append(items,
		menuItem{"d", "remove", act(always(docker.ActRemove))},
		menuItem{"D", "force remove", act(always(docker.ActForce))},
		menuItem{"l", "logs", func(m *Model) tea.Cmd { return m.openLogs(c.ID, c.Name, c.Tty) }},
	)
	if c.State == container.StateRunning {
		items = append(items, menuItem{"e", "shell (docker exec)", func(m *Model) tea.Cmd { return m.execShell(c.ID, c.Name) }})
	}
	return &menuState{title: c.Name, items: items}
}

// ---- layout & rendering ----

type resLayoutT struct {
	topH, evH, tblH int
	tableTop        int // screen row of first data row
	rows            int // visible data rows
}

func (m Model) resLayout() resLayoutT {
	body := max(m.h-2, 0)
	var l resLayoutT
	switch {
	case body >= 24:
		l.topH = min(max(body*35/100, 8), 14)
	case body >= 14:
		l.topH = 6
	}
	if body >= 36 {
		l.evH = 7
	}
	l.tblH = body - l.topH - l.evH
	l.tableTop = 1 + l.topH + 2
	l.rows = max(l.tblH-3, 1)
	return l
}

func (m Model) viewResources() []string {
	l := m.resLayout()
	var out []string
	if l.topH > 0 {
		out = append(out, strings.Split(m.renderTop(l.topH), "\n")...)
	}
	out = append(out, strings.Split(m.renderContainers(l.tblH), "\n")...)
	if l.evH > 0 {
		out = append(out, strings.Split(m.renderEvents(l.evH), "\n")...)
	}
	return out
}

func (m Model) renderTop(h int) string {
	w := m.w
	var cpuW, memW, netW int
	if w >= 100 {
		cpuW = w / 2
		memW = w / 4
		netW = w - cpuW - memW
	} else {
		cpuW = w * 6 / 10
		memW = w - cpuW
	}
	boxes := []string{m.renderCPU(cpuW, h), m.renderMem(memW, h)}
	if netW > 0 {
		boxes = append(boxes, m.renderNet(netW, h))
	}
	return joinH(boxes...)
}

func (m Model) renderCPU(w, h int) string {
	cap, t := m.snap.Capacity, m.snap.Totals
	in := w - 2
	lines := []string{
		spread(
			sMain.Render(fmt.Sprintf("docker %s / %d cores", sTitle.Render(fmt.Sprintf("%.2f", t.CPUCores)), cap.CPUs)),
			level(t.CPUShare).Bold(true).Render(fmt.Sprintf("%5.1f%%", t.CPUShare)), in),
	}
	hostLine := fmt.Sprintf("host %d cores", cap.HostCPUs)
	if cap.HostCPUs > 0 && cap.CPUs > 0 && cap.CPUs != cap.HostCPUs {
		hostLine += fmt.Sprintf(" · docker may use %d (%.0f%%)", cap.CPUs, float64(cap.CPUs)/float64(cap.HostCPUs)*100)
	}
	lines = append(lines, sDim.Render(hostLine))
	lines = append(lines, graph(t.CPUHist, in, h-2-len(lines), 100, gCPU, m.opt.Mode)...)
	return box(sTitle.Render("cpu"), lines, w, h, cBoxCPU)
}

func (m Model) renderMem(w, h int) string {
	cap, t := m.snap.Capacity, m.snap.Totals
	in := w - 2
	free := cap.Mem - t.MemUsed
	lines := []string{
		spread(sMain.Render("limit ")+sTitle.Render(humanize.IBytes(uint64(cap.Mem))),
			sDim.Render(fmt.Sprintf("host %s", humanize.IBytes(uint64(cap.HostMem)))), in),
		spread(sMain.Render("used  ")+sTitle.Render(humanize.IBytes(uint64(t.MemUsed))),
			level(t.MemShare).Bold(true).Render(fmt.Sprintf("%5.1f%%", t.MemShare)), in),
		meter(t.MemShare, in, gMem, m.opt.Mode),
		sDim.Render("free  " + humanize.IBytes(uint64(max(free, 0)))),
	}
	if g := h - 2 - len(lines); g > 0 {
		lines = append(lines, graph(t.MemHist, in, g, 100, gMem, m.opt.Mode)...)
	}
	return box(sTitle.Render("mem"), lines, w, h, cBoxMem)
}

func (m Model) renderNet(w, h int) string {
	t := m.snap.Totals
	in := w - 2
	gh := max((h-2-3)/2, 1)
	peak := func(v []float64) float64 {
		p := 1024.0
		for _, x := range v[max(len(v)-in, 0):] {
			p = max(p, x)
		}
		return p
	}
	rxMax, txMax := peak(t.NetRxHist), peak(t.NetTxHist)
	lines := []string{spread(sBlue.Render("↓ rx ")+sTitle.Render(rate(t.NetRx)), sDim.Render("top "+rate(rxMax)), in)}
	lines = append(lines, graph(t.NetRxHist, in, gh, rxMax, gRx, m.opt.Mode)...)
	lines = append(lines, spread(sPurp.Render("↑ tx ")+sTitle.Render(rate(t.NetTx)), sDim.Render("top "+rate(txMax)), in))
	lines = append(lines, graph(t.NetTxHist, in, gh, txMax, gTx, m.opt.Mode)...)
	lines = append(lines, sDim.Render("io ")+sMain.Render("r "+rate(t.BlkRead)+"  w "+rate(t.BlkWrite)))
	return box(sTitle.Render("net·io"), lines, w, h, cBoxNet)
}

var exitCode = regexp.MustCompile(`Exited \((\-?\d+)\)`)

func stateCell(c collector.ContainerStat) string {
	switch {
	case c.Health == "unhealthy":
		return sRed.Render("unhealthy")
	case c.Health == "starting":
		return sYel.Render("starting")
	}
	switch c.State {
	case container.StateRunning:
		if c.Health == "healthy" {
			return sGreen.Render("healthy")
		}
		return sGreen.Render("running")
	case container.StatePaused:
		return sYel.Render("paused")
	case container.StateRestarting:
		return sOrng.Render("restarting")
	case container.StateDead:
		return sRed.Render("dead")
	case container.StateExited:
		code := "?"
		if mm := exitCode.FindStringSubmatch(c.Status); mm != nil {
			code = mm[1]
		}
		if c.OOMKilled {
			return sRed.Render("oom-killed")
		}
		if code != "0" {
			return sRed.Render("exited(" + code + ")")
		}
		return sDim.Render("exited(0)")
	}
	return sDim.Render(string(c.State))
}

func (m Model) resColumns(width int) []column {
	all := []column{
		{id: "name", title: "NAME", w: 14, flex: true},
		{id: "state", title: "STATE", w: 10},
		{id: "cpu", title: "CPU%", w: 6, right: true},
		{id: "cpuD", title: "/DKR", w: 5, right: true},
		{id: "graph", title: "CPU HIST", w: 10},
		{id: "mem", title: "MEM", w: 7, right: true},
		{id: "memD", title: "/DKR", w: 5, right: true},
		{id: "memL", title: "/LIM", w: 5, right: true},
		{id: "net", title: "NET/s ↓/↑", w: 15, right: true},
		{id: "io", title: "IO/s r/w", w: 15, right: true},
		{id: "pids", title: "PIDS", w: 4, right: true},
		{id: "up", title: "UP", w: 7, right: true},
	}
	need := func(cs []column) int {
		n := 0
		for _, c := range cs {
			n += c.w + 1
		}
		return n
	}
	for _, drop := range []string{"io", "graph", "pids", "memL", "net", "cpuD", "up"} {
		if need(all) <= width {
			break
		}
		for i, c := range all {
			if c.id == drop {
				all = append(all[:i], all[i+1:]...)
				break
			}
		}
	}
	return all
}

func (m Model) renderContainers(h int) string {
	in := m.w - 2
	cols := m.resColumns(in)
	sortCol := -1
	for i, c := range cols {
		if c.id == resSortCol[m.res.sort] {
			sortCol = i
		}
	}
	rows := make([][]string, len(m.res.rows))
	for i, c := range m.res.rows {
		row := make([]string, len(cols))
		for j, col := range cols {
			row[j] = m.resCell(col.id, c)
		}
		rows[i] = row
	}
	lines := renderTable(cols, rows, m.res.index(), m.res.off, in, h-2, sortCol, m.res.desc,
		func(i int) bool { return m.res.isMarked(m.res.rows[i].ID) })
	if len(m.res.rows) == 0 {
		msg := "no running containers (a: show stopped)"
		if m.res.showStopped {
			msg = "no containers"
		}
		if !m.hasSnap {
			msg = "connecting to docker…"
		}
		if len(lines) > 1 {
			lines[1] = sDim.Render(msg)
		}
	}
	t := m.snap.Totals
	title := sTitle.Render("containers") + sDim.Render(fmt.Sprintf(" %d running · %d paused · %d stopped", t.Running, t.Paused, t.Stopped))
	if n := len(m.res.marked); n > 0 {
		title += sPurp.Bold(true).Render(fmt.Sprintf(" %d selected", n))
	}
	if f := m.filter.Value(); f != "" {
		title += sYel.Render(" /" + f)
	}
	if m.res.group {
		title += sCyan.Render(" [compose]")
	}
	if !m.res.showStopped {
		title += sDim.Render(" [running]")
	}
	return box(title, lines, m.w, h, cBoxProc)
}

func (m Model) resCell(id string, c collector.ContainerStat) string {
	run := c.Running()
	dash := sDim.Render("-")
	switch id {
	case "name":
		n := c.Name
		if m.res.group && c.Project != "" {
			n = sDim.Render(c.Project+"/") + sMain.Render(strings.TrimPrefix(n, c.Project+"-"))
		} else {
			n = sMain.Render(n)
		}
		if c.RestartCount > 0 {
			n += sOrng.Render(fmt.Sprintf(" ↻%d", c.RestartCount))
		}
		return n
	case "state":
		return stateCell(c)
	case "cpu":
		if !run {
			return dash
		}
		return level(c.CPUShare).Render(fmt.Sprintf("%.1f", c.CPUPct))
	case "cpuD":
		if !run {
			return dash
		}
		return level(c.CPUShare).Render(fmt.Sprintf("%.1f", c.CPUShare))
	case "graph":
		if !run {
			return ""
		}
		peak := 1.0
		for _, v := range c.CPUHist {
			peak = max(peak, v)
		}
		return sparkline(c.CPUHist, 10, peak, gCPU, m.opt.Mode)
	case "mem":
		if !run {
			return dash
		}
		return sMain.Render(short(c.MemUsed))
	case "memD":
		if !run {
			return dash
		}
		return level(c.MemShare).Render(fmt.Sprintf("%.1f", c.MemShare))
	case "memL":
		if !run || c.MemOfLimit < 0 {
			return dash
		}
		return level(c.MemOfLimit).Render(fmt.Sprintf("%.0f", c.MemOfLimit))
	case "net":
		if !run {
			return dash
		}
		return sBlue.Render(short(int64(c.NetRx))) + sDim.Render("/") + sPurp.Render(short(int64(c.NetTx)))
	case "io":
		if !run {
			return dash
		}
		return sMain.Render(short(int64(c.BlkRead))) + sDim.Render("/") + sMain.Render(short(int64(c.BlkWrite)))
	case "pids":
		if !run {
			return dash
		}
		return sMain.Render(fmt.Sprint(c.PIDs))
	case "up":
		if run && !c.StartedAt.IsZero() {
			return sMain.Render(dur(time.Since(c.StartedAt)))
		}
		if !c.FinishedAt.IsZero() {
			return sDim.Render("-" + dur(time.Since(c.FinishedAt)))
		}
		return dash
	}
	return ""
}

func (m Model) renderEvents(h int) string {
	n := h - 2
	start := max(len(m.evts)-n, 0)
	lines := make([]string, 0, n)
	for _, e := range m.evts[start:] {
		st := sMain
		switch e.action {
		case "start", "unpause", "create", "pull":
			st = sGreen
		case "die", "kill", "oom":
			st = sRed
		case "stop", "pause", "restart":
			st = sYel
		case "destroy", "delete", "untag", "prune":
			st = sPurp
		}
		lines = append(lines, sDim.Render(e.at.Format("15:04:05")+" ")+sCyan.Render(fit(e.typ, 9))+" "+st.Render(fit(e.action, 10))+" "+sMain.Render(e.target))
	}
	if len(lines) == 0 {
		lines = append(lines, sDim.Render("waiting for docker events…"))
	}
	return box(sTitle.Render("events"), lines, m.w, h, cBoxEvent)
}

// ---- small helpers ----

// short is a compact byte size for tables, always with a unit: 89B, 12K, 300M, 1.2G.
func short(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%dB", max(n, 0))
	}
	s := strings.Replace(humanize.IBytes(uint64(n)), " ", "", 1)
	return strings.TrimSuffix(s, "iB")
}

func rate(v float64) string { return humanize.IBytes(uint64(max(v, 0))) + "/s" }

// dur renders a compact duration: 42s, 5m, 3h05m, 4d, 2mo, 1y.
func dur(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	case d < 60*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo", int(d.Hours()/24/30))
	}
	return fmt.Sprintf("%dy", int(d.Hours()/24/365))
}

func ago(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	return dur(time.Since(t)) + " ago"
}

func joinH(boxes ...string) string {
	split := make([][]string, len(boxes))
	h := 0
	for i, b := range boxes {
		split[i] = strings.Split(b, "\n")
		h = max(h, len(split[i]))
	}
	out := make([]string, h)
	for r := 0; r < h; r++ {
		var sb strings.Builder
		for _, s := range split {
			if r < len(s) {
				sb.WriteString(s[r])
			}
		}
		out[r] = sb.String()
	}
	return strings.Join(out, "\n")
}
