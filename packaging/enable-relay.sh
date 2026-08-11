#!/usr/bin/env bash

set -Eeuo pipefail

die() {
  echo "FaroOS relay setup: $*" >&2
  exit 1
}

(( EUID == 0 )) || die "run once with sudo"
(( $# == 1 )) || die "usage: $0 https://relay.example.com"
relay_base="${1%/}"
[[ "$relay_base" == https://* ]] || die "the public relay must use HTTPS"
[[ "$relay_base" != *$'\n'* && "$relay_base" != *$'\r'* && "$relay_base" != *'"'* ]] || die "invalid relay URL"

for command_name in awk install mktemp systemctl; do
  command -v "$command_name" >/dev/null 2>&1 || die "required command not found: $command_name"
done

install -d -o root -g root -m 0755 /etc/faroos
env_file="/etc/faroos/server.env"
env_tmp="$(mktemp /etc/faroos/server.env.XXXXXX)"
trap 'rm -f "$env_tmp"' EXIT
if [[ -f "$env_file" ]]; then
  awk '!/^FAROOS_RELAY_URL=/ && !/^FAROOS_RELAY_PUBLIC_BASE=/' "$env_file" >"$env_tmp"
fi
relay_ws="wss://${relay_base#https://}/relay/connect"
printf 'FAROOS_RELAY_URL="%s"\nFAROOS_RELAY_PUBLIC_BASE="%s/p"\n' "$relay_ws" "$relay_base" >>"$env_tmp"
install -o root -g root -m 0644 "$env_tmp" "$env_file"

systemctl restart faroos-server.service
systemctl is-active --quiet faroos-server.service || die "FaroOS server did not restart"
echo "FaroOS Relay enabled through $relay_base."
