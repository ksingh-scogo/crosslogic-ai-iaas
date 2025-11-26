package gateway

import (
	"encoding/json"
	"net/http"

	"github.com/crosslogic/control-plane/internal/credentials"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// TenantCreateCredentialRequest represents the request to create a credential (tenant self-service)
type TenantCreateCredentialRequest struct {
	Provider    string      `json:"provider"`
	Name        string      `json:"name"`
	Credentials interface{} `json:"credentials"`
	IsDefault   bool        `json:"is_default"`
}

// TenantUpdateCredentialRequest represents the request to update a credential
type TenantUpdateCredentialRequest struct {
	Credentials interface{} `json:"credentials"`
}

// handleCreateTenantCredential creates a new cloud credential for the authenticated tenant
// POST /v1/credentials
func (g *Gateway) handleCreateTenantCredential(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get tenant ID from auth context
	tenantID, ok := ctx.Value("tenant_id").(uuid.UUID)
	if !ok {
		g.writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req TenantCreateCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields
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

	// Create credential input (tenant-level, no environment_id)
	input := credentials.CredentialInput{
		TenantID:      tenantID,
		EnvironmentID: nil, // Tenant-level credentials
		Provider:      req.Provider,
		Name:          req.Name,
		Credentials:   req.Credentials,
		IsDefault:     req.IsDefault,
	}

	// Create credential
	credential, err := g.credentialService.CreateCredential(ctx, input)
	if err != nil {
		g.logger.Error("failed to create tenant credential",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
			zap.String("provider", req.Provider),
		)
		g.writeError(w, http.StatusInternalServerError, "failed to create credential: "+err.Error())
		return
	}

	g.logger.Info("tenant credential created",
		zap.String("credential_id", credential.ID.String()),
		zap.String("tenant_id", tenantID.String()),
		zap.String("provider", req.Provider),
		zap.String("name", req.Name),
	)

	g.writeJSON(w, http.StatusCreated, credential.ToOutput())
}

// handleListTenantCredentials lists all credentials for the authenticated tenant
// GET /v1/credentials
func (g *Gateway) handleListTenantCredentials(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get tenant ID from auth context
	tenantID, ok := ctx.Value("tenant_id").(uuid.UUID)
	if !ok {
		g.writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// List credentials (tenant-level only)
	credentialsList, err := g.credentialService.ListCredentials(ctx, tenantID, nil)
	if err != nil {
		g.logger.Error("failed to list tenant credentials",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
		)
		g.writeError(w, http.StatusInternalServerError, "failed to list credentials")
		return
	}

	if credentialsList == nil {
		credentialsList = []credentials.CredentialOutput{}
	}

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": credentialsList,
	})
}

// handleGetTenantCredential retrieves a specific credential (without decrypted secrets)
// GET /v1/credentials/{id}
func (g *Gateway) handleGetTenantCredential(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get tenant ID from auth context
	tenantID, ok := ctx.Value("tenant_id").(uuid.UUID)
	if !ok {
		g.writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	credentialIDStr := chi.URLParam(r, "id")
	credentialID, err := uuid.Parse(credentialIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid credential ID")
		return
	}

	// Get credential (verifies tenant ownership)
	credential, err := g.credentialService.GetCredential(ctx, credentialID, tenantID)
	if err != nil {
		g.logger.Error("failed to get tenant credential",
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

// handleUpdateTenantCredential updates a credential's encrypted data
// PUT /v1/credentials/{id}
func (g *Gateway) handleUpdateTenantCredential(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get tenant ID from auth context
	tenantID, ok := ctx.Value("tenant_id").(uuid.UUID)
	if !ok {
		g.writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	credentialIDStr := chi.URLParam(r, "id")
	credentialID, err := uuid.Parse(credentialIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid credential ID")
		return
	}

	var req TenantUpdateCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Credentials == nil {
		g.writeError(w, http.StatusBadRequest, "credentials are required")
		return
	}

	// Update credential (verifies tenant ownership)
	err = g.credentialService.UpdateCredential(ctx, credentialID, tenantID, req.Credentials)
	if err != nil {
		g.logger.Error("failed to update tenant credential",
			zap.Error(err),
			zap.String("credential_id", credentialID.String()),
			zap.String("tenant_id", tenantID.String()),
		)
		g.writeError(w, http.StatusInternalServerError, "failed to update credential: "+err.Error())
		return
	}

	g.logger.Info("tenant credential updated",
		zap.String("credential_id", credentialID.String()),
		zap.String("tenant_id", tenantID.String()),
	)

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "updated",
		"message": "credential updated successfully",
	})
}

// handleDeleteTenantCredential soft deletes a credential
// DELETE /v1/credentials/{id}
func (g *Gateway) handleDeleteTenantCredential(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get tenant ID from auth context
	tenantID, ok := ctx.Value("tenant_id").(uuid.UUID)
	if !ok {
		g.writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	credentialIDStr := chi.URLParam(r, "id")
	credentialID, err := uuid.Parse(credentialIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid credential ID")
		return
	}

	// Delete credential (verifies tenant ownership)
	err = g.credentialService.DeleteCredential(ctx, credentialID, tenantID)
	if err != nil {
		g.logger.Error("failed to delete tenant credential",
			zap.Error(err),
			zap.String("credential_id", credentialID.String()),
			zap.String("tenant_id", tenantID.String()),
		)
		g.writeError(w, http.StatusInternalServerError, "failed to delete credential: "+err.Error())
		return
	}

	g.logger.Info("tenant credential deleted",
		zap.String("credential_id", credentialID.String()),
		zap.String("tenant_id", tenantID.String()),
	)

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "deleted",
		"message": "credential deleted successfully",
	})
}

// handleValidateTenantCredential validates a credential by attempting to use it
// POST /v1/credentials/{id}/validate
func (g *Gateway) handleValidateTenantCredential(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get tenant ID from auth context
	tenantID, ok := ctx.Value("tenant_id").(uuid.UUID)
	if !ok {
		g.writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	credentialIDStr := chi.URLParam(r, "id")
	credentialID, err := uuid.Parse(credentialIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid credential ID")
		return
	}

	// Get the credential to validate it exists and belongs to tenant
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

	g.logger.Info("tenant credential validated",
		zap.String("credential_id", credentialID.String()),
		zap.String("tenant_id", tenantID.String()),
		zap.Bool("valid", validationError == nil),
	)

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"valid":            validationError == nil,
		"validation_error": validationError,
	})
}

// handleSetDefaultTenantCredential sets a credential as the default for its provider
// POST /v1/credentials/{id}/default
func (g *Gateway) handleSetDefaultTenantCredential(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get tenant ID from auth context
	tenantID, ok := ctx.Value("tenant_id").(uuid.UUID)
	if !ok {
		g.writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	credentialIDStr := chi.URLParam(r, "id")
	credentialID, err := uuid.Parse(credentialIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid credential ID")
		return
	}

	// Set as default (verifies tenant ownership)
	err = g.credentialService.SetDefaultCredential(ctx, credentialID, tenantID)
	if err != nil {
		g.logger.Error("failed to set default tenant credential",
			zap.Error(err),
			zap.String("credential_id", credentialID.String()),
			zap.String("tenant_id", tenantID.String()),
		)
		g.writeError(w, http.StatusInternalServerError, "failed to set default credential: "+err.Error())
		return
	}

	g.logger.Info("tenant credential set as default",
		zap.String("credential_id", credentialID.String()),
		zap.String("tenant_id", tenantID.String()),
	)

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "credential set as default",
	})
}
