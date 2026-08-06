# Architecture decisions

Captured from the initial planning session. Revisit and update as the project evolves — this is a snapshot, not a contract.

| Area | Decision | Why |
|---|---|---|
| Backend / agent language | Go | Static binaries, easy cross-compilation to Linux/macOS/Windows, minimal RAM footprint, no runtime to install. |
| Frontend | SvelteKit + Tailwind | No virtual DOM, small bundles, compiles to static assets the Go server can serve directly — no Node process needed in production. |
| Desktop app | Tauri (Rust) | Uses the OS-native webview instead of bundling Chromium (Electron), far lower RAM/disk footprint. |
| Multi-server model | Agent per server, outbound connection | Works behind NAT/no public IP, no inbound ports to open, matches how Tailscale/Rancher agents behave. |
| Agent↔server auth | Rotating pairing tokens + TLS | Simple to implement without external infra; no need to reimplement a mesh network like WireGuard/Tailscale for v1. |
| Central panel deployment | Installable service on any managed server | Reachable from any browser (including mobile) without a dedicated appliance. |
| Failure mode | Servers keep running independently if the panel is down | Panel is a control/visibility plane, not an orchestrator in the critical path (unlike Kubernetes-style active orchestration). |
| License | AGPL-3.0 | Same category as Nextcloud/Immich — anyone offering it as a hosted service must share modifications back. |
| MVP scope | Full NAS: Docker multi-server management + storage/RAID + app store + web terminal + file manager | Deliberate choice to launch closer to feature parity with CasaOS/ZimaOS rather than a bare container manager. |
| Linux install | curl script + .deb/.rpm + Docker image | Bare-metal ISO explicitly deferred — it's a project of its own (kernel, partitioning, installer UX). |
| Desktop apps | Modular: pure remote client, or opt-in local agent to also become a managed node | User decides per install; avoids forcing Docker Desktop dependency on everyone. |
| Code signing | Deferred | Avoid recurring cert costs (Apple Developer, Windows signing) until there's an audience; ship with "unknown publisher" warnings for now. |
| Design | Match ZimaOS's visual style AND navigation structure, adapted for multi-server | Sidebar + top resource dashboard + separate app store section, reinterpreted for a fleet of servers instead of one. |
| Theming | Dark + light from day one | Cheap to do correctly from the first component; expensive to retrofit. |
| CI/CD | GitHub Actions from the first commit | Project is open to contributors from day one — CI catches broken PRs early. |
| Telemetry | None | Matches the self-hosted/privacy-conscious audience this tool targets. |
| Demo/reference instance | Runs on the maintainer's existing home server | No extra cost; shares infra with other self-hosted services already running there. |
| Bare-metal ISO | Explicitly fase 2, not MVP | Too large a scope addition for v1. |
