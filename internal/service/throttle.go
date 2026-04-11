package service

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/nicedavid98/notification-service/internal/config"
	"github.com/nicedavid98/notification-service/internal/model"
)

const (
	throttleKeyFormat = "throttle:%s:%s" // throttle:{userId}:{channel}
	throttleWindow    = time.Hour
)

// ThrottleService implements Redis-backed sliding window rate limiting per user.
type ThrottleService struct {
	redisClient *redis.Client
	cfg         config.ThrottleConfig
	logger      *zap.Logger
}

// NewThrottleService creates a new throttle service.
func NewThrottleService(redisClient *redis.Client, cfg config.ThrottleConfig, logger *zap.Logger) *ThrottleService {
	return &ThrottleService{
		redisClient: redisClient,
		cfg:         cfg,
		logger:      logger,
	}
}

// IsThrottled checks if a user has exceeded the rate limit for a given channel and priority.
// Returns true if the notification should be blocked.
func (s *ThrottleService) IsThrottled(ctx context.Context, userID string, channel model.NotificationChannel, priority model.Priority) (bool, error) {
	// High priority notifications bypass throttling
	if priority == model.PriorityHigh {
		return false, nil
	}

	limit := s.getLimitForChannel(channel)
	key := fmt.Sprintf(throttleKeyFormat, userID, string(channel))

	now := time.Now()
	windowStart := now.Add(-throttleWindow)

	pipe := s.redisClient.TxPipeline()

	// Sliding window: remove expired entries
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart.UnixMilli()))

	// Count current entries in window
	countCmd := pipe.ZCard(ctx, key)

	if _, err := pipe.Exec(ctx); err != nil {
		s.logger.Error("Throttle check pipeline failed", zap.Error(err), zap.String("user_id", userID))
		// Fail open: allow notification if Redis fails
		return false, fmt.Errorf("throttle check: %w", err)
	}

	count := countCmd.Val()
	if count >= int64(limit) {
		s.logger.Info("Notification throttled",
			zap.String("user_id", userID),
			zap.String("channel", string(channel)),
			zap.Int64("count", count),
			zap.Int("limit", limit),
		)
		return true, nil
	}
	return false, nil
}

// Record records a notification delivery attempt for throttle tracking.
func (s *ThrottleService) Record(ctx context.Context, userID string, channel model.NotificationChannel) error {
	key := fmt.Sprintf(throttleKeyFormat, userID, string(channel))
	now := time.Now()

	pipe := s.redisClient.TxPipeline()

	// Add current timestamp to sliding window set
	pipe.ZAdd(ctx, key, redis.Z{
		Score:  float64(now.UnixMilli()),
		Member: fmt.Sprintf("%d", now.UnixNano()),
	})

	// Set expiry slightly beyond the window
	pipe.Expire(ctx, key, throttleWindow+time.Minute)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("record throttle: %w", err)
	}
	return nil
}

// GetCurrentCount returns the current notification count for a user/channel in the window.
func (s *ThrottleService) GetCurrentCount(ctx context.Context, userID string, channel model.NotificationChannel) (int64, error) {
	key := fmt.Sprintf(throttleKeyFormat, userID, string(channel))
	windowStart := time.Now().Add(-throttleWindow)

	// Remove expired entries first
	if err := s.redisClient.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart.UnixMilli())).Err(); err != nil {
		return 0, fmt.Errorf("clean throttle window: %w", err)
	}

	count, err := s.redisClient.ZCard(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("get throttle count: %w", err)
	}
	return count, nil
}

// getLimitForChannel returns the rate limit for a given channel.
func (s *ThrottleService) getLimitForChannel(channel model.NotificationChannel) int {
	switch channel {
	case model.ChannelFCM, model.ChannelAPNS:
		return s.cfg.MaxPushPerHour
	case model.ChannelSES:
		return s.cfg.MaxEmailPerHour
	case model.ChannelSNS:
		return s.cfg.MaxSMSPerHour
	case model.ChannelInApp:
		return s.cfg.MaxInAppPerHour
	default:
		return s.cfg.MaxPerUserPerHour
	}
}
