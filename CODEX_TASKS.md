# Codex tasks

Scope for this pass: **only** the placeholder route pages listed below, inside `web/src/routes/`. Do not touch `web/src/lib/`, `internal/`, `cmd/`, or the Dashboard (`+page.svelte` at the root route) — those are being worked on elsewhere in parallel.

## Context

FaroOS is an open-source, self-hosted control panel for managing multiple servers (think CasaOS/ZimaOS but multi-server). Frontend is SvelteKit 5 (runes mode) + Tailwind v4, built as a static SPA served by a Go backend. The sidebar (`web/src/lib/components/Sidebar.svelte`) already links to these routes, but the pages don't exist yet, so they 404.

Existing conventions to follow (look at `web/src/routes/+page.svelte` and `web/src/lib/components/NodeCard.svelte` for reference):
- Use the `TopBar` component (`$lib/components/TopBar.svelte`) at the top of every page, passing a `title` prop.
- Use the CSS variables already defined in `web/src/routes/layout.css` for colors — `var(--surface)`, `var(--border)`, `var(--fg)`, `var(--fg-muted)`, `var(--fg-subtle)`, `var(--accent)`, `var(--accent-fg)`, `var(--bg)`, `var(--track)`. Don't hardcode hex colors or use Tailwind's default gray/blue palette — everything must theme correctly in both light and dark (the app toggles `data-theme` on `<html>`).
- Rounded-2xl cards (`rounded-2xl border border-[var(--border)] bg-[var(--surface)] p-5`) are the base visual unit, matching `NodeCard.svelte`.
- Svelte 5 runes only (`$state`, `$derived`, `$props`, `$effect`) — no `export let`, no Svelte 4 stores unless there's a real reason.
- TypeScript, no `any`.

## Pages to build

All under `web/src/routes/<name>/+page.svelte`. Each currently 404s because the sidebar links exist but the route doesn't.

1. **`/nodes`** — "Servers" page. For now this can reuse the same node list as the dashboard (`$lib/api.ts` already exports `listNodes()`), but as a **table** instead of cards: columns Name, Status (connected/disconnected dot), CPU %, Memory, Disk, Uptime, Last seen. Sortable by clicking column headers is a nice-to-have, not required.
2. **`/containers`** — Empty state page: centered message "No servers connected yet" if `listNodes()` returns zero connected nodes, otherwise "Container management is coming soon" — this page is a placeholder, there's no container API yet. Keep it simple, just the empty-state pattern already used in the dashboard's `+page.svelte` (copy that visual pattern: dashed border box, centered text).
3. **`/storage`** — Same placeholder pattern as containers: "Storage management is coming soon."
4. **`/apps`** — "App Store" placeholder. Slightly more effort: show a static, hardcoded grid of ~6 example app cards (name, one-line description, a "Coming soon" disabled button) to communicate intent — e.g. Nextcloud, Immich, Vaultwarden, Jellyfin, Pi-hole, Uptime Kuma. Purely visual, no backend calls.
5. **`/terminal`** — Placeholder: "Web terminal is coming soon." Same dashed-box pattern.
6. **`/files`** — Placeholder: "File manager is coming soon." Same dashed-box pattern.
7. **`/settings`** — Slightly more real: a simple form-like page with a couple of sections in cards — "Appearance" (a light/dark toggle that calls `toggleTheme()` / `getTheme()` from `$lib/theme.svelte.ts`, redundant with the top bar toggle but expected here too) and "About" (shows "FaroOS" name, and links to the (future) GitHub repo — use a placeholder `#` href for now, don't invent a real URL).

## Definition of done

- `npx svelte-check --tsconfig ./tsconfig.json` (run from `web/`) reports 0 errors.
- `npm run build` (from `web/`) succeeds.
- All 7 routes are reachable from the sidebar without 404s.
- No new dependencies added to `package.json` without a clear reason.

Do not run `git commit`. Leave changes staged/unstaged for review.
