# Release process

Tooling: [GoReleaser](https://goreleaser.com) v2 (MIT), config in `.goreleaser.yaml`, run by
`.github/workflows/release.yml` (goreleaser-action) on every pushed tag `v*`.

## Cutting a release
```sh
# 1. make sure main is green (ci workflow) and .knowledge/changelog.md has the new version
git tag v0.3.0
git push origin v0.3.0        # → GitHub Actions does the rest (~2 min)
```
The workflow:
1. runs `go mod tidy` + `go test ./...` (GoReleaser `before` hooks);
2. builds `whaletop` for darwin/linux × amd64/arm64 (`CGO_ENABLED=0`, `-trimpath`, `-X main.version=v<version>`);
3. creates `whaletop_<version>_<os>_<arch>.tar.gz` (with LICENSE, README, THIRD_PARTY_NOTICES), `.deb`/`.rpm`
   (with the `/usr/bin/wtop` symlink) and `checksums.txt`;
4. publishes the GitHub release with an auto changelog (commits starting with `docs:`, `test:`, `ci:` are excluded);
5. writes `Casks/whaletop.rb` into **albedev/homebrew-tap** and pushes it.

Users then install with `brew install --cask albedev/tap/whaletop` (or `brew upgrade`), or with the one-line
installer `curl -fsSL https://raw.githubusercontent.com/albedev/whaletop/main/install.sh | sh`.

## install.sh
POSIX sh (tested with macOS sh and dash; shellcheck in CI). Resolves the latest tag through the
`/releases/latest` redirect (no GitHub API call, so no rate limit), downloads the archive + `checksums.txt`,
verifies sha256, installs into `$WHALETOP_INSTALL_DIR` (default `~/.local/bin`, no sudo) and creates the `wtop`
symlink unless a foreign `wtop` exists. It depends on the archive naming `whaletop_<version>_<os>_<arch>.tar.gz`
in `.goreleaser.yaml`: change both together. CI job `install-script` runs it against the latest release.
A self-hosted APT repository was considered and dropped (user decision, 2026-10-09): the `.deb` on each release
plus install.sh are enough.

## Homebrew details
- **Cask, not formula**: GoReleaser ≥ 2.10 deprecated `brews` (formulas built from prebuilt binaries) in favour of
  `homebrew_casks`. homebrew-core is not an option anyway: it only accepts open source (DFSG/OSI) licenses, and our
  license is source-available (ADR 0009). Hence our own tap.
- **`wtop` alias**: `custom_block: binary "whaletop", target: "wtop"` → two symlinks in `$(brew --prefix)/bin`.
  Verified on 2026-10-09 by installing the snapshot cask from a temporary local tap (`file://` URL): both links created,
  uninstall removes both.
- **Gatekeeper**: binaries are not signed/notarized. A `postflight` hook runs
  `xattr -dr com.apple.quarantine` on the staged binary. Apple may break this in the future; the real fix is signing +
  notarization (needs an Apple Developer account, GoReleaser supports it via `notarize:`).
- **Auth**: the workflow pushes to the tap over SSH with a **deploy key** that has write access to
  `albedev/homebrew-tap` only (title "whaletop release (GoReleaser)"). Its private half is the Actions secret
  `HOMEBREW_TAP_DEPLOY_KEY` on `albedev/whaletop`; no copy exists elsewhere. To rotate it:
  ```sh
  ssh-keygen -t ed25519 -N "" -f k -C whaletop-release
  gh repo deploy-key add k.pub -R albedev/homebrew-tap --allow-write --title "whaletop release (GoReleaser)"
  gh secret set HOMEBREW_TAP_DEPLOY_KEY -R albedev/whaletop < k && rm k k.pub
  # then delete the old key: gh repo deploy-key list/delete -R albedev/homebrew-tap
  ```
  Chosen over a personal access token because it cannot touch any other repository and never expires silently.

## Local checks
```sh
go install github.com/goreleaser/goreleaser/v2@latest
make release-check       # validate .goreleaser.yaml
make release-snapshot    # full dry run into dist/ (nothing is published)
```

## Notes
- The main repo must be **public** for users: release assets of a private repo return 404 to anonymous downloads,
  so the cask (and `go install`) only work once `albedev/whaletop` is public.
- `go install` users get only the `whaletop` binary (one binary per main package): the README tells them to alias it.
- CI (`.github/workflows/ci.yml`): `go vet`, `go test`, `go build` on ubuntu and macos for pushes to main and PRs.
