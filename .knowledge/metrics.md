# Metrics and formulas

## Docker capacity ("maximum usable")
From `GET /info`: `NCPU` and `MemTotal`. On Docker Desktop / colima / OrbStack these are the resources of the **VM**,
not of the host. The host resources come from gopsutil (`runtime.NumCPU`, `mem.VirtualMemory`).
Example: host 16 cores / 64 GiB, Docker Desktop VM 8 cores / 16 GiB.

## CPU
Stats in one-shot mode (`stream=false`, no previous sample): `precpu_stats` is empty,
so the delta is computed by the collector against its **own** previous sample:
```
CPUPct   = Δcpu_usage.total_usage / Δsystem_cpu_usage × online_cpus × 100   # as `docker stats`: 100% = 1 core
CPUShare = CPUPct / NCPU                                                    # % of the CPU that docker can use
Total    = Σ CPUPct / 100 cores;  total CPUShare = cores / NCPU × 100
```
The first sample of a container shows 0. Counters that decrease (restart) → rate 0.
`CPULimit` (from inspect): `NanoCpus/1e9` or `CpuQuota/CpuPeriod`.

## Memory
```
MemUsed    = usage − inactive_file (cgroup v2)  |  usage − total_inactive_file (cgroup v1)   # as docker CLI
MemShare   = MemUsed / MemTotal × 100
MemOfLimit = MemUsed / limit × 100   only if limit < MemTotal (otherwise "no limit" → "-")
```

## Network / IO
Rate = Δcounter / Δt (t = `read` field of the stats). IO from `blkio_stats.io_service_bytes_recursive`
(ops `read`/`write`). On Docker Desktop the block IO is often 0 (overlay on virtiofs/VM).

## Disk: capacity ("maximum usable by docker")
The Docker API **does not expose** the storage size. `host.DetectDisk` in order:
1. `--disk-limit 64G` (manual override)
2. remote daemon (not unix socket) → unknown
3. **Docker Desktop macOS**: `~/Library/Containers/com.docker.docker/Data/vms/0/data/Docker.raw`
   - logical size of the sparse file = maximum disk of the VM (e.g. 56 GiB)
   - allocated blocks × 512 = space actually used on the host
   - if `~/Library/Group Containers/group.com.docker/settings-store.json` exists with `DiskSizeMiB`, it takes precedence
     (in sandbox it may give "Operation not permitted": falls back to Docker.raw)
   - available = min(total − allocated, free space on host disk)
4. **colima**: `~/.colima/_lima/colima/diffdisk` (same sparse logic)
5. **Docker Desktop Linux**: `~/.docker/desktop/vms/0/data/Docker.raw`
6. native local daemon (Linux): `statfs(DockerRootDir)`: total = fs, available = free
7. otherwise "unknown"

OrbStack: dynamic disk, not managed (falls back to "unknown" or statfs). See roadmap.

## Disk: used
`GET /system/df?verbose=1` → per section `TotalSize` and `Reclaimable` computed by the daemon
(same numbers as `docker system df`). "docker" = sum of the 4 sections.
Images: `Size` includes shared layers, `SharedSize` = shared part; "free up to" = Size − Shared.
Containers: `SizeRw` (writable layer). Volumes: `UsageData.Size` (may be −1 = not computed → "n/a").

## States (`collector.Usage`)
| Usage | images | containers | volumes | build cache |
|---|---|---|---|---|
| Active | used by running container | running/paused/restarting | mounted by running container | `InUse` (build in progress) |
| InUse | used only by stopped containers | — | mounted only by stopped containers | `Shared` with an image |
| Unused | tagged, no container | `created` never started | named, no container | not used |
| Dangling | untagged | stopped (exited/dead) → label "stopped" | anonymous (64 hex or anonymous label) | — |
`Reclaimable()` = Unused or Dangling.

## "Last used"
Docker **does not record** the last use of images and volumes. Heuristic:
- container: max(StartedAt, FinishedAt) from inspect; running → "now"
- image / volume: max of the times of the containers that use/mount it; if one is running → "now";
  no container → "never" (even if it may have been used by containers since removed)
- build cache: native `LastUsedAt` of BuildKit
