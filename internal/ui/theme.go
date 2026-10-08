package ui

import (
	"github.com/charmbracelet/lipgloss"
	colorful "github.com/lucasb-eyer/go-colorful"
)

// Palette inspired by btop's default theme.
var (
	cMain     = lipgloss.Color("#cccccc")
	cTitle    = lipgloss.Color("#ffffff")
	cDim      = lipgloss.Color("#5c5c5c")
	cBorder   = lipgloss.Color("#4a4a4a")
	cHi       = lipgloss.Color("#b54040") // hotkey highlight
	cSelBg    = lipgloss.Color("#0b3d5c")
	cSelFg    = lipgloss.Color("#ffffff")
	cMarkBg   = lipgloss.Color("#4b2f6b") // multi-selected rows
	cGreen    = lipgloss.Color("#77ca9b")
	cYellow   = lipgloss.Color("#cbc06c")
	cRed      = lipgloss.Color("#dc4c4c")
	cBlue     = lipgloss.Color("#4897d8")
	cPurple   = lipgloss.Color("#a065d9")
	cCyan     = lipgloss.Color("#56b6c2")
	cOrange   = lipgloss.Color("#e5944b")
	cBoxCPU   = lipgloss.Color("#556d59")
	cBoxMem   = lipgloss.Color("#6c6c4b")
	cBoxNet   = lipgloss.Color("#5c588d")
	cBoxProc  = lipgloss.Color("#805252")
	cBoxDisk  = lipgloss.Color("#5c7c8d")
	cBoxEvent = lipgloss.Color("#6b5c8d")
)

var (
	sMain  = lipgloss.NewStyle().Foreground(cMain)
	sTitle = lipgloss.NewStyle().Foreground(cTitle).Bold(true)
	sDim   = lipgloss.NewStyle().Foreground(cDim)
	sHi    = lipgloss.NewStyle().Foreground(cHi).Bold(true)
	sSel   = lipgloss.NewStyle().Background(cSelBg).Foreground(cSelFg).Bold(true)
	sMark  = lipgloss.NewStyle().Background(cMarkBg).Foreground(cSelFg)
	sGreen = lipgloss.NewStyle().Foreground(cGreen)
	sYel   = lipgloss.NewStyle().Foreground(cYellow)
	sRed   = lipgloss.NewStyle().Foreground(cRed)
	sBlue  = lipgloss.NewStyle().Foreground(cBlue)
	sPurp  = lipgloss.NewStyle().Foreground(cPurple)
	sCyan  = lipgloss.NewStyle().Foreground(cCyan)
	sOrng  = lipgloss.NewStyle().Foreground(cOrange)
)

// Gradient maps 0..1 to a color (precomputed, 101 steps).
type Gradient [101]lipgloss.Style

func newGradient(stops ...string) *Gradient {
	cs := make([]colorful.Color, len(stops))
	for i, s := range stops {
		cs[i], _ = colorful.Hex(s)
	}
	var g Gradient
	for i := 0; i <= 100; i++ {
		t := float64(i) / 100 * float64(len(cs)-1)
		k := int(t)
		if k >= len(cs)-1 {
			k = len(cs) - 2
		}
		c := cs[k].BlendLab(cs[k+1], t-float64(k)).Clamped()
		g[i] = lipgloss.NewStyle().Foreground(lipgloss.Color(c.Hex()))
	}
	return &g
}

// At returns the style for position f in [0,1].
func (g *Gradient) At(f float64) lipgloss.Style {
	if f < 0 {
		f = 0
	}
	if f > 1 {
		f = 1
	}
	return g[int(f*100+0.5)]
}

var (
	gCPU  = newGradient("#77ca9b", "#cbc06c", "#dc4c4c")
	gMem  = newGradient("#4897d8", "#a065d9", "#dc4c4c")
	gRx   = newGradient("#3a6ea5", "#56b6c2", "#9be3ea")
	gTx   = newGradient("#6b3fa0", "#a065d9", "#e0a0f0")
	gDisk = newGradient("#5c7c8d", "#cbc06c", "#dc4c4c")
)

// level colors a value by thresholds (used for percentages in tables).
func level(pct float64) lipgloss.Style {
	switch {
	case pct >= 80:
		return sRed
	case pct >= 50:
		return sYel
	case pct > 0:
		return sGreen
	}
	return sDim
}
