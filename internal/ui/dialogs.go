package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// confirmState guards every destructive action: nothing is deleted/killed without "y".
type confirmState struct {
	title, body string
	run         tea.Cmd
	onYes       func(*Model) // optional, e.g. clear the multi-selection
}

type menuItem struct {
	key, label string
	run        func(*Model) tea.Cmd
}

type menuState struct {
	title string
	items []menuItem
	sel   int
}

func (m Model) confirmKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "y", "Y":
		run, onYes := m.confirm.run, m.confirm.onYes
		m.setStatus(m.confirm.title+" …", false)
		m.confirm = nil
		if onYes != nil {
			onYes(&m)
		}
		return m, run
	case "n", "N", "esc", "q", "enter":
		m.confirm = nil
		m.setStatus("cancelled", false)
	}
	return m, nil
}

func (m Model) menuKey(key string) (tea.Model, tea.Cmd) {
	mn := m.menu
	switch key {
	case "esc", "q":
		m.menu = nil
		return m, nil
	case "up":
		mn.sel = (mn.sel + len(mn.items) - 1) % len(mn.items)
		return m, nil
	case "down":
		mn.sel = (mn.sel + 1) % len(mn.items)
		return m, nil
	case "enter", " ":
		m.menu = nil
		return m, mn.items[mn.sel].run(&m)
	}
	for _, it := range mn.items {
		if it.key == key {
			m.menu = nil
			return m, it.run(&m)
		}
	}
	return m, nil
}

func dialog(title string, lines []string, border lipgloss.Color) string {
	w := lipgloss.Width(title) + 6
	for _, l := range lines {
		w = max(w, lipgloss.Width(l)+4)
	}
	w = min(w, 80)
	padded := make([]string, 0, len(lines)+2)
	padded = append(padded, "")
	for _, l := range lines {
		padded = append(padded, " "+l)
	}
	padded = append(padded, "")
	return box(title, padded, w, len(padded)+2, border)
}

func (m Model) renderConfirm() string {
	lines := wrap(m.confirm.body, 60)
	lines = append(lines, "", sRed.Bold(true).Render("y")+sMain.Render(" confirm    ")+sTitle.Render("n/esc")+sMain.Render(" cancel"))
	return dialog(sRed.Bold(true).Render(" "+m.confirm.title+" "), lines, cRed)
}

func (m Model) renderMenu() string {
	var lines []string
	for i, it := range m.menu.items {
		l := sHi.Render(fit(it.key, 2)) + " " + sMain.Render(it.label)
		if i == m.menu.sel {
			l = sSel.Render(fit(it.key+"  "+it.label, 30))
		} else {
			l = fit(l, 30)
		}
		lines = append(lines, l)
	}
	return dialog(sTitle.Render(" "+m.menu.title+" "), lines, cBlue)
}

var helpText = [][2]string{
	{"1 / 2", "resources / disk view"},
	{"↑ ↓ pgup pgdn", "move cursor (mouse wheel and click work too)"},
	{"ctrl/shift+↑↓", "select multiple rows: actions apply to all of them"},
	{"← →  S", "change sort column / reverse sort"},
	{"/  esc", "filter by name, image, id / esc: clear selection, then filter"},
	{"enter", "actions menu for the selected item"},
	{"m", "graph style: block ▁▂▃▅▇ / tty ░▒▓█"},
	{"+ -", "sampling interval"},
	{"ctrl+r", "refresh now (also rescans disk)"},
	{"q", "quit"},
	{"", ""},
	{"resources", ""},
	{"t r p", "start-stop toggle / restart / pause-unpause"},
	{"k", "kill (SIGKILL, asks confirmation)"},
	{"d  D", "remove / force remove (asks confirmation)"},
	{"l  e", "logs / shell (docker exec)"},
	{"a  g", "show stopped containers / group by compose project"},
	{"", ""},
	{"disk", ""},
	{"tab  i c v b", "next section / images containers volumes build-cache"},
	{"d  D", "remove / force remove selected item (asks confirmation)"},
	{"P", "prune menu"},
	{"u", "show only reclaimable items"},
}

func (m Model) renderHelp() string {
	var lines []string
	for _, h := range helpText {
		if h[1] == "" {
			lines = append(lines, sTitle.Render(h[0]))
			continue
		}
		lines = append(lines, sHi.Render(fit(h[0], 14))+" "+sMain.Render(h[1]))
	}
	if m.updLatest != "" {
		lines = append(lines, "", sYel.Bold(true).Render("⬆ whaletop "+m.updLatest+" is available: ")+sMain.Render(m.updHint))
	}
	lines = append(lines, "", sDim.Render("CPU%: docker stats style, 100% = 1 core. /DKR: share of what docker can use."))
	lines = append(lines, sDim.Render("/LIM: share of the container's own memory limit."))
	return dialog(sTitle.Render(" help "), lines, cBorder)
}

func wrap(s string, w int) []string {
	var out []string
	line := ""
	for _, word := range strings.Fields(s) {
		if line != "" && len(line)+1+len(word) > w {
			out = append(out, sMain.Render(line))
			line = word
			continue
		}
		if line != "" {
			line += " "
		}
		line += word
	}
	if line != "" {
		out = append(out, sMain.Render(line))
	}
	return out
}
