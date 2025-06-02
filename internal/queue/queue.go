package queue

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/hibiken/asynq"
)

const (
	QueueCollectionImport = "collection_import"
)

type CollectionImportPayload struct {
	UserID       int64  `json:"user_id"`
	CollectionID string `json:"collection_id"`
	Name         string `json:"name"`
}

var (
	client    *asynq.Client
	inspector *asynq.Inspector
)

// InitQueue initializes the Redis connection for Asynq
func InitQueue() error {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	redisOpt := asynq.RedisClientOpt{
		Addr: redisAddr,
	}

	client = asynq.NewClient(redisOpt)
	inspector = asynq.NewInspector(redisOpt)

	// Test connection
	if err := client.Close(); err != nil {
		return fmt.Errorf("failed to connect to Redis: %v", err)
	}

	// Recreate client after test
	client = asynq.NewClient(redisOpt)

	slog.Info("Successfully initialized task queue")
	return nil
}

// EnqueueCollectionImport creates a new task to import a collection
func EnqueueCollectionImport(payload CollectionImportPayload) (string, error) {
	// Marshal payload
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %v", err)
	}

	// Create task
	task := asynq.NewTask(QueueCollectionImport, payloadBytes)

	// Enqueue task with retry options
	info, err := client.Enqueue(task,
		asynq.Queue(QueueCollectionImport),
		asynq.MaxRetry(3),
		asynq.Timeout(10*time.Minute),
		asynq.Retention(24*time.Hour),
	)
	if err != nil {
		return "", fmt.Errorf("failed to enqueue task: %v", err)
	}

	return info.ID, nil
}

// GetTaskStatus returns the current status of a task
func GetTaskStatus(taskID string) (*asynq.TaskInfo, error) {
	info, err := inspector.GetTaskInfo(QueueCollectionImport, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to get task info: %v", err)
	}
	return info, nil
}

// Close closes the Redis connection
func Close() error {
	if client != nil {
		return client.Close()
	}
	return nil
}
