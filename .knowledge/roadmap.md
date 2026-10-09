# Roadmap and ideas

Proposals raised during development (not yet done), in order of estimated value:
1. **Alerts/thresholds**: highlight/notify containers above X% of their memory limit, restart loops, OOM.
2. **Network view**: Docker networks, connected containers, exposed ports (`docker network ls` + inspect).
3. **Compose view**: projects as collapsible groups with group actions (restart/stop the project).
4. **Container details**: panel with dedicated historical CPU/MEM charts, env, mounts, ports, limits (`enter` → "inspect").
5. **Multi-selection** for batch actions (rm of multiple images, kill of multiple containers).
6. **Config file** (`~/.config/whaletop/config.toml`): theme, interval, columns, chart mode, confirmations.
7. **Alternative themes** (like btop: nord, gruvbox, monochrome).
8. **Disk capacity for OrbStack / Rancher Desktop / Podman** (Podman exposes a Docker-compatible API).
9. **ssh://** and TLS for remote daemons (the SDK supports TLS via `client.WithTLSClientConfig`; ssh requires the CLI's connhelper).
10. **Export** of JSON/Prometheus snapshots (`--json` one-shot) for scripting.
11. **More accurate image last-used**: use historical events (`/events?since=`) to remember the last container created from an image even after it has been removed; persist in `~/.cache/whaletop`.
12. ~~**Name**: `dtop` collides with amir20/dtop (also on Homebrew).~~ Done: renamed to `whaletop` with `wtop` alias (ADR 0010).
13. CI (GitHub Actions: vet, test, cross build, release with goreleaser).
