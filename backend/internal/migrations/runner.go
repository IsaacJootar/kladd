package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type AppliedMigration struct {
	ID        string
	AppliedAt time.Time
}

type Runner struct {
	db  *sql.DB
	dir string
}

func NewRunner(db *sql.DB, dir string) Runner {
	return Runner{
		db:  db,
		dir: dir,
	}
}

func (runner Runner) Apply(ctx context.Context) ([]AppliedMigration, error) {
	if err := runner.ensureSchemaTable(ctx); err != nil {
		return nil, err
	}

	applied, err := runner.appliedIDs(ctx)
	if err != nil {
		return nil, err
	}

	files, err := filepath.Glob(filepath.Join(runner.dir, "*.sql"))
	if err != nil {
		return nil, err
	}

	sort.Strings(files)

	results := make([]AppliedMigration, 0, len(files))
	for _, file := range files {
		id := filepath.Base(file)
		if applied[id] {
			continue
		}

		result, err := runner.applyFile(ctx, id, file)
		if err != nil {
			return nil, err
		}

		results = append(results, result)
	}

	return results, nil
}

func (runner Runner) ensureSchemaTable(ctx context.Context) error {
	_, err := runner.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
    id TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`)
	return err
}

func (runner Runner) appliedIDs(ctx context.Context) (map[string]bool, error) {
	rows, err := runner.db.QueryContext(ctx, `SELECT id FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		applied[id] = true
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return applied, nil
}

func (runner Runner) applyFile(ctx context.Context, id, path string) (AppliedMigration, error) {
	sqlBytes, err := os.ReadFile(path)
	if err != nil {
		return AppliedMigration{}, err
	}

	tx, err := runner.db.BeginTx(ctx, nil)
	if err != nil {
		return AppliedMigration{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(ctx, string(sqlBytes)); err != nil {
		return AppliedMigration{}, fmt.Errorf("apply migration %s: %w", id, err)
	}

	var appliedAt time.Time
	if err := tx.QueryRowContext(ctx, `
INSERT INTO schema_migrations (id)
VALUES ($1)
RETURNING applied_at`, id).Scan(&appliedAt); err != nil {
		return AppliedMigration{}, fmt.Errorf("record migration %s: %w", id, err)
	}

	if err := tx.Commit(); err != nil {
		return AppliedMigration{}, err
	}

	return AppliedMigration{
		ID:        id,
		AppliedAt: appliedAt,
	}, nil
}
