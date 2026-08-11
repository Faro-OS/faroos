#!/usr/bin/env bash

set -Eeuo pipefail

config_file="/etc/faroos/update.env"
[[ -r "$config_file" ]] || exit 0

FAROOS_UPDATE_CHANNEL="stable"
FAROOS_UPDATE_LOCAL_DIR=""
# The file is administrator-owned and uses the same shell-compatible format
# consumed by systemd's EnvironmentFile directive.
# shellcheck disable=SC1090
source "$config_file"

[[ "$FAROOS_UPDATE_CHANNEL" == "local" ]] || exit 0
[[ -n "$FAROOS_UPDATE_LOCAL_DIR" && -d "$FAROOS_UPDATE_LOCAL_DIR" ]] || {
  echo "FaroOS local update channel is enabled but its staging directory is unavailable." >&2
  exit 1
}
[[ -w "$(dirname "$FAROOS_UPDATE_LOCAL_DIR")" ]] || {
  echo "FaroOS local update staging directory is not writable by this developer." >&2
  exit 1
}

exec "$(dirname "${BASH_SOURCE[0]}")/stage-local-update.sh" "$FAROOS_UPDATE_LOCAL_DIR"
