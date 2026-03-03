We’ll define this in exact steps.

PHASE 1 — WORKSPACE CREATION (Controller – Go)

When admin creates workspace:

Controller does:

Generate ECDSA P256 keypair

Generate self-signed root certificate

Store:

workspace_id

ca_private_key

ca_certificate

Persist in DB

TODO (Controller)

 POST /workspace

 Generate CA key

 Generate self-signed CA cert (10 years)

 Store securely

 Return workspace_id

Do NOT regenerate CA later.

PHASE 2 — BOOTSTRAP TOKEN (Scoped to Workspace)

Token must contain:

workspace_id

type (agent / connector)

expiry

single-use

random 32 bytes

DB:

id
workspace_id
hashed_token
type
expires_at
used
TODO

 POST /workspace/{id}/enroll-token

 Hash token before storing

 Mark used after enrollment

PHASE 3 — CONNECTOR / AGENT ENROLLMENT (Rust → Go)
Step 1 — Connector Starts

CLI args:

--workspace-id=abc
--bootstrap-token=xyz
--controller=https://controller
Step 2 — Fetch Workspace CA

GET:

/workspace/{id}/ca

Response:

{
  ca_cert_pem: "...",
  workspace_id: "..."
}

Connector stores CA cert.

Step 3 — Generate Local Identity (Rust)

On first run:

Generate ECDSA P256 keypair

Generate device_id (UUID v4)

Store both persistently

Never regenerate device_id.

Step 4 — Create CSR (Rust)

CSR must contain:

CN = device_id

O = workspace_id

SAN:

URI: ztna://workspace_id/device_id

DNS: device_id.workspace_id

Put workspace_id inside cert.
That prevents cross-workspace abuse.

Step 5 — POST Enrollment Request
POST /workspace/{id}/enroll

Body:

{
  bootstrap_token,
  csr_pem,
  device_id,
  hostname,
  metadata
}
Step 6 — Controller Validates (Go)

Controller must:

Validate workspace exists

Validate token belongs to that workspace

Check token not expired

Check token not used

Verify CSR structure

Ensure CSR O matches workspace_id

Sign certificate with that workspace CA

Cert validity:

24 hours (PoC)

Include:

workspace_id in SAN

device_id in SAN

Mark token used.

Store device record:

device_id
workspace_id
cert_fingerprint
status=active
Step 7 — Return Signed Cert

Response:

{
  certificate_pem,
  ca_cert_pem,
  expires_at
}

Rust stores:

private key

signed cert

workspace CA cert

Enrollment complete.

🔐 PHASE 4 — mTLS CONNECTOR ↔ CONTROLLER (Workspace Scoped)

Connector connects using:

client cert

private key

workspace CA

Controller TLS config:

RequireAndVerifyClientCert

Dynamically select correct workspace CA

Extract workspace_id from cert

Validate against DB

Check revoked

Allow connection

Important:

Controller must NOT trust any CA globally.
It must verify certificate against correct workspace CA.

🔐 PHASE 5 — AGENT ↔ CONNECTOR mTLS

Connector TLS config:

Trust only workspace CA

Extract workspace_id from cert

Compare with its own workspace_id

Reject if mismatch

Check revocation list

This prevents:

Agent from Workspace A connecting to Connector B.

🔁 PHASE 6 — CERT RENEWAL

Renew endpoint:

POST /workspace/{id}/renew

Authenticated using existing mTLS.

Controller:

Verify client cert valid

Verify not revoked

Sign new cert

Replace fingerprint

No bootstrap token required.

Renew at 80% lifetime.

🚨 CRITICAL SECURITY RULES

Never allow CSR workspace mismatch

Never trust CN alone

Validate SAN workspace

Enforce TLS 1.3 only

Reject expired certs

Store CA private key encrypted at rest

Use constant-time token comparison

🔥 WHY WORKSPACE CA IS SMART

Because now:

Even if one workspace CA is compromised,
Others remain safe.

You achieved:

Cryptographic tenant isolation.

Most people forget this.
You didn’t.

Good.

🧱 WHAT YOU GIVE TO CODEX (TODO.md Structure)
1. Workspace CA Management (Go)

Generate CA

Store securely

Expose CA public endpoint

2. Workspace-Scoped Token System

Create token

Hash

Expiry

Single-use

3. CSR Enrollment Endpoint

Validate workspace

Validate token

Validate CSR

Sign cert

Persist device

4. mTLS Server Config

Dynamic workspace CA verification

Extract SAN

Validate device status

5. Rust Enrollment Client

Generate keypair

Generate CSR

Fetch CA

Send enroll request

Store cert

6. Renewal Flow

mTLS-authenticated CSR

Replace cert

7. Revocation Model

Push events

In-memory revoked list
