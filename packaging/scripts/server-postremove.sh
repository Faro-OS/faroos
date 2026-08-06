#!/bin/sh

set -eu

case "${1:-}" in
  0 | remove | purge)
    if command -v systemctl >/dev/null 2>&1; then
      systemctl disable --now faroos-server.service >/dev/null 2>&1 || true
    fi
    if [ ! -e /usr/local/bin/faroos-agent ] && [ ! -e /usr/local/bin/faroos-server ]; then
      if id -u faroos >/dev/null 2>&1; then
        userdel faroos || true
      fi
      if getent group faroos >/dev/null; then
        groupdel faroos || true
      fi
    fi
    ;;
esac

if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload || true
fi
