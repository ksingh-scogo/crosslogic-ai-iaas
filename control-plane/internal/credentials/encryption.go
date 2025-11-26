package credentials

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

// EncryptionService handles encryption and decryption of credentials
type EncryptionService struct {
	masterKey []byte
	keyID     string
}

// NewEncryptionService creates a new encryption service
// masterKey should be a 32-byte key for AES-256
func NewEncryptionService(masterKey string, keyID string) (*EncryptionService, error) {
	if len(masterKey) == 0 {
		return nil, fmt.Errorf("master key cannot be empty")
	}

	// Derive a 32-byte key from the master key using PBKDF2
	derivedKey := pbkdf2.Key([]byte(masterKey), []byte("crosslogic-credentials-salt"), 100000, 32, sha256.New)

	return &EncryptionService{
		masterKey: derivedKey,
		keyID:     keyID,
	}, nil
}

// Encrypt encrypts credentials using AES-256-GCM
// The input should be a struct that can be marshaled to JSON
func (e *EncryptionService) Encrypt(credentials interface{}) ([]byte, error) {
	// Marshal credentials to JSON
	plaintext, err := json.Marshal(credentials)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal credentials: %w", err)
	}

	// Create AES cipher block
	block, err := aes.NewCipher(e.masterKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate a random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt the data
	// The nonce is prepended to the ciphertext
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	return ciphertext, nil
}

// Decrypt decrypts credentials using AES-256-GCM
// The output is unmarshaled into the provided interface
func (e *EncryptionService) Decrypt(ciphertext []byte, output interface{}) error {
	if len(ciphertext) == 0 {
		return fmt.Errorf("ciphertext is empty")
	}

	// Create AES cipher block
	block, err := aes.NewCipher(e.masterKey)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}

	// Extract nonce from the beginning of ciphertext
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt the data
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return fmt.Errorf("failed to decrypt: %w", err)
	}

	// Unmarshal JSON into output
	if err := json.Unmarshal(plaintext, output); err != nil {
		return fmt.Errorf("failed to unmarshal decrypted data: %w", err)
	}

	return nil
}

// DecryptToMap decrypts credentials to a generic map (useful when provider type is unknown)
func (e *EncryptionService) DecryptToMap(ciphertext []byte) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := e.Decrypt(ciphertext, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetKeyID returns the key ID used for this encryption service
func (e *EncryptionService) GetKeyID() string {
	return e.keyID
}

// RotateKey creates a new encryption service with a new key
// This is used for key rotation - decrypt with old key, re-encrypt with new key
func RotateKey(oldService, newService *EncryptionService, oldCiphertext []byte) ([]byte, error) {
	// Decrypt with old key
	var credentials map[string]interface{}
	if err := oldService.Decrypt(oldCiphertext, &credentials); err != nil {
		return nil, fmt.Errorf("failed to decrypt with old key: %w", err)
	}

	// Re-encrypt with new key
	newCiphertext, err := newService.Encrypt(credentials)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt with new key: %w", err)
	}

	return newCiphertext, nil
}

// ValidateCredentialsStructure validates that the credentials match the expected structure for the provider
func ValidateCredentialsStructure(provider string, credentials interface{}) error {
	// Convert to JSON and back to validate structure
	jsonData, err := json.Marshal(credentials)
	if err != nil {
		return fmt.Errorf("invalid credentials structure: %w", err)
	}

	switch provider {
	case "aws":
		var aws AWSCredentials
		if err := json.Unmarshal(jsonData, &aws); err != nil {
			return fmt.Errorf("invalid AWS credentials structure: %w", err)
		}
		if aws.AccessKeyID == "" || aws.SecretAccessKey == "" {
			return fmt.Errorf("AWS credentials must include access_key_id and secret_access_key")
		}

	case "azure":
		var azure AzureCredentials
		if err := json.Unmarshal(jsonData, &azure); err != nil {
			return fmt.Errorf("invalid Azure credentials structure: %w", err)
		}
		if azure.ClientID == "" || azure.ClientSecret == "" || azure.TenantID == "" || azure.SubscriptionID == "" {
			return fmt.Errorf("Azure credentials must include client_id, client_secret, tenant_id, and subscription_id")
		}

	case "gcp":
		var gcp GCPCredentials
		if err := json.Unmarshal(jsonData, &gcp); err != nil {
			return fmt.Errorf("invalid GCP credentials structure: %w", err)
		}
		if gcp.ProjectID == "" || gcp.ServiceAccountJSON == nil {
			return fmt.Errorf("GCP credentials must include project_id and service_account_json")
		}

	case "lambda":
		var lambda LambdaCredentials
		if err := json.Unmarshal(jsonData, &lambda); err != nil {
			return fmt.Errorf("invalid Lambda credentials structure: %w", err)
		}
		if lambda.APIKey == "" {
			return fmt.Errorf("Lambda credentials must include api_key")
		}

	case "runpod":
		var runpod RunPodCredentials
		if err := json.Unmarshal(jsonData, &runpod); err != nil {
			return fmt.Errorf("invalid RunPod credentials structure: %w", err)
		}
		if runpod.APIKey == "" {
			return fmt.Errorf("RunPod credentials must include api_key")
		}

	case "oci":
		var oci OCICredentials
		if err := json.Unmarshal(jsonData, &oci); err != nil {
			return fmt.Errorf("invalid OCI credentials structure: %w", err)
		}
		if oci.UserOCID == "" || oci.TenancyOCID == "" || oci.Fingerprint == "" || oci.PrivateKey == "" {
			return fmt.Errorf("OCI credentials must include user_ocid, tenancy_ocid, fingerprint, and private_key")
		}

	case "nebius":
		var nebius NebiusCredentials
		if err := json.Unmarshal(jsonData, &nebius); err != nil {
			return fmt.Errorf("invalid Nebius credentials structure: %w", err)
		}
		if nebius.APIKey == "" || nebius.ProjectID == "" {
			return fmt.Errorf("Nebius credentials must include api_key and project_id")
		}

	default:
		return fmt.Errorf("unsupported provider: %s", provider)
	}

	return nil
}
