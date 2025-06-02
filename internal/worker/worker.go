package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"

	"github.com/hibiken/asynq"

	"integratorV2/internal/db"
	"integratorV2/internal/postman"
	"integratorV2/internal/queue"
)

type Worker struct {
	server *asynq.Server
}

func NewWorker() *Worker {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	redisOpt := asynq.RedisClientOpt{
		Addr: redisAddr,
	}

	server := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: 10,
			Queues: map[string]int{
				queue.QueueCollectionImport: 10,
			},
		},
	)

	return &Worker{
		server: server,
	}
}

func (w *Worker) Start(ctx context.Context) error {
	// Create a new mux
	mux := asynq.NewServeMux()

	// Register task handlers
	mux.HandleFunc(queue.QueueCollectionImport, w.handleCollectionImport)

	// Start server
	if err := w.server.Start(mux); err != nil {
		return err
	}

	// Wait for context cancellation
	<-ctx.Done()

	// Stop server
	w.server.Stop()
	return nil
}

func (w *Worker) handleCollectionImport(ctx context.Context, t *asynq.Task) error {
	var payload queue.CollectionImportPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return err
	}

	// Get API key for the user
	apiKey, err := db.GetPostmanAPIKey(payload.UserID)
	if err != nil {
		errMsg := "Failed to get API key"
		slog.Error(errMsg, "error", err, "user_id", payload.UserID)
		return err
	}

	// Get collection from Postman
	collection, err := postman.GetCollection(apiKey, payload.CollectionID)
	if err != nil {
		errMsg := "Failed to fetch collection from Postman"
		slog.Error(errMsg, "error", err, "user_id", payload.UserID, "collection_id", payload.CollectionID)
		return err
	}

	// Convert collection to JSON for storage
	content, err := json.Marshal(collection)
	if err != nil {
		errMsg := "Failed to process collection data"
		slog.Error(errMsg, "error", err, "user_id", payload.UserID, "collection_id", payload.CollectionID)
		return err
	}

	// Store collection and create snapshot with custom name
	if err := postman.StoreCollectionSnapshotWithName(payload.CollectionID, payload.Name, content); err != nil {
		errMsg := "Failed to store collection"
		slog.Error(errMsg, "error", err, "user_id", payload.UserID, "collection_id", payload.CollectionID)
		return err
	}

	slog.Info("Successfully processed collection import",
		"user_id", payload.UserID,
		"collection_id", payload.CollectionID,
		"name", payload.Name)

	return nil
}
