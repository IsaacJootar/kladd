package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

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

const maxEvidenceUploadBytes = 10 << 20

type UserCreator interface {
	Create(ctx context.Context, input users.CreateInput) (users.User, error)
}

type UserGetter interface {
	Get(ctx context.Context, id uuid.UUID) (users.User, error)
	GetByEmail(ctx context.Context, email string) (users.User, error)
}

type SecurityPINSetter interface {
	Setup(ctx context.Context, input securitypin.SetupInput) (securitypin.SetupResult, error)
}

type SecurityPINResetter interface {
	Reset(ctx context.Context, input securitypin.ResetInput) (securitypin.SetupResult, error)
}

type Authenticator interface {
	Login(ctx context.Context, input auth.LoginInput) (auth.LoginResult, error)
	Authenticate(tokenString string) (uuid.UUID, error)
}

type EvidenceManager interface {
	Create(ctx context.Context, input evidence.CreateInput) (evidence.EvidenceItem, error)
	List(ctx context.Context, userID uuid.UUID) ([]evidence.EvidenceItem, error)
}

type AuditLogLister interface {
	ListForUser(ctx context.Context, userID uuid.UUID) ([]audit.Event, error)
	ListForOrganization(ctx context.Context, organizationID uuid.UUID) ([]audit.Event, error)
}

type TruthDefinitionLister interface {
	ListDefinitions(ctx context.Context) ([]truths.Definition, error)
}

type ClaimRequestManager interface {
	Create(ctx context.Context, input claimrequests.CreateInput) (claimrequests.ClaimRequest, error)
	ListForUser(ctx context.Context, userID uuid.UUID) ([]claimrequests.ClaimRequest, error)
	ListForOrganization(ctx context.Context, organizationID uuid.UUID) ([]claimrequests.ClaimRequest, error)
	GetForUser(ctx context.Context, userID uuid.UUID, requestID uuid.UUID) (claimrequests.ClaimRequest, error)
	Approve(ctx context.Context, input claimrequests.ApproveInput) (claimrequests.ApprovalResult, error)
	Deny(ctx context.Context, input claimrequests.DenyInput) (claimrequests.ClaimRequest, error)
}

type ClaimManager interface {
	ListForUser(ctx context.Context, userID uuid.UUID) ([]claims.Claim, error)
	ListForOrganization(ctx context.Context, organizationID uuid.UUID) ([]claims.Claim, error)
	GetForUser(ctx context.Context, userID uuid.UUID, claimID uuid.UUID) (claims.Claim, error)
	GetStatus(ctx context.Context, claimID uuid.UUID) (claims.Claim, error)
	Revoke(ctx context.Context, userID uuid.UUID, claimID uuid.UUID) (claims.Claim, error)
	CreateExchangePIN(ctx context.Context, userID uuid.UUID, claimID uuid.UUID) (claims.ExchangePIN, error)
	ResolveExchangePIN(ctx context.Context, exchangePIN string) (claims.Claim, error)
}

type OrganizationAuthenticator interface {
	Authenticate(ctx context.Context, apiKey string) (claimrequests.Organization, error)
}

type OrganizationAccountManager interface {
	RegisterAccount(ctx context.Context, input orgauth.RegisterInput) (orgauth.Account, error)
	Login(ctx context.Context, input orgauth.LoginInput) (orgauth.LoginResult, error)
}

type OrganizationWebhookEndpointManager interface {
	ConfigureEndpoint(ctx context.Context, input webhooks.ConfigureEndpointInput) (webhooks.Endpoint, error)
	GetEndpointForOrganization(ctx context.Context, organizationID uuid.UUID) (webhooks.Endpoint, error)
	ListDeliveriesForOrganization(ctx context.Context, organizationID uuid.UUID) ([]webhooks.DeliveryLog, error)
}

func NewRouter(cfg config.Config, userCreator UserCreator, userGetter UserGetter, pinSetter SecurityPINSetter, pinResetter SecurityPINResetter, authenticator Authenticator, evidenceManager EvidenceManager, auditLogLister AuditLogLister, truthDefinitionLister TruthDefinitionLister, claimRequestManager ClaimRequestManager, claimManager ClaimManager) http.Handler {
	return buildRouter(cfg, userCreator, userGetter, pinSetter, pinResetter, authenticator, evidenceManager, auditLogLister, truthDefinitionLister, claimRequestManager, claimManager, nil, nil)
}

