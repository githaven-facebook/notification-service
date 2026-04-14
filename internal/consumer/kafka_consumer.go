package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	"github.com/nicedavid98/notification-service/internal/config"
	"github.com/nicedavid98/notification-service/internal/service"
)

// KafkaConsumer consumes notification events from Kafka topics.
type KafkaConsumer struct {
	readers  []*kafka.Reader
	notifSvc *service.NotificationService
	prefSvc  *service.PreferenceService
	logger   *zap.Logger
	cfg      *config.KafkaConfig
}

// NewKafkaConsumer creates a new Kafka consumer for all configured topics.
func NewKafkaConsumer(
	cfg *config.KafkaConfig,
	notifSvc *service.NotificationService,
	prefSvc *service.PreferenceService,
	logger *zap.Logger,
) *KafkaConsumer {
	readers := make([]*kafka.Reader, 0, len(cfg.Topics))
	for _, topic := range cfg.Topics {
		reader := kafka.NewReader(kafka.ReaderConfig{
			Brokers:        cfg.Brokers,
			GroupID:        cfg.ConsumerGroup,
			Topic:          topic,
			MinBytes:       cfg.MinBytes,
			MaxBytes:       cfg.MaxBytes,
			MaxWait:        cfg.MaxWait,
			CommitInterval: time.Second,
			StartOffset:    kafka.LastOffset,
			ErrorLogger: kafka.LoggerFunc(func(msg string, args ...interface{}) {
				logger.Error(fmt.Sprintf(msg, args...))
			}),
		})
		readers = append(readers, reader)
	}

	return &KafkaConsumer{
		readers:  readers,
		notifSvc: notifSvc,
		prefSvc:  prefSvc,
		logger:   logger,
		cfg:      cfg,
	}
}

// Start launches a goroutine per topic reader and blocks until the context is canceled.
func (c *KafkaConsumer) Start(ctx context.Context) error {
	errCh := make(chan error, len(c.readers))

	for _, reader := range c.readers {
		go func(r *kafka.Reader) {
			c.logger.Info("Starting Kafka consumer",
				zap.String("topic", r.Config().Topic),
				zap.String("group", r.Config().GroupID),
			)
			if err := c.consume(ctx, r); err != nil {
				c.logger.Error("Kafka consumer error",
					zap.Error(err),
					zap.String("topic", r.Config().Topic),
				)
				errCh <- err
			}
		}(reader)
	}

	select {
	case <-ctx.Done():
		return c.Close()
	case err := <-errCh:
		_ = c.Close()
		return err
	}
}

// consume reads messages from a single Kafka reader and processes them.
func (c *KafkaConsumer) consume(ctx context.Context, reader *kafka.Reader) error {
	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil // context canceled, normal shutdown
			}
			c.logger.Error("Fetch Kafka message failed",
				zap.Error(err),
				zap.String("topic", reader.Config().Topic),
			)
			return fmt.Errorf("fetch kafka message: %w", err)
		}

		if err := c.processMessage(ctx, &msg); err != nil {
			c.logger.Error("Process Kafka message failed",
				zap.Error(err),
				zap.String("topic", msg.Topic),
				zap.Int64("offset", msg.Offset),
				zap.Int("partition", msg.Partition),
			)
			// Continue processing even if one message fails
		}

		// Commit offset after processing (even if processing failed, to avoid infinite retry loops)
		if err := reader.CommitMessages(ctx, msg); err != nil {
			c.logger.Error("Commit Kafka offset failed",
				zap.Error(err),
				zap.Int64("offset", msg.Offset),
			)
		}
	}
}

// processMessage deserializes and routes a Kafka message to the appropriate handler.
func (c *KafkaConsumer) processMessage(ctx context.Context, msg *kafka.Message) error {
	// Detect event type from header or payload
	eventTypeHeader := getHeader(msg.Headers, "event_type")

	switch EventType(eventTypeHeader) {
	case EventTypeNotification:
		return c.handleNotification(ctx, msg.Value)
	case EventTypeBatchNotification:
		return c.handleBatchNotification(ctx, msg.Value)
	case EventTypePreferenceUpdate:
		return c.handlePreferenceUpdate(ctx, msg.Value)
	default:
		return c.handleNotification(ctx, msg.Value)
	}
}

// handleNotification processes a single notification event.
func (c *KafkaConsumer) handleNotification(ctx context.Context, data []byte) error {
	var event NotificationEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("unmarshal notification event: %w", err)
	}

	req := event.ToSendRequest()
	_, err := c.notifSvc.Send(ctx, req)
	if err != nil {
		c.logger.Error("Send notification from Kafka failed",
			zap.Error(err),
			zap.String("user_id", event.UserID),
			zap.String("trace_id", event.TraceID),
		)
		return fmt.Errorf("send notification: %w", err)
	}

	c.logger.Debug("Notification event processed",
		zap.String("user_id", event.UserID),
		zap.String("trace_id", event.TraceID),
	)
	return nil
}

// handleBatchNotification processes a batch notification event.
func (c *KafkaConsumer) handleBatchNotification(ctx context.Context, data []byte) error {
	var event BatchNotificationEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("unmarshal batch notification event: %w", err)
	}

	for i := range event.Notifications {
		notifEvent := &event.Notifications[i]
		req := notifEvent.ToSendRequest()
		if _, err := c.notifSvc.Send(ctx, req); err != nil {
			c.logger.Error("Send batch notification from Kafka failed",
				zap.Error(err),
				zap.String("user_id", notifEvent.UserID),
				zap.String("trace_id", event.TraceID),
			)
		}
	}
	return nil
}

// handlePreferenceUpdate processes a preference update event.
func (c *KafkaConsumer) handlePreferenceUpdate(ctx context.Context, data []byte) error {
	var event PreferenceUpdateEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("unmarshal preference update event: %w", err)
	}

	req := event.ToUpdateRequest()
	if _, err := c.prefSvc.UpdatePreference(ctx, event.UserID, event.Channel, req); err != nil {
		return fmt.Errorf("update preference from Kafka: %w", err)
	}

	c.logger.Debug("Preference update event processed",
		zap.String("user_id", event.UserID),
		zap.String("channel", string(event.Channel)),
		zap.String("trace_id", event.TraceID),
	)
	return nil
}

// Close closes all Kafka readers.
func (c *KafkaConsumer) Close() error {
	var lastErr error
	for _, reader := range c.readers {
		if err := reader.Close(); err != nil {
			c.logger.Error("Failed to close Kafka reader",
				zap.Error(err),
				zap.String("topic", reader.Config().Topic),
			)
			lastErr = err
		}
	}
	return lastErr
}

// getHeader extracts a header value from a Kafka message.
func getHeader(headers []kafka.Header, key string) string {
	for _, h := range headers {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}
