package gateway

import (
	"errors"
	"time"

	"github.com/crosslogic-ai-iaas/control-plane/pkg/cache"
	"github.com/crosslogic-ai-iaas/control-plane/pkg/database"
	"github.com/crosslogic-ai-iaas/control-plane/pkg/models"
)

// Authenticator validates API keys and caches lookups.
type Authenticator struct {
	store database.Store
	cache *cache.LocalCache
}

func NewAuthenticator(store database.Store, cache *cache.LocalCache) *Authenticator {
	return &Authenticator{store: store, cache: cache}
}

// ValidateAPIKey checks the key against the store and caches the result.
func (a *Authenticator) ValidateAPIKey(key string) (models.APIKey, models.Tenant, error) {
	if key == "" {
		return models.APIKey{}, models.Tenant{}, errors.New("missing API key")
	}

	if cached, ok := a.cache.Get("apikey:" + key); ok {
		pair := cached.(cachedKey)
		return pair.key, pair.tenant, nil
	}

	apiKey, ok := a.store.FindAPIKey(key)
	if !ok {
		return models.APIKey{}, models.Tenant{}, errors.New("invalid API key")
	}
	tenant, ok := a.store.FindTenantByKey(key)
	if !ok {
		return models.APIKey{}, models.Tenant{}, errors.New("tenant not found")
	}

	a.cache.Set("apikey:"+key, cachedKey{key: apiKey, tenant: tenant}, 5*time.Minute)
	return apiKey, tenant, nil
}

type cachedKey struct {
	key    models.APIKey
	tenant models.Tenant
}