func NewRouterWithOrganizationAPI(cfg config.Config, userCreator UserCreator, userGetter UserGetter, pinSetter SecurityPINSetter, pinResetter SecurityPINResetter, authenticator Authenticator, evidenceManager EvidenceManager, auditLogLister AuditLogLister, truthDefinitionLister TruthDefinitionLister, claimRequestManager ClaimRequestManager, claimManager ClaimManager, organizationAuthenticator OrganizationAuthenticator, webhookEndpointManagers ...OrganizationWebhookEndpointManager) http.Handler {
	var webhookEndpointManager OrganizationWebhookEndpointManager
	if len(webhookEndpointManagers) > 0 {
		webhookEndpointManager = webhookEndpointManagers[0]
	}

	return buildRouter(cfg, userCreator, userGetter, pinSetter, pinResetter, authenticator, evidenceManager, auditLogLister, truthDefinitionLister, claimRequestManager, claimManager, organizationAuthenticator, webhookEndpointManager)
}

func buildRouter(cfg config.Config, userCreator UserCreator, userGetter UserGetter, pinSetter SecurityPINSetter, pinResetter SecurityPINResetter, authenticator Authenticator, evidenceManager EvidenceManager, auditLogLister AuditLogLister, truthDefinitionLister TruthDefinitionLister, claimRequestManager ClaimRequestManager, claimManager ClaimManager, organizationAuthenticator OrganizationAuthenticator, webhookEndpointManager OrganizationWebhookEndpointManager) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler(cfg))
	mux.HandleFunc("/api/users", createUserHandler(userCreator))
	mux.HandleFunc("/api/auth/login", loginHandler(authenticator))
	mux.HandleFunc("/api/account/me", currentAccountHandler(userGetter, authenticator))
	mux.HandleFunc("/api/account/security-pin", setupSecurityPINHandler(pinSetter, authenticator))
	mux.HandleFunc("/api/account/security-pin/reset", resetSecurityPINHandler(pinResetter, authenticator))
	mux.HandleFunc("/api/evidence-items", evidenceItemsHandler(evidenceManager, authenticator))
	mux.HandleFunc("/api/audit-logs", auditLogsHandler(auditLogLister, authenticator))
	mux.HandleFunc("/api/truth-definitions", truthDefinitionsHandler(truthDefinitionLister, authenticator))
	mux.HandleFunc("/api/claim-requests", claimRequestsHandler(claimRequestManager, authenticator))
	mux.HandleFunc("/api/claim-requests/", claimRequestByIDHandler(claimRequestManager, authenticator))
	if organizationAuthenticator != nil {
		if organizationAccountManager, ok := organizationAuthenticator.(OrganizationAccountManager); ok {
			mux.HandleFunc("/api/organizations", createOrganizationHandler(organizationAccountManager))
			mux.HandleFunc("/api/organization/auth/login", organizationLoginHandler(organizationAccountManager))
		}
		mux.HandleFunc("/api/organization/me", organizationProfileHandler(organizationAuthenticator))
		mux.HandleFunc("/api/organization/audit-logs", organizationAuditLogsHandler(auditLogLister, organizationAuthenticator))
		mux.HandleFunc("/api/organization/claim-requests", organizationClaimRequestsHandler(claimRequestManager, userGetter, organizationAuthenticator))
		mux.HandleFunc("/api/organization/claims", organizationClaimsHandler(claimManager, organizationAuthenticator))
		if webhookEndpointManager != nil {
			mux.HandleFunc("/api/organization/webhook-endpoint", organizationWebhookEndpointHandler(webhookEndpointManager, organizationAuthenticator))
			mux.HandleFunc("/api/organization/webhook-deliveries", organizationWebhookDeliveriesHandler(webhookEndpointManager, organizationAuthenticator))
		}
	}
	mux.HandleFunc("/api/exchange-pins/resolve", resolveExchangePINHandler(claimManager))
	mux.HandleFunc("/api/claims", claimsHandler(claimManager, authenticator))
	mux.HandleFunc("/api/claims/", claimByIDHandler(claimManager, authenticator))

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

