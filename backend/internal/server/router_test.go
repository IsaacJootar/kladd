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

	"github.com/IsaacJootar/kladd/backend/internal/auth"
	"github.com/IsaacJootar/kladd/backend/internal/claimrequests"
	"github.com/IsaacJootar/kladd/backend/internal/config"
	"github.com/IsaacJootar/kladd/backend/internal/evidence"
	"github.com/IsaacJootar/kladd/backend/internal/securitypin"
	"github.com/IsaacJootar/kladd/backend/internal/truths"
	"github.com/IsaacJootar/kladd/backend/internal/users"
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
}

func (getter *fakeUserGetter) Get(ctx context.Context, id uuid.UUID) (users.User, error) {
	getter.userID = id
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
	err      error
	input    claimrequests.CreateInput
	userID   uuid.UUID
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

func (manager *fakeClaimRequestManager) GetForUser(ctx context.Context, userID uuid.UUID, requestID uuid.UUID) (claimrequests.ClaimRequest, error) {
	manager.userID = userID
	manager.getID = requestID
	if manager.err != nil {
		return claimrequests.ClaimRequest{}, manager.err
	}
	return manager.request, nil
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

	return NewRouter(config.Config{}, userCreator, userGetter, pinSetter, authenticator, evidenceManager, &fakeTruthDefinitionLister{}, &fakeClaimRequestManager{})
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
		&fakeAuthenticator{userID: uuid.New()},
		&fakeEvidenceManager{},
		lister,
		&fakeClaimRequestManager{},
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
		&fakeAuthenticator{},
		&fakeEvidenceManager{},
		&fakeTruthDefinitionLister{},
		&fakeClaimRequestManager{},
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
		&fakeAuthenticator{userID: uuid.New()},
		&fakeEvidenceManager{},
		&fakeTruthDefinitionLister{err: errors.New("boom")},
		&fakeClaimRequestManager{},
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
		&fakeAuthenticator{userID: uuid.New()},
		&fakeEvidenceManager{},
		&fakeTruthDefinitionLister{},
		&fakeClaimRequestManager{},
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
		&fakeAuthenticator{userID: userID},
		&fakeEvidenceManager{},
		&fakeTruthDefinitionLister{},
		manager,
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
		&fakeAuthenticator{userID: userID},
		&fakeEvidenceManager{},
		&fakeTruthDefinitionLister{},
		manager,
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
		&fakeAuthenticator{userID: userID},
		&fakeEvidenceManager{},
		&fakeTruthDefinitionLister{},
		manager,
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
				&fakeAuthenticator{userID: uuid.New()},
				&fakeEvidenceManager{},
				&fakeTruthDefinitionLister{},
				&fakeClaimRequestManager{err: test.err},
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
