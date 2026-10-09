# ADR 0012 — Update notifications and explicit self-update
Date: 2026-10-09 · Status: accepted

**Decision.** whaletop notifies about new releases (header badge + hint, checked at most once a day) and offers an
explicit `whaletop update` command for installs that no package manager owns. It never updates itself silently.

**Why.**
- Silent self-replacement surprises users and widens the attack surface; most terminal tools notify instead.
- Updating a binary owned by Homebrew or dpkg/rpm would desync the package manager: those installs get the right
  command (`brew upgrade --cask whaletop`, new .deb/.rpm) instead.
- The latest tag comes from the `/releases/latest` redirect, not the GitHub API, so there is no rate limit and no token.

**Library choice.** First implemented with `creativeprojects/go-selfupdate` (MIT), then replaced by
`minio/selfupdate` (Apache-2.0) + ~80 lines of stdlib code: go-selfupdate pulled GitHub/GitLab/Gitea SDKs and three
HashiCorp libraries under MPL-2.0 (weak copyleft) that we do not need. minio/selfupdate does the delicate part (atomic
replace with rollback); download, checksum and tar extraction mirror install.sh.

**Privacy.** One HEAD request to github.com per day at most; no identifiers beyond what any HTTP request carries.
Opt-out: `--no-update-check` or `WHALETOP_NO_UPDATE_CHECK=1`.
