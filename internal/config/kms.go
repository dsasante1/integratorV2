package config

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"
)

var (
	KMSKeyID  string
	KMSClient *kms.Client
)

// InitKMS initializes the AWS KMS client and configuration
func InitKMS() error {
	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		slog.Error("Failed to load AWS SDK config", "error", err)
		return fmt.Errorf("unable to load AWS SDK config: %v", err)
	}

	// Create KMS client
	KMSClient = kms.NewFromConfig(cfg)

	// Get KMS key ID from environment
	KMSKeyID = os.Getenv("AWS_KMS_KEY_ID")
	if KMSKeyID == "" {
		slog.Error("Missing required environment variable", "variable", "AWS_KMS_KEY_ID")
		return fmt.Errorf("AWS_KMS_KEY_ID environment variable is required")
	}

	slog.Info("Successfully initialized AWS KMS client")
	return nil
}

// EncryptAPIKey encrypts an API key using AWS KMS
func EncryptAPIKey(apiKey string) (string, error) {
	if KMSClient == nil {
		slog.Error("KMS client not initialized")
		return "", fmt.Errorf("KMS client not initialized")
	}

	// Encrypt the API key
	input := &kms.EncryptInput{
		KeyId:     aws.String(KMSKeyID),
		Plaintext: []byte(apiKey),
	}

	result, err := KMSClient.Encrypt(context.TODO(), input)
	if err != nil {
		slog.Error("Failed to encrypt API key", "error", err)
		return "", fmt.Errorf("failed to encrypt API key: %v", err)
	}

	// Return base64 encoded ciphertext
	return base64.StdEncoding.EncodeToString(result.CiphertextBlob), nil
}

// DecryptAPIKey decrypts an API key using AWS KMS
func DecryptAPIKey(encryptedKey string) (string, error) {
	if KMSClient == nil {
		slog.Error("KMS client not initialized")
		return "", fmt.Errorf("KMS client not initialized")
	}

	// Decode base64 ciphertext
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedKey)
	if err != nil {
		slog.Error("Failed to decode encrypted key", "error", err)
		return "", fmt.Errorf("failed to decode encrypted key: %v", err)
	}

	// Decrypt the API key
	input := &kms.DecryptInput{
		CiphertextBlob: ciphertext,
	}

	result, err := KMSClient.Decrypt(context.TODO(), input)
	if err != nil {
		slog.Error("Failed to decrypt API key", "error", err)
		return "", fmt.Errorf("failed to decrypt API key: %v", err)
	}

	return string(result.Plaintext), nil
}
