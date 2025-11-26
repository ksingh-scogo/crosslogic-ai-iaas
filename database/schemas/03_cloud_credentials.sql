-- Cloud Provider Credentials Schema
-- Version: 1.0.0
-- Description: Secure storage of cloud provider credentials per tenant/environment

-- ============================================================================
-- CLOUD CREDENTIALS TABLE
-- ============================================================================
-- Stores encrypted cloud provider credentials for multi-tenant infrastructure management
-- Supports tenant-level (shared) and environment-level (isolated) credentials

CREATE TABLE cloud_credentials (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    environment_id UUID REFERENCES environments(id) ON DELETE CASCADE,

    -- Provider and naming
    provider VARCHAR(50) NOT NULL CHECK (provider IN ('aws', 'azure', 'gcp', 'lambda', 'runpod', 'oci', 'nebius')),
    name VARCHAR(255) NOT NULL,

    -- Encrypted credentials storage
    credentials_encrypted BYTEA NOT NULL,
    encryption_key_id VARCHAR(255) NOT NULL,

    -- Default credential management
    is_default BOOLEAN NOT NULL DEFAULT false,

    -- Status tracking
    status VARCHAR(50) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'deleted')),

    -- Usage and validation tracking
    last_used_at TIMESTAMP WITH TIME ZONE,
    last_validated_at TIMESTAMP WITH TIME ZONE,
    validation_error TEXT,

    -- Audit fields
    created_by_user_id UUID,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- ============================================================================
-- CONSTRAINTS
-- ============================================================================

-- Unique credential name per tenant/environment/provider combination
-- NULL environment_id means tenant-level (shared across all environments)
CREATE UNIQUE INDEX idx_cloud_credentials_unique_name
    ON cloud_credentials(tenant_id, COALESCE(environment_id::text, 'NULL'), provider, name)
    WHERE status != 'deleted';

-- Only one default credential per tenant/provider combination
-- This allows one default at tenant level and optionally per environment
CREATE UNIQUE INDEX idx_cloud_credentials_default_tenant
    ON cloud_credentials(tenant_id, provider)
    WHERE is_default = true AND environment_id IS NULL AND status = 'active';

-- Only one default credential per environment/provider combination
CREATE UNIQUE INDEX idx_cloud_credentials_default_environment
    ON cloud_credentials(tenant_id, environment_id, provider)
    WHERE is_default = true AND environment_id IS NOT NULL AND status = 'active';

-- ============================================================================
-- INDEXES
-- ============================================================================

-- Fast lookup by tenant + provider (most common query pattern)
CREATE INDEX idx_cloud_credentials_tenant_provider
    ON cloud_credentials(tenant_id, provider)
    WHERE status = 'active';

-- Fast lookup by environment + provider
CREATE INDEX idx_cloud_credentials_environment_provider
    ON cloud_credentials(environment_id, provider)
    WHERE status = 'active' AND environment_id IS NOT NULL;

-- Active credentials filtering
CREATE INDEX idx_cloud_credentials_status
    ON cloud_credentials(status);

-- Default credentials lookup
CREATE INDEX idx_cloud_credentials_is_default
    ON cloud_credentials(is_default)
    WHERE is_default = true AND status = 'active';

-- Track validation status
CREATE INDEX idx_cloud_credentials_last_validated
    ON cloud_credentials(last_validated_at DESC);

-- Tenant lookup for management
CREATE INDEX idx_cloud_credentials_tenant_id
    ON cloud_credentials(tenant_id);

-- Environment lookup
CREATE INDEX idx_cloud_credentials_environment_id
    ON cloud_credentials(environment_id)
    WHERE environment_id IS NOT NULL;

-- Provider filtering
CREATE INDEX idx_cloud_credentials_provider
    ON cloud_credentials(provider);

-- ============================================================================
-- TRIGGERS
-- ============================================================================

