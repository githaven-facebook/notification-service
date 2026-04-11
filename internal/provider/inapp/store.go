package inapp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/nicedavid98/notification-service/internal/config"
	"github.com/nicedavid98/notification-service/internal/model"
	"github.com/nicedavid98/notification-service/internal/repository"
)

const (
	inAppTTL           = 30 * 24 * time.Hour // 30 days
	userInboxKeyFormat = "inapp:inbox:%s"
	maxInboxSize       = 100
)

// InAppProvider stores in-app notifications in the database and publishes them
// to Redis PubSub for real-time delivery.
type InAppProvider struct {
	repo        repository.NotificationRepository
	redisClient *redis.Client
	cfg         config.InAppConfig
	logger      *zap.Logger
}

// inAppEvent is the Redis PubSub event structure.
type inAppEvent struct {
	NotificationID string    `json:"notification_id"`
	UserID         string    `json:"user_id"`
	Title          string    `json:"title"`
	Body           string    `json:"body"`
	Data           map[string]string `json:"data,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// NewInAppProvider creates a new in-app notification provider.
func NewInAppProvider(repo repository.NotificationRepository, redisClient *redis.Client, cfg config.InAppConfig, logger *zap.Logger) *InAppProvider {
	return &InAppProvider{
		repo:        repo,
		redisClient: redisClient,
		cfg:         cfg,
		logger:      logger,
	}
}

// Name returns the provider name.
func (p *InAppProvider) Name() string {
	return "in_app"
}

// Send stores the notification and publishes to Redis PubSub for real-time delivery.
func (p *InAppProvider) Send(ctx context.Context, n *model.Notification) (model.DeliveryResult, error) {
	// Store in user's inbox using Redis sorted set (score = timestamp)
	inboxKey := fmt.Sprintf(userInboxKeyFormat, n.UserID)

	eventData, err := json.Marshal(inAppEvent{
		NotificationID: n.ID.String(),
		UserID:         n.UserID,
		Title:          n.Title,
		Body:           n.Body,
		Data:           n.Data,
		CreatedAt:      n.CreatedAt,
	})
	if err != nil {
		return model.DeliveryResult{}, fmt.Errorf("marshal in-app event: %w", err)
	}

	pipe := p.redisClient.TxPipeline()

	// Add to sorted set with timestamp as score
	pipe.ZAdd(ctx, inboxKey, redis.Z{
		Score:  float64(time.Now().UnixMilli()),
		Member: string(eventData),
	})

	// Trim to max inbox size (keep latest)
	pipe.ZRemRangeByRank(ctx, inboxKey, 0, -int64(maxInboxSize+1))

	// Set TTL on inbox key
	pipe.Expire(ctx, inboxKey, inAppTTL)

	if _, err := pipe.Exec(ctx); err != nil {
		p.logger.Error("Failed to store in-app notification in Redis",
			zap.Error(err),
			zap.String("user_id", n.UserID),
		)
		return model.DeliveryResult{}, fmt.Errorf("store in-app notification: %w", err)
	}

	// Publish to PubSub channel for real-time delivery
	channel := fmt.Sprintf("%s:%s", p.cfg.RedisChannel, n.UserID)
	if err := p.redisClient.Publish(ctx, channel, eventData).Err(); err != nil {
		// Non-fatal: notification is stored, just not real-time
		p.logger.Warn("Failed to publish in-app notification to PubSub",
			zap.Error(err),
			zap.String("channel", channel),
			zap.String("user_id", n.UserID),
		)
	}

	p.logger.Debug("In-app notification stored and published",
		zap.String("notification_id", n.ID.String()),
		zap.String("user_id", n.UserID),
	)

	return model.DeliveryResult{
		Success:   true,
		MessageID: n.ID.String(),
	}, nil
}

// GetUnread retrieves unread in-app notifications for a user from Redis.
func (p *InAppProvider) GetUnread(ctx context.Context, userID string, limit int) ([]inAppEvent, error) {
	inboxKey := fmt.Sprintf(userInboxKeyFormat, userID)

	results, err := p.redisClient.ZRevRangeByScore(ctx, inboxKey, &redis.ZRangeBy{
		Min:    "-inf",
		Max:    "+inf",
		Offset: 0,
		Count:  int64(limit),
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("get in-app notifications: %w", err)
	}

	events := make([]inAppEvent, 0, len(results))
	for _, raw := range results {
		var event inAppEvent
		if err := json.Unmarshal([]byte(raw), &event); err != nil {
			p.logger.Warn("Failed to unmarshal in-app event", zap.Error(err))
			continue
		}
		events = append(events, event)
	}
	return events, nil
}
