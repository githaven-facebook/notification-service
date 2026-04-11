package unit

import (
	"context"
	"errors"
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

// -- Mock implementations --

type mockNotifRepo struct {
	notifications map[uuid.UUID]*model.Notification
}

func newMockNotifRepo() *mockNotifRepo {
	return &mockNotifRepo{notifications: make(map[uuid.UUID]*model.Notification)}
}

func (m *mockNotifRepo) Create(_ context.Context, n *model.Notification) error {
	m.notifications[n.ID] = n
	return nil
}

func (m *mockNotifRepo) BatchCreate(_ context.Context, notifications []*model.Notification) error {
	for _, n := range notifications {
		m.notifications[n.ID] = n
	}
	return nil
}

func (m *mockNotifRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Notification, error) {
	return m.notifications[id], nil
}

func (m *mockNotifRepo) UpdateStatus(_ context.Context, id uuid.UUID, status model.NotificationStatus, errMsg string) error {
	if n, ok := m.notifications[id]; ok {
		n.Status = status
		n.ErrorMessage = errMsg
	}
	return nil
}

func (m *mockNotifRepo) UpdateSentAt(_ context.Context, id uuid.UUID, sentAt time.Time) error {
	if n, ok := m.notifications[id]; ok {
		n.SentAt = &sentAt
	}
	return nil
}

func (m *mockNotifRepo) UpdateDeliveredAt(_ context.Context, id uuid.UUID, deliveredAt time.Time) error {
	if n, ok := m.notifications[id]; ok {
		n.DeliveredAt = &deliveredAt
	}
	return nil
}

func (m *mockNotifRepo) IncrementRetryCount(_ context.Context, id uuid.UUID) error {
	if n, ok := m.notifications[id]; ok {
		n.RetryCount++
	}
	return nil
}

func (m *mockNotifRepo) GetByUserID(_ context.Context, userID string, _, _ int) ([]*model.Notification, error) {
	var result []*model.Notification
	for _, n := range m.notifications {
		if n.UserID == userID {
			result = append(result, n)
		}
	}
	return result, nil
}

func (m *mockNotifRepo) GetByStatus(_ context.Context, _ model.NotificationStatus, _ int) ([]*model.Notification, error) {
	return nil, nil
}

func (m *mockNotifRepo) GetByUserAndType(_ context.Context, _ string, _ model.NotificationType, _ int) ([]*model.Notification, error) {
	return nil, nil
}

func (m *mockNotifRepo) GetPendingForRetry(_ context.Context, _, _ int) ([]*model.Notification, error) {
	return nil, nil
}

type mockPrefRepo struct {
	prefs map[string]*model.UserPreference
}

func newMockPrefRepo() *mockPrefRepo {
	return &mockPrefRepo{prefs: make(map[string]*model.UserPreference)}
}

func (m *mockPrefRepo) GetByUserID(_ context.Context, userID string) ([]*model.UserPreference, error) {
	var result []*model.UserPreference
	for _, p := range m.prefs {
		if p.UserID == userID {
			result = append(result, p)
		}
	}
	return result, nil
}

func (m *mockPrefRepo) GetByUserAndChannel(_ context.Context, userID string, channel model.NotificationChannel) (*model.UserPreference, error) {
	key := userID + ":" + string(channel)
	return m.prefs[key], nil
}

func (m *mockPrefRepo) Upsert(_ context.Context, pref *model.UserPreference) error {
	key := pref.UserID + ":" + string(pref.Channel)
	m.prefs[key] = pref
	return nil
}

func (m *mockPrefRepo) Delete(_ context.Context, userID string, channel model.NotificationChannel) error {
	key := userID + ":" + string(channel)
	delete(m.prefs, key)
	return nil
}

type mockTemplateRepo struct {
	templates map[string]*model.NotificationTemplate
}

func newMockTemplateRepo() *mockTemplateRepo {
	return &mockTemplateRepo{templates: make(map[string]*model.NotificationTemplate)}
}

func (m *mockTemplateRepo) Create(_ context.Context, t *model.NotificationTemplate) error {
	key := t.Name + ":" + string(t.Channel) + ":" + t.Locale
	m.templates[key] = t
	return nil
}

func (m *mockTemplateRepo) GetByID(_ context.Context, _ int64) (*model.NotificationTemplate, error) {
	return nil, nil
}

func (m *mockTemplateRepo) GetByNameAndLocale(_ context.Context, name string, channel model.NotificationChannel, locale string) (*model.NotificationTemplate, error) {
	key := name + ":" + string(channel) + ":" + locale
	return m.templates[key], nil
}

func (m *mockTemplateRepo) GetLatestVersion(_ context.Context, name string, channel model.NotificationChannel, locale string) (*model.NotificationTemplate, error) {
	key := name + ":" + string(channel) + ":" + locale
	return m.templates[key], nil
}

func (m *mockTemplateRepo) List(_ context.Context, _ model.NotificationChannel, _, _ int) ([]*model.NotificationTemplate, error) {
	return nil, nil
}

func (m *mockTemplateRepo) Update(_ context.Context, _ int64, _ *model.UpdateTemplateRequest) error {
	return nil
}

func (m *mockTemplateRepo) Delete(_ context.Context, _ int64) error {
	return nil
}

type mockProvider struct {
	providerName string
	sendErr      error
	calls        int
}

func (p *mockProvider) Send(_ context.Context, _ *model.Notification) (model.DeliveryResult, error) {
	p.calls++
	if p.sendErr != nil {
		return model.DeliveryResult{Success: false, Error: p.sendErr}, p.sendErr
	}
	return model.DeliveryResult{Success: true, MessageID: "mock-msg-id"}, nil
}

func (p *mockProvider) Name() string { return p.providerName }

// -- Test helpers --

func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	// Flush test keys on cleanup
	t.Cleanup(func() { client.Close() })
	return client
}