CREATE TRIGGER update_cloud_credentials_updated_at BEFORE UPDATE ON cloud_credentials
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- COMMENTS AND DOCUMENTATION
-- ============================================================================

COMMENT ON TABLE cloud_credentials IS 'Encrypted cloud provider credentials for tenant and environment-level access';
COMMENT ON COLUMN cloud_credentials.tenant_id IS 'Owner tenant of the credentials';
COMMENT ON COLUMN cloud_credentials.environment_id IS 'NULL for tenant-level (shared), or specific environment ID for isolated credentials';
COMMENT ON COLUMN cloud_credentials.provider IS 'Cloud provider: aws, azure, gcp, lambda, runpod, oci, nebius';
COMMENT ON COLUMN cloud_credentials.name IS 'Human-readable name for the credential set';
COMMENT ON COLUMN cloud_credentials.credentials_encrypted IS 'Encrypted JSON blob containing provider-specific credentials';
COMMENT ON COLUMN cloud_credentials.encryption_key_id IS 'Reference to encryption key used (for key rotation support)';
COMMENT ON COLUMN cloud_credentials.is_default IS 'Whether this is the default credential for this provider';
COMMENT ON COLUMN cloud_credentials.status IS 'Credential status: active, suspended, deleted';
COMMENT ON COLUMN cloud_credentials.last_used_at IS 'Last time these credentials were used for an operation';
COMMENT ON COLUMN cloud_credentials.last_validated_at IS 'Last time credentials were validated against the provider';
COMMENT ON COLUMN cloud_credentials.validation_error IS 'Latest validation error message if validation failed';
COMMENT ON COLUMN cloud_credentials.created_by_user_id IS 'User who created these credentials';

-- ============================================================================
-- CREDENTIAL JSON STRUCTURE EXAMPLES
-- ============================================================================
/*
The credentials_encrypted column contains a JSON blob specific to each provider.
Before encryption, the JSON structure should follow these patterns:

AWS Credentials:
{
  "access_key_id": "AKIAIOSFODNN7EXAMPLE",
  "secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
  "region": "us-east-1",
  "role_arn": "arn:aws:iam::123456789012:role/CrossLogicRole" // Optional
}

Azure Credentials:
{
  "client_id": "12345678-1234-1234-1234-123456789012",
  "client_secret": "your-client-secret-value",
  "tenant_id": "87654321-4321-4321-4321-210987654321",
  "subscription_id": "abcdef12-3456-7890-abcd-ef1234567890"
}

GCP Credentials:
{
  "service_account_json": {
    "type": "service_account",
    "project_id": "crosslogic-prod",
    "private_key_id": "key-id",
    "private_key": "-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----\n",
    "client_email": "service-account@crosslogic-prod.iam.gserviceaccount.com",
    "client_id": "123456789012345678901",
    "auth_uri": "https://accounts.google.com/o/oauth2/auth",
    "token_uri": "https://oauth2.googleapis.com/token",
    "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
    "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/..."
  },
  "project_id": "crosslogic-prod"
}

Lambda Labs Credentials:
{
  "api_key": "your-lambda-api-key",
  "endpoint": "https://cloud.lambdalabs.com/api/v1" // Optional custom endpoint
}

RunPod Credentials:
{
  "api_key": "your-runpod-api-key",
  "endpoint": "https://api.runpod.io/graphql" // Optional custom endpoint
}

Oracle Cloud (OCI) Credentials:
{
  "user_ocid": "ocid1.user.oc1...",
  "tenancy_ocid": "ocid1.tenancy.oc1...",
  "fingerprint": "aa:bb:cc:dd:ee:ff:00:11:22:33:44:55:66:77:88:99",
  "private_key": "-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----\n",
  "region": "us-ashburn-1"
}

Nebius Credentials:
{
  "api_key": "your-nebius-api-key",
  "project_id": "nebius-project-id",
  "endpoint": "https://api.nebius.com" // Optional custom endpoint
}
*/

