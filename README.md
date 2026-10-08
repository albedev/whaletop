# dtop

A [btop](https://github.com/aristocratos/btop)-style terminal monitor for Docker.
It shows how much CPU, memory and disk Docker **can** use, how much it is using and in what proportion,
container by container and object by object, and lets you act right away (stop, kill, restart, rm, prune…).

```
 dtop docker 29.5.3 · Docker Desktop · aarch64 · overlayfs                      1resources 2disk  12:30:07 1s
╭─┐cpu┌─────────────────────────────────╮╭─┐mem┌──────────────────╮╭─┐net·io┌──────────────╮
│docker 1.00 / 8 cores           12.5%││limit 16 GiB host 64 GiB││↓ rx 143 B/s            │
│host 16 cores · docker may use 8 (50%)││used  81 MiB        0.7%││                     ▅▅▅│
│                                  ▆▆▆││■■■■■■■■■■■■■■■■■■■■■■■■││↑ tx 143 B/s            │
│                                  ███││free  16 GiB            ││io r 0 B/s  w 0 B/s     │
╰─────────────────────────────────────╯╰────────────────────────╯╰────────────────────────╯
╭─┐containers 4 running · 0 paused · 1 stopped┌────────────────────────────────────────────╮
│NAME              STATE      CPU%↓  /DKR CPU HIST     MEM  /DKR  /LIM   NET/s ↓/↑   UP │
│dtop-test-burn    running    100.0  12.5 ██████████  536K   0.0     -       0B/0B   1m │
│dtop-test-redis   running      0.2   0.0 ▁▁▁▁▁▁▁▁▁▁  9.3M   0.1     4       0B/0B   1m │
│dtop-test-exited  exited(3)      -     -                -     -     -           -  -1m │
╰───────────────────────────────────────────────────────────────────────────────────────╯
```


## Features
- **Resources**: the CPU and RAM Docker can use (on Docker Desktop: the VM) compared with the host; total usage,
  absolute and in %; network and IO. Per container: CPU% (100% = 1 core), share of Docker's capacity, memory
  (absolute / % of Docker / % of its own limit), network, IO, PIDs, uptime, health, restart count; live event feed.
- **Disk**: Docker's disk capacity (Docker Desktop, colima, native Linux or `--disk-limit`), usage and % of capacity,
  reclaimable space, per-category bar; **images, containers, volumes, build cache** sections with state
  (active / in use / unused / dangling), who uses it, size, shared size, created, last used.
- **Actions**: start/stop, restart, pause, kill, rm, force rm, removal of images/volumes/cache records,
  prune (stopped containers, dangling or unused images, anonymous or all volumes, build cache), streaming logs,
  shell into a container. Destructive actions always ask for confirmation.
- **Multi-selection**: `ctrl+↑/↓` (or `shift+↑/↓`) selects a range of rows; actions apply to all of them.
- **Block** (`▁▂▃▅▇█`) or **tty** (`░▒▓█`) graphs, toggled with `m`. Filter `/`, sort `←→`, compose grouping `g`, mouse.

## Build
Requires Go ≥ 1.26.
```
make build        # → bin/dtop
make install      # → ~/.local/bin/dtop (override with PREFIX=/usr/local)
make cross        # darwin/linux × amd64/arm64
```

## Usage
```
dtop                       # endpoint resolved like the docker CLI: DOCKER_HOST, docker context, well-known sockets
dtop --graph tty           # ░▒▓█ graphs
dtop --interval 2s         # sampling interval (also + / - at runtime)
dtop --disk-limit 100G     # force disk capacity (remote daemons or unrecognized setups)
dtop --host unix:///var/run/docker.sock
dtop --no-mouse            # leave text selection to the terminal
```
Press `?` for the full key list.

> macOS: `ctrl+↑/↓` are bound to Mission Control by default and never reach the terminal; use `shift+↑/↓`
> or disable them in System Settings → Keyboard → Keyboard Shortcuts → Mission Control.

Platforms: macOS and Linux (Docker Desktop, colima, OrbStack, native Docker Engine). Windows is not supported.

## Developer documentation
Decisions, formulas, architecture and known pitfalls live in [`.knowledge/`](.knowledge/README.md).

## License
[MIT No-Sale v1.0](LICENSE) — *source-available*, not OSI-approved.
In short (the binding text is `LICENSE`):
- you may use, modify, fork, build derivative works and contribute, including inside companies;
- you may **not** sell it or distribute it for a fee, neither dtop nor any derivative work
  (including sold bundles and paid hosted services based on dtop);
- derivative works must be distributed free of charge, with source code, under the same license.

Third-party libraries keep their own licenses: [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
