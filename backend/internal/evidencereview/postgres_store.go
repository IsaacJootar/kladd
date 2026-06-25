package evidencereview

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

func (store PostgresStore) Review(ctx context.Context, input ReviewInput, reviewedAt time.Time) (ReviewResult, error) {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return ReviewResult{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	result, err := updateEvidenceReview(ctx, tx, input, reviewedAt)
	if err != nil {
		return ReviewResult{}, err
	}

	if err := insertEvidenceReviewAudit(ctx, tx, result); err != nil {
		return ReviewResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return ReviewResult{}, err
	}

	return result, nil
}

func updateEvidenceReview(ctx context.Context, tx *sql.Tx, input ReviewInput, reviewedAt time.Time) (ReviewResult, error) {
	var result ReviewResult
	var metadataBytes []byte
	err := tx.QueryRowContext(ctx, `
UPDATE evidence_items
SET status = $3
FROM users
WHERE evidence_items.user_id = users.id
    AND users.email = $1
    AND evidence_items.id = $2
RETURNING
    evidence_items.id,
    evidence_items.user_id,
    users.email,
    evidence_items.category,
    evidence_items.metadata_json,
    evidence_items.status`,
		input.UserEmail,
		input.EvidenceID,
		input.Status,
	).Scan(
		&result.ID,
		&result.UserID,
		&result.UserEmail,
		&result.Category,
		&metadataBytes,
		&result.Status,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return ReviewResult{}, ErrEvidenceNotFound
		}
		return ReviewResult{}, err
	}

	var metadata struct {
		DisplayName string `json:"display_name"`
	}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return ReviewResult{}, err
	}

	result.DisplayName = metadata.DisplayName
	result.ReviewedAt = reviewedAt
	return result, nil
}

func insertEvidenceReviewAudit(ctx context.Context, tx *sql.Tx, result ReviewResult) error {
	eventType := "evidence.rejected"
	if result.Status == StatusVerified {
		eventType = "evidence.verified"
	}

	metadata, err := json.Marshal(map[string]string{
		"evidence_id": result.ID.String(),
		"user_id":     result.UserID.String(),
		"category":    result.Category,
		"status":      result.Status,
		"reviewed_at": result.ReviewedAt.Format(time.RFC3339),
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
		"admin",
		nil,
		eventType,
		string(metadata),
	)
	if err != nil {
		return fmt.Errorf("insert evidence review audit: %w", err)
	}

	return nil
}
