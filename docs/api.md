# API

Control-plane API is defined in `proto/ztna/controlplane/v1/controlplane.proto`.

## DeviceService

- `StreamRevocations`: server-streaming feed of revocation events per workspace.
- `Heartbeat`: device liveness/status update.
- `RevokeDevice`: marks an active device certificate as revoked, updates device status, and publishes a revocation event.
- `ListDevices`: returns current device records for a workspace (status, fingerprint, last seen timestamp).

## Admin Auth

Admin RPCs require gRPC metadata header:
- `x-admin-token: <CONTROLLER_ADMIN_TOKEN>`

Current admin RPCs:
- `WorkspaceService/CreateWorkspace`
- `WorkspaceService/CreateEnrollToken`
- `DeviceService/RevokeDevice`
- `DeviceService/ListDevices`
