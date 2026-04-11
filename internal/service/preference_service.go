package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/nicedavid98/notification-service/internal/model"
	"github.com/nicedavid98/notification-service/internal/repository"
)

// PreferenceService manages user notification preferences.
type PreferenceService struct {
	repo   repository.PreferenceRepository
	logger *zap.Logger
}

// NewPreferenceService creates a new preference service.
func NewPreferenceService(repo repository.PreferenceRepository, logger *zap.Logger) *PreferenceService {
	return &PreferenceService{
		repo:   repo,
		logger: logger,
	}
}

// GetPreferences returns all preferences for a user.
func (s *PreferenceService) GetPreferences(ctx context.Context, userID string) ([]*model.UserPreference, error) {
	prefs, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user preferences: %w", err)
	}
	return prefs, nil
}

// GetChannelPreference returns the preference for a specific channel.
// Returns a default enabled preference if none is set.
func (s *PreferenceService) GetChannelPreference(ctx context.Context, userID string, channel model.NotificationChannel) (*model.UserPreference, error) {
	pref, err := s.repo.GetByUserAndChannel(ctx, userID, channel)
	if err != nil {
		return nil, fmt.Errorf("get channel preference: %w", err)
	}

	// Default: enabled if no preference is set
	if pref == nil {
		return &model.UserPreference{
			UserID:  userID,
			Channel: channel,
			Enabled: true,
		}, nil
	}
	return pref, nil
}

// UpdatePreference creates or updates a user's preference for a channel.
func (s *PreferenceService) UpdatePreference(ctx context.Context, userID string, channel model.NotificationChannel, req *model.UpdatePreferenceRequest) (*model.UserPreference, error) {
	pref := &model.UserPreference{
		UserID:          userID,
		Channel:         channel,
		Enabled:         req.Enabled,
		QuietHoursStart: req.QuietHoursStart,
		QuietHoursEnd:   req.QuietHoursEnd,
		DigestMode:      req.DigestMode,
		Frequency:       req.Frequency,
	}

	if pref.Frequency == "" {
		pref.Frequency = model.DigestFrequencyDaily
	}

	if err := s.repo.Upsert(ctx, pref); err != nil {
		return nil, fmt.Errorf("update preference: %w", err)
	}

	s.logger.Info("User preference updated",
		zap.String("user_id", userID),
		zap.String("channel", string(channel)),
		zap.Bool("enabled", req.Enabled),
	)

	return pref, nil
}

// IsChannelEnabled returns true if the given channel is enabled for the user.
func (s *PreferenceService) IsChannelEnabled(ctx context.Context, userID string, channel model.NotificationChannel) (bool, error) {
	pref, err := s.GetChannelPreference(ctx, userID, channel)
	if err != nil {
		return false, err
	}
	return pref.Enabled, nil
}

// IsInQuietHours returns true if the current time falls within the user's quiet hours.
func (s *PreferenceService) IsInQuietHours(ctx context.Context, userID string, channel model.NotificationChannel) (bool, error) {
	pref, err := s.GetChannelPreference(ctx, userID, channel)
	if err != nil {
		return false, err
	}

	if pref.QuietHoursStart == nil || pref.QuietHoursEnd == nil {
		return false, nil
	}

	return isInQuietHours(*pref.QuietHoursStart, *pref.QuietHoursEnd), nil
}

// IsDigestMode returns true if the user has digest mode enabled for the channel.
func (s *PreferenceService) IsDigestMode(ctx context.Context, userID string, channel model.NotificationChannel) (bool, error) {
	pref, err := s.GetChannelPreference(ctx, userID, channel)
	if err != nil {
		return false, err
	}
	return pref.DigestMode, nil
}

// isInQuietHours checks if the current time is within the quiet hours range.
// Supports overnight ranges (e.g., 22:00 - 08:00).
func isInQuietHours(start, end string) bool {
	now := time.Now()
	currentMinutes := now.Hour()*60 + now.Minute()

	startMinutes, err := parseTimeString(start)
	if err != nil {
		return false
	}
	endMinutes, err := parseTimeString(end)
	if err != nil {
		return false
	}

	if startMinutes <= endMinutes {
		// Same-day range: e.g., 09:00 - 17:00
		return currentMinutes >= startMinutes && currentMinutes < endMinutes
	}

	// Overnight range: e.g., 22:00 - 08:00
	return currentMinutes >= startMinutes || currentMinutes < endMinutes
}

// parseTimeString parses a "HH:MM" time string and returns total minutes.
func parseTimeString(t string) (int, error) {
	parts := strings.Split(t, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid time format: %s", t)
	}
	hours, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid hours in time: %s", t)
	}
	minutes, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("invalid minutes in time: %s", t)
	}
	return hours*60 + minutes, nil
}
