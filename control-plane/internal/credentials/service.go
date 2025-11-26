package credentials

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/crosslogic/control-plane/pkg/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// Service handles credential management operations
type Service struct {
	db         *database.Database
	encryption *EncryptionService
	logger     *zap.Logger
}

// NewService creates a new credential service
func NewService(db *database.Database, encryptionKey string, keyID string, logger *zap.Logger) (*Service, error) {
	encryption, err := NewEncryptionService(encryptionKey, keyID)
	if err != nil {
		return nil, fmt.Errorf("failed to create encryption service: %w", err)
	}

	return &Service{
		db:         db,
		encryption: encryption,
		logger:     logger,
	}, nil
}

// CreateCredential encrypts and stores cloud provider credentials
func (s *Service) CreateCredential(ctx context.Context, input CredentialInput) (*CloudCredential, error) {
	// Validate provider
	if !IsValidProvider(input.Provider) {
		return nil, fmt.Errorf("unsupported provider: %s", input.Provider)
	}

	// Validate credentials structure
	if err := ValidateCredentialsStructure(input.Provider, input.Credentials); err != nil {
		return nil, fmt.Errorf("invalid credentials: %w", err)
	}

	// Encrypt credentials
	encryptedData, err := s.encryption.Encrypt(input.Credentials)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt credentials: %w", err)
	}

	// If setting as default, unset any existing default for this tenant/environment/provider
	if input.IsDefault {
		if err := s.unsetDefaultCredential(ctx, input.TenantID, input.EnvironmentID, input.Provider); err != nil {
			return nil, fmt.Errorf("failed to unset existing default: %w", err)
		}
	}

	// Insert into database
	var credential CloudCredential
	query := `
		INSERT INTO cloud_credentials (
			tenant_id, environment_id, provider, name,
			credentials_encrypted, encryption_key_id, is_default,
			status, created_by_user_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, tenant_id, environment_id, provider, name,
		          credentials_encrypted, encryption_key_id, is_default,
		          status, last_used_at, last_validated_at, validation_error,
		          created_by_user_id, created_at, updated_at, deleted_at
	`

	err = s.db.Pool.QueryRow(ctx, query,
		input.TenantID,
		input.EnvironmentID,
		input.Provider,
		input.Name,
		encryptedData,
		s.encryption.GetKeyID(),
		input.IsDefault,
		StatusActive,
		input.CreatedByUserID,
	).Scan(
		&credential.ID,
		&credential.TenantID,
		&credential.EnvironmentID,
		&credential.Provider,
		&credential.Name,
		&credential.CredentialsEncrypted,
		&credential.EncryptionKeyID,
		&credential.IsDefault,
		&credential.Status,
		&credential.LastUsedAt,
		&credential.LastValidatedAt,
		&credential.ValidationError,
		&credential.CreatedByUserID,
		&credential.CreatedAt,
		&credential.UpdatedAt,
		&credential.DeletedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create credential: %w", err)
	}

	s.logger.Info("created cloud credential",
		zap.String("credential_id", credential.ID.String()),
		zap.String("tenant_id", credential.TenantID.String()),
		zap.String("provider", credential.Provider),
		zap.String("name", credential.Name),
	)

	// Don't return encrypted data in the response
	credential.CredentialsEncrypted = nil

	return &credential, nil
}

