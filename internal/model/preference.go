package model

import "time"

// DigestFrequency defines how often digest notifications are sent.
type DigestFrequency string

const (
	DigestFrequencyHourly DigestFrequency = "hourly"
	DigestFrequencyDaily  DigestFrequency = "daily"
	DigestFrequencyWeekly DigestFrequency = "weekly"
)

// UserPreference represents a user's notification preferences for a channel.
type UserPreference struct {
	ID              int64               `json:"id" db:"id"`
	UserID          string              `json:"user_id" db:"user_id"`
	Channel         NotificationChannel `json:"channel" db:"channel"`
	Enabled         bool                `json:"enabled" db:"enabled"`
	QuietHoursStart *string             `json:"quiet_hours_start,omitempty" db:"quiet_hours_start"` // "22:00"
	QuietHoursEnd   *string             `json:"quiet_hours_end,omitempty" db:"quiet_hours_end"`     // "08:00"
	DigestMode      bool                `json:"digest_mode" db:"digest_mode"`
	Frequency       DigestFrequency     `json:"frequency,omitempty" db:"frequency"`
	CreatedAt       time.Time           `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at" db:"updated_at"`
}

// UpdatePreferenceRequest is the input DTO for updating a user preference.
type UpdatePreferenceRequest struct {
	Enabled         bool            `json:"enabled"`
	QuietHoursStart *string         `json:"quiet_hours_start,omitempty"`
	QuietHoursEnd   *string         `json:"quiet_hours_end,omitempty"`
	DigestMode      bool            `json:"digest_mode"`
	Frequency       DigestFrequency `json:"frequency,omitempty"`
}
