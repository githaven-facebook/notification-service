package provider

import (
	"context"

	"github.com/nicedavid98/notification-service/internal/model"
)

// Provider defines the interface for sending notifications through a specific channel.
type Provider interface {
	// Send delivers a notification and returns the delivery result.
	Send(ctx context.Context, notification *model.Notification) (model.DeliveryResult, error)

	// Name returns the provider name for logging and metrics.
	Name() string
}
