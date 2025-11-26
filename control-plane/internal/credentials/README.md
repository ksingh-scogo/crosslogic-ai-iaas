# Credential Management Service

A secure credential management service for storing and retrieving encrypted cloud provider credentials in the CrossLogic AI IaaS platform.

## Features

- **AES-256-GCM Encryption**: Industry-standard encryption for credential storage
- **Multi-Provider Support**: AWS, Azure, GCP, Lambda Labs, RunPod, Oracle Cloud, Nebius
- **Multi-Tenant**: Isolated credentials per tenant and environment
- **Default Credentials**: Set default credentials per provider
- **Key Rotation**: Support for encryption key rotation
- **Validation Tracking**: Track credential validation status and errors
- **Usage Tracking**: Monitor when credentials are last used
- **Soft Delete**: Credentials are soft-deleted for audit trails
- **Type-Safe**: Provider-specific credential structures

## Architecture

### Components

1. **Service** (`service.go`): Main credential management operations
2. **Encryption** (`encryption.go`): AES-256-GCM encryption/decryption with PBKDF2 key derivation
3. **Models** (`models.go`): Type-safe data structures for credentials

### Database Schema

Credentials are stored in the `cloud_credentials` table with the following structure:

```sql
CREATE TABLE cloud_credentials (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    environment_id UUID,  -- NULL for tenant-level credentials
    provider VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    credentials_encrypted BYTEA NOT NULL,
    encryption_key_id VARCHAR(255) NOT NULL,
    is_default BOOLEAN DEFAULT false,
    status VARCHAR(50) DEFAULT 'active',
    last_used_at TIMESTAMP,
    last_validated_at TIMESTAMP,
    validation_error TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

## Usage

### Initialize Service

```go
import (
    "github.com/crosslogic/control-plane/internal/credentials"
    "github.com/crosslogic/control-plane/pkg/database"
    "go.uber.org/zap"
)

// Create database connection
db, err := database.NewDatabase(config.DatabaseConfig{
    Host:     "localhost",
    Port:     5432,
    User:     "crosslogic",
    Password: "password",
    Database: "crosslogic_iaas",
})

// Initialize credential service
logger, _ := zap.NewProduction()
service, err := credentials.NewService(
    db,
    "your-32-byte-encryption-key-here!",
    "key-v1",
    logger,
)
```

### Create Credentials

#### AWS Credentials

```go
awsInput := credentials.CredentialInput{
    TenantID:      tenantID,
    EnvironmentID: &environmentID,
    Provider:      "aws",
    Name:          "Production AWS",
    Credentials: credentials.AWSCredentials{
        AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
        SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
        Region:          "us-east-1",
    },
    IsDefault: true,
}

credential, err := service.CreateCredential(ctx, awsInput)
```

#### Azure Credentials

```go
azureInput := credentials.CredentialInput{
    TenantID:      tenantID,
    EnvironmentID: &environmentID,
    Provider:      "azure",
    Name:          "Production Azure",
    Credentials: credentials.AzureCredentials{
        ClientID:       "12345678-1234-1234-1234-123456789012",
        ClientSecret:   "your-client-secret",
        TenantID:       "87654321-4321-4321-4321-210987654321",
        SubscriptionID: "abcdef12-3456-7890-abcd-ef1234567890",
    },
    IsDefault: true,
}

credential, err := service.CreateCredential(ctx, azureInput)
```

#### GCP Credentials

```go
gcpInput := credentials.CredentialInput{
    TenantID:      tenantID,
    EnvironmentID: &environmentID,
    Provider:      "gcp",
    Name:          "Production GCP",
    Credentials: credentials.GCPCredentials{
        ProjectID: "crosslogic-prod",
        ServiceAccountJSON: map[string]interface{}{
            "type":                        "service_account",
            "project_id":                  "crosslogic-prod",
            "private_key_id":              "key-id",
            "private_key":                 "-----BEGIN PRIVATE KEY-----\n...",
            "client_email":                "service@crosslogic-prod.iam.gserviceaccount.com",
            "client_id":                   "123456789012345678901",
        },
    },
    IsDefault: true,
}

