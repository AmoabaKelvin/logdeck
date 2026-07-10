#!/bin/sh
# LogDeck CLI installer.
#   curl -fsSL https://raw.githubusercontent.com/AmoabaKelvin/logdeck/main/install.sh | sh
# Installs the latest logdeck release binary for this OS/arch.
set -eu

REPO="AmoabaKelvin/logdeck"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
  darwin | linux) ;;
  *)
    echo "Unsupported OS: $os (darwin and linux binaries are published)" >&2
    exit 1
    ;;
esac

arch=$(uname -m)
case "$arch" in
  x86_64 | amd64) arch="amd64" ;;
  arm64 | aarch64) arch="arm64" ;;
  *)
    echo "Unsupported architecture: $arch (amd64 and arm64 binaries are published)" >&2
    exit 1
    ;;
esac

url="https://github.com/${REPO}/releases/latest/download/logdeck_${os}_${arch}.tar.gz"

if [ -w /usr/local/bin ]; then
  install_dir="/usr/local/bin"
else
  install_dir="${HOME}/.local/bin"
  mkdir -p "$install_dir"
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "Downloading $url"
curl -fsSL "$url" -o "$tmp/logdeck.tar.gz"
tar -xzf "$tmp/logdeck.tar.gz" -C "$tmp"
install -m 0755 "$tmp/logdeck" "$install_dir/logdeck"

echo "Installed $("$install_dir/logdeck" --version) to $install_dir/logdeck"

case ":$PATH:" in
  *":$install_dir:"*) ;;
  *)
    echo "Note: $install_dir is not on your PATH. Add it with:"
    echo "  export PATH=\"$install_dir:\$PATH\""
    ;;
esac
