# FaroOS

Open-source, self-hosted control panel for managing one or many servers from a single dashboard — Docker containers, storage, an app store, terminal and file access — with a ZimaOS-inspired UI. Unlike CasaOS/ZimaOS, FaroOS is built from day one to manage a fleet of servers, not just one.

## Why

CasaOS and ZimaOS give you a nice dashboard for **one** machine. ZimaOS specifically only ships as a bare-metal/VM OS image, so it can't be layered onto a server you already use for other things, and neither offers real multi-server management. FaroOS is meant to fill that gap: install it anywhere, add as many servers as you have, manage them all from one place — a "faro" (lighthouse) watching over the whole fleet.

## Architecture

```
cmd/agent/    Entry point for the agent binary. Connects OUTBOUND to the
              central panel over a websocket using a rotating pairing
              token — works even behind NAT, no inbound ports required.
              Reports system stats and runs commands the panel sends it
              (Docker, files, terminal, port checks).

cmd/server/   Entry point for the central panel binary. Serves the API,
              the websocket hub agents connect to, and the web UI
              (embedded into the binary at build time — see
              internal/webui). Runs as a normal service on any one of
              your machines, or a dedicated one — not a special appliance.

internal/     Where the actual logic lives, shared between cmd/agent and
              cmd/server as needed: registry (paired nodes, SQLite), auth
              (admin session), api (HTTP/websocket handlers), dockerclient,
              fileops (sandboxed file manager), termsession (PTY), catalog
              + appcatalog (the App Store, curated + imported), sysstats.

web/          SvelteKit + Tailwind frontend — a single-page control panel
              (dashboard, servers, containers, storage, app store,
              terminal, files, settings), not a multi-route app. Builds
              into internal/webui/dist, which cmd/server go:embeds.

desktop/      Planned Tauri wrapper around web/ for native Windows/macOS/
              Linux apps — not built yet, see desktop/README.md.

packaging/    curl-installer, .deb/.rpm (nfpm), and the bare-metal install
              ISO (Ubuntu autoinstall-based).
```

## Status

Functional end to end: pairing, live stats, Docker container management, a real web terminal, a sandboxed file manager, and an App Store with thousands of one-click-deployable apps (a curated set plus an imported [Unraid Community Applications](https://github.com/Squidly271/AppFeed) catalog, refreshed daily). Still early — expect rough edges. See `docs/decisions.md` for the architecture decisions made so far.

## License

AGPL-3.0. See `LICENSE`. Chosen so that anyone offering FaroOS as a hosted service must also share their modifications back to the community.

## Contributing

Open to contributors from day one. CI runs lint/build/test on every PR. No CLA required. (CONTRIBUTING.md coming soon.)

## Installing

Install the central panel, then open `http://<hostname>:8090` to create the first administrator account:

```sh
curl -fsSL https://raw.githubusercontent.com/Faro-OS/faroos/main/packaging/install.sh | sudo bash -s -- server
```

On each managed server, use Dashboard → Add server in the panel, then install the agent and follow the printed pairing instructions:

```sh
curl -fsSL https://raw.githubusercontent.com/Faro-OS/faroos/main/packaging/install.sh | sudo bash -s -- agent
```

Prebuilt `.deb` and `.rpm` packages are also available from GitHub Releases.

Starting from a blank machine instead? See `packaging/iso/` for a bootable, unattended-install ISO (Ubuntu Server underneath, not a custom distro).
