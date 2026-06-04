package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/IsaacJootar/kladd/backend/internal/audit"
	"github.com/IsaacJootar/kladd/backend/internal/auth"
	"github.com/IsaacJootar/kladd/backend/internal/claimrequests"
	"github.com/IsaacJootar/kladd/backend/internal/claims"
	"github.com/IsaacJootar/kladd/backend/internal/config"
	"github.com/IsaacJootar/kladd/backend/internal/evidence"
	"github.com/IsaacJootar/kladd/backend/internal/orgauth"
	"github.com/IsaacJootar/kladd/backend/internal/securitypin"
	"github.com/IsaacJootar/kladd/backend/internal/truths"
	"github.com/IsaacJootar/kladd/backend/internal/users"
	"github.com/IsaacJootar/kladd/backend/internal/webhooks"
	"github.com/google/uuid"
)

type fakeUserCreator struct {
	user  users.User
	err   error
	input users.CreateInput
}

func (creator *fakeUserCreator) Create(ctx context.Context, input users.CreateInput) (users.User, error) {
	creator.input = input
	if creator.err != nil {
		return users.User{}, creator.err
	}
	return creator.user, nil
}

type fakeUserGetter struct {
	user   users.User
	err    error
	userID uuid.UUID
	email  string
}

func (getter *fakeUserGetter) Get(ctx context.Context, id uuid.UUID) (users.User, error) {
	getter.userID = id
	if getter.err != nil {
		return users.User{}, getter.err
	}
	return getter.user, nil
}

func (getter *fakeUserGetter) GetByEmail(ctx context.Context, email string) (users.User, error) {
	getter.email = email
	if getter.err != nil {
		return users.User{}, getter.err
	}
	return getter.user, nil
}

type fakeSecurityPINSetter struct {
	result securitypin.SetupResult
	err    error
	input  securitypin.SetupInput
}

func (setter *fakeSecurityPINSetter) Setup(ctx context.Context, input securitypin.SetupInput) (securitypin.SetupResult, error) {
	setter.input = input
	if setter.err != nil {
		return securitypin.SetupResult{}, setter.err
	}
	return setter.result, nil
}

type fakeSecurityPINResetter struct {
	result securitypin.SetupResult
	err    error
	input  securitypin.ResetInput
}

func (resetter *fakeSecurityPINResetter) Reset(ctx context.Context, input securitypin.ResetInput) (securitypin.SetupResult, error) {
	resetter.input = input
	if resetter.err != nil {
		return securitypin.SetupResult{}, resetter.err
	}
	return resetter.result, nil
}

type fakeAuthenticator struct {
	loginResult auth.LoginResult
	loginErr    error
	loginInput  auth.LoginInput
	userID      uuid.UUID
	authErr     error
	token       string
}

func (authenticator *fakeAuthenticator) Login(ctx context.Context, input auth.LoginInput) (auth.LoginResult, error) {
	authenticator.loginInput = input
	if authenticator.loginErr != nil {
		return auth.LoginResult{}, authenticator.loginErr
	}
	return authenticator.loginResult, nil
}

func (authenticator *fakeAuthenticator) Authenticate(tokenString string) (uuid.UUID, error) {
	authenticator.token = tokenString
	if authenticator.authErr != nil {
		return uuid.Nil, authenticator.authErr
	}
	return authenticator.userID, nil
}

type fakeEvidenceManager struct {
	items  []evidence.EvidenceItem
	item   evidence.EvidenceItem
	err    error
	input  evidence.CreateInput
	userID uuid.UUID
}

func (manager *fakeEvidenceManager) Create(ctx context.Context, input evidence.CreateInput) (evidence.EvidenceItem, error) {
	manager.input = input
	if manager.err != nil {
		return evidence.EvidenceItem{}, manager.err
	}
	return manager.item, nil
}

func (manager *fakeEvidenceManager) List(ctx context.Context, userID uuid.UUID) ([]evidence.EvidenceItem, error) {
	manager.userID = userID
	if manager.err != nil {
		return nil, manager.err
	}
	return manager.items, nil
}

type fakeAuditLogLister struct {
	events         []audit.Event
	err            error
	userID         uuid.UUID
	organizationID uuid.UUID
}

func (lister *fakeAuditLogLister) ListForUser(ctx context.Context, userID uuid.UUID) ([]audit.Event, error) {
	lister.userID = userID
	if lister.err != nil {
		return nil, lister.err
	}
	return lister.events, nil
}

func (lister *fakeAuditLogLister) ListForOrganization(ctx context.Context, organizationID uuid.UUID) ([]audit.Event, error) {
	lister.organizationID = organizationID
	if lister.err != nil {
		return nil, lister.err
	}
	return lister.events, nil
}

type fakeTruthDefinitionLister struct {
	definitions []truths.Definition
	err         error
	called      bool
}

func (lister *fakeTruthDefinitionLister) ListDefinitions(ctx context.Context) ([]truths.Definition, error) {
	lister.called = true
	if lister.err != nil {
		return nil, lister.err
	}
	return lister.definitions, nil
}

type fakeClaimRequestManager struct {
	request  claimrequests.ClaimRequest
	requests []claimrequests.ClaimRequest
	approval claimrequests.ApprovalResult
	err      error
	input    claimrequests.CreateInput
	approve  claimrequests.ApproveInput
	deny     claimrequests.DenyInput
	userID   uuid.UUID
	orgID    uuid.UUID
	getID    uuid.UUID
}

func (manager *fakeClaimRequestManager) Create(ctx context.Context, input claimrequests.CreateInput) (claimrequests.ClaimRequest, error) {
	manager.input = input
	if manager.err != nil {
		return claimrequests.ClaimRequest{}, manager.err
	}
	return manager.request, nil
}

func (manager *fakeClaimRequestManager) ListForUser(ctx context.Context, userID uuid.UUID) ([]claimrequests.ClaimRequest, error) {
	manager.userID = userID
	if manager.err != nil {
		return nil, manager.err
	}
	return manager.requests, nil
}

func (manager *fakeClaimRequestManager) ListForOrganization(ctx context.Context, organizationID uuid.UUID) ([]claimrequests.ClaimRequest, error) {
	manager.orgID = organizationID
	if manager.err != nil {
		return nil, manager.err
	}
	return manager.requests, nil
}

func (manager *fakeClaimRequestManager) GetForUser(ctx context.Context, userID uuid.UUID, requestID uuid.UUID) (claimrequests.ClaimRequest, error) {
	manager.userID = userID
	manager.getID = requestID
	if manager.err != nil {
		return claimrequests.ClaimRequest{}, manager.err
	}
	return manager.request, nil
}

func (manager *fakeClaimRequestManager) Approve(ctx context.Context, input claimrequests.ApproveInput) (claimrequests.ApprovalResult, error) {
	manager.approve = input
	if manager.err != nil {
		return claimrequests.ApprovalResult{}, manager.err
	}
	return manager.approval, nil
}

func (manager *fakeClaimRequestManager) Deny(ctx context.Context, input claimrequests.DenyInput) (claimrequests.ClaimRequest, error) {
	manager.deny = input
	if manager.err != nil {
		return claimrequests.ClaimRequest{}, manager.err
	}
	return manager.request, nil
}

type fakeClaimManager struct {
	claim       claims.Claim
	claims      []claims.Claim
	err         error
	userID      uuid.UUID
	orgID       uuid.UUID
	getID       uuid.UUID
	statusID    uuid.UUID
	revokeID    uuid.UUID
	pinClaimID  uuid.UUID
	exchangePIN string
	pin         claims.ExchangePIN
}

func (manager *fakeClaimManager) ListForUser(ctx context.Context, userID uuid.UUID) ([]claims.Claim, error) {
	manager.userID = userID
	if manager.err != nil {
		return nil, manager.err
	}
	return manager.claims, nil
}

func (manager *fakeClaimManager) ListForOrganization(ctx context.Context, organizationID uuid.UUID) ([]claims.Claim, error) {
	manager.orgID = organizationID
	if manager.err != nil {
		return nil, manager.err
	}
	return manager.claims, nil
}

func (manager *fakeClaimManager) GetForUser(ctx context.Context, userID uuid.UUID, claimID uuid.UUID) (claims.Claim, error) {
	manager.userID = userID
	manager.getID = claimID
	if manager.err != nil {
		return claims.Claim{}, manager.err
	}
	return manager.claim, nil
}

func (manager *fakeClaimManager) GetStatus(ctx context.Context, claimID uuid.UUID) (claims.Claim, error) {
	manager.statusID = claimID
	if manager.err != nil {
		return claims.Claim{}, manager.err
	}
	return manager.claim, nil
}

func (manager *fakeClaimManager) Revoke(ctx context.Context, userID uuid.UUID, claimID uuid.UUID) (claims.Claim, error) {
	manager.userID = userID
	manager.revokeID = claimID
	if manager.err != nil {
		return claims.Claim{}, manager.err
	}
	manager.claim.Status = claims.StatusRevoked
	manager.claim.DetailsVisible = false
	manager.claim.ApprovedTruths = nil
	return manager.claim, nil
}

func (manager *fakeClaimManager) CreateExchangePIN(ctx context.Context, userID uuid.UUID, claimID uuid.UUID) (claims.ExchangePIN, error) {
	manager.userID = userID
	manager.pinClaimID = claimID
	if manager.err != nil {
		return claims.ExchangePIN{}, manager.err
	}
	return manager.pin, nil
}

func (manager *fakeClaimManager) ResolveExchangePIN(ctx context.Context, exchangePIN string) (claims.Claim, error) {
	manager.exchangePIN = exchangePIN
	if manager.err != nil {
		return claims.Claim{}, manager.err
	}
	return manager.claim, nil
}

type fakeOrganizationAuthenticator struct {
	organization claimrequests.Organization
	err          error
	apiKey       string
}

