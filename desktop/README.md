# FaroOS desktop app (planned, not yet scaffolded)

This directory will hold the Tauri wrapper that produces the Windows `.exe`, macOS `.app`/`.dmg`, and Linux `.AppImage`/`.deb` builds. It's intentionally not scaffolded yet — `cargo install tauri-cli` pulls a non-trivial Rust dependency tree, and this was explicitly deprioritized behind the server/agent/web loop.

## Plan

- Wraps the same static build in `web/build` (Tauri's `frontendDist`) — no separate UI code, it's the same SvelteKit app running in the OS-native webview instead of a normal browser tab.
- Two modes, chosen by the user after install (per the "modular apps" decision in `docs/decisions.md`):
  1. **Pure remote client** — just opens the FaroOS UI pointed at a server the user already has running elsewhere. No local agent, no local Docker dependency.
  2. **Also a managed node** — additionally runs the same `agent` binary (see `../agent`, once it exists as its own module — currently `cmd/agent`) as a sidecar process, so this machine shows up in the dashboard like any other server. Requires Docker to be available locally if the user wants to manage containers on it.
- The mode is a setting in the app, not a build-time flag — same installer for both.

## To actually scaffold this

```
cargo install tauri-cli   # one-time, several minutes
cd desktop
cargo tauri init          # point frontendDist at ../web/build, devUrl at http://localhost:5173
```

Then wire the "also a managed node" toggle to spawn/stop the agent binary as a Tauri sidecar (`tauri.conf.json` → `bundle.externalBin`), passing it the same `FAROOS_SERVER` / `FAROOS_NODE_ID` / `FAROOS_TOKEN` the web pairing flow already generates.

Code signing (Apple Developer cert, Windows Authenticode) is deferred per `docs/decisions.md` — early builds will show "unknown publisher" warnings, which is accepted for now.
