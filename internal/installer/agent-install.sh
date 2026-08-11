#!/usr/bin/env bash
set -euo pipefail

die() {
  echo "FaroOS installer: $*" >&2
  exit 1
}

panel_url=""
server_url=""
node_id=""
token=""

while (( $# > 0 )); do
  case "$1" in
    --panel) panel_url="${2:-}"; shift 2 ;;
    --server) server_url="${2:-}"; shift 2 ;;
    --node) node_id="${2:-}"; shift 2 ;;
    --token) token="${2:-}"; shift 2 ;;
    *) die "unknown option: $1" ;;
  esac
done

(( EUID == 0 )) || die "run the command with sudo"
[[ "$(uname -s)" == "Linux" ]] || die "only Linux servers are currently supported"
[[ -n "$panel_url" && -n "$server_url" && -n "$node_id" && -n "$token" ]] || die "pairing information is incomplete"

for value in "$panel_url" "$server_url" "$node_id" "$token"; do
  [[ "$value" != *$'\n'* ]] || die "pairing values cannot contain newlines"
done

case "$(uname -m)" in
  x86_64 | amd64) arch="amd64" ;;
  aarch64 | arm64) arch="arm64" ;;
  *) die "unsupported architecture: $(uname -m)" ;;
esac

install_docker() {
  command -v docker >/dev/null 2>&1 && return
  echo "Installing Docker..."
  if command -v apt-get >/dev/null 2>&1; then
    apt-get update
    DEBIAN_FRONTEND=noninteractive apt-get install -y docker.io ca-certificates curl
  elif command -v dnf >/dev/null 2>&1; then
    dnf install -y docker curl ca-certificates
  elif command -v yum >/dev/null 2>&1; then
    yum install -y docker curl ca-certificates
  elif command -v pacman >/dev/null 2>&1; then
    pacman -Sy --noconfirm docker curl ca-certificates
  elif command -v zypper >/dev/null 2>&1; then
    zypper --non-interactive install docker curl ca-certificates
  else
    die "Docker is missing and this Linux distribution has no supported package manager"
  fi
}

command -v curl >/dev/null 2>&1 || die "curl is required"
command -v systemctl >/dev/null 2>&1 || die "systemd is required"
install_docker
systemctl enable --now docker.service >/dev/null 2>&1 || systemctl enable --now docker >/dev/null 2>&1 || die "Docker could not be started"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT
binary_url="${panel_url%/}/install/agent/${arch}"
echo "Downloading FaroOS agent for linux/${arch}..."
curl -fL --retry 3 -o "$tmp_dir/faroos-agent" "$binary_url" || die "could not download $binary_url"
chmod 0755 "$tmp_dir/faroos-agent"

install -d -m 0755 /etc/faroos /usr/local/libexec /var/lib/faroos /var/lib/faroos/agent /var/lib/faroos/apps
install -m 0755 "$tmp_dir/faroos-agent" /usr/local/bin/faroos-agent
curl -fL --retry 3 -o "$tmp_dir/faroos-update" "${panel_url%/}/install/updater" || die "could not download the automatic updater"
install -o root -g root -m 0755 "$tmp_dir/faroos-update" /usr/local/libexec/faroos-agent-update

escape_env() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  printf '%s' "$value"
}

env_file="$tmp_dir/agent.env"
printf 'FAROOS_SERVER="%s"\nFAROOS_NODE_ID="%s"\nFAROOS_TOKEN="%s"\nFAROOS_FILES_ROOT="/"\nFAROOS_APPS_DATA_DIR="/var/lib/faroos/apps"\n' \
  "$(escape_env "$server_url")" "$(escape_env "$node_id")" "$(escape_env "$token")" >"$env_file"
install -o root -g root -m 0600 "$env_file" /etc/faroos/agent.env

update_env="$tmp_dir/agent-update.env"
printf 'FAROOS_UPDATE_CHANNEL="panel"\nFAROOS_UPDATE_URL="%s/install/update"\n' \
  "$(escape_env "${panel_url%/}")" >"$update_env"
install -o root -g root -m 0644 "$update_env" /etc/faroos/agent-update.env

cat >/etc/systemd/system/faroos-agent.service <<'EOF'
[Unit]
Description=FaroOS node agent
Wants=network-online.target
After=network-online.target docker.service

[Service]
Type=simple
User=root
Group=root
WorkingDirectory=/var/lib/faroos/agent
EnvironmentFile=/etc/faroos/agent.env
ExecStart=/usr/local/bin/faroos-agent
Restart=on-failure
RestartSec=5s
TimeoutStopSec=30s

[Install]
WantedBy=multi-user.target
EOF

cat >/etc/systemd/system/faroos-agent-update.service <<'EOF'
[Unit]
Description=Update FaroOS node agent safely
Wants=network-online.target
After=network-online.target

[Service]
Type=oneshot
User=root
Group=root
EnvironmentFile=-/etc/faroos/update.env
EnvironmentFile=-/etc/faroos/agent-update.env
ExecStart=/usr/local/libexec/faroos-agent-update --component agent
ProtectHome=true
ProtectSystem=strict
ReadWritePaths=/usr/local/bin /usr/local/libexec /var/lib/faroos
NoNewPrivileges=true
EOF

cat >/etc/systemd/system/faroos-agent-update.timer <<'EOF'
[Unit]
Description=Check automatically for FaroOS agent updates

[Timer]
OnBootSec=1min
OnUnitActiveSec=1min
RandomizedDelaySec=10s
Persistent=true
Unit=faroos-agent-update.service

[Install]
WantedBy=timers.target
EOF

systemctl daemon-reload
systemctl enable --now faroos-agent.service
systemctl enable --now faroos-agent-update.timer
sleep 1
systemctl is-active --quiet faroos-agent.service || die "the agent service did not start"

echo
echo "FaroOS agent installed, configured and connected."
echo "This server now exposes Docker, terminal, storage and its complete filesystem to your authenticated FaroOS panel."
