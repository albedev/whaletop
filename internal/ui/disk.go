package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"

	"github.com/albedev/dtop/internal/collector"
	"github.com/albedev/dtop/internal/docker"
)

type diskSort int

const (
	dsortSize diskSort = iota
	dsortLastUsed
	dsortCreated
	dsortName
	dsortUsage
	nDiskSorts
)

var diskSortCol = [...]string{"size", "last", "created", "name", "usage"}

var sectionNames = [...]string{"images", "containers", "volumes", "build cache"}
var sectionKeys = [...]string{"i", "c", "v", "b"}
var sectionStyles = [...]lipgloss.Style{sBlue, sGreen, sPurp, sOrng}

type diskState struct {
	section    int
	sel        [4]selector
	rows       []collector.DiskItem
	sort       diskSort
	desc       bool
	unusedOnly bool
}

func (s *diskState) cur() *selector { return &s.sel[s.section] }

func (m *Model) rebuildDisk() {
	if !m.hasDisk || len(m.disk.Sections) <= m.dsk.section {
		m.dsk.rows = nil
		return
	}
	q := strings.ToLower(strings.TrimSpace(m.filter.Value()))
	sec := m.disk.Sections[m.dsk.section]
	rows := make([]collector.DiskItem, 0, len(sec.Items))
	for _, it := range sec.Items {
		if m.dsk.unusedOnly && !it.Reclaimable() {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(it.Name+" "+it.ID+" "+it.Detail+" "+strings.Join(it.UsedBy, " ")), q) {
			continue
		}
		rows = append(rows, it)
	}
	desc := m.dsk.desc
	sort.SliceStable(rows, func(a, b int) bool {
		x, y := rows[a], rows[b]
		var less, eq bool
		switch m.dsk.sort {
		case dsortSize:
			less, eq = x.Size < y.Size, x.Size == y.Size
		case dsortLastUsed:
			xt, yt := lastUsedKey(x), lastUsedKey(y)
			less, eq = xt.Before(yt), xt.Equal(yt)
		case dsortCreated:
			less, eq = x.Created.Before(y.Created), x.Created.Equal(y.Created)
		case dsortName:
			less, eq = x.Name < y.Name, x.Name == y.Name
		case dsortUsage:
			// descending = most removable first
			less, eq = x.Usage < y.Usage, x.Usage == y.Usage
		}
		if eq {
			return x.Name < y.Name
		}
		if desc {
			return !less
		}
		return less
	})
	m.dsk.rows = rows
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	m.dsk.cur().setIDs(ids, m.diskLayout().rows)
}

func lastUsedKey(d collector.DiskItem) time.Time {
	if d.Running {
		return time.Now().Add(time.Hour)
	}
	return d.LastUsed
}

func (m *Model) selectedDiskItem() (collector.DiskItem, bool) {
	if len(m.dsk.rows) == 0 {
		return collector.DiskItem{}, false
	}
	return m.dsk.rows[m.dsk.cur().index()], true
}

// targetDiskItems are the multi-selected rows of the current section, or the cursor row.
func (m *Model) targetDiskItems() []collector.DiskItem {
	sel := m.dsk.cur()
	if sel.hasMarks() {
		var out []collector.DiskItem
		for _, it := range m.dsk.rows {
			if sel.isMarked(it.ID) {
				out = append(out, it)
			}
		}
		return out
	}
	if it, ok := m.selectedDiskItem(); ok {
		return []collector.DiskItem{it}
	}
	return nil
}

func (m *Model) sectionKind() string {
	return [...]string{"images", "containers", "volumes", "cache"}[m.dsk.section]
}

