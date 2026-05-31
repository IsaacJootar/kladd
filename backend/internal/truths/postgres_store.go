package truths

import (
	"context"
	"database/sql"
	"encoding/json"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) PostgresStore {
	return PostgresStore{db: db}
}

func (store PostgresStore) ListDefinitions(ctx context.Context) ([]Definition, error) {
	rows, err := store.db.QueryContext(ctx, `
SELECT
    id,
    truth_key,
    category,
    return_type,
    sensitivity,
    validity_days,
    derivation_rule,
    required_evidence_json,
    created_at
FROM truth_definitions
ORDER BY category, truth_key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	definitions := []Definition{}
	for rows.Next() {
		var definition Definition
		var requiredEvidenceBytes []byte
		err := rows.Scan(
			&definition.ID,
			&definition.TruthKey,
			&definition.Category,
			&definition.ReturnType,
			&definition.Sensitivity,
			&definition.ValidityDays,
			&definition.DerivationRule,
			&requiredEvidenceBytes,
			&definition.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(requiredEvidenceBytes, &definition.RequiredEvidence); err != nil {
			return nil, err
		}

		definitions = append(definitions, definition)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return definitions, nil
}
