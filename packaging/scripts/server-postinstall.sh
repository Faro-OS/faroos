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
install -d -m 0755 -o faroos -g faroos /var/lib/faroos /var/lib/faroos/server
chown -R -h faroos:faroos /var/lib/faroos/server

if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload || true
  systemctl enable faroos-server.service || true
fi
