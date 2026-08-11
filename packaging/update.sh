#!/usr/bin/env bash

set -Eeuo pipefail

REPO="${REPO:-Faro-OS/faroos}"
FAROOS_UPDATE_CHANNEL="${FAROOS_UPDATE_CHANNEL:-stable}"
FAROOS_UPDATE_LOCAL_DIR="${FAROOS_UPDATE_LOCAL_DIR:-}"
FAROOS_UPDATE_URL="${FAROOS_UPDATE_URL:-}"
FAROOS_INSTALL_ROOT="${FAROOS_INSTALL_ROOT:-}"

die() {
  echo "FaroOS updater: $*" >&2
  exit 1
}

usage() {
  echo "Usage: $0 --component server|agent" >&2
  exit 2
}

component=""
while (( $# > 0 )); do
  case "$1" in
    --component) component="${2:-}"; shift 2 ;;
    *) usage ;;
  esac
done
[[ "$component" == "server" || "$component" == "agent" ]] || usage
(( EUID == 0 )) || die "must run as root"
[[ "$(uname -s)" == "Linux" ]] || die "automatic updates currently require Linux"
[[ -z "$FAROOS_INSTALL_ROOT" || "$FAROOS_INSTALL_ROOT" == /* ]] || die "FAROOS_INSTALL_ROOT must be absolute"
root_prefix="${FAROOS_INSTALL_ROOT%/}"

case "$(uname -m)" in
  x86_64 | amd64) arch="amd64" ;;
  aarch64 | arm64) arch="arm64" ;;
  *) die "unsupported architecture: $(uname -m)" ;;
esac

for command_name in awk cp curl flock install mv sha256sum systemctl; do
  command -v "$command_name" >/dev/null 2>&1 || die "required command not found: $command_name"
done

state_dir="${root_prefix}/var/lib/faroos/update/${component}"
install -d -o root -g root -m 0755 "$state_dir" "$state_dir/backups" "${root_prefix}/usr/local/libexec"
exec 9>"$state_dir/update.lock"
flock -n 9 || exit 0

tmp_dir="$(mktemp -d "$state_dir/download.XXXXXX")"
trap 'rm -rf "$tmp_dir"' EXIT

fetch_file() {
  local name="$1" destination="$2"
  if [[ "$FAROOS_UPDATE_CHANNEL" == "local" ]]; then
    [[ -n "$FAROOS_UPDATE_LOCAL_DIR" ]] || die "local channel requires FAROOS_UPDATE_LOCAL_DIR"
    cp -- "$FAROOS_UPDATE_LOCAL_DIR/$name" "$destination"
  else
    curl -fL --retry 3 --connect-timeout 15 -o "$destination" "$release_base/$name"
  fi
}

case "$FAROOS_UPDATE_CHANNEL" in
  local)
    [[ -d "$FAROOS_UPDATE_LOCAL_DIR" ]] || die "local update directory does not exist"
    cp -- "$FAROOS_UPDATE_LOCAL_DIR/SHA256SUMS" "$tmp_dir/SHA256SUMS"
    release_tag="$(tr -d '[:space:]' <"$FAROOS_UPDATE_LOCAL_DIR/VERSION")"
    release_base=""
    ;;
  panel)
    [[ "$FAROOS_UPDATE_URL" == http://* || "$FAROOS_UPDATE_URL" == https://* ]] || die "panel channel requires an HTTP(S) FAROOS_UPDATE_URL"
    [[ "$FAROOS_UPDATE_URL" != *$'\n'* && "$FAROOS_UPDATE_URL" != *$'\r'* ]] || die "invalid panel update URL"
    release_base="${FAROOS_UPDATE_URL%/}"
    curl -fL --retry 3 --connect-timeout 15 -o "$tmp_dir/SHA256SUMS" "$release_base/SHA256SUMS"
    curl -fL --retry 3 --connect-timeout 15 -o "$tmp_dir/VERSION" "$release_base/VERSION"
    release_tag="$(tr -d '[:space:]' <"$tmp_dir/VERSION")"
    ;;
  stable)
    sums_endpoint="https://github.com/${REPO}/releases/latest/download/SHA256SUMS"
    effective_url="$(curl -fL --retry 3 --connect-timeout 15 -o "$tmp_dir/SHA256SUMS" -w '%{url_effective}' "$sums_endpoint")"
    release_tag="$(sed -nE 's#^.*/releases/download/([^/]+)/SHA256SUMS$#\1#p' <<<"$effective_url")"
    [[ -n "$release_tag" ]] || die "could not determine latest release from $effective_url"
    release_base="https://github.com/${REPO}/releases/download/${release_tag}"
    ;;
  *) die "unsupported update channel: $FAROOS_UPDATE_CHANNEL" ;;
esac
[[ "$release_tag" =~ ^[vV0-9A-Za-z][vV0-9A-Za-z._+-]*$ ]] || die "invalid release version"

binary_name="faroos-${component}"
binary_target="${root_prefix}/usr/local/bin/${binary_name}"
asset_name="${binary_name}-linux-${arch}"
[[ -x "$binary_target" ]] || die "$binary_target is not installed"
current_version="$($binary_target --version 2>/dev/null || true)"
needs_binary_update=1
[[ "$current_version" == "$release_tag" ]] && needs_binary_update=0
if (( needs_binary_update )) && [[ "$FAROOS_UPDATE_CHANNEL" == "stable" ]]; then
  current_numeric="${current_version#v}"
  release_numeric="${release_tag#v}"
  if [[ "$current_numeric" =~ ^[0-9]+(\.[0-9]+){1,3}$ && "$release_numeric" =~ ^[0-9]+(\.[0-9]+){1,3}$ ]]; then
    newest_numeric="$(printf '%s\n%s\n' "$current_numeric" "$release_numeric" | sort -V | tail -n 1)"
    if [[ "$newest_numeric" != "$release_numeric" ]]; then
      echo "FaroOS updater: refusing to downgrade ${component} from ${current_version} to ${release_tag}."
      exit 0
    fi
  fi
fi

verify_file() {
  local name="$1" path="$2" expected
  expected="$(awk -v name="$name" '$2 == name || $2 == "*" name { print $1; exit }' "$tmp_dir/SHA256SUMS")"
  [[ "$expected" =~ ^[0-9a-fA-F]{64}$ ]] || die "checksum missing for $name"
  printf '%s  %s\n' "$expected" "$path" | sha256sum -c - >/dev/null
}

if (( needs_binary_update )); then
  fetch_file "$asset_name" "$tmp_dir/$asset_name"
  verify_file "$asset_name" "$tmp_dir/$asset_name"
  chmod 0755 "$tmp_dir/$asset_name"
  downloaded_version="$("$tmp_dir/$asset_name" --version 2>/dev/null || true)"
  [[ "$downloaded_version" == "$release_tag" ]] || die "downloaded binary reports ${downloaded_version:-no version}, expected $release_tag"
fi

if [[ "$component" == "server" ]]; then
  for agent_arch in amd64 arm64; do
    cached_agent="faroos-agent-linux-${agent_arch}"
    existing_agent="${root_prefix}/var/lib/faroos/server/downloads/${cached_agent}"
    expected_agent="$(awk -v name="$cached_agent" '$2 == name || $2 == "*" name { print $1; exit }' "$tmp_dir/SHA256SUMS")"
    if [[ -f "$existing_agent" && "$expected_agent" =~ ^[0-9a-fA-F]{64}$ ]] && printf '%s  %s\n' "$expected_agent" "$existing_agent" | sha256sum -c - >/dev/null 2>&1; then
      cp -- "$existing_agent" "$tmp_dir/$cached_agent"
    else
      fetch_file "$cached_agent" "$tmp_dir/$cached_agent"
    fi
    verify_file "$cached_agent" "$tmp_dir/$cached_agent"
    chmod 0755 "$tmp_dir/$cached_agent"
  done
fi

updater_available=0
if fetch_file "faroos-update" "$tmp_dir/faroos-update" 2>/dev/null && verify_file "faroos-update" "$tmp_dir/faroos-update" 2>/dev/null; then
  chmod 0755 "$tmp_dir/faroos-update"
  updater_available=1
fi

install_server_cache() {
  [[ "$component" == "server" ]] || return 0
  local cache_dir="${root_prefix}/var/lib/faroos/server/downloads" agent_arch
  install -d -o faroos -g faroos -m 0755 "$cache_dir"
  for agent_arch in amd64 arm64; do
    install -o faroos -g faroos -m 0755 "$tmp_dir/faroos-agent-linux-${agent_arch}" "$cache_dir/faroos-agent-linux-${agent_arch}"
  done
}

install_server_feed() {
  [[ "$component" == "server" ]] || return 0
  local feed_dir="${root_prefix}/var/lib/faroos/server/update-feed" agent_arch
  install -d -o faroos -g faroos -m 0755 "$feed_dir"
  for agent_arch in amd64 arm64; do
    install -o faroos -g faroos -m 0755 "$tmp_dir/faroos-agent-linux-${agent_arch}" "$feed_dir/faroos-agent-linux-${agent_arch}.new"
    mv -f -- "$feed_dir/faroos-agent-linux-${agent_arch}.new" "$feed_dir/faroos-agent-linux-${agent_arch}"
  done
  if (( updater_available )); then
    install -o faroos -g faroos -m 0755 "$tmp_dir/faroos-update" "$feed_dir/faroos-update.new"
    mv -f -- "$feed_dir/faroos-update.new" "$feed_dir/faroos-update"
  fi
  install -o faroos -g faroos -m 0644 "$tmp_dir/SHA256SUMS" "$feed_dir/SHA256SUMS.new"
  mv -f -- "$feed_dir/SHA256SUMS.new" "$feed_dir/SHA256SUMS"
  printf '%s\n' "$release_tag" >"$feed_dir/VERSION.new"
  chown faroos:faroos "$feed_dir/VERSION.new"
  chmod 0644 "$feed_dir/VERSION.new"
  mv -f -- "$feed_dir/VERSION.new" "$feed_dir/VERSION"
}

if (( ! needs_binary_update )); then
  install_server_cache
  install_server_feed
  if (( updater_available )); then
    install -o root -g root -m 0755 "$tmp_dir/faroos-update" "${root_prefix}/usr/local/libexec/faroos-${component}-update"
  fi
  echo "FaroOS ${component} is already current (${release_tag})."
  exit 0
fi

backup_dir="$state_dir/backups/$(date -u +%Y%m%dT%H%M%SZ)-${current_version:-unknown}"
install -d -o root -g root -m 0700 "$backup_dir"
cp -a -- "$binary_target" "$backup_dir/$binary_name"
replaced=0

rollback() {
  local status=$?
  if (( replaced )); then
    echo "FaroOS updater: update failed; restoring ${current_version:-previous version}." >&2
    install -o root -g root -m 0755 "$backup_dir/$binary_name" "$binary_target"
    systemctl restart "faroos-${component}.service" || true
  fi
  exit "$status"
}
trap rollback ERR

install -o root -g root -m 0755 "$tmp_dir/$asset_name" "$binary_target.new"
mv -f -- "$binary_target.new" "$binary_target"
replaced=1

install_server_cache
install_server_feed

systemctl restart "faroos-${component}.service"
sleep 2
systemctl is-active --quiet "faroos-${component}.service"

printf '%s\n' "$release_tag" >"$state_dir/current-version"
chmod 0644 "$state_dir/current-version"
replaced=0
trap - ERR

# Keep only the five newest recoverable versions.
mapfile -t old_backups < <(find "$state_dir/backups" -mindepth 1 -maxdepth 1 -type d -printf '%T@ %p\n' | sort -nr | tail -n +6 | cut -d' ' -f2-)
for old_backup in "${old_backups[@]}"; do
  rm -rf -- "$old_backup"
done

if (( updater_available )); then
  install -o root -g root -m 0755 "$tmp_dir/faroos-update" "${root_prefix}/usr/local/libexec/faroos-${component}-update"
fi

echo "FaroOS ${component} updated automatically: ${current_version:-unknown} -> ${release_tag}."
