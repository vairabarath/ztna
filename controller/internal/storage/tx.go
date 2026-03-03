package storage

import (
	"context"
	"database/sql"
	"fmt"
)

type contextKey string

const txContextKey contextKey = "sql_tx"

type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type TxRunner interface {
	RunInTx(ctx context.Context, fn func(txCtx context.Context) error) error
}

type PostgresTxRunner struct {
	db *sql.DB
}

func NewPostgresTxRunner(db *sql.DB) *PostgresTxRunner {
	return &PostgresTxRunner{db: db}
}

func (r *PostgresTxRunner) RunInTx(ctx context.Context, fn func(txCtx context.Context) error) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	txCtx := WithTx(ctx, tx)
	if err := fn(txCtx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("tx rollback after %v failed: %w", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func WithTx(ctx context.Context, tx *sql.Tx) context.Context {
	return context.WithValue(ctx, txContextKey, tx)
}

func TxFromContext(ctx context.Context) (*sql.Tx, bool) {
	tx, ok := ctx.Value(txContextKey).(*sql.Tx)
	return tx, ok
}

func Queryer(ctx context.Context, db *sql.DB) DBTX {
	if tx, ok := TxFromContext(ctx); ok {
		return tx
	}
	return db
}
