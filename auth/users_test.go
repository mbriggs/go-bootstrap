package auth_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/mbriggs/go-bootstrap/auth"
	"github.com/mbriggs/go-bootstrap/db"
	"github.com/mbriggs/go-bootstrap/webtest"
)

func TestMain(m *testing.M) { webtest.Main(m) }

// No per-test rollback in the webtest model — every test uses its own
// unique emails, so they can all run in parallel.

func TestCreateHashesPasswordAndAuthenticates(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	created, err := auth.Create(ctx, auth.CreateInput{
		Email:    "hash@example.com",
		Password: "s3cret-pw",
		Name:     "Hash",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.PasswordHash == "s3cret-pw" || created.PasswordHash == "" {
		t.Fatal("password stored unhashed or empty")
	}

	got, err := auth.Authenticate(ctx, "hash@example.com", "s3cret-pw")
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("authenticated id = %d, want %d", got.ID, created.ID)
	}
}

func TestAuthenticateRejectsBadPassword(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	if _, err := auth.Create(ctx, auth.CreateInput{Email: "badpw@example.com", Password: "right-pw"}); err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err := auth.Authenticate(ctx, "badpw@example.com", "wrong")
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("err = %v, want ErrInvalidCredentials", err)
	}
}

func TestAuthenticateRejectsUnknownEmailWithSameError(t *testing.T) {
	t.Parallel()

	_, err := auth.Authenticate(t.Context(), "nobody@example.com", "whatever")
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("err = %v, want ErrInvalidCredentials", err)
	}
}

func TestCreateRejectsDuplicateEmailCaseInsensitively(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	if _, err := auth.Create(ctx, auth.CreateInput{Email: "dupe@example.com", Password: "throwaway-pw"}); err != nil {
		t.Fatalf("first create: %v", err)
	}

	_, err := auth.Create(ctx, auth.CreateInput{Email: "DUPE@example.com", Password: "throwaway-pw"})
	if !errors.Is(err, auth.ErrEmailTaken) {
		t.Fatalf("err = %v, want ErrEmailTaken", err)
	}
}

func TestCreateDefaultsNameFromEmailLocalPart(t *testing.T) {
	t.Parallel()

	created, err := auth.Create(t.Context(), auth.CreateInput{Email: "named@example.com", Password: "throwaway-pw"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Name != "named" {
		t.Fatalf("name = %q, want %q", created.Name, "named")
	}
}

func TestCreateValidatesRequiredFields(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	if _, err := auth.Create(ctx, auth.CreateInput{Password: "throwaway-pw"}); !errors.Is(err, auth.ErrEmailRequired) {
		t.Fatalf("err = %v, want ErrEmailRequired", err)
	}
	if _, err := auth.Create(ctx, auth.CreateInput{Email: "novalid@example.com"}); !errors.Is(err, auth.ErrPasswordRequired) {
		t.Fatalf("err = %v, want ErrPasswordRequired", err)
	}
}

func TestCreateEnforcesPasswordPolicy(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	_, err := auth.Create(ctx, auth.CreateInput{Email: "short@example.com", Password: "seven77"})
	if !errors.Is(err, auth.ErrPasswordTooShort) {
		t.Fatalf("err = %v, want ErrPasswordTooShort", err)
	}

	_, err = auth.Create(ctx, auth.CreateInput{Email: "long@example.com", Password: strings.Repeat("a", 73)})
	if !errors.Is(err, auth.ErrPasswordTooLong) {
		t.Fatalf("err = %v, want ErrPasswordTooLong", err)
	}
}

func TestCreateStoresRolesAndHasRole(t *testing.T) {
	t.Parallel()

	created, err := auth.Create(t.Context(), auth.CreateInput{
		Email:    "roles@example.com",
		Password: "throwaway-pw",
		Roles:    []string{auth.RoleAdmin, "support"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !created.HasRole(auth.RoleAdmin) || !created.HasRole("support") {
		t.Fatalf("roles = %v, want admin+support", created.Roles)
	}
	if created.HasRole("viewer") {
		t.Fatalf("roles = %v, unexpectedly has viewer", created.Roles)
	}
}

func TestByEmailMatchesCaseInsensitively(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	created, err := auth.Create(ctx, auth.CreateInput{Email: "Mixed.Case@example.com", Password: "throwaway-pw"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := auth.ByEmail(ctx, "mixed.case@EXAMPLE.com")
	if err != nil {
		t.Fatalf("by email: %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("id = %d, want %d", got.ID, created.ID)
	}
}

func TestByIDReturnsNotFoundSentinel(t *testing.T) {
	t.Parallel()

	_, err := auth.ByID(t.Context(), 99999999)
	if !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("err = %v, want db.ErrNotFound", err)
	}
}

var errForceRollback = errors.New("force rollback")

func TestMakeInsertsUniqueUsersWithKnownPassword(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	first, err := auth.Make(ctx)
	if err != nil {
		t.Fatalf("make: %v", err)
	}
	second, err := auth.Make(ctx)
	if err != nil {
		t.Fatalf("make: %v", err)
	}
	if first.Email == second.Email {
		t.Fatalf("factory reused email %q", first.Email)
	}

	if _, err := auth.Authenticate(ctx, first.Email, auth.MakePassword); err != nil {
		t.Fatalf("authenticate factory user: %v", err)
	}

	admin, err := auth.Make(ctx, auth.WithRoles(auth.RoleAdmin))
	if err != nil {
		t.Fatalf("make admin: %v", err)
	}
	if !admin.HasRole(auth.RoleAdmin) {
		t.Fatalf("roles = %v, want admin", admin.Roles)
	}
}

func TestCreateInsideTxParticipatesInRollback(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	rollback := errForceRollback
	err := db.ExecInTx(ctx, func(tx pgx.Tx) error {
		if _, err := auth.CreateTx(ctx, tx, auth.CreateInput{Email: "rolledback@example.com", Password: "throwaway-pw"}); err != nil {
			return fmt.Errorf("create in tx: %w", err)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("err = %v, want forced rollback", err)
	}

	if _, err := auth.ByEmail(ctx, "rolledback@example.com"); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("err = %v, want db.ErrNotFound after rollback", err)
	}
}
