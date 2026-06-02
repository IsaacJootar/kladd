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

func (store PostgresStore) GetPasswordHash(ctx context.Context, userID uuid.UUID) (string, error) {
	var passwordHash string
	err := store.db.QueryRowContext(ctx, `
SELECT password_hash
FROM users
WHERE id = $1`,
		userID,
	).Scan(&passwordHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", ErrUserNotFound
		}
		return "", err
	}

	return passwordHash, nil
}

func (store PostgresStore) ResetPIN(ctx context.Context, userID uuid.UUID, pinHash string, resetAt time.Time) (SetupResult, error) {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return SetupResult{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	result, err := updateUserPIN(ctx, tx, userID, pinHash, resetAt)
	if err != nil {
		return SetupResult{}, err
	}

	if err := insertSecurityPINResetAudit(ctx, tx, userID); err != nil {
		return SetupResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return SetupResult{}, err
	}

	return result, nil
}

func (store PostgresStore) GetValidationState(ctx context.Context, userID uuid.UUID) (ValidationState, error) {
	var state ValidationState
	var lockedUntil sql.NullTime
	err := store.db.QueryRowContext(ctx, `
SELECT
    COALESCE(security_pin_hash, ''),
    pin_failed_attempts,
    pin_locked_until
FROM users
WHERE id = $1`,
		userID,
	).Scan(
		&state.PINHash,
		&state.FailedAttempts,
		&lockedUntil,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return ValidationState{}, ErrUserNotFound
		}
		return ValidationState{}, err
	}

	if lockedUntil.Valid {
		state.LockedUntil = &lockedUntil.Time
	}

	return state, nil
}

func (store PostgresStore) RecordValidationFailure(ctx context.Context, userID uuid.UUID, decision LockoutDecision) error {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	result, err := tx.ExecContext(ctx, `
UPDATE users
SET
    pin_failed_attempts = $2,
    pin_locked_until = $3
WHERE id = $1`,
		userID,
		decision.FailedAttempts,
		nullTime(decision.LockedUntil),
	)
	if err != nil {
		return err
	}
	if count, err := result.RowsAffected(); err != nil {
		return err
	} else if count == 0 {
		return ErrUserNotFound
	}

	if err := insertSecurityPINValidationAudit(ctx, tx, userID, "security_pin.validation_failed", decision); err != nil {
		return err
	}

	return tx.Commit()
}

func (store PostgresStore) RecordValidationSuccess(ctx context.Context, userID uuid.UUID) error {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	result, err := tx.ExecContext(ctx, `
UPDATE users
SET
    pin_failed_attempts = 0,
    pin_locked_until = NULL
WHERE id = $1`,
		userID,
	)
	if err != nil {
		return err
	}
	if count, err := result.RowsAffected(); err != nil {
		return err
	} else if count == 0 {
		return ErrUserNotFound
	}

	if err := insertSecurityPINValidationAudit(ctx, tx, userID, "security_pin.validation_succeeded", ResetAfterSuccess()); err != nil {
		return err
	}

	return tx.Commit()
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

func insertSecurityPINResetAudit(ctx context.Context, tx *sql.Tx, userID uuid.UUID) error {
	metadata, err := json.Marshal(map[string]string{
		"method": "password_reauthentication",
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
		"security_pin.reset",
		string(metadata),
	)
	if err != nil {
		return fmt.Errorf("insert security pin reset audit: %w", err)
	}

	return nil
}

func insertSecurityPINValidationAudit(ctx context.Context, tx *sql.Tx, userID uuid.UUID, eventType string, decision LockoutDecision) error {
	metadata := map[string]any{
		"method":          "security_pin",
		"failed_attempts": decision.FailedAttempts,
		"locked":          decision.LockedUntil != nil,
	}
	if decision.LockedUntil != nil {
		metadata["locked_until"] = decision.LockedUntil.Format(time.RFC3339)
	}

	metadataJSON, err := json.Marshal(metadata)
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
		eventType,
		string(metadataJSON),
	)
	if err != nil {
		return fmt.Errorf("insert security pin validation audit: %w", err)
	}

	return nil
}

func nullTime(value *time.Time) sql.NullTime {
	if value == nil {
		return sql.NullTime{}
	}

	return sql.NullTime{Time: *value, Valid: true}
}