// GetCredential retrieves and decrypts a credential by ID
func (s *Service) GetCredential(ctx context.Context, credentialID uuid.UUID, tenantID uuid.UUID) (*DecryptedCredential, error) {
	var credential CloudCredential

	query := `
		SELECT id, tenant_id, environment_id, provider, name,
		       credentials_encrypted, encryption_key_id, is_default,
		       status, last_used_at, last_validated_at, validation_error,
		       created_by_user_id, created_at, updated_at, deleted_at
		FROM cloud_credentials
		WHERE id = $1 AND tenant_id = $2 AND status != $3
	`

	err := s.db.Pool.QueryRow(ctx, query, credentialID, tenantID, StatusDeleted).Scan(
		&credential.ID,
		&credential.TenantID,
		&credential.EnvironmentID,
		&credential.Provider,
		&credential.Name,
		&credential.CredentialsEncrypted,
		&credential.EncryptionKeyID,
		&credential.IsDefault,
		&credential.Status,
		&credential.LastUsedAt,
		&credential.LastValidatedAt,
		&credential.ValidationError,
		&credential.CreatedByUserID,
		&credential.CreatedAt,
		&credential.UpdatedAt,
		&credential.DeletedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("credential not found")
		}
		return nil, fmt.Errorf("failed to get credential: %w", err)
	}

	// Decrypt credentials
	decryptedData := make(map[string]interface{})
	if err := s.encryption.Decrypt(credential.CredentialsEncrypted, &decryptedData); err != nil {
		return nil, fmt.Errorf("failed to decrypt credentials: %w", err)
	}

	// Update last used timestamp (async)
	go s.updateLastUsed(context.Background(), credentialID)

	return &DecryptedCredential{
		CloudCredential: credential,
		DecryptedData:   decryptedData,
	}, nil
}

// GetCredentialByProvider retrieves credentials for a specific tenant/environment/provider
// It prefers environment-specific credentials over tenant-level credentials
func (s *Service) GetCredentialByProvider(ctx context.Context, tenantID uuid.UUID, environmentID *uuid.UUID, provider string) (*DecryptedCredential, error) {
	var credential CloudCredential

	// Query with fallback: prefer environment-specific, then tenant-level default
	query := `
		SELECT id, tenant_id, environment_id, provider, name,
		       credentials_encrypted, encryption_key_id, is_default,
		       status, last_used_at, last_validated_at, validation_error,
		       created_by_user_id, created_at, updated_at, deleted_at
		FROM cloud_credentials
		WHERE tenant_id = $1
		  AND provider = $2
		  AND status = $3
		  AND (environment_id = $4 OR environment_id IS NULL)
		ORDER BY
		  CASE WHEN environment_id IS NOT NULL THEN 1 ELSE 2 END,
		  CASE WHEN is_default = true THEN 1 ELSE 2 END
		LIMIT 1
	`

	err := s.db.Pool.QueryRow(ctx, query, tenantID, provider, StatusActive, environmentID).Scan(
		&credential.ID,
		&credential.TenantID,
		&credential.EnvironmentID,
		&credential.Provider,
		&credential.Name,
		&credential.CredentialsEncrypted,
		&credential.EncryptionKeyID,
		&credential.IsDefault,
		&credential.Status,
		&credential.LastUsedAt,
		&credential.LastValidatedAt,
		&credential.ValidationError,
		&credential.CreatedByUserID,
		&credential.CreatedAt,
		&credential.UpdatedAt,
		&credential.DeletedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("no credentials found for provider %s", provider)
		}
		return nil, fmt.Errorf("failed to get credential: %w", err)
	}

	// Decrypt credentials
	decryptedData := make(map[string]interface{})
	if err := s.encryption.Decrypt(credential.CredentialsEncrypted, &decryptedData); err != nil {
		return nil, fmt.Errorf("failed to decrypt credentials: %w", err)
	}

	// Update last used timestamp (async)
	go s.updateLastUsed(context.Background(), credential.ID)

	return &DecryptedCredential{
		CloudCredential: credential,
		DecryptedData:   decryptedData,
	}, nil
}

