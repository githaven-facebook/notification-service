//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/nicedavid98/notification-service/internal/config"
	"github.com/nicedavid98/notification-service/internal/model"
	"github.com/nicedavid98/notification-service/internal/provider"
	"github.com/nicedavid98/notification-service/internal/service"
	tmpl "github.com/nicedavid98/notification-service/internal/template"
)

// mockIntegrationProvider records all Send calls and succeeds.
type mockIntegrationProvider struct {
	name  string
	calls []*model.Notification
}

func (p *mockIntegrationProvider) Send(_ context.Context, n *model.Notification) (model.DeliveryResult, error) {
	p.calls = append(p.calls, n)
	return model.DeliveryResult{Success: true, MessageID: "integration-msg-id"}, nil
}

func (p *mockIntegrationProvider) Name() string { return p.name }

// mockIntegrationNotifRepo stores notifications in memory.
type mockIntegrationNotifRepo struct {
	notifications map[uuid.UUID]*model.Notification
}

func newMockIntegrationNotifRepo() *mockIntegrationNotifRepo {
	return &mockIntegrationNotifRepo{notifications: make(map[uuid.UUID]*model.Notification)}
}

func (m *mockIntegrationNotifRepo) Create(_ context.Context, n *model.Notification) error {
	m.notifications[n.ID] = n
	return nil
}

func (m *mockIntegrationNotifRepo) BatchCreate(_ context.Context, notifications []*model.Notification) error {
	for _, n := range notifications {
		m.notifications[n.ID] = n
	}
	return nil
}

func (m *mockIntegrationNotifRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Notification, error) {
	return m.notifications[id], nil
}

func (m *mockIntegrationNotifRepo) UpdateStatus(_ context.Context, id uuid.UUID, status model.NotificationStatus, errMsg string) error {
	if n, ok := m.notifications[id]; ok {
		n.Status = status
		n.ErrorMessage = errMsg
	}
	return nil
}

func (m *mockIntegrationNotifRepo) UpdateSentAt(_ context.Context, id uuid.UUID, sentAt time.Time) error {
	if n, ok := m.notifications[id]; ok {
		n.SentAt = &sentAt
	}
	return nil
}

func (m *mockIntegrationNotifRepo) UpdateDeliveredAt(_ context.Context, id uuid.UUID, deliveredAt time.Time) error {
	if n, ok := m.notifications[id]; ok {
		n.DeliveredAt = &deliveredAt
	}
	return nil
}

func (m *mockIntegrationNotifRepo) IncrementRetryCount(_ context.Context, id uuid.UUID) error {
	if n, ok := m.notifications[id]; ok {
		n.RetryCount++
	}
	return nil
}

func (m *mockIntegrationNotifRepo) GetByUserID(_ context.Context, userID string, _, _ int) ([]*model.Notification, error) {
	var result []*model.Notification
	for _, n := range m.notifications {
		if n.UserID == userID {
			result = append(result, n)
		}
	}
	return result, nil
}

func (m *mockIntegrationNotifRepo) GetByStatus(_ context.Context, _ model.NotificationStatus, _ int) ([]*model.Notification, error) {
	return nil, nil
}

func (m *mockIntegrationNotifRepo) GetByUserAndType(_ context.Context, _ string, _ model.NotificationType, _ int) ([]*model.Notification, error) {
	return nil, nil
}

func (m *mockIntegrationNotifRepo) GetPendingForRetry(_ context.Context, _, _ int) ([]*model.Notification, error) {
	return nil, nil
}

type mockIntegrationPrefRepo struct{}

func (m *mockIntegrationPrefRepo) GetByUserID(_ context.Context, _ string) ([]*model.UserPreference, error) {
	return nil, nil
}

func (m *mockIntegrationPrefRepo) GetByUserAndChannel(_ context.Context, _ string, _ model.NotificationChannel) (*model.UserPreference, error) {
	return nil, nil
}

func (m *mockIntegrationPrefRepo) Upsert(_ context.Context, _ *model.UserPreference) error {
	return nil
}

func (m *mockIntegrationPrefRepo) Delete(_ context.Context, _ string, _ model.NotificationChannel) error {
	return nil
}

type mockIntegrationTemplateRepo struct{}

func (m *mockIntegrationTemplateRepo) Create(_ context.Context, _ *model.NotificationTemplate) error {
	return nil
}

func (m *mockIntegrationTemplateRepo) GetByID(_ context.Context, _ int64) (*model.NotificationTemplate, error) {
	return nil, nil
}

func (m *mockIntegrationTemplateRepo) GetByNameAndLocale(_ context.Context, _ string, _ model.NotificationChannel, _ string) (*model.NotificationTemplate, error) {
	return nil, nil
}

func (m *mockIntegrationTemplateRepo) GetLatestVersion(_ context.Context, _ string, _ model.NotificationChannel, _ string) (*model.NotificationTemplate, error) {
	return nil, nil
}

func (m *mockIntegrationTemplateRepo) List(_ context.Context, _ model.NotificationChannel, _, _ int) ([]*model.NotificationTemplate, error) {
	return nil, nil
}

