# Dependencies

Direct (go.mod): bubbletea v1.3.10, bubbles v1.0.0, lipgloss v1.1.0, charmbracelet/x/ansi, go-colorful,
moby/moby/client v0.6.1, moby/moby/api v1.56.1, gopsutil/v4 v4.26.9, go-humanize v1.1.0, golang.org/x/sys.

Licenses of everything that ends up in the binary: only **MIT, Apache-2.0, BSD-3** (list in `THIRD_PARTY_NOTICES.md`,
regenerable with `make licenses`). No NOTICE file to propagate. All compatible with whaletop's license (ADR 0009):
permissive licenses allow combining their code with terms more restrictive for ours, provided their copyright
notices are kept.

Why bubbletea **v1** and not v2: v2 (`charm.land/...`) changes the API extensively; v1 is stable and very widely used.
Evaluate migration only for concrete needs (e.g. faster rendering).

Size: static binary ~8.5 MB (`-s -w -trimpath`, `CGO_ENABLED=0`). The moby SDK brings along OpenTelemetry (indirect).
