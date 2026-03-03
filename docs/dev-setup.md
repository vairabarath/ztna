# Developer Setup

1. Install Go, Rust, Docker, and Buf.
2. Generate protobuf stubs.
3. Start Postgres, then run controller and clients (connector/agent).
4. Optional: use the web admin console documented in `docs/admin-dashboard.md`.

## Quickstart (Single Machine)

```bash
# Database (Docker)
docker compose up -d postgres

# Dev TLS cert for controller (localhost SANs)
make tls-dev-cert

# Protobuf (Buf preferred)
buf generate

# Protobuf (fallback using protoc)
protoc -I proto -I /usr/include \
  --go_out=paths=source_relative:proto/gen/go \
  --go-grpc_out=paths=source_relative:proto/gen/go \
  proto/ztna/controlplane/v1/controlplane.proto

# Controller
cd controller
export CONTROLLER_LISTEN_ADDR=':8443'
export CONTROLLER_DB_DSN='postgres://ztna:ztna@localhost:5432/ztna?sslmode=disable'
export CONTROLLER_ADMIN_TOKEN='dev-admin-token'
export CONTROLLER_TLS_ENABLED=true
export CONTROLLER_TLS_CERT_FILE='../deploy/tls/controller.crt'
export CONTROLLER_TLS_KEY_FILE='../deploy/tls/controller.key'
go run ./cmd/controller

# Connector
cd connector
cargo run -- \
  --workspace-id demo \
  --bootstrap-token demo \
  --controller-addr https://127.0.0.1:8443 \
  --controller-ca-cert ../deploy/tls/controller.crt \
  --dataplane-listen-addr 0.0.0.0:9443

# Agent
cd agent
cargo run -- \
  --workspace-id demo \
  --bootstrap-token demo \
  --controller-addr https://127.0.0.1:8443 \
  --controller-ca-cert ../deploy/tls/controller.crt \
  --connector-addr 127.0.0.1:9443 \
  --connector-server-name connector.demo
```

## LAN Setup (Controller + Many Computers)

Run these on the controller host.

```bash
# Example controller LAN IP
export CONTROLLER_HOST_IP='192.168.1.50'

# Generate cert with SANs for localhost + LAN IP (+ optional DNS name)
make tls-dev-cert \
  TLS_DEV_CN='ztna-controller' \
  TLS_DEV_SAN="DNS:localhost,IP:127.0.0.1,IP:${CONTROLLER_HOST_IP},DNS:ztna-controller.local"

docker compose up -d postgres

cd controller
export CONTROLLER_LISTEN_ADDR='0.0.0.0:8443'
export CONTROLLER_DB_DSN='postgres://ztna:ztna@localhost:5432/ztna?sslmode=disable'
export CONTROLLER_ADMIN_TOKEN='dev-admin-token'
export CONTROLLER_TLS_ENABLED=true
export CONTROLLER_TLS_CERT_FILE='../deploy/tls/controller.crt'
export CONTROLLER_TLS_KEY_FILE='../deploy/tls/controller.key'
go run ./cmd/controller
```

Then on each connector/agent machine:

```bash
# Copy the controller cert to this machine first, e.g. ./controller.crt
cd connector
cargo run -- \
  --workspace-id demo \
  --bootstrap-token demo \
  --controller-addr https://192.168.1.50:8443 \
  --controller-ca-cert ./controller.crt \
  --dataplane-listen-addr 0.0.0.0:9443

cd agent
cargo run -- \
  --workspace-id demo \
  --bootstrap-token demo \
  --controller-addr https://192.168.1.50:8443 \
  --controller-ca-cert ./controller.crt \
  --connector-addr 192.168.1.60:9443 \
  --connector-server-name connector.demo
```

Notes:
- `Renew` RPC requires mTLS when `CONTROLLER_TLS_ENABLED=true`.
- If you use `http://` controller address, connector falls back to non-TLS transport.
- Agent dataplane stream to connector always requires TLS (`https://`).
- Admin RPCs (`CreateWorkspace`, `CreateEnrollToken`, `RevokeDevice`) require metadata header `x-admin-token`.
- Open inbound `8443/tcp` on the controller host for LAN clients.
- Open inbound `9443/tcp` on connector hosts for agent dataplane sessions.
- Postgres is intentionally bound to `127.0.0.1` in `docker-compose.yml` and is not exposed to LAN.