func (authenticator *fakeOrganizationAuthenticator) Authenticate(ctx context.Context, apiKey string) (claimrequests.Organization, error) {
	authenticator.apiKey = apiKey
	if authenticator.err != nil {
		return claimrequests.Organization{}, authenticator.err
	}
	return authenticator.organization, nil
}

type fakeWebhookEndpointManager struct {
	endpoint       webhooks.Endpoint
	deliveries     []webhooks.DeliveryLog
	err            error
	input          webhooks.ConfigureEndpointInput
	organizationID uuid.UUID
}

func (manager *fakeWebhookEndpointManager) ConfigureEndpoint(ctx context.Context, input webhooks.ConfigureEndpointInput) (webhooks.Endpoint, error) {
	manager.input = input
	if manager.err != nil {
		return webhooks.Endpoint{}, manager.err
	}
	return manager.endpoint, nil
}

func (manager *fakeWebhookEndpointManager) GetEndpointForOrganization(ctx context.Context, organizationID uuid.UUID) (webhooks.Endpoint, error) {
	manager.organizationID = organizationID
	if manager.err != nil {
		return webhooks.Endpoint{}, manager.err
	}
	return manager.endpoint, nil
}

func (manager *fakeWebhookEndpointManager) ListDeliveriesForOrganization(ctx context.Context, organizationID uuid.UUID) ([]webhooks.DeliveryLog, error) {
	manager.organizationID = organizationID
	if manager.err != nil {
		return nil, manager.err
	}
	return manager.deliveries, nil
}

func newTestRouter(userCreator *fakeUserCreator, userGetter *fakeUserGetter, pinSetter *fakeSecurityPINSetter, authenticator *fakeAuthenticator, evidenceManagers ...*fakeEvidenceManager) http.Handler {
	if userCreator == nil {
		userCreator = &fakeUserCreator{}
	}
	if userGetter == nil {
		userGetter = &fakeUserGetter{}
	}
	if pinSetter == nil {
		pinSetter = &fakeSecurityPINSetter{}
	}
	if authenticator == nil {
		authenticator = &fakeAuthenticator{userID: uuid.New()}
	}
	evidenceManager := &fakeEvidenceManager{}
	if len(evidenceManagers) > 0 && evidenceManagers[0] != nil {
		evidenceManager = evidenceManagers[0]
	}

	return NewRouter(config.Config{}, userCreator, userGetter, pinSetter, &fakeSecurityPINResetter{}, authenticator, evidenceManager, &fakeAuditLogLister{}, &fakeTruthDefinitionLister{}, &fakeClaimRequestManager{}, &fakeClaimManager{})
}

func TestCreateUserHandlerCreatesUser(t *testing.T) {
	creator := &fakeUserCreator{
		user: users.User{
			ID:                 uuid.New(),
			Name:               "Ada Lovelace",
			Email:              "ada@example.com",
			AccountType:        users.AccountTypeIndividual,
			VerificationStatus: users.VerificationStatusUnverified,
		},
	}
	router := newTestRouter(creator, nil, nil, nil)

	requestBody := `{"name":"Ada Lovelace","email":"ada@example.com","password":"strong-password","account_type":"individual"}`
	request := httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(requestBody))
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusCreated, response.Body.String())
	}

	if creator.input.Password != "strong-password" {
		t.Fatal("expected password to be passed to user service")
	}

	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if _, ok := payload["password"]; ok {
		t.Fatal("response exposed password")
	}

	if _, ok := payload["password_hash"]; ok {
		t.Fatal("response exposed password hash")
	}
}

func TestCreateUserHandlerRejectsInvalidJSON(t *testing.T) {
	router := newTestRouter(nil, nil, nil, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewBufferString(`{`))
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestCreateUserHandlerMapsValidationErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "invalid name", err: users.ErrInvalidName, status: http.StatusBadRequest},
		{name: "invalid email", err: users.ErrInvalidEmail, status: http.StatusBadRequest},
		{name: "invalid password", err: users.ErrInvalidPassword, status: http.StatusBadRequest},
		{name: "invalid account type", err: users.ErrInvalidAccountType, status: http.StatusBadRequest},
		{name: "email taken", err: users.ErrEmailTaken, status: http.StatusConflict},
		{name: "unknown error", err: errors.New("boom"), status: http.StatusInternalServerError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := newTestRouter(&fakeUserCreator{err: test.err}, nil, nil, nil)
			request := httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(`{"name":"Ada Lovelace","email":"ada@example.com","password":"strong-password"}`))
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
		})
	}
}

