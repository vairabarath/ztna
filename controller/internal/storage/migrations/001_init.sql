CREATE TABLE IF NOT EXISTS workspaces (
    id TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    ca_cert_pem TEXT NOT NULL,
    ca_private_key_encrypted BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS enroll_tokens (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('connector', 'agent')),
    expires_at TIMESTAMPTZ NOT NULL,
    used BOOLEAN NOT NULL DEFAULT FALSE,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (workspace_id, token_hash)
);

CREATE INDEX IF NOT EXISTS idx_enroll_tokens_workspace_used_expires
    ON enroll_tokens (workspace_id, used, expires_at);

CREATE TABLE IF NOT EXISTS devices (
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    device_id TEXT NOT NULL,
    cert_fingerprint TEXT NOT NULL,
    status TEXT NOT NULL,
    last_seen_at BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (workspace_id, device_id)
);

CREATE INDEX IF NOT EXISTS idx_devices_workspace_status
    ON devices (workspace_id, status);