type createOrganizationRequest struct {
	Name             string `json:"name"`
	Email            string `json:"email"`
	Password         string `json:"password"`
	OrganizationType string `json:"organization_type"`
}

type organizationLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func createOrganizationHandler(organizationAccountManager OrganizationAccountManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var request createOrganizationRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON.")
			return
		}

		account, err := organizationAccountManager.RegisterAccount(r.Context(), orgauth.RegisterInput{
			Name:             request.Name,
			Email:            request.Email,
			Password:         request.Password,
			OrganizationType: request.OrganizationType,
		})
		if err != nil {
			writeCreateOrganizationError(w, err)
			return
		}

		writeJSON(w, http.StatusCreated, account)
	}
}

func organizationLoginHandler(organizationAccountManager OrganizationAccountManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var request organizationLoginRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON.")
			return
		}

		result, err := organizationAccountManager.Login(r.Context(), orgauth.LoginInput{
			Email:    request.Email,
			Password: request.Password,
		})
		if err != nil {
			writeOrganizationLoginError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
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

func resetSecurityPINHandler(pinResetter SecurityPINResetter, authenticator Authenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		userID, ok := authenticateRequest(w, r, authenticator)
		if !ok {
			return
		}

		var request resetSecurityPINRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON.")
			return
		}

		result, err := pinResetter.Reset(r.Context(), securitypin.ResetInput{
			UserID:   userID,
			Password: request.Password,
			PIN:      request.SecurityPIN,
		})
		if err != nil {
			writeResetSecurityPINError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}

type setupSecurityPINRequest struct {
	SecurityPIN string `json:"security_pin"`
}

type resetSecurityPINRequest struct {
	Password    string `json:"password"`
	SecurityPIN string `json:"security_pin"`
}

type createClaimRequestRequest struct {
	OrganizationName string   `json:"organization_name"`
	OrganizationType string   `json:"organization_type"`
	Purpose          string   `json:"purpose"`
	RequestedTruths  []string `json:"requested_truths"`
	DurationDays     int      `json:"duration_days"`
}

type createOrganizationClaimRequestRequest struct {
	UserEmail       string   `json:"user_email"`
	Purpose         string   `json:"purpose"`
	RequestedTruths []string `json:"requested_truths"`
	DurationDays    int      `json:"duration_days"`
}

type approveClaimRequestRequest struct {
	SecurityPIN string `json:"security_pin"`
}

type resolveExchangePINRequest struct {
	ExchangePIN string `json:"exchange_pin"`
}

type configureOrganizationWebhookEndpointRequest struct {
	URL string `json:"url"`
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

func auditLogsHandler(auditLogLister AuditLogLister, authenticator Authenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		userID, ok := authenticateRequest(w, r, authenticator)
		if !ok {
			return
		}

		events, err := auditLogLister.ListForUser(r.Context(), userID)
		if err != nil {
			writeListAuditLogsError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string][]audit.Event{
			"items": events,
		})
	}
}

func organizationAuditLogsHandler(auditLogLister AuditLogLister, organizationAuthenticator OrganizationAuthenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		organization, ok := authenticateOrganizationRequest(w, r, organizationAuthenticator)
		if !ok {
			return
		}

		events, err := auditLogLister.ListForOrganization(r.Context(), organization.ID)
		if err != nil {
			writeListOrganizationAuditLogsError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string][]audit.Event{
			"items": events,
		})
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

func truthDefinitionsHandler(truthDefinitionLister TruthDefinitionLister, authenticator Authenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if _, ok := authenticateRequest(w, r, authenticator); !ok {
			return
		}

		definitions, err := truthDefinitionLister.ListDefinitions(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "server_error", "Unable to load truth definitions.")
			return
		}

		writeJSON(w, http.StatusOK, map[string][]truths.Definition{
			"items": definitions,
		})
	}
}

