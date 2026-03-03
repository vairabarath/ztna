# TODO: ZTNA gRPC Implementation Plan

This plan turns the idea into a clean, feature-oriented codebase with gRPC as the control-plane protocol.

## 1. Engineering Rules (keep code understandable)

- [ ] API-first: protobuf is the single source of truth.
- [ ] Feature folders over generic "utils" dumping ground.
- [ ] One responsibility per file; avoid "god files".
- [ ] Keep business logic out of transport/server bootstrap code.
- [ ] Every feature ships with unit tests; security-critical paths also get integration tests.

## 2. Target Repository Structure

```text
.
├── proto/
│   └── ztna/controlplane/v1/
│       ├── controlplane.proto
│       └── buf.yaml / buf.gen.yaml
├── controller/                      # Go control plane
│   ├── cmd/controller/main.go
│   ├── internal/
│   │   ├── config/
│   │   │   └── config.go
│   │   ├── grpcserver/
│   │   │   ├── server.go
│   │   │   └── interceptors.go
│   │   ├── workspace/
│   │   │   ├── service.go
│   │   │   ├── repository.go
│   │   │   └── handler.go
│   │   ├── token/
│   │   │   ├── service.go
│   │   │   ├── repository.go
│   │   │   └── hash.go
│   │   ├── enrollment/
│   │   │   ├── service.go
│   │   │   ├── csr_validate.go
│   │   │   ├── signer.go
│   │   │   └── handler.go
│   │   ├── device/
│   │   │   ├── service.go
│   │   │   └── repository.go
│   │   ├── revocation/
│   │   │   ├── service.go
│   │   │   └── cache.go
│   │   ├── pki/
│   │   │   ├── ca_factory.go
│   │   │   └── cert_profile.go
│   │   └── storage/
│   │       ├── db.go
│   │       └── migrations/
│   └── go.mod
├── connector/                       # Rust connector
│   ├── src/
│   │   ├── main.rs
│   │   ├── cli.rs
│   │   ├── config.rs
│   │   ├── bootstrap/
│   │   │   ├── mod.rs
│   │   │   └── ca_fetch.rs
│   │   ├── identity/
│   │   │   ├── mod.rs
│   │   │   ├── device_id.rs
│   │   │   ├── keypair.rs
│   │   │   └── keystore.rs
│   │   ├── enrollment/
│   │   │   ├── mod.rs
│   │   │   ├── csr.rs
│   │   │   └── client.rs
│   │   ├── mtls/
│   │   │   ├── mod.rs
│   │   │   ├── tls_config.rs
│   │   │   └── peer_validate.rs
│   │   ├── renewal/
│   │   │   ├── mod.rs
│   │   │   └── scheduler.rs
│   │   └── storage/
│   │       ├── mod.rs
│   │       └── file_store.rs
│   └── Cargo.toml
├── agent/                           # Rust agent (same pattern as connector)
└── docs/
    ├── api.md
    ├── security.md
    └── dev-setup.md
```

## 3. gRPC API Contract (Proto First)

- [x] Define services in `controlplane.proto`:
  - `WorkspaceService`: `CreateWorkspace`, `CreateEnrollToken`, `GetWorkspaceCA`
  - `EnrollmentService`: `Enroll`, `Renew`
  - `DeviceService`: `StreamRevocations` (server-streaming), optional `Heartbeat`
- [x] Define canonical message fields:
  - `workspace_id`, `device_id`, `bootstrap_token`, `csr_pem`, `ca_cert_pem`, `certificate_pem`, `expires_at`
- [x] Standardize errors with gRPC status codes:
  - `InvalidArgument`, `NotFound`, `Unauthenticated`, `PermissionDenied`, `FailedPrecondition`
- [x] Add protobuf comments for every RPC and security-sensitive field.
- [x] Add code generation workflow (Go + Rust) via `buf`.

## 4. Controller (Go) Implementation Phases

### Phase A: Workspace CA Management
- [x] Implement `CreateWorkspace`:
  - Generate ECDSA P-256 keypair.
  - Generate self-signed workspace CA cert (10 years).
  - Store CA private key encrypted at rest.
- [x] Implement `GetWorkspaceCA`.
- [x] Add DB schema for `workspaces`.
- [ ] Ensure CA is immutable after creation (never re-generated).

### Phase B: Workspace-Scoped Token System
- [x] Implement `CreateEnrollToken`:
  - Random 32-byte token.
  - Hash token before DB store.
  - Store type (`agent`/`connector`), expiry, `used=false`.
