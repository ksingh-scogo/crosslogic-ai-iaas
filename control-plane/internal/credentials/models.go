package credentials

import (
	"time"

	"github.com/google/uuid"
)

// CloudCredential represents stored cloud provider credentials
type CloudCredential struct {
	ID                   uuid.UUID  `json:"id" db:"id"`
	TenantID             uuid.UUID  `json:"tenant_id" db:"tenant_id"`
	EnvironmentID        *uuid.UUID `json:"environment_id,omitempty" db:"environment_id"`
	Provider             string     `json:"provider" db:"provider"`
	Name                 string     `json:"name" db:"name"`
	CredentialsEncrypted []byte     `json:"-" db:"credentials_encrypted"`
	EncryptionKeyID      string     `json:"encryption_key_id" db:"encryption_key_id"`
	IsDefault            bool       `json:"is_default" db:"is_default"`
	Status               string     `json:"status" db:"status"`
	LastUsedAt           *time.Time `json:"last_used_at,omitempty" db:"last_used_at"`
	LastValidatedAt      *time.Time `json:"last_validated_at,omitempty" db:"last_validated_at"`
	ValidationError      *string    `json:"validation_error,omitempty" db:"validation_error"`
	CreatedByUserID      *uuid.UUID `json:"created_by_user_id,omitempty" db:"created_by_user_id"`
	CreatedAt            time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt            *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
}

// CredentialInput represents the input for creating or updating credentials
type CredentialInput struct {
	TenantID        uuid.UUID               `json:"tenant_id"`
	EnvironmentID   *uuid.UUID              `json:"environment_id,omitempty"`
	Provider        string                  `json:"provider"`
	Name            string                  `json:"name"`
	Credentials     interface{}             `json:"credentials"` // Provider-specific credentials
	IsDefault       bool                    `json:"is_default"`
	CreatedByUserID *uuid.UUID              `json:"created_by_user_id,omitempty"`
}

// CredentialOutput represents the output when listing credentials (without decrypted data)
type CredentialOutput struct {
	ID              uuid.UUID  `json:"id"`
	TenantID        uuid.UUID  `json:"tenant_id"`
	EnvironmentID   *uuid.UUID `json:"environment_id,omitempty"`
	Provider        string     `json:"provider"`
	Name            string     `json:"name"`
	IsDefault       bool       `json:"is_default"`
	Status          string     `json:"status"`
	LastUsedAt      *time.Time `json:"last_used_at,omitempty"`
	LastValidatedAt *time.Time `json:"last_validated_at,omitempty"`
	ValidationError *string    `json:"validation_error,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// DecryptedCredential represents a credential with decrypted data
type DecryptedCredential struct {
	CloudCredential
	DecryptedData interface{} `json:"decrypted_data"`
}

// Provider-specific credential structures

// AWSCredentials contains AWS-specific credentials
type AWSCredentials struct {
	AccessKeyID     string  `json:"access_key_id"`
	SecretAccessKey string  `json:"secret_access_key"`
	Region          string  `json:"region,omitempty"`
	RoleArn         *string `json:"role_arn,omitempty"`
	SessionToken    *string `json:"session_token,omitempty"`
}

// AzureCredentials contains Azure-specific credentials
type AzureCredentials struct {
	ClientID       string `json:"client_id"`
	ClientSecret   string `json:"client_secret"`
	TenantID       string `json:"tenant_id"`
	SubscriptionID string `json:"subscription_id"`
}

// GCPCredentials contains GCP-specific credentials
type GCPCredentials struct {
	ProjectID          string                 `json:"project_id"`
	ServiceAccountJSON map[string]interface{} `json:"service_account_json"`
}

// LambdaCredentials contains Lambda Labs-specific credentials
type LambdaCredentials struct {
	APIKey   string  `json:"api_key"`
	Endpoint *string `json:"endpoint,omitempty"`
}

// RunPodCredentials contains RunPod-specific credentials
type RunPodCredentials struct {
	APIKey   string  `json:"api_key"`
	Endpoint *string `json:"endpoint,omitempty"`
}

// OCICredentials contains Oracle Cloud Infrastructure credentials
type OCICredentials struct {
	UserOCID     string `json:"user_ocid"`
	TenancyOCID  string `json:"tenancy_ocid"`
	Fingerprint  string `json:"fingerprint"`
	PrivateKey   string `json:"private_key"`
	Region       string `json:"region"`
}

// NebiusCredentials contains Nebius-specific credentials
type NebiusCredentials struct {
	APIKey    string  `json:"api_key"`
	ProjectID string  `json:"project_id"`
	Endpoint  *string `json:"endpoint,omitempty"`
}

// SupportedProviders lists all supported cloud providers
var SupportedProviders = []string{
	"aws",
	"azure",
	"gcp",
	"lambda",
	"runpod",
	"oci",
	"nebius",
}

// CredentialStatus represents possible credential statuses
const (
	StatusActive    = "active"
	StatusSuspended = "suspended"
	StatusDeleted   = "deleted"
)

// ToOutput converts a CloudCredential to CredentialOutput (safe for public exposure)
func (c *CloudCredential) ToOutput() CredentialOutput {
	return CredentialOutput{
		ID:              c.ID,
		TenantID:        c.TenantID,
		EnvironmentID:   c.EnvironmentID,
		Provider:        c.Provider,
		Name:            c.Name,
		IsDefault:       c.IsDefault,
		Status:          c.Status,
		LastUsedAt:      c.LastUsedAt,
		LastValidatedAt: c.LastValidatedAt,
		ValidationError: c.ValidationError,
		CreatedAt:       c.CreatedAt,
		UpdatedAt:       c.UpdatedAt,
	}
}

// IsValidProvider checks if the provider is supported
func IsValidProvider(provider string) bool {
	for _, p := range SupportedProviders {
		if p == provider {
			return true
		}
	}
	return false
}