func TestCreateUserHandlerRequiresPost(t *testing.T) {
	router := newTestRouter(nil, nil, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func TestLoginHandlerLogsUserIn(t *testing.T) {
	userID := uuid.New()
	authenticator := &fakeAuthenticator{
		loginResult: auth.LoginResult{
			AccessToken: "token",
			TokenType:   auth.TokenTypeBearer,
			ExpiresAt:   time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC),
			User: users.User{
				ID:    userID,
				Name:  "Ada Lovelace",
				Email: "ada@example.com",
			},
		},
	}
	router := newTestRouter(nil, nil, nil, authenticator)

	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"ada@example.com","password":"strong-password"}`))
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}

	if authenticator.loginInput.Email != "ada@example.com" {
		t.Fatalf("email = %q, want ada@example.com", authenticator.loginInput.Email)
	}

	if authenticator.loginInput.Password != "strong-password" {
		t.Fatal("expected password to be passed to auth service")
	}

	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if _, ok := payload["password"]; ok {
		t.Fatal("response exposed password")
	}

	if _, ok := payload["password_hash"]; ok {
		t.Fatal("response exposed password hash")
	}
}

func TestLoginHandlerRejectsInvalidJSON(t *testing.T) {
	router := newTestRouter(nil, nil, nil, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{`))
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestLoginHandlerMapsErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "invalid credentials", err: auth.ErrInvalidCredentials, status: http.StatusUnauthorized},
		{name: "unknown error", err: errors.New("boom"), status: http.StatusInternalServerError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := newTestRouter(nil, nil, nil, &fakeAuthenticator{loginErr: test.err})
			request := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"ada@example.com","password":"strong-password"}`))
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
		})
	}
}

func TestLoginHandlerRequiresPost(t *testing.T) {
	router := newTestRouter(nil, nil, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/auth/login", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func TestCurrentAccountHandlerReturnsAuthenticatedUser(t *testing.T) {
	userID := uuid.New()
	getter := &fakeUserGetter{
		user: users.User{
			ID:                 userID,
			Name:               "Ada Lovelace",
			Email:              "ada@example.com",
			AccountType:        users.AccountTypeIndividual,
			VerificationStatus: users.VerificationStatusUnverified,
		},
	}
	authenticator := &fakeAuthenticator{userID: userID}
	router := newTestRouter(nil, getter, nil, authenticator)

	request := httptest.NewRequest(http.MethodGet, "/api/account/me", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}

	if getter.userID != userID {
		t.Fatalf("user id = %s, want %s", getter.userID, userID)
	}

	if authenticator.token != "test-token" {
		t.Fatalf("token = %q, want test-token", authenticator.token)
	}

	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if _, ok := payload["password"]; ok {
		t.Fatal("response exposed password")
	}

	if _, ok := payload["password_hash"]; ok {
		t.Fatal("response exposed password hash")
	}

	if _, ok := payload["security_pin"]; ok {
		t.Fatal("response exposed security pin")
	}

	if _, ok := payload["security_pin_hash"]; ok {
		t.Fatal("response exposed security pin hash")
	}
}

func TestCurrentAccountHandlerRequiresBearerToken(t *testing.T) {
	router := newTestRouter(nil, nil, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/account/me", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestCurrentAccountHandlerRejectsInvalidToken(t *testing.T) {
	router := newTestRouter(nil, nil, nil, &fakeAuthenticator{authErr: auth.ErrInvalidToken})
	request := httptest.NewRequest(http.MethodGet, "/api/account/me", nil)
	request.Header.Set("Authorization", "Bearer bad-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestCurrentAccountHandlerMapsErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "user not found", err: users.ErrUserNotFound, status: http.StatusNotFound},
		{name: "unknown error", err: errors.New("boom"), status: http.StatusInternalServerError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := newTestRouter(nil, &fakeUserGetter{err: test.err}, nil, &fakeAuthenticator{userID: uuid.New()})
			request := httptest.NewRequest(http.MethodGet, "/api/account/me", nil)
			request.Header.Set("Authorization", "Bearer test-token")
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
		})
	}
}

func TestCurrentAccountHandlerRequiresGet(t *testing.T) {
	router := newTestRouter(nil, nil, nil, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/account/me", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func TestSetupSecurityPINHandlerSetsPIN(t *testing.T) {
	userID := uuid.New()
	setter := &fakeSecurityPINSetter{
		result: securitypin.SetupResult{
			UserID: userID,
			Set:    true,
			SetAt:  time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC),
		},
	}
	authenticator := &fakeAuthenticator{userID: userID}
	router := newTestRouter(nil, nil, setter, authenticator)

	requestBody := `{"security_pin":"4829"}`
	request := httptest.NewRequest(http.MethodPost, "/api/account/security-pin", strings.NewReader(requestBody))
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}

	if setter.input.UserID != userID {
		t.Fatalf("user id = %s, want %s", setter.input.UserID, userID)
	}

	if setter.input.PIN != "4829" {
		t.Fatal("expected pin to be passed to security pin service")
	}

	if authenticator.token != "test-token" {
		t.Fatalf("token = %q, want test-token", authenticator.token)
	}

	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if _, ok := payload["security_pin"]; ok {
		t.Fatal("response exposed security pin")
	}

	if _, ok := payload["security_pin_hash"]; ok {
		t.Fatal("response exposed security pin hash")
	}
}

func TestSetupSecurityPINHandlerRejectsInvalidJSON(t *testing.T) {
	router := newTestRouter(nil, nil, nil, &fakeAuthenticator{userID: uuid.New()})
	request := httptest.NewRequest(http.MethodPost, "/api/account/security-pin", bytes.NewBufferString(`{`))
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestSetupSecurityPINHandlerRequiresBearerToken(t *testing.T) {
	router := newTestRouter(nil, nil, nil, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/account/security-pin", strings.NewReader(`{"security_pin":"4829"}`))
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestSetupSecurityPINHandlerRejectsInvalidToken(t *testing.T) {
	router := newTestRouter(nil, nil, nil, &fakeAuthenticator{authErr: auth.ErrInvalidToken})
	request := httptest.NewRequest(http.MethodPost, "/api/account/security-pin", strings.NewReader(`{"security_pin":"4829"}`))
	request.Header.Set("Authorization", "Bearer bad-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestSetupSecurityPINHandlerMapsErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "invalid pin", err: securitypin.ErrInvalidFormat, status: http.StatusBadRequest},
		{name: "user not found", err: securitypin.ErrUserNotFound, status: http.StatusNotFound},
		{name: "unknown error", err: errors.New("boom"), status: http.StatusInternalServerError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := newTestRouter(nil, nil, &fakeSecurityPINSetter{err: test.err}, &fakeAuthenticator{userID: uuid.New()})
			request := httptest.NewRequest(http.MethodPost, "/api/account/security-pin", strings.NewReader(`{"security_pin":"4829"}`))
			request.Header.Set("Authorization", "Bearer test-token")
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
		})
	}
}

func TestSetupSecurityPINHandlerRequiresPost(t *testing.T) {
	router := newTestRouter(nil, nil, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/account/security-pin", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func TestResetSecurityPINHandlerResetsPINAfterPasswordCheck(t *testing.T) {
	userID := uuid.New()
	resetter := &fakeSecurityPINResetter{
		result: securitypin.SetupResult{
			UserID: userID,
			Set:    true,
			SetAt:  time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC),
		},
	}
	authenticator := &fakeAuthenticator{userID: userID}
	router := NewRouter(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		resetter,
		authenticator,
		&fakeEvidenceManager{},
		&fakeAuditLogLister{},
		&fakeTruthDefinitionLister{},
		&fakeClaimRequestManager{},
		&fakeClaimManager{},
	)

	requestBody := `{"password":"account-password","security_pin":"7391"}`
	request := httptest.NewRequest(http.MethodPost, "/api/account/security-pin/reset", strings.NewReader(requestBody))
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}
	if resetter.input.UserID != userID {
		t.Fatalf("user id = %s, want %s", resetter.input.UserID, userID)
	}
	if resetter.input.Password != "account-password" {
		t.Fatal("expected password to be passed for re-authentication")
	}
	if resetter.input.PIN != "7391" {
		t.Fatal("expected new pin to be passed to reset service")
	}
	if authenticator.token != "test-token" {
		t.Fatalf("token = %q, want test-token", authenticator.token)
	}

	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	for _, forbidden := range []string{"security_pin", "security_pin_hash", "password", "password_hash"} {
		if _, ok := payload[forbidden]; ok {
			t.Fatalf("response exposed forbidden field %q", forbidden)
		}
	}
}

func TestResetSecurityPINHandlerMapsErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "invalid pin", err: securitypin.ErrInvalidFormat, status: http.StatusBadRequest},
		{name: "invalid password", err: securitypin.ErrInvalidPassword, status: http.StatusUnauthorized},
		{name: "user not found", err: securitypin.ErrUserNotFound, status: http.StatusNotFound},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := NewRouter(
				config.Config{},
				&fakeUserCreator{},
				&fakeUserGetter{},
				&fakeSecurityPINSetter{},
				&fakeSecurityPINResetter{err: test.err},
				&fakeAuthenticator{userID: uuid.New()},
				&fakeEvidenceManager{},
				&fakeAuditLogLister{},
				&fakeTruthDefinitionLister{},
				&fakeClaimRequestManager{},
				&fakeClaimManager{},
			)
			request := httptest.NewRequest(http.MethodPost, "/api/account/security-pin/reset", strings.NewReader(`{"password":"account-password","security_pin":"7391"}`))
			request.Header.Set("Authorization", "Bearer test-token")
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
		})
	}
}

func TestResetSecurityPINHandlerRequiresBearerToken(t *testing.T) {
	router := newTestRouter(nil, nil, nil, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/account/security-pin/reset", strings.NewReader(`{"password":"account-password","security_pin":"7391"}`))
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestResetSecurityPINHandlerRequiresPost(t *testing.T) {
	router := newTestRouter(nil, nil, nil, &fakeAuthenticator{userID: uuid.New()})
	request := httptest.NewRequest(http.MethodGet, "/api/account/security-pin/reset", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func TestEvidenceItemsHandlerListsMetadataOnly(t *testing.T) {
	userID := uuid.New()
	manager := &fakeEvidenceManager{
		items: []evidence.EvidenceItem{
			{
				ID:          uuid.New(),
				Category:    "passport",
				DisplayName: "Passport",
				FileName:    "passport.pdf",
				ContentType: "application/pdf",
				SizeBytes:   12,
				Status:      evidence.StatusUploaded,
				UploadedAt:  time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC),
			},
		},
	}
	router := newTestRouter(nil, nil, nil, &fakeAuthenticator{userID: userID}, manager)

	request := httptest.NewRequest(http.MethodGet, "/api/evidence-items", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}

	if manager.userID != userID {
		t.Fatalf("user id = %s, want %s", manager.userID, userID)
	}

	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if _, ok := payload["file_path"]; ok {
		t.Fatal("response exposed file path")
	}

	if strings.Contains(response.Body.String(), "storage") {
		t.Fatal("response exposed storage details")
	}
}

func TestEvidenceItemsHandlerCreatesEvidenceItem(t *testing.T) {
	userID := uuid.New()
	manager := &fakeEvidenceManager{
		item: evidence.EvidenceItem{
			ID:          uuid.New(),
			Category:    "passport",
			DisplayName: "Passport",
			FileName:    "passport.pdf",
			ContentType: "application/pdf",
			SizeBytes:   12,
			Status:      evidence.StatusUploaded,
			UploadedAt:  time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC),
		},
	}
	router := newTestRouter(nil, nil, nil, &fakeAuthenticator{userID: userID}, manager)

	body, contentType := multipartEvidenceBody(t, map[string]string{
		"category":     "passport",
		"display_name": "Passport",
	}, "passport.pdf", "fake-content")
	request := httptest.NewRequest(http.MethodPost, "/api/evidence-items", body)
	request.Header.Set("Authorization", "Bearer test-token")
	request.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusCreated, response.Body.String())
	}

	if manager.input.UserID != userID {
		t.Fatalf("user id = %s, want %s", manager.input.UserID, userID)
	}

	if manager.input.Category != "passport" {
		t.Fatalf("category = %q, want passport", manager.input.Category)
	}

	if manager.input.DisplayName != "Passport" {
		t.Fatalf("display name = %q, want Passport", manager.input.DisplayName)
	}

	if manager.input.FileName != "passport.pdf" {
		t.Fatalf("file name = %q, want passport.pdf", manager.input.FileName)
	}

	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if _, ok := payload["file_path"]; ok {
		t.Fatal("response exposed file path")
	}
}

func TestEvidenceItemsHandlerRequiresBearerToken(t *testing.T) {
	router := newTestRouter(nil, nil, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/evidence-items", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestEvidenceItemsHandlerMapsCreateErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "invalid category", err: evidence.ErrInvalidCategory, status: http.StatusBadRequest},
		{name: "invalid file", err: evidence.ErrInvalidFile, status: http.StatusBadRequest},
		{name: "unknown error", err: errors.New("boom"), status: http.StatusInternalServerError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := newTestRouter(nil, nil, nil, &fakeAuthenticator{userID: uuid.New()}, &fakeEvidenceManager{err: test.err})
			body, contentType := multipartEvidenceBody(t, map[string]string{
				"category": "passport",
			}, "passport.pdf", "fake-content")
			request := httptest.NewRequest(http.MethodPost, "/api/evidence-items", body)
			request.Header.Set("Authorization", "Bearer test-token")
			request.Header.Set("Content-Type", contentType)
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
		})
	}
}

func TestEvidenceItemsHandlerRequiresKnownMethod(t *testing.T) {
	router := newTestRouter(nil, nil, nil, &fakeAuthenticator{userID: uuid.New()})
	request := httptest.NewRequest(http.MethodDelete, "/api/evidence-items", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func TestAuditLogsHandlerListsSafeUserEvents(t *testing.T) {
	userID := uuid.New()
	lister := &fakeAuditLogLister{
		events: []audit.Event{
			{
				ID:          uuid.New(),
				EventType:   "security_pin.reset",
				Title:       "Security PIN reset",
				Description: "Your Security PIN was reset after account confirmation.",
				CreatedAt:   time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC),
			},
		},
	}
	router := NewRouter(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: userID},
		&fakeEvidenceManager{},
		lister,
		&fakeTruthDefinitionLister{},
		&fakeClaimRequestManager{},
		&fakeClaimManager{},
	)

	request := httptest.NewRequest(http.MethodGet, "/api/audit-logs", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}
	if lister.userID != userID {
		t.Fatalf("user id = %s, want %s", lister.userID, userID)
	}

	body := response.Body.String()
	if !strings.Contains(body, "Security PIN reset") {
		t.Fatal("expected safe event title in response")
	}
	for _, forbidden := range []string{"metadata_json", "password", "password_hash", "security_pin_hash", "file_path", "raw_document"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response exposed forbidden field %q", forbidden)
		}
	}
}

func TestAuditLogsHandlerRequiresBearerToken(t *testing.T) {
	router := newTestRouter(nil, nil, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/audit-logs", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestAuditLogsHandlerMapsErrors(t *testing.T) {
	router := NewRouter(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: uuid.New()},
		&fakeEvidenceManager{},
		&fakeAuditLogLister{err: errors.New("boom")},
		&fakeTruthDefinitionLister{},
		&fakeClaimRequestManager{},
		&fakeClaimManager{},
	)
	request := httptest.NewRequest(http.MethodGet, "/api/audit-logs", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
}

func TestAuditLogsHandlerRequiresGet(t *testing.T) {
	router := newTestRouter(nil, nil, nil, &fakeAuthenticator{userID: uuid.New()})
	request := httptest.NewRequest(http.MethodPost, "/api/audit-logs", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func TestTruthDefinitionsHandlerListsDefinitionMetadata(t *testing.T) {
	lister := &fakeTruthDefinitionLister{
		definitions: []truths.Definition{
			{
				ID:               uuid.New(),
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
	}
	router := NewRouter(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: uuid.New()},
		&fakeEvidenceManager{},
		&fakeAuditLogLister{},
		lister,
		&fakeClaimRequestManager{},
		&fakeClaimManager{},
	)

	request := httptest.NewRequest(http.MethodGet, "/api/truth-definitions", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}

	if !lister.called {
		t.Fatal("expected truth definitions lister to be called")
	}

	body := response.Body.String()
	for _, forbidden := range []string{"truth_value", "raw_document", "bvn", "nin", "passport_number", "tax_id"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response exposed forbidden field %q", forbidden)
		}
	}
}

func TestTruthDefinitionsHandlerRequiresBearerToken(t *testing.T) {
	router := NewRouter(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{},
		&fakeEvidenceManager{},
		&fakeAuditLogLister{},
		&fakeTruthDefinitionLister{},
		&fakeClaimRequestManager{},
		&fakeClaimManager{},
	)
	request := httptest.NewRequest(http.MethodGet, "/api/truth-definitions", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestTruthDefinitionsHandlerMapsErrors(t *testing.T) {
	router := NewRouter(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: uuid.New()},
		&fakeEvidenceManager{},
		&fakeAuditLogLister{},
		&fakeTruthDefinitionLister{err: errors.New("boom")},
		&fakeClaimRequestManager{},
		&fakeClaimManager{},
	)
	request := httptest.NewRequest(http.MethodGet, "/api/truth-definitions", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
}

func TestTruthDefinitionsHandlerRequiresGet(t *testing.T) {
	router := NewRouter(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: uuid.New()},
		&fakeEvidenceManager{},
		&fakeAuditLogLister{},
		&fakeTruthDefinitionLister{},
		&fakeClaimRequestManager{},
		&fakeClaimManager{},
	)
	request := httptest.NewRequest(http.MethodPost, "/api/truth-definitions", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func TestClaimRequestsHandlerListsPendingRequests(t *testing.T) {
	userID := uuid.New()
	manager := &fakeClaimRequestManager{
		requests: []claimrequests.ClaimRequest{
			{
				ID: uuid.New(),
				Organization: claimrequests.Organization{
					ID:               uuid.New(),
					Name:             "Acme Bank",
					OrganizationType: "bank",
				},
				UserID:          userID,
				Purpose:         "Employment onboarding",
				RequestedTruths: []string{"identity_verified", "degree_verified"},
				Status:          claimrequests.StatusPendingApproval,
				ExpiresAt:       time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC),
				CreatedAt:       time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
			},
		},
	}
	router := NewRouter(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: userID},
		&fakeEvidenceManager{},
		&fakeAuditLogLister{},
		&fakeTruthDefinitionLister{},
		manager,
		&fakeClaimManager{},
	)

	request := httptest.NewRequest(http.MethodGet, "/api/claim-requests", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}
	if manager.userID != userID {
		t.Fatalf("user id = %s, want %s", manager.userID, userID)
	}

	body := response.Body.String()
	for _, forbidden := range []string{"truth_value", "raw_document", "file_path", "security_pin"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response exposed forbidden field %q", forbidden)
		}
	}
}

func TestClaimRequestsHandlerCreatesPendingRequest(t *testing.T) {
	userID := uuid.New()
	requestID := uuid.New()
	manager := &fakeClaimRequestManager{
		request: claimrequests.ClaimRequest{
			ID: requestID,
			Organization: claimrequests.Organization{
				ID:               uuid.New(),
				Name:             "Acme Bank",
				OrganizationType: "bank",
			},
			UserID:          userID,
			Purpose:         "Employment onboarding",
			RequestedTruths: []string{"identity_verified"},
			Status:          claimrequests.StatusPendingApproval,
			ExpiresAt:       time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC),
			CreatedAt:       time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
		},
	}
	router := NewRouter(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: userID},
		&fakeEvidenceManager{},
		&fakeAuditLogLister{},
		&fakeTruthDefinitionLister{},
		manager,
		&fakeClaimManager{},
	)

	body := `{"organization_name":"Acme Bank","organization_type":"bank","purpose":"Employment onboarding","requested_truths":["identity_verified"],"duration_days":30}`
	request := httptest.NewRequest(http.MethodPost, "/api/claim-requests", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusCreated, response.Body.String())
	}
	if manager.input.UserID != userID {
		t.Fatalf("user id = %s, want %s", manager.input.UserID, userID)
	}
	if manager.input.OrganizationName != "Acme Bank" {
		t.Fatalf("organization name = %q, want Acme Bank", manager.input.OrganizationName)
	}
	if manager.input.RequestedTruths[0] != "identity_verified" {
		t.Fatalf("requested truth = %q, want identity_verified", manager.input.RequestedTruths[0])
	}
}

func TestOrganizationClaimRequestsHandlerCreatesPendingRequest(t *testing.T) {
	userID := uuid.New()
	orgID := uuid.New()
	requestID := uuid.New()
	userGetter := &fakeUserGetter{
		user: users.User{
			ID:    userID,
			Email: "ada@example.com",
		},
	}
	manager := &fakeClaimRequestManager{
		request: claimrequests.ClaimRequest{
			ID:     requestID,
			UserID: userID,
			Organization: claimrequests.Organization{
				ID:               orgID,
				Name:             "Acme Bank",
				OrganizationType: "bank",
			},
			Purpose:         "Account opening",
			RequestedTruths: []string{"identity_verified"},
			Status:          claimrequests.StatusPendingApproval,
			ExpiresAt:       time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC),
			CreatedAt:       time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
		},
	}
	orgAuthenticator := &fakeOrganizationAuthenticator{
		organization: claimrequests.Organization{
			ID:               orgID,
			Name:             "Acme Bank",
			OrganizationType: "bank",
		},
	}
	router := NewRouterWithOrganizationAPI(
		config.Config{},
		&fakeUserCreator{},
		userGetter,
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: uuid.New()},
		&fakeEvidenceManager{},
		&fakeAuditLogLister{},
		&fakeTruthDefinitionLister{},
		manager,
		&fakeClaimManager{},
		orgAuthenticator,
	)

	body := `{"user_email":"ada@example.com","purpose":"Account opening","requested_truths":["identity_verified"],"duration_days":30}`
	request := httptest.NewRequest(http.MethodPost, "/api/organization/claim-requests", strings.NewReader(body))
	request.Header.Set("X-Kladd-API-Key", "test-api-key")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusCreated, response.Body.String())
	}
	if orgAuthenticator.apiKey != "test-api-key" {
		t.Fatalf("api key = %q, want test-api-key", orgAuthenticator.apiKey)
	}
	if userGetter.email != "ada@example.com" {
		t.Fatalf("user email = %q, want ada@example.com", userGetter.email)
	}
	if manager.input.UserID != userID {
		t.Fatalf("user id = %s, want %s", manager.input.UserID, userID)
	}
	if manager.input.OrganizationName != "Acme Bank" {
		t.Fatalf("organization name = %q, want Acme Bank", manager.input.OrganizationName)
	}
	if manager.input.OrganizationType != "bank" {
		t.Fatalf("organization type = %q, want bank", manager.input.OrganizationType)
	}
	if manager.input.RequestedTruths[0] != "identity_verified" {
		t.Fatalf("requested truth = %q, want identity_verified", manager.input.RequestedTruths[0])
	}

	responseBody := response.Body.String()
	for _, forbidden := range []string{"raw_document", "file_path", "security_pin", "security_pin_hash", "truth_value", "api_key", "key_hash"} {
		if strings.Contains(responseBody, forbidden) {
			t.Fatalf("response exposed forbidden field %q", forbidden)
		}
	}
}

func TestOrganizationClaimRequestsHandlerRequiresAPIKey(t *testing.T) {
	router := NewRouterWithOrganizationAPI(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: uuid.New()},
		&fakeEvidenceManager{},
		&fakeAuditLogLister{},
		&fakeTruthDefinitionLister{},
		&fakeClaimRequestManager{},
		&fakeClaimManager{},
		&fakeOrganizationAuthenticator{},
	)

	request := httptest.NewRequest(http.MethodPost, "/api/organization/claim-requests", strings.NewReader(`{}`))
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestOrganizationClaimRequestsHandlerMapsInvalidAPIKey(t *testing.T) {
	router := NewRouterWithOrganizationAPI(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: uuid.New()},
		&fakeEvidenceManager{},
		&fakeAuditLogLister{},
		&fakeTruthDefinitionLister{},
		&fakeClaimRequestManager{},
		&fakeClaimManager{},
		&fakeOrganizationAuthenticator{err: orgauth.ErrInvalidAPIKey},
	)

	request := httptest.NewRequest(http.MethodPost, "/api/organization/claim-requests", strings.NewReader(`{}`))
	request.Header.Set("X-Kladd-API-Key", "bad-key")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestOrganizationClaimRequestsHandlerMapsMissingTargetUser(t *testing.T) {
	router := NewRouterWithOrganizationAPI(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{err: users.ErrUserNotFound},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: uuid.New()},
		&fakeEvidenceManager{},
		&fakeAuditLogLister{},
		&fakeTruthDefinitionLister{},
		&fakeClaimRequestManager{},
		&fakeClaimManager{},
		&fakeOrganizationAuthenticator{organization: claimrequests.Organization{ID: uuid.New(), Name: "Acme Bank", OrganizationType: "bank"}},
	)

	request := httptest.NewRequest(http.MethodPost, "/api/organization/claim-requests", strings.NewReader(`{"user_email":"missing@example.com","purpose":"Account opening","requested_truths":["identity_verified"],"duration_days":30}`))
	request.Header.Set("X-Kladd-API-Key", "test-api-key")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestOrganizationAuditLogsHandlerListsSafeEvents(t *testing.T) {
	orgID := uuid.New()
	auditLister := &fakeAuditLogLister{
		events: []audit.Event{
			{
				ID:          uuid.New(),
				EventType:   "claim_request.created",
				Title:       "Activity recorded",
				Description: "A Kladd account activity was recorded.",
				CreatedAt:   time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC),
			},
		},
	}
	router := NewRouterWithOrganizationAPI(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: uuid.New()},
		&fakeEvidenceManager{},
		auditLister,
		&fakeTruthDefinitionLister{},
		&fakeClaimRequestManager{},
		&fakeClaimManager{},
		&fakeOrganizationAuthenticator{
			organization: claimrequests.Organization{
				ID:               orgID,
				Name:             "Acme Bank",
				OrganizationType: "bank",
			},
		},
	)

	request := httptest.NewRequest(http.MethodGet, "/api/organization/audit-logs", nil)
	request.Header.Set("X-Kladd-API-Key", "test-api-key")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}
	if auditLister.organizationID != orgID {
		t.Fatalf("organization id = %s, want %s", auditLister.organizationID, orgID)
	}

	var responseBody struct {
		Items []audit.Event `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &responseBody); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(responseBody.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(responseBody.Items))
	}

	body := response.Body.String()
	for _, forbidden := range []string{"metadata", "raw_document", "file_path", "security_pin", "security_pin_hash", "api_key", "key_hash", "truth_value"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response exposed forbidden field %q", forbidden)
		}
	}
}

