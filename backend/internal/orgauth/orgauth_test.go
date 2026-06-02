package orgauth

import (
	"context"
	"errors"
	"testing"

	"github.com/IsaacJootar/kladd/backend/internal/claimrequests"
	"github.com/google/uuid"
)

type recordingStore struct {
	keyHash      string
	organization claimrequests.Organization
	err          error
}

func (store *recordingStore) AuthenticateAPIKey(ctx context.Context, keyHash string) (claimrequests.Organization, error) {
	store.keyHash = keyHash
	if store.err != nil {
		return claimrequests.Organization{}, store.err
	}
	return store.organization, nil
}

func TestServiceAuthenticateHashesAPIKey(t *testing.T) {
	orgID := uuid.New()
	store := &recordingStore{
		organization: claimrequests.Organization{
			ID:               orgID,
			Name:             "Acme Bank",
			OrganizationType: "bank",
		},
	}
	service := NewService(store)

	organization, err := service.Authenticate(context.Background(), " kladd_test_key ")
	if err != nil {
		t.Fatalf("authenticate api key: %v", err)
	}

	if organization.ID != orgID {
		t.Fatalf("organization id = %s, want %s", organization.ID, orgID)
	}
	if store.keyHash == "kladd_test_key" {
		t.Fatal("store received raw api key instead of hash")
	}
	if store.keyHash != HashAPIKey("kladd_test_key") {
		t.Fatalf("hash = %q, want expected hash", store.keyHash)
	}
}

func TestServiceAuthenticateRequiresAPIKey(t *testing.T) {
	service := NewService(&recordingStore{})

	_, err := service.Authenticate(context.Background(), " ")
	if !errors.Is(err, ErrMissingAPIKey) {
		t.Fatalf("err = %v, want %v", err, ErrMissingAPIKey)
	}
}
