# .knowledge — whaletop project memory

This folder collects **everything that cannot be understood by reading the code alone**:
why certain choices were made, formulas, known limits, pitfalls already encountered.
It serves anyone resuming the project (people or AI agents) so they don't have to start from scratch.

## Golden rule
Every non-trivial change updates at least one of these files in the same commit:
- new architectural or library choice → new ADR in `decisions/`
- subtle bug fixed → `gotchas.md`
- new feature / key → `ui.md` + `changelog.md`
- formula or heuristic changed → `metrics.md`

## Index
| File | Contents |
|---|---|
| [overview.md](overview.md) | What whaletop is, original requirements, current status |
| [architecture.md](architecture.md) | Packages, data flow, bubbletea loop, refresh cadences |
| [metrics.md](metrics.md) | CPU/memory/network/IO formulas, disk capacity, "last used" and state heuristics |
| [docker-api.md](docker-api.md) | Endpoints and SDK options used, quirks of the moby SDK v29+ |
| [ui.md](ui.md) | Layout, keys, charts, colors, widgets |
| [release.md](release.md) | How releases are built and published (GoReleaser, Homebrew tap, deploy key), install.sh, update check / self-update |
| [testing.md](testing.md) | How to test: unit tests, `--dump`, tmux, fixtures, `WHALETOP_DEBUG` |
| [gotchas.md](gotchas.md) | Bugs already encountered and how to avoid them |
| [dependencies.md](dependencies.md) | Dependencies and licenses |
| [roadmap.md](roadmap.md) | Proposed ideas, things to do |
| [changelog.md](changelog.md) | Change history |
| [decisions/](decisions/) | ADRs: one decision per file, numbered |