func TestOrganizationAuditLogsHandlerRequiresAPIKey(t *testing.T) {
	router := NewRouterWithOrganizationAPI(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: uuid.New()},
		&fakeEvidenceManager{},
		&fakeAuditLogLister{},
		&fakeTruthDefinitionLister{},
		&fakeClaimRequestManager{},
		&fakeClaimManager{},
		&fakeOrganizationAuthenticator{},
	)

	request := httptest.NewRequest(http.MethodGet, "/api/organization/audit-logs", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestOrganizationProfileHandlerReturnsOrganization(t *testing.T) {
	orgID := uuid.New()
	orgAuthenticator := &fakeOrganizationAuthenticator{
		organization: claimrequests.Organization{
			ID:                 orgID,
			Name:               "Acme Bank",
			OrganizationType:   "bank",
			VerificationStatus: "verified",
		},
	}
	router := NewRouterWithOrganizationAPI(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: uuid.New()},
		&fakeEvidenceManager{},
		&fakeAuditLogLister{},
		&fakeTruthDefinitionLister{},
		&fakeClaimRequestManager{},
		&fakeClaimManager{},
		orgAuthenticator,
	)

	request := httptest.NewRequest(http.MethodGet, "/api/organization/me", nil)
	request.Header.Set("X-Kladd-API-Key", "test-api-key")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}
	if orgAuthenticator.apiKey != "test-api-key" {
		t.Fatalf("api key = %q, want test-api-key", orgAuthenticator.apiKey)
	}

	var organization claimrequests.Organization
	if err := json.Unmarshal(response.Body.Bytes(), &organization); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if organization.ID != orgID {
		t.Fatalf("organization id = %s, want %s", organization.ID, orgID)
	}
	if organization.Name != "Acme Bank" {
		t.Fatalf("organization name = %q, want Acme Bank", organization.Name)
	}

	responseBody := response.Body.String()
	for _, forbidden := range []string{"api_key", "key_hash", "raw_document", "security_pin", "security_pin_hash"} {
		if strings.Contains(responseBody, forbidden) {
			t.Fatalf("response exposed forbidden field %q", forbidden)
		}
	}
}

