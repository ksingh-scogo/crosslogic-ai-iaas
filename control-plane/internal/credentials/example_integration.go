package credentials

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Example HTTP handlers showing how to integrate the credential service
// These examples demonstrate proper patterns for API endpoints

// CredentialHandler handles HTTP requests for credential management
type CredentialHandler struct {
	service *Service
	logger  *zap.Logger
}

// NewCredentialHandler creates a new credential HTTP handler
func NewCredentialHandler(service *Service, logger *zap.Logger) *CredentialHandler {
	return &CredentialHandler{
		service: service,
		logger:  logger,
	}
}

// CreateCredentialRequest represents the HTTP request body for creating credentials
type CreateCredentialRequest struct {
	EnvironmentID *string     `json:"environment_id,omitempty"`
	Provider      string      `json:"provider"`
	Name          string      `json:"name"`
	Credentials   interface{} `json:"credentials"`
	IsDefault     bool        `json:"is_default"`
}

// CreateCredentialResponse represents the HTTP response
type CreateCredentialResponse struct {
	ID        string `json:"id"`
	Provider  string `json:"provider"`
	Name      string `json:"name"`
	IsDefault bool   `json:"is_default"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// HandleCreateCredential creates a new cloud credential
// POST /api/v1/tenants/{tenantID}/credentials
func (h *CredentialHandler) HandleCreateCredential(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get tenant ID from URL or auth context
	tenantIDStr := chi.URLParam(r, "tenantID")
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid tenant ID", err)
		return
	}

	// Parse request body
	var req CreateCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Validate required fields
	if req.Provider == "" || req.Name == "" || req.Credentials == nil {
		h.respondError(w, http.StatusBadRequest, "Missing required fields", nil)
		return
	}

	// Parse environment ID if provided
	var environmentID *uuid.UUID
	if req.EnvironmentID != nil {
		envID, err := uuid.Parse(*req.EnvironmentID)
		if err != nil {
			h.respondError(w, http.StatusBadRequest, "Invalid environment ID", err)
			return
		}
		environmentID = &envID
	}

	// Get user ID from context (set by auth middleware)
	userID, _ := ctx.Value("user_id").(uuid.UUID)

	// Create credential input
	input := CredentialInput{
		TenantID:        tenantID,
		EnvironmentID:   environmentID,
		Provider:        req.Provider,
		Name:            req.Name,
		Credentials:     req.Credentials,
		IsDefault:       req.IsDefault,
		CreatedByUserID: &userID,
	}

	// Create credential
	credential, err := h.service.CreateCredential(ctx, input)
	if err != nil {
		h.logger.Error("failed to create credential",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
			zap.String("provider", req.Provider),
		)
		h.respondError(w, http.StatusInternalServerError, "Failed to create credential", err)
		return
	}

	// Prepare response
	response := CreateCredentialResponse{
		ID:        credential.ID.String(),
		Provider:  credential.Provider,
		Name:      credential.Name,
		IsDefault: credential.IsDefault,
		Status:    credential.Status,
		CreatedAt: credential.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	h.respondJSON(w, http.StatusCreated, response)
}

// HandleGetCredential retrieves a credential by ID (without decrypted data)
// GET /api/v1/tenants/{tenantID}/credentials/{credentialID}
func (h *CredentialHandler) HandleGetCredential(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tenantID, err := uuid.Parse(chi.URLParam(r, "tenantID"))
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid tenant ID", err)
		return
	}

	credentialID, err := uuid.Parse(chi.URLParam(r, "credentialID"))
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid credential ID", err)
		return
	}

	// Get credential metadata only (no decryption)
	credential, err := h.service.GetCredential(ctx, credentialID, tenantID)
	if err != nil {
		h.logger.Error("failed to get credential",
			zap.Error(err),
			zap.String("credential_id", credentialID.String()),
		)
		h.respondError(w, http.StatusNotFound, "Credential not found", err)
		return
	}

	// Return sanitized output (no decrypted data)
	output := credential.CloudCredential.ToOutput()
	h.respondJSON(w, http.StatusOK, output)
}

// HandleListCredentials lists all credentials for a tenant/environment
// GET /api/v1/tenants/{tenantID}/credentials?environment_id={envID}
func (h *CredentialHandler) HandleListCredentials(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tenantID, err := uuid.Parse(chi.URLParam(r, "tenantID"))
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid tenant ID", err)
		return
	}

	// Optional environment filter
	var environmentID *uuid.UUID
	if envIDStr := r.URL.Query().Get("environment_id"); envIDStr != "" {
		envID, err := uuid.Parse(envIDStr)
		if err != nil {
			h.respondError(w, http.StatusBadRequest, "Invalid environment ID", err)
			return
		}
		environmentID = &envID
	}

	credentials, err := h.service.ListCredentials(ctx, tenantID, environmentID)
	if err != nil {
		h.logger.Error("failed to list credentials",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
		)
		h.respondError(w, http.StatusInternalServerError, "Failed to list credentials", err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"credentials": credentials,
		"total":       len(credentials),
	})
}

// HandleUpdateCredential updates existing credentials
// PUT /api/v1/tenants/{tenantID}/credentials/{credentialID}
func (h *CredentialHandler) HandleUpdateCredential(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tenantID, err := uuid.Parse(chi.URLParam(r, "tenantID"))
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid tenant ID", err)
		return
	}

	credentialID, err := uuid.Parse(chi.URLParam(r, "credentialID"))
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid credential ID", err)
		return
	}

	// Parse request body
	var req struct {
		Credentials interface{} `json:"credentials"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Update credential
	err = h.service.UpdateCredential(ctx, credentialID, tenantID, req.Credentials)
	if err != nil {
		h.logger.Error("failed to update credential",
			zap.Error(err),
			zap.String("credential_id", credentialID.String()),
		)
		h.respondError(w, http.StatusInternalServerError, "Failed to update credential", err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]string{
		"message": "Credential updated successfully",
	})
}