func (m Model) diskKey(key string) (tea.Model, tea.Cmd) {
	visible := m.diskLayout().rows
	sel := m.dsk.cur()
	switch key {
	case "up", "down", "pgup", "pgdown", "home", "end":
		sel.clearMarks()
		sel.move(navDelta(key, visible), visible)
	case "ctrl+up", "shift+up", "ctrl+down", "shift+down":
		sel.extend(navDelta(key, visible), visible)
	case "tab":
		m.dsk.section = (m.dsk.section + 1) % 4
		m.rebuildDisk()
	case "shift+tab":
		m.dsk.section = (m.dsk.section + 3) % 4
		m.rebuildDisk()
	case "i", "c", "v", "b":
		for i, k := range sectionKeys {
			if k == key {
				m.dsk.section = i
			}
		}
		m.rebuildDisk()
	case "left":
		m.dsk.sort = (m.dsk.sort + nDiskSorts - 1) % nDiskSorts
		m.rebuildDisk()
	case "right":
		m.dsk.sort = (m.dsk.sort + 1) % nDiskSorts
		m.rebuildDisk()
	case "S":
		m.dsk.desc = !m.dsk.desc
		m.rebuildDisk()
	case "u":
		m.dsk.unusedOnly = !m.dsk.unusedOnly
		m.rebuildDisk()
	case "P":
		m.menu = m.pruneMenu()
	case "d", "delete", "D":
		if its := m.targetDiskItems(); len(its) > 0 {
			m.confirmRemove(its, key == "D")
		}
	case "enter", " ":
		if its := m.targetDiskItems(); len(its) > 0 {
			m.menu = m.diskItemMenu(its)
		}
	case "l":
		if it, ok := m.selectedDiskItem(); ok && m.sectionKind() == "containers" {
			return m, m.openLogs(it.ID, it.Name, false)
		}
	}
	return m, nil
}

func (m *Model) confirmRemove(its []collector.DiskItem, force bool) {
	kind := m.sectionKind()
	cli := m.cli
	verb := "remove"
	if force {
		verb = "force remove"
	}
	singular := strings.TrimSuffix(strings.TrimSuffix(kind, "s"), " ")
	var (
		frees  int64
		users  []string
		active bool
		names  = make([]string, len(its))
		jobs   = make([]job, len(its))
	)
	seenUser := map[string]bool{}
	for i, it := range its {
		frees += max(it.Size-it.Shared, 0)
		names[i] = it.Name
		jobs[i] = job{id: it.ID, name: it.Name}
		active = active || it.Usage == collector.UsageActive
		for _, u := range it.UsedBy {
			if !seenUser[u] {
				seenUser[u] = true
				users = append(users, u)
			}
		}
	}
	title := fmt.Sprintf("%s %s %s?", verb, singular, its[0].Name)
	label := fmt.Sprintf("%s %s", verb, its[0].Name)
	if len(its) > 1 {
		title = fmt.Sprintf("%s %d %s?", verb, len(its), kind)
		label = fmt.Sprintf("%s %d %s", verb, len(its), kind)
	}
	body := fmt.Sprintf("Frees up to %s.", humanize.IBytes(uint64(frees)))
	if len(its) > 1 {
		body += " Targets: " + listNames(names, 8) + "."
	}
	if len(users) > 0 {
		body += " Used by: " + listNames(users, 8) + "."
	}
	if active && !force {
		body += " Some are in use: the daemon will probably refuse them (use D to force)."
	}
	sel := m.dsk.section
	m.confirm = &confirmState{
		title: title,
		body:  body,
		// sequential: image removals can depend on each other (parent/child, shared tags)
		run: m.run(func(ctx context.Context) (string, error) {
			var failed []string
			var extra string
			for _, j := range jobs {
				msg, err := cli.RemoveObject(ctx, kind, j.id, force)
				if err != nil {
					failed = append(failed, fmt.Sprintf("%s: %v", j.name, err))
				} else if len(jobs) == 1 && msg != "" {
					extra = " (" + msg + ")"
				}
			}
			out, err := summarize(label, len(jobs), failed)
			return out + extra, err
		}),
		onYes: func(m *Model) { m.dsk.sel[sel].clearMarks() },
	}
}

