package gateway

import (
	"encoding/json"
	"net/http"

	"github.com/crosslogic/control-plane/internal/credentials"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// CreateCredentialRequest represents the request to create a new credential
type CreateCredentialRequest struct {
	TenantID      uuid.UUID   `json:"tenant_id"`
	EnvironmentID *uuid.UUID  `json:"environment_id,omitempty"`
	Provider      string      `json:"provider"`
	Name          string      `json:"name"`
	Credentials   interface{} `json:"credentials"`
	IsDefault     bool        `json:"is_default"`
}

// UpdateCredentialRequest represents the request to update a credential
type UpdateCredentialRequest struct {
	Credentials interface{} `json:"credentials"`
}

// CredentialResponse represents the sanitized credential response (no secrets)
type CredentialResponse struct {
	credentials.CredentialOutput
}

// ListCredentialsResponse represents the response for listing credentials
type ListCredentialsResponse struct {
	Data []credentials.CredentialOutput `json:"data"`
}

// ValidateCredentialRequest represents the request to validate a credential
type ValidateCredentialRequest struct {
	CredentialID uuid.UUID `json:"credential_id"`
}

// ValidateCredentialResponse represents the response after validation
type ValidateCredentialResponse struct {
	Valid          bool    `json:"valid"`
	ValidationError *string `json:"validation_error,omitempty"`
}

// handleCreateCredential creates a new cloud credential
// POST /admin/credentials
func (g *Gateway) handleCreateCredential(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req CreateCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields
	if req.TenantID == uuid.Nil {
		g.writeError(w, http.StatusBadRequest, "tenant_id is required")
		return
	}
	if req.Provider == "" {
		g.writeError(w, http.StatusBadRequest, "provider is required")
		return
	}
	if req.Name == "" {
		g.writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Credentials == nil {
		g.writeError(w, http.StatusBadRequest, "credentials are required")
		return
	}

	// Validate provider
	if !credentials.IsValidProvider(req.Provider) {
		g.writeError(w, http.StatusBadRequest, "unsupported provider: "+req.Provider)
		return
	}

	// Create credential input
	input := credentials.CredentialInput{
		TenantID:      req.TenantID,
		EnvironmentID: req.EnvironmentID,
		Provider:      req.Provider,
		Name:          req.Name,
		Credentials:   req.Credentials,
		IsDefault:     req.IsDefault,
	}

	// Create credential
	credential, err := g.credentialService.CreateCredential(ctx, input)
	if err != nil {
		g.logger.Error("failed to create credential",
			zap.Error(err),
			zap.String("tenant_id", req.TenantID.String()),
			zap.String("provider", req.Provider),
		)
		g.writeError(w, http.StatusInternalServerError, "failed to create credential: "+err.Error())
		return
	}

	g.logger.Info("credential created",
		zap.String("credential_id", credential.ID.String()),
		zap.String("tenant_id", req.TenantID.String()),
		zap.String("provider", req.Provider),
		zap.String("name", req.Name),
	)

	g.writeJSON(w, http.StatusCreated, credential.ToOutput())
}

// handleListCredentials lists all credentials for a tenant
// GET /admin/credentials?tenant_id={tenant_id}&environment_id={environment_id}
func (g *Gateway) handleListCredentials(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	tenantIDStr := r.URL.Query().Get("tenant_id")
	if tenantIDStr == "" {
		g.writeError(w, http.StatusBadRequest, "tenant_id query parameter is required")
		return
	}

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid tenant_id")
		return
	}

	var environmentID *uuid.UUID
	environmentIDStr := r.URL.Query().Get("environment_id")
	if environmentIDStr != "" {
		envID, err := uuid.Parse(environmentIDStr)
		if err != nil {
			g.writeError(w, http.StatusBadRequest, "invalid environment_id")
			return
		}
		environmentID = &envID
	}

	// List credentials
	credentialsList, err := g.credentialService.ListCredentials(ctx, tenantID, environmentID)
	if err != nil {
		g.logger.Error("failed to list credentials",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
		)
		g.writeError(w, http.StatusInternalServerError, "failed to list credentials")
		return
	}

	if credentialsList == nil {
		credentialsList = []credentials.CredentialOutput{}
	}

	g.writeJSON(w, http.StatusOK, ListCredentialsResponse{
		Data: credentialsList,
	})
}

// handleGetCredential retrieves a specific credential (without decrypted secrets)
// GET /admin/credentials/{id}?tenant_id={tenant_id}
func (g *Gateway) handleGetCredential(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	credentialIDStr := chi.URLParam(r, "id")
	credentialID, err := uuid.Parse(credentialIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid credential ID")
		return
	}

	tenantIDStr := r.URL.Query().Get("tenant_id")
	if tenantIDStr == "" {
		g.writeError(w, http.StatusBadRequest, "tenant_id query parameter is required")
		return
	}

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid tenant_id")
		return
	}

	// Get credential (this will decrypt, but we won't expose the decrypted data)
	credential, err := g.credentialService.GetCredential(ctx, credentialID, tenantID)
	if err != nil {
		g.logger.Error("failed to get credential",
			zap.Error(err),
			zap.String("credential_id", credentialID.String()),
			zap.String("tenant_id", tenantID.String()),
		)
		g.writeError(w, http.StatusNotFound, "credential not found")
		return
	}

	// Return sanitized output (no decrypted data)
	g.writeJSON(w, http.StatusOK, credential.ToOutput())
}

