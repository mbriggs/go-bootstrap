package flows

import (
	"context"
	"fmt"
	"time"

	"github.com/inngest/inngestgo"
	"github.com/inngest/inngestgo/step"

	"github.com/mbriggs/go-bootstrap/mailer"
)

// WelcomeData is the payload of "app/user.created". cmd/createuser emits
// it (best-effort); emit it from signup too when the app grows one:
//
//	flows.Send(ctx, "app/user.created", map[string]any{"user_id": u.ID, "email": u.Email})
type WelcomeData struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
}

// registerWelcome is the worked example of a durable flow: each step.Run is
// checkpointed (a crash or deploy mid-flow re-runs only unfinished steps)
// and the sleep costs no process state — Inngest re-invokes the function a
// day later and replays the first step's recorded result.
func registerWelcome(c inngestgo.Client) error {
	_, err := inngestgo.CreateFunction(
		c,
		inngestgo.FunctionOpts{ID: "welcome", Retries: new(3)},
		inngestgo.EventTrigger("app/user.created", nil),
		func(ctx context.Context, input inngestgo.Input[WelcomeData]) (any, error) {
			_, err := step.Run(ctx, "send-welcome", func(ctx context.Context) (any, error) {
				return nil, mailer.Send(ctx, mailer.Message{
					To:      input.Event.Data.Email,
					Subject: "Welcome!",
					Body:    "Thanks for signing up.",
				})
			})
			if err != nil {
				return nil, fmt.Errorf("send-welcome: %w", err)
			}

			step.Sleep(ctx, "settle-in", 24*time.Hour)

			_, err = step.Run(ctx, "log-follow-up", func(ctx context.Context) (any, error) {
				logger.Info("welcome follow-up due", "user_id", input.Event.Data.UserID)
				return nil, nil
			})
			if err != nil {
				return nil, fmt.Errorf("follow-up: %w", err)
			}

			return nil, nil
		},
	)
	if err != nil {
		return fmt.Errorf("creating welcome function: %w", err)
	}

	return nil
}