func claimRequestsHandler(claimRequestManager ClaimRequestManager, authenticator Authenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := authenticateRequest(w, r, authenticator)
		if !ok {
			return
		}

		switch r.Method {
		case http.MethodGet:
			requests, err := claimRequestManager.ListForUser(r.Context(), userID)
			if err != nil {
				writeListClaimRequestsError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, map[string][]claimrequests.ClaimRequest{
				"items": requests,
			})
		case http.MethodPost:
			var request createClaimRequestRequest
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&request); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON.")
				return
			}

			claimRequest, err := claimRequestManager.Create(r.Context(), claimrequests.CreateInput{
				UserID:           userID,
				OrganizationName: request.OrganizationName,
				OrganizationType: request.OrganizationType,
				Purpose:          request.Purpose,
				RequestedTruths:  request.RequestedTruths,
				DurationDays:     request.DurationDays,
			})
			if err != nil {
				writeCreateClaimRequestError(w, err)
				return
			}

			writeJSON(w, http.StatusCreated, claimRequest)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

func organizationClaimRequestsHandler(claimRequestManager ClaimRequestManager, userGetter UserGetter, organizationAuthenticator OrganizationAuthenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organization, ok := authenticateOrganizationRequest(w, r, organizationAuthenticator)
		if !ok {
			return
		}

		if r.Method == http.MethodGet {
			requests, err := claimRequestManager.ListForOrganization(r.Context(), organization.ID)
			if err != nil {
				writeListOrganizationClaimRequestsError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, map[string][]claimrequests.ClaimRequest{
				"items": requests,
			})
			return
		}

		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var request createOrganizationClaimRequestRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON.")
			return
		}

		user, err := userGetter.GetByEmail(r.Context(), request.UserEmail)
		if err != nil {
			writeOrganizationClaimRequestUserError(w, err)
			return
		}

		claimRequest, err := claimRequestManager.Create(r.Context(), claimrequests.CreateInput{
			UserID:           user.ID,
			OrganizationName: organization.Name,
			OrganizationType: organization.OrganizationType,
			Purpose:          request.Purpose,
			RequestedTruths:  request.RequestedTruths,
			DurationDays:     request.DurationDays,
		})
		if err != nil {
			writeCreateClaimRequestError(w, err)
			return
		}

		writeJSON(w, http.StatusCreated, claimRequest)
	}
}

func organizationProfileHandler(organizationAuthenticator OrganizationAuthenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		organization, ok := authenticateOrganizationRequest(w, r, organizationAuthenticator)
		if !ok {
			return
		}

		writeJSON(w, http.StatusOK, organization)
	}
}

func organizationClaimsHandler(claimManager ClaimManager, organizationAuthenticator OrganizationAuthenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		organization, ok := authenticateOrganizationRequest(w, r, organizationAuthenticator)
		if !ok {
			return
		}

		claimList, err := claimManager.ListForOrganization(r.Context(), organization.ID)
		if err != nil {
			writeListOrganizationClaimsError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string][]claims.Claim{
			"items": claimList,
		})
	}
}

func organizationWebhookEndpointHandler(webhookEndpointManager OrganizationWebhookEndpointManager, organizationAuthenticator OrganizationAuthenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organization, ok := authenticateOrganizationRequest(w, r, organizationAuthenticator)
		if !ok {
			return
		}

		if r.Method == http.MethodGet {
			endpoint, err := webhookEndpointManager.GetEndpointForOrganization(r.Context(), organization.ID)
			if err != nil {
				writeGetOrganizationWebhookEndpointError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, endpoint)
			return
		}

		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var request configureOrganizationWebhookEndpointRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON.")
			return
		}

		endpoint, err := webhookEndpointManager.ConfigureEndpoint(r.Context(), webhooks.ConfigureEndpointInput{
			OrganizationName: organization.Name,
			OrganizationType: organization.OrganizationType,
			URL:              request.URL,
		})
		if err != nil {
			writeConfigureOrganizationWebhookEndpointError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, endpoint)
	}
}

func organizationWebhookDeliveriesHandler(webhookEndpointManager OrganizationWebhookEndpointManager, organizationAuthenticator OrganizationAuthenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		organization, ok := authenticateOrganizationRequest(w, r, organizationAuthenticator)
		if !ok {
			return
		}

		deliveries, err := webhookEndpointManager.ListDeliveriesForOrganization(r.Context(), organization.ID)
		if err != nil {
			writeListOrganizationWebhookDeliveriesError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string][]webhooks.DeliveryLog{
			"items": deliveries,
		})
	}
}

