package unit

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"

	"github.com/nicedavid98/notification-service/internal/config"
	"github.com/nicedavid98/notification-service/internal/model"
	"github.com/nicedavid98/notification-service/internal/service"
)

func newThrottleService(t *testing.T, redisClient *redis.Client, maxPush int) *service.ThrottleService {
	t.Helper()
	cfg := config.ThrottleConfig{
		MaxPerUserPerHour: 100,
		MaxPushPerHour:    maxPush,
		MaxEmailPerHour:   10,
		MaxSMSPerHour:     5,
		MaxInAppPerHour:   200,
		DeduplicationTTL:  300,
	}
	logger := newTestLogger()
	return service.NewThrottleService(redisClient, cfg, logger)
}

func TestThrottle_AllowsUnderLimit(t *testing.T) {
	redisClient := newTestRedis(t)
	redisClient.FlushDB(context.Background())

	svc := newThrottleService(t, redisClient, 10)

	throttled, err := svc.IsThrottled(context.Background(), "user-throttle-1", model.ChannelFCM, model.PriorityNormal)
	if err != nil {
		t.Fatalf("throttle check error: %v", err)
	}
	if throttled {
		t.Error("expected not throttled for fresh user")
	}
}

func TestThrottle_BlocksAtLimit(t *testing.T) {
	redisClient := newTestRedis(t)
	redisClient.FlushDB(context.Background())

	limit := 3
	svc := newThrottleService(t, redisClient, limit)
	userID := "user-throttle-2"

	ctx := context.Background()

	// Record up to the limit
	for i := 0; i < limit; i++ {
		if err := svc.Record(ctx, userID, model.ChannelFCM); err != nil {
			t.Fatalf("record error: %v", err)
		}
	}

	// Next check should be throttled
	throttled, err := svc.IsThrottled(ctx, userID, model.ChannelFCM, model.PriorityNormal)
	if err != nil {
		t.Fatalf("throttle check error: %v", err)
	}
	if !throttled {
		t.Error("expected throttled after reaching limit")
	}
}

func TestThrottle_HighPriorityBypassesThrottle(t *testing.T) {
	redisClient := newTestRedis(t)
	redisClient.FlushDB(context.Background())

	limit := 1
	svc := newThrottleService(t, redisClient, limit)
	userID := "user-throttle-3"

	ctx := context.Background()

	// Fill up the limit
	if err := svc.Record(ctx, userID, model.ChannelFCM); err != nil {
		t.Fatalf("record error: %v", err)
	}

	// High priority should bypass throttle
	throttled, err := svc.IsThrottled(ctx, userID, model.ChannelFCM, model.PriorityHigh)
	if err != nil {
		t.Fatalf("throttle check error: %v", err)
	}
	if throttled {
		t.Error("expected high priority to bypass throttle")
	}
}

func TestThrottle_GetCurrentCount(t *testing.T) {
	redisClient := newTestRedis(t)
	redisClient.FlushDB(context.Background())

	svc := newThrottleService(t, redisClient, 10)
	userID := "user-throttle-4"
	ctx := context.Background()

	count, err := svc.GetCurrentCount(ctx, userID, model.ChannelFCM)
	if err != nil {
		t.Fatalf("get count error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 initial count, got %d", count)
	}

	_ = svc.Record(ctx, userID, model.ChannelFCM)
	_ = svc.Record(ctx, userID, model.ChannelFCM)

	count, err = svc.GetCurrentCount(ctx, userID, model.ChannelFCM)
	if err != nil {
		t.Fatalf("get count error: %v", err)
	}
	if count != 2 {
		t.Errorf("expected count 2, got %d", count)
	}
}

func TestThrottle_DifferentChannelsIndependent(t *testing.T) {
	redisClient := newTestRedis(t)
	redisClient.FlushDB(context.Background())

	svc := newThrottleService(t, redisClient, 1)
	userID := "user-throttle-5"
	ctx := context.Background()

	// Fill push limit
	_ = svc.Record(ctx, userID, model.ChannelFCM)

	// Push should be throttled
	throttledFCM, _ := svc.IsThrottled(ctx, userID, model.ChannelFCM, model.PriorityNormal)
	if !throttledFCM {
		t.Error("expected FCM to be throttled")
	}

	// In-app should not be throttled (different channel, different limit)
	throttledInApp, _ := svc.IsThrottled(ctx, userID, model.ChannelInApp, model.PriorityNormal)
	if throttledInApp {
		t.Error("expected in-app to NOT be throttled when FCM is throttled")
	}
}
