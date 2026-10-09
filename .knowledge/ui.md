# UI

Framework: bubbletea v1 + lipgloss v1 + bubbles v1 (textinput, viewport). AltScreen, mouse cell-motion (`--no-mouse` to disable it).

## Layout
```
header: whaletop · version · OS · arch · storage driver           1resources 2disk  now  interval
── resources ──────────────────────────────────────────────────────────────────────────
╭cpu (50%)──────────────╮╭mem (25%)────────╮╭net·io (25%)──╮   height 35% (8..14 rows)
│cores used / NCPU  %   ││limit / used / ■■ ││rx graph / tx │   below 100 columns: no net·io
╰───────────────────────╯╰──────────────────╯╰──────────────╯
╭containers N running · N paused · N stopped [filters]──────╮   table, columns disappear when narrow:
│NAME STATE CPU% /DKR CPU-HIST MEM /DKR /LIM NET IO PIDS UP│   io, graph, pids, /lim, net, /dkr, up
╰──────────────────────────────────────────────────────────╯
╭events (only if ≥ 36 body rows)───────────────────────────╮
footer: context keys + "m graph:block|tty" (always visible, moves to the front if space is short) / status messages 6s
── disk ───────────────────────────────────────────────────────────────────────────────
╭disk: capacity, used %, reclaimable, stacked bar per section, legend, host allocated/available╮
╭i images │ c containers │ v volumes │ b build cache  (tabs in the title)╮  NAME STATE USED-BY SIZE [SHARED] CREATED LAST-USED DETAIL
╭detail: full id, name, used by (only if ≥ 26 rows)╮
```
Minimum 60x12. Centered overlays (confirm, menu, help) composed with `overlay()` (ANSI-aware, x/ansi).

## Keys
Global: `1`/`2` views · `↑↓ pgup pgdn home end` (clear the multi-selection)
· `ctrl+↑↓` or `shift+↑↓` range multi-selection · `←→` sort column · `S` invert · `/` filter (`enter` applies, `esc` clears)
· `esc` first removes the selection, then the filter · `enter`/`space` action menu · `m` graphs block/tty · `+`/`-` interval (0.5–10s) · `ctrl+r`/`F5` refresh · `?`/`h` help · `q` quit.
Resources: `t` start/stop · `r` restart · `p` pause/unpause · `k` kill · `d`/`delete` rm · `D` force rm · `l` log · `e` shell · `a` show stopped · `g` group by compose.
Disk: `tab`/`shift+tab` or `i c v b` sections · `d` rm · `D` force rm · `P` prune menu · `u` reclaimable only · `l` log (container section).
Log: `esc`/`q`/`l` close · `f` follow · `g`/`G` start/end · arrows/pgup/wheel scroll.

## Multi-selection
Range anchor→cursor as in file managers: `ctrl/shift+↓` extends, `ctrl/shift+↑` shrinks/extends upward.
Stored as a **set of IDs** in `selector.marked` (survives refresh and re-sort; rows that disappear are removed).
Selection is separate per view and per disk section. Marked rows: purple background (`sMark`), text without colors
(cell styles end with ANSI resets that would interrupt the background); the cursor stays blue.
Box title: "N selected" (+ total size in disk). Mouse clicks and plain arrows clear it.
Actions on selection:
- resources: `t r p k d D` and the `enter` menu (start/stop/restart/pause/unpause/kill/rm/force) on all selected;
  containers for which the action makes no sense are skipped (e.g. kill on stopped ones) and the dialog says so.
  Runs in parallel (max 8), result summarized: "kill 3 containers: 2 ok, 1 failed (…)".
- disk: `d`/`D` and menu; removal is **sequential** (images may depend on each other).
- `l` (log) and `e` (shell) act only on the cursor row.
The selection is cleared after the `y` confirmation or immediately for non-destructive actions.

⚠ macOS: `ctrl+↑/↓` by default are Mission Control / App Exposé and do not reach the terminal
(System Settings → Keyboard → Keyboard Shortcuts → Mission Control). That is why `shift+↑/↓` also exists.

**Confirmations**: kill, rm, force rm, every removal from disk and every prune ask for `y`. Default = cancel (also `enter`).
Start/stop/restart/pause do not ask for confirmation (reversible).

## Graphs (no braille, ADR 0004)
- `block`: `▁▂▃▄▅▆▇█`, 8 levels per cell; meter with `■`
- `tty`: `░▒▓█`, 4 levels; meter `█`/`░`
Area chart with the latest sample on the right, color per **row** from the gradient (low cold → high hot) like btop.
CPU and MEM use a fixed 0–100% scale of the docker capacity; network autoscales to the visible peak (min 1 KiB/s).
Sparkline per container (`CPU HIST`) autoscales to its own peak (min 1%).

## Colors
Palette inspired by btop's default theme (`theme.go`). Gradients precomputed at 101 steps in Lab space (go-colorful).
Percentages in the table: green >0, yellow ≥50, red ≥80. States: running/healthy green, paused/starting yellow,
restarting orange, exited(≠0)/oom-killed/dead/unhealthy red, exited(0) grey. `↻N` = restart count.
UP column: running = uptime, stopped = `-3h` (stopped for 3 hours).