func claimRequestByIDHandler(claimRequestManager ClaimRequestManager, authenticator Authenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/claim-requests/")
		if strings.HasSuffix(path, "/approve") {
			approveClaimRequest(w, r, strings.TrimSuffix(path, "/approve"), claimRequestManager, authenticator)
			return
		}
		if strings.HasSuffix(path, "/deny") {
			denyClaimRequest(w, r, strings.TrimSuffix(path, "/deny"), claimRequestManager, authenticator)
			return
		}

		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		userID, ok := authenticateRequest(w, r, authenticator)
		if !ok {
			return
		}

		requestID, err := uuid.Parse(path)
		if err != nil {
			writeError(w, http.StatusNotFound, "claim_request_not_found", "Claim request was not found.")
			return
		}

		claimRequest, err := claimRequestManager.GetForUser(r.Context(), userID, requestID)
		if err != nil {
			writeGetClaimRequestError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, claimRequest)
	}
}

func approveClaimRequest(w http.ResponseWriter, r *http.Request, requestIDValue string, claimRequestManager ClaimRequestManager, authenticator Authenticator) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	userID, ok := authenticateRequest(w, r, authenticator)
	if !ok {
		return
	}

	requestID, err := uuid.Parse(requestIDValue)
	if err != nil {
		writeError(w, http.StatusNotFound, "claim_request_not_found", "Claim request was not found.")
		return
	}

	var request approveClaimRequestRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON.")
		return
	}

	result, err := claimRequestManager.Approve(r.Context(), claimrequests.ApproveInput{
		UserID:      userID,
		RequestID:   requestID,
		SecurityPIN: request.SecurityPIN,
		IPAddress:   readRequestIP(r),
		UserAgent:   r.UserAgent(),
	})
	if err != nil {
		writeApproveClaimRequestError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func denyClaimRequest(w http.ResponseWriter, r *http.Request, requestIDValue string, claimRequestManager ClaimRequestManager, authenticator Authenticator) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	userID, ok := authenticateRequest(w, r, authenticator)
	if !ok {
		return
	}

	requestID, err := uuid.Parse(requestIDValue)
	if err != nil {
		writeError(w, http.StatusNotFound, "claim_request_not_found", "Claim request was not found.")
		return
	}

	claimRequest, err := claimRequestManager.Deny(r.Context(), claimrequests.DenyInput{
		UserID:    userID,
		RequestID: requestID,
	})
	if err != nil {
		writeDenyClaimRequestError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, claimRequest)
}

func claimsHandler(claimManager ClaimManager, authenticator Authenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		userID, ok := authenticateRequest(w, r, authenticator)
		if !ok {
			return
		}

		claimList, err := claimManager.ListForUser(r.Context(), userID)
		if err != nil {
			writeListClaimsError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string][]claims.Claim{
			"items": claimList,
		})
	}
}

func claimByIDHandler(claimManager ClaimManager, authenticator Authenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/claims/")
		if strings.HasSuffix(path, "/status") {
			claimStatus(w, r, strings.TrimSuffix(path, "/status"), claimManager)
			return
		}
		if strings.HasSuffix(path, "/revoke") {
			revokeClaim(w, r, strings.TrimSuffix(path, "/revoke"), claimManager, authenticator)
			return
		}
		if strings.HasSuffix(path, "/exchange-pin") {
			createClaimExchangePIN(w, r, strings.TrimSuffix(path, "/exchange-pin"), claimManager, authenticator)
			return
		}

		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		userID, ok := authenticateRequest(w, r, authenticator)
		if !ok {
			return
		}

		claimID, err := uuid.Parse(path)
		if err != nil {
			writeError(w, http.StatusNotFound, "claim_not_found", "Claim was not found.")
			return
		}

		claim, err := claimManager.GetForUser(r.Context(), userID, claimID)
		if err != nil {
			writeGetClaimError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, claim)
	}
}

func claimStatus(w http.ResponseWriter, r *http.Request, claimIDValue string, claimManager ClaimManager) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	claimID, err := uuid.Parse(claimIDValue)
	if err != nil {
		writeError(w, http.StatusNotFound, "claim_not_found", "Claim was not found.")
		return
	}

	claim, err := claimManager.GetStatus(r.Context(), claimID)
	if err != nil {
		writeGetClaimError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, claim)
}

