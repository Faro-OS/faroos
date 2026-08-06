# FaroOS

Open-source, self-hosted control panel for managing one or many servers from a single dashboard — Docker containers, storage, an app store, terminal and file access — with a ZimaOS-inspired UI. Unlike CasaOS/ZimaOS, FaroOS is built from day one to manage a fleet of servers, not just one.

## Why

CasaOS and ZimaOS give you a nice dashboard for **one** machine. ZimaOS specifically only ships as a bare-metal/VM OS image, so it can't be layered onto a server you already use for other things, and neither offers real multi-server management. FaroOS is meant to fill that gap: install it anywhere, add as many servers as you have, manage them all from one place — a "faro" (lighthouse) watching over the whole fleet.

## Architecture

```
agent/    Go binary that runs on each managed server. Connects OUTBOUND to
          the central panel over a TLS websocket using a rotating pairing
          token — works even behind NAT, no inbound ports required. Reports
          system stats and can be told to manage local Docker containers,
          storage, files, etc.

server/   Go binary: the central panel. Serves the web UI, the API, and
          holds the registry of paired nodes. Runs as a normal service on
          any one of your machines (or a dedicated one) — not a special
          appliance.

web/      SvelteKit + Tailwind frontend. Dashboard, node list, containers,
          storage, app store, terminal, file manager, settings. Dark/light
          themes from day one.

desktop/  Tauri wrapper around web/ for native Windows/macOS/Linux apps.
          Each install can act purely as a remote client, or opt in to also
          run the agent locally and register itself as a managed node.
```

## Status

Early scaffolding — not usable yet. See `docs/decisions.md` for the architecture decisions made so far.

## License

AGPL-3.0. See `LICENSE`. Chosen so that anyone offering FaroOS as a hosted service must also share their modifications back to the community.

## Contributing

Open to contributors from day one. CI runs lint/build/test on every PR. No CLA required. (CONTRIBUTING.md coming soon.)
