package users

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) PostgresStore {
	return PostgresStore{db: db}
}

func (store PostgresStore) Create(ctx context.Context, record CreateRecord) (User, error) {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	user, err := insertUser(ctx, tx, record)
	if err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrEmailTaken
		}
		return User{}, err
	}

	if err := insertUserCreatedAudit(ctx, tx, user); err != nil {
		return User{}, err
	}

	if err := tx.Commit(); err != nil {
		return User{}, err
	}

	return user, nil
}

func (store PostgresStore) Get(ctx context.Context, id uuid.UUID) (User, error) {
	return store.getBy(ctx, "id = $1", id)
}

func (store PostgresStore) GetByEmail(ctx context.Context, email string) (User, error) {
	return store.getBy(ctx, "email = $1", email)
}

func (store PostgresStore) getBy(ctx context.Context, predicate string, value any) (User, error) {
	var user User
	err := store.db.QueryRowContext(ctx, `
SELECT
    id,
    name,
    email,
    COALESCE(phone, ''),
    account_type,
    verification_status,
    created_at
FROM users
WHERE `+predicate,
		value,
	).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.Phone,
		&user.AccountType,
		&user.VerificationStatus,
		&user.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return User{}, ErrUserNotFound
		}
		return User{}, err
	}

	return user, nil
}

func insertUser(ctx context.Context, tx *sql.Tx, record CreateRecord) (User, error) {
	var user User
	err := tx.QueryRowContext(ctx, `
INSERT INTO users (
    id,
    name,
    email,
    phone,
    password_hash,
    account_type,
    verification_status
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, name, email, COALESCE(phone, ''), account_type, verification_status, created_at`,
		record.ID,
		record.Name,
		record.Email,
		nullString(record.Phone),
		record.PasswordHash,
		record.AccountType,
		record.VerificationStatus,
	).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.Phone,
		&user.AccountType,
		&user.VerificationStatus,
		&user.CreatedAt,
	)
	if err != nil {
		return User{}, err
	}

	return user, nil
}

func insertUserCreatedAudit(ctx context.Context, tx *sql.Tx, user User) error {
	metadata, err := json.Marshal(map[string]string{
		"email":        user.Email,
		"account_type": user.AccountType,
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
		user.ID,
		"user.created",
		string(metadata),
	)
	if err != nil {
		return fmt.Errorf("insert user created audit: %w", err)
	}

	return nil
}

func nullString(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
