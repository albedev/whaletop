package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

type column struct {
	id    string
	title string
	w     int // fixed width, or minimum width when flex
	right bool
	flex  bool
}

// renderTable draws a header plus rows into exactly height lines of width cells.
// Cells may contain ANSI styles; the cursor row and marked rows are re-rendered plain
// on a highlight (cell styles end with resets that would cut a background short).
func renderTable(cols []column, rows [][]string, sel, off, width, height, sortCol int, desc bool, marked func(int) bool) []string {
	fixed, flex := 0, 0
	for _, c := range cols {
		fixed += c.w + 1
		if c.flex {
			flex++
		}
	}
	extra := max(width-fixed+1, 0)
	ws := make([]int, len(cols))
	for i, c := range cols {
		ws[i] = c.w
		if c.flex && flex > 0 {
			add := extra / flex
			ws[i] += add
			extra -= add
			flex--
		}
	}

	line := func(cells []string, header bool) string {
		var b strings.Builder
		for i, c := range cols {
			if i > 0 {
				b.WriteByte(' ')
			}
			cell := ""
			if i < len(cells) {
				cell = cells[i]
			}
			if c.right {
				b.WriteString(fitLeft(cell, ws[i]))
			} else {
				b.WriteString(fit(cell, ws[i]))
			}
		}
		_ = header
		return fit(b.String(), width)
	}

	out := make([]string, 0, height)
	hdr := make([]string, len(cols))
	for i, c := range cols {
		t := c.title
		if i == sortCol {
			if desc {
				t += "↓"
			} else {
				t += "↑"
			}
			hdr[i] = sTitle.Render(t)
		} else {
			hdr[i] = sDim.Bold(true).Render(t)
		}
	}
	out = append(out, line(hdr, true))

	for i := off; i < len(rows) && len(out) < height; i++ {
		l := line(rows[i], false)
		switch {
		case i == sel:
			l = sSel.Render(fit(ansi.Strip(l), width))
		case marked != nil && marked(i):
			l = sMark.Render(fit(ansi.Strip(l), width))
		}
		out = append(out, l)
	}
	for len(out) < height {
		out = append(out, "")
	}
	return out
}

// selector keeps the cursor on an item id, so refreshes and re-sorts don't move it.
// It also holds the multi-selection: a range from anchor to cursor (ctrl/shift+↑↓),
// stored as a set of ids so it survives re-sorts and refreshes.
type selector struct {
	ids    []string
	selID  string
	off    int
	anchor string
	marked map[string]bool
}

func (s *selector) indexOf(id string) int {
	for i, x := range s.ids {
		if x == id {
			return i
		}
	}
	return -1
}

// extend moves the cursor by d and marks every row between the anchor and the cursor.
func (s *selector) extend(d, visible int) {
	if len(s.ids) == 0 {
		return
	}
	if s.anchor == "" || s.indexOf(s.anchor) < 0 {
		s.anchor = s.selID
	}
	s.move(d, visible)
	a, c := s.indexOf(s.anchor), s.index()
	s.marked = make(map[string]bool, max(a, c)-min(a, c)+1)
	for i := min(a, c); i <= max(a, c); i++ {
		s.marked[s.ids[i]] = true
	}
}

func (s *selector) clearMarks()             { s.anchor, s.marked = "", nil }
func (s *selector) hasMarks() bool          { return len(s.marked) > 0 }
func (s *selector) isMarked(id string) bool { return s.marked[id] }

func (s *selector) index() int { return max(s.indexOf(s.selID), 0) }

func (s *selector) setIDs(ids []string, visible int) {
	s.ids = ids
	if len(ids) == 0 {
		s.selID, s.off = "", 0
		return
	}
	if s.indexOf(s.selID) < 0 {
		s.selID = ids[0]
	}
	// forget marked rows that disappeared (removed, filtered out)
	for id := range s.marked {
		if s.indexOf(id) < 0 {
			delete(s.marked, id)
		}
	}
	if s.anchor != "" && s.indexOf(s.anchor) < 0 {
		s.anchor = ""
	}
	if len(s.marked) == 0 {
		s.clearMarks()
	}
	s.clamp(visible)
}

func (s *selector) move(d, visible int) {
	if len(s.ids) == 0 {
		return
	}
	i := min(max(s.index()+d, 0), len(s.ids)-1)
	s.selID = s.ids[i]
	s.clamp(visible)
}

func (s *selector) click(row, visible int) {
	if i := s.off + row; i >= 0 && i < len(s.ids) {
		s.selID = s.ids[i]
		s.clamp(visible)
	}
}

func (s *selector) clamp(visible int) {
	if visible < 1 {
		visible = 1
	}
	i := s.index()
	if i < s.off {
		s.off = i
	}
	if i >= s.off+visible {
		s.off = i - visible + 1
	}
	s.off = max(0, min(s.off, len(s.ids)-visible))
}
