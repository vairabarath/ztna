# Admin Dashboard (React + gRPC Gateway)

This feature adds a web admin console for provisioning:
- workspaces
- connector bootstrap profiles
- agent bootstrap profiles under each connector
- workspace-scoped edit/delete workflows for connectors and agents
- connector/agent verification + active status based on controller device records

It is intentionally split into two services:
- `admin-api` (Node/TypeScript): REST API + install-link generator + gRPC controller client
- `admin-web` (React/Vite): browser dashboard UI

## Why a Gateway

Browsers cannot call raw gRPC (`grpc-go` server) directly without grpc-web infrastructure. The `admin-api` service bridges browser requests to your control-plane gRPC API.

## Start Steps

1. Ensure controller is running with TLS and valid admin token.
2. Install Node 20+.
3. Install dependencies:

```bash
cd admin-api && npm install
cd ../admin-web && npm install
```

4. Start admin API:

```bash
cd admin-api
ADMIN_CONTROLLER_ADDR='127.0.0.1:8443' \
ADMIN_CONTROLLER_CA_CERT_FILE='../deploy/tls/controller.crt' \
ADMIN_CONTROLLER_TLS_SERVER_NAME='localhost' \
ADMIN_TOKEN='dev-admin-token' \
ADMIN_PUBLIC_BASE_URL='http://localhost:8787' \
ADMIN_CORS_ORIGIN='http://localhost:5173' \
npm run dev
```

5. Start admin web:

```bash
cd admin-web
npm run dev
```

Open `http://localhost:5173`.

## Binary Runtime For Install Scripts

Install links are now binary-only.

- Connector installer requires `ztna-connector` to be available in `PATH`.
- Agent installer requires `ztna-agent` to be available in `PATH`.
- Scripts fail fast if the binary is missing.

Build binaries once:

```bash
cd /home/igris/Arise/projects/ztna
cd connector && cargo build --release
cd ../agent && cargo build --release
```

Copy binaries to target hosts and install:

```bash
scp /home/igris/Arise/projects/ztna/connector/target/release/ztna-connector user@<connector-host>:/tmp/
scp /home/igris/Arise/projects/ztna/agent/target/release/ztna-agent user@<agent-host>:/tmp/

ssh user@<connector-host> 'sudo install -m 0755 /tmp/ztna-connector /usr/local/bin/ztna-connector'
ssh user@<agent-host> 'sudo install -m 0755 /tmp/ztna-agent /usr/local/bin/ztna-agent'
```

## API Endpoints (admin-api)

- `GET /health`
- `GET /api/workspaces`
- `POST /api/workspaces`
- `PATCH /api/workspaces/:workspaceId`
- `DELETE /api/workspaces/:workspaceId`
- `GET /api/workspaces/:workspaceId/connectors`
- `POST /api/workspaces/:workspaceId/connectors`
- `PATCH /api/workspaces/:workspaceId/connectors/:connectorId`
- `DELETE /api/workspaces/:workspaceId/connectors/:connectorId`
- `GET /api/workspaces/:workspaceId/connectors/:connectorId/agents`
- `POST /api/workspaces/:workspaceId/connectors/:connectorId/agents`
- `PATCH /api/workspaces/:workspaceId/connectors/:connectorId/agents/:agentId`
- `DELETE /api/workspaces/:workspaceId/connectors/:connectorId/agents/:agentId`
- `GET /install/connector/:connectorId.sh`
- `GET /install/agent/:agentId.sh`

## Security Notes

- Install links embed bootstrap tokens; treat links as secrets.
- Bootstrap tokens expire (`ADMIN_TOKEN_TTL_HOURS`, default 24h).
- State is persisted in `admin-api/data/state.json`.
- Install profiles pin deterministic `--device-id` values so UI status maps to the correct enrolled device.
- Install scripts use the controller trust anchor from `ADMIN_CONTROLLER_CA_CERT_FILE` for TLS verification.
- `active` status is heartbeat-based (default recency window `ADMIN_DEVICE_ACTIVE_WINDOW_SECS=45`).
- If your controller address is an IP, set `ADMIN_CONTROLLER_TLS_SERVER_NAME` to a DNS name present in the controller certificate SAN.

## LAN Notes

For remote machines:
- Set connector public address to reachable LAN `host:port`, e.g. `192.168.1.60:9443`.
- Open `9443/tcp` on connector hosts.
- Keep `--connector-server-name` aligned with connector cert SAN (`connector.<workspace_id>`).
- Ensure `ztna-connector` / `ztna-agent` binaries are installed before running install links.
