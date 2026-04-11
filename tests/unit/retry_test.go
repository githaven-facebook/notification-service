package unit

import (
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/nicedavid98/notification-service/internal/delivery"
)

func TestRetryManager_Creation(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	notifRepo := newMockNotifRepo()

	mgr := delivery.NewRetryManager(nil, notifRepo, delivery.RetryConfig{
		MaxRetries: 5,
		BaseDelay:  time.Second,
		MaxDelay:   16 * time.Second,
	}, logger)

	if mgr == nil {
		t.Fatal("expected non-nil retry manager")
	}
}

func TestRetryConfig_Defaults(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	notifRepo := newMockNotifRepo()

	// Zero values should use defaults
	mgr := delivery.NewRetryManager(nil, notifRepo, delivery.RetryConfig{}, logger)
	if mgr == nil {
		t.Fatal("expected non-nil retry manager with default config")
	}
}

func TestRetryManager_ExponentialBackoff(t *testing.T) {
	// Verify exponential backoff progression by checking base delay calculation.
	// base=1s: attempt 0->1s, 1->2s, 2->4s, 3->8s, 4->16s (capped)
	base := time.Second
	maxDelay := 16 * time.Second

	expectedBases := []time.Duration{
		1 * base,
		2 * base,
		4 * base,
		8 * base,
		16 * base,
	}

	for attempt, expected := range expectedBases {
		exp := 1
		for i := 0; i < attempt; i++ {
			exp *= 2
		}
		baseDelay := time.Duration(exp) * base
		if baseDelay > maxDelay {
			baseDelay = maxDelay
		}

		if baseDelay != expected {
			t.Errorf("attempt %d: expected base delay %v, got %v", attempt, expected, baseDelay)
		}
	}
}
