# Changelog

## Unreleased
- `install.sh` one-line installer (latest or pinned release, sha256 check, `~/.local/bin`, `wtop` alias, never
  overwrites a foreign `wtop`); tested in CI on Linux and macOS together with shellcheck.
- `--version` shows the module version for `go install …@vX.Y.Z` builds (was `dev`).

## v0.3.0 — 2026-10-09
- Renamed from `dtop` to **whaletop** (module `github.com/albedev/whaletop`, binary `whaletop`), see ADR 0010.
- `wtop` short alias: symlink installed by `make install` (and by the Homebrew formula).
- Debug env var renamed `DTOP_DEBUG` → `WHALETOP_DEBUG`.
- `make install` / `make uninstall` targets (default `PREFIX=~/.local`).
- Release pipeline: GoReleaser + GitHub Actions on tags `v*` (binaries, .deb/.rpm, checksums), Homebrew cask in
  `albedev/homebrew-tap`, CI workflow (vet/test/build on Linux and macOS). See release.md and ADR 0011.

## v0.2.0 — 2026-10-08
- Range multi-selection with `ctrl+↑/↓` (alias `shift+↑/↓`) in both views; batch actions
  (start/stop/restart/pause/unpause/kill/rm/force rm on containers, rm/force rm on images/containers/volumes/cache).
- `esc` removes the selection first, then the filter. Confirmation dialog with a list of targets and total freed space.

## v0.1.0 — 2026-10-08
First version.
- Resources view: cpu/mem/net·io with historical charts, container table (docker-style CPU%, docker quota, memory vs capacity and vs limit, network, IO, pids, uptime), live event feed.
- Disk view: detected capacity (Docker Desktop, colima, statfs, override), used, reclaimable, stacked bar, sections for images/containers/volumes/build cache with status, used by, size, shared, created, last used.
- Actions: start/stop, restart, pause/unpause, kill, rm, force rm, rm images/volumes/cache records, 6 types of prune,
  streaming logs, shell via `docker exec`.
- block/tty charts (no braille), `m` key; filter, sort, compose grouping, mouse.
- `--dump` for rendering without a TTY, `WHALETOP_DEBUG` for logs.
- MIT No-Sale v1.0 license.