func (m *Model) diskItemMenu(its []collector.DiskItem) *menuState {
	title := its[0].Name
	if len(its) > 1 {
		title = fmt.Sprintf("%d %s", len(its), m.sectionKind())
	}
	items := []menuItem{
		{"d", "remove", func(m *Model) tea.Cmd { m.confirmRemove(its, false); return nil }},
		{"D", "force remove", func(m *Model) tea.Cmd { m.confirmRemove(its, true); return nil }},
	}
	if m.sectionKind() == "containers" && len(its) == 1 {
		it := its[0]
		items = append(items, menuItem{"l", "logs", func(m *Model) tea.Cmd { return m.openLogs(it.ID, it.Name, false) }})
	}
	return &menuState{title: title, items: items}
}

func (m *Model) pruneMenu() *menuState {
	p := func(a docker.Action, what string) func(*Model) tea.Cmd {
		return func(m *Model) tea.Cmd {
			cli := m.cli
			m.confirm = &confirmState{
				title: "prune " + what + "?",
				body:  "This permanently deletes data and cannot be undone.",
				run:   m.run(func(ctx context.Context) (string, error) { return cli.Prune(ctx, a) }),
			}
			return nil
		}
	}
	return &menuState{title: "prune", items: []menuItem{
		{"c", "stopped containers", p(docker.ActPruneContainers, "stopped containers")},
		{"d", "dangling images", p(docker.ActPruneDangling, "dangling images")},
		{"i", "all unused images", p(docker.ActPruneImages, "all images without containers")},
		{"a", "anonymous unused volumes", p(docker.ActPruneAnonVols, "anonymous unused volumes")},
		{"v", "all unused volumes", p(docker.ActPruneVolumes, "ALL unused volumes (named too)")},
		{"b", "build cache", p(docker.ActPruneCache, "the whole build cache")},
	}}
}

// ---- layout & rendering ----

type diskLayoutT struct {
	sumH, tblH, detH int
	tableTop, rows   int
}

func (m Model) diskLayout() diskLayoutT {
	body := max(m.h-2, 0)
	l := diskLayoutT{sumH: 7}
	if body < 16 {
		l.sumH = 0
	}
	if body >= 26 {
		l.detH = 5
	}
	l.tblH = body - l.sumH - l.detH
	l.tableTop = 1 + l.sumH + 2
	l.rows = max(l.tblH-3, 1)
	return l
}

func (m Model) viewDisk() []string {
	l := m.diskLayout()
	var out []string
	if l.sumH > 0 {
		out = append(out, strings.Split(m.renderDiskSummary(l.sumH), "\n")...)
	}
	out = append(out, strings.Split(m.renderDiskTable(l.tblH), "\n")...)
	if l.detH > 0 {
		out = append(out, strings.Split(m.renderDiskDetail(l.detH), "\n")...)
	}
	return out
}

