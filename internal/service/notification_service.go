package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/nicedavid98/notification-service/internal/model"
	"github.com/nicedavid98/notification-service/internal/provider"
	"github.com/nicedavid98/notification-service/internal/repository"
	tmpl "github.com/nicedavid98/notification-service/internal/template"
)

const (
	dedupKeyFormat = "dedup:%s"
)

// NotificationService orchestrates the full notification send flow.
type NotificationService struct {
	notifRepo   repository.NotificationRepository
	prefService *PreferenceService
	throttle    *ThrottleService
	renderer    *tmpl.Renderer
	providers   map[model.NotificationChannel]provider.Provider
	redisClient *redis.Client
	dedupTTL    time.Duration
	logger      *zap.Logger
}

// NotificationServiceConfig holds the dependencies for NotificationService.
type NotificationServiceConfig struct {
	NotifRepo   repository.NotificationRepository
	PrefService *PreferenceService
	Throttle    *ThrottleService
	Renderer    *tmpl.Renderer
	Providers   map[model.NotificationChannel]provider.Provider
	RedisClient *redis.Client
	DedupTTL    time.Duration
	Logger      *zap.Logger
}

// NewNotificationService creates a new notification service.
func NewNotificationService(cfg NotificationServiceConfig) *NotificationService {
	dedupTTL := cfg.DedupTTL
	if dedupTTL == 0 {
		dedupTTL = 5 * time.Minute
	}
	return &NotificationService{
		notifRepo:   cfg.NotifRepo,
		prefService: cfg.PrefService,
		throttle:    cfg.Throttle,
		renderer:    cfg.Renderer,
		providers:   cfg.Providers,
		redisClient: cfg.RedisClient,
		dedupTTL:    dedupTTL,
		logger:      cfg.Logger,
	}
}

// Send validates, deduplicates, and delivers a single notification.
func (s *NotificationService) Send(ctx context.Context, req *model.SendRequest) (*model.Notification, error) {
	// Validate request
	if err := validateSendRequest(req); err != nil {
		return nil, fmt.Errorf("invalid send request: %w", err)
	}

	// Check user preferences
	enabled, err := s.prefService.IsChannelEnabled(ctx, req.UserID, req.Channel)
	if err != nil {
		s.logger.Warn("Failed to check channel preference, proceeding",
			zap.Error(err),
			zap.String("user_id", req.UserID),
		)
	} else if !enabled {
		return nil, fmt.Errorf("channel %s is disabled for user %s", req.Channel, req.UserID)
	}

	// Check quiet hours (skip for high priority)
	if req.Priority != model.PriorityHigh {
		inQuiet, err := s.prefService.IsInQuietHours(ctx, req.UserID, req.Channel)
		if err != nil {
			s.logger.Warn("Failed to check quiet hours, proceeding", zap.Error(err))
		} else if inQuiet {
			return nil, fmt.Errorf("user %s is in quiet hours for channel %s", req.UserID, req.Channel)
		}
	}

	// Check throttle limits
	throttled, err := s.throttle.IsThrottled(ctx, req.UserID, req.Channel, req.Priority)
	if err != nil {
		s.logger.Warn("Throttle check failed, proceeding", zap.Error(err))
	} else if throttled {
		return nil, fmt.Errorf("notification throttled for user %s on channel %s", req.UserID, req.Channel)
	}

	// Render template if specified
	rendered, err := s.renderer.RenderNotification(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("render notification: %w", err)
	}

	// Build notification entity
	n := &model.Notification{
		ID:             uuid.New(),
		UserID:         req.UserID,
		Type:           req.Type,
		Channel:        req.Channel,
		Title:          rendered.Subject,
		Body:           rendered.Body,
		Data:           req.Data,
		Priority:       req.Priority,
		Status:         model.StatusPending,
		TemplateID:     req.TemplateID,
		TemplateParams: req.TemplateParams,
		DeviceToken:    req.DeviceToken,
		Recipient:      req.Recipient,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Check deduplication
	dedupKey := buildDedupKey(req)
	n.DeduplicationKey = dedupKey

	isDup, err := s.checkAndSetDedup(ctx, dedupKey)
	if err != nil {
		s.logger.Warn("Dedup check failed, proceeding", zap.Error(err))
	} else if isDup {
		s.logger.Info("Duplicate notification suppressed",
			zap.String("dedup_key", dedupKey),
			zap.String("user_id", req.UserID),
		)
		return nil, fmt.Errorf("duplicate notification suppressed")
	}

	// Persist notification
	if err := s.notifRepo.Create(ctx, n); err != nil {
		return nil, fmt.Errorf("persist notification: %w", err)
	}

	// Dispatch to provider
	prov, ok := s.providers[req.Channel]
	if !ok {
		_ = s.notifRepo.UpdateStatus(ctx, n.ID, model.StatusFailed, fmt.Sprintf("no provider for channel %s", req.Channel))
		return nil, fmt.Errorf("no provider configured for channel %s", req.Channel)
	}

	result, err := prov.Send(ctx, n)
	if err != nil {
		errMsg := err.Error()
		_ = s.notifRepo.UpdateStatus(ctx, n.ID, model.StatusFailed, errMsg)
		n.Status = model.StatusFailed
		n.ErrorMessage = errMsg
		s.logger.Error("Notification send failed",
			zap.Error(err),
			zap.String("notification_id", n.ID.String()),
			zap.String("user_id", req.UserID),
			zap.String("channel", string(req.Channel)),
		)
		return n, fmt.Errorf("send notification via %s: %w", req.Channel, err)
	}

	// Update status to sent
	sentAt := time.Now()
	_ = s.notifRepo.UpdateSentAt(ctx, n.ID, sentAt)
	_ = s.notifRepo.UpdateStatus(ctx, n.ID, model.StatusSent, "")
	n.Status = model.StatusSent
	n.SentAt = &sentAt

	// Record throttle hit
	if err := s.throttle.Record(ctx, req.UserID, req.Channel); err != nil {
		s.logger.Warn("Failed to record throttle", zap.Error(err))
	}

	s.logger.Info("Notification sent successfully",
		zap.String("notification_id", n.ID.String()),
		zap.String("message_id", result.MessageID),
		zap.String("user_id", req.UserID),
		zap.String("channel", string(req.Channel)),
	)

	return n, nil
}

// SendBatch sends multiple notifications, returning results for each.
func (s *NotificationService) SendBatch(ctx context.Context, req *model.BatchSendRequest) ([]*model.Notification, []error) {
	results := make([]*model.Notification, len(req.Notifications))
	errors := make([]error, len(req.Notifications))

	for i, notifReq := range req.Notifications {
		notifReqCopy := notifReq
		n, err := s.Send(ctx, &notifReqCopy)
		results[i] = n
		errors[i] = err
	}

	return results, errors
}

// GetNotification retrieves a notification by ID.
func (s *NotificationService) GetNotification(ctx context.Context, id uuid.UUID) (*model.Notification, error) {
	n, err := s.notifRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get notification: %w", err)
	}
	return n, nil
}

