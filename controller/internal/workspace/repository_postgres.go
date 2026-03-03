package workspace

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/igris/ztna/controller/internal/storage"
	"github.com/jackc/pgx/v5/pgconn"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Insert(ctx context.Context, ws Workspace) error {
	q := storage.Queryer(ctx, r.db)
	_, err := q.ExecContext(ctx, `
		INSERT INTO workspaces (id, display_name, ca_cert_pem, ca_private_key_encrypted)
		VALUES ($1, $2, $3, $4)
	`, ws.ID, ws.DisplayName, ws.CACertPEM, ws.CAPrivateKeyEncrypted)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrAlreadyExist
		}
		return fmt.Errorf("insert workspace: %w", err)
	}
	return nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id string) (Workspace, error) {
	var ws Workspace
	q := storage.Queryer(ctx, r.db)
	err := q.QueryRowContext(ctx, `
		SELECT id, display_name, ca_cert_pem, ca_private_key_encrypted
		FROM workspaces
		WHERE id = $1
	`, id).Scan(&ws.ID, &ws.DisplayName, &ws.CACertPEM, &ws.CAPrivateKeyEncrypted)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Workspace{}, ErrNotFound
		}
		return Workspace{}, fmt.Errorf("get workspace by id: %w", err)
	}
	return ws, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