func (m *mockIntegrationTemplateRepo) Update(_ context.Context, _ int64, _ *model.UpdateTemplateRequest) error {
	return nil
}

func (m *mockIntegrationTemplateRepo) Delete(_ context.Context, _ int64) error {
	return nil
}

func newIntegrationService(t *testing.T, redisClient *redis.Client, prov provider.Provider) *service.NotificationService {
	t.Helper()
	logger, _ := zap.NewDevelopment()

	notifRepo := newMockIntegrationNotifRepo()
	prefRepo := &mockIntegrationPrefRepo{}
	templateRepo := &mockIntegrationTemplateRepo{}

	providers := map[model.NotificationChannel]provider.Provider{
		model.ChannelFCM: prov,
	}

	templateEngine := tmpl.NewEngine(templateRepo, logger)
	renderer := tmpl.NewRenderer(templateEngine)
	prefService := service.NewPreferenceService(prefRepo, logger)
	throttleService := service.NewThrottleService(redisClient, config.ThrottleConfig{
		MaxPerUserPerHour: 100,
		MaxPushPerHour:    50,
		MaxEmailPerHour:   10,
		MaxSMSPerHour:     5,
		MaxInAppPerHour:   200,
		DeduplicationTTL:  300,
	}, logger)

	return service.NewNotificationService(service.NotificationServiceConfig{
		NotifRepo:   notifRepo,
		PrefService: prefService,
		Throttle:    throttleService,
		Renderer:    renderer,
		Providers:   providers,
		RedisClient: redisClient,
		DedupTTL:    5 * time.Minute,
		Logger:      logger,
	})
}

func TestIntegration_EndToEnd_SendNotification(t *testing.T) {
	redisClient := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer redisClient.Close()

	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		t.Skip("Redis not available, skipping integration test")
	}
	redisClient.FlushDB(context.Background())

	fcmProv := &mockIntegrationProvider{name: "fcm"}
	svc := newIntegrationService(t, redisClient, fcmProv)

	req := &model.SendRequest{
		UserID:      "integration-user-1",
		Type:        model.NotificationTypePush,
		Channel:     model.ChannelFCM,
		Title:       "Integration test",
		Body:        "This is an integration test notification",
		Priority:    model.PriorityNormal,
		DeviceToken: "test-device-token",
	}

	n, err := svc.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}
	if n.Status != model.StatusSent {
		t.Errorf("expected status sent, got %s", n.Status)
	}
	if n.SentAt == nil {
		t.Error("expected sent_at to be set")
	}
	if len(fcmProv.calls) != 1 {
		t.Errorf("expected 1 FCM call, got %d", len(fcmProv.calls))
	}
}

func TestIntegration_BatchSend(t *testing.T) {
	redisClient := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer redisClient.Close()

	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		t.Skip("Redis not available, skipping integration test")
	}
	redisClient.FlushDB(context.Background())

	fcmProv := &mockIntegrationProvider{name: "fcm"}
	svc := newIntegrationService(t, redisClient, fcmProv)

	batch := &model.BatchSendRequest{
		Notifications: []model.SendRequest{
			{UserID: "batch-user-1", Type: model.NotificationTypePush, Channel: model.ChannelFCM, Body: "Msg 1", Priority: model.PriorityNormal},
			{UserID: "batch-user-2", Type: model.NotificationTypePush, Channel: model.ChannelFCM, Body: "Msg 2", Priority: model.PriorityNormal},
			{UserID: "batch-user-3", Type: model.NotificationTypePush, Channel: model.ChannelFCM, Body: "Msg 3", Priority: model.PriorityNormal},
		},
	}

	results, errs := svc.SendBatch(context.Background(), batch)
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
	for i, err := range errs {
		if err != nil {
			t.Errorf("batch[%d] unexpected error: %v", i, err)
		}
	}
	if len(fcmProv.calls) != 3 {
		t.Errorf("expected 3 FCM calls, got %d", len(fcmProv.calls))
	}
}

func TestIntegration_ThrottleAndDedup(t *testing.T) {
	redisClient := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer redisClient.Close()

	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		t.Skip("Redis not available, skipping integration test")
	}
	redisClient.FlushDB(context.Background())

	fcmProv := &mockIntegrationProvider{name: "fcm"}
	svc := newIntegrationService(t, redisClient, fcmProv)

	req := &model.SendRequest{
		UserID:   "throttle-dedup-user",
		Type:     model.NotificationTypePush,
		Channel:  model.ChannelFCM,
		Title:    "Same notification",
		Body:     "Same body",
		Priority: model.PriorityNormal,
		Data:     map[string]string{"key": "same"},
	}

	// First send succeeds
	_, err := svc.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("first send failed: %v", err)
	}

	// Identical second send should be deduplicated
	_, err = svc.Send(context.Background(), req)
	if err == nil {
		t.Error("expected dedup error on identical second send")
	}

	if len(fcmProv.calls) != 1 {
		t.Errorf("expected only 1 FCM call after dedup, got %d", len(fcmProv.calls))
	}
}
