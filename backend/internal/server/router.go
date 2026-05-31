package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/IsaacJootar/kladd/backend/internal/config"
	"github.com/IsaacJootar/kladd/backend/internal/securitypin"
	"github.com/IsaacJootar/kladd/backend/internal/users"
	"github.com/google/uuid"
)

type UserCreator interface {
	Create(ctx context.Context, input users.CreateInput) (users.User, error)
}

type SecurityPINSetter interface {
	Setup(ctx context.Context, input securitypin.SetupInput) (securitypin.SetupResult, error)
}

func NewRouter(cfg config.Config, userCreator UserCreator, pinSetter SecurityPINSetter) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler(cfg))
	mux.HandleFunc("/api/users", createUserHandler(userCreator))
	mux.HandleFunc("/api/account/security-pin", setupSecurityPINHandler(pinSetter))

	return mux
}

func healthHandler(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		_ = json.NewEncoder(w).Encode(map[string]string{
			"service": "kladd-api",
			"status":  "ok",
			"addr":    cfg.HTTPAddr,
		})
	}
}

func createUserHandler(userCreator UserCreator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var request createUserRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON.")
			return
		}

		user, err := userCreator.Create(r.Context(), users.CreateInput{
			Name:        request.Name,
			Email:       request.Email,
			Phone:       request.Phone,
			Password:    request.Password,
			AccountType: request.AccountType,
		})
		if err != nil {
			writeCreateUserError(w, err)
			return
		}

		writeJSON(w, http.StatusCreated, user)
	}
}

type createUserRequest struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	Password    string `json:"password"`
	AccountType string `json:"account_type"`
}

func setupSecurityPINHandler(pinSetter SecurityPINSetter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var request setupSecurityPINRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON.")
			return
		}

		userID, err := uuid.Parse(request.UserID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_user_id", "A valid user_id is required.")
			return
		}

		result, err := pinSetter.Setup(r.Context(), securitypin.SetupInput{
			UserID: userID,
			PIN:    request.SecurityPIN,
		})
		if err != nil {
			writeSetupSecurityPINError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}

type setupSecurityPINRequest struct {
	UserID      string `json:"user_id"`
	SecurityPIN string `json:"security_pin"`
}

func writeCreateUserError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, users.ErrInvalidName):
		writeError(w, http.StatusBadRequest, "invalid_name", "Name is required.")
	case errors.Is(err, users.ErrInvalidEmail):
		writeError(w, http.StatusBadRequest, "invalid_email", "A valid email is required.")
	case errors.Is(err, users.ErrInvalidPassword):
		writeError(w, http.StatusBadRequest, "invalid_password", "Password must be at least 8 characters.")
	case errors.Is(err, users.ErrInvalidAccountType):
		writeError(w, http.StatusBadRequest, "invalid_account_type", "Account type must be individual or business.")
	case errors.Is(err, users.ErrEmailTaken):
		writeError(w, http.StatusConflict, "email_taken", "Email is already registered.")
	default:
		writeError(w, http.StatusInternalServerError, "server_error", "Unable to create user.")
	}
}

func writeSetupSecurityPINError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, securitypin.ErrInvalidFormat):
		writeError(w, http.StatusBadRequest, "invalid_security_pin", "Security PIN must be 4-6 digits.")
	case errors.Is(err, securitypin.ErrUserNotFound):
		writeError(w, http.StatusNotFound, "user_not_found", "User was not found.")
	default:
		writeError(w, http.StatusInternalServerError, "server_error", "Unable to set Security PIN.")
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{
		"error":   code,
		"message": message,
	})
}
