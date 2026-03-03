CREATE TABLE IF NOT EXISTS revocations (
    id BIGSERIAL PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    device_id TEXT NOT NULL,
    fingerprint TEXT NOT NULL,
    reason TEXT NOT NULL,
    revoked_at TIMESTAMPTZ NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_revocations_workspace_fingerprint
    ON revocations (workspace_id, fingerprint);

CREATE INDEX IF NOT EXISTS idx_revocations_workspace_revoked_at
    ON revocations (workspace_id, revoked_at DESC);
