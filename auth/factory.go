package auth

import (
	"context"
	"fmt"

	"github.com/mbriggs/go-bootstrap/fixture"
)

// MakePassword is the known cleartext for factory users — tests that sign
// in through the UI need it.
const MakePassword = "factory-pw"

var makeSeq = fixture.NewSequence()

// MakeOption customizes the user Make inserts.
type MakeOption func(*CreateInput)

func WithEmail(email string) MakeOption { return func(in *CreateInput) { in.Email = email } }

func WithPassword(pw string) MakeOption { return func(in *CreateInput) { in.Password = pw } }

func WithName(name string) MakeOption { return func(in *CreateInput) { in.Name = name } }

func WithRoles(roles ...string) MakeOption { return func(in *CreateInput) { in.Roles = roles } }

// Make inserts a user with unique random-tagged defaults, for tests that
// need an actor without caring about the particulars. Uniqueness keeps
// parallel tests row-scoped (see the webtest-workflow skill).
func Make(ctx context.Context, opts ...MakeOption) (User, error) {
	tag := makeSeq.Next()
	in := CreateInput{
		Email:    fmt.Sprintf("user-%s@example.com", tag),
		Password: MakePassword,
		Name:     "User " + tag,
	}
	for _, opt := range opts {
		opt(&in)
	}

	return Create(ctx, in)
}