## Proto Compatibility Checks

```bash
# Check current proto against committed baseline (fails on breaking changes)
make proto-compat

# If breaking change is intentional, update the baseline image
make proto-baseline
```

## Manual Full Flow (Workspace -> Enroll -> Renew/Revoke)

Use this flow to test end-to-end before automation. It assumes:
- controller is running with TLS at `https://<controller-ip>:8443`
- `grpcurl` is installed
- you have `deploy/tls/controller.crt` on the machine running `grpcurl`

```bash
export CONTROLLER_ADDR='<controller-ip>:8443'
export CA_CERT='./deploy/tls/controller.crt'
export ADMIN_TOKEN='dev-admin-token'
export CONNECTOR_ADDR='<connector-ip>:9443'
```

1. Create workspace:

```bash
grpcurl -vv \
  -cacert "$CA_CERT" \
  -H "x-admin-token: $ADMIN_TOKEN" \
  -d '{"display_name":"demo"}' \
  "$CONTROLLER_ADDR" \
  ztna.controlplane.v1.WorkspaceService/CreateWorkspace
```

Save `workspace_id` from response as `WS_ID`.

2. Create connector and agent tokens:

```bash
export WS_ID='<workspace_id>'

grpcurl -vv \
  -cacert "$CA_CERT" \
  -H "x-admin-token: $ADMIN_TOKEN" \
  -d "{\"workspace_id\":\"$WS_ID\",\"type\":\"ENROLL_TOKEN_TYPE_CONNECTOR\"}" \
  "$CONTROLLER_ADDR" \
  ztna.controlplane.v1.WorkspaceService/CreateEnrollToken

grpcurl -vv \
  -cacert "$CA_CERT" \
  -H "x-admin-token: $ADMIN_TOKEN" \
  -d "{\"workspace_id\":\"$WS_ID\",\"type\":\"ENROLL_TOKEN_TYPE_AGENT\"}" \
  "$CONTROLLER_ADDR" \
  ztna.controlplane.v1.WorkspaceService/CreateEnrollToken
```

Save returned raw tokens as `CONNECTOR_TOKEN` and `AGENT_TOKEN`.

3. Run connector:

```bash
make connector-run \
  WORKSPACE_ID="$WS_ID" \
  BOOTSTRAP_TOKEN="$CONNECTOR_TOKEN" \
  CONTROLLER_ADDR="https://$CONTROLLER_ADDR" \
  CONTROLLER_CA_CERT="$CA_CERT" \
  CONNECTOR_DATAPLANE_LISTEN_ADDR="0.0.0.0:9443"
```

4. Run agent:

```bash
make agent-run \
  WORKSPACE_ID="$WS_ID" \
  BOOTSTRAP_TOKEN="$AGENT_TOKEN" \
  CONTROLLER_ADDR="https://$CONTROLLER_ADDR" \
  CONTROLLER_CA_CERT="$CA_CERT" \
  AGENT_STORAGE_DIR="/tmp/ztna-agent" \
  AGENT_CONNECTOR_ADDR="$CONNECTOR_ADDR" \
  AGENT_CONNECTOR_SERVER_NAME="connector.$WS_ID"
```

5. Revoke one device and observe client reaction (stream + local stop):

```bash
export DEVICE_ID='<device_id_from_client_logs>'

grpcurl -vv \
  -cacert "$CA_CERT" \
  -H "x-admin-token: $ADMIN_TOKEN" \
  -d "{\"workspace_id\":\"$WS_ID\",\"device_id\":\"$DEVICE_ID\",\"reason\":\"manual test\"}" \
  "$CONTROLLER_ADDR" \
  ztna.controlplane.v1.DeviceService/RevokeDevice
```

Expected:
- revoked client receives revocation event and stops its loop
- renew attempts with old certificate are denied by controller
