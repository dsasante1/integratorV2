package notification

import "time"

type NotificationType string

type Notification struct {
	ID        string                 `json:"id"`
	UserID    string                 `json:"user_id"`
	Type      NotificationType       `json:"type"`
	Title     string                 `json:"title"`
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Read      bool                   `json:"read"`
	CreatedAt time.Time              `json:"created_at"`
	ExpiresAt *time.Time             `json:"expires_at,omitempty"`
}

type NotificationRequest struct {
	UserID  string                 `json:"user_id"`
	Type    NotificationType       `json:"type"`
	Title   string                 `json:"title"`
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data,omitempty"`
	TTL     *time.Duration         `json:"ttl,omitempty"` 
}

type NotificationFilter struct {
	UserID string           `json:"user_id"`
	Type   NotificationType `json:"type,omitempty"`
	Read   *bool            `json:"read,omitempty"`
	Limit  int              `json:"limit,omitempty"`
}

type NotificationStats struct {
	Total  int `json:"total"`
	Unread int `json:"unread"`
}