func TestOrganizationProfileHandlerRequiresAPIKey(t *testing.T) {
	router := NewRouterWithOrganizationAPI(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: uuid.New()},
		&fakeEvidenceManager{},
		&fakeAuditLogLister{},
		&fakeTruthDefinitionLister{},
		&fakeClaimRequestManager{},
		&fakeClaimManager{},
		&fakeOrganizationAuthenticator{},
	)

	request := httptest.NewRequest(http.MethodGet, "/api/organization/me", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestOrganizationProfileHandlerMapsInvalidAPIKey(t *testing.T) {
	router := NewRouterWithOrganizationAPI(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: uuid.New()},
		&fakeEvidenceManager{},
		&fakeAuditLogLister{},
		&fakeTruthDefinitionLister{},
		&fakeClaimRequestManager{},
		&fakeClaimManager{},
		&fakeOrganizationAuthenticator{err: orgauth.ErrInvalidAPIKey},
	)

	request := httptest.NewRequest(http.MethodGet, "/api/organization/me", nil)
	request.Header.Set("X-Kladd-API-Key", "bad-key")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestOrganizationClaimRequestsHandlerListsOrganizationRequests(t *testing.T) {
	orgID := uuid.New()
	requestID := uuid.New()
	manager := &fakeClaimRequestManager{
		requests: []claimrequests.ClaimRequest{
			{
				ID:     requestID,
				UserID: uuid.New(),
				Organization: claimrequests.Organization{
					ID:                 orgID,
					Name:               "Acme Bank",
					OrganizationType:   "bank",
					VerificationStatus: "verified",
				},
				Purpose:         "Account opening",
				RequestedTruths: []string{"identity_verified"},
				Status:          claimrequests.StatusPendingApproval,
				ExpiresAt:       time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC),
				CreatedAt:       time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
			},
		},
	}
	router := NewRouterWithOrganizationAPI(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: uuid.New()},
		&fakeEvidenceManager{},
		&fakeAuditLogLister{},
		&fakeTruthDefinitionLister{},
		manager,
		&fakeClaimManager{},
		&fakeOrganizationAuthenticator{
			organization: claimrequests.Organization{
				ID:               orgID,
				Name:             "Acme Bank",
				OrganizationType: "bank",
			},
		},
	)

	request := httptest.NewRequest(http.MethodGet, "/api/organization/claim-requests", nil)
	request.Header.Set("X-Kladd-API-Key", "test-api-key")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}
	if manager.orgID != orgID {
		t.Fatalf("organization id = %s, want %s", manager.orgID, orgID)
	}

	var responseBody struct {
		Items []claimrequests.ClaimRequest `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &responseBody); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(responseBody.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(responseBody.Items))
	}
	if responseBody.Items[0].ID != requestID {
		t.Fatalf("request id = %s, want %s", responseBody.Items[0].ID, requestID)
	}

	body := response.Body.String()
	for _, forbidden := range []string{"raw_document", "file_path", "security_pin", "security_pin_hash", "truth_value", "api_key", "key_hash"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response exposed forbidden field %q", forbidden)
		}
	}
}

func TestOrganizationClaimsHandlerListsOrganizationClaims(t *testing.T) {
	orgID := uuid.New()
	claimID := uuid.New()
	manager := &fakeClaimManager{
		claims: []claims.Claim{
			{
				ID:             claimID,
				ClaimRequestID: uuid.New(),
				Organization: claimrequests.Organization{
					ID:                 orgID,
					Name:               "Acme Bank",
					OrganizationType:   "bank",
					VerificationStatus: "verified",
				},
				Purpose:        "Account opening",
				Status:         claims.StatusExpired,
				IssuedAt:       time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
				ExpiresAt:      time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC),
				DetailsVisible: false,
			},
		},
	}
	router := NewRouterWithOrganizationAPI(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: uuid.New()},
		&fakeEvidenceManager{},
		&fakeAuditLogLister{},
		&fakeTruthDefinitionLister{},
		&fakeClaimRequestManager{},
		manager,
		&fakeOrganizationAuthenticator{
			organization: claimrequests.Organization{
				ID:               orgID,
				Name:             "Acme Bank",
				OrganizationType: "bank",
			},
		},
	)

	request := httptest.NewRequest(http.MethodGet, "/api/organization/claims", nil)
	request.Header.Set("X-Kladd-API-Key", "test-api-key")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}
	if manager.orgID != orgID {
		t.Fatalf("organization id = %s, want %s", manager.orgID, orgID)
	}

	var responseBody struct {
		Items []claims.Claim `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &responseBody); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(responseBody.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(responseBody.Items))
	}
	if responseBody.Items[0].ID != claimID {
		t.Fatalf("claim id = %s, want %s", responseBody.Items[0].ID, claimID)
	}
	if responseBody.Items[0].DetailsVisible {
		t.Fatal("expired claim details should not be visible")
	}

	body := response.Body.String()
	for _, forbidden := range []string{"approved_truths", "raw_document", "file_path", "security_pin", "security_pin_hash", "truth_value", "api_key", "key_hash"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response exposed forbidden field %q", forbidden)
		}
	}
}

