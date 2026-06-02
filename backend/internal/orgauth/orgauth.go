package orgauth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/IsaacJootar/kladd/backend/internal/claimrequests"
)

var (
	ErrMissingAPIKey = errors.New("organization api key is required")
	ErrInvalidAPIKey = errors.New("organization api key is invalid")
)

type Store interface {
	AuthenticateAPIKey(ctx context.Context, keyHash string) (claimrequests.Organization, error)
}

type Service struct {
	store Store
}

func NewService(store Store) Service {
	return Service{store: store}
}

func (service Service) Authenticate(ctx context.Context, apiKey string) (claimrequests.Organization, error) {
	key := strings.TrimSpace(apiKey)
	if key == "" {
		return claimrequests.Organization{}, ErrMissingAPIKey
	}

	return service.store.AuthenticateAPIKey(ctx, HashAPIKey(key))
}

func HashAPIKey(apiKey string) string {
	sum := sha256.Sum256([]byte("kladd-organization-api-key:" + apiKey))
	return hex.EncodeToString(sum[:])
}
