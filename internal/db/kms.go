package db

import (
	"fmt"
	"time"
)

// InitKMSRotation initializes the KMS key rotation task
func InitKMSRotation(keyID string) error {
	// Check if rotation task already exists
	var exists bool
	err := DB.Get(&exists, `
		SELECT EXISTS (
			SELECT 1 FROM kms_key_rotation 
			WHERE key_id = $1
		)
	`, keyID)
	if err != nil {
		return fmt.Errorf("failed to check existing rotation task: %v", err)
	}

	if !exists {
		// Create new rotation task
		nextRotation := time.Now().Add(3 * 30 * 24 * time.Hour) // 3 months
		_, err = DB.Exec(`
			INSERT INTO kms_key_rotation (key_id, next_rotation_at)
			VALUES ($1, $2)
		`, keyID, nextRotation)
		if err != nil {
			return fmt.Errorf("failed to create rotation task: %v", err)
		}
	}

	return nil
} 