func TestOrganizationWebhookEndpointHandlerConfiguresEndpoint(t *testing.T) {
	orgID := uuid.New()
	endpointID := uuid.New()
	manager := &fakeWebhookEndpointManager{
		endpoint: webhooks.Endpoint{
			ID: endpointID,
			Organization: webhooks.Organization{
				ID:                 orgID,
				Name:               "Acme Bank",
				OrganizationType:   "bank",
				VerificationStatus: "verified",
			},
			URL:       "https://example.com/kladd/webhooks",
			Status:    webhooks.EndpointStatusActive,
			CreatedAt: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC),
		},
	}
	router := NewRouterWithOrganizationAPI(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: uuid.New()},
		&fakeEvidenceManager{},
		&fakeAuditLogLister{},
		&fakeTruthDefinitionLister{},
		&fakeClaimRequestManager{},
		&fakeClaimManager{},
		&fakeOrganizationAuthenticator{
			organization: claimrequests.Organization{
				ID:               orgID,
				Name:             "Acme Bank",
				OrganizationType: "bank",
			},
		},
		manager,
	)

	request := httptest.NewRequest(http.MethodPost, "/api/organization/webhook-endpoint", strings.NewReader(`{"url":"https://example.com/kladd/webhooks"}`))
	request.Header.Set("X-Kladd-API-Key", "test-api-key")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}
	if manager.input.OrganizationName != "Acme Bank" {
		t.Fatalf("organization name = %q, want Acme Bank", manager.input.OrganizationName)
	}
	if manager.input.OrganizationType != "bank" {
		t.Fatalf("organization type = %q, want bank", manager.input.OrganizationType)
	}
	if manager.input.URL != "https://example.com/kladd/webhooks" {
		t.Fatalf("url = %q, want configured url", manager.input.URL)
	}

	var endpoint webhooks.Endpoint
	if err := json.Unmarshal(response.Body.Bytes(), &endpoint); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if endpoint.ID != endpointID {
		t.Fatalf("endpoint id = %s, want %s", endpoint.ID, endpointID)
	}

	body := response.Body.String()
	for _, forbidden := range []string{"payload_json", "signature", "security_pin", "api_key", "key_hash", "truth_value"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response exposed forbidden field %q", forbidden)
		}
	}
}

func TestOrganizationWebhookEndpointHandlerReturnsEndpoint(t *testing.T) {
	orgID := uuid.New()
	endpointID := uuid.New()
	manager := &fakeWebhookEndpointManager{
		endpoint: webhooks.Endpoint{
			ID: endpointID,
			Organization: webhooks.Organization{
				ID:                 orgID,
				Name:               "Acme Bank",
				OrganizationType:   "bank",
				VerificationStatus: "verified",
			},
			URL:       "https://example.com/kladd/webhooks",
			Status:    webhooks.EndpointStatusActive,
			CreatedAt: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC),
		},
	}
	router := NewRouterWithOrganizationAPI(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: uuid.New()},
		&fakeEvidenceManager{},
		&fakeAuditLogLister{},
		&fakeTruthDefinitionLister{},
		&fakeClaimRequestManager{},
		&fakeClaimManager{},
		&fakeOrganizationAuthenticator{
			organization: claimrequests.Organization{
				ID:               orgID,
				Name:             "Acme Bank",
				OrganizationType: "bank",
			},
		},
		manager,
	)

	request := httptest.NewRequest(http.MethodGet, "/api/organization/webhook-endpoint", nil)
	request.Header.Set("X-Kladd-API-Key", "test-api-key")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}
	if manager.organizationID != orgID {
		t.Fatalf("organization id = %s, want %s", manager.organizationID, orgID)
	}

	var endpoint webhooks.Endpoint
	if err := json.Unmarshal(response.Body.Bytes(), &endpoint); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if endpoint.ID != endpointID {
		t.Fatalf("endpoint id = %s, want %s", endpoint.ID, endpointID)
	}
	if endpoint.URL != "https://example.com/kladd/webhooks" {
		t.Fatalf("url = %q, want configured url", endpoint.URL)
	}

	body := response.Body.String()
	for _, forbidden := range []string{"payload_json", "signature", "security_pin", "api_key", "key_hash", "truth_value"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response exposed forbidden field %q", forbidden)
		}
	}
}

func TestOrganizationWebhookEndpointHandlerMapsMissingEndpoint(t *testing.T) {
	router := NewRouterWithOrganizationAPI(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: uuid.New()},
		&fakeEvidenceManager{},
		&fakeAuditLogLister{},
		&fakeTruthDefinitionLister{},
		&fakeClaimRequestManager{},
		&fakeClaimManager{},
		&fakeOrganizationAuthenticator{
			organization: claimrequests.Organization{
				ID:               uuid.New(),
				Name:             "Acme Bank",
				OrganizationType: "bank",
			},
		},
		&fakeWebhookEndpointManager{err: webhooks.ErrEndpointNotFound},
	)

	request := httptest.NewRequest(http.MethodGet, "/api/organization/webhook-endpoint", nil)
	request.Header.Set("X-Kladd-API-Key", "test-api-key")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusNotFound, response.Body.String())
	}
}