credential, err := service.CreateCredential(ctx, gcpInput)
```

### Retrieve Credentials

#### Get by ID

```go
// Returns decrypted credentials
decrypted, err := service.GetCredential(ctx, credentialID, tenantID)
if err != nil {
    log.Fatal(err)
}

// Access decrypted data
fmt.Printf("Provider: %s\n", decrypted.Provider)
fmt.Printf("Credentials: %+v\n", decrypted.DecryptedData)
```

#### Get by Provider

```go
// Automatically selects best match (environment-specific or tenant-level)
decrypted, err := service.GetCredentialByProvider(ctx, tenantID, &environmentID, "aws")
if err != nil {
    log.Fatal(err)
}
```

#### Get Default Credential

```go
// Get tenant-level default credential
decrypted, err := service.GetDefaultCredential(ctx, tenantID, "aws")
if err != nil {
    log.Fatal(err)
}
```

#### Get Formatted Credentials

```go
// Returns provider-specific struct instead of generic map
creds, err := service.GetCredentialFormatted(ctx, credentialID, tenantID)
if err != nil {
    log.Fatal(err)
}

// Type assert to specific provider
awsCreds, ok := creds.(credentials.AWSCredentials)
if ok {
    fmt.Printf("Access Key: %s\n", awsCreds.AccessKeyID)
}
```

### List Credentials

```go
// List all credentials for tenant/environment (returns sanitized output)
credentials, err := service.ListCredentials(ctx, tenantID, &environmentID)
if err != nil {
    log.Fatal(err)
}

for _, cred := range credentials {
    fmt.Printf("ID: %s, Provider: %s, Name: %s, Default: %t\n",
        cred.ID, cred.Provider, cred.Name, cred.IsDefault)
}
```

### Update Credentials

```go
updatedCreds := credentials.AWSCredentials{
    AccessKeyID:     "AKIAIOSFODNN7NEWKEY",
    SecretAccessKey: "newSecretAccessKey",
    Region:          "us-west-2",
}

err := service.UpdateCredential(ctx, credentialID, tenantID, updatedCreds)
```

### Delete Credentials (Soft Delete)

```go
err := service.DeleteCredential(ctx, credentialID, tenantID)
```

### Set Default Credential

```go
err := service.SetDefaultCredential(ctx, credentialID, tenantID)
```

### Validate Credentials

```go
// Mark as successfully validated
err := service.ValidateCredential(ctx, credentialID, tenantID, nil)

// Mark as failed validation
validationError := "Invalid access key"
err := service.ValidateCredential(ctx, credentialID, tenantID, &validationError)
```

## Security Best Practices

### 1. Encryption Key Management

```go
// Use environment variables for encryption keys
encryptionKey := os.Getenv("CREDENTIAL_ENCRYPTION_KEY")
if len(encryptionKey) < 32 {
    log.Fatal("Encryption key must be at least 32 characters")
}

service, err := credentials.NewService(db, encryptionKey, "key-v1", logger)
```

### 2. Key Rotation

```go
// Create new encryption service with new key
newService, err := credentials.NewService(db, newKey, "key-v2", logger)

// Rotate existing credential
oldCred, _ := service.GetCredential(ctx, credID, tenantID)
newEncrypted, err := credentials.RotateKey(
    service.encryption,
    newService.encryption,
    oldCred.CredentialsEncrypted,
)

// Update in database
service.UpdateCredential(ctx, credID, tenantID, newEncrypted)
```

### 3. Audit Logging

```go
// Log all credential access
logger.Info("credential accessed",
    zap.String("credential_id", credentialID.String()),
    zap.String("tenant_id", tenantID.String()),
    zap.String("provider", provider),
    zap.String("user_id", userID.String()),
    zap.String("action", "decrypt"),
)
```

### 4. Never Log Decrypted Credentials

```go
// BAD - Don't do this
logger.Info("decrypted", zap.Any("creds", decryptedCreds))

// GOOD - Log metadata only
logger.Info("credential retrieved",
    zap.String("id", cred.ID.String()),
    zap.String("provider", cred.Provider),
)
```

### 5. Short-Lived Access

```go
// Decrypt credentials only when needed
decrypted, err := service.GetCredential(ctx, credID, tenantID)

