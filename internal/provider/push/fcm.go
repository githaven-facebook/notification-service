package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/nicedavid98/notification-service/internal/config"
	"github.com/nicedavid98/notification-service/internal/model"
)

const (
	fcmSendURL = "https://fcm.googleapis.com/v1/projects/%s/messages:send"
)

// FCMProvider sends push notifications via Firebase Cloud Messaging HTTP v1 API.
type FCMProvider struct {
	cfg        config.FCMConfig
	httpClient *http.Client
	logger     *zap.Logger
	accessToken string
}

// fcmMessage represents the FCM v1 API message payload.
type fcmMessage struct {
	Message fcmMessageBody `json:"message"`
}

type fcmMessageBody struct {
	Token        string                 `json:"token,omitempty"`
	Topic        string                 `json:"topic,omitempty"`
	Notification *fcmNotification       `json:"notification,omitempty"`
	Data         map[string]string      `json:"data,omitempty"`
	Android      *fcmAndroidConfig      `json:"android,omitempty"`
	APNS         *fcmAPNSConfig         `json:"apns,omitempty"`
}

type fcmNotification struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type fcmAndroidConfig struct {
	Priority string `json:"priority,omitempty"` // "normal" or "high"
	TTL      string `json:"ttl,omitempty"`      // e.g., "86400s"
}

type fcmAPNSConfig struct {
	Headers map[string]string `json:"headers,omitempty"`
}

type fcmResponse struct {
	Name string `json:"name"`
}

type fcmErrorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

// NewFCMProvider creates a new FCM push notification provider.
func NewFCMProvider(cfg config.FCMConfig, logger *zap.Logger) *FCMProvider {
	return &FCMProvider{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// Name returns the provider name.
func (p *FCMProvider) Name() string {
	return "fcm"
}

// Send delivers a push notification via FCM.
func (p *FCMProvider) Send(ctx context.Context, n *model.Notification) (model.DeliveryResult, error) {
	if n.DeviceToken == "" {
		return model.DeliveryResult{}, fmt.Errorf("device token is required for FCM")
	}

	priority := "normal"
	if n.Priority == model.PriorityHigh {
		priority = "high"
	}

	ttl := fmt.Sprintf("%ds", p.cfg.DefaultTTL)

	msg := fcmMessage{
		Message: fcmMessageBody{
			Token: n.DeviceToken,
			Notification: &fcmNotification{
				Title: n.Title,
				Body:  n.Body,
			},
			Data: n.Data,
			Android: &fcmAndroidConfig{
				Priority: priority,
				TTL:      ttl,
			},
		},
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return model.DeliveryResult{}, fmt.Errorf("marshal FCM message: %w", err)
	}

	url := fmt.Sprintf(fcmSendURL, p.cfg.ProjectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return model.DeliveryResult{}, fmt.Errorf("create FCM request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.accessToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return model.DeliveryResult{}, fmt.Errorf("send FCM request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return model.DeliveryResult{}, fmt.Errorf("read FCM response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp fcmErrorResponse
		_ = json.Unmarshal(body, &errResp)
		p.logger.Error("FCM send failed",
			zap.Int("status_code", resp.StatusCode),
			zap.String("error", errResp.Error.Message),
			zap.String("user_id", n.UserID),
		)
		return model.DeliveryResult{
			Success:    false,
			StatusCode: resp.StatusCode,
		}, fmt.Errorf("FCM error %d: %s", errResp.Error.Code, errResp.Error.Message)
	}

	var fcmResp fcmResponse
	if err := json.Unmarshal(body, &fcmResp); err != nil {
		return model.DeliveryResult{}, fmt.Errorf("unmarshal FCM response: %w", err)
	}

	p.logger.Debug("FCM notification sent",
		zap.String("message_id", fcmResp.Name),
		zap.String("user_id", n.UserID),
	)

	return model.DeliveryResult{
		Success:    true,
		MessageID:  fcmResp.Name,
		StatusCode: resp.StatusCode,
	}, nil
}

// SendToTopic sends a notification to all devices subscribed to a topic.
func (p *FCMProvider) SendToTopic(ctx context.Context, topic string, n *model.Notification) (model.DeliveryResult, error) {
	msg := fcmMessage{
		Message: fcmMessageBody{
			Topic: topic,
			Notification: &fcmNotification{
				Title: n.Title,
				Body:  n.Body,
			},
			Data: n.Data,
		},
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return model.DeliveryResult{}, fmt.Errorf("marshal FCM topic message: %w", err)
	}

	url := fmt.Sprintf(fcmSendURL, p.cfg.ProjectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return model.DeliveryResult{}, fmt.Errorf("create FCM topic request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.accessToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return model.DeliveryResult{}, fmt.Errorf("send FCM topic request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return model.DeliveryResult{
			Success:    false,
			StatusCode: resp.StatusCode,
		}, fmt.Errorf("FCM topic send failed with status %d", resp.StatusCode)
	}

	return model.DeliveryResult{
		Success:    true,
		StatusCode: resp.StatusCode,
	}, nil
}

// SetAccessToken sets the OAuth2 access token for FCM API calls.
func (p *FCMProvider) SetAccessToken(token string) {
	p.accessToken = token
}
