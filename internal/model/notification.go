package model

import (
	"time"

	"github.com/google/uuid"
)

// NotificationType defines the type of notification.
type NotificationType string

const (
	NotificationTypePush  NotificationType = "push"
	NotificationTypeEmail NotificationType = "email"
	NotificationTypeSMS   NotificationType = "sms"
	NotificationTypeInApp NotificationType = "in_app"
)

// NotificationChannel defines the delivery channel.
type NotificationChannel string

const (
	ChannelFCM   NotificationChannel = "fcm"
	ChannelAPNS  NotificationChannel = "apns"
	ChannelSES   NotificationChannel = "ses"
	ChannelSNS   NotificationChannel = "sns"
	ChannelInApp NotificationChannel = "in_app"
)

// Priority defines the notification priority level.
type Priority string

const (
	PriorityHigh   Priority = "high"
	PriorityNormal Priority = "normal"
	PriorityLow    Priority = "low"
)

// NotificationStatus defines the delivery status of a notification.
type NotificationStatus string

const (
	StatusPending   NotificationStatus = "pending"
	StatusSent      NotificationStatus = "sent"
	StatusDelivered NotificationStatus = "delivered"
	StatusFailed    NotificationStatus = "failed"
)

// Notification represents a notification entity.
type Notification struct {
	ID              uuid.UUID          `json:"id" db:"id"`
	UserID          string             `json:"user_id" db:"user_id"`
	Type            NotificationType   `json:"type" db:"type"`
	Channel         NotificationChannel `json:"channel" db:"channel"`
	Title           string             `json:"title" db:"title"`
	Body            string             `json:"body" db:"body"`
	Data            map[string]string  `json:"data,omitempty" db:"data"`
	Priority        Priority           `json:"priority" db:"priority"`
	Status          NotificationStatus `json:"status" db:"status"`
	TemplateID      string             `json:"template_id,omitempty" db:"template_id"`
	TemplateParams  map[string]string  `json:"template_params,omitempty" db:"template_params"`
	DeviceToken     string             `json:"device_token,omitempty" db:"device_token"`
	Recipient       string             `json:"recipient,omitempty" db:"recipient"` // email or phone number
	ErrorMessage    string             `json:"error_message,omitempty" db:"error_message"`
	RetryCount      int                `json:"retry_count" db:"retry_count"`
	DeduplicationKey string            `json:"deduplication_key,omitempty" db:"deduplication_key"`
	CreatedAt       time.Time          `json:"created_at" db:"created_at"`
	SentAt          *time.Time         `json:"sent_at,omitempty" db:"sent_at"`
	DeliveredAt     *time.Time         `json:"delivered_at,omitempty" db:"delivered_at"`
	UpdatedAt       time.Time          `json:"updated_at" db:"updated_at"`
}

// SendRequest is the input DTO for sending a notification.
type SendRequest struct {
	UserID         string             `json:"user_id"`
	Type           NotificationType   `json:"type"`
	Channel        NotificationChannel `json:"channel"`
	Title          string             `json:"title"`
	Body           string             `json:"body"`
	Data           map[string]string  `json:"data,omitempty"`
	Priority       Priority           `json:"priority"`
	TemplateID     string             `json:"template_id,omitempty"`
	TemplateParams map[string]string  `json:"template_params,omitempty"`
	DeviceToken    string             `json:"device_token,omitempty"`
	Recipient      string             `json:"recipient,omitempty"`
	Locale         string             `json:"locale,omitempty"`
}

// BatchSendRequest is the input DTO for batch sending notifications.
type BatchSendRequest struct {
	Notifications []SendRequest `json:"notifications"`
}

// DeliveryResult holds the result of a delivery attempt.
type DeliveryResult struct {
	Success    bool
	MessageID  string
	StatusCode int
	Error      error
}

// IsValid checks if the notification type is valid.
func (t NotificationType) IsValid() bool {
	switch t {
	case NotificationTypePush, NotificationTypeEmail, NotificationTypeSMS, NotificationTypeInApp:
		return true
	}
	return false
}

// IsValid checks if the notification channel is valid.
func (c NotificationChannel) IsValid() bool {
	switch c {
	case ChannelFCM, ChannelAPNS, ChannelSES, ChannelSNS, ChannelInApp:
		return true
	}
	return false
}

// IsValid checks if the priority is valid.
func (p Priority) IsValid() bool {
	switch p {
	case PriorityHigh, PriorityNormal, PriorityLow:
		return true
	}
	return false
}