func TestOrganizationWebhookDeliveriesHandlerListsSafeDeliveryHistory(t *testing.T) {
	orgID := uuid.New()
	deliveryID := uuid.New()
	manager := &fakeWebhookEndpointManager{
		deliveries: []webhooks.DeliveryLog{
			{
				ID:             deliveryID,
				EventType:      webhooks.EventClaimApproved,
				AggregateID:    uuid.New(),
				OrganizationID: orgID,
				Status:         webhooks.StatusDelivered,
				Attempts:       1,
				CreatedAt:      time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC),
				UpdatedAt:      time.Date(2026, 6, 4, 12, 5, 0, 0, time.UTC),
			},
		},
	}
	router := NewRouterWithOrganizationAPI(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: uuid.New()},
		&fakeEvidenceManager{},
		&fakeAuditLogLister{},
		&fakeTruthDefinitionLister{},
		&fakeClaimRequestManager{},
		&fakeClaimManager{},
		&fakeOrganizationAuthenticator{
			organization: claimrequests.Organization{
				ID:               orgID,
				Name:             "Acme Bank",
				OrganizationType: "bank",
			},
		},
		manager,
	)

	request := httptest.NewRequest(http.MethodGet, "/api/organization/webhook-deliveries", nil)
	request.Header.Set("X-Kladd-API-Key", "test-api-key")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}
	if manager.organizationID != orgID {
		t.Fatalf("organization id = %s, want %s", manager.organizationID, orgID)
	}

	var responseBody struct {
		Items []webhooks.DeliveryLog `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &responseBody); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(responseBody.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(responseBody.Items))
	}
	if responseBody.Items[0].ID != deliveryID {
		t.Fatalf("delivery id = %s, want %s", responseBody.Items[0].ID, deliveryID)
	}

	body := response.Body.String()
	for _, forbidden := range []string{"payload_json", "signature", "raw_document", "file_path", "security_pin", "api_key", "key_hash", "truth_value"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response exposed forbidden field %q", forbidden)
		}
	}
}

func TestOrganizationWebhookEndpointHandlerMapsInvalidURL(t *testing.T) {
	router := NewRouterWithOrganizationAPI(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: uuid.New()},
		&fakeEvidenceManager{},
		&fakeAuditLogLister{},
		&fakeTruthDefinitionLister{},
		&fakeClaimRequestManager{},
		&fakeClaimManager{},
		&fakeOrganizationAuthenticator{
			organization: claimrequests.Organization{
				ID:               uuid.New(),
				Name:             "Acme Bank",
				OrganizationType: "bank",
			},
		},
		&fakeWebhookEndpointManager{err: webhooks.ErrInvalidEndpointURL},
	)

	request := httptest.NewRequest(http.MethodPost, "/api/organization/webhook-endpoint", strings.NewReader(`{"url":"ftp://example.com/hooks"}`))
	request.Header.Set("X-Kladd-API-Key", "test-api-key")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusBadRequest, response.Body.String())
	}
}

func TestClaimRequestByIDHandlerReturnsOwnedRequest(t *testing.T) {
	userID := uuid.New()
	requestID := uuid.New()
	manager := &fakeClaimRequestManager{
		request: claimrequests.ClaimRequest{
			ID:              requestID,
			Organization:    claimrequests.Organization{ID: uuid.New(), Name: "Acme Bank", OrganizationType: "bank"},
			UserID:          userID,
			Purpose:         "Employment onboarding",
			RequestedTruths: []string{"identity_verified"},
			Status:          claimrequests.StatusPendingApproval,
			ExpiresAt:       time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC),
			CreatedAt:       time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
		},
	}
	router := NewRouter(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: userID},
		&fakeEvidenceManager{},
		&fakeAuditLogLister{},
		&fakeTruthDefinitionLister{},
		manager,
		&fakeClaimManager{},
	)

	request := httptest.NewRequest(http.MethodGet, "/api/claim-requests/"+requestID.String(), nil)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}
	if manager.userID != userID {
		t.Fatalf("user id = %s, want %s", manager.userID, userID)
	}
	if manager.getID != requestID {
		t.Fatalf("request id = %s, want %s", manager.getID, requestID)
	}
}

func TestClaimRequestApproveHandlerApprovesWithSecurityPIN(t *testing.T) {
	userID := uuid.New()
	requestID := uuid.New()
	claimID := uuid.New()
	consentID := uuid.New()
	manager := &fakeClaimRequestManager{
		approval: claimrequests.ApprovalResult{
			ConsentID: consentID,
			ClaimID:   claimID,
			ClaimRequest: claimrequests.ClaimRequest{
				ID:              requestID,
				Organization:    claimrequests.Organization{ID: uuid.New(), Name: "Acme Bank", OrganizationType: "bank"},
				UserID:          userID,
				Purpose:         "Employment onboarding",
				RequestedTruths: []string{"identity_verified"},
				Status:          claimrequests.StatusApprovedWithPIN,
				ExpiresAt:       time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC),
				CreatedAt:       time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
			},
			ApprovedAt: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
		},
	}
	router := NewRouter(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: userID},
		&fakeEvidenceManager{},
		&fakeAuditLogLister{},
		&fakeTruthDefinitionLister{},
		manager,
		&fakeClaimManager{},
	)

	request := httptest.NewRequest(http.MethodPost, "/api/claim-requests/"+requestID.String()+"/approve", strings.NewReader(`{"security_pin":"4829"}`))
	request.Header.Set("Authorization", "Bearer test-token")
	request.Header.Set("X-Forwarded-For", "127.0.0.1")
	request.Header.Set("User-Agent", "test-agent")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}
	if manager.approve.UserID != userID {
		t.Fatalf("user id = %s, want %s", manager.approve.UserID, userID)
	}
	if manager.approve.RequestID != requestID {
		t.Fatalf("request id = %s, want %s", manager.approve.RequestID, requestID)
	}
	if manager.approve.SecurityPIN != "4829" {
		t.Fatalf("security pin = %q, want 4829", manager.approve.SecurityPIN)
	}
	if manager.approve.IPAddress != "127.0.0.1" {
		t.Fatalf("ip address = %q, want 127.0.0.1", manager.approve.IPAddress)
	}

	body := response.Body.String()
	for _, forbidden := range []string{`"security_pin"`, "security_pin_hash", "raw_document", "file_path", "truth_value"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response exposed forbidden field %q", forbidden)
		}
	}
}

func TestClaimRequestApproveHandlerMapsErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "not found", err: claimrequests.ErrClaimRequestNotFound, status: http.StatusNotFound},
		{name: "pin not set", err: securitypin.ErrPINNotSet, status: http.StatusBadRequest},
		{name: "bad pin", err: securitypin.ErrInvalidPIN, status: http.StatusUnauthorized},
		{name: "pin locked", err: securitypin.ErrPINLocked, status: http.StatusTooManyRequests},
		{name: "expired", err: claimrequests.ErrClaimRequestExpired, status: http.StatusConflict},
		{name: "not open", err: claimrequests.ErrClaimRequestNotOpen, status: http.StatusConflict},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := NewRouter(
				config.Config{},
				&fakeUserCreator{},
				&fakeUserGetter{},
				&fakeSecurityPINSetter{},
				&fakeSecurityPINResetter{},
				&fakeAuthenticator{userID: uuid.New()},
				&fakeEvidenceManager{},
				&fakeAuditLogLister{},
				&fakeTruthDefinitionLister{},
				&fakeClaimRequestManager{err: test.err},
				&fakeClaimManager{},
			)
			request := httptest.NewRequest(http.MethodPost, "/api/claim-requests/"+uuid.New().String()+"/approve", strings.NewReader(`{"security_pin":"4829"}`))
			request.Header.Set("Authorization", "Bearer test-token")
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
		})
	}
}

func TestClaimRequestApproveHandlerRequiresPost(t *testing.T) {
	router := newTestRouter(nil, nil, nil, &fakeAuthenticator{userID: uuid.New()})
	request := httptest.NewRequest(http.MethodGet, "/api/claim-requests/"+uuid.New().String()+"/approve", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func TestClaimRequestDenyHandlerDeniesRequest(t *testing.T) {
	userID := uuid.New()
	requestID := uuid.New()
	manager := &fakeClaimRequestManager{
		request: claimrequests.ClaimRequest{
			ID:              requestID,
			Organization:    claimrequests.Organization{ID: uuid.New(), Name: "Acme Bank", OrganizationType: "bank"},
			UserID:          userID,
			Purpose:         "Account opening",
			RequestedTruths: []string{"identity_verified"},
			Status:          claimrequests.StatusDenied,
			ExpiresAt:       time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC),
			CreatedAt:       time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
		},
	}
	router := NewRouter(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: userID},
		&fakeEvidenceManager{},
		&fakeAuditLogLister{},
		&fakeTruthDefinitionLister{},
		manager,
		&fakeClaimManager{},
	)

	request := httptest.NewRequest(http.MethodPost, "/api/claim-requests/"+requestID.String()+"/deny", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}
	if manager.deny.UserID != userID {
		t.Fatalf("user id = %s, want %s", manager.deny.UserID, userID)
	}
	if manager.deny.RequestID != requestID {
		t.Fatalf("request id = %s, want %s", manager.deny.RequestID, requestID)
	}

	body := response.Body.String()
	for _, forbidden := range []string{`"security_pin"`, "security_pin_hash", "raw_document", "file_path", "truth_value"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response exposed forbidden field %q", forbidden)
		}
	}
}

func TestClaimRequestDenyHandlerMapsErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "not found", err: claimrequests.ErrClaimRequestNotFound, status: http.StatusNotFound},
		{name: "expired", err: claimrequests.ErrClaimRequestExpired, status: http.StatusConflict},
		{name: "not open", err: claimrequests.ErrClaimRequestNotOpen, status: http.StatusConflict},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := NewRouter(
				config.Config{},
				&fakeUserCreator{},
				&fakeUserGetter{},
				&fakeSecurityPINSetter{},
				&fakeSecurityPINResetter{},
				&fakeAuthenticator{userID: uuid.New()},
				&fakeEvidenceManager{},
				&fakeAuditLogLister{},
				&fakeTruthDefinitionLister{},
				&fakeClaimRequestManager{err: test.err},
				&fakeClaimManager{},
			)
			request := httptest.NewRequest(http.MethodPost, "/api/claim-requests/"+uuid.New().String()+"/deny", nil)
			request.Header.Set("Authorization", "Bearer test-token")
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
		})
	}
}

func TestClaimRequestDenyHandlerRequiresPost(t *testing.T) {
	router := newTestRouter(nil, nil, nil, &fakeAuthenticator{userID: uuid.New()})
	request := httptest.NewRequest(http.MethodGet, "/api/claim-requests/"+uuid.New().String()+"/deny", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func TestClaimRequestsHandlerRequiresBearerToken(t *testing.T) {
	router := newTestRouter(nil, nil, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/claim-requests", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestClaimRequestsHandlerMapsCreateErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "invalid organization", err: claimrequests.ErrInvalidOrganization, status: http.StatusBadRequest},
		{name: "invalid purpose", err: claimrequests.ErrInvalidPurpose, status: http.StatusBadRequest},
		{name: "invalid scope", err: claimrequests.ErrInvalidScope, status: http.StatusBadRequest},
		{name: "invalid duration", err: claimrequests.ErrInvalidDuration, status: http.StatusBadRequest},
		{name: "unknown error", err: errors.New("boom"), status: http.StatusInternalServerError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := NewRouter(
				config.Config{},
				&fakeUserCreator{},
				&fakeUserGetter{},
				&fakeSecurityPINSetter{},
				&fakeSecurityPINResetter{},
				&fakeAuthenticator{userID: uuid.New()},
				&fakeEvidenceManager{},
				&fakeAuditLogLister{},
				&fakeTruthDefinitionLister{},
				&fakeClaimRequestManager{err: test.err},
				&fakeClaimManager{},
			)
			body := `{"organization_name":"Acme Bank","purpose":"Employment onboarding","requested_truths":["identity_verified"],"duration_days":30}`
			request := httptest.NewRequest(http.MethodPost, "/api/claim-requests", strings.NewReader(body))
			request.Header.Set("Authorization", "Bearer test-token")
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
		})
	}
}

func TestClaimRequestsHandlerRequiresKnownMethod(t *testing.T) {
	router := newTestRouter(nil, nil, nil, &fakeAuthenticator{userID: uuid.New()})
	request := httptest.NewRequest(http.MethodDelete, "/api/claim-requests", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func TestClaimsHandlerListsUserClaims(t *testing.T) {
	userID := uuid.New()
	manager := &fakeClaimManager{
		claims: []claims.Claim{
			{
				ID:             uuid.New(),
				ClaimRequestID: uuid.New(),
				Organization:   claimrequests.Organization{ID: uuid.New(), Name: "Acme Bank", OrganizationType: "bank"},
				Purpose:        "Employment onboarding",
				ApprovedTruths: []string{"identity_verified"},
				Status:         claims.StatusActive,
				IssuedAt:       time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
				ExpiresAt:      time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC),
				DetailsVisible: true,
			},
		},
	}
	router := NewRouter(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: userID},
		&fakeEvidenceManager{},
		&fakeAuditLogLister{},
		&fakeTruthDefinitionLister{},
		&fakeClaimRequestManager{},
		manager,
	)

	request := httptest.NewRequest(http.MethodGet, "/api/claims", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}
	if manager.userID != userID {
		t.Fatalf("user id = %s, want %s", manager.userID, userID)
	}

	body := response.Body.String()
	for _, forbidden := range []string{"raw_document", "file_path", "security_pin", "security_pin_hash", "truth_value"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response exposed forbidden field %q", forbidden)
		}
	}
}

func TestClaimByIDHandlerReturnsUserClaim(t *testing.T) {
	userID := uuid.New()
	claimID := uuid.New()
	manager := &fakeClaimManager{
		claim: claims.Claim{
			ID:             claimID,
			ClaimRequestID: uuid.New(),
			Organization:   claimrequests.Organization{ID: uuid.New(), Name: "Acme Bank", OrganizationType: "bank"},
			Purpose:        "Employment onboarding",
			ApprovedTruths: []string{"identity_verified"},
			Status:         claims.StatusActive,
			IssuedAt:       time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
			ExpiresAt:      time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC),
			DetailsVisible: true,
		},
	}
	router := NewRouter(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: userID},
		&fakeEvidenceManager{},
		&fakeAuditLogLister{},
		&fakeTruthDefinitionLister{},
		&fakeClaimRequestManager{},
		manager,
	)

	request := httptest.NewRequest(http.MethodGet, "/api/claims/"+claimID.String(), nil)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}
	if manager.getID != claimID {
		t.Fatalf("claim id = %s, want %s", manager.getID, claimID)
	}
}

func TestClaimsHandlerRequiresBearerToken(t *testing.T) {
	router := newTestRouter(nil, nil, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/claims", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestClaimByIDHandlerMapsNotFound(t *testing.T) {
	router := NewRouter(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: uuid.New()},
		&fakeEvidenceManager{},
		&fakeAuditLogLister{},
		&fakeTruthDefinitionLister{},
		&fakeClaimRequestManager{},
		&fakeClaimManager{err: claims.ErrClaimNotFound},
	)
	request := httptest.NewRequest(http.MethodGet, "/api/claims/"+uuid.New().String(), nil)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestClaimStatusHandlerReturnsVerificationStatusWithoutBearerToken(t *testing.T) {
	claimID := uuid.New()
	manager := &fakeClaimManager{
		claim: claims.Claim{
			ID:             claimID,
			ClaimRequestID: uuid.New(),
			Organization:   claimrequests.Organization{ID: uuid.New(), Name: "Acme Bank", OrganizationType: "bank"},
			Purpose:        "Account opening",
			ApprovedTruths: []string{"identity_verified"},
			Status:         claims.StatusActive,
			IssuedAt:       time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
			ExpiresAt:      time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC),
			DetailsVisible: true,
		},
	}
	router := NewRouter(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: uuid.New()},
		&fakeEvidenceManager{},
		&fakeAuditLogLister{},
		&fakeTruthDefinitionLister{},
		&fakeClaimRequestManager{},
		manager,
	)

	request := httptest.NewRequest(http.MethodGet, "/api/claims/"+claimID.String()+"/status", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}
	if manager.statusID != claimID {
		t.Fatalf("claim id = %s, want %s", manager.statusID, claimID)
	}

	body := response.Body.String()
	if !strings.Contains(body, "identity_verified") {
		t.Fatal("active verification response should include approved truths")
	}
	for _, forbidden := range []string{"raw_document", "file_path", "security_pin", "security_pin_hash", "truth_value"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response exposed forbidden field %q", forbidden)
		}
	}
}

func TestClaimStatusHandlerMapsNotFound(t *testing.T) {
	router := NewRouter(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: uuid.New()},
		&fakeEvidenceManager{},
		&fakeAuditLogLister{},
		&fakeTruthDefinitionLister{},
		&fakeClaimRequestManager{},
		&fakeClaimManager{err: claims.ErrClaimNotFound},
	)
	request := httptest.NewRequest(http.MethodGet, "/api/claims/"+uuid.New().String()+"/status", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestClaimStatusHandlerRequiresGet(t *testing.T) {
	router := newTestRouter(nil, nil, nil, &fakeAuthenticator{userID: uuid.New()})
	request := httptest.NewRequest(http.MethodPost, "/api/claims/"+uuid.New().String()+"/status", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func TestCreateClaimExchangePINHandlerCreatesTemporaryPIN(t *testing.T) {
	userID := uuid.New()
	claimID := uuid.New()
	manager := &fakeClaimManager{
		pin: claims.ExchangePIN{
			ClaimID:     claimID,
			ExchangePIN: "12345678",
			ExpiresAt:   time.Date(2026, 6, 1, 12, 15, 0, 0, time.UTC),
		},
	}
	router := NewRouter(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: userID},
		&fakeEvidenceManager{},
		&fakeAuditLogLister{},
		&fakeTruthDefinitionLister{},
		&fakeClaimRequestManager{},
		manager,
	)

	request := httptest.NewRequest(http.MethodPost, "/api/claims/"+claimID.String()+"/exchange-pin", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusCreated, response.Body.String())
	}
	if manager.userID != userID {
		t.Fatalf("user id = %s, want %s", manager.userID, userID)
	}
	if manager.pinClaimID != claimID {
		t.Fatalf("claim id = %s, want %s", manager.pinClaimID, claimID)
	}

	body := response.Body.String()
	if !strings.Contains(body, "12345678") {
		t.Fatal("response should include the temporary exchange pin once")
	}
	for _, forbidden := range []string{"pin_hash", "security_pin", "security_pin_hash", "raw_document", "file_path"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response exposed forbidden field %q", forbidden)
		}
	}
}

func TestCreateClaimExchangePINHandlerMapsInactiveClaim(t *testing.T) {
	router := NewRouter(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: uuid.New()},
		&fakeEvidenceManager{},
		&fakeAuditLogLister{},
		&fakeTruthDefinitionLister{},
		&fakeClaimRequestManager{},
		&fakeClaimManager{err: claims.ErrClaimNotActive},
	)
	request := httptest.NewRequest(http.MethodPost, "/api/claims/"+uuid.New().String()+"/exchange-pin", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusConflict)
	}
}

func TestResolveExchangePINHandlerReturnsVerificationStatusWithoutBearerToken(t *testing.T) {
	claimID := uuid.New()
	manager := &fakeClaimManager{
		claim: claims.Claim{
			ID:             claimID,
			ClaimRequestID: uuid.New(),
			Organization:   claimrequests.Organization{ID: uuid.New(), Name: "Acme Bank", OrganizationType: "bank"},
			Purpose:        "Account opening",
			ApprovedTruths: []string{"identity_verified"},
			Status:         claims.StatusActive,
			IssuedAt:       time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
			ExpiresAt:      time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC),
			DetailsVisible: true,
		},
	}
	router := NewRouter(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: uuid.New()},
		&fakeEvidenceManager{},
		&fakeAuditLogLister{},
		&fakeTruthDefinitionLister{},
		&fakeClaimRequestManager{},
		manager,
	)

	request := httptest.NewRequest(http.MethodPost, "/api/exchange-pins/resolve", strings.NewReader(`{"exchange_pin":"12345678"}`))
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}
	if manager.exchangePIN != "12345678" {
		t.Fatalf("exchange pin = %q, want %q", manager.exchangePIN, "12345678")
	}

	body := response.Body.String()
	if !strings.Contains(body, "identity_verified") {
		t.Fatal("active verification response should include approved truths")
	}
	for _, forbidden := range []string{"raw_document", "file_path", "security_pin", "security_pin_hash", "pin_hash", "truth_value"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response exposed forbidden field %q", forbidden)
		}
	}
}

func TestResolveExchangePINHandlerMapsInvalidPIN(t *testing.T) {
	router := NewRouter(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: uuid.New()},
		&fakeEvidenceManager{},
		&fakeAuditLogLister{},
		&fakeTruthDefinitionLister{},
		&fakeClaimRequestManager{},
		&fakeClaimManager{err: claims.ErrInvalidExchangePIN},
	)
	request := httptest.NewRequest(http.MethodPost, "/api/exchange-pins/resolve", strings.NewReader(`{"exchange_pin":"12ab"}`))
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestResolveExchangePINHandlerMapsMissingPIN(t *testing.T) {
	router := NewRouter(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: uuid.New()},
		&fakeEvidenceManager{},
		&fakeAuditLogLister{},
		&fakeTruthDefinitionLister{},
		&fakeClaimRequestManager{},
		&fakeClaimManager{err: claims.ErrExchangePINNotFound},
	)
	request := httptest.NewRequest(http.MethodPost, "/api/exchange-pins/resolve", strings.NewReader(`{"exchange_pin":"12345678"}`))
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestClaimRevokeHandlerRevokesClaim(t *testing.T) {
	userID := uuid.New()
	claimID := uuid.New()
	manager := &fakeClaimManager{
		claim: claims.Claim{
			ID:             claimID,
			ClaimRequestID: uuid.New(),
			Organization:   claimrequests.Organization{ID: uuid.New(), Name: "Acme Bank", OrganizationType: "bank"},
			Purpose:        "Employment onboarding",
			ApprovedTruths: []string{"identity_verified"},
			Status:         claims.StatusActive,
			IssuedAt:       time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
			ExpiresAt:      time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC),
			DetailsVisible: true,
		},
	}
	router := NewRouter(
		config.Config{},
		&fakeUserCreator{},
		&fakeUserGetter{},
		&fakeSecurityPINSetter{},
		&fakeSecurityPINResetter{},
		&fakeAuthenticator{userID: userID},
		&fakeEvidenceManager{},
		&fakeAuditLogLister{},
		&fakeTruthDefinitionLister{},
		&fakeClaimRequestManager{},
		manager,
	)

	request := httptest.NewRequest(http.MethodPost, "/api/claims/"+claimID.String()+"/revoke", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}
	if manager.userID != userID {
		t.Fatalf("user id = %s, want %s", manager.userID, userID)
	}
	if manager.revokeID != claimID {
		t.Fatalf("claim id = %s, want %s", manager.revokeID, claimID)
	}

	body := response.Body.String()
	if strings.Contains(body, "identity_verified") {
		t.Fatal("revoked claim response exposed proof details")
	}
}

func TestClaimRevokeHandlerMapsErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "not found", err: claims.ErrClaimNotFound, status: http.StatusNotFound},
		{name: "not active", err: claims.ErrClaimNotActive, status: http.StatusConflict},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := NewRouter(
				config.Config{},
				&fakeUserCreator{},
				&fakeUserGetter{},
				&fakeSecurityPINSetter{},
				&fakeSecurityPINResetter{},
				&fakeAuthenticator{userID: uuid.New()},
				&fakeEvidenceManager{},
				&fakeAuditLogLister{},
				&fakeTruthDefinitionLister{},
				&fakeClaimRequestManager{},
				&fakeClaimManager{err: test.err},
			)
			request := httptest.NewRequest(http.MethodPost, "/api/claims/"+uuid.New().String()+"/revoke", nil)
			request.Header.Set("Authorization", "Bearer test-token")
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
		})
	}
}

func TestClaimRevokeHandlerRequiresPost(t *testing.T) {
	router := newTestRouter(nil, nil, nil, &fakeAuthenticator{userID: uuid.New()})
	request := httptest.NewRequest(http.MethodGet, "/api/claims/"+uuid.New().String()+"/revoke", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func multipartEvidenceBody(t *testing.T, fields map[string]string, fileName string, fileContent string) (*bytes.Buffer, string) {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("write field: %v", err)
		}
	}

	fileWriter, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}

	if _, err := fileWriter.Write([]byte(fileContent)); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	return body, writer.FormDataContentType()
}
