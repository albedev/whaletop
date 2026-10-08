package ui

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// GraphMode selects the glyph set, like btop's graph_symbol option.
// Braille is intentionally not offered (project decision, see .knowledge).
type GraphMode int

const (
	ModeBlock GraphMode = iota // ▁▂▃▄▅▆▇█ : 8 vertical levels per cell
	ModeTTY                    // ░▒▓█    : 4 levels, works on any terminal/font
)

func (m GraphMode) String() string {
	if m == ModeTTY {
		return "tty"
	}
	return "block"
}

var (
	blockLevels = []rune{' ', '▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	ttyLevels   = []rune{' ', '░', '▒', '▓', '█'}
)

func (m GraphMode) glyph(fill float64) rune {
	lv := blockLevels
	if m == ModeTTY {
		lv = ttyLevels
	}
	if fill <= 0 {
		return lv[0]
	}
	if fill >= 1 {
		return lv[len(lv)-1]
	}
	n := len(lv) - 1
	i := int(math.Ceil(fill * float64(n)))
	if i < 1 {
		i = 1
	}
	return lv[i]
}

// graph renders the last w values as an h-rows high area chart, newest on the right.
// Each row gets its own gradient color (bottom = cold, top = hot) like btop.
func graph(values []float64, w, h int, maxV float64, g *Gradient, mode GraphMode) []string {
	if w <= 0 || h <= 0 {
		return nil
	}
	if maxV <= 0 {
		maxV = 1
	}
	vals := make([]float64, w)
	off := w - len(values)
	for i := range vals {
		if j := i - off; j >= 0 && j < len(values) {
			vals[i] = values[j]
		}
	}
	rows := make([]string, h)
	buf := make([]rune, w)
	for r := 0; r < h; r++ {
		fromBottom := float64(h - 1 - r)
		for i, v := range vals {
			scaled := v / maxV * float64(h)
			buf[i] = mode.glyph(scaled - fromBottom)
		}
		rows[r] = g.At((fromBottom + 0.5) / float64(h)).Render(string(buf))
	}
	return rows
}

// sparkline is a single-row graph.
func sparkline(values []float64, w int, maxV float64, g *Gradient, mode GraphMode) string {
	if w <= 0 {
		return ""
	}
	if maxV <= 0 {
		maxV = 1
	}
	var b strings.Builder
	off := w - len(values)
	for i := 0; i < w; i++ {
		j := i - off
		if j < 0 || j >= len(values) {
			b.WriteRune(' ')
			continue
		}
		f := values[j] / maxV
		b.WriteString(g.At(f).Render(string(mode.glyph(f))))
	}
	return b.String()
}

// meter is a btop-like horizontal bar; each filled cell is colored by its position.
func meter(pct float64, w int, g *Gradient, mode GraphMode) string {
	if w <= 0 {
		return ""
	}
	on, off := "■", "■"
	if mode == ModeTTY {
		on, off = "█", "░"
	}
	n := int(math.Round(pct / 100 * float64(w)))
	n = max(0, min(n, w))
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString(g.At(float64(i) / float64(max(w-1, 1))).Render(on))
	}
	b.WriteString(sDim.Render(strings.Repeat(off, w-n)))
	return b.String()
}

// segment is a slice of a stacked bar.
type segment struct {
	value int64
	style lipgloss.Style
}

// stackedBar renders segments proportionally to total over w cells.
func stackedBar(segs []segment, total int64, w int, mode GraphMode) string {
	if w <= 0 || total <= 0 {
		return strings.Repeat(" ", max(w, 0))
	}
	on, off := "█", "░"
	if mode == ModeBlock {
		on, off = "■", "■"
	}
	var b strings.Builder
	used := 0
	var acc int64
	for _, s := range segs {
		acc += s.value
		end := int(math.Round(float64(acc) / float64(total) * float64(w)))
		end = min(end, w)
		if n := end - used; n > 0 {
			b.WriteString(s.style.Render(strings.Repeat(on, n)))
			used = end
		}
	}
	if used < w {
		b.WriteString(sDim.Render(strings.Repeat(off, w-used)))
	}
	return b.String()
}

// box draws a rounded border with a title embedded in the top edge, btop style:
// ╭─┐¹cpu┌──────╮
func box(title string, lines []string, w, h int, border lipgloss.Color) string {
	if w < 4 || h < 2 {
		return ""
	}
	bs := lipgloss.NewStyle().Foreground(border)
	inner := w - 2
	top := bs.Render("╭─")
	used := 2
	if title != "" {
		t := bs.Render("┐") + title + bs.Render("┌")
		tw := lipgloss.Width(t)
		if used+tw < w-1 {
			top += t
			used += tw
		}
	}
	top += bs.Render(strings.Repeat("─", max(w-1-used, 0)) + "╮")

	out := make([]string, 0, h)
	out = append(out, top)
	side := bs.Render("│")
	for i := 0; i < h-2; i++ {
		l := ""
		if i < len(lines) {
			l = lines[i]
		}
		out = append(out, side+fit(l, inner)+side)
	}
	out = append(out, bs.Render("╰"+strings.Repeat("─", inner)+"╯"))
	return strings.Join(out, "\n")
}

// fit truncates or right-pads s (ANSI aware) to exactly w cells.
func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	sw := lipgloss.Width(s)
	if sw > w {
		return ansi.Truncate(s, w, "…")
	}
	return s + strings.Repeat(" ", w-sw)
}

// fitLeft right-aligns s in w cells.
func fitLeft(s string, w int) string {
	sw := lipgloss.Width(s)
	if sw > w {
		return ansi.Truncate(s, w, "…")
	}
	return strings.Repeat(" ", w-sw) + s
}

// spread puts left and right on one line of width w.
func spread(left, right string, w int) string {
	gap := w - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return fit(left, w)
	}
	return left + strings.Repeat(" ", gap) + right
}

// overlay draws fg centered on top of bg (both multi-line strings).
func overlay(bg, fg string, width, height int) string {
	bl := strings.Split(bg, "\n")
	for len(bl) < height {
		bl = append(bl, "")
	}
	fl := strings.Split(fg, "\n")
	fw := 0
	for _, l := range fl {
		fw = max(fw, lipgloss.Width(l))
	}
	x := max((width-fw)/2, 0)
	y := max((height-len(fl))/2, 0)
	for i, l := range fl {
		if y+i >= len(bl) {
			break
		}
		row := fit(bl[y+i], width)
		left := ansi.Truncate(row, x, "")
		right := ansi.TruncateLeft(row, x+fw, "")
		bl[y+i] = left + fit(l, fw) + right
	}
	return strings.Join(bl, "\n")
}
