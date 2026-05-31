package truths

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Definition struct {
	ID               uuid.UUID `json:"id"`
	TruthKey         string    `json:"truth_key"`
	Category         string    `json:"category"`
	ReturnType       string    `json:"return_type"`
	Sensitivity      string    `json:"sensitivity"`
	ValidityDays     int       `json:"validity_days"`
	DerivationRule   string    `json:"derivation_rule"`
	RequiredEvidence []string  `json:"required_evidence"`
	CreatedAt        time.Time `json:"created_at"`
}

type Store interface {
	ListDefinitions(ctx context.Context) ([]Definition, error)
}

type Service struct {
	store Store
}

func NewService(store Store) Service {
	return Service{store: store}
}

func (service Service) ListDefinitions(ctx context.Context) ([]Definition, error) {
	return service.store.ListDefinitions(ctx)
}
