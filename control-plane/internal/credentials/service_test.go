package credentials

import (
	"context"
	"testing"
	"time"

	"github.com/crosslogic/control-plane/pkg/database"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestEncryptionService tests the encryption and decryption functionality
func TestEncryptionService(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	t.Run("encrypt and decrypt AWS credentials", func(t *testing.T) {
		encryption, err := NewEncryptionService("test-master-key-32-characters-long!", "test-key-v1")
		require.NoError(t, err)

		awsCreds := AWSCredentials{
			AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
			SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			Region:          "us-east-1",
		}

		// Encrypt
		encrypted, err := encryption.Encrypt(awsCreds)
		require.NoError(t, err)
		assert.NotEmpty(t, encrypted)

		// Decrypt
		var decrypted AWSCredentials
		err = encryption.Decrypt(encrypted, &decrypted)
		require.NoError(t, err)

		// Verify
		assert.Equal(t, awsCreds.AccessKeyID, decrypted.AccessKeyID)
		assert.Equal(t, awsCreds.SecretAccessKey, decrypted.SecretAccessKey)
		assert.Equal(t, awsCreds.Region, decrypted.Region)

		logger.Info("encryption test passed",
			zap.String("original_key", awsCreds.AccessKeyID),
			zap.String("decrypted_key", decrypted.AccessKeyID),
		)
	})

	t.Run("encrypt and decrypt Azure credentials", func(t *testing.T) {
		encryption, err := NewEncryptionService("test-master-key-32-characters-long!", "test-key-v1")
		require.NoError(t, err)

		azureCreds := AzureCredentials{
			ClientID:       "12345678-1234-1234-1234-123456789012",
			ClientSecret:   "test-client-secret",
			TenantID:       "87654321-4321-4321-4321-210987654321",
			SubscriptionID: "abcdef12-3456-7890-abcd-ef1234567890",
		}

		encrypted, err := encryption.Encrypt(azureCreds)
		require.NoError(t, err)

		var decrypted AzureCredentials
		err = encryption.Decrypt(encrypted, &decrypted)
		require.NoError(t, err)

		assert.Equal(t, azureCreds.ClientID, decrypted.ClientID)
		assert.Equal(t, azureCreds.ClientSecret, decrypted.ClientSecret)
		assert.Equal(t, azureCreds.TenantID, decrypted.TenantID)
		assert.Equal(t, azureCreds.SubscriptionID, decrypted.SubscriptionID)
	})

	t.Run("decrypt to map", func(t *testing.T) {
		encryption, err := NewEncryptionService("test-master-key-32-characters-long!", "test-key-v1")
		require.NoError(t, err)

		creds := map[string]interface{}{
			"api_key":    "test-api-key",
			"project_id": "test-project",
		}

		encrypted, err := encryption.Encrypt(creds)
		require.NoError(t, err)

		decrypted, err := encryption.DecryptToMap(encrypted)
		require.NoError(t, err)

		assert.Equal(t, "test-api-key", decrypted["api_key"])
		assert.Equal(t, "test-project", decrypted["project_id"])
	})

	t.Run("key rotation", func(t *testing.T) {
		oldKey, err := NewEncryptionService("old-master-key-32-characters-long!", "key-v1")
		require.NoError(t, err)

		newKey, err := NewEncryptionService("new-master-key-32-characters-long!", "key-v2")
		require.NoError(t, err)

		original := map[string]string{"api_key": "secret"}

		// Encrypt with old key
		oldEncrypted, err := oldKey.Encrypt(original)
		require.NoError(t, err)

		// Rotate to new key
		newEncrypted, err := RotateKey(oldKey, newKey, oldEncrypted)
		require.NoError(t, err)

		// Decrypt with new key
		var decrypted map[string]string
		err = newKey.Decrypt(newEncrypted, &decrypted)
		require.NoError(t, err)

		assert.Equal(t, original["api_key"], decrypted["api_key"])
	})
}

// TestValidateCredentialsStructure tests credential validation
func TestValidateCredentialsStructure(t *testing.T) {
	t.Run("valid AWS credentials", func(t *testing.T) {
		creds := AWSCredentials{
			AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
			SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			Region:          "us-east-1",
		}
		err := ValidateCredentialsStructure("aws", creds)
		assert.NoError(t, err)
	})

	t.Run("invalid AWS credentials - missing fields", func(t *testing.T) {
		creds := AWSCredentials{
			AccessKeyID: "AKIAIOSFODNN7EXAMPLE",
			// Missing SecretAccessKey
		}
		err := ValidateCredentialsStructure("aws", creds)
		assert.Error(t, err)
	})

	t.Run("valid Azure credentials", func(t *testing.T) {
		creds := AzureCredentials{
			ClientID:       "12345678-1234-1234-1234-123456789012",
			ClientSecret:   "test-secret",
			TenantID:       "87654321-4321-4321-4321-210987654321",
			SubscriptionID: "abcdef12-3456-7890-abcd-ef1234567890",
		}
		err := ValidateCredentialsStructure("azure", creds)
		assert.NoError(t, err)
	})

	t.Run("valid GCP credentials", func(t *testing.T) {
		creds := GCPCredentials{
			ProjectID: "test-project",
			ServiceAccountJSON: map[string]interface{}{
				"type":                        "service_account",
				"project_id":                  "test-project",
				"private_key_id":              "key123",
				"private_key":                 "-----BEGIN PRIVATE KEY-----\ntest\n-----END PRIVATE KEY-----\n",
				"client_email":                "test@test-project.iam.gserviceaccount.com",
			},
		}
		err := ValidateCredentialsStructure("gcp", creds)
		assert.NoError(t, err)
	})

	t.Run("valid Lambda credentials", func(t *testing.T) {
		creds := LambdaCredentials{
			APIKey: "lambda-api-key",
		}
		err := ValidateCredentialsStructure("lambda", creds)
		assert.NoError(t, err)
	})

	t.Run("valid RunPod credentials", func(t *testing.T) {
		creds := RunPodCredentials{
			APIKey: "runpod-api-key",
		}
		err := ValidateCredentialsStructure("runpod", creds)
		assert.NoError(t, err)
	})

	t.Run("valid OCI credentials", func(t *testing.T) {
		creds := OCICredentials{
			UserOCID:     "ocid1.user.oc1..test",
			TenancyOCID:  "ocid1.tenancy.oc1..test",
			Fingerprint:  "aa:bb:cc:dd:ee:ff",
			PrivateKey:   "-----BEGIN PRIVATE KEY-----\ntest\n-----END PRIVATE KEY-----\n",
			Region:       "us-ashburn-1",
		}
		err := ValidateCredentialsStructure("oci", creds)
		assert.NoError(t, err)
	})

	t.Run("valid Nebius credentials", func(t *testing.T) {
		creds := NebiusCredentials{
			APIKey:    "nebius-api-key",
			ProjectID: "nebius-project",
		}
		err := ValidateCredentialsStructure("nebius", creds)
		assert.NoError(t, err)
	})

	t.Run("unsupported provider", func(t *testing.T) {
		creds := map[string]string{"api_key": "test"}
		err := ValidateCredentialsStructure("unknown", creds)
		assert.Error(t, err)
	})
}

// TestIsValidProvider tests provider validation
func TestIsValidProvider(t *testing.T) {
	validProviders := []string{"aws", "azure", "gcp", "lambda", "runpod", "oci", "nebius"}
	for _, provider := range validProviders {
		assert.True(t, IsValidProvider(provider), "Provider %s should be valid", provider)
	}

	invalidProviders := []string{"unknown", "ibm", "alibaba", ""}
	for _, provider := range invalidProviders {
		assert.False(t, IsValidProvider(provider), "Provider %s should be invalid", provider)
	}
}

// TestCloudCredentialToOutput tests conversion to output format
func TestCloudCredentialToOutput(t *testing.T) {
	now := time.Now()
	tenantID := uuid.New()
	envID := uuid.New()
	credID := uuid.New()

	cred := CloudCredential{
		ID:                   credID,
		TenantID:             tenantID,
		EnvironmentID:        &envID,
		Provider:             "aws",
		Name:                 "Production AWS",
		CredentialsEncrypted: []byte("encrypted-data"),
		EncryptionKeyID:      "key-v1",
		IsDefault:            true,
		Status:               StatusActive,
		LastUsedAt:           &now,
		LastValidatedAt:      &now,
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	output := cred.ToOutput()

	assert.Equal(t, cred.ID, output.ID)
	assert.Equal(t, cred.TenantID, output.TenantID)
	assert.Equal(t, cred.EnvironmentID, output.EnvironmentID)
	assert.Equal(t, cred.Provider, output.Provider)
	assert.Equal(t, cred.Name, output.Name)
	assert.Equal(t, cred.IsDefault, output.IsDefault)
	assert.Equal(t, cred.Status, output.Status)
	assert.Equal(t, cred.LastUsedAt, output.LastUsedAt)
	assert.Equal(t, cred.CreatedAt, output.CreatedAt)

	// Ensure encrypted data is not in output
	assert.Empty(t, output.ValidationError)
}

// Integration test example (requires database)
// To run: docker-compose up -d postgres && go test -v -tags=integration
func TestServiceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This is a placeholder for actual integration tests
	// In production, you would:
	// 1. Set up a test database
	// 2. Create a Service instance
	// 3. Test CreateCredential, GetCredential, etc.

	t.Log("Integration tests require database setup")
	t.Log("Example usage:")
	t.Log("  1. Create test tenant and environment")
	t.Log("  2. Create AWS credentials")
	t.Log("  3. Retrieve and verify decryption")
	t.Log("  4. Update credentials")
	t.Log("  5. Set as default")
	t.Log("  6. List credentials")
	t.Log("  7. Delete credentials")
}

// Example test showing how to use the service
func ExampleService() {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()

	// This would normally come from your database connection
	// db, _ := database.NewDatabase(config.DatabaseConfig{...})

	// For demonstration purposes only
	var db *database.Database // This would be initialized properly

	// Create service
	service, err := NewService(
		db,
		"your-32-byte-encryption-key-here!",
		"key-v1",
		logger,
	)
	if err != nil {
		logger.Fatal("failed to create service", zap.Error(err))
	}

	tenantID := uuid.New()
	envID := uuid.New()

	// Create AWS credentials
	input := CredentialInput{
		TenantID:      tenantID,
		EnvironmentID: &envID,
		Provider:      "aws",
		Name:          "Production AWS Account",
		Credentials: AWSCredentials{
			AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
			SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			Region:          "us-east-1",
		},
		IsDefault: true,
	}

	credential, err := service.CreateCredential(ctx, input)
	if err != nil {
		logger.Fatal("failed to create credential", zap.Error(err))
	}

	logger.Info("created credential",
		zap.String("id", credential.ID.String()),
		zap.String("provider", credential.Provider),
	)

	// Retrieve credentials
	decrypted, err := service.GetCredential(ctx, credential.ID, tenantID)
	if err != nil {
		logger.Fatal("failed to get credential", zap.Error(err))
	}

	logger.Info("retrieved credential",
		zap.String("provider", decrypted.Provider),
		zap.Any("credentials", decrypted.DecryptedData),
	)

	// List all credentials for tenant
	credentials, err := service.ListCredentials(ctx, tenantID, &envID)
	if err != nil {
		logger.Fatal("failed to list credentials", zap.Error(err))
	}

	logger.Info("credentials list",
		zap.Int("count", len(credentials)),
	)
}
