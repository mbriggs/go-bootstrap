// Package mailer is the outbound-email seam. Email is an external service
// the app shouldn't be coupled to, so the seam is earned: the default
// sender logs the message — right for development and tests — and main
// swaps in the SES sender at boot when MAIL_FROM is set, before any worker
// that sends mail starts.
//
// Sending belongs in a background job, not a request handler: enqueue
// through the jobs package in the same transaction as the state change the
// email announces, and let the worker call Send. That keeps SMTP latency
// and provider outages out of the request path and gets retries for free.
package mailer

import (
	"context"

	"github.com/mbriggs/go-bootstrap/logging"
)

var logger = logging.Logger("mailer")

// Message is a plain-text email. HTML and attachments are a provider
// concern; add fields when a real provider needs them.
type Message struct {
	To      string
	Subject string
	Body    string
}

// Sender delivers a message. Implementations must be safe for concurrent
// use — job workers send from parallel goroutines.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// Outbox is the process-wide sender. main swaps it at boot for the real
// provider (see NewSES); everything else sends through it blindly.
var Outbox Sender = logSender{}

// Send delivers msg through the configured Outbox.
func Send(ctx context.Context, msg Message) error {
	return Outbox.Send(ctx, msg)
}

// logSender writes the email to the log instead of sending it. The dev loop
// reads reset links straight from the air output; tests assert on enqueued
// jobs rather than delivered mail.
type logSender struct{}

func (logSender) Send(_ context.Context, msg Message) error {
	logger.Info(
		"outbound email (log sender — swap mailer.Outbox for real delivery)",
		"to", msg.To,
		"subject", msg.Subject,
		"body", msg.Body,
	)

	return nil
}
