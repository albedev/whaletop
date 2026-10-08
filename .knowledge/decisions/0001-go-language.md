# ADR 0001 — Go instead of Rust
Date: 2026-10-08 · Status: accepted

**Context.** Request: "Rust or Go, as native as possible". Both produce native binaries without a VM.

**Decision.** Go.

**Reasons.**
- Docker itself is written in Go: the official SDK (`moby/moby/client`) is Go, always aligned with the Engine API.
  In Rust one would use `bollard`, excellent but third-party.
- Single static binary with `CGO_ENABLED=0`, trivial cross-compilation (`make cross`).
- Mature TUI ecosystem (bubbletea/lipgloss).
- Go toolchain already present on the user's machine; Rust is not.

**Costs.** GC (irrelevant for a monitor that samples at 1 Hz); binary ~8.5 MB instead of ~3–5 MB in Rust.