-- ============================================================================
-- ENCRYPTION BEST PRACTICES
-- ============================================================================
/*
IMPORTANT: Credential encryption should be handled at the application layer.

Recommended approach:
1. Use envelope encryption (AWS KMS, GCP Cloud KMS, Azure Key Vault)
2. Store only the encrypted data in credentials_encrypted (BYTEA)
3. Store the key ID reference in encryption_key_id for key rotation
4. Never log decrypted credentials
5. Validate credentials immediately after decryption
6. Use short-lived credentials where possible (IAM roles, service accounts)
7. Implement regular key rotation (update encryption_key_id)
8. Audit all credential access via audit_logs table

Example encryption flow:
1. Generate data encryption key (DEK) from KMS
2. Encrypt credentials JSON with DEK (AES-256-GCM)
3. Store encrypted blob in credentials_encrypted
4. Store KMS key ID in encryption_key_id
5. Discard DEK from memory

Example decryption flow:
1. Retrieve encryption_key_id
2. Fetch DEK from KMS using key ID
3. Decrypt credentials_encrypted with DEK
4. Parse JSON and use credentials
5. Immediately discard decrypted credentials from memory
*/

-- ============================================================================
-- USAGE PATTERNS
-- ============================================================================
/*
Get default AWS credentials for a tenant:
  SELECT * FROM cloud_credentials
  WHERE tenant_id = $1
    AND provider = 'aws'
    AND is_default = true
    AND environment_id IS NULL
    AND status = 'active';

Get environment-specific credentials (with fallback to tenant-level):
  SELECT * FROM cloud_credentials
  WHERE tenant_id = $1
    AND provider = $2
    AND (environment_id = $3 OR environment_id IS NULL)
    AND status = 'active'
  ORDER BY environment_id DESC NULLS LAST, is_default DESC
  LIMIT 1;

List all active credentials for a tenant:
  SELECT id, name, provider, environment_id, is_default, last_validated_at
  FROM cloud_credentials
  WHERE tenant_id = $1
    AND status = 'active'
  ORDER BY provider, environment_id NULLS FIRST, is_default DESC;

Soft delete credentials:
  UPDATE cloud_credentials
  SET status = 'deleted', deleted_at = CURRENT_TIMESTAMP
  WHERE id = $1 AND tenant_id = $2;

Track credential usage:
  UPDATE cloud_credentials
  SET last_used_at = CURRENT_TIMESTAMP
  WHERE id = $1;

Record validation result:
  UPDATE cloud_credentials
  SET last_validated_at = CURRENT_TIMESTAMP,
      validation_error = NULL  -- or error message if failed
  WHERE id = $1;
*/

-- ============================================================================
-- SECURITY CONSIDERATIONS
-- ============================================================================
/*
1. Row-Level Security (RLS):
   Consider enabling RLS to ensure tenants can only access their own credentials:

   ALTER TABLE cloud_credentials ENABLE ROW LEVEL SECURITY;

   CREATE POLICY cloud_credentials_tenant_isolation ON cloud_credentials
     USING (tenant_id = current_setting('app.current_tenant_id')::uuid);

2. Audit Logging:
   All credential access should be logged to audit_logs table:

   INSERT INTO audit_logs (tenant_id, user_id, action, resource_type, resource_id, metadata)
   VALUES ($1, $2, 'credential.decrypt', 'cloud_credentials', $3,
           jsonb_build_object('provider', $4, 'environment_id', $5));

3. Access Control:
   - Limit database users with SELECT access to this table
   - Application layer should enforce role-based access (admin only)
   - Consider separate read-only replicas that exclude this table

4. Backup Security:
   - Ensure database backups are encrypted
   - Store backups in secure, access-controlled locations
   - Test credential recovery procedures regularly

5. Compliance:
   - GDPR: Credentials are pseudonymized via encryption
   - SOC 2: Audit logging covers all access
   - PCI DSS: If storing payment-related cloud access, ensure compliance
*/
