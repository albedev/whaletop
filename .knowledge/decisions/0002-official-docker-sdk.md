# ADR 0002 — Official moby SDK, no hand-written HTTP client
Date: 2026-10-08 · Status: accepted (supersedes an earlier draft)

**Context.** An earlier version had a minimal HTTP client on the Docker socket, to keep dependencies few.
The user explicitly asked: "se esiste roba open source su cui basarsi con licenza compatibile, procedi, non reinventare la ruota" (if there is open source to build on with a compatible license, go ahead, don't reinvent the wheel).

**Decision.** Use `github.com/moby/moby/client` + `github.com/moby/moby/api` (Apache-2.0), the module supported by Docker v29.
Only addition of ours: `ResolveEndpoint()` for docker contexts (~40 lines), because the SDK only reads `DOCKER_HOST`
and `github.com/docker/cli` is huge.

**Consequences.** Typed, maintained API; stdcopy ready for logs; prune/filters already handled.
More indirect dependencies (OpenTelemetry). The v29 SDK API changed a lot compared to `docker/docker`: old guides
and snippets online no longer apply (see docker-api.md).
