#!/bin/sh
# Installs the latest taskam release into /usr/local/bin (or $TASKAM_INSTALL_DIR).
#   curl -fsSL https://raw.githubusercontent.com/Taskam-Tracker/Tasktam-CLI/main/install.sh | sh
set -eu

repo="Taskam-Tracker/Tasktam-CLI"
dir="${TASKAM_INSTALL_DIR:-/usr/local/bin}"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in darwin|linux) ;; *) echo "Unsupported OS: $os (download a release from GitHub)" >&2; exit 1 ;; esac
arch=$(uname -m)
case "$arch" in x86_64|amd64) arch=amd64 ;; arm64|aarch64) arch=arm64 ;; *) echo "Unsupported CPU: $arch" >&2; exit 1 ;; esac

asset="taskam_${os}_${arch}.tar.gz"
base="https://github.com/$repo/releases/latest/download"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "Downloading $asset…"
curl -fsSL "$base/$asset" -o "$tmp/$asset"
curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt"
( cd "$tmp" && grep " $asset\$" checksums.txt | { command -v sha256sum >/dev/null && sha256sum -c - || shasum -a 256 -c -; } )
tar -xzf "$tmp/$asset" -C "$tmp" taskam

if [ -w "$dir" ]; then
  install -m 0755 "$tmp/taskam" "$dir/taskam"
else
  echo "Installing to $dir (needs sudo)…"
  sudo install -m 0755 "$tmp/taskam" "$dir/taskam"
fi
echo "Installed $("$dir/taskam" --version). Next: taskam login"
