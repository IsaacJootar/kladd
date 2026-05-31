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

func newTestRouter(userCreator *fakeUserCreator, pinSetter *fakeSecurityPINSetter) http.Handler {
	if userCreator == nil {
		userCreator = &fakeUserCreator{}
	}
	if pinSetter == nil {
		pinSetter = &fakeSecurityPINSetter{}
	}

	return NewRouter(config.Config{}, userCreator, pinSetter)
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
	router := newTestRouter(creator, nil)

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
	router := newTestRouter(nil, nil)
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
			router := newTestRouter(&fakeUserCreator{err: test.err}, nil)
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
	router := newTestRouter(nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/users", nil)
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
	router := newTestRouter(nil, setter)

	requestBody := `{"user_id":"` + userID.String() + `","security_pin":"4829"}`
	request := httptest.NewRequest(http.MethodPost, "/api/account/security-pin", strings.NewReader(requestBody))
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
	router := newTestRouter(nil, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/account/security-pin", bytes.NewBufferString(`{`))
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestSetupSecurityPINHandlerRejectsInvalidUserID(t *testing.T) {
	router := newTestRouter(nil, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/account/security-pin", strings.NewReader(`{"user_id":"bad","security_pin":"4829"}`))
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
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
			router := newTestRouter(nil, &fakeSecurityPINSetter{err: test.err})
			request := httptest.NewRequest(http.MethodPost, "/api/account/security-pin", strings.NewReader(`{"user_id":"`+uuid.New().String()+`","security_pin":"4829"}`))
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
		})
	}
}

func TestSetupSecurityPINHandlerRequiresPost(t *testing.T) {
	router := newTestRouter(nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/account/security-pin", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}
