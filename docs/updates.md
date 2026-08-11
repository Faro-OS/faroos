# Automatic updates

FaroOS enables automatic updates for both the central panel and managed-node agents.

## Stable installations

The systemd timers `faroos-server-update.timer` and `faroos-agent-update.timer` check the latest published GitHub release hourly with a randomized delay. Each update:

1. downloads the release checksum manifest;
2. verifies every binary with SHA-256;
3. rejects release downgrades;
4. saves the installed binary as a recoverable backup;
5. installs and restarts the relevant service;
6. restores the previous binary if the service does not become active.

The server updater also refreshes both Linux agent architectures used by the dashboard's one-command server installer and publishes them through the panel's `/install/update/` feed. Agents installed from the dashboard check that feed every minute, so servers in other networks receive the panel's current agent build through their outbound HTTP(S) connection. They do not need access to the panel machine's local staging directory.

For servers outside the panel's LAN, FaroOS Relay provides the reachable HTTPS address automatically while both the panel and agent make outbound connections. A manually supplied public HTTPS domain or mesh-VPN address remains available as a fallback. FaroOS does not require opening an inbound port on each managed server.

Automatic updates can be disabled without uninstalling FaroOS:

```sh
sudo systemctl disable --now faroos-server-update.timer
sudo systemctl disable --now faroos-agent-update.timer
```

## Local development channel

Maintainers can opt into local automatic deployment. This grants the owner of the staging directory permission to supply binaries that the root updater installs, so it must only be enabled on a development machine controlled by that maintainer.

Stage a build as the developer user:

```sh
./packaging/stage-local-update.sh /var/tmp/faroos-local-update
```

Enable the local channel once with administrator authorization:

```sh
sudo ./packaging/enable-local-updates.sh /var/tmp/faroos-local-update
```

After that one-time setup, rerunning the staging script is enough. The local timers detect and apply a new staged version within roughly 15 seconds, without additional `sudo` commands. Once the panel applies it, dashboard-installed remote agents discover the matching agent build through the panel feed.