// Use immediately
client := aws.NewClient(decrypted.DecryptedData)

// Discard from memory
decrypted = nil
```

## Supported Providers

| Provider | Type | Required Fields |
|----------|------|-----------------|
| AWS | `AWSCredentials` | `access_key_id`, `secret_access_key` |
| Azure | `AzureCredentials` | `client_id`, `client_secret`, `tenant_id`, `subscription_id` |
| GCP | `GCPCredentials` | `project_id`, `service_account_json` |
| Lambda Labs | `LambdaCredentials` | `api_key` |
| RunPod | `RunPodCredentials` | `api_key` |
| Oracle Cloud | `OCICredentials` | `user_ocid`, `tenancy_ocid`, `fingerprint`, `private_key`, `region` |
| Nebius | `NebiusCredentials` | `api_key`, `project_id` |

## Error Handling

```go
credential, err := service.GetCredential(ctx, credID, tenantID)
if err != nil {
    switch {
    case strings.Contains(err.Error(), "not found"):
        // Handle credential not found
        return fmt.Errorf("credential does not exist")
    case strings.Contains(err.Error(), "decrypt"):
        // Handle decryption failure
        return fmt.Errorf("failed to decrypt credential")
    case strings.Contains(err.Error(), "not active"):
        // Handle inactive credential
        return fmt.Errorf("credential is suspended or deleted")
    default:
        return fmt.Errorf("unexpected error: %w", err)
    }
}
```

## Testing

### Run Unit Tests

```bash
cd control-plane/internal/credentials
go test -v
```

### Run Integration Tests

```bash
# Start test database
docker-compose up -d postgres

# Run tests
go test -v -tags=integration
```

### Example Test

```go
func TestCredentialService(t *testing.T) {
    encryption, err := credentials.NewEncryptionService(
        "test-key-32-characters-long!!",
        "test-v1",
    )
    require.NoError(t, err)

    awsCreds := credentials.AWSCredentials{
        AccessKeyID:     "AKIATEST",
        SecretAccessKey: "secrettest",
        Region:          "us-east-1",
    }

    encrypted, err := encryption.Encrypt(awsCreds)
    require.NoError(t, err)

    var decrypted credentials.AWSCredentials
    err = encryption.Decrypt(encrypted, &decrypted)
    require.NoError(t, err)

    assert.Equal(t, awsCreds.AccessKeyID, decrypted.AccessKeyID)
}
```

## Performance Considerations

1. **Connection Pooling**: Database connections are pooled automatically
2. **Async Updates**: `last_used_at` updates are asynchronous to avoid blocking
3. **Encryption Overhead**: AES-256-GCM is fast (~1-2ms per operation)
4. **Index Usage**: Queries use optimized indexes on tenant_id, provider, environment_id

## Monitoring

```go
// Track credential usage
service.GetCredential(ctx, credID, tenantID) // Updates last_used_at automatically

// Monitor validation status
credentials, _ := service.ListCredentials(ctx, tenantID, nil)
for _, cred := range credentials {
    if cred.ValidationError != nil {
        logger.Warn("invalid credential",
            zap.String("id", cred.ID.String()),
            zap.String("error", *cred.ValidationError),
        )
    }
}
```

## Environment Variables

Required environment variables for production:

```bash
# Credential encryption key (minimum 32 characters)
CREDENTIAL_ENCRYPTION_KEY=your-secure-32-byte-key-here!

# Encryption key version (for key rotation)
CREDENTIAL_ENCRYPTION_KEY_ID=key-v1

# Database connection
DB_HOST=postgres
DB_PORT=5432
DB_USER=crosslogic
DB_PASSWORD=secure-password
DB_NAME=crosslogic_iaas
```

## Migration Guide

If migrating from file-based credentials or other storage:

1. Create encryption service
2. Read old credentials
3. Create new encrypted credentials
4. Validate new credentials
5. Update references to use new IDs
6. Remove old credential storage

## Support

For issues or questions, contact the CrossLogic platform team.
