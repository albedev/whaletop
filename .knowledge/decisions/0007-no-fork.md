# ADR 0007 — New project on libraries, not a fork of an existing tool
Date: 2026-10-08 · Status: accepted

Evaluated:
- **amir20/dtop** (Rust, MIT): multi-host dashboard, monitoring only; start/stop/remove actions on the roadmap; no disk.
  ⚠ Same name "dtop" (also the Homebrew formula): if we publish, we need a different name or a namespace (see roadmap).
- **lazydocker** (Go, MIT, gocui): full manager, but no resources related to docker capacity and no disk view
  with last-use/reclaimable; gocui architecture is far from a btop-style layout.
- **DockBub** (Go, MIT, bubbletea): disk usage and actions, but pre-release (no tags, inactive since Nov 2025) and oriented toward "management" more than "monitor".
- **ctop** (Go, MIT): container metrics only, poorly maintained.
None covers the main requirements (usage relative to what docker can use, disk sections with state/last-use).
A fork would have required rewriting much of the UI. Instead, the "heavy" layer is reused through libraries
(Docker SDK, gopsutil, bubbletea, humanize). No third-party code has been copied.
