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

	"github.com/IsaacJootar/kladd/backend/internal/config"
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
	router := NewRouter(config.Config{}, creator)

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
	router := NewRouter(config.Config{}, &fakeUserCreator{})
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
			router := NewRouter(config.Config{}, &fakeUserCreator{err: test.err})
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
	router := NewRouter(config.Config{}, &fakeUserCreator{})
	request := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}
