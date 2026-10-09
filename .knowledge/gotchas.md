# Gotchas (bugs already encountered)

1. **`Init()` has a value receiver** (bubbletea API): anything `Init` writes to the model is lost.
   The event subscription was done in `Init` → the channels ended up on a copy, the first event arrived
   and then the loop blocked on a nil channel. Fix: subscribe and set the "in flight" flags in `New()`.
   Rule: in `Init` only return commands, never mutate state.
2. **Timers multiplying**: every `snapMsg` schedules a tick; an extra sample (after an action) created a second
   parallel timer, and so on. Fix: `tickMsg{gen}` + `tickGen`: only the latest one is honored. For the disk a fixed 5s
   poll that decides whether the scan is expired, plus one-shot ticks for the debounce of events.
3. **View has a value receiver**: don't save layout data there (e.g. the table row for the mouse). The layout is recomputed
   with pure functions (`resLayout()`, `diskLayout()`) used by both View and Update.
4. **`short()` without units** below 1 KiB ("89" instead of "89B"): reported by the user. Now always with units, there is a test.
5. **`DiskUsage` without `Verbose`** returns only the totals, empty lists.
6. **Multiplexed logs**: without a TTY every frame has an 8-byte header; reading raw shows garbage. Use `stdcopy`.
   Line scanners must respect `ctx.Done()` when sending, otherwise goroutine leak on viewer close.
7. **Image prune**: `ImagesDeleted` contains both `Untagged` and `Deleted` entries; count only `Deleted`.
   With the containerd store (recent Docker Desktop) `SpaceReclaimed` may come out very small: it is the daemon's value.
8. **Footer**: the `m` key for the chart toggle was not evident (there was only "mode:block"): it is now always shown as a hotkey.
9. **ctrl+arrows on macOS** are intercepted by the system (Mission Control) and don't reach whaletop: alias `shift+↑/↓`.
10. **Testing with tmux**: `Escape` followed immediately by another key is read by bubbletea as `alt+<key>`.
    Send `Escape` in a separate `send-keys` with a pause, otherwise the following keys become actions
    (it happened: `/redi` became restart + exec).
11. **Docker Desktop's settings-store.json** may be unreadable in sandboxed environments ("Operation not permitted"):
   the disk estimate falls back to the logical size of `Docker.raw`.
