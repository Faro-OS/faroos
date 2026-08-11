# FaroOS Relay

FaroOS Relay connects panels and managed servers that are behind unrelated NATs. Neither side needs an inbound port, a public IP, Tailscale, or another VPN:

1. the panel creates a random panel ID and a separate 256-bit secret;
2. the panel opens one authenticated outbound WebSocket to the public relay;
3. the relay multiplexes short-lived streams over that connection;
4. panel and agent exchange an authenticated WebRTC offer through that initial
   path and use our STUN service to attempt UDP hole punching;
5. when ICE finds a route, the encrypted DTLS data channel becomes the active
   management connection and the relayed WebSocket closes.

An established direct connection does not depend on the FaroOS relay. If a
network uses a NAT combination that cannot be hole-punched, FaroOS retains the
existing HTTPS relay as a compatibility fallback. This fallback is unavoidable
for pairs of networks where neither side can accept a direct packet.

The public relay deliberately exposes only:

- `GET` and `HEAD` requests below `/install/`;
- WebSocket upgrades to `/api/agent/connect`.

It does not expose the panel UI, login, node API, terminal, files, containers, or settings. Each panel secret is stored only on the panel and as a SHA-256 hash in the relay database. A second client cannot claim an existing public panel ID with a different secret.

Production traffic must use HTTPS/WSS. TLS protects both public connections in transit. The relay is trusted infrastructure and terminates the public TLS connection, so its host and database must be administered as carefully as the central FaroOS service.

## Deploy one relay

Requirements:

- a public Linux server with Docker Compose;
- a DNS name pointing to that server;
- inbound TCP ports 80 and 443;
- inbound UDP port 3478 for the FaroOS STUN rendezvous service.

From the repository:

```sh
cd packaging/relay
cp .env.example .env
# Set RELAY_DOMAIN to the real DNS name.
docker compose up -d --build
```

Caddy obtains and renews the public TLS certificate. The relay database and Caddy state live in named Docker volumes. Back up the `relay-data` volume: it contains the hashes that prevent a known panel ID from being reclaimed with another secret.

If the public host already uses Nginx on ports 80 and 443, do not start the
bundled Caddy service. Publish the relay container on a loopback-only port such
as `127.0.0.1:18080` and add a separate Nginx `server_name` that proxies to it.
The managed deployment uses `packaging/relay/nginx-managed.conf` for this. Nginx
continues to own the public ports and routes each domain independently through
TLS SNI.

Verify it:

```sh
curl -fsS https://relay.example.com/healthz
```

## Connect a panel

For a self-hosted relay, enable its URL once on the panel host:

```sh
sudo ./packaging/enable-relay.sh https://relay.example.com
```

The panel creates `/var/lib/faroos/server/relay-credentials.json` on first start with mode `0600`. After the outbound connection succeeds, Dashboard → Add server automatically generates a relay-based one-line installer. The remote server needs ordinary outbound HTTPS only.

Official FaroOS builds use the managed service automatically, so end users do
not need the self-hosting command. A custom deployment can override these two
variables, or set `FAROOS_RELAY_DISABLED=1` to opt out:

```text
FAROOS_RELAY_URL="wss://relay.example.com/relay/connect"
FAROOS_RELAY_PUBLIC_BASE="https://relay.example.com/p"
```

P2P is enabled by default with `stun:relay.faroos.dev:3478`. Self-hosted
installations can set `FAROOS_STUN_URL` to their own STUN endpoint or
`FAROOS_P2P_DISABLED=1` to retain relay-only behavior.

## Current scaling model

The included service is a secure single-relay deployment: credentials persist in SQLite, while active tunnel sessions live in memory. Multiple independent relay instances require session-aware routing or a shared tunnel directory and are intentionally left for the managed-service deployment rather than pretending that ordinary HTTP load balancing is sufficient.
