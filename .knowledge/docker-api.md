# Docker Engine API: what we use and quirks

SDK: `github.com/moby/moby/client` v0.6.1 + `github.com/moby/moby/api` v1.56.1.
Since Docker v29 (Nov 2025) `github.com/docker/docker` is **deprecated**: client and types are separate modules,
versioned independently (tags `client/vX`, `api/vX`). The `docker-v29.x` tags of the moby repo must NOT be used with `go get`.
All v29+ methods take an `XxxOptions` struct and return an `XxxResult` (even when empty).

Connection: `client.New(client.WithHost(ep), client.WithAPIVersionNegotiation())`.
Tested against Docker Desktop 29.5.3, API 1.54 (negotiated down from client 1.56).

| Use | SDK method | Notes |
|---|---|---|
| capacity | `Info` | `NCPU`, `MemTotal`, `DockerRootDir`, `OperatingSystem` ("Docker Desktop" → VM detection) |
| list | `ContainerList{All:true}` | every tick |
| details | `ContainerInspect` | cached; StartedAt/FinishedAt are **strings** RFC3339Nano (zero = "0001-01-01…") |
| stats | `ContainerStats{Stream:false}` | one-shot: no 1s wait, but `precpu_stats` empty → our own delta. Body must be closed |
| disk | `DiskUsage{…, Verbose:true}` | without `Verbose` the `Items` are empty! Slow: seconds on large hosts |
| events | `Events` | returns `Messages`/`Err` channels; on error we resubscribe after 3s |
| logs | `ContainerLogs{Follow, Tail:"500"}` | if the container has NO TTY the stream is multiplexed → `stdcopy.StdCopy` (pkg `moby/api/pkg/stdcopy`) |
| actions | `ContainerStart/Stop/Restart/Kill/Pause/Unpause/Remove` | stop/restart timeout 10s; kill = SIGKILL |
| rm | `ImageRemove{Force, PruneChildren:true}`, `VolumeRemove{Force}` | |
| single cache entry | `BuildCachePrune{All:true, Filters: id=<ID>}` | only way to remove a single record; if `CachesDeleted` is empty the record was in use/shared |
| prune | `ContainerPrune`, `ImagePrune` (filter `dangling=true/false`), `VolumePrune{All}`, `BuildCachePrune{All:true}` | since API 1.42 volume prune only touches anonymous volumes unless `All` is set |

## Noisy events
`exec_*` and `health_status*` are excluded from the feed (healthchecks every few seconds would flood it).
Events that change disk usage trigger a scan after a 2s debounce.

## Endpoint resolution (not done by the SDK)
The SDK only reads `DOCKER_HOST`. To respect `docker context use …` we read
`~/.docker/config.json` → `currentContext` → `~/.docker/contexts/meta/<sha256(name)>/meta.json` → `Endpoints.docker.Host`.
Importing `github.com/docker/cli` for this would have pulled in hundreds of dependencies (ADR 0002).
Socket fallback: `/var/run/docker.sock`, `~/.docker/run/docker.sock`, OrbStack, colima, Rancher Desktop.
Only `unix://` is considered "local" (enables host probes for disk). `ssh://` is not supported by the pure SDK.
