package model

import "time"

// NotificationTemplate represents a notification template stored in the database.
type NotificationTemplate struct {
	ID        int64               `json:"id" db:"id"`
	Name      string              `json:"name" db:"name"`
	Channel   NotificationChannel `json:"channel" db:"channel"`
	Subject   string              `json:"subject" db:"subject"`
	Body      string              `json:"body" db:"body"`
	Locale    string              `json:"locale" db:"locale"` // e.g., "en", "ko", "ko-KR"
	Version   int                 `json:"version" db:"version"`
	Active    bool                `json:"active" db:"active"`
	CreatedAt time.Time           `json:"created_at" db:"created_at"`
	UpdatedAt time.Time           `json:"updated_at" db:"updated_at"`
}

// CreateTemplateRequest is the input DTO for creating a template.
type CreateTemplateRequest struct {
	Name    string              `json:"name"`
	Channel NotificationChannel `json:"channel"`
	Subject string              `json:"subject"`
	Body    string              `json:"body"`
	Locale  string              `json:"locale"`
	Version int                 `json:"version"`
}

// UpdateTemplateRequest is the input DTO for updating a template.
type UpdateTemplateRequest struct {
	Subject string `json:"subject,omitempty"`
	Body    string `json:"body,omitempty"`
	Active  *bool  `json:"active,omitempty"`
}
