package push

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
	"golang.org/x/net/http2"

	"github.com/nicedavid98/notification-service/internal/config"
	"github.com/nicedavid98/notification-service/internal/model"
)

const (
	apnsSandboxURL    = "https://api.sandbox.push.apple.com/3/device/%s"
	apnsProductionURL = "https://api.push.apple.com/3/device/%s"
)

// APNSProvider sends push notifications via Apple Push Notification Service.
type APNSProvider struct {
	cfg        config.APNSConfig
	httpClient *http.Client
	logger     *zap.Logger
	baseURL    string
}

// apnsPayload represents the APNS notification payload.
type apnsPayload struct {
	APS  apnsAPS           `json:"aps"`
	Data map[string]string `json:"data,omitempty"`
}

type apnsAPS struct {
	Alert            apnsAlert `json:"alert"`
	Badge            *int      `json:"badge,omitempty"`
	Sound            string    `json:"sound,omitempty"`
	ContentAvailable int       `json:"content-available,omitempty"`
	MutableContent   int       `json:"mutable-content,omitempty"`
	Category         string    `json:"category,omitempty"`
	ThreadID         string    `json:"thread-id,omitempty"`
}

type apnsAlert struct {
	Title    string `json:"title"`
	Body     string `json:"body"`
	Subtitle string `json:"subtitle,omitempty"`
}

type apnsErrorResponse struct {
	Reason    string `json:"reason"`
	Timestamp int64  `json:"timestamp,omitempty"`
}

// NewAPNSProvider creates a new APNS push notification provider.
func NewAPNSProvider(cfg config.APNSConfig, logger *zap.Logger) (*APNSProvider, error) {
	var cert tls.Certificate
	var err error

	if cfg.CertificateFile != "" {
		cert, err = tls.LoadX509KeyPair(cfg.CertificateFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load APNS certificate: %w", err)
		}
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	transport := &http2.Transport{
		TLSClientConfig: tlsConfig,
	}

	baseURL := apnsSandboxURL
	if cfg.Production {
		baseURL = apnsProductionURL
	}

	return &APNSProvider{
		cfg: cfg,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
		logger:  logger,
		baseURL: baseURL,
	}, nil
}

// Name returns the provider name.
func (p *APNSProvider) Name() string {
	return "apns"
}

// Send delivers a push notification via APNS.
func (p *APNSProvider) Send(ctx context.Context, n *model.Notification) (model.DeliveryResult, error) {
	if n.DeviceToken == "" {
		return model.DeliveryResult{}, fmt.Errorf("device token is required for APNS")
	}

	payload := apnsPayload{
		APS: apnsAPS{
			Alert: apnsAlert{
				Title: n.Title,
				Body:  n.Body,
			},
			Sound: "default",
		},
		Data: n.Data,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return model.DeliveryResult{}, fmt.Errorf("marshal APNS payload: %w", err)
	}

	url := fmt.Sprintf(p.baseURL, n.DeviceToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payloadBytes))
	if err != nil {
		return model.DeliveryResult{}, fmt.Errorf("create APNS request: %w", err)
	}

	apnsPriority := "5"
	if n.Priority == model.PriorityHigh {
		apnsPriority = "10"
	}

	expiration := time.Now().Add(time.Duration(p.cfg.DefaultExpiration) * time.Second).Unix()

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apns-priority", apnsPriority)
	req.Header.Set("apns-topic", p.cfg.BundleID)
	req.Header.Set("apns-expiration", fmt.Sprintf("%d", expiration))

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return model.DeliveryResult{}, fmt.Errorf("send APNS request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return model.DeliveryResult{}, fmt.Errorf("read APNS response: %w", err)
	}

	apnsID := resp.Header.Get("apns-id")

	if resp.StatusCode != http.StatusOK {
		var errResp apnsErrorResponse
		_ = json.Unmarshal(body, &errResp)
		p.logger.Error("APNS send failed",
			zap.Int("status_code", resp.StatusCode),
			zap.String("reason", errResp.Reason),
			zap.String("user_id", n.UserID),
		)
		return model.DeliveryResult{
			Success:    false,
			StatusCode: resp.StatusCode,
		}, fmt.Errorf("APNS error: %s", errResp.Reason)
	}

	p.logger.Debug("APNS notification sent",
		zap.String("apns_id", apnsID),
		zap.String("user_id", n.UserID),
	)

	return model.DeliveryResult{
		Success:    true,
		MessageID:  apnsID,
		StatusCode: resp.StatusCode,
	}, nil
}
