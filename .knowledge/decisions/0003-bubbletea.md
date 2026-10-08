# ADR 0003 — bubbletea + lipgloss + bubbles (v1)
Date: 2026-10-08 · Status: accepted

Alternatives evaluated: tview/tcell (ready-made widgets but a less controllable look), gocui (used by lazydocker), termui (stalled).
Chose bubbletea (MIT): Elm architecture (state → View) suits a dashboard that redraws on every sample, async commands for I/O, mouse support, `tea.ExecProcess` to suspend the TUI and open the container's shell.
From bubbles we only use `textinput` (filter) and `viewport` (log). The table is our own because `bubbles/table` (v1.0.0, `table.go` lines 422/435) truncates cells with
`runewidth.Truncate`, which counts the bytes of ANSI escape sequences as width: colored cells and sparklines would break.
