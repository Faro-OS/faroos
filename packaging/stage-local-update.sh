#!/usr/bin/env bash

set -Eeuo pipefail

die() {
  echo "FaroOS local update builder: $*" >&2
  exit 1
}

(( EUID != 0 )) || die "run this as the developer user, not root"
[[ "$(uname -s)" == "Linux" ]] || die "local automatic deployment currently requires Linux"
(( $# >= 1 && $# <= 2 )) || die "usage: $0 OUTPUT_DIRECTORY [VERSION]"

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
output_dir="$(realpath -m "$1")"
version="${2:-dev-$(date -u +%Y%m%d%H%M%S)}"
[[ "$version" =~ ^[vV0-9A-Za-z][vV0-9A-Za-z._+-]*$ ]] || die "invalid version: $version"
[[ "$output_dir" != / && "$output_dir" != "$repo_dir" ]] || die "unsafe output directory"

case "$(uname -m)" in
  x86_64 | amd64) arch="amd64" ;;
  aarch64 | arm64) arch="arm64" ;;
  *) die "unsupported architecture: $(uname -m)" ;;
esac

for command_name in go npm realpath sha256sum; do
  command -v "$command_name" >/dev/null 2>&1 || die "required command not found: $command_name"
done

echo "Building FaroOS web interface..."
(cd "$repo_dir/web" && npm run check && npm run build)

parent_dir="$(dirname "$output_dir")"
mkdir -p "$parent_dir"
tmp_dir="$(mktemp -d "$parent_dir/.faroos-stage.XXXXXX")"
trap 'rm -rf "$tmp_dir"' EXIT

echo "Building FaroOS ${version}..."
(cd "$repo_dir" && CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -ldflags "-X main.version=$version" -o "$tmp_dir/faroos-server-linux-$arch" ./cmd/server)
for agent_arch in amd64 arm64; do
  (cd "$repo_dir" && CGO_ENABLED=0 GOOS=linux GOARCH="$agent_arch" go build -ldflags "-X main.version=$version" -o "$tmp_dir/faroos-agent-linux-$agent_arch" ./cmd/agent)
done
install -m 0755 "$repo_dir/packaging/update.sh" "$tmp_dir/faroos-update"
printf '%s\n' "$version" >"$tmp_dir/VERSION"
(
  cd "$tmp_dir"
  sha256sum faroos-agent-linux-amd64 faroos-agent-linux-arm64 "faroos-server-linux-$arch" faroos-update >SHA256SUMS
)

previous_dir="${output_dir}.previous"
rm -rf -- "$previous_dir"
if [[ -e "$output_dir" ]]; then
  mv -- "$output_dir" "$previous_dir"
fi
mv -- "$tmp_dir" "$output_dir"
trap - EXIT
chmod -R go-w "$output_dir"

echo "Local FaroOS update staged: $version"
echo "The installed update timer will apply it automatically."