- [x] Add DB schema for `enroll_tokens`.
- [x] Implement constant-time token hash comparison.
- [x] Enforce single-use token semantics.

### Phase C: Enrollment
- [x] Implement `Enroll` RPC.
- [x] Validate workspace exists.
- [x] Validate token: workspace match, type, not expired, not used.
- [x] Validate CSR:
  - `O == workspace_id`
  - SAN contains workspace and device identity
  - Reject malformed/missing SAN
- [x] Sign cert with workspace CA (initial validity: 24h).
- [x] Persist device record + cert fingerprint in `devices`.
- [x] Mark token as used in same transaction as enrollment success.

### Phase D: mTLS AuthN/AuthZ in Controller
- [x] TLS 1.3 only.
- [x] Require client cert for authenticated RPCs.
- [x] Resolve workspace trust root from cert identity; do not trust global CA pool.
- [x] Validate cert workspace/device against DB status and revocation cache.

### Phase E: Renewal + Revocation
- [x] Implement `Renew` RPC over existing mTLS identity (no bootstrap token).
- [x] Rotate fingerprint on renew.
- [x] Implement revocation model and in-memory cache.
- [x] Implement `StreamRevocations` for connector/agent updates.

## 5. Connector (Rust) Implementation Phases

### Phase A: Bootstrap + Identity
- [x] Parse CLI/config: `workspace_id`, `bootstrap_token`, `controller_addr`.
- [x] Fetch workspace CA via gRPC.
- [x] Generate and persist on first run:
  - `device_id` (UUID v4, never regenerated)
  - ECDSA P-256 keypair

### Phase B: Enrollment Client
- [x] Build CSR with:
  - `CN = device_id`
  - `O = workspace_id`
  - SAN URI and DNS fields containing workspace/device
- [x] Call `Enroll`.
- [x] Persist keypair + issued cert + workspace CA.

### Phase C: Runtime mTLS
- [x] Configure TLS 1.3 mTLS for controller and peer communication.
- [x] Trust only workspace CA.
- [x] Enforce peer workspace match from SAN; reject cross-workspace.
- [x] Check revocation cache before allowing peer session.
- [x] Expose connector data-plane gRPC streaming endpoint for agent sessions.

### Phase D: Renewal Loop
- [x] Schedule renew at 80% cert lifetime.
- [x] Call `Renew` via mTLS.
- [x] Atomically replace local cert and fingerprint state.

## 6. Agent (Rust) Implementation Phases

- [x] Reuse connector enrollment and mTLS feature pattern.
- [x] Keep agent code layout parallel to connector for maintainability.
- [x] Implement same workspace/SAN enforcement rules.
- [x] Dial persistent mTLS gRPC stream to connector using issued workspace certificate.

## 7. Data Model + Migrations

- [x] `workspaces`:
  - `id`, `ca_cert_pem`, `ca_private_key_encrypted`, `created_at`
- [x] `enroll_tokens`:
  - `id`, `workspace_id`, `hashed_token`, `type`, `expires_at`, `used`, `created_at`
- [x] `devices`:
  - `device_id`, `workspace_id`, `cert_fingerprint`, `status`, `last_seen_at`, `created_at`
- [x] `revocations`:
  - `id`, `workspace_id`, `device_id`, `fingerprint`, `reason`, `revoked_at`
- [x] Add indexes for workspace lookups, expiry filtering, and device status checks.

## 8. Security Checklist (Must Pass)

- [ ] Reject CSR workspace mismatch.
- [ ] Never trust CN alone for authorization.
- [ ] Validate SAN workspace + device claims.
- [ ] Reject expired/revoked certs.
- [ ] Token compare uses constant-time path.
- [ ] CA private keys encrypted at rest.
- [ ] Audit logs for enrollment, renew, revoke, and auth failures.

## 9. Testing Strategy

- [x] Unit tests (Go): CSR validator, token validator, cert signer, SAN parser.
- [x] Unit tests (Rust): identity persistence, CSR builder, SAN extraction, renew scheduler.
- [x] Integration tests:
  - workspace creation -> token -> enrollment -> mTLS auth
  - token replay rejection
  - cross-workspace cert rejection
  - revoked cert rejection
  - renewal success + old fingerprint invalidation
- [x] Proto compatibility tests (breaking-change checks via `buf`).

## 10. Milestones

- [ ] M1: Proto + codegen + base project layout
- [ ] M2: Workspace + token flows
- [ ] M3: Enrollment end-to-end (controller + connector)
- [ ] M4: mTLS enforcement + revocation
- [ ] M5: Renewal + hardening + full test suite
- [ ] M6: Documentation and operational runbooks
