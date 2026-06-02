package audit

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) PostgresStore {
	return PostgresStore{db: db}
}

func (store PostgresStore) ListForUser(ctx context.Context, userID uuid.UUID, limit int) ([]Record, error) {
	rows, err := store.db.QueryContext(ctx, `
SELECT id, event_type, metadata_json, created_at
FROM audit_logs
WHERE actor_type = 'user' AND actor_id = $1
ORDER BY created_at DESC
LIMIT $2`,
		userID,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := []Record{}
	for rows.Next() {
		record, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return records, nil
}

type recordScanner interface {
	Scan(dest ...any) error
}

func scanRecord(scanner recordScanner) (Record, error) {
	var record Record
	var metadataBytes []byte
	err := scanner.Scan(
		&record.ID,
		&record.EventType,
		&metadataBytes,
		&record.CreatedAt,
	)
	if err != nil {
		return Record{}, err
	}

	var metadata map[string]any
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return Record{}, err
	}
	record.Metadata = metadata

	return record, nil
}
