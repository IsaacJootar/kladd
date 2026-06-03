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
	record       IssueRecord
	organization claimrequests.Organization
	issued       IssuedAPIKey
	err          error
}

func (store *recordingStore) AuthenticateAPIKey(ctx context.Context, keyHash string) (claimrequests.Organization, error) {
	store.keyHash = keyHash
	if store.err != nil {
		return claimrequests.Organization{}, store.err
	}
	return store.organization, nil
}

func (store *recordingStore) IssueAPIKey(ctx context.Context, record IssueRecord) (IssuedAPIKey, error) {
	store.record = record
	if store.err != nil {
		return IssuedAPIKey{}, store.err
	}
	if store.issued.KeyID != uuid.Nil {
		return store.issued, nil
	}
	return IssuedAPIKey{
		Organization: claimrequests.Organization{
			ID:               record.OrganizationID,
			Name:             record.OrganizationName,
			OrganizationType: record.OrganizationType,
		},
		KeyID:     record.KeyID,
		KeyName:   record.KeyName,
		KeyPrefix: record.KeyPrefix,
		CreatedAt: record.CreatedAt,
	}, nil
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

func TestServiceIssueAPIKeyStoresHashAndReturnsRawKeyOnce(t *testing.T) {
	store := &recordingStore{}
	service := NewService(store)

	issued, err := service.IssueAPIKey(context.Background(), IssueInput{
		OrganizationName: " Acme Bank ",
		OrganizationType: "bank",
		KeyName:          " Local setup ",
	})
	if err != nil {
		t.Fatalf("issue api key: %v", err)
	}

	if issued.APIKey == "" {
		t.Fatal("issued api key should include raw key for one-time setup")
	}
	if store.record.OrganizationName != "Acme Bank" {
		t.Fatalf("organization name = %q, want Acme Bank", store.record.OrganizationName)
	}
	if store.record.OrganizationType != "bank" {
		t.Fatalf("organization type = %q, want bank", store.record.OrganizationType)
	}
	if store.record.KeyName != "Local setup" {
		t.Fatalf("key name = %q, want Local setup", store.record.KeyName)
	}
	if store.record.KeyHash == issued.APIKey {
		t.Fatal("store received raw api key instead of hash")
	}
	if store.record.KeyHash != HashAPIKey(issued.APIKey) {
		t.Fatal("stored api key hash does not match issued api key")
	}
	if store.record.KeyPrefix == "" || store.record.KeyPrefix == issued.APIKey {
		t.Fatalf("key prefix = %q, want non-secret prefix", store.record.KeyPrefix)
	}
}

func TestServiceIssueAPIKeyValidatesInput(t *testing.T) {
	tests := []struct {
		name  string
		input IssueInput
		err   error
	}{
		{
			name: "missing organization",
			input: IssueInput{
				KeyName: "Local setup",
			},
			err: ErrInvalidOrganization,
		},
		{
			name: "missing key name",
			input: IssueInput{
				OrganizationName: "Acme Bank",
			},
			err: ErrInvalidKeyName,
		},
	}

	service := NewService(&recordingStore{})
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.IssueAPIKey(context.Background(), test.input)
			if !errors.Is(err, test.err) {
				t.Fatalf("err = %v, want %v", err, test.err)
			}
		})
	}
}
