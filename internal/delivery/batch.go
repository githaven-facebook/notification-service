package delivery

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/nicedavid98/notification-service/internal/model"
	"github.com/nicedavid98/notification-service/internal/repository"
)

// BatchKey identifies a unique user+channel batch bucket.
type BatchKey struct {
	UserID  string
	Channel model.NotificationChannel
}

// BatchEntry holds a pending notification awaiting digest delivery.
type BatchEntry struct {
	Notification *model.Notification
	QueuedAt     time.Time
}

// BatchProcessor collects notifications for digest mode and sends them at configured intervals.
type BatchProcessor struct {
	dispatcher    *Dispatcher
	notifRepo     repository.NotificationRepository
	queue         map[BatchKey][]*BatchEntry
	mu            sync.Mutex
	logger        *zap.Logger
	flushInterval time.Duration
	stopCh        chan struct{}
	wg            sync.WaitGroup
}

// NewBatchProcessor creates a new batch processor.
func NewBatchProcessor(
	dispatcher *Dispatcher,
	notifRepo repository.NotificationRepository,
	flushInterval time.Duration,
	logger *zap.Logger,
) *BatchProcessor {
	if flushInterval <= 0 {
		flushInterval = time.Hour
	}
	return &BatchProcessor{
		dispatcher:    dispatcher,
		notifRepo:     notifRepo,
		queue:         make(map[BatchKey][]*BatchEntry),
		logger:        logger,
		flushInterval: flushInterval,
		stopCh:        make(chan struct{}),
	}
}

// Enqueue adds a notification to the digest batch queue.
func (p *BatchProcessor) Enqueue(n *model.Notification) {
	key := BatchKey{
		UserID:  n.UserID,
		Channel: n.Channel,
	}
	p.mu.Lock()
	p.queue[key] = append(p.queue[key], &BatchEntry{
		Notification: n,
		QueuedAt:     time.Now(),
	})
	p.mu.Unlock()

	p.logger.Debug("Notification queued for batch delivery",
		zap.String("user_id", n.UserID),
		zap.String("channel", string(n.Channel)),
	)
}

// Start launches the background flush worker.
func (p *BatchProcessor) Start(ctx context.Context) {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		ticker := time.NewTicker(p.flushInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				p.flush(ctx)
			case <-p.stopCh:
				// Final flush before stopping
				p.flush(ctx)
				return
			case <-ctx.Done():
				return
			}
		}
	}()
	p.logger.Info("Batch processor started", zap.Duration("flush_interval", p.flushInterval))
}

// Stop gracefully stops the batch processor and waits for in-flight work.
func (p *BatchProcessor) Stop() {
	close(p.stopCh)
	p.wg.Wait()
	p.logger.Info("Batch processor stopped")
}

// flush drains the queue and sends aggregated notifications.
func (p *BatchProcessor) flush(ctx context.Context) {
	p.mu.Lock()
	if len(p.queue) == 0 {
		p.mu.Unlock()
		return
	}

	// Snapshot and clear the queue
	snapshot := p.queue
	p.queue = make(map[BatchKey][]*BatchEntry)
	p.mu.Unlock()

	for key, entries := range snapshot {
		if err := ctx.Err(); err != nil {
			return
		}
		p.sendBatch(ctx, key, entries)
	}
}

// sendBatch aggregates and sends a batch of notifications for a user+channel.
func (p *BatchProcessor) sendBatch(ctx context.Context, key BatchKey, entries []*BatchEntry) {
	if len(entries) == 0 {
		return
	}

	p.logger.Info("Sending batch digest",
		zap.String("user_id", key.UserID),
		zap.String("channel", string(key.Channel)),
		zap.Int("count", len(entries)),
	)

	// For digest mode, aggregate notification bodies
	aggregated := aggregateNotifications(entries)

	_, err := p.dispatcher.Dispatch(ctx, aggregated)
	if err != nil {
		p.logger.Error("Batch dispatch failed",
			zap.Error(err),
			zap.String("user_id", key.UserID),
			zap.String("channel", string(key.Channel)),
			zap.Int("count", len(entries)),
		)
	}
}

// aggregateNotifications combines multiple notifications into a single digest.
func aggregateNotifications(entries []*BatchEntry) *model.Notification {
	if len(entries) == 0 {
		return nil
	}

	first := entries[0].Notification
	if len(entries) == 1 {
		return first
	}

	// Build aggregate body
	body := fmt.Sprintf("You have %d new notifications:\n", len(entries))
	for i, e := range entries {
		if i >= 5 {
			body += fmt.Sprintf("... and %d more\n", len(entries)-5)
			break
		}
		body += fmt.Sprintf("• %s\n", e.Notification.Title)
	}

	return &model.Notification{
		ID:      first.ID,
		UserID:  first.UserID,
		Type:    first.Type,
		Channel: first.Channel,
		Title:   fmt.Sprintf("You have %d new notifications", len(entries)),
		Body:    body,
		Priority: model.PriorityNormal,
		Status:  model.StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}
