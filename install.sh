#!/usr/bin/env bash
# Installs the csr binary (claude-session-remove) from a prebuilt GitHub
# release, verifies its checksum, and optionally installs the /csr command
# for Claude Code.
#
#   curl -fsSL https://raw.githubusercontent.com/DanielRoman11/claude-session-remove/main/install.sh | bash
set -euo pipefail

REPO="DanielRoman11/claude-session-remove"
BIN_NAME="csr"
INSTALL_DIR="${CSR_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${CSR_VERSION:-latest}"

log() { printf '%s\n' "$*" >&2; }
die() { log "error: $*"; exit 1; }

need() { command -v "$1" >/dev/null 2>&1 || die "'$1' is required but not found on PATH"; }
need curl
need tar

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
  linux|darwin) ;;
  *) die "unsupported OS: $os (only linux and darwin have prebuilt binaries; build from source instead, see README)" ;;
esac

arch=$(uname -m)
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) die "unsupported architecture: $arch" ;;
esac

asset="${BIN_NAME}_${os}_${arch}.tar.gz"
if [ "$VERSION" = "latest" ]; then
  base_url="https://github.com/$REPO/releases/latest/download"
else
  base_url="https://github.com/$REPO/releases/download/$VERSION"
fi

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT

log "Downloading $asset ($VERSION)..."
curl -fsSL "$base_url/$asset" -o "$tmpdir/$asset" \
  || die "could not download $base_url/$asset (no release for $os/$arch yet?)"

log "Verifying checksum..."
curl -fsSL "$base_url/checksums.txt" -o "$tmpdir/checksums.txt" \
  || die "could not download checksums.txt"

expected=$(grep " $asset\$" "$tmpdir/checksums.txt" | awk '{print $1}')
[ -n "$expected" ] || die "no checksum entry for $asset"

if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$tmpdir/$asset" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
  actual=$(shasum -a 256 "$tmpdir/$asset" | awk '{print $1}')
else
  die "need sha256sum or shasum to verify the download"
fi

[ "$expected" = "$actual" ] || die "checksum mismatch for $asset (expected $expected, got $actual)"

tar -xzf "$tmpdir/$asset" -C "$tmpdir"

mkdir -p "$INSTALL_DIR"
mv "$tmpdir/$BIN_NAME" "$INSTALL_DIR/$BIN_NAME"
chmod +x "$INSTALL_DIR/$BIN_NAME"
log "Installed $BIN_NAME to $INSTALL_DIR/$BIN_NAME"

case ":${PATH}:" in
  *":$INSTALL_DIR:"*) ;;
  *) log "Note: $INSTALL_DIR is not on your PATH. Add this to your shell profile:"
     log "  export PATH=\"$INSTALL_DIR:\$PATH\"" ;;
esac

if [ -d "$HOME/.claude/commands" ] || [ "${CSR_INSTALL_COMMAND:-}" = "1" ]; then
  mkdir -p "$HOME/.claude/commands"
  cmd_url="https://raw.githubusercontent.com/$REPO/main/commands/csr.md"
  if curl -fsSL "$cmd_url" -o "$HOME/.claude/commands/csr.md"; then
    log "Installed the /csr command to $HOME/.claude/commands/csr.md"
  else
    log "Note: could not fetch the /csr command file; skipping (the binary still works standalone)"
  fi
fi

log "Done. Run '$BIN_NAME' in any project directory to open the session picker."
