package kms

import (
	"errors"
	"log/slog"
	"os"

	"integratorV2/internal/db"
	"integratorV2/internal/queue"
)

// InitRotation initializes the KMS key rotation
func InitRotation() error {
	// Get current key ID
	keyID := os.Getenv("AWS_KMS_KEY_ID")
	if keyID == "" {
		slog.Error("AWS_KMS_KEY_ID environment variable not set")
		return errors.New("AWS_KMS_KEY_ID environment variable not set")
	}

	// Check if rotation record exists
	var exists bool
	err := db.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM kms_key_rotation WHERE key_id = $1
		)
	`, keyID).Scan(&exists)
	if err != nil {
		slog.Error("failed to check rotation record", "error", err)
		return errors.New("failed to check rotation record")
	}

	if !exists {
		// Create rotation record
		_, err = db.DB.Exec(`
			INSERT INTO kms_key_rotation (key_id, last_rotated_at, next_rotation_at)
			VALUES ($1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP + INTERVAL '3 months')
		`, keyID)
		if err != nil {
			slog.Error("failed to create rotation record", "error", err)
			return errors.New("failed to create rotation record")
		}
		slog.Info("Created new KMS rotation record")
	}

	// Schedule next rotation
	if err := queue.ScheduleKMSRotation(keyID); err != nil {
		slog.Error("failed to schedule KMS rotation", "error", err)
		return errors.New("failed to schedule KMS rotation")
	}

	slog.Info("KMS rotation initialized successfully")
	return nil
}
