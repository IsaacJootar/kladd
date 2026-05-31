package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/IsaacJootar/kladd/backend/internal/users"
	"github.com/google/uuid"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) PostgresStore {
	return PostgresStore{db: db}
}

func (store PostgresStore) FindCredentialsByEmail(ctx context.Context, email string) (Credentials, error) {
	var credentials Credentials
	err := store.db.QueryRowContext(ctx, `
SELECT
    id,
    name,
    email,
    COALESCE(phone, ''),
    password_hash,
    account_type,
    verification_status,
    created_at
FROM users
WHERE email = $1`,
		email,
	).Scan(
		&credentials.User.ID,
		&credentials.User.Name,
		&credentials.User.Email,
		&credentials.User.Phone,
		&credentials.PasswordHash,
		&credentials.User.AccountType,
		&credentials.User.VerificationStatus,
		&credentials.User.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return Credentials{}, ErrInvalidCredentials
		}
		return Credentials{}, err
	}

	return credentials, nil
}

func (store PostgresStore) RecordLogin(ctx context.Context, user users.User) error {
	metadata, err := json.Marshal(map[string]string{
		"method": "password",
	})
	if err != nil {
		return err
	}

	_, err = store.db.ExecContext(ctx, `
INSERT INTO audit_logs (
    id,
    actor_type,
    actor_id,
    event_type,
    metadata_json
) VALUES ($1, $2, $3, $4, $5::jsonb)`,
		uuid.New(),
		"user",
		user.ID,
		"user.login",
		string(metadata),
	)
	if err != nil {
		return fmt.Errorf("insert user login audit: %w", err)
	}

	return nil
}
