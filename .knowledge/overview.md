# Overview

**dtop** is a terminal monitor in the style of [btop](https://github.com/aristocratos/btop), dedicated to Docker.
Single, static Go binary, no external runtime (the `docker` CLI is only needed for the `e` shell).

## Original requirements (user, 2026-10-08)
- **resources**
  - maximum resources usable by docker (CPU/RAM of the daemon: on Docker Desktop = VM)
  - resources used by docker in absolute terms
  - resources used relative to those usable by docker
  - container list with absolute and relative usage
- **disk**
  - maximum disk usable by docker
  - disk used by docker in absolute terms
  - sections: images, containers, volumes, build cache
  - for each section: name, state (active / in use / last_use), size
- actions: delete / kill / restart (in addition to start/stop/pause, prune)
- "as native as possible" ("il più nativo possibile"), Rust or Go by choice → **Go** (see ADR 0001)
- follow-up requests: "don't reinvent the wheel" ("non reinventare la ruota") if there is open source with a compatible license (ADR 0002, 0007);
  "no braille, voglio blocks o tty stile btop" (no braille, I want blocks or tty in the btop style) (ADR 0004)
- document everything in `.knowledge` (this folder)

## Status (v0.1, 2026-10-08)
Working and tested on Docker Desktop 29.5.3 (macOS arm64, API 1.54):
- **resources** view: cpu / mem / net·io boxes with historical graphs, container table, event feed
- **disk** view: capacity summary + stacked bar, 4 sections with table and detail
- actions: start/stop, restart, pause/unpause, kill, rm, force rm, logs (follow), shell, prune (6 types)
- `/` filter, sorting, compose grouping, mouse (wheel + click)
- mandatory confirmation for every destructive action

Platforms: macOS and Linux. Windows not supported (the `host` package is `//go:build unix`).