// handleUpdateCredential updates a credential's encrypted data
// PUT /admin/credentials/{id}?tenant_id={tenant_id}
func (g *Gateway) handleUpdateCredential(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	credentialIDStr := chi.URLParam(r, "id")
	credentialID, err := uuid.Parse(credentialIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid credential ID")
		return
	}

	tenantIDStr := r.URL.Query().Get("tenant_id")
	if tenantIDStr == "" {
		g.writeError(w, http.StatusBadRequest, "tenant_id query parameter is required")
		return
	}

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid tenant_id")
		return
	}

	var req UpdateCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Credentials == nil {
		g.writeError(w, http.StatusBadRequest, "credentials are required")
		return
	}

	// Update credential
	err = g.credentialService.UpdateCredential(ctx, credentialID, tenantID, req.Credentials)
	if err != nil {
		g.logger.Error("failed to update credential",
			zap.Error(err),
			zap.String("credential_id", credentialID.String()),
			zap.String("tenant_id", tenantID.String()),
		)
		g.writeError(w, http.StatusInternalServerError, "failed to update credential: "+err.Error())
		return
	}

	g.logger.Info("credential updated",
		zap.String("credential_id", credentialID.String()),
		zap.String("tenant_id", tenantID.String()),
	)

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "updated",
		"message": "credential updated successfully",
	})
}

// handleDeleteCredential soft deletes a credential
// DELETE /admin/credentials/{id}?tenant_id={tenant_id}
func (g *Gateway) handleDeleteCredential(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	credentialIDStr := chi.URLParam(r, "id")
	credentialID, err := uuid.Parse(credentialIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid credential ID")
		return
	}

	tenantIDStr := r.URL.Query().Get("tenant_id")
	if tenantIDStr == "" {
		g.writeError(w, http.StatusBadRequest, "tenant_id query parameter is required")
		return
	}

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid tenant_id")
		return
	}

	// Delete credential
	err = g.credentialService.DeleteCredential(ctx, credentialID, tenantID)
	if err != nil {
		g.logger.Error("failed to delete credential",
			zap.Error(err),
			zap.String("credential_id", credentialID.String()),
			zap.String("tenant_id", tenantID.String()),
		)
		g.writeError(w, http.StatusInternalServerError, "failed to delete credential: "+err.Error())
		return
	}

	g.logger.Info("credential deleted",
		zap.String("credential_id", credentialID.String()),
		zap.String("tenant_id", tenantID.String()),
	)

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "deleted",
		"message": "credential deleted successfully",
	})
}

// handleValidateCredential validates a credential by attempting to use it
// POST /admin/credentials/{id}/validate?tenant_id={tenant_id}
func (g *Gateway) handleValidateCredential(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	credentialIDStr := chi.URLParam(r, "id")
	credentialID, err := uuid.Parse(credentialIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid credential ID")
		return
	}

	tenantIDStr := r.URL.Query().Get("tenant_id")
	if tenantIDStr == "" {
		g.writeError(w, http.StatusBadRequest, "tenant_id query parameter is required")
		return
	}

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid tenant_id")
		return
	}

	// Get the credential to validate it exists
	credential, err := g.credentialService.GetCredential(ctx, credentialID, tenantID)
	if err != nil {
		g.logger.Error("failed to get credential for validation",
			zap.Error(err),
			zap.String("credential_id", credentialID.String()),
		)
		g.writeError(w, http.StatusNotFound, "credential not found")
		return
	}

	// TODO: Implement actual validation logic per provider
	// For now, we'll just mark it as validated if we can decrypt it
	var validationError *string
	if credential.DecryptedData == nil {
		errMsg := "failed to decrypt credentials"
		validationError = &errMsg
	}

	// Update validation status
	err = g.credentialService.ValidateCredential(ctx, credentialID, tenantID, validationError)
	if err != nil {
		g.logger.Error("failed to update credential validation",
			zap.Error(err),
			zap.String("credential_id", credentialID.String()),
		)
		g.writeError(w, http.StatusInternalServerError, "failed to validate credential")
		return
	}

	g.logger.Info("credential validated",
		zap.String("credential_id", credentialID.String()),
		zap.String("tenant_id", tenantID.String()),
		zap.Bool("valid", validationError == nil),
	)

	g.writeJSON(w, http.StatusOK, ValidateCredentialResponse{
		Valid:          validationError == nil,
		ValidationError: validationError,
	})
}

// handleSetDefaultCredential sets a credential as the default for its tenant/provider
// POST /admin/credentials/{id}/default?tenant_id={tenant_id}
func (g *Gateway) handleSetDefaultCredential(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	credentialIDStr := chi.URLParam(r, "id")
	credentialID, err := uuid.Parse(credentialIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid credential ID")
		return
	}

	tenantIDStr := r.URL.Query().Get("tenant_id")
	if tenantIDStr == "" {
		g.writeError(w, http.StatusBadRequest, "tenant_id query parameter is required")
		return
	}

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid tenant_id")
		return
	}

	// Set as default
	err = g.credentialService.SetDefaultCredential(ctx, credentialID, tenantID)
	if err != nil {
		g.logger.Error("failed to set default credential",
			zap.Error(err),
			zap.String("credential_id", credentialID.String()),
			zap.String("tenant_id", tenantID.String()),
		)
		g.writeError(w, http.StatusInternalServerError, "failed to set default credential: "+err.Error())
		return
	}

	g.logger.Info("credential set as default",
		zap.String("credential_id", credentialID.String()),
		zap.String("tenant_id", tenantID.String()),
	)

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "credential set as default",
	})
}
