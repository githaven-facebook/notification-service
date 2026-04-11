package email

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
	"go.uber.org/zap"

	"github.com/nicedavid98/notification-service/internal/config"
	"github.com/nicedavid98/notification-service/internal/model"
)

// SESProvider sends email notifications via AWS Simple Email Service.
type SESProvider struct {
	client *ses.Client
	cfg    config.SESConfig
	logger *zap.Logger
}

// NewSESProvider creates a new AWS SES email provider.
func NewSESProvider(ctx context.Context, cfg config.SESConfig, logger *zap.Logger) (*SESProvider, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.Region),
	)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	return &SESProvider{
		client: ses.NewFromConfig(awsCfg),
		cfg:    cfg,
		logger: logger,
	}, nil
}

// Name returns the provider name.
func (p *SESProvider) Name() string {
	return "ses"
}

// Send delivers an email notification via AWS SES.
func (p *SESProvider) Send(ctx context.Context, n *model.Notification) (model.DeliveryResult, error) {
	if n.Recipient == "" {
		return model.DeliveryResult{}, fmt.Errorf("recipient email is required for SES")
	}

	toAddresses := []string{n.Recipient}
	input := &ses.SendEmailInput{
		Source: aws.String(p.cfg.FromAddress),
		Destination: &types.Destination{
			ToAddresses: toAddresses,
		},
		Message: &types.Message{
			Subject: &types.Content{
				Data:    aws.String(n.Title),
				Charset: aws.String("UTF-8"),
			},
			Body: &types.Body{
				Html: &types.Content{
					Data:    aws.String(n.Body),
					Charset: aws.String("UTF-8"),
				},
				Text: &types.Content{
					Data:    aws.String(n.Body),
					Charset: aws.String("UTF-8"),
				},
			},
		},
	}

	if p.cfg.ReplyTo != "" {
		input.ReplyToAddresses = []string{p.cfg.ReplyTo}
	}

	resp, err := p.client.SendEmail(ctx, input)
	if err != nil {
		p.logger.Error("SES send failed",
			zap.Error(err),
			zap.String("recipient", n.Recipient),
			zap.String("user_id", n.UserID),
		)
		return model.DeliveryResult{
			Success: false,
			Error:   err,
		}, fmt.Errorf("send SES email: %w", err)
	}

	messageID := ""
	if resp.MessageId != nil {
		messageID = *resp.MessageId
	}

	p.logger.Debug("SES email sent",
		zap.String("message_id", messageID),
		zap.String("recipient", n.Recipient),
		zap.String("user_id", n.UserID),
	)

	return model.DeliveryResult{
		Success:   true,
		MessageID: messageID,
	}, nil
}

// SendRaw sends a raw email with custom headers and attachments.
func (p *SESProvider) SendRaw(ctx context.Context, from string, toAddresses []string, rawMessage []byte) (string, error) {
	input := &ses.SendRawEmailInput{
		Source:       aws.String(from),
		Destinations: toAddresses,
		RawMessage: &types.RawMessage{
			Data: rawMessage,
		},
	}

	resp, err := p.client.SendRawEmail(ctx, input)
	if err != nil {
		return "", fmt.Errorf("send raw SES email: %w", err)
	}

	if resp.MessageId != nil {
		return *resp.MessageId, nil
	}
	return "", nil
}
