#!/bin/sh
# whaletop installer: downloads the latest (or a given) release binary from GitHub,
# verifies its checksum and installs `whaletop` plus the `wtop` alias.
#
#   curl -fsSL https://raw.githubusercontent.com/albedev/whaletop/main/install.sh | sh
#
# Environment:
#   WHALETOP_VERSION      release tag to install, e.g. v0.3.0 (default: latest)
#   WHALETOP_INSTALL_DIR  target directory (default: ~/.local/bin, no sudo needed)
set -eu

REPO="albedev/whaletop"
VERSION="${WHALETOP_VERSION:-}"
DIR="${WHALETOP_INSTALL_DIR:-$HOME/.local/bin}"

say() { printf '%s\n' "whaletop-install: $*"; }
die() { say "error: $*" >&2; exit 1; }

if command -v curl >/dev/null 2>&1; then
	fetch() { curl -fsSL "$1" -o "$2"; }
elif command -v wget >/dev/null 2>&1; then
	fetch() { wget -q "$1" -O "$2"; }
else
	die "curl or wget is required"
fi

case "$(uname -s)" in
	Darwin) os=darwin ;;
	Linux) os=linux ;;
	*) die "unsupported OS $(uname -s) (whaletop runs on macOS and Linux)" ;;
esac
case "$(uname -m)" in
	x86_64 | amd64) arch=amd64 ;;
	arm64 | aarch64) arch=arm64 ;;
	*) die "unsupported architecture $(uname -m)" ;;
esac

if [ -z "$VERSION" ]; then
	# /releases/latest redirects to /releases/tag/vX.Y.Z: no API call, no rate limit
	if command -v curl >/dev/null 2>&1; then
		url=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest")
	else
		url=$(wget -q -S --spider "https://github.com/$REPO/releases/latest" 2>&1 | sed -n 's/^ *[Ll]ocation: *//p' | tail -n 1)
	fi
	VERSION="${url##*/}"
	case "$VERSION" in v*) ;; *) die "could not determine the latest release" ;; esac
fi
num="${VERSION#v}"
archive="whaletop_${num}_${os}_${arch}.tar.gz"
base="https://github.com/$REPO/releases/download/$VERSION"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

say "downloading whaletop $VERSION ($os/$arch)"
fetch "$base/$archive" "$tmp/$archive" || die "download failed: $base/$archive"
fetch "$base/checksums.txt" "$tmp/checksums.txt" || die "download failed: $base/checksums.txt"

want=$(grep " $archive\$" "$tmp/checksums.txt" | cut -d ' ' -f 1)
[ -n "$want" ] || die "$archive not listed in checksums.txt"
if command -v sha256sum >/dev/null 2>&1; then
	got=$(sha256sum "$tmp/$archive" | cut -d ' ' -f 1)
else
	got=$(shasum -a 256 "$tmp/$archive" | cut -d ' ' -f 1)
fi
[ "$want" = "$got" ] || die "checksum mismatch for $archive"

tar -xzf "$tmp/$archive" -C "$tmp" whaletop
mkdir -p "$DIR"
install -m 755 "$tmp/whaletop" "$DIR/whaletop" 2>/dev/null || {
	cp "$tmp/whaletop" "$DIR/whaletop" && chmod 755 "$DIR/whaletop"
} || die "cannot write to $DIR (set WHALETOP_INSTALL_DIR or run with sudo)"
# binaries are not notarized: make sure Gatekeeper does not block them
[ "$os" = darwin ] && xattr -d com.apple.quarantine "$DIR/whaletop" 2>/dev/null || true

# short alias, never over a foreign `wtop` (e.g. the PyPI web-server tool)
if [ -e "$DIR/wtop" ] && [ "$(readlink "$DIR/wtop" 2>/dev/null)" != whaletop ]; then
	say "warning: $DIR/wtop already exists and is not whaletop: alias not installed"
else
	ln -sfn whaletop "$DIR/wtop"
fi

say "installed $("$DIR/whaletop" --version) to $DIR (alias: wtop)"
case ":$PATH:" in
	*":$DIR:"*) ;;
	*) say "note: $DIR is not in your PATH; add it, e.g.: echo 'export PATH=\"$DIR:\$PATH\"' >> ~/.profile" ;;
esac
