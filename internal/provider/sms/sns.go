package sms

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	"go.uber.org/zap"

	"github.com/nicedavid98/notification-service/internal/config"
	"github.com/nicedavid98/notification-service/internal/model"
)

// SNSProvider sends SMS notifications via AWS Simple Notification Service.
type SNSProvider struct {
	client *sns.Client
	cfg    config.SNSConfig
	logger *zap.Logger
}

// NewSNSProvider creates a new AWS SNS SMS provider.
func NewSNSProvider(ctx context.Context, cfg config.SNSConfig, logger *zap.Logger) (*SNSProvider, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.Region),
	)
	if err != nil {
		return nil, fmt.Errorf("load AWS config for SNS: %w", err)
	}

	return &SNSProvider{
		client: sns.NewFromConfig(awsCfg),
		cfg:    cfg,
		logger: logger,
	}, nil
}

// Name returns the provider name.
func (p *SNSProvider) Name() string {
	return "sns"
}

// Send delivers an SMS notification via AWS SNS.
func (p *SNSProvider) Send(ctx context.Context, n *model.Notification) (model.DeliveryResult, error) {
	if n.Recipient == "" {
		return model.DeliveryResult{}, fmt.Errorf("recipient phone number is required for SNS")
	}

	msgType := "Promotional"
	if n.Priority == model.PriorityHigh {
		msgType = "Transactional"
	}

	attrs := map[string]snstypes.MessageAttributeValue{
		"AWS.SNS.SMS.SMSType": {
			DataType:    aws.String("String"),
			StringValue: aws.String(msgType),
		},
	}

	if p.cfg.SenderID != "" {
		attrs["AWS.SNS.SMS.SenderID"] = snstypes.MessageAttributeValue{
			DataType:    aws.String("String"),
			StringValue: aws.String(p.cfg.SenderID),
		}
	}

	// SMS body is the notification body (title is not used for SMS)
	message := n.Body
	if len(message) > 160 {
		// Truncate to standard SMS length
		message = message[:157] + "..."
	}

	input := &sns.PublishInput{
		PhoneNumber:       aws.String(n.Recipient),
		Message:           aws.String(message),
		MessageAttributes: attrs,
	}

	resp, err := p.client.Publish(ctx, input)
	if err != nil {
		p.logger.Error("SNS send failed",
			zap.Error(err),
			zap.String("recipient", n.Recipient),
			zap.String("user_id", n.UserID),
		)
		return model.DeliveryResult{
			Success: false,
			Error:   err,
		}, fmt.Errorf("send SNS SMS: %w", err)
	}

	messageID := ""
	if resp.MessageId != nil {
		messageID = *resp.MessageId
	}

	p.logger.Debug("SNS SMS sent",
		zap.String("message_id", messageID),
		zap.String("user_id", n.UserID),
	)

	return model.DeliveryResult{
		Success:   true,
		MessageID: messageID,
	}, nil
}