func (m Model) renderDiskSummary(h int) string {
	in := m.w - 2
	if !m.hasDisk {
		return box(sTitle.Render("disk"), []string{sDim.Render("scanning docker disk usage (docker system df)…")}, m.w, h, cBoxDisk)
	}
	d := m.disk
	c := d.Capacity
	var lines []string

	capTxt := sYel.Render("unknown")
	if c.Known {
		capTxt = sTitle.Render(humanize.IBytes(uint64(c.Total)))
	}
	lines = append(lines, spread(sMain.Render("capacity  ")+capTxt+sDim.Render("  "+c.Source), "", in))

	var reclaim int64
	for _, s := range d.Sections {
		reclaim += s.Reclaimable
	}
	usedTxt := sMain.Render("docker    ") + sTitle.Render(humanize.IBytes(uint64(d.Used)))
	if c.Known && c.Total > 0 {
		pct := float64(d.Used) / float64(c.Total) * 100
		usedTxt += " " + level(pct).Render(fmt.Sprintf("%.1f%% of capacity", pct))
	}
	recl := sGreen.Render("reclaimable " + humanize.IBytes(uint64(reclaim)))
	if d.Used > 0 {
		recl += sDim.Render(fmt.Sprintf(" (%.0f%%)", float64(reclaim)/float64(d.Used)*100))
	}
	lines = append(lines, spread(usedTxt, recl, in))

	total := d.Used
	if c.Known && c.Total > d.Used {
		total = c.Total
	}
	segs := make([]segment, 0, 4)
	for i, s := range d.Sections {
		segs = append(segs, segment{s.Total, sectionStyles[i]})
	}
	lines = append(lines, stackedBar(segs, total, in, m.opt.Mode))

	var legend []string
	for i, s := range d.Sections {
		legend = append(legend, sectionStyles[i].Render("■ ")+sMain.Render(sectionNames[i]+" ")+sTitle.Render(short(s.Total)))
	}
	if c.Known && c.Total > d.Used {
		legend = append(legend, sDim.Render("■ free "+short(c.Total-d.Used)))
	}
	lines = append(lines, strings.Join(legend, "   "))

	var info []string
	if c.Allocated >= 0 && c.Known {
		lbl := "fs used"
		if c.VM {
			lbl = "vm disk allocated on host"
		}
		info = append(info, fmt.Sprintf("%s %s", lbl, humanize.IBytes(uint64(c.Allocated))))
	}
	if c.Available >= 0 && c.Known {
		info = append(info, "available "+humanize.IBytes(uint64(c.Available)))
	}
	scan := fmt.Sprintf("scanned %s in %s", ago(d.At), d.Took.Round(10*time.Millisecond))
	if m.diskLoading {
		scan = "rescanning…"
	}
	lines = append(lines, spread(sDim.Render(strings.Join(info, " · ")), sDim.Render(scan), in))
	return box(sTitle.Render("disk"), lines, m.w, h, cBoxDisk)
}

func (m Model) diskColumns(width int) []column {
	kind := m.sectionKind()
	detail := map[string]string{"images": "TAGS", "containers": "IMAGE", "volumes": "DRIVER", "cache": "TYPE"}[kind]
	cols := []column{
		{id: "name", title: "NAME", w: 20, flex: true},
		{id: "usage", title: "STATE", w: 8},
		{id: "usedby", title: "USED BY", w: 16, flex: true},
		{id: "size", title: "SIZE", w: 7, right: true},
	}
	if kind == "images" {
		cols = append(cols, column{id: "shared", title: "SHARED", w: 7, right: true})
	}
	cols = append(cols,
		column{id: "created", title: "CREATED", w: 9, right: true},
		column{id: "last", title: "LAST USED", w: 10, right: true},
		column{id: "detail", title: detail, w: 12},
	)
	need := func() int {
		n := 0
		for _, c := range cols {
			n += c.w + 1
		}
		return n
	}
	for _, drop := range []string{"detail", "shared", "created", "usedby"} {
		if need() <= width {
			break
		}
		for i, c := range cols {
			if c.id == drop {
				cols = append(cols[:i], cols[i+1:]...)
				break
			}
		}
	}
	if kind == "cache" || kind == "containers" {
		for i := range cols {
			if cols[i].id == "usedby" && kind == "cache" {
				cols[i].title = "DESCRIPTION"
			}
		}
	}
	return cols
}

func usageStyle(u collector.Usage) lipgloss.Style {
	switch u {
	case collector.UsageActive:
		return sGreen
	case collector.UsageInUse:
		return sCyan
	case collector.UsageUnused:
		return sYel
	}
	return sRed
}

