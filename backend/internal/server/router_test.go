package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/IsaacJootar/kladd/backend/internal/auth"
	"github.com/IsaacJootar/kladd/backend/internal/config"
	"github.com/IsaacJootar/kladd/backend/internal/securitypin"
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

func newTestRouter(userCreator *fakeUserCreator, pinSetter *fakeSecurityPINSetter, authenticator *fakeAuthenticator) http.Handler {
	if userCreator == nil {
		userCreator = &fakeUserCreator{}
	}
	if pinSetter == nil {
		pinSetter = &fakeSecurityPINSetter{}
	}
	if authenticator == nil {
		authenticator = &fakeAuthenticator{userID: uuid.New()}
	}

	return NewRouter(config.Config{}, userCreator, pinSetter, authenticator)
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
	router := newTestRouter(creator, nil, nil)

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
	router := newTestRouter(nil, nil, nil)
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
			router := newTestRouter(&fakeUserCreator{err: test.err}, nil, nil)
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
	router := newTestRouter(nil, nil, nil)
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
	router := newTestRouter(nil, nil, authenticator)

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
	router := newTestRouter(nil, nil, nil)
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
			router := newTestRouter(nil, nil, &fakeAuthenticator{loginErr: test.err})
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
	router := newTestRouter(nil, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/auth/login", nil)
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
	router := newTestRouter(nil, setter, authenticator)

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
	router := newTestRouter(nil, nil, &fakeAuthenticator{userID: uuid.New()})
	request := httptest.NewRequest(http.MethodPost, "/api/account/security-pin", bytes.NewBufferString(`{`))
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestSetupSecurityPINHandlerRequiresBearerToken(t *testing.T) {
	router := newTestRouter(nil, nil, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/account/security-pin", strings.NewReader(`{"security_pin":"4829"}`))
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestSetupSecurityPINHandlerRejectsInvalidToken(t *testing.T) {
	router := newTestRouter(nil, nil, &fakeAuthenticator{authErr: auth.ErrInvalidToken})
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
			router := newTestRouter(nil, &fakeSecurityPINSetter{err: test.err}, &fakeAuthenticator{userID: uuid.New()})
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
	router := newTestRouter(nil, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/account/security-pin", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}
