// SES is the production Sender, delivering through the Amazon SES v2 API.
// Region and credentials come from the standard AWS_* environment (or the
// instance role) — the SDK consumes those directly, like pgx does PG*.
// main swaps it into Outbox at boot when MAIL_FROM is set.

package mailer

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

// SES sends through Amazon SES. The client is safe for concurrent use, so
// one instance serves every worker.
type SES struct {
	client *sesv2.Client
	from   string
}

// NewSES builds an SES sender from the ambient AWS configuration. from is
// the sender address (MAIL_FROM) and must be SES-verified.
func NewSES(ctx context.Context, from string) (*SES, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading aws config: %w", err)
	}

	return &SES{client: sesv2.NewFromConfig(cfg), from: from}, nil
}

func (s *SES) Send(ctx context.Context, msg Message) error {
	_, err := s.client.SendEmail(ctx, &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(s.from),
		Destination:      &types.Destination{ToAddresses: []string{msg.To}},
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{Data: aws.String(msg.Subject)},
				Body: &types.Body{
					Text: &types.Content{Data: aws.String(msg.Body)},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("sending via ses: %w", err)
	}

	return nil
}
