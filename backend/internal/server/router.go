package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/IsaacJootar/kladd/backend/internal/auth"
	"github.com/IsaacJootar/kladd/backend/internal/config"
	"github.com/IsaacJootar/kladd/backend/internal/evidence"
	"github.com/IsaacJootar/kladd/backend/internal/securitypin"
	"github.com/IsaacJootar/kladd/backend/internal/users"
	"github.com/google/uuid"
)

const maxEvidenceUploadBytes = 10 << 20

type UserCreator interface {
	Create(ctx context.Context, input users.CreateInput) (users.User, error)
}

type UserGetter interface {
	Get(ctx context.Context, id uuid.UUID) (users.User, error)
}

type SecurityPINSetter interface {
	Setup(ctx context.Context, input securitypin.SetupInput) (securitypin.SetupResult, error)
}

type Authenticator interface {
	Login(ctx context.Context, input auth.LoginInput) (auth.LoginResult, error)
	Authenticate(tokenString string) (uuid.UUID, error)
}

type EvidenceManager interface {
	Create(ctx context.Context, input evidence.CreateInput) (evidence.EvidenceItem, error)
	List(ctx context.Context, userID uuid.UUID) ([]evidence.EvidenceItem, error)
}

func NewRouter(cfg config.Config, userCreator UserCreator, userGetter UserGetter, pinSetter SecurityPINSetter, authenticator Authenticator, evidenceManager EvidenceManager) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler(cfg))
	mux.HandleFunc("/api/users", createUserHandler(userCreator))
	mux.HandleFunc("/api/auth/login", loginHandler(authenticator))
	mux.HandleFunc("/api/account/me", currentAccountHandler(userGetter, authenticator))
	mux.HandleFunc("/api/account/security-pin", setupSecurityPINHandler(pinSetter, authenticator))
	mux.HandleFunc("/api/evidence-items", evidenceItemsHandler(evidenceManager, authenticator))

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

func loginHandler(authenticator Authenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var request loginRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON.")
			return
		}

		result, err := authenticator.Login(r.Context(), auth.LoginInput{
			Email:    request.Email,
			Password: request.Password,
		})
		if err != nil {
			writeLoginError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func currentAccountHandler(userGetter UserGetter, authenticator Authenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		userID, ok := authenticateRequest(w, r, authenticator)
		if !ok {
			return
		}

		user, err := userGetter.Get(r.Context(), userID)
		if err != nil {
			writeCurrentAccountError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, user)
	}
}

func setupSecurityPINHandler(pinSetter SecurityPINSetter, authenticator Authenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		userID, ok := authenticateRequest(w, r, authenticator)
		if !ok {
			return
		}

		var request setupSecurityPINRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON.")
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
	SecurityPIN string `json:"security_pin"`
}

func evidenceItemsHandler(evidenceManager EvidenceManager, authenticator Authenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := authenticateRequest(w, r, authenticator)
		if !ok {
			return
		}

		switch r.Method {
		case http.MethodGet:
			items, err := evidenceManager.List(r.Context(), userID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "server_error", "Unable to load evidence records.")
				return
			}

			writeJSON(w, http.StatusOK, map[string][]evidence.EvidenceItem{
				"items": items,
			})
		case http.MethodPost:
			createEvidenceItem(w, r, userID, evidenceManager)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

func createEvidenceItem(w http.ResponseWriter, r *http.Request, userID uuid.UUID, evidenceManager EvidenceManager) {
	r.Body = http.MaxBytesReader(w, r.Body, maxEvidenceUploadBytes)
	if err := r.ParseMultipartForm(maxEvidenceUploadBytes); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_upload", "Evidence upload must include a valid file.")
		return
	}

	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing_file", "Evidence file is required.")
		return
	}
	defer file.Close()

	item, err := evidenceManager.Create(r.Context(), evidence.CreateInput{
		UserID:      userID,
		Category:    r.FormValue("category"),
		DisplayName: r.FormValue("display_name"),
		FileName:    fileHeader.Filename,
		ContentType: fileHeader.Header.Get("Content-Type"),
		SizeBytes:   fileHeader.Size,
		Reader:      file,
	})
	if err != nil {
		writeCreateEvidenceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, item)
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

func writeLoginError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "Email or password is incorrect.")
	default:
		writeError(w, http.StatusInternalServerError, "server_error", "Unable to log in.")
	}
}

func writeCurrentAccountError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, users.ErrUserNotFound):
		writeError(w, http.StatusNotFound, "user_not_found", "User was not found.")
	default:
		writeError(w, http.StatusInternalServerError, "server_error", "Unable to load current account.")
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

func writeCreateEvidenceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, evidence.ErrInvalidCategory):
		writeError(w, http.StatusBadRequest, "invalid_category", "Evidence category is required.")
	case errors.Is(err, evidence.ErrInvalidFile):
		writeError(w, http.StatusBadRequest, "invalid_file", "Evidence file is required.")
	default:
		writeError(w, http.StatusInternalServerError, "server_error", "Unable to save evidence record.")
	}
}

func authenticateRequest(w http.ResponseWriter, r *http.Request, authenticator Authenticator) (uuid.UUID, bool) {
	tokenString, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Bearer access token is required.")
		return uuid.Nil, false
	}

	userID, err := authenticator.Authenticate(tokenString)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_token", "Bearer access token is invalid or expired.")
		return uuid.Nil, false
	}

	return userID, true
}

func bearerToken(header string) (string, bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}

	token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	return token, token != ""
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
