package truths

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type recordingStore struct {
	definitions []Definition
	err         error
}

func (store *recordingStore) ListDefinitions(ctx context.Context) ([]Definition, error) {
	if store.err != nil {
		return nil, store.err
	}
	return store.definitions, nil
}

func TestServiceListDefinitions(t *testing.T) {
	definitionID := uuid.New()
	service := NewService(&recordingStore{
		definitions: []Definition{
			{
				ID:               definitionID,
				TruthKey:         "age_over_18",
				Category:         "age",
				ReturnType:       "boolean",
				Sensitivity:      "low",
				ValidityDays:     365,
				DerivationRule:   "verified_date_of_birth_evidence",
				RequiredEvidence: []string{"passport"},
				CreatedAt:        time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC),
			},
		},
	})

	definitions, err := service.ListDefinitions(context.Background())
	if err != nil {
		t.Fatalf("list definitions: %v", err)
	}

	if len(definitions) != 1 {
		t.Fatalf("definitions length = %d, want 1", len(definitions))
	}

	if definitions[0].ID != definitionID {
		t.Fatalf("definition id = %s, want %s", definitions[0].ID, definitionID)
	}
}

func TestDefinitionJSONDoesNotExposeTruthValues(t *testing.T) {
	payload, err := json.Marshal(Definition{
		ID:               uuid.New(),
		TruthKey:         "identity_verified",
		Category:         "identity",
		ReturnType:       "boolean",
		Sensitivity:      "high",
		ValidityDays:     365,
		DerivationRule:   "verified_government_identity_evidence",
		RequiredEvidence: []string{"passport"},
	})
	if err != nil {
		t.Fatalf("marshal definition: %v", err)
	}

	body := string(payload)
	for _, forbidden := range []string{"truth_value", "raw_document", "bvn", "nin", "passport_number", "tax_id"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("definition response exposed forbidden field %q", forbidden)
		}
	}
}
