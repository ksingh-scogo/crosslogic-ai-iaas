package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/crosslogic/control-plane/pkg/cache"
	"github.com/crosslogic/control-plane/pkg/database"
	"github.com/crosslogic/control-plane/pkg/models"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Authenticator handles API key authentication
type Authenticator struct {
	db     *database.Database
	cache  *cache.Cache
	logger *zap.Logger
}

// NewAuthenticator creates a new authenticator
func NewAuthenticator(db *database.Database, cache *cache.Cache, logger *zap.Logger) *Authenticator {
	return &Authenticator{
		db:     db,
		cache:  cache,
		logger: logger,
	}
}

// ValidateAPIKey validates an API key and returns the key information
func (a *Authenticator) ValidateAPIKey(ctx context.Context, apiKey string) (*models.APIKey, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is empty")
	}

	// Hash the API key
	keyHash := hashAPIKey(apiKey)

	// Check cache first
	cacheKey := fmt.Sprintf("api_key:%s", keyHash)
	if cached, err := a.cache.Get(ctx, cacheKey); err == nil {
		var keyInfo models.APIKey
		if err := json.Unmarshal([]byte(cached), &keyInfo); err == nil {
			// Validate key is still active
			if keyInfo.Status == "active" {
				// Check expiration
				if keyInfo.ExpiresAt != nil && keyInfo.ExpiresAt.Before(time.Now()) {
					return nil, fmt.Errorf("API key has expired")
				}
				return &keyInfo, nil
			}
		}
	}

	// Query from database
	var keyInfo models.APIKey
	err := a.db.Pool.QueryRow(ctx, `
		SELECT
			k.id, k.key_hash, k.key_prefix, k.tenant_id, k.environment_id,
			k.user_id, k.name, k.role, k.rate_limit_tokens_per_min,
			k.rate_limit_requests_per_min, k.concurrency_limit, k.status,
			k.created_at, k.last_used_at, k.expires_at, k.metadata
		FROM api_keys k
		WHERE k.key_hash = $1
	`, keyHash).Scan(
		&keyInfo.ID,
		&keyInfo.KeyHash,
		&keyInfo.KeyPrefix,
		&keyInfo.TenantID,
		&keyInfo.EnvironmentID,
		&keyInfo.UserID,
		&keyInfo.Name,
		&keyInfo.Role,
		&keyInfo.RateLimitTokensPerMin,
		&keyInfo.RateLimitRequestsPerMin,
		&keyInfo.ConcurrencyLimit,
		&keyInfo.Status,
		&keyInfo.CreatedAt,
		&keyInfo.LastUsedAt,
		&keyInfo.ExpiresAt,
		&keyInfo.Metadata,
	)
	if err != nil {
		return nil, fmt.Errorf("API key not found")
	}

	// Validate status
	if keyInfo.Status != "active" {
		return nil, fmt.Errorf("API key is not active")
	}

	// Check expiration
	if keyInfo.ExpiresAt != nil && keyInfo.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("API key has expired")
	}

	// Validate tenant and environment status
	var tenantStatus, envStatus string
	err = a.db.Pool.QueryRow(ctx, `
		SELECT t.status, e.status
		FROM tenants t
		JOIN environments e ON e.tenant_id = t.id
		WHERE t.id = $1 AND e.id = $2
	`, keyInfo.TenantID, keyInfo.EnvironmentID).Scan(&tenantStatus, &envStatus)
	if err != nil {
		return nil, fmt.Errorf("tenant or environment not found")
	}

	if tenantStatus != "active" {
		return nil, fmt.Errorf("tenant is not active")
	}

	if envStatus != "active" {
		return nil, fmt.Errorf("environment is not active")
	}

	// Cache the key info for 60 seconds
	keyJSON, _ := json.Marshal(keyInfo)
	a.cache.Set(ctx, cacheKey, string(keyJSON), 60*time.Second)

	// Update last used timestamp (async)
	go a.updateLastUsed(keyInfo.ID)

	return &keyInfo, nil
}

// updateLastUsed updates the last_used_at timestamp for an API key
func (a *Authenticator) updateLastUsed(keyID uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := a.db.Pool.Exec(ctx, `
		UPDATE api_keys
		SET last_used_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`, keyID)
	if err != nil {
		a.logger.Warn("failed to update last_used_at", zap.Error(err), zap.String("key_id", keyID.String()))
	}
}

// hashAPIKey creates a SHA-256 hash of the API key
func hashAPIKey(apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(hash[:])
}

// GenerateAPIKey generates a new API key
func GenerateAPIKey(env string) string {
	// Format: clsk_{env}_{random}
	// Example: clsk_live_a4f5b2c8d9e0f1g2h3i4j5k6l7m8n9o0
	randomPart := uuid.New().String()
	randomPart = randomPart[:32] // Take first 32 chars
	return fmt.Sprintf("clsk_%s_%s", env, randomPart)
}

// CreateAPIKey creates a new API key in the database
func (a *Authenticator) CreateAPIKey(ctx context.Context, tenantID, environmentID uuid.UUID, name string) (string, error) {
	// Generate new API key
	apiKey := GenerateAPIKey("live")
	keyHash := hashAPIKey(apiKey)
	keyPrefix := apiKey[:12] // "clsk_live_xx"

	// Insert into database
	var keyID uuid.UUID
	err := a.db.Pool.QueryRow(ctx, `
		INSERT INTO api_keys (
			key_hash, key_prefix, tenant_id, environment_id,
			name, role, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`, keyHash, keyPrefix, tenantID, environmentID, name, "developer", "active").Scan(&keyID)
	if err != nil {
		return "", fmt.Errorf("failed to create API key: %w", err)
	}

	a.logger.Info("created new API key",
		zap.String("key_id", keyID.String()),
		zap.String("tenant_id", tenantID.String()),
		zap.String("environment_id", environmentID.String()),
	)

	return apiKey, nil
}

// RevokeAPIKey revokes an API key
func (a *Authenticator) RevokeAPIKey(ctx context.Context, keyID uuid.UUID) error {
	_, err := a.db.Pool.Exec(ctx, `
		UPDATE api_keys
		SET status = 'revoked'
		WHERE id = $1
	`, keyID)
	if err != nil {
		return fmt.Errorf("failed to revoke API key: %w", err)
	}

	// Invalidate cache
	// We don't know the key hash, so we'll rely on cache TTL
	// In production, maintain a reverse index

	a.logger.Info("revoked API key", zap.String("key_id", keyID.String()))
	return nil
}
