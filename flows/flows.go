// Package flows is the orchestration tier, backed by Inngest: durable
// multi-step functions with sleeps, event waits, and per-step retries.
//
// The dividing line against the jobs package: jobs (River) is for
// single-step background work that belongs to a transaction — "this
// committed, so send the email"; flows is for processes that outlive a
// request and coordinate over time — "welcome them now, check back in a
// day, abandon if they cancel". A flow step that touches the database goes
// through the same domain functions as everything else; a flow that needs
// transactional enqueue semantics emits its event from inside the
// transaction's River job. Reach for jobs first; reach for flows when you
// catch yourself building a state machine out of chained jobs.
//
// Flows execute via HTTP: the Inngest server (dev: `docker compose up
// inngest`, browse http://localhost:8288) calls back into the app at
// /api/inngest, replaying completed step results so only unfinished steps
// run. Set INNGEST_DEV=1 in development; production wants
// INNGEST_SIGNING_KEY and INNGEST_EVENT_KEY.
package flows

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/inngest/inngestgo"

	"github.com/mbriggs/go-bootstrap/appname"
	"github.com/mbriggs/go-bootstrap/logging"
)

var logger = logging.Logger("flows")

// ErrNotConfigured: Send before Configure is a wiring error — webtest
// never configures flows, so a handler reaching here from a test means the
// event belongs behind a seam or the call belongs elsewhere.
var ErrNotConfigured = errors.New("flows: Send before Configure — call flows.Configure at boot")

// client is process-wide like db.Conn and jobs.Client; Send is the
// package's door to it.
var client inngestgo.Client

// Configure builds the Inngest client, registers every flow, and returns
// the handler main mounts at /api/inngest.
func Configure() (http.Handler, error) {
	c, err := inngestgo.NewClient(inngestgo.ClientOpts{
		AppID:  appID(),
		Logger: logger,
	})
	if err != nil {
		return nil, fmt.Errorf("creating inngest client: %w", err)
	}

	// Every flow registers here, before the handler serves its first sync.
	if err := registerWelcome(c); err != nil {
		return nil, fmt.Errorf("registering welcome flow: %w", err)
	}

	client = c

	return c.Serve(), nil
}

// Send publishes an event for flows to react to. Event names are
// "noun.action" namespaced by app area, e.g. "app/user.created".
func Send(ctx context.Context, name string, data map[string]any) error {
	if client == nil {
		return fmt.Errorf("%w (event %s)", ErrNotConfigured, name)
	}

	if _, err := client.Send(ctx, inngestgo.Event{Name: name, Data: data}); err != nil {
		return fmt.Errorf("sending %s: %w", name, err)
	}

	return nil
}

// appID derives from the module path, like database names.
func appID() string {
	if name := appname.Base(); name != "" {
		return name
	}

	return "app"
}