func createClaimExchangePIN(w http.ResponseWriter, r *http.Request, claimIDValue string, claimManager ClaimManager, authenticator Authenticator) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	userID, ok := authenticateRequest(w, r, authenticator)
	if !ok {
		return
	}

	claimID, err := uuid.Parse(claimIDValue)
	if err != nil {
		writeError(w, http.StatusNotFound, "claim_not_found", "Claim was not found.")
		return
	}

	exchangePIN, err := claimManager.CreateExchangePIN(r.Context(), userID, claimID)
	if err != nil {
		writeCreateExchangePINError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, exchangePIN)
}

func resolveExchangePINHandler(claimManager ClaimManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var request resolveExchangePINRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON.")
			return
		}

		claim, err := claimManager.ResolveExchangePIN(r.Context(), request.ExchangePIN)
		if err != nil {
			writeResolveExchangePINError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, claim)
	}
}

func revokeClaim(w http.ResponseWriter, r *http.Request, claimIDValue string, claimManager ClaimManager, authenticator Authenticator) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	userID, ok := authenticateRequest(w, r, authenticator)
	if !ok {
		return
	}

	claimID, err := uuid.Parse(claimIDValue)
	if err != nil {
		writeError(w, http.StatusNotFound, "claim_not_found", "Claim was not found.")
		return
	}

	claim, err := claimManager.Revoke(r.Context(), userID, claimID)
	if err != nil {
		writeRevokeClaimError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, claim)
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

func writeCreateOrganizationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, orgauth.ErrInvalidOrganization):
		writeError(w, http.StatusBadRequest, "invalid_organization", "Organization name is required.")
	case errors.Is(err, orgauth.ErrInvalidEmail):
		writeError(w, http.StatusBadRequest, "invalid_email", "A valid organization email is required.")
	case errors.Is(err, orgauth.ErrInvalidPassword):
		writeError(w, http.StatusBadRequest, "invalid_password", "Password must be at least 8 characters.")
	case errors.Is(err, orgauth.ErrEmailTaken):
		writeError(w, http.StatusConflict, "organization_email_taken", "Organization email is already registered.")
	default:
		writeError(w, http.StatusInternalServerError, "server_error", "Unable to create organization.")
	}
}

func writeOrganizationLoginError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, orgauth.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "Organization email or password is incorrect.")
	default:
		writeError(w, http.StatusInternalServerError, "server_error", "Unable to log in organization.")
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

func writeResetSecurityPINError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, securitypin.ErrInvalidFormat):
		writeError(w, http.StatusBadRequest, "invalid_security_pin", "Security PIN must be 4-6 digits.")
	case errors.Is(err, securitypin.ErrInvalidPassword):
		writeError(w, http.StatusUnauthorized, "invalid_password", "Account password is incorrect.")
	case errors.Is(err, securitypin.ErrInvalidUser):
		writeError(w, http.StatusUnauthorized, "unauthorized", "Bearer access token is required.")
	case errors.Is(err, securitypin.ErrUserNotFound):
		writeError(w, http.StatusNotFound, "user_not_found", "User was not found.")
	default:
		writeError(w, http.StatusInternalServerError, "server_error", "Unable to reset Security PIN.")
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

func writeListAuditLogsError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, audit.ErrInvalidUser):
		writeError(w, http.StatusUnauthorized, "unauthorized", "Bearer access token is required.")
	default:
		writeError(w, http.StatusInternalServerError, "server_error", "Unable to load access history.")
	}
}

func writeListOrganizationAuditLogsError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, audit.ErrInvalidOrganizationID):
		writeError(w, http.StatusUnauthorized, "organization_api_key_required", "Organization API key is required.")
	default:
		writeError(w, http.StatusInternalServerError, "server_error", "Unable to load organization access history.")
	}
}

func writeCreateClaimRequestError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, claimrequests.ErrInvalidOrganization):
		writeError(w, http.StatusBadRequest, "invalid_organization", "Organization name is required.")
	case errors.Is(err, claimrequests.ErrInvalidPurpose):
		writeError(w, http.StatusBadRequest, "invalid_purpose", "Purpose is required.")
	case errors.Is(err, claimrequests.ErrInvalidScope):
		writeError(w, http.StatusBadRequest, "invalid_scope", "At least one requested proof is required.")
	case errors.Is(err, claimrequests.ErrInvalidDuration):
		writeError(w, http.StatusBadRequest, "invalid_duration", "Duration must be at least 1 day.")
	default:
		writeError(w, http.StatusInternalServerError, "server_error", "Unable to create claim request.")
	}
}