// GetDefaultCredential gets the default credential for a tenant and provider
func (s *Service) GetDefaultCredential(ctx context.Context, tenantID uuid.UUID, provider string) (*DecryptedCredential, error) {
	var credential CloudCredential

	query := `
		SELECT id, tenant_id, environment_id, provider, name,
		       credentials_encrypted, encryption_key_id, is_default,
		       status, last_used_at, last_validated_at, validation_error,
		       created_by_user_id, created_at, updated_at, deleted_at
		FROM cloud_credentials
		WHERE tenant_id = $1
		  AND provider = $2
		  AND is_default = true
		  AND status = $3
		  AND environment_id IS NULL
		LIMIT 1
	`

	err := s.db.Pool.QueryRow(ctx, query, tenantID, provider, StatusActive).Scan(
		&credential.ID,
		&credential.TenantID,
		&credential.EnvironmentID,
		&credential.Provider,
		&credential.Name,
		&credential.CredentialsEncrypted,
		&credential.EncryptionKeyID,
		&credential.IsDefault,
		&credential.Status,
		&credential.LastUsedAt,
		&credential.LastValidatedAt,
		&credential.ValidationError,
		&credential.CreatedByUserID,
		&credential.CreatedAt,
		&credential.UpdatedAt,
		&credential.DeletedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("no default credential found for provider %s", provider)
		}
		return nil, fmt.Errorf("failed to get default credential: %w", err)
	}

	// Decrypt credentials
	decryptedData := make(map[string]interface{})
	if err := s.encryption.Decrypt(credential.CredentialsEncrypted, &decryptedData); err != nil {
		return nil, fmt.Errorf("failed to decrypt credentials: %w", err)
	}

	// Update last used timestamp (async)
	go s.updateLastUsed(context.Background(), credential.ID)

	return &DecryptedCredential{
		CloudCredential: credential,
		DecryptedData:   decryptedData,
	}, nil
}

// ListCredentials lists all credentials for a tenant/environment (without decryption)
func (s *Service) ListCredentials(ctx context.Context, tenantID uuid.UUID, environmentID *uuid.UUID) ([]CredentialOutput, error) {
	var query string
	var args []interface{}

	if environmentID != nil {
		query = `
			SELECT id, tenant_id, environment_id, provider, name,
			       is_default, status, last_used_at, last_validated_at,
			       validation_error, created_at, updated_at
			FROM cloud_credentials
			WHERE tenant_id = $1
			  AND (environment_id = $2 OR environment_id IS NULL)
			  AND status != $3
			ORDER BY provider, environment_id NULLS FIRST, is_default DESC, name
		`
		args = []interface{}{tenantID, environmentID, StatusDeleted}
	} else {
		query = `
			SELECT id, tenant_id, environment_id, provider, name,
			       is_default, status, last_used_at, last_validated_at,
			       validation_error, created_at, updated_at
			FROM cloud_credentials
			WHERE tenant_id = $1
			  AND status != $2
			ORDER BY provider, environment_id NULLS FIRST, is_default DESC, name
		`
		args = []interface{}{tenantID, StatusDeleted}
	}

	rows, err := s.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list credentials: %w", err)
	}
	defer rows.Close()

	var credentials []CredentialOutput
	for rows.Next() {
		var cred CredentialOutput
		err := rows.Scan(
			&cred.ID,
			&cred.TenantID,
			&cred.EnvironmentID,
			&cred.Provider,
			&cred.Name,
			&cred.IsDefault,
			&cred.Status,
			&cred.LastUsedAt,
			&cred.LastValidatedAt,
			&cred.ValidationError,
			&cred.CreatedAt,
			&cred.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan credential: %w", err)
		}
		credentials = append(credentials, cred)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating credentials: %w", err)
	}

	return credentials, nil
}

// UpdateCredential updates existing credentials
func (s *Service) UpdateCredential(ctx context.Context, credentialID uuid.UUID, tenantID uuid.UUID, credentials interface{}) error {
	// Get existing credential to verify ownership and get provider
	var provider string
	err := s.db.Pool.QueryRow(ctx,
		`SELECT provider FROM cloud_credentials WHERE id = $1 AND tenant_id = $2 AND status != $3`,
		credentialID, tenantID, StatusDeleted,
	).Scan(&provider)

	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("credential not found")
		}
		return fmt.Errorf("failed to get credential: %w", err)
	}

	// Validate new credentials structure
	if err := ValidateCredentialsStructure(provider, credentials); err != nil {
		return fmt.Errorf("invalid credentials: %w", err)
	}

	// Encrypt new credentials
	encryptedData, err := s.encryption.Encrypt(credentials)
	if err != nil {
		return fmt.Errorf("failed to encrypt credentials: %w", err)
	}

	// Update database
	query := `
		UPDATE cloud_credentials
		SET credentials_encrypted = $1,
		    encryption_key_id = $2,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $3 AND tenant_id = $4 AND status != $5
	`

	result, err := s.db.Pool.Exec(ctx, query,
		encryptedData,
		s.encryption.GetKeyID(),
		credentialID,
		tenantID,
		StatusDeleted,
	)

	if err != nil {
		return fmt.Errorf("failed to update credential: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("credential not found or already deleted")
	}

	s.logger.Info("updated cloud credential",
		zap.String("credential_id", credentialID.String()),
		zap.String("tenant_id", tenantID.String()),
	)

	return nil
}

