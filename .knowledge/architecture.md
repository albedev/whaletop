# Architecture

```
cmd/whaletop/main.go          flags, connection, starts tea.Program (or --dump)
internal/docker/          thin wrapper over the official moby SDK
  client.go               New(), ResolveEndpoint() (DOCKER_HOST > DOCKER_CONTEXT > config.json > well-known sockets)
  actions.go              ContainerAction, RemoveObject, Prune (+ Action.Destructive())
internal/host/            host resources and docker disk capacity (platform specific)
  host.go                 Detect() via gopsutil, DetectDisk(), vmDisk() (block stat of the sparse disk image)
  host_darwin.go          Docker Desktop (Docker.raw + settings-store.json), colima
  host_linux.go           Docker Desktop for Linux; otherwise statfs of DockerRootDir
internal/collector/       raw data → metrics, UI independent
  resources.go            Collector, Sample(): container list + one-shot stats in parallel (max 16)
  disk.go                 Disk(): /system/df verbose + inspect → DiskReport with Usage and LastUsed
internal/ui/              bubbletea
  app.go                  Model, messages, Update, async commands, events, mouse, exec shell
  resources.go / disk.go  state, sorting, keys and rendering of the two views
  table.go                generic table + selector (cursor bound to the ID, not the index)
  widgets.go              box btop, graph, sparkline, meter, stackedBar, overlay, fit
  dialogs.go              confirmation, action menu, help
  logs.go                 streaming log viewer (stdcopy demux)
  view.go                 View(), header, footer
  theme.go                palette and gradients (btop default theme style)
  dump.go                 render without a TTY for tests/debug
```

## Data flow
```
           tickMsg{gen} ──► sampleCmd ──► Collector.Sample ──► snapMsg ──► rebuildRes ──► View
diskPollMsg (5s) / event ──► diskCmd ──► Collector.Disk ──► diskMsg ──► rebuildDisk ──► View
     Events stream (SDK) ──► waitEvent ──► eventMsg ──► feed + Invalidate(inspect) + disk debounce 2s
              key/action ──► m.run(...) ──► actionMsg ──► status + new sample + new disk scan
```

- All I/O happens inside `tea.Cmd` (bubbletea goroutines): `Update`/`View` never block.
- **One sample at a time** (`sampling`), **one disk scan at a time** (`diskLoading` + `diskQueued`).
- The tick uses a **generation** (`tickGen`): only the last scheduled tick produces a sample,
  so extra samples (after an action) do not multiply the timers. See gotchas.
- `/system/df` is expensive: every `--disk-interval` (30s) with the disk view open, ×4 otherwise,
  plus one scan 2s after events that change space (create/destroy/delete/prune/pull/tag/die…).
- Container `inspect` results are cached, invalidated on state/health change or by a container event.
- Histories (300 samples) are **immutable** slices: `push` copies, because the snapshot passed to the UI
  must not change underneath it.
