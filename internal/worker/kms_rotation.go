package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"integratorV2/internal/config"
	"integratorV2/internal/db"
	"integratorV2/internal/queue"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/hibiken/asynq"
)


func (w *Worker) HandleKMSRotation(ctx context.Context, t *asynq.Task) error {
	var payload queue.KMSRotationPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %v", err)
	}

	
	input := &kms.CreateKeyInput{
		Description: aws.String("Auto-rotated KMS key"),
		Tags: []types.Tag{
			{
				TagKey:   aws.String("AutoRotated"),
				TagValue: aws.String("true"),
			},
		},
	}

	result, err := config.KMSClient.CreateKey(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to create new KMS key: %v", err)
	}

	
	if err := os.Setenv("AWS_KMS_KEY_ID", *result.KeyMetadata.KeyId); err != nil {
		return fmt.Errorf("failed to update KMS key ID: %v", err)
	}

	
	nextRotation := time.Now().Add(3 * 30 * 24 * time.Hour) // 3 months
	_, err = db.DB.Exec(`
		UPDATE kms_key_rotation
		SET last_rotated_at = CURRENT_TIMESTAMP,
			next_rotation_at = $1,
			updated_at = CURRENT_TIMESTAMP
		WHERE key_id = $2
	`, nextRotation, payload.KeyID)
	if err != nil {
		return fmt.Errorf("failed to update rotation record: %v", err)
	}

	
	if err := queue.ScheduleKMSRotation(*result.KeyMetadata.KeyId); err != nil {
		return fmt.Errorf("failed to schedule next rotation: %v", err)
	}

	slog.Info("Successfully rotated KMS key",
		"old_key_id", payload.KeyID,
		"new_key_id", *result.KeyMetadata.KeyId)

	return nil
}
