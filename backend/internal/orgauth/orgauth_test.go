package orgauth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/IsaacJootar/kladd/backend/internal/auth"
	"github.com/IsaacJootar/kladd/backend/internal/claimrequests"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type recordingStore struct {
	keyHash      string
	record       IssueRecord
	register     RegisterRecord
	organization claimrequests.Organization
	issued       IssuedAPIKey
	account      Account
	credentials  Credentials
	loginAccount Account
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

func (store *recordingStore) RegisterAccount(ctx context.Context, record RegisterRecord) (Account, error) {
	store.register = record
	if store.err != nil {
		return Account{}, store.err
	}
	if store.account.ID != uuid.Nil {
		return store.account, nil
	}
	return Account{
		ID:                 record.ID,
		Name:               record.Name,
		Email:              record.Email,
		OrganizationType:   record.OrganizationType,
		VerificationStatus: record.VerificationStatus,
		CreatedAt:          record.CreatedAt,
	}, nil
}

func (store *recordingStore) FindCredentialsByEmail(ctx context.Context, email string) (Credentials, error) {
	if store.err != nil {
		return Credentials{}, store.err
	}
	return store.credentials, nil
}

func (store *recordingStore) RecordLogin(ctx context.Context, account Account) error {
	store.loginAccount = account
	return store.err
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

func TestServiceRegisterAccountHashesPassword(t *testing.T) {
	store := &recordingStore{}
	service := NewService(store)

	account, err := service.RegisterAccount(context.Background(), RegisterInput{
		Name:             " Acme Bank ",
		Email:            " Admin@Example.COM ",
		Password:         "strong-password",
		OrganizationType: "bank",
	})
	if err != nil {
		t.Fatalf("register account: %v", err)
	}

	if account.Name != "Acme Bank" {
		t.Fatalf("name = %q, want Acme Bank", account.Name)
	}
	if account.Email != "admin@example.com" {
		t.Fatalf("email = %q, want admin@example.com", account.Email)
	}
	if store.register.PasswordHash == "" || store.register.PasswordHash == "strong-password" {
		t.Fatal("expected hashed password, not raw password")
	}
	if bcrypt.CompareHashAndPassword([]byte(store.register.PasswordHash), []byte("strong-password")) != nil {
		t.Fatal("stored password hash does not match password")
	}
	if store.register.VerificationStatus != "unverified" {
		t.Fatalf("verification status = %q, want unverified", store.register.VerificationStatus)
	}
}

func TestServiceRegisterAccountValidatesInput(t *testing.T) {
	tests := []struct {
		name  string
		input RegisterInput
		err   error
	}{
		{name: "missing name", input: RegisterInput{Email: "admin@example.com", Password: "strong-password"}, err: ErrInvalidOrganization},
		{name: "bad email", input: RegisterInput{Name: "Acme Bank", Email: "bad", Password: "strong-password"}, err: ErrInvalidEmail},
		{name: "short password", input: RegisterInput{Name: "Acme Bank", Email: "admin@example.com", Password: "short"}, err: ErrInvalidPassword},
	}

	service := NewService(&recordingStore{})
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.RegisterAccount(context.Background(), test.input)
			if !errors.Is(err, test.err) {
				t.Fatalf("err = %v, want %v", err, test.err)
			}
		})
	}
}

func TestServiceLoginIssuesTokenAndRecordsAudit(t *testing.T) {
	orgID := uuid.New()
	hash, err := bcrypt.GenerateFromPassword([]byte("strong-password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	store := &recordingStore{
		credentials: Credentials{
			Account: Account{
				ID:                 orgID,
				Name:               "Acme Bank",
				Email:              "admin@example.com",
				OrganizationType:   "bank",
				VerificationStatus: "unverified",
				CreatedAt:          now,
			},
			PasswordHash: string(hash),
		},
	}
	service := NewServiceWithTokenManager(store, auth.NewTokenManagerWithClock("test-secret", time.Hour, func() time.Time {
		return now
	}))

	result, err := service.Login(context.Background(), LoginInput{
		Email:    " Admin@Example.COM ",
		Password: "strong-password",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if result.AccessToken == "" {
		t.Fatal("expected access token")
	}
	if result.TokenType != auth.TokenTypeBearer {
		t.Fatalf("token type = %q, want %q", result.TokenType, auth.TokenTypeBearer)
	}
	if result.Organization.ID != orgID {
		t.Fatalf("organization id = %s, want %s", result.Organization.ID, orgID)
	}
	if store.loginAccount.ID != orgID {
		t.Fatalf("login audit organization id = %s, want %s", store.loginAccount.ID, orgID)
	}
}

func TestServiceLoginRejectsBadPassword(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("strong-password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	store := &recordingStore{
		credentials: Credentials{
			Account:      Account{ID: uuid.New(), Email: "admin@example.com"},
			PasswordHash: string(hash),
		},
	}
	service := NewServiceWithTokenManager(store, auth.NewTokenManager("test-secret", time.Hour))

	_, err = service.Login(context.Background(), LoginInput{
		Email:    "admin@example.com",
		Password: "wrong-password",
	})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("err = %v, want %v", err, ErrInvalidCredentials)
	}
}
