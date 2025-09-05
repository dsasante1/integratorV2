package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/hibiken/asynq"

	"integratorV2/internal/db"
	"integratorV2/internal/notification"
	"integratorV2/internal/postman"
	"integratorV2/internal/queue"
	"strconv"
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
				queue.QueueKMSRotation:      1,
			},
		},
	)

	return &Worker{
		server: server,
	}
}

func (w *Worker) Start(ctx context.Context) error {

	mux := asynq.NewServeMux()

	mux.HandleFunc(queue.QueueCollectionImport, w.handleCollectionImport)
	mux.HandleFunc(queue.QueueKMSRotation, w.HandleKMSRotation)

	slog.Info("Starting worker",
		"queues", []string{queue.QueueCollectionImport, queue.QueueKMSRotation},
		"concurrency", 10)

	
	if err := w.server.Start(mux); err != nil {
		return err
	}

	slog.Info("Worker started successfully")

	
	<-ctx.Done()

	
	w.server.Stop()
	slog.Info("Worker stopped")
	return nil
}

func (w *Worker) handleCollectionImport(ctx context.Context, t *asynq.Task) error {
	var payload queue.CollectionImportPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return err
	}

	userIDStr := strconv.Itoa(int(payload.UserID))

	apiKey, err := db.GetPostmanAPIKey(payload.UserID)
	if err != nil {
		errMsg := "Failed to get API key"

		notification.NotificationServices.SendNotification(ctx, &notification.NotificationRequest{
			UserID:  userIDStr,
			Type:    "fail",
			Title:   "fetch collection snapshot failed",
			Message: fmt.Sprintf("fetch collection snapshot failed '%s'", payload.Name),
		})
		slog.Error(errMsg, "error", err, "user_id", payload.UserID)
		return err
	}

	collection, err := postman.GetCollection(apiKey, payload.CollectionID)

	if err != nil {
		errMsg := "Failed to fetch collection from Postman"

		notification.NotificationServices.SendNotification(ctx, &notification.NotificationRequest{
			UserID:  userIDStr,
			Type:    "fail",
			Title:   "fetch collection snapshot failed",
			Message: fmt.Sprintf("fetch collection snapshot failed '%s'", payload.Name),
		})
		slog.Error(errMsg, "error", err, "user_id", payload.UserID, "collection_id", payload.CollectionID)
		return err
	}


	maskedCollection, err := postman.MaskCollection(collection)
	if err != nil {
		errMsg := "Failed to mask sensitive data in collection"

		notification.NotificationServices.SendNotification(ctx, &notification.NotificationRequest{
			UserID:  userIDStr,
			Type:    "fail",
			Title:   "fetch collection snapshot failed",
			Message: fmt.Sprintf("fetch collection snapshot data failed '%s'", payload.Name),
		})

		slog.Error(errMsg, "error", err, "user_id", payload.UserID, "collection_id", payload.CollectionID)
		return err
	}

	content, err := json.Marshal(maskedCollection)
	if err != nil {
		errMsg := "Failed to process collection data"

		notification.NotificationServices.SendNotification(ctx, &notification.NotificationRequest{
			UserID:  userIDStr,
			Type:    "fail",
			Title:   "fetch collection snapshot failed",
			Message: fmt.Sprintf("fetch collection snapshot data failed '%s'", payload.Name),
		})
		slog.Error(errMsg, "error", err, "user_id", payload.UserID, "collection_id", payload.CollectionID)
		return err
	}

	if err := postman.StoreCollectionSnapshotWithName(payload.CollectionID, payload.Name, content, payload.UserID); err != nil {
		errMsg := "Failed to store collection"

		notification.NotificationServices.SendNotification(ctx, &notification.NotificationRequest{
			UserID:  userIDStr,
			Type:    "fail",
			Title:   "store collection snapshot failed",
			Message: fmt.Sprintf("import collection failed '%s'", payload.Name),
		})
		slog.Error(errMsg, "error", err, "user_id", payload.UserID, "collection_id", payload.CollectionID)
		return err
	}

	slog.Info("Successfully processed collection import",
		"user_id", payload.UserID,
		"collection_id", payload.CollectionID,
		"name", payload.Name,
	)

	notification.NotificationServices.SendNotification(ctx, &notification.NotificationRequest{
		UserID:  userIDStr,
		Type:    "success",
		Title:   "Collection Import Successful",
		Message: fmt.Sprintf("Successfully imported collection '%s'", payload.Name),
	})

	return nil
}
