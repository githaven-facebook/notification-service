package consumer

import (
	"github.com/nicedavid98/notification-service/internal/model"
)

// EventType defines the type of Kafka event.
type EventType string

const (
	EventTypeNotification      EventType = "notification"
	EventTypeBatchNotification EventType = "batch_notification"
	EventTypePreferenceUpdate  EventType = "preference_update"
)

// NotificationEvent is the Kafka event for a single notification request.
type NotificationEvent struct {
	EventType      EventType                 `json:"event_type"`
	TraceID        string                    `json:"trace_id,omitempty"`
	UserID         string                    `json:"user_id"`
	Type           model.NotificationType    `json:"type"`
	Channel        model.NotificationChannel `json:"channel"`
	Title          string                    `json:"title,omitempty"`
	Body           string                    `json:"body,omitempty"`
	Data           map[string]string         `json:"data,omitempty"`
	Priority       model.Priority            `json:"priority"`
	TemplateID     string                    `json:"template_id,omitempty"`
	TemplateParams map[string]string         `json:"template_params,omitempty"`
	DeviceToken    string                    `json:"device_token,omitempty"`
	Recipient      string                    `json:"recipient,omitempty"`
	Locale         string                    `json:"locale,omitempty"`
}

// ToSendRequest converts a NotificationEvent to a model.SendRequest.
func (e *NotificationEvent) ToSendRequest() *model.SendRequest {
	priority := e.Priority
	if !priority.IsValid() {
		priority = model.PriorityNormal
	}
	return &model.SendRequest{
		UserID:         e.UserID,
		Type:           e.Type,
		Channel:        e.Channel,
		Title:          e.Title,
		Body:           e.Body,
		Data:           e.Data,
		Priority:       priority,
		TemplateID:     e.TemplateID,
		TemplateParams: e.TemplateParams,
		DeviceToken:    e.DeviceToken,
		Recipient:      e.Recipient,
		Locale:         e.Locale,
	}
}

// BatchNotificationEvent is the Kafka event for sending multiple notifications.
type BatchNotificationEvent struct {
	EventType     EventType           `json:"event_type"`
	TraceID       string              `json:"trace_id,omitempty"`
	Notifications []NotificationEvent `json:"notifications"`
}

// PreferenceUpdateEvent is the Kafka event for updating a user's notification preferences.
type PreferenceUpdateEvent struct {
	EventType       EventType                 `json:"event_type"`
	TraceID         string                    `json:"trace_id,omitempty"`
	UserID          string                    `json:"user_id"`
	Channel         model.NotificationChannel `json:"channel"`
	Enabled         bool                      `json:"enabled"`
	QuietHoursStart *string                   `json:"quiet_hours_start,omitempty"`
	QuietHoursEnd   *string                   `json:"quiet_hours_end,omitempty"`
	DigestMode      bool                      `json:"digest_mode"`
	Frequency       model.DigestFrequency     `json:"frequency,omitempty"`
}

// ToUpdateRequest converts a PreferenceUpdateEvent to a model.UpdatePreferenceRequest.
func (e *PreferenceUpdateEvent) ToUpdateRequest() *model.UpdatePreferenceRequest {
	return &model.UpdatePreferenceRequest{
		Enabled:         e.Enabled,
		QuietHoursStart: e.QuietHoursStart,
		QuietHoursEnd:   e.QuietHoursEnd,
		DigestMode:      e.DigestMode,
		Frequency:       e.Frequency,
	}
}