func (m Model) diskCell(id string, it collector.DiskItem) string {
	kind := m.sectionKind()
	switch id {
	case "name":
		return sMain.Render(it.Name)
	case "usage":
		return usageStyle(it.Usage).Render(it.Usage.Label(kind))
	case "usedby":
		if kind == "cache" || kind == "containers" {
			return sDim.Render(it.Extra)
		}
		if len(it.UsedBy) == 0 {
			return sDim.Render("-")
		}
		s := it.UsedBy[0]
		if len(it.UsedBy) > 1 {
			s += fmt.Sprintf(" +%d", len(it.UsedBy)-1)
		}
		return sMain.Render(s)
	case "size":
		if it.Size < 0 {
			return sDim.Render("n/a")
		}
		return sTitle.Render(short(it.Size))
	case "shared":
		if it.Shared <= 0 {
			return sDim.Render("-")
		}
		return sDim.Render(short(it.Shared))
	case "created":
		if it.Created.IsZero() {
			return sDim.Render("-")
		}
		return sDim.Render(dur(time.Since(it.Created)))
	case "last":
		if it.Running {
			return sGreen.Render("now")
		}
		if it.LastUsed.IsZero() {
			return sDim.Render("never")
		}
		return sMain.Render(dur(time.Since(it.LastUsed)))
	case "detail":
		if kind == "images" {
			return sDim.Render(it.Extra)
		}
		return sDim.Render(it.Detail)
	}
	return ""
}

func (m Model) renderDiskTable(h int) string {
	in := m.w - 2
	cols := m.diskColumns(in)
	sortCol := -1
	for i, c := range cols {
		if c.id == diskSortCol[m.dsk.sort] {
			sortCol = i
		}
	}
	rows := make([][]string, len(m.dsk.rows))
	for i, it := range m.dsk.rows {
		r := make([]string, len(cols))
		for j, c := range cols {
			r[j] = m.diskCell(c.id, it)
		}
		rows[i] = r
	}
	sel := m.dsk.sel[m.dsk.section]
	lines := renderTable(cols, rows, sel.index(), sel.off, in, h-2, sortCol, m.dsk.desc,
		func(i int) bool { return sel.isMarked(m.dsk.rows[i].ID) })
	if len(rows) == 0 && len(lines) > 1 {
		msg := "nothing here"
		if !m.hasDisk {
			msg = "scanning…"
		}
		lines[1] = sDim.Render(msg)
	}

	// section tabs embedded in the title, btop style
	var tabs []string
	for i, n := range sectionNames {
		label := n
		if m.hasDisk && i < len(m.disk.Sections) {
			s := m.disk.Sections[i]
			label += fmt.Sprintf(" %d·%s", len(s.Items), short(s.Total))
		}
		// section names start with their hotkey letter: highlight it in place
		st := sDim
		if i == m.dsk.section {
			st = sSel
		}
		tabs = append(tabs, sHi.Inherit(st).Render(label[:1])+st.Render(label[1:]))
	}
	title := strings.Join(tabs, sDim.Render(" │ "))
	if n := len(sel.marked); n > 0 {
		var size int64
		for _, it := range m.dsk.rows {
			if sel.isMarked(it.ID) {
				size += it.Size
			}
		}
		title += sPurp.Bold(true).Render(fmt.Sprintf(" %d selected · %s", n, short(size)))
	}
	if m.dsk.unusedOnly {
		title += sYel.Render(" [reclaimable]")
	}
	if f := m.filter.Value(); f != "" {
		title += sYel.Render(" /" + f)
	}
	return box(title, lines, m.w, h, cBoxDisk)
}

func (m Model) renderDiskDetail(h int) string {
	in := m.w - 2
	it, ok := m.selectedDiskItem()
	if !ok {
		return box(sTitle.Render("detail"), nil, m.w, h, cBoxDisk)
	}
	lines := []string{
		sDim.Render("id   ") + sMain.Render(it.ID),
		sDim.Render("name ") + sMain.Render(it.Name),
	}
	var used string
	switch {
	case len(it.UsedBy) > 0:
		used = sDim.Render("used by ") + sMain.Render(strings.Join(it.UsedBy, ", "))
	case it.Detail != "":
		used = sDim.Render(map[string]string{"containers": "image ", "volumes": "driver ", "cache": "type "}[m.sectionKind()]) + sMain.Render(it.Detail)
	}
	if it.Extra != "" {
		used += sDim.Render("   " + it.Extra)
	}
	lines = append(lines, fit(used, in))
	return box(sTitle.Render("detail"), lines, m.w, h, cBoxDisk)
}
