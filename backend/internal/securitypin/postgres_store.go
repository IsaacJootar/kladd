package securitypin

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) PostgresStore {
	return PostgresStore{db: db}
}

func (store PostgresStore) SetPIN(ctx context.Context, userID uuid.UUID, pinHash string, setAt time.Time) (SetupResult, error) {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return SetupResult{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	result, err := updateUserPIN(ctx, tx, userID, pinHash, setAt)
	if err != nil {
		return SetupResult{}, err
	}

	if err := insertSecurityPINSetupAudit(ctx, tx, userID); err != nil {
		return SetupResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return SetupResult{}, err
	}

	return result, nil
}

func updateUserPIN(ctx context.Context, tx *sql.Tx, userID uuid.UUID, pinHash string, setAt time.Time) (SetupResult, error) {
	var result SetupResult
	err := tx.QueryRowContext(ctx, `
UPDATE users
SET
    security_pin_hash = $2,
    security_pin_set_at = $3,
    pin_failed_attempts = 0,
    pin_locked_until = NULL
WHERE id = $1
RETURNING id, security_pin_set_at`,
		userID,
		pinHash,
		setAt,
	).Scan(&result.UserID, &result.SetAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return SetupResult{}, ErrUserNotFound
		}
		return SetupResult{}, err
	}

	result.Set = true
	return result, nil
}

func insertSecurityPINSetupAudit(ctx context.Context, tx *sql.Tx, userID uuid.UUID) error {
	metadata, err := json.Marshal(map[string]string{
		"method": "security_pin",
	})
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
INSERT INTO audit_logs (
    id,
    actor_type,
    actor_id,
    event_type,
    metadata_json
) VALUES ($1, $2, $3, $4, $5::jsonb)`,
		uuid.New(),
		"user",
		userID,
		"security_pin.set",
		string(metadata),
	)
	if err != nil {
		return fmt.Errorf("insert security pin setup audit: %w", err)
	}

	return nil
}
