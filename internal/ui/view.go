package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.w == 0 || m.h == 0 {
		return ""
	}
	if m.w < 60 || m.h < 12 {
		return sYel.Render(fmt.Sprintf("terminal too small (%dx%d), need at least 60x12", m.w, m.h))
	}
	if m.logs != nil {
		return m.renderLogs() + "\n" + m.footer()
	}

	lines := []string{m.header()}
	if m.view == viewDisk {
		lines = append(lines, m.viewDisk()...)
	} else {
		lines = append(lines, m.viewResources()...)
	}
	for len(lines) < m.h-1 {
		lines = append(lines, "")
	}
	lines = lines[:m.h-1]
	lines = append(lines, m.footer())
	screen := strings.Join(lines, "\n")

	switch {
	case m.confirm != nil:
		screen = overlay(screen, m.renderConfirm(), m.w, m.h)
	case m.menu != nil:
		screen = overlay(screen, m.renderMenu(), m.w, m.h)
	case m.help:
		screen = overlay(screen, m.renderHelp(), m.w, m.h)
	}
	return screen
}

func (m Model) header() string {
	i := m.col.Info()
	left := sTitle.Render(" whaletop ") + sDim.Render(fmt.Sprintf("docker %s · %s · %s · %s", i.Version, i.OS, i.Arch, i.Driver))
	tab := func(id viewID, key, label string) string {
		if m.view == id {
			return sHi.Render(key) + sSel.Render(label)
		}
		return sHi.Render(key) + sDim.Render(label)
	}
	tabs := tab(viewResources, "1", "resources") + " " + tab(viewDisk, "2", "disk")
	right := tabs + sDim.Render("  "+time.Now().Format("15:04:05")+" ") + sMain.Render(m.opt.Interval.String()+" ")
	return spread(left, right, m.w)
}

func (m Model) footer() string {
	if m.filtering {
		return fit(m.filter.View()+sDim.Render("  enter apply · esc clear"), m.w)
	}
	if m.status != "" && time.Since(m.statusAt) < 6*time.Second {
		st := sGreen
		if m.statusErr {
			st = sRed
		}
		return fit(" "+st.Render(m.status), m.w)
	}
	var keys [][2]string
	switch {
	case m.logs != nil:
		keys = [][2]string{{"esc", "close"}, {"f", "follow"}, {"↑↓", "scroll"}}
	case m.view == viewDisk:
		keys = [][2]string{{"tab", "section"}, {"ctrl+↑↓", "select"}, {"←→", "sort"}, {"enter", "menu"}, {"d", "remove"}, {"P", "prune"}, {"u", "reclaimable"}, {"/", "filter"}, {"?", "help"}, {"q", "quit"}}
	default:
		keys = [][2]string{{"ctrl+↑↓", "select"}, {"enter", "menu"}, {"t", "start/stop"}, {"r", "restart"}, {"k", "kill"}, {"d", "rm"}, {"l", "logs"}, {"e", "shell"}, {"←→", "sort"}, {"a", "all"}, {"/", "filter"}, {"?", "help"}, {"q", "quit"}}
	}
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(sHi.Render(k[0]) + sDim.Render(" "+k[1]+"  "))
	}
	hints := b.String()
	if m.logs != nil {
		return fit(" "+hints, m.w)
	}
	graphKey := sHi.Render("m") + sDim.Render(" graph:") + sMain.Render(m.opt.Mode.String())
	// the graph toggle must stay visible on narrow terminals: move it first when space is short
	if lipgloss.Width(hints+graphKey)+1 > m.w {
		return fit(" "+graphKey+"  "+hints, m.w)
	}
	return fit(" "+hints+graphKey, m.w)
}
