#!/usr/bin/env bash

set -Eeuo pipefail

die() {
  echo "FaroOS local updater setup: $*" >&2
  exit 1
}

(( EUID == 0 )) || die "run once with sudo"
(( $# == 1 )) || die "usage: $0 LOCAL_UPDATE_DIRECTORY"
[[ "$(uname -s)" == "Linux" ]] || die "only Linux is supported"

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
local_dir="$(realpath -e "$1")"
[[ -d "$local_dir" && -f "$local_dir/VERSION" && -f "$local_dir/SHA256SUMS" ]] || die "the local update directory is not staged"
[[ "$local_dir" != / && "$local_dir" != /tmp ]] || die "unsafe local update directory"
[[ "$local_dir" != /home/* ]] || die "use a directory outside /home because updater services protect home directories"

dir_mode="$(stat -c '%a' "$local_dir")"
if (( (8#$dir_mode & 0022) != 0 )); then
  die "local update directory must not be writable by group or others"
fi

install -d -o root -g root -m 0755 /etc/faroos /usr/local/libexec /var/lib/faroos/update
escaped_dir="${local_dir//\\/\\\\}"
escaped_dir="${escaped_dir//\"/\\\"}"
printf 'FAROOS_UPDATE_CHANNEL="local"\nFAROOS_UPDATE_LOCAL_DIR="%s"\n' "$escaped_dir" >/etc/faroos/update.env.new
install -o root -g root -m 0644 /etc/faroos/update.env.new /etc/faroos/update.env
rm -f /etc/faroos/update.env.new

enabled=0
for component in server agent; do
  [[ -x "/usr/local/bin/faroos-$component" ]] || continue
  install -o root -g root -m 0755 "$repo_dir/packaging/update.sh" "/usr/local/libexec/faroos-${component}-update"
  install -o root -g root -m 0644 "$repo_dir/packaging/systemd/faroos-${component}-update.service" "/etc/systemd/system/faroos-${component}-update.service"
  install -o root -g root -m 0644 "$repo_dir/packaging/systemd/faroos-${component}-update.timer" "/etc/systemd/system/faroos-${component}-update.timer"
  dropin_dir="/etc/systemd/system/faroos-${component}-update.timer.d"
  install -d -o root -g root -m 0755 "$dropin_dir"
  cat >"$dropin_dir/local.conf" <<'EOF'
[Timer]
OnBootSec=
OnActiveSec=10s
OnUnitActiveSec=
OnUnitActiveSec=15s
RandomizedDelaySec=
RandomizedDelaySec=0
EOF
  enabled=$((enabled + 1))
done
(( enabled > 0 )) || die "no FaroOS server or agent installation was found"

systemctl daemon-reload
for component in server agent; do
  [[ -x "/usr/local/bin/faroos-$component" ]] || continue
  systemctl enable --now "faroos-${component}-update.timer"
  systemctl start "faroos-${component}-update.service"
done

echo "FaroOS local automatic updates are enabled."
echo "Future staged builds will be installed without sudo commands."
