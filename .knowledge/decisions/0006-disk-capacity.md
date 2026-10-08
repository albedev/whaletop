# ADR 0006 — Docker disk capacity detection
Date: 2026-10-08 · Status: accepted

The Engine API does not say how much space docker can use. A cascading detection per platform was chosen
(details in metrics.md): override `--disk-limit` > sparse VM disk (Docker Desktop `Docker.raw`, colima `diffdisk`)
> `statfs(DockerRootDir)` for local native daemons > unknown.
The idea of launching a helper container that runs `df` inside the VM was discarded: invasive (image pulls, extra containers in the user's list) and slow. For remote daemons nothing is estimated: "unknown" is better than a wrong number.
