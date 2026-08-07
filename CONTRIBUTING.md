# Contributing to FaroOS

Thanks for looking at this. It's early — expect things to move fast and occasionally break. No CLA, no formal process to gate a first PR on; just read this, make sure CI would pass, and open it.

## Building it

You need Go 1.25+, Node 22+, and Docker (only if you want to actually exercise the container/app-store features locally — the rest of the app runs fine without it).

```sh
git clone https://github.com/Faro-OS/faroos.git
cd faroos
go build ./...           # cmd/agent and cmd/server compile against the
                          # tracked placeholder in internal/webui/dist —
                          # this works from a completely fresh clone with
                          # no Node involved
```

To get the real UI instead of the placeholder:

```sh
cd web
npm install
npm run build             # writes into ../internal/webui/dist, which
                           # cmd/server go:embeds — don't commit that
                           # output, it's gitignored except for the
                           # placeholder itself
cd ..
go build -o faroos-server ./cmd/server
```

Running it locally:

```sh
FAROOS_PORT=8090 FAROOS_DB=./dev.db ./faroos-server
# open http://localhost:8090, create the admin account

# in another terminal, pair a node from the dashboard, then:
FAROOS_SERVER="ws://localhost:8090/api/agent/connect" \
FAROOS_NODE_ID="<from the pairing dialog>" \
FAROOS_TOKEN="<from the pairing dialog>" \
go run ./cmd/agent
```

For frontend-only iteration, `cd web && npm run dev` runs SvelteKit's dev server; set `VITE_API_BASE=http://localhost:8090` in `web/.env` first if the panel isn't on the same origin.

## Before opening a PR

```sh
gofmt -l .                                  # must be empty
go vet ./...
go test ./...

cd web
npx svelte-check --tsconfig ./tsconfig.json # must be 0 errors
npm run build
```

CI runs all of this (plus cross-compiling the Go binaries for every supported platform) on every push — matching it locally first saves a round trip.

## Code style

- Comments explain *why*, not *what* — if removing a comment wouldn't confuse a future reader, it shouldn't be there. Well-named things don't need comments restating their name.
- No speculative abstraction. If something's only used once, it doesn't need to be pluggable yet.
- Backend: standard Go idioms, no framework beyond the standard library + `gorilla/websocket`. New third-party dependencies should earn their place — several packages here (`dockerclient`, `sysstats`) talk to Docker/`/proc` directly over plain HTTP/file reads specifically to avoid pulling in heavier SDKs.
- Frontend: Svelte 5 runes (`$state`, `$derived`, `$effect`) — no `export let`, no Svelte 4 stores unless there's a specific reason. TypeScript, no `any`.
- If you're touching something security-sensitive (the file manager's path sandboxing, auth, the port-conflict "stop and free" flow), add a test that would catch a regression, not just a manual check — see `internal/fileops/fileops_test.go` for the shape that's expected (e.g. explicit path-traversal attempts asserted to fail).

## Where things live

See the Architecture section in `README.md` for the directory layout. `docs/decisions.md` has the reasoning behind the bigger architectural calls (why Go, why SQLite, why agents connect outbound instead of the panel dialing in, etc.) — worth a skim before proposing something that cuts against one of them, so we're arguing about the actual tradeoff instead of re-deriving it from scratch.

## Reporting bugs / proposing features

GitHub Issues. For anything touching the file manager's sandboxing, the port-conflict handling, or auth, please say so explicitly in the issue/PR description — those get read more carefully.
