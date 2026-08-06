#!/usr/bin/env bash

set -euo pipefail

REPO="${REPO:-gonzalo/faroos}"
VERSION="${VERSION:-latest}"

die() {
  echo "Error: $*" >&2
  exit 1
}

usage() {
  echo "Usage: $0 [agent|server]" >&2
  exit 2
}

if (( $# > 1 )); then
  usage
fi

component="${1:-}"
if [[ -z "$component" ]]; then
  component="agent"
  echo "No component specified; installing the FaroOS agent."
fi

case "$component" in
  agent | server) ;;
  *) usage ;;
esac

[[ "$(uname -s)" == "Linux" ]] || die "FaroOS installation is currently supported only on Linux."

case "$(uname -m)" in
  x86_64 | amd64) arch="amd64" ;;
  aarch64 | arm64) arch="arm64" ;;
  *) die "Unsupported architecture: $(uname -m). Supported architectures are amd64 and arm64." ;;
esac

(( EUID == 0 )) || die "Run this installer as root (for example, pipe it to 'sudo bash')."

for command_name in curl getent groupadd id install systemctl useradd usermod; do
  command -v "$command_name" >/dev/null 2>&1 || die "Required command not found: $command_name"
done

release_tag="$VERSION"
if [[ "$VERSION" == "latest" ]]; then
  echo "Resolving the latest FaroOS release..."
  release_json="$(curl -fsSL --retry 3 \
    -H "Accept: application/vnd.github+json" \
    "https://api.github.com/repos/${REPO}/releases/latest")" || die "Could not query the latest release for ${REPO}."
  release_tag="$(sed -nE 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/p' <<<"$release_json")"
  [[ -n "$release_tag" ]] || die "The GitHub API response did not contain a release tag."
fi

asset_name="faroos-${component}-linux-${arch}"
binary_url="https://github.com/${REPO}/releases/download/${release_tag}/${asset_name}"
unit_name="faroos-${component}.service"
unit_url="https://raw.githubusercontent.com/${REPO}/${release_tag}/packaging/systemd/${unit_name}"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

echo "Downloading FaroOS ${component} ${release_tag} for linux/${arch}..."
curl -fL --retry 3 -o "${tmp_dir}/${asset_name}" "$binary_url" || die "Could not download ${binary_url}"
curl -fL --retry 3 -o "${tmp_dir}/${unit_name}" "$unit_url" || die "Could not download ${unit_url}"

if ! getent group faroos >/dev/null; then
  groupadd --system faroos
fi

if ! id -u faroos >/dev/null 2>&1; then
  nologin_shell=/usr/sbin/nologin
  if [[ ! -x "$nologin_shell" ]]; then
    nologin_shell=/sbin/nologin
  fi
  if [[ ! -x "$nologin_shell" ]]; then
    nologin_shell=/bin/false
  fi
  useradd --system --gid faroos --home-dir /var/lib/faroos --create-home --shell "$nologin_shell" faroos
fi

install -d -m 0755 /etc/faroos /etc/systemd/system
install -d -m 0755 -o faroos -g faroos /var/lib/faroos "/var/lib/faroos/${component}"
install -m 0755 "${tmp_dir}/${asset_name}" "/usr/local/bin/faroos-${component}"
install -m 0644 "${tmp_dir}/${unit_name}" "/etc/systemd/system/${unit_name}"

if getent group docker >/dev/null; then
  usermod -aG docker faroos
elif [[ "$component" == "agent" ]]; then
  echo "Warning: the docker group does not exist. Add the faroos user to it after installing Docker:" >&2
  echo "  sudo usermod -aG docker faroos" >&2
fi

systemctl daemon-reload

agent_env_is_configured() {
  [[ -f /etc/faroos/agent.env ]] &&
    grep -Eq '^FAROOS_SERVER=.+$' /etc/faroos/agent.env &&
    grep -Eq '^FAROOS_NODE_ID=.+$' /etc/faroos/agent.env &&
    grep -Eq '^FAROOS_TOKEN=.+$' /etc/faroos/agent.env
}

write_agent_env() {
  local server_url node_id token escaped_value env_tmp

  echo
  echo "Pair this machine from Dashboard -> Add server in the FaroOS panel."
  echo "The panel will provide the server URL, node ID, and token requested below."

  server_url=""
  while [[ -z "$server_url" ]]; do
    read -r -p "FAROOS_SERVER (websocket URL): " server_url
  done

  node_id=""
  while [[ -z "$node_id" ]]; do
    read -r -p "FAROOS_NODE_ID: " node_id
  done

  token=""
  while [[ -z "$token" ]]; do
    read -r -s -p "FAROOS_TOKEN: " token
    echo
  done

  env_tmp="${tmp_dir}/agent.env"
  : >"$env_tmp"
  for escaped_value in "$server_url" "$node_id" "$token"; do
    [[ "$escaped_value" != *$'\n'* ]] || die "Agent settings must not contain newlines."
  done
  server_url="${server_url//\\/\\\\}"
  server_url="${server_url//\"/\\\"}"
  node_id="${node_id//\\/\\\\}"
  node_id="${node_id//\"/\\\"}"
  token="${token//\\/\\\\}"
  token="${token//\"/\\\"}"
  printf 'FAROOS_SERVER="%s"\nFAROOS_NODE_ID="%s"\nFAROOS_TOKEN="%s"\n' \
    "$server_url" "$node_id" "$token" >"$env_tmp"
  install -m 0600 -o root -g root "$env_tmp" /etc/faroos/agent.env
}

if [[ "$component" == "agent" ]]; then
  if agent_env_is_configured; then
    echo "Using the existing configuration in /etc/faroos/agent.env."
    chown root:root /etc/faroos/agent.env
    chmod 0600 /etc/faroos/agent.env
  elif [ -t 0 ]; then
    write_agent_env
  else
    echo
    echo "The agent was installed but was not started because this shell has no interactive input."
    echo "In the FaroOS panel, use Dashboard -> Add server, then create /etc/faroos/agent.env with:"
    cat <<'EOF'
FAROOS_SERVER="value-from-the-panel"
FAROOS_NODE_ID="value-from-the-panel"
FAROOS_TOKEN="value-from-the-panel"
EOF
    echo "Protect the file and start the service with:"
    echo "  sudo chmod 600 /etc/faroos/agent.env"
    echo "  sudo systemctl enable --now faroos-agent.service"
    exit 0
  fi

  systemctl enable "$unit_name"
  systemctl restart "$unit_name"
  echo "FaroOS agent ${release_tag} is installed and running."
else
  systemctl enable "$unit_name"
  systemctl restart "$unit_name"
  host_name="$(hostname -f 2>/dev/null || hostname)"
  echo "FaroOS server ${release_tag} is installed and running."
  echo "Open http://${host_name}:8090 to create the first administrator account."
fi