// HandleDeleteCredential soft deletes a credential
// DELETE /api/v1/tenants/{tenantID}/credentials/{credentialID}
func (h *CredentialHandler) HandleDeleteCredential(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tenantID, err := uuid.Parse(chi.URLParam(r, "tenantID"))
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid tenant ID", err)
		return
	}

	credentialID, err := uuid.Parse(chi.URLParam(r, "credentialID"))
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid credential ID", err)
		return
	}

	err = h.service.DeleteCredential(ctx, credentialID, tenantID)
	if err != nil {
		h.logger.Error("failed to delete credential",
			zap.Error(err),
			zap.String("credential_id", credentialID.String()),
		)
		h.respondError(w, http.StatusInternalServerError, "Failed to delete credential", err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]string{
		"message": "Credential deleted successfully",
	})
}

// HandleSetDefaultCredential sets a credential as default
// POST /api/v1/tenants/{tenantID}/credentials/{credentialID}/set-default
func (h *CredentialHandler) HandleSetDefaultCredential(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tenantID, err := uuid.Parse(chi.URLParam(r, "tenantID"))
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid tenant ID", err)
		return
	}

	credentialID, err := uuid.Parse(chi.URLParam(r, "credentialID"))
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid credential ID", err)
		return
	}

	err = h.service.SetDefaultCredential(ctx, credentialID, tenantID)
	if err != nil {
		h.logger.Error("failed to set default credential",
			zap.Error(err),
			zap.String("credential_id", credentialID.String()),
		)
		h.respondError(w, http.StatusInternalServerError, "Failed to set default credential", err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]string{
		"message": "Default credential set successfully",
	})
}

// HandleValidateCredential validates credentials against cloud provider
// POST /api/v1/tenants/{tenantID}/credentials/{credentialID}/validate
func (h *CredentialHandler) HandleValidateCredential(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tenantID, err := uuid.Parse(chi.URLParam(r, "tenantID"))
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid tenant ID", err)
		return
	}

	credentialID, err := uuid.Parse(chi.URLParam(r, "credentialID"))
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid credential ID", err)
		return
	}

	// Get and decrypt credentials
	credential, err := h.service.GetCredential(ctx, credentialID, tenantID)
	if err != nil {
		h.respondError(w, http.StatusNotFound, "Credential not found", err)
		return
	}

	// Validate against cloud provider
	validationErr := h.validateAgainstProvider(ctx, credential.Provider, credential.DecryptedData)

	// Update validation status
	if validationErr != nil {
		errMsg := validationErr.Error()
		err = h.service.ValidateCredential(ctx, credentialID, tenantID, &errMsg)
	} else {
		err = h.service.ValidateCredential(ctx, credentialID, tenantID, nil)
	}

	if err != nil {
		h.logger.Error("failed to update validation status",
			zap.Error(err),
			zap.String("credential_id", credentialID.String()),
		)
	}

	if validationErr != nil {
		h.respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"valid":   false,
			"message": validationErr.Error(),
		})
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"valid":   true,
		"message": "Credentials validated successfully",
	})
}

// validateAgainstProvider validates credentials against the actual cloud provider
func (h *CredentialHandler) validateAgainstProvider(ctx context.Context, provider string, creds interface{}) error {
	// This is a placeholder - implement actual validation against each provider
	switch provider {
	case "aws":
		// Validate AWS credentials by making a test API call
		// Example: Use AWS STS GetCallerIdentity
		return nil
	case "azure":
		// Validate Azure credentials
		return nil
	case "gcp":
		// Validate GCP credentials
		return nil
	default:
		return fmt.Errorf("validation not implemented for provider: %s", provider)
	}
}

// RegisterRoutes registers all credential routes
func (h *CredentialHandler) RegisterRoutes(r chi.Router) {
	r.Route("/tenants/{tenantID}/credentials", func(r chi.Router) {
		r.Post("/", h.HandleCreateCredential)
		r.Get("/", h.HandleListCredentials)
		r.Get("/{credentialID}", h.HandleGetCredential)
		r.Put("/{credentialID}", h.HandleUpdateCredential)
		r.Delete("/{credentialID}", h.HandleDeleteCredential)
		r.Post("/{credentialID}/set-default", h.HandleSetDefaultCredential)
		r.Post("/{credentialID}/validate", h.HandleValidateCredential)
	})
}

// Helper methods

func (h *CredentialHandler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *CredentialHandler) respondError(w http.ResponseWriter, status int, message string, err error) {
	response := ErrorResponse{
		Error: message,
	}
	if err != nil {
		response.Message = err.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}
