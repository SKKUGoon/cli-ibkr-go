#!/usr/bin/env bash
set -euo pipefail

version="${1:-}"
readonly repository="SKKUGoon/cli-ibkr-go"
install_dir="${IBKR_INSTALL_DIR:-${HOME}/.local/bin}"
if [[ "$#" -ne 1 ]] || [[ ! "$version" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+([.-][A-Za-z0-9.-]+)?$ ]]; then
  echo 'Usage: ./deploy-ibkr.sh VERSION' >&2
  exit 1
fi
case "$(uname -s):$(uname -m)" in
  Linux:x86_64) target=linux_amd64 ;;
  Darwin:arm64) target=darwin_arm64 ;;
  *) echo 'Unsupported platform' >&2; exit 1 ;;
esac
version="v${version#v}"
archive="ibkr-${version}-${target}.tar.gz"
base_url="https://github.com/${repository}/releases/download/${version}"
working_dir="$(mktemp -d)"
trap 'rm -rf "$working_dir"' EXIT
curl -fL "$base_url/$archive" -o "$working_dir/$archive"
curl -fL "$base_url/SHA256SUMS" -o "$working_dir/SHA256SUMS"
expected="$(awk -v name="$archive" '$2 == name {print $1}' "$working_dir/SHA256SUMS")"
if command -v sha256sum >/dev/null; then
  actual="$(sha256sum "$working_dir/$archive" | awk '{print $1}')"
else
  actual="$(shasum -a 256 "$working_dir/$archive" | awk '{print $1}')"
fi
if [[ -z "$expected" || "$actual" != "$expected" ]]; then
  echo 'Release checksum mismatch' >&2
  exit 1
fi
tar -xzf "$working_dir/$archive" -C "$working_dir" ibkr
mkdir -p "$install_dir"
install -m 0755 "$working_dir/ibkr" "$install_dir/ibkr"
echo "Installed $install_dir/ibkr"
