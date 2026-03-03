package device

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/igris/ztna/controller/internal/storage"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Upsert(ctx context.Context, d Device) error {
	q := storage.Queryer(ctx, r.db)
	_, err := q.ExecContext(ctx, `
		INSERT INTO devices (workspace_id, device_id, cert_fingerprint, status, last_seen_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (workspace_id, device_id)
		DO UPDATE SET
			cert_fingerprint = EXCLUDED.cert_fingerprint,
			status = EXCLUDED.status,
			last_seen_at = EXCLUDED.last_seen_at,
			updated_at = NOW()
	`, d.WorkspaceID, d.DeviceID, d.CertFingerprint, d.Status, d.LastSeenUnixTime)
	if err != nil {
		return fmt.Errorf("upsert device: %w", err)
	}
	return nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, workspaceID, deviceID string) (Device, error) {
	var d Device
	q := storage.Queryer(ctx, r.db)
	err := q.QueryRowContext(ctx, `
		SELECT workspace_id, device_id, cert_fingerprint, status, last_seen_at
		FROM devices
		WHERE workspace_id = $1 AND device_id = $2
	`, workspaceID, deviceID).Scan(
		&d.WorkspaceID,
		&d.DeviceID,
		&d.CertFingerprint,
		&d.Status,
		&d.LastSeenUnixTime,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Device{}, ErrNotFound
		}
		return Device{}, fmt.Errorf("get device by id: %w", err)
	}
	return d, nil
}

func (r *PostgresRepository) ListByWorkspace(ctx context.Context, workspaceID string) ([]Device, error) {
	q := storage.Queryer(ctx, r.db)
	rows, err := q.QueryContext(ctx, `
		SELECT workspace_id, device_id, cert_fingerprint, status, last_seen_at
		FROM devices
		WHERE workspace_id = $1
		ORDER BY updated_at DESC, device_id ASC
	`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list devices by workspace: %w", err)
	}
	defer rows.Close()

	out := make([]Device, 0)
	for rows.Next() {
		var d Device
		if err := rows.Scan(
			&d.WorkspaceID,
			&d.DeviceID,
			&d.CertFingerprint,
			&d.Status,
			&d.LastSeenUnixTime,
		); err != nil {
			return nil, fmt.Errorf("scan device row: %w", err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate device rows: %w", err)
	}
	return out, nil
}

func (r *PostgresRepository) UpdateStatus(ctx context.Context, workspaceID, deviceID, status string) error {
	q := storage.Queryer(ctx, r.db)
	res, err := q.ExecContext(ctx, `
		UPDATE devices
		SET status = $3, updated_at = NOW()
		WHERE workspace_id = $1 AND device_id = $2
	`, workspaceID, deviceID, status)
	if err != nil {
		return fmt.Errorf("update device status: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update device status rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}
