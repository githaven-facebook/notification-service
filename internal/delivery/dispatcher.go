package delivery

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/nicedavid98/notification-service/internal/model"
	"github.com/nicedavid98/notification-service/internal/provider"
	"github.com/nicedavid98/notification-service/internal/repository"
)

const (
	defaultDispatchTimeout = 30 * time.Second
)

// Dispatcher routes notifications to channel providers and tracks delivery results.
type Dispatcher struct {
	providers map[model.NotificationChannel]provider.Provider
	notifRepo repository.NotificationRepository
	logger    *zap.Logger
	timeout   time.Duration
}

// NewDispatcher creates a new notification dispatcher.
func NewDispatcher(
	providers map[model.NotificationChannel]provider.Provider,
	notifRepo repository.NotificationRepository,
	logger *zap.Logger,
) *Dispatcher {
	return &Dispatcher{
		providers: providers,
		notifRepo: notifRepo,
		logger:    logger,
		timeout:   defaultDispatchTimeout,
	}
}

// Dispatch sends a notification through the appropriate provider with a context timeout.
// It records the delivery result in the repository.
func (d *Dispatcher) Dispatch(ctx context.Context, n *model.Notification) (model.DeliveryResult, error) {
	prov, ok := d.providers[n.Channel]
	if !ok {
		err := fmt.Errorf("no provider configured for channel: %s", n.Channel)
		_ = d.notifRepo.UpdateStatus(ctx, n.ID, model.StatusFailed, err.Error())
		return model.DeliveryResult{Success: false, Error: err}, err
	}

	// Apply per-dispatch timeout
	dispatchCtx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	start := time.Now()
	result, err := prov.Send(dispatchCtx, n)
	elapsed := time.Since(start)

	if err != nil {
		d.logger.Error("Dispatch failed",
			zap.Error(err),
			zap.String("notification_id", n.ID.String()),
			zap.String("channel", string(n.Channel)),
			zap.String("provider", prov.Name()),
			zap.Duration("elapsed", elapsed),
		)

		_ = d.notifRepo.UpdateStatus(ctx, n.ID, model.StatusFailed, err.Error())
		return result, fmt.Errorf("dispatch via %s: %w", prov.Name(), err)
	}

	sentAt := time.Now()
	_ = d.notifRepo.UpdateSentAt(ctx, n.ID, sentAt)
	_ = d.notifRepo.UpdateStatus(ctx, n.ID, model.StatusSent, "")

	d.logger.Info("Dispatch succeeded",
		zap.String("notification_id", n.ID.String()),
		zap.String("message_id", result.MessageID),
		zap.String("channel", string(n.Channel)),
		zap.String("provider", prov.Name()),
		zap.Duration("elapsed", elapsed),
	)

	return result, nil
}

// DispatchWithResult dispatches and returns a DispatchResult with full metadata.
func (d *Dispatcher) DispatchWithResult(ctx context.Context, n *model.Notification) *DispatchResult {
	result, err := d.Dispatch(ctx, n)
	return &DispatchResult{
		NotificationID: n.ID.String(),
		DeliveryResult: result,
		Error:          err,
		DispatchedAt:   time.Now(),
	}
}

// DispatchResult holds the full result of a dispatch operation.
type DispatchResult struct {
	NotificationID string
	DeliveryResult model.DeliveryResult
	Error          error
	DispatchedAt   time.Time
}