// GetUserNotifications retrieves notifications for a user.
func (s *NotificationService) GetUserNotifications(ctx context.Context, userID string, limit, offset int) ([]*model.Notification, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	notifications, err := s.notifRepo.GetByUserID(ctx, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("get user notifications: %w", err)
	}
	return notifications, nil
}

// checkAndSetDedup checks if a notification is a duplicate and sets the dedup key if not.
// Returns true if the notification is a duplicate.
func (s *NotificationService) checkAndSetDedup(ctx context.Context, key string) (bool, error) {
	redisKey := fmt.Sprintf(dedupKeyFormat, key)

	// SET NX (only set if not exists)
	set, err := s.redisClient.SetNX(ctx, redisKey, "1", s.dedupTTL).Result()
	if err != nil {
		return false, fmt.Errorf("dedup check: %w", err)
	}

	// If set is false, key already existed -> duplicate
	return !set, nil
}

// validateSendRequest validates the send request fields.
func validateSendRequest(req *model.SendRequest) error {
	if req.UserID == "" {
		return fmt.Errorf("user_id is required")
	}
	if !req.Type.IsValid() {
		return fmt.Errorf("invalid notification type: %s", req.Type)
	}
	if !req.Channel.IsValid() {
		return fmt.Errorf("invalid notification channel: %s", req.Channel)
	}
	if !req.Priority.IsValid() {
		return fmt.Errorf("invalid priority: %s", req.Priority)
	}
	if req.TemplateID == "" && req.Body == "" {
		return fmt.Errorf("either template_id or body must be provided")
	}
	return nil
}

// buildDedupKey builds a SHA-256 deduplication key from the notification request.
func buildDedupKey(req *model.SendRequest) string {
	// Sort data map keys for deterministic hashing
	dataKeys := make([]string, 0, len(req.Data))
	for k := range req.Data {
		dataKeys = append(dataKeys, k)
	}
	sort.Strings(dataKeys)

	dataParts := make([]string, 0, len(dataKeys))
	for _, k := range dataKeys {
		dataParts = append(dataParts, k+"="+req.Data[k])
	}

	parts := []string{
		req.UserID,
		string(req.Type),
		string(req.Channel),
		req.TemplateID,
		strings.Join(dataParts, ","),
	}

	raw := strings.Join(parts, "|")
	hash := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", hash)
}
