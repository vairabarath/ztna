package token

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/igris/ztna/controller/internal/storage"
	"github.com/jackc/pgx/v5/pgconn"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Insert(ctx context.Context, in EnrollmentToken) error {
	q := storage.Queryer(ctx, r.db)
	_, err := q.ExecContext(ctx, `
		INSERT INTO enroll_tokens (id, workspace_id, token_hash, type, expires_at, used)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, in.ID, in.WorkspaceID, in.TokenHash, in.Type, in.ExpiresAt.UTC(), in.Used)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrTokenAlreadyExists
		}
		return fmt.Errorf("insert enroll token: %w", err)
	}
	return nil
}

func (r *PostgresRepository) GetByWorkspaceAndHash(ctx context.Context, workspaceID string, tokenHash []byte) (EnrollmentToken, error) {
	var rec EnrollmentToken
	q := storage.Queryer(ctx, r.db)
	err := q.QueryRowContext(ctx, `
		SELECT id, workspace_id, type, token_hash, expires_at, used
		FROM enroll_tokens
		WHERE workspace_id = $1 AND token_hash = $2
		LIMIT 1
	`, workspaceID, tokenHash).Scan(
		&rec.ID,
		&rec.WorkspaceID,
		&rec.Type,
		&rec.TokenHash,
		&rec.ExpiresAt,
		&rec.Used,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return EnrollmentToken{}, ErrTokenNotFound
		}
		return EnrollmentToken{}, fmt.Errorf("get enroll token by hash: %w", err)
	}
	return rec, nil
}

func (r *PostgresRepository) MarkUsed(ctx context.Context, tokenID string, usedAt time.Time) error {
	q := storage.Queryer(ctx, r.db)
	res, err := q.ExecContext(ctx, `
		UPDATE enroll_tokens
		SET used = TRUE, used_at = $2
		WHERE id = $1 AND used = FALSE
	`, tokenID, usedAt.UTC())
	if err != nil {
		return fmt.Errorf("mark token used: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mark token used rows affected: %w", err)
	}
	if rows == 1 {
		return nil
	}

	var used bool
	err = q.QueryRowContext(ctx, `SELECT used FROM enroll_tokens WHERE id = $1`, tokenID).Scan(&used)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrTokenNotFound
	}
	if err != nil {
		return fmt.Errorf("lookup token usage status: %w", err)
	}
	if used {
		return ErrTokenUsed
	}
	return ErrTokenNotFound
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