func writeOrganizationClaimRequestUserError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, users.ErrInvalidEmail):
		writeError(w, http.StatusBadRequest, "invalid_user_email", "A valid user email is required.")
	case errors.Is(err, users.ErrUserNotFound):
		writeError(w, http.StatusNotFound, "user_not_found", "Target user was not found.")
	default:
		writeError(w, http.StatusInternalServerError, "server_error", "Unable to load target user.")
	}
}

func writeListClaimRequestsError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, claimrequests.ErrInvalidUser):
		writeError(w, http.StatusUnauthorized, "unauthorized", "Bearer access token is required.")
	default:
		writeError(w, http.StatusInternalServerError, "server_error", "Unable to load claim requests.")
	}
}

func writeListOrganizationClaimRequestsError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, claimrequests.ErrInvalidOrganizationID):
		writeError(w, http.StatusUnauthorized, "organization_api_key_required", "Organization API key is required.")
	default:
		writeError(w, http.StatusInternalServerError, "server_error", "Unable to load organization claim requests.")
	}
}

func writeGetClaimRequestError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, claimrequests.ErrClaimRequestNotFound):
		writeError(w, http.StatusNotFound, "claim_request_not_found", "Claim request was not found.")
	default:
		writeError(w, http.StatusInternalServerError, "server_error", "Unable to load claim request.")
	}
}

func writeApproveClaimRequestError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, claimrequests.ErrClaimRequestNotFound):
		writeError(w, http.StatusNotFound, "claim_request_not_found", "Claim request was not found.")
	case errors.Is(err, claimrequests.ErrInvalidSecurityPIN), errors.Is(err, securitypin.ErrInvalidFormat):
		writeError(w, http.StatusBadRequest, "invalid_security_pin", "Security PIN must be 4-6 digits.")
	case errors.Is(err, securitypin.ErrPINNotSet):
		writeError(w, http.StatusBadRequest, "security_pin_not_set", "Set your Security PIN before approving requests.")
	case errors.Is(err, securitypin.ErrInvalidPIN):
		writeError(w, http.StatusUnauthorized, "invalid_security_pin", "Security PIN is incorrect.")
	case errors.Is(err, securitypin.ErrPINLocked):
		writeError(w, http.StatusTooManyRequests, "security_pin_locked", "Security PIN approvals are temporarily locked.")
	case errors.Is(err, claimrequests.ErrClaimRequestExpired):
		writeError(w, http.StatusConflict, "claim_request_expired", "This request has expired.")
	case errors.Is(err, claimrequests.ErrClaimRequestNotOpen):
		writeError(w, http.StatusConflict, "claim_request_not_pending", "This request is no longer pending approval.")
	default:
		writeError(w, http.StatusInternalServerError, "server_error", "Unable to approve claim request.")
	}
}

func writeDenyClaimRequestError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, claimrequests.ErrClaimRequestNotFound):
		writeError(w, http.StatusNotFound, "claim_request_not_found", "Claim request was not found.")
	case errors.Is(err, claimrequests.ErrClaimRequestExpired):
		writeError(w, http.StatusConflict, "claim_request_expired", "This request has expired.")
	case errors.Is(err, claimrequests.ErrClaimRequestNotOpen):
		writeError(w, http.StatusConflict, "claim_request_not_pending", "This request is no longer pending approval.")
	default:
		writeError(w, http.StatusInternalServerError, "server_error", "Unable to deny claim request.")
	}
}

func writeListClaimsError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, claims.ErrInvalidUser):
		writeError(w, http.StatusUnauthorized, "unauthorized", "Bearer access token is required.")
	default:
		writeError(w, http.StatusInternalServerError, "server_error", "Unable to load claims.")
	}
}

func writeListOrganizationClaimsError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, claims.ErrInvalidOrganizationID):
		writeError(w, http.StatusUnauthorized, "organization_api_key_required", "Organization API key is required.")
	default:
		writeError(w, http.StatusInternalServerError, "server_error", "Unable to load organization claims.")
	}
}

