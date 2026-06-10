// Command createuser provisions a login from the command line — the way the
// first admin gets created on a fresh database.
//
// Usage:
//
//	go run ./cmd/createuser -email you@example.com -password secret -roles admin
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/mbriggs/go-bootstrap/auth"
	"github.com/mbriggs/go-bootstrap/db"
	"github.com/mbriggs/go-bootstrap/flows"
)

func main() {
	os.Exit(run())
}

func run() int {
	email := flag.String("email", "", "email address (required)")
	password := flag.String("password", "", "password (required)")
	name := flag.String("name", "", "display name (defaults to the email local part)")
	roles := flag.String("roles", "", "comma-separated roles, e.g. admin")
	flag.Parse()

	ctx := context.Background()

	if err := db.Configure(ctx, ""); err != nil {
		fmt.Fprintln(os.Stderr, "createuser: db:", err)
		return 1
	}
	defer db.Close()

	var roleList []string
	for role := range strings.SplitSeq(*roles, ",") {
		if role = strings.TrimSpace(role); role != "" {
			roleList = append(roleList, role)
		}
	}

	user, err := auth.Create(ctx, auth.CreateInput{
		Email:    *email,
		Password: *password,
		Name:     *name,
		Roles:    roleList,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "createuser:", err)
		return 1
	}

	fmt.Printf("created user %d (%s)\n", user.ID, user.Email)

	// Best-effort: the welcome flow is orchestration sugar, and the first
	// admin usually gets created before any Inngest server is running.
	if _, err := flows.Configure(); err != nil {
		fmt.Fprintln(os.Stderr, "createuser: welcome event skipped:", err)
		return 0
	}
	if err := flows.Send(ctx, flows.UserCreated, map[string]any{
		"user_id": user.ID,
		"email":   user.Email,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "createuser: welcome event not delivered (fine if no Inngest server is up):", err)
	}

	return 0
}