func newTestLogger() *zap.Logger {
	logger, _ := zap.NewDevelopment()
	return logger
}

func testThrottleConfig() config.ThrottleConfig {
	return config.ThrottleConfig{
		MaxPerUserPerHour: 100,
		MaxPushPerHour:    50,
		MaxEmailPerHour:   10,
		MaxSMSPerHour:     5,
		MaxInAppPerHour:   200,
		DeduplicationTTL:  300,
	}
}

func newTestService(t *testing.T, prov provider.Provider, redisClient *redis.Client) *service.NotificationService {
	t.Helper()
	notifRepo := newMockNotifRepo()
	prefRepo := newMockPrefRepo()
	templateRepo := newMockTemplateRepo()
	logger := newTestLogger()

	templateEngine := tmpl.NewEngine(templateRepo, logger)
	renderer := tmpl.NewRenderer(templateEngine)
	prefService := service.NewPreferenceService(prefRepo, logger)
	throttleService := service.NewThrottleService(redisClient, testThrottleConfig(), logger)

	providers := map[model.NotificationChannel]provider.Provider{
		model.ChannelFCM:   prov,
		model.ChannelInApp: prov,
	}

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

// -- Tests --

func TestSendNotification_Success(t *testing.T) {
	redisClient := newTestRedis(t)
	prov := &mockProvider{providerName: "fcm"}
	svc := newTestService(t, prov, redisClient)

	req := &model.SendRequest{
		UserID:   "user-1",
		Type:     model.NotificationTypePush,
		Channel:  model.ChannelFCM,
		Title:    "Test notification",
		Body:     "Hello world",
		Priority: model.PriorityNormal,
	}

	n, err := svc.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if n == nil {
		t.Fatal("expected notification, got nil")
	}
	if n.Status != model.StatusSent {
		t.Errorf("expected status %s, got %s", model.StatusSent, n.Status)
	}
	if prov.calls != 1 {
		t.Errorf("expected 1 provider call, got %d", prov.calls)
	}
}

func TestSendNotification_InvalidRequest(t *testing.T) {
	redisClient := newTestRedis(t)
	prov := &mockProvider{providerName: "fcm"}
	svc := newTestService(t, prov, redisClient)

	// Missing user_id
	req := &model.SendRequest{
		Type:     model.NotificationTypePush,
		Channel:  model.ChannelFCM,
		Body:     "Hello",
		Priority: model.PriorityNormal,
	}

	_, err := svc.Send(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing user_id, got nil")
	}
}

func TestSendNotification_ProviderFailure(t *testing.T) {
	redisClient := newTestRedis(t)
	provErr := errors.New("FCM unavailable")
	prov := &mockProvider{providerName: "fcm", sendErr: provErr}
	svc := newTestService(t, prov, redisClient)

	req := &model.SendRequest{
		UserID:   "user-2",
		Type:     model.NotificationTypePush,
		Channel:  model.ChannelFCM,
		Title:    "Test",
		Body:     "Hello",
		Priority: model.PriorityNormal,
	}

	n, err := svc.Send(context.Background(), req)
	if err == nil {
		t.Fatal("expected error from provider failure")
	}
	if n != nil && n.Status != model.StatusFailed {
		t.Errorf("expected status failed, got %s", n.Status)
	}
}

func TestSendNotification_ChannelDisabled(t *testing.T) {
	redisClient := newTestRedis(t)
	prov := &mockProvider{providerName: "fcm"}

	notifRepo := newMockNotifRepo()
	prefRepo := newMockPrefRepo()
	templateRepo := newMockTemplateRepo()
	logger := newTestLogger()

	// Disable FCM channel for user
	_ = prefRepo.Upsert(context.Background(), &model.UserPreference{
		UserID:  "user-3",
		Channel: model.ChannelFCM,
		Enabled: false,
	})

	templateEngine := tmpl.NewEngine(templateRepo, logger)
	renderer := tmpl.NewRenderer(templateEngine)
	prefService := service.NewPreferenceService(prefRepo, logger)
	throttleService := service.NewThrottleService(redisClient, testThrottleConfig(), logger)

	svc := service.NewNotificationService(service.NotificationServiceConfig{
		NotifRepo:   notifRepo,
		PrefService: prefService,
		Throttle:    throttleService,
		Renderer:    renderer,
		Providers:   map[model.NotificationChannel]provider.Provider{model.ChannelFCM: prov},
		RedisClient: redisClient,
		DedupTTL:    5 * time.Minute,
		Logger:      logger,
	})

	req := &model.SendRequest{
		UserID:   "user-3",
		Type:     model.NotificationTypePush,
		Channel:  model.ChannelFCM,
		Body:     "Hello",
		Priority: model.PriorityNormal,
	}

	_, err := svc.Send(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for disabled channel")
	}
	if prov.calls != 0 {
		t.Errorf("expected 0 provider calls for disabled channel, got %d", prov.calls)
	}
}

func TestSendNotification_Deduplication(t *testing.T) {
	redisClient := newTestRedis(t)
	// Clear any existing dedup keys
	redisClient.FlushDB(context.Background())

	prov := &mockProvider{providerName: "fcm"}
	svc := newTestService(t, prov, redisClient)

	req := &model.SendRequest{
		UserID:   "user-4",
		Type:     model.NotificationTypePush,
		Channel:  model.ChannelFCM,
		Title:    "Dedup test",
		Body:     "Hello",
		Priority: model.PriorityNormal,
		Data:     map[string]string{"key": "val"},
	}

	// First send should succeed
	_, err := svc.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("first send failed: %v", err)
	}

	// Second identical send should be deduplicated
	_, err = svc.Send(context.Background(), req)
	if err == nil {
		t.Fatal("expected dedup error on second send")
	}

	if prov.calls != 1 {
		t.Errorf("expected only 1 provider call (dedup), got %d", prov.calls)
	}
}
