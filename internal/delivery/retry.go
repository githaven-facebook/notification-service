package delivery

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	"go.uber.org/zap"

	"github.com/nicedavid98/notification-service/internal/model"
	"github.com/nicedavid98/notification-service/internal/repository"
)

const (
	defaultMaxRetries  = 5
	defaultBaseDelay   = time.Second
	defaultMaxDelay    = 16 * time.Second
	defaultJitterRange = 0.3 // ±30% jitter
)

// RetryManager handles exponential backoff retries for failed notifications.
type RetryManager struct {
	dispatcher *Dispatcher
	notifRepo  repository.NotificationRepository
	maxRetries int
	baseDelay  time.Duration
	maxDelay   time.Duration
	logger     *zap.Logger
}

// RetryConfig configures the retry manager.
type RetryConfig struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
}

// NewRetryManager creates a new retry manager.
func NewRetryManager(dispatcher *Dispatcher, notifRepo repository.NotificationRepository, cfg RetryConfig, logger *zap.Logger) *RetryManager {
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = defaultMaxRetries
	}
	baseDelay := cfg.BaseDelay
	if baseDelay <= 0 {
		baseDelay = defaultBaseDelay
	}
	maxDelay := cfg.MaxDelay
	if maxDelay <= 0 {
		maxDelay = defaultMaxDelay
	}

	return &RetryManager{
		dispatcher: dispatcher,
		notifRepo:  notifRepo,
		maxRetries: maxRetries,
		baseDelay:  baseDelay,
		maxDelay:   maxDelay,
		logger:     logger,
	}
}

// RetryFailed fetches failed notifications and retries them with exponential backoff.
// This is called by a background worker periodically.
func (r *RetryManager) RetryFailed(ctx context.Context) error {
	notifications, err := r.notifRepo.GetPendingForRetry(ctx, r.maxRetries, 50)
	if err != nil {
		return err
	}

	for _, n := range notifications {
		if err := ctx.Err(); err != nil {
			return err
		}

		if err := r.RetryOne(ctx, n); err != nil {
			r.logger.Error("Retry failed",
				zap.Error(err),
				zap.String("notification_id", n.ID.String()),
				zap.Int("retry_count", n.RetryCount),
			)
		}
	}

	return nil
}

// RetryOne retries a single notification with exponential backoff delay.
func (r *RetryManager) RetryOne(ctx context.Context, n *model.Notification) error {
	if n.RetryCount >= r.maxRetries {
		r.logger.Warn("Notification exceeded max retries, sending to DLQ",
			zap.String("notification_id", n.ID.String()),
			zap.Int("retry_count", n.RetryCount),
		)
		return r.sendToDeadLetterQueue(ctx, n)
	}

	// Calculate backoff delay: base * 2^retryCount with jitter
	delay := r.calculateDelay(n.RetryCount)

	r.logger.Info("Retrying notification",
		zap.String("notification_id", n.ID.String()),
		zap.Int("retry_count", n.RetryCount),
		zap.Duration("delay", delay),
	)

	// Increment retry count before attempting
	if err := r.notifRepo.IncrementRetryCount(ctx, n.ID); err != nil {
		r.logger.Error("Failed to increment retry count", zap.Error(err))
	}

	select {
	case <-time.After(delay):
	case <-ctx.Done():
		return ctx.Err()
	}

	_, err := r.dispatcher.Dispatch(ctx, n)
	return err
}

// calculateDelay returns the backoff delay with jitter for the given retry attempt.
// Formula: min(base * 2^attempt, maxDelay) * (1 ± jitter)
func (r *RetryManager) calculateDelay(attempt int) time.Duration {
	exp := math.Pow(2, float64(attempt))
	delay := time.Duration(float64(r.baseDelay) * exp)

	if delay > r.maxDelay {
		delay = r.maxDelay
	}

	// Add jitter: ±30%
	jitter := 1.0 + (rand.Float64()*2-1)*defaultJitterRange //nolint:gosec
	delay = time.Duration(float64(delay) * jitter)

	if delay < 0 {
		delay = r.baseDelay
	}
	return delay
}

// sendToDeadLetterQueue marks a notification as permanently failed.
// In production, this would publish to a DLQ topic for manual review.
func (r *RetryManager) sendToDeadLetterQueue(ctx context.Context, n *model.Notification) error {
	r.logger.Error("Moving notification to dead letter queue",
		zap.String("notification_id", n.ID.String()),
		zap.String("user_id", n.UserID),
		zap.String("channel", string(n.Channel)),
		zap.Int("retry_count", n.RetryCount),
		zap.String("error", n.ErrorMessage),
	)

	// Update status to permanently failed (max retries exceeded)
	return r.notifRepo.UpdateStatus(ctx, n.ID, model.StatusFailed,
		fmt.Sprintf("max retries (%d) exceeded: %s", r.maxRetries, n.ErrorMessage))
}
