package notification

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/google/uuid"
	"google.golang.org/api/iterator"

	"integratorV2/internal/config"
)

type NotificationService struct {
	db *firestore.Client
}

var NotificationServices *NotificationService

func NewNotificationService(firestoreDB *firestore.Client) *NotificationService {
	return &NotificationService{
		db: firestoreDB,
	}
}

func InitNotificationService() error {
	if config.FirebaseConnection == nil || config.FirebaseConnection.Firestore == nil {
		return errors.New("firebase connection not initialized. Call config.InitFireStore() first")
	}

	NotificationServices = NewNotificationService(config.FirebaseConnection.Firestore)
	slog.Info("Notification service initialized successfully")
	return nil
}

func GetNotificationService() *NotificationService {
	if NotificationServices == nil {
		slog.Error("Notification service not initialized. Call InitNotificationService() first.")
		return nil
	}
	return NotificationServices
}

func (s *NotificationService) SendNotification(ctx context.Context, req *NotificationRequest) (*Notification, error) {
	notification := &Notification{
		ID:        uuid.New().String(),
		UserID:    req.UserID,
		Type:      req.Type,
		Title:     req.Title,
		Message:   req.Message,
		Data:      req.Data,
		Read:      false,
		CreatedAt: time.Now(),
	}

	
	if req.TTL != nil {
		expiresAt := time.Now().Add(*req.TTL)
		notification.ExpiresAt = &expiresAt
	}

	_, err := s.db.Collection("notifications").Doc(notification.ID).Set(ctx, notification)
	if err != nil {
		return nil, fmt.Errorf("failed to send notification: %w", err)
	}

	if err := s.updateNotificationCount(ctx, req.UserID); err != nil {
		slog.Warn("failed to update notification count", "user_id", req.UserID, "error", err)
	}

	return notification, nil
}

func (s *NotificationService) GetNotifications(ctx context.Context, filter *NotificationFilter) ([]*Notification, error) {
	query := s.db.Collection("notifications").Where("user_id", "==", filter.UserID).OrderBy("created_at", firestore.Desc)

	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}

	iter := query.Documents(ctx)
	defer iter.Stop()

	var result []*Notification
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to get notifications: %w", err)
		}

		var notification Notification
		if err := doc.DataTo(&notification); err != nil {
			return nil, fmt.Errorf("failed to parse notification: %w", err)
		}

		if filter.Type != "" && notification.Type != filter.Type {
			continue
		}

		if filter.Read != nil && notification.Read != *filter.Read {
			continue
		}

		if notification.ExpiresAt != nil && time.Now().After(*notification.ExpiresAt) {
			continue
		}

		result = append(result, &notification)
	}

	return result, nil
}

func (s *NotificationService) MarkAsRead(ctx context.Context, userID, notificationID string) error {
	_, err := s.db.Collection("notifications").Doc(notificationID).Update(ctx, []firestore.Update{
		{Path: "read", Value: true},
	})
	if err != nil {
		return fmt.Errorf("failed to mark notification as read: %w", err)
	}

	if err := s.updateNotificationCount(ctx, userID); err != nil {
		slog.Warn("failed to update notification count", "user_id", userID, "error", err)
	}

	return nil
}

func (s *NotificationService) MarkAllAsRead(ctx context.Context, userID string) error {
	iter := s.db.Collection("notifications").Where("user_id", "==", userID).Where("read", "==", false).Documents(ctx)
	defer iter.Stop()

	bulkWriter := s.db.BulkWriter(ctx)
	defer bulkWriter.End()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to get notifications: %w", err)
		}

		_, err = bulkWriter.Update(doc.Ref, []firestore.Update{
			{Path: "read", Value: true},
		})
		if err != nil {
			return fmt.Errorf("failed to add update to bulk writer: %w", err)
		}
	}

	bulkWriter.Flush()

	if err := s.updateNotificationCount(ctx, userID); err != nil {
		slog.Warn("failed to update notification count", "user_id", userID, "error", err)
	}

	return nil
}

func (s *NotificationService) DeleteNotification(ctx context.Context, userID, notificationID string) error {
	_, err := s.db.Collection("notifications").Doc(notificationID).Delete(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete notification: %w", err)
	}

	if err := s.updateNotificationCount(ctx, userID); err != nil {
		slog.Warn("failed to update notification count", "user_id", userID, "error", err)
	}

	return nil
}

func (s *NotificationService) GetNotificationStats(ctx context.Context, userID string) (*NotificationStats, error) {
	query := s.db.Collection("notifications").Where("user_id", "==", userID)
	iter := query.Documents(ctx)
	defer iter.Stop()

	stats := &NotificationStats{}
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error occurred fetching notification stats: %w", err)
		}

		var notification Notification
		if err := doc.DataTo(&notification); err != nil {
			slog.Warn("failed to parse notification in stats", "doc_id", doc.Ref.ID, "error", err)
			continue
		}

		if notification.ExpiresAt != nil && time.Now().After(*notification.ExpiresAt) {
			continue
		}

		stats.Total++
		if !notification.Read {
			stats.Unread++
		}
	}

	return stats, nil
}

func (s *NotificationService) CleanupExpiredNotifications(ctx context.Context, userID string) error {
	now := time.Now()
	query := s.db.Collection("notifications").Where("user_id", "==", userID).Where("expires_at", "<=", now)

	iter := query.Documents(ctx)
	defer iter.Stop()

	
	bulkWriter := s.db.BulkWriter(ctx)
	defer bulkWriter.End()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to get expired notifications: %w", err)
		}

		_, err = bulkWriter.Delete(doc.Ref)
		if err != nil {
			return fmt.Errorf("failed to add delete to bulk writer: %w", err)
		}
	}

	bulkWriter.Flush()

	return nil
}

func (s *NotificationService) updateNotificationCount(ctx context.Context, userID string) error {
	stats, err := s.GetNotificationStats(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get notification stats: %w", err)
	}

	_, err = s.db.Collection("user_stats").Doc(userID).Set(ctx, map[string]interface{}{
		"notifications": stats,
	}, firestore.MergeAll)
	if err != nil {
		return fmt.Errorf("failed to update user stats: %w", err)
	}

	return nil
}
