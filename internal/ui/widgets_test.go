package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestGraphShape(t *testing.T) {
	rows := graph([]float64{0, 50, 100}, 5, 2, 100, gCPU, ModeBlock)
	if len(rows) != 2 {
		t.Fatalf("rows = %d", len(rows))
	}
	top, bottom := ansi.Strip(rows[0]), ansi.Strip(rows[1])
	// right aligned: 2 empty cols, then 0, 50, 100
	if top != "    █" || bottom != "   ██" {
		t.Fatalf("unexpected graph:\n%q\n%q", top, bottom)
	}
	for _, r := range rows {
		if strings.ContainsAny(ansi.Strip(r), "⠀⣿⡀") {
			t.Fatal("braille must never be used")
		}
	}
}

func TestTTYGlyphs(t *testing.T) {
	got := ansi.Strip(sparkline([]float64{0, 20, 45, 70, 100}, 5, 100, gCPU, ModeTTY))
	if got != " ░▒▓█" {
		t.Fatalf("tty sparkline = %q", got)
	}
}

func TestFitAndBox(t *testing.T) {
	if w := lipgloss.Width(fit(sRed.Render("hello world"), 5)); w != 5 {
		t.Fatalf("fit width %d", w)
	}
	b := box("t", []string{"a", "bb"}, 10, 4, cBorder)
	for _, l := range strings.Split(b, "\n") {
		if lipgloss.Width(l) != 10 {
			t.Fatalf("box line width %d: %q", lipgloss.Width(l), ansi.Strip(l))
		}
	}
}

func TestShortAlwaysHasUnit(t *testing.T) {
	for n, want := range map[int64]string{0: "0B", 89: "89B", 2048: "2.0K", 5 << 30: "5.0G"} {
		if got := short(n); got != want {
			t.Errorf("short(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestSelectorFollowsID(t *testing.T) {
	var s selector
	s.setIDs([]string{"a", "b", "c"}, 2)
	s.move(2, 2)
	if s.selID != "c" || s.off != 1 {
		t.Fatalf("sel=%s off=%d", s.selID, s.off)
	}
	s.setIDs([]string{"c", "a"}, 2) // re-sort: cursor stays on "c"
	if s.index() != 0 {
		t.Fatalf("index=%d", s.index())
	}
	s.setIDs([]string{"x"}, 2) // selected item vanished
	if s.selID != "x" {
		t.Fatal("must fall back to first row")
	}
}

func TestOverlayKeepsWidth(t *testing.T) {
	bg := strings.Repeat(strings.Repeat(".", 20)+"\n", 5)
	out := overlay(strings.TrimSuffix(bg, "\n"), "XX\nXX", 20, 5)
	for _, l := range strings.Split(out, "\n") {
		if lipgloss.Width(l) != 20 {
			t.Fatalf("width %d: %q", lipgloss.Width(l), l)
		}
	}
}

func TestSelectorRange(t *testing.T) {
	var s selector
	s.setIDs([]string{"a", "b", "c", "d"}, 10)
	s.extend(1, 10) // a..b
	s.extend(1, 10) // a..c
	if len(s.marked) != 3 || !s.isMarked("a") || !s.isMarked("c") {
		t.Fatalf("marked = %v", s.marked)
	}
	s.extend(-1, 10) // shrink back to a..b
	if len(s.marked) != 2 || s.isMarked("c") {
		t.Fatalf("shrink failed: %v", s.marked)
	}
	s.setIDs([]string{"b", "d"}, 10) // "a" removed, re-sorted: b stays marked
	if len(s.marked) != 1 || !s.isMarked("b") {
		t.Fatalf("after refresh: %v", s.marked)
	}
	s.clearMarks()
	if s.hasMarks() {
		t.Fatal("clear failed")
	}
}

func TestSummarize(t *testing.T) {
	if msg, err := summarize("restart 3 containers", 3, nil); err != nil || msg != "restart 3 containers: done" {
		t.Fatal(msg, err)
	}
	if _, err := summarize("kill 2 containers", 2, []string{"b: x", "a: y"}); err == nil ||
		err.Error() != "kill 2 containers: 0 ok, 2 failed (a: y; b: x)" {
		t.Fatal(err)
	}
}
