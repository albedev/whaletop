# ADR 0011 — Release tooling: GoReleaser, own Homebrew tap, deploy key
Date: 2026-10-09 · Status: accepted

**Decision.** Releases are produced by GoReleaser v2 in GitHub Actions on tag push: archives for darwin/linux ×
amd64/arm64, `.deb`/`.rpm` via nfpm, checksums, GitHub release, and a Homebrew **cask** in `albedev/homebrew-tap`.

**Why.**
- GoReleaser (MIT) is the de-facto standard for Go CLIs; writing release scripts would be reinventing the wheel.
- homebrew-core rejects non-OSI licenses (ADR 0009) and already has an unrelated `dtop`; a personal tap is the only
  Homebrew route. GoReleaser now generates casks (`homebrew_casks`) instead of formulas for prebuilt binaries.
- The tap is updated over SSH with a write deploy key scoped to the tap repo, instead of a personal access token that
  would grant access to every repository of the account.

**Not done (yet).** Apple signing/notarization (needs a paid Apple Developer account); quarantine is removed by a cask
postflight hook instead. Docker image on ghcr.io (disk capacity detection does not work from inside a container).
AUR package (could be added with GoReleaser `aurs:`).
