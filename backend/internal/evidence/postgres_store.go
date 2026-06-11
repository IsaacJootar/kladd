package evidence

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

func (store PostgresStore) Create(ctx context.Context, record CreateRecord) (EvidenceItem, error) {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return EvidenceItem{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	item, err := insertEvidenceItem(ctx, tx, record)
	if err != nil {
		return EvidenceItem{}, err
	}

	if err := insertEvidenceCreatedAudit(ctx, tx, record.UserID, item); err != nil {
		return EvidenceItem{}, err
	}

	if err := tx.Commit(); err != nil {
		return EvidenceItem{}, err
	}

	return item, nil
}

func (store PostgresStore) List(ctx context.Context, userID uuid.UUID) ([]EvidenceItem, error) {
	rows, err := store.db.QueryContext(ctx, `
SELECT id, category, status, metadata_json, uploaded_at
FROM evidence_items
WHERE user_id = $1
ORDER BY uploaded_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []EvidenceItem{}
	for rows.Next() {
		item, err := scanEvidenceItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (store PostgresStore) RequestReview(ctx context.Context, userID uuid.UUID, evidenceID uuid.UUID) (EvidenceItem, error) {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return EvidenceItem{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	item, err := updateEvidenceStatus(ctx, tx, userID, evidenceID, StatusPendingVerification)
	if err != nil {
		return EvidenceItem{}, err
	}

	if err := insertEvidenceReviewRequestedAudit(ctx, tx, userID, item, time.Now().UTC()); err != nil {
		return EvidenceItem{}, err
	}

	if err := tx.Commit(); err != nil {
		return EvidenceItem{}, err
	}

	return item, nil
}

func insertEvidenceItem(ctx context.Context, tx *sql.Tx, record CreateRecord) (EvidenceItem, error) {
	metadata, err := json.Marshal(record.Metadata)
	if err != nil {
		return EvidenceItem{}, err
	}

	row := tx.QueryRowContext(ctx, `
INSERT INTO evidence_items (
    id,
    user_id,
    category,
    file_path,
    status,
    metadata_json
) VALUES ($1, $2, $3, $4, $5, $6::jsonb)
RETURNING id, category, status, metadata_json, uploaded_at`,
		record.ID,
		record.UserID,
		record.Category,
		record.FilePath,
		record.Status,
		string(metadata),
	)

	return scanEvidenceItem(row)
}

func updateEvidenceStatus(ctx context.Context, tx *sql.Tx, userID uuid.UUID, evidenceID uuid.UUID, status string) (EvidenceItem, error) {
	item, err := scanEvidenceItem(tx.QueryRowContext(ctx, `
UPDATE evidence_items
SET status = $3
WHERE user_id = $1
    AND id = $2
    AND status = $4
RETURNING id, category, status, metadata_json, uploaded_at`,
		userID,
		evidenceID,
		status,
		StatusUploaded,
	))
	if err != nil {
		if err == sql.ErrNoRows {
			var exists bool
			checkErr := tx.QueryRowContext(ctx, `
SELECT EXISTS (
    SELECT 1 FROM evidence_items WHERE user_id = $1 AND id = $2
)`,
				userID,
				evidenceID,
			).Scan(&exists)
			if checkErr != nil {
				return EvidenceItem{}, checkErr
			}
			if !exists {
				return EvidenceItem{}, ErrEvidenceNotFound
			}

			return EvidenceItem{}, ErrEvidenceNotReviewable
		}
		return EvidenceItem{}, err
	}

	return item, nil
}

func insertEvidenceCreatedAudit(ctx context.Context, tx *sql.Tx, userID uuid.UUID, item EvidenceItem) error {
	metadata, err := json.Marshal(map[string]string{
		"evidence_id": item.ID.String(),
		"category":    item.Category,
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
		"evidence.created",
		string(metadata),
	)
	if err != nil {
		return fmt.Errorf("insert evidence created audit: %w", err)
	}

	return nil
}

func insertEvidenceReviewRequestedAudit(ctx context.Context, tx *sql.Tx, userID uuid.UUID, item EvidenceItem, requestedAt time.Time) error {
	metadata, err := json.Marshal(map[string]string{
		"evidence_id":  item.ID.String(),
		"category":     item.Category,
		"requested_at": requestedAt.Format(time.RFC3339),
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
		"evidence.review_requested",
		string(metadata),
	)
	if err != nil {
		return fmt.Errorf("insert evidence review requested audit: %w", err)
	}

	return nil
}

type evidenceScanner interface {
	Scan(dest ...any) error
}

func scanEvidenceItem(scanner evidenceScanner) (EvidenceItem, error) {
	var item EvidenceItem
	var metadataBytes []byte
	err := scanner.Scan(
		&item.ID,
		&item.Category,
		&item.Status,
		&metadataBytes,
		&item.UploadedAt,
	)
	if err != nil {
		return EvidenceItem{}, err
	}

	var metadata Metadata
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return EvidenceItem{}, err
	}

	item.DisplayName = metadata.DisplayName
	item.FileName = metadata.OriginalFileName
	item.ContentType = metadata.ContentType
	item.SizeBytes = metadata.SizeBytes

	return item, nil
}