// DeleteCredential performs a soft delete on credentials
func (s *Service) DeleteCredential(ctx context.Context, credentialID uuid.UUID, tenantID uuid.UUID) error {
	query := `
		UPDATE cloud_credentials
		SET status = $1,
		    deleted_at = CURRENT_TIMESTAMP,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $2 AND tenant_id = $3 AND status != $1
	`

	result, err := s.db.Pool.Exec(ctx, query, StatusDeleted, credentialID, tenantID)
	if err != nil {
		return fmt.Errorf("failed to delete credential: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("credential not found or already deleted")
	}

	s.logger.Info("deleted cloud credential",
		zap.String("credential_id", credentialID.String()),
		zap.String("tenant_id", tenantID.String()),
	)

	return nil
}

// ValidateCredential marks a credential as validated
func (s *Service) ValidateCredential(ctx context.Context, credentialID uuid.UUID, tenantID uuid.UUID, validationError *string) error {
	query := `
		UPDATE cloud_credentials
		SET last_validated_at = CURRENT_TIMESTAMP,
		    validation_error = $1,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $2 AND tenant_id = $3 AND status = $4
	`

	result, err := s.db.Pool.Exec(ctx, query, validationError, credentialID, tenantID, StatusActive)
	if err != nil {
		return fmt.Errorf("failed to validate credential: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("credential not found or not active")
	}

	if validationError != nil {
		s.logger.Warn("credential validation failed",
			zap.String("credential_id", credentialID.String()),
			zap.String("error", *validationError),
		)
	} else {
		s.logger.Info("credential validated successfully",
			zap.String("credential_id", credentialID.String()),
		)
	}

	return nil
}

// SetDefaultCredential sets a credential as the default for its tenant/provider
func (s *Service) SetDefaultCredential(ctx context.Context, credentialID uuid.UUID, tenantID uuid.UUID) error {
	// Get credential to check provider and environment
	var provider string
	var environmentID *uuid.UUID

	err := s.db.Pool.QueryRow(ctx,
		`SELECT provider, environment_id FROM cloud_credentials WHERE id = $1 AND tenant_id = $2 AND status = $3`,
		credentialID, tenantID, StatusActive,
	).Scan(&provider, &environmentID)

	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("credential not found or not active")
		}
		return fmt.Errorf("failed to get credential: %w", err)
	}

	// Start transaction
	tx, err := s.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Unset existing default
	if err := s.unsetDefaultCredentialTx(ctx, tx, tenantID, environmentID, provider); err != nil {
		return fmt.Errorf("failed to unset existing default: %w", err)
	}

	// Set new default
	query := `
		UPDATE cloud_credentials
		SET is_default = true,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND tenant_id = $2 AND status = $3
	`

	result, err := tx.Exec(ctx, query, credentialID, tenantID, StatusActive)
	if err != nil {
		return fmt.Errorf("failed to set default credential: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("credential not found or not active")
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.Info("set default cloud credential",
		zap.String("credential_id", credentialID.String()),
		zap.String("tenant_id", tenantID.String()),
		zap.String("provider", provider),
	)

	return nil
}

// unsetDefaultCredential unsets any existing default credential for the given tenant/environment/provider
func (s *Service) unsetDefaultCredential(ctx context.Context, tenantID uuid.UUID, environmentID *uuid.UUID, provider string) error {
	var query string
	var args []interface{}

	if environmentID != nil {
		query = `
			UPDATE cloud_credentials
			SET is_default = false, updated_at = CURRENT_TIMESTAMP
			WHERE tenant_id = $1 AND environment_id = $2 AND provider = $3 AND is_default = true AND status = $4
		`
		args = []interface{}{tenantID, environmentID, provider, StatusActive}
	} else {
		query = `
			UPDATE cloud_credentials
			SET is_default = false, updated_at = CURRENT_TIMESTAMP
			WHERE tenant_id = $1 AND environment_id IS NULL AND provider = $2 AND is_default = true AND status = $3
		`
		args = []interface{}{tenantID, provider, StatusActive}
	}

	_, err := s.db.Pool.Exec(ctx, query, args...)
	return err
}

// unsetDefaultCredentialTx is the transaction version of unsetDefaultCredential
func (s *Service) unsetDefaultCredentialTx(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, environmentID *uuid.UUID, provider string) error {
	var query string
	var args []interface{}

	if environmentID != nil {
		query = `
			UPDATE cloud_credentials
			SET is_default = false, updated_at = CURRENT_TIMESTAMP
			WHERE tenant_id = $1 AND environment_id = $2 AND provider = $3 AND is_default = true AND status = $4
		`
		args = []interface{}{tenantID, environmentID, provider, StatusActive}
	} else {
		query = `
			UPDATE cloud_credentials
			SET is_default = false, updated_at = CURRENT_TIMESTAMP
			WHERE tenant_id = $1 AND environment_id IS NULL AND provider = $2 AND is_default = true AND status = $3
		`
		args = []interface{}{tenantID, provider, StatusActive}
	}

	_, err := tx.Exec(ctx, query, args...)
	return err
}

// updateLastUsed updates the last_used_at timestamp for a credential
func (s *Service) updateLastUsed(ctx context.Context, credentialID uuid.UUID) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := s.db.Pool.Exec(ctx, `
		UPDATE cloud_credentials
		SET last_used_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`, credentialID)

	if err != nil {
		s.logger.Warn("failed to update last_used_at",
			zap.Error(err),
			zap.String("credential_id", credentialID.String()),
		)
	}
}

// GetCredentialFormatted retrieves and decrypts credentials in provider-specific format
func (s *Service) GetCredentialFormatted(ctx context.Context, credentialID uuid.UUID, tenantID uuid.UUID) (interface{}, error) {
	decrypted, err := s.GetCredential(ctx, credentialID, tenantID)
	if err != nil {
		return nil, err
	}

	// Convert to provider-specific struct
	return s.formatCredentials(decrypted.Provider, decrypted.DecryptedData)
}

// formatCredentials converts generic map to provider-specific struct
func (s *Service) formatCredentials(provider string, data interface{}) (interface{}, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal credentials: %w", err)
	}

	switch provider {
	case "aws":
		var creds AWSCredentials
		if err := json.Unmarshal(jsonData, &creds); err != nil {
			return nil, fmt.Errorf("failed to unmarshal AWS credentials: %w", err)
		}
		return creds, nil

	case "azure":
		var creds AzureCredentials
		if err := json.Unmarshal(jsonData, &creds); err != nil {
			return nil, fmt.Errorf("failed to unmarshal Azure credentials: %w", err)
		}
		return creds, nil

	case "gcp":
		var creds GCPCredentials
		if err := json.Unmarshal(jsonData, &creds); err != nil {
			return nil, fmt.Errorf("failed to unmarshal GCP credentials: %w", err)
		}
		return creds, nil

	case "lambda":
		var creds LambdaCredentials
		if err := json.Unmarshal(jsonData, &creds); err != nil {
			return nil, fmt.Errorf("failed to unmarshal Lambda credentials: %w", err)
		}
		return creds, nil

	case "runpod":
		var creds RunPodCredentials
		if err := json.Unmarshal(jsonData, &creds); err != nil {
			return nil, fmt.Errorf("failed to unmarshal RunPod credentials: %w", err)
		}
		return creds, nil

	case "oci":
		var creds OCICredentials
		if err := json.Unmarshal(jsonData, &creds); err != nil {
			return nil, fmt.Errorf("failed to unmarshal OCI credentials: %w", err)
		}
		return creds, nil

	case "nebius":
		var creds NebiusCredentials
		if err := json.Unmarshal(jsonData, &creds); err != nil {
			return nil, fmt.Errorf("failed to unmarshal Nebius credentials: %w", err)
		}
		return creds, nil

	default:
		return data, nil
	}
}
