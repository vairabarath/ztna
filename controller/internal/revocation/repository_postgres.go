package revocation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Insert(ctx context.Context, in Entry) error {
	revokedAt := time.UnixMilli(in.RevokedUnixMilli).UTC()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO revocations (workspace_id, device_id, fingerprint, reason, revoked_at)
		VALUES ($1, $2, $3, $4, $5)
	`, in.WorkspaceID, in.DeviceID, in.CertFingerprint, in.Reason, revokedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrAlreadyRevoked
		}
		return fmt.Errorf("insert revocation: %w", err)
	}
	return nil
}

func (r *PostgresRepository) Exists(ctx context.Context, workspaceID, fingerprint string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM revocations
			WHERE workspace_id = $1 AND fingerprint = $2
		)
	`, workspaceID, fingerprint).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check revocation exists: %w", err)
	}
	return exists, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
