// Package email sends transactional emails (invites, password resets, join
// decisions). The low-level Sender abstracts the transport; Mailer renders the
// concrete messages. The default transport is AWS SES; a logging transport is
// used in local dev so no AWS credentials are required.
package email

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"go.uber.org/zap"
)

// Message is a rendered email ready to send.
type Message struct {
	From     string
	To       string
	Subject  string
	HTMLBody string
	TextBody string
}

// Sender delivers a rendered Message.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// SESSender delivers email through AWS SES v2.
type SESSender struct {
	client *sesv2.Client
}

// NewSESSender loads AWS config from the environment (same as storage.NewS3Presigner)
// and constructs an SES v2 client for the given region.
func NewSESSender(ctx context.Context, region string) (*SESSender, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return &SESSender{client: sesv2.NewFromConfig(cfg)}, nil
}

// Send delivers the message via ses:SendEmail.
func (s *SESSender) Send(ctx context.Context, msg Message) error {
	_, err := s.client.SendEmail(ctx, &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(msg.From),
		Destination:      &types.Destination{ToAddresses: []string{msg.To}},
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{Data: aws.String(msg.Subject), Charset: aws.String("UTF-8")},
				Body: &types.Body{
					Html: &types.Content{Data: aws.String(msg.HTMLBody), Charset: aws.String("UTF-8")},
					Text: &types.Content{Data: aws.String(msg.TextBody), Charset: aws.String("UTF-8")},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("ses send email: %w", err)
	}
	return nil
}

// LogSender logs emails instead of sending them. Used in local dev / when no
// sender address is configured, so the app runs without AWS credentials.
type LogSender struct {
	log *zap.Logger
}

// NewLogSender returns a Sender that logs each message at info level.
func NewLogSender(log *zap.Logger) *LogSender {
	return &LogSender{log: log}
}

// Send logs the message metadata (body omitted to keep logs readable).
func (s *LogSender) Send(_ context.Context, msg Message) error {
	s.log.Info("email (log sender, not actually sent)",
		zap.String("to", msg.To),
		zap.String("from", msg.From),
		zap.String("subject", msg.Subject),
	)
	return nil
}
