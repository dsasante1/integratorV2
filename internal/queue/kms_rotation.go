package queue

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
)

const (
	QueueKMSRotation = "kms_rotation"
)

type KMSRotationPayload struct {
	KeyID string `json:"key_id"`
}


func ScheduleKMSRotation(keyID string) error {
	payload := KMSRotationPayload{
		KeyID: keyID,
	}

	
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %v", err)
	}

	
	task := asynq.NewTask(QueueKMSRotation, payloadBytes)

	// Schedule task for 3 months from now
	_, err = client.Enqueue(task,
		asynq.Queue(QueueKMSRotation),
		asynq.ProcessIn(3*30*24*time.Hour),
		asynq.MaxRetry(3),
		asynq.Timeout(10*time.Minute),
	)
	if err != nil {
		return fmt.Errorf("failed to enqueue KMS rotation task: %v", err)
	}

	return nil
}