func writeConfigureOrganizationWebhookEndpointError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, webhooks.ErrInvalidOrganization):
		writeError(w, http.StatusBadRequest, "invalid_organization", "Organization name is required.")
	case errors.Is(err, webhooks.ErrInvalidEndpointURL):
		writeError(w, http.StatusBadRequest, "invalid_webhook_url", "Webhook endpoint URL must be http or https.")
	default:
		writeError(w, http.StatusInternalServerError, "server_error", "Unable to configure webhook endpoint.")
	}
}

func writeGetOrganizationWebhookEndpointError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, webhooks.ErrInvalidOrganizationID):
		writeError(w, http.StatusUnauthorized, "organization_api_key_required", "Organization API key is required.")
	case errors.Is(err, webhooks.ErrEndpointNotFound):
		writeError(w, http.StatusNotFound, "webhook_endpoint_not_found", "Webhook endpoint is not configured.")
	default:
		writeError(w, http.StatusInternalServerError, "server_error", "Unable to load webhook endpoint.")
	}
}

func writeListOrganizationWebhookDeliveriesError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, webhooks.ErrInvalidOrganizationID):
		writeError(w, http.StatusUnauthorized, "organization_api_key_required", "Organization API key is required.")
	default:
		writeError(w, http.StatusInternalServerError, "server_error", "Unable to load webhook deliveries.")
	}
}

func writeGetClaimError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, claims.ErrClaimNotFound):
		writeError(w, http.StatusNotFound, "claim_not_found", "Claim was not found.")
	default:
		writeError(w, http.StatusInternalServerError, "server_error", "Unable to load claim.")
	}
}

func writeRevokeClaimError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, claims.ErrClaimNotFound):
		writeError(w, http.StatusNotFound, "claim_not_found", "Claim was not found.")
	case errors.Is(err, claims.ErrClaimNotActive):
		writeError(w, http.StatusConflict, "claim_not_active", "Only active claims can be revoked.")
	default:
		writeError(w, http.StatusInternalServerError, "server_error", "Unable to revoke claim.")
	}
}

func writeCreateExchangePINError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, claims.ErrClaimNotFound):
		writeError(w, http.StatusNotFound, "claim_not_found", "Claim was not found.")
	case errors.Is(err, claims.ErrClaimNotActive):
		writeError(w, http.StatusConflict, "claim_not_active", "Only active claims can create exchange PINs.")
	default:
		writeError(w, http.StatusInternalServerError, "server_error", "Unable to create exchange PIN.")
	}
}

func writeResolveExchangePINError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, claims.ErrInvalidExchangePIN):
		writeError(w, http.StatusBadRequest, "invalid_exchange_pin", "Exchange PIN must be 6-8 digits.")
	case errors.Is(err, claims.ErrExchangePINNotFound):
		writeError(w, http.StatusNotFound, "exchange_pin_not_found", "Exchange PIN was not found or has expired.")
	default:
		writeError(w, http.StatusInternalServerError, "server_error", "Unable to verify exchange PIN.")
	}
}

func readRequestIP(r *http.Request) string {
	forwardedFor := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if forwardedFor != "" {
		parts := strings.Split(forwardedFor, ",")
		return strings.TrimSpace(parts[0])
	}

	return strings.TrimSpace(r.RemoteAddr)
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

func authenticateOrganizationRequest(w http.ResponseWriter, r *http.Request, authenticator OrganizationAuthenticator) (claimrequests.Organization, bool) {
	apiKey := strings.TrimSpace(r.Header.Get("X-Kladd-API-Key"))
	if apiKey == "" {
		writeError(w, http.StatusUnauthorized, "organization_api_key_required", "Organization API key is required.")
		return claimrequests.Organization{}, false
	}

	organization, err := authenticator.Authenticate(r.Context(), apiKey)
	if err != nil {
		switch {
		case errors.Is(err, orgauth.ErrMissingAPIKey):
			writeError(w, http.StatusUnauthorized, "organization_api_key_required", "Organization API key is required.")
		case errors.Is(err, orgauth.ErrInvalidAPIKey):
			writeError(w, http.StatusUnauthorized, "invalid_organization_api_key", "Organization API key is invalid.")
		default:
			writeError(w, http.StatusInternalServerError, "server_error", "Unable to authenticate organization.")
		}
		return claimrequests.Organization{}, false
	}

	return organization, true
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
