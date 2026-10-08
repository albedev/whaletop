# ADR 0008 — Mandatory confirmation for destructive actions
Date: 2026-10-08 · Status: accepted

`docker.Action.Destructive()` is the single source of truth: kill, remove, force-remove and all prune actions ask for `y` in a dialog;
`enter`, `n`, `esc`, `q` cancel. Start/stop/restart/pause/unpause run immediately (reversible).
The disk-removal dialog shows how much space it frees and which containers use the object. The prune "all unused volumes"
is explicitly labeled "ALL unused volumes (named too)" because it deletes data.
