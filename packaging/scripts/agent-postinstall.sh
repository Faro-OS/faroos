#!/bin/sh

set -eu

if ! getent group faroos >/dev/null; then
  groupadd --system faroos
fi

if ! id -u faroos >/dev/null 2>&1; then
  nologin_shell=/usr/sbin/nologin
  if [ ! -x "$nologin_shell" ]; then
    nologin_shell=/sbin/nologin
  fi
  if [ ! -x "$nologin_shell" ]; then
    nologin_shell=/bin/false
  fi
  useradd --system --gid faroos --home-dir /var/lib/faroos --create-home --shell "$nologin_shell" faroos
fi

install -d -m 0755 /etc/faroos
install -d -m 0755 -o faroos -g faroos /var/lib/faroos /var/lib/faroos/agent
chown -R -h faroos:faroos /var/lib/faroos/agent

if getent group docker >/dev/null; then
  usermod -aG docker faroos
else
  echo "FaroOS agent: Docker is not installed or its group is unavailable." >&2
  echo "After installing Docker, run: usermod -aG docker faroos" >&2
fi

if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload || true
  systemctl enable faroos-agent.service || true
fi

if [ ! -f /etc/faroos/agent.env ]; then
  echo "Configure /etc/faroos/agent.env with values from Dashboard -> Add server, then start faroos-agent.service."
fi